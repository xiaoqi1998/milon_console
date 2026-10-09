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
	req.Header.Set("Access-Control-Request-Headers", "X-Milon-Network")

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	allowed := resp.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(allowed, "X-Milon-Network") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want it to contain X-Milon-Network (preflight would fail for browser clients)", allowed)
	}
}
