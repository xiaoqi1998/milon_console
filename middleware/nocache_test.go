package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNoCacheHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/static/x.js", NoCache(), func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest("GET", "/static/x.js", nil))
	if resp.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", resp.Header().Get("Cache-Control"))
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
}
