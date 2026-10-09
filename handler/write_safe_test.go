package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

// newTask5Router 组装 write-safe 与 error_lookup 路由；fake RPC 返回协议错误，
// 使 write-safe 停在模拟阶段（这正是它要防住的上链前失败）。
func newTask5Router(t *testing.T) *gin.Engine {
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
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	api.POST("/write-safe", NewContractHandler(nm).WriteSafe)
	api.GET("/errors/:query", NewErrorLookupHandler().Lookup)
	return r
}

// TestWriteSafeStopsAtSimulate：模拟失败 → 200 + stage=simulate_failed，
// 绝不进入提交（fake 若收到 submit 也只会返回错误，但 stage 已证明停在模拟）。
func TestWriteSafeStopsAtSimulate(t *testing.T) {
	r := newTask5Router(t)
	w := httptest.NewRecorder()
	body := `{"appName":"token","methodName":"Transfer","args":{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000},"paymentMode":"unified_payer_all","payerPrivateKey":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","payerAddress":"2DSTxAj1h6JtrhyayNFo4heZBa29","signatureMode":{"type":"pubkey","publicKey":"0x02f175c4673255dc8e674d70c3d6d45550ccebafb745fbd386fa52ce6f74bb2ef6"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/write-safe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Stage string `json:"stage"`
			Error string `json:"error"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.Data.Stage != "simulate_failed" {
		t.Fatalf("want stage=simulate_failed, got %q: %s", resp.Data.Stage, w.Body.String())
	}
}

// TestErrorLookup：数字命中 API 码表；名字子串命中 IDL 链上错误码；
// 无命中返回空数组（200）。
func TestErrorLookup(t *testing.T) {
	r := newTask5Router(t)

	get := func(q string) (int, string) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/errors/"+q, nil))
		return w.Code, w.Body.String()
	}

	code, body := get("404")
	if code != http.StatusOK || !strings.Contains(body, "NOT_FOUND") {
		t.Fatalf("404 应命中 API 码表: %d %s", code, body)
	}
	code, body = get("Cooldown")
	if code != http.StatusOK || !strings.Contains(body, "FaucetCooldownActive") {
		t.Fatalf("Cooldown 应命中 token app 链上错误: %d %s", code, body)
	}
	code, body = get("zzz_nothing")
	if code != http.StatusOK || !strings.Contains(body, `"data":[]`) {
		t.Fatalf("无命中应返回空数组: %d %s", code, body)
	}
}
