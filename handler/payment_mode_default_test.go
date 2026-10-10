package handler

// 2026-10 AI 易用性修复（Step D）：/api/write/multi-agent 与 /api/write/multisig
// 端点本身已隐含 paymentMode（unified_dual_sign / split），此前仍 binding:"required"
// 强制显式传——纯冗余摩擦。修复后：缺省自动填充，显式传错值仍被拒。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

func newPaymentModeRouter(t *testing.T) *gin.Engine {
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
	ch := NewContractHandler(nm)
	api.POST("/write/multi-agent", ch.WriteContractMultiAgent)
	api.POST("/write/multisig", ch.WriteContractMultisig)
	return r
}

// TestPaymentModeDefaultsOnDedicatedEndpoints：专用端点缺省 paymentMode。
func TestPaymentModeDefaultsOnDedicatedEndpoints(t *testing.T) {
	r := newPaymentModeRouter(t)

	post := func(path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	dualBody := `{"appName":"token","methodName":"Transfer","args":{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000},` +
		`"payerPrivateKey":"` + testSeedHex + `","ixPrivateKey":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1e"}`
	w := post("/api/write/multi-agent", dualBody)
	if strings.Contains(w.Body.String(), "paymentMode") || strings.Contains(w.Body.String(), "Field validation") {
		t.Fatalf("multi-agent 缺省 paymentMode 应放行: %d %s", w.Code, w.Body.String())
	}

	splitBody := `{"appName":"token","methodName":"Transfer","args":{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000},` +
		`"ownerPrivateKey":"` + testSeedHex + `"}`
	w = post("/api/write/multisig", splitBody)
	if strings.Contains(w.Body.String(), "paymentMode") || strings.Contains(w.Body.String(), "Field validation") {
		t.Fatalf("multisig 缺省 paymentMode 应放行: %d %s", w.Code, w.Body.String())
	}

	// 显式传错值仍被拒
	w = post("/api/write/multi-agent", `{"appName":"token","methodName":"Transfer","paymentMode":"split"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unified_dual_sign") {
		t.Fatalf("multi-agent 传错 paymentMode 应 400 并提示正确值: %d %s", w.Code, w.Body.String())
	}
}
