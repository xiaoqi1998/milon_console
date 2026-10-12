package handler

// 安全不变量测试：服务器不持久化任何私钥。
// saved-instructions 只存"怎么调用"（方法/参数/地址/签名模式），
// 签名身份在 send 执行时由请求方临时提供（过境即焚）。
// 覆盖三条路径：Create 落盘前剥离、Update 落盘前剥离、旧文件 load 洗刷重写。

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

// newSavedInstrRouter 构造 saved-instructions 测试路由：fake RPC + 临时数据目录。
func newSavedInstrRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1,"message":"fake"}`))
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	dataDir := t.TempDir()
	h, err := NewSavedInstructionHandler(nm, dataDir)
	if err != nil {
		t.Fatalf("init handler: %v", err)
	}
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	api.POST("/saved-instructions", h.CreateSavedInstruction)
	api.GET("/saved-instructions", h.ListSavedInstructions)
	api.GET("/saved-instructions/:id", h.GetSavedInstruction)
	api.PUT("/saved-instructions/:id", h.UpdateSavedInstruction)
	api.POST("/saved-instructions/:id/execute", h.ExecuteSavedInstruction)
	return r, filepath.Join(dataDir, "saved_instructions.json")
}

func savedDo(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(w, req)
	return w
}

var savedSecrets = []string{
	"SECRET-PAYER-KEY", "SECRET-OWNER-KEY", "SECRET-IX-KEY",
	"SECRET-SIGNER-KEY", "SECRET-GAS-KEY", "LEGACY-SECRET-KEY",
}

const createBodyWithKeys = `{"name":"sec-test","appName":"token","methodName":"Transfer",` +
	`"args":{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000},` +
	`"paymentMode":"unified_payer_all","payerAddress":"2DSTxAj1h6JtrhyayNFo4heZBa29",` +
	`"payerPrivateKey":"SECRET-PAYER-KEY","ownerPrivateKey":"SECRET-OWNER-KEY","ixPrivateKey":"SECRET-IX-KEY",` +
	`"signers":[{"address":"2DSTxAj1h6JtrhyayNFo4heZBa29","privateKey":"SECRET-SIGNER-KEY"}],` +
	`"gasPayer":{"address":"2DSTxAj1h6JtrhyayNFo4heZBa29","privateKey":"SECRET-GAS-KEY"}}`

func assertNoSecrets(t *testing.T, label, s string) {
	t.Helper()
	for _, secret := range savedSecrets {
		if strings.Contains(s, secret) {
			t.Fatalf("%s 泄漏密钥 %s", label, secret)
		}
	}
}

// TestSavedInstructionCreateStripsSecrets：带密钥 create → 响应/列表/落盘文件三处均无密钥。
func TestSavedInstructionCreateStripsSecrets(t *testing.T) {
	r, filePath := newSavedInstrRouter(t)

	w := savedDo(r, http.MethodPost, "/api/saved-instructions", createBodyWithKeys)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	assertNoSecrets(t, "create 响应", w.Body.String())

	w = savedDo(r, http.MethodGet, "/api/saved-instructions", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	assertNoSecrets(t, "匿名 list 响应", w.Body.String())

	file, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read store file: %v", err)
	}
	assertNoSecrets(t, "落盘文件", string(file))

	// id 解析出来供 PUT 用
	idStart := strings.Index(w.Body.String(), `"id":"`)
	if idStart < 0 {
		t.Fatalf("list 无 id: %s", w.Body.String())
	}
	id := w.Body.String()[idStart+6:]
	id = id[:strings.Index(id, `"`)]

	// Update 路径同样不得落密钥
	w = savedDo(r, http.MethodPut, "/api/saved-instructions/"+id,
		`{"payerPrivateKey":"SECRET-PAYER-KEY"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	assertNoSecrets(t, "update 响应", w.Body.String())
	file, _ = os.ReadFile(filePath)
	assertNoSecrets(t, "update 后落盘文件", string(file))
}

// TestSavedInstructionExecuteSendKeyContract：send 模式密钥从请求体临时提供。
func TestSavedInstructionExecuteSendKeyContract(t *testing.T) {
	r, _ := newSavedInstrRouter(t)

	// create 不带任何密钥（新契约下的正常保存形态）；args 用完整参数表，
	// 保证 encode 阶段放行、错误定位在密钥契约上
	w := savedDo(r, http.MethodPost, "/api/saved-instructions",
		`{"name":"send-test","appName":"token","methodName":"Transfer",`+
			`"args":{"from":"2DSTxAj1h6JtrhyayNFo4heZBa29","to":"2DSTxAj1h6JtrhyayNFo4heZBa29","token":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000},`+
			`"paymentMode":"unified_payer_all","payerAddress":"2DSTxAj1h6JtrhyayNFo4heZBa29"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	idStart := strings.Index(w.Body.String(), `"id":"`)
	id := w.Body.String()[idStart+6:]
	id = id[:strings.Index(id, `"`)]

	// (a) send 不带密钥 → 400，且错误信息指明缺 payerPrivateKey
	w = savedDo(r, http.MethodPost, "/api/saved-instructions/"+id+"/execute?mode=send", "{}")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("无密钥 send 应 400: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "payerPrivateKey") {
		t.Fatalf("错误信息应指明缺少 payerPrivateKey: %s", w.Body.String())
	}

	// (b) send 请求体带密钥 → 必须通过密钥校验（不得再报 privateKey 类 400）
	w = savedDo(r, http.MethodPost, "/api/saved-instructions/"+id+"/execute?mode=send",
		`{"payerPrivateKey":"`+testSeedHex+`"}`)
	if w.Code == http.StatusBadRequest && strings.Contains(strings.ToLower(w.Body.String()), "privatekey") {
		t.Fatalf("带密钥仍被 privateKey 校验拒绝: %d %s", w.Code, w.Body.String())
	}

	// (c) read/simulate 模式不受影响（不读密钥，走正常链路）
	w = savedDo(r, http.MethodPost, "/api/saved-instructions/"+id+"/execute?mode=simulate", "")
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "privateKey") {
		t.Fatalf("simulate 不应涉及密钥校验: %d %s", w.Code, w.Body.String())
	}
}

