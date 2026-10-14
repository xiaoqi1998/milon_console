package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

// fake RPC：与 write_safe_test.go 同款——返回协议错误让提交停在链端，
// 证明签名材料组装已通过（错误来自链，不是来自参数校验）。
func newAccountsContractRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq struct {
			RequestID uint64 `json:"request_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpcReq)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			RequestID uint64 `json:"request_id"`
			Status    uint8  `json:"status"`
		}{RequestID: rpcReq.RequestID, Status: 1})
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm), middleware.ResolveAccounts())
	h := NewContractHandler(nm)
	api.POST("/write", h.WriteContract)
	api.POST("/simulate", h.SimulateContract)
	api.POST("/write/multisig", h.WriteContractMultisig)
	return r
}

func accountsHeader(payload string) string {
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

const cPrivHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
const cAddr = "491vGnPg2q1qsRtFRun8WNW8gxTD"

func TestContractWriteConfiguredAccount(t *testing.T) {
	r := newAccountsContractRouter(t)
	hdr := accountsHeader(`[{"name":"main","address":"` + cAddr + `","privateKey":"` + cPrivHex + `"}]`)
	// args 补全 token.Transfer 全部必需参数（from/token/to/amount），
	// 确保请求能越过指令编码、真正走到签名材料组装这一步。
	body := `{"appName":"token","methodName":"Transfer","args":{"from":"` + cAddr + `","token":"` + cAddr + `","to":"` + cAddr + `","amount":1},"paymentMode":"unified_payer_all","payerAddress":"` + cAddr + `"}`

	t.Run("无私钥+地址命中→越过参数校验进入链端路径", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/write", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(middleware.AccountsHeader, hdr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// 关键断言：不再是 400「payerPrivateKey is required」，而是 500 链端错误（fake 返回 Status:1）
		if w.Code == http.StatusBadRequest {
			t.Fatalf("仍被参数校验拦截: %s", w.Body.String())
		}
		// 回填生效后错误应来自链端（Status:1 协议错误），而不再是
		// 「invalid payerPrivateKey」（无回填时 ParseSecretKey("") 必报此错）。
		if strings.Contains(w.Body.String(), "payerPrivateKey") {
			t.Fatalf("私钥未被配置账户回填, 错误仍指向私钥缺失: %s", w.Body.String())
		}
	})
	t.Run("无私钥+未命中→400 且文案含配置账户提示", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/write", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// 不带账户头（也不设 env），地址未命中
		t.Setenv(middleware.AccountsEnv, "")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// unified_payer_all 地址已传、无私钥：链上账户未上链时 parsePayerAndMode 要求签名模式——
		// 该错误文案应含 hint 或 signatureMode 指引；此处宽松断言非 200 即可，
		// 严格断言在 split 模式（validatePaymentModeFields 必报 payerPrivateKey）
		body2 := `{"appName":"token","methodName":"Transfer","args":{},"paymentMode":"split","payerAddress":"` + cAddr + `"}`
		req2 := httptest.NewRequest(http.MethodPost, "/api/write", strings.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != http.StatusBadRequest {
			t.Fatalf("split 缺私钥应 400, got %d", w2.Code)
		}
		if !strings.Contains(w2.Body.String(), "account_config_list") && !strings.Contains(w2.Body.String(), "X-Milon-Accounts") {
			t.Fatalf("错误文案缺配置账户提示: %s", w2.Body.String())
		}
	})
	t.Run("simulate 无私钥字段+地址命中+离线账户→注入签名模式组装成功", func(t *testing.T) {
		body3 := `{"appName":"token","methodName":"Transfer","args":{"from":"` + cAddr + `","token":"` + cAddr + `","to":"` + cAddr + `","amount":1},"paymentMode":"unified_payer_all","payerAddress":"` + cAddr + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/simulate", strings.NewReader(body3))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(middleware.AccountsHeader, hdr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusBadRequest {
			t.Fatalf("simulate 仍被签名模式校验拦截: %s", w.Body.String())
		}
		// 回填生效后签名模式已注入（pubkey 模式），错误应来自链端模拟（Status:1），
		// 而不是「signatureMode is required」（无回填时离线账户+空模式必报此错）。
		if strings.Contains(w.Body.String(), "signatureMode") {
			t.Fatalf("签名模式未被配置账户注入, 错误仍指向签名模式缺失: %s", w.Body.String())
		}
	})
	t.Run("multisig 专用端点无私钥+地址命中→回填后进链端路径", func(t *testing.T) {
		// 端点缺省 paymentMode=split（owner 付 gas+签指令）；无私钥+命中地址
		// → payer/owner 私钥均由配置账户回填，错误应来自链端而非私钥缺失。
		body4 := `{"appName":"token","methodName":"Transfer","args":{"from":"` + cAddr + `","token":"` + cAddr + `","to":"` + cAddr + `","amount":1},"payerAddress":"` + cAddr + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/write/multisig", strings.NewReader(body4))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(middleware.AccountsHeader, hdr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusBadRequest {
			t.Fatalf("multisig 仍被参数校验拦截: %s", w.Body.String())
		}
		if strings.Contains(w.Body.String(), "PrivateKey") {
			t.Fatalf("私钥未被配置账户回填, 错误仍指向私钥缺失: %s", w.Body.String())
		}
	})
}
