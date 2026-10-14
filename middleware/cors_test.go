package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSPreflightAllowsNetworkHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SetupCORS("http://example.com"))
	r.POST("/anything", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest("OPTIONS", "/anything", nil)
	// httptest.NewRequest 默认 Host=example.com，与 Origin 同源会被库跳过，这里模拟真实跨域 preflight
	req.Host = "api.example.com"
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Milon-Network, X-Milon-Accounts")

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	allowed := resp.Header().Get("Access-Control-Allow-Headers")
	for _, h := range []string{"X-Milon-Network", "X-Milon-Accounts"} {
		if !strings.Contains(allowed, h) {
			t.Fatalf("Access-Control-Allow-Headers = %q, want it to contain %s (preflight would fail for browser clients)", allowed, h)
		}
	}
}
