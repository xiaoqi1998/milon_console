package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/static/x.js", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, "{}") })

	for _, path := range []string{"/", "/static/x.js", "/api/health"} {
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, httptest.NewRequest("GET", path, nil))

		if resp.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", path, resp.Code)
		}
		if got := resp.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Fatalf("%s: X-Frame-Options = %q, want DENY", path, got)
		}
		csp := resp.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Fatalf("%s: CSP missing frame-ancestors 'none': %q", path, csp)
		}
		if !strings.Contains(csp, "script-src 'self'") {
			t.Fatalf("%s: CSP missing script-src 'self': %q", path, csp)
		}
		if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("%s: X-Content-Type-Options = %q, want nosniff", path, got)
		}
		if got := resp.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
			t.Fatalf("%s: Referrer-Policy = %q", path, got)
		}
	}
}