// TestSavedInstructionLoadScrubsLegacyKeys：旧版本落盘的密钥在 load 时被洗刷并重写文件。
func TestSavedInstructionLoadScrubsLegacyKeys(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1,"message":"fake"}`))
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)

	dataDir := t.TempDir()
	legacy := `[{"id":"legacy1","name":"old","appName":"token","methodName":"Transfer","kind":"entry",` +
		`"paymentMode":"unified_payer_all","payerAddress":"2DSTxAj1h6JtrhyayNFo4heZBa29",` +
		`"payerPrivateKey":"LEGACY-SECRET-KEY",` +
		`"signers":[{"address":"2DSTxAj1h6JtrhyayNFo4heZBa29","privateKey":"SECRET-SIGNER-KEY"}],` +
		`"gasPayer":{"address":"2DSTxAj1h6JtrhyayNFo4heZBa29","privateKey":"SECRET-GAS-KEY"},` +
		`"createdAt":1,"updatedAt":1}]`
	filePath := filepath.Join(dataDir, "saved_instructions.json")
	if err := os.WriteFile(filePath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	h, err := NewSavedInstructionHandler(nm, dataDir)
	if err != nil {
		t.Fatalf("init handler: %v", err)
	}
	r := gin.New()
	r.GET("/api/saved-instructions", h.ListSavedInstructions)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/saved-instructions", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	assertNoSecrets(t, "旧数据 list 响应", w.Body.String())

	file, _ := os.ReadFile(filePath)
	assertNoSecrets(t, "洗刷后落盘文件", string(file))
}
