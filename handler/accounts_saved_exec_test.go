package handler

// Task 7 契约测试：saved instruction 的 send 模式按「存储侧地址」从本请求
// 配置账户（X-Milon-Accounts）回填一次性私钥——过境即焚不变量保持。
//
// 场景：存储条目只绑 payerAddress + paymentMode=unified_payer_all（密钥永不落盘）；
// send 请求体不带 payerPrivateKey：
//   - 携带该地址的配置账户头 → 回填后越过 requireSendKeys；
//   - 未携带 → 400「send 模式需要私钥…」+ configuredAccountHint（含 account_config_list）。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"milon-api-server/client"
	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// newSavedInstrExecRouter 构造带配置账户解析的 saved-instructions 测试路由。
// 在 saved_instruction_secrets_test.go 的脚手架基础上补挂 middleware.ResolveAccounts
// （不挂则 X-Milon-Accounts 不会被解析进 context，回填无从发生）。
func newSavedInstrExecRouter(t *testing.T) (*gin.Engine, *SavedInstructionHandler) {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1,"message":"fake"}`))
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	h, err := NewSavedInstructionHandler(nm, t.TempDir())
	if err != nil {
		t.Fatalf("init handler: %v", err)
	}
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm), middleware.ResolveAccounts())
	api.POST("/saved-instructions/:id/execute", h.ExecuteSavedInstruction)
	return r, h
}

// mustCreateEntry 直接向 store 写入一条条目，返回其 id。
// 落库形态与线上 create 路径一致：scrubSecrets 后只存地址/签名模式（无私钥）。
func (h *SavedInstructionHandler) mustCreateEntry(t *testing.T, item *SavedInstruction) string {
	t.Helper()
	item.ID = generateID()
	now := time.Now().UnixMilli()
	item.CreatedAt = now
	item.UpdatedAt = now
	scrubSecrets(item)
	if err := h.store.Create(item); err != nil {
		t.Fatalf("create entry: %v", err)
	}
	return item.ID
}

// TestSavedInstructionSendConfiguredAccount：存储绑定 payerAddress=cAddr、
// paymentMode=unified_payer_all；send 请求体不带 payerPrivateKey，但请求携带
// 该地址的配置账户 → 回填后越过 requireSendKeys；未携带 → 400 + hint。
func TestSavedInstructionSendConfiguredAccount(t *testing.T) {
	r, h := newSavedInstrExecRouter(t)
	id := h.mustCreateEntry(t, &SavedInstruction{
		Name: "t", AppName: "token", MethodName: "Transfer", Kind: "entry",
		PaymentMode: "unified_payer_all", PayerAddress: cAddr,
	})
	// mode 只从 query 读取（executeSavedInstructionRequest 不含 mode/id 字段）
	body := `{"id":"` + id + `","mode":"send"}`

	// (a) 携带账户头：回填后不得再被「send 模式需要私钥」预检拦截
	req := httptest.NewRequest(http.MethodPost, "/api/saved-instructions/"+id+"/execute?mode=send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.AccountsHeader, accountsHeader(`[{"address":"`+cAddr+`","privateKey":"`+cPrivHex+`"}]`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "send 模式需要私钥") {
		t.Fatalf("仍被密钥预检拦截: %s", w.Body.String())
	}

	// (b) 不带账户头（env 也清空）：回填未命中 → 400 且错误文案带配置账户 hint
	t.Setenv(middleware.AccountsEnv, "")
	req2 := httptest.NewRequest(http.MethodPost, "/api/saved-instructions/"+id+"/execute?mode=send", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest || !strings.Contains(w2.Body.String(), "account_config_list") {
		t.Fatalf("want 400+hint got %d: %s", w2.Code, w2.Body.String())
	}
}

// TestSavedInstructionSendFillNotPersisted：回填只发生在请求内的 work 副本上，
// 不得写回 store 内存条目——send 之后 list/单查响应仍无任何私钥。
func TestSavedInstructionSendFillNotPersisted(t *testing.T) {
	r, h := newSavedInstrExecRouter(t)
	id := h.mustCreateEntry(t, &SavedInstruction{
		Name: "t2", AppName: "token", MethodName: "Transfer", Kind: "entry",
		PaymentMode: "multi_signer", PayerAddress: cAddr,
		Signers:  []types.SignerEntry{{Address: cAddr}},
		GasPayer: &types.SignerEntry{Address: cAddr},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/saved-instructions/"+id+"/execute?mode=send", nil)
	req.Header.Set(middleware.AccountsHeader, accountsHeader(`[{"address":"`+cAddr+`","privateKey":"`+cPrivHex+`"}]`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 无论执行结果如何，store 内条目不得残留回填的私钥
	got, ok := h.store.Get(id)
	if !ok {
		t.Fatalf("entry missing after execute (resp: %s)", w.Body.String())
	}
	if got.PayerPrivateKey != "" || got.Signers[0].PrivateKey != "" || got.GasPayer.PrivateKey != "" {
		t.Fatalf("回填私钥泄漏进内存 store: %+v", got)
	}
}
