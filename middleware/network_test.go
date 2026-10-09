package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"

	"github.com/gin-gonic/gin"
)

// newNetworkRouter 组装 ResolveNetwork + 探针路由（回显解析结果）。
// fake RPC 后端经 MILON_RPC_URL 注入为 devNet 端点（NewClient 不拨号，安全）。
func newNetworkRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(fake.Close)
	t.Setenv("MILON_RPC_URL", fake.URL)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	nm := client.NewNetworkManager("devNet", fake.URL)
	r.Use(func(c *gin.Context) { c.Set("nm", nm); c.Next() })
	r.Use(ResolveNetwork(nm))
	r.GET("/probe", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"hasClient": ClientFrom(c) != nil,
			"network":   NetworkNameFrom(c),
		})
	})
	return r
}

func TestResolveNetwork(t *testing.T) {
	r := newNetworkRouter(t)

	cases := []struct {
		header string
		want   string
	}{
		{"", "devNet"},           // 不带 → 默认网络
		{"devNet", "devNet"},     // 显式 devNet
		{"localNet", "localNet"}, // 显式 localNet
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		if tc.header != "" {
			req.Header.Set(NetworkHeader, tc.header)
		}
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("header=%q http %d: %s", tc.header, w.Code, w.Body.String())
		}
		var out struct {
			HasClient bool   `json:"hasClient"`
			Network   string `json:"network"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if !out.HasClient || out.Network != tc.want {
			t.Errorf("header=%q got client=%v network=%q, want %q", tc.header, out.HasClient, out.Network, tc.want)
		}
	}

	// 未知名 → 400 ERR_INVALID_PARAMETER
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set(NetworkHeader, "mainNet")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未知网络应 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "mainNet") || !strings.Contains(w.Body.String(), "400") {
		t.Fatalf("错误响应应含未知名与错误码: %s", w.Body.String())
	}
}
