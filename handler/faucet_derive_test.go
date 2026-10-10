package handler

// 2026-10 AI 易用性修复（Step C-faucet）：faucet_claim 的 signatureMode 此前
// binding:"required"——即使传了 privateKey+address 也不给缺省。修复后：
// signatureMode 缺省时从 privateKey 派生（keyType 可选，缺省 secp256k1）。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"

	"github.com/gin-gonic/gin"
	"milon-api-server/middleware"
)

func newFaucetRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1,"message":"fake"}`))
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	api.POST("/faucet/claim", NewFaucetHandler(nm).ClaimFaucet)
	return r
}

// TestFaucetClaimAutoDerivesSignatureMode：signatureMode 缺省 → 从私钥派生，
// 不应再报 signatureMode 相关错误（后续在假 RPC 上失败与签名解析无关）。
func TestFaucetClaimAutoDerivesSignatureMode(t *testing.T) {
	r := newFaucetRouter(t)
	w := httptest.NewRecorder()
	body := `{"privateKey":"` + testSeedHex + `","address":"2DSTxAj1h6JtrhyayNFo4heZBa29"}`
	req := httptest.NewRequest(http.MethodPost, "/api/faucet/claim", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "signatureMode") {
		t.Fatalf("signatureMode 缺省应自动派生，不应报 signatureMode 错误: %d %s", w.Code, w.Body.String())
	}
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "invalid request body") {
		t.Fatalf("binding 不应再强制 signatureMode: %s", w.Body.String())
	}
}
