package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

// newIDLPagesRouter 组装 /api/idl/apps 与 /api/idl/apps/:appName/methods 路由。
// IDL 由 SDK 在 NewClient 时内置加载、不触网；fake 后端仅为 NetworkManager 构造兜底。
func newIDLPagesRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(fake.Close)

	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	idl := NewIDLHandler(nm)
	api.GET("/idl/apps", idl.ListApps)
	api.GET("/idl/apps/:appName/methods", idl.AppMethods)
	return r
}

// TestIDLAppsList：轻量清单——每 app 只有 id/name/description/instructionCount
// 四字段（不带 instructions 明细，解决 idl_metadata 一次 200KB 的上下文爆炸）。
func TestIDLAppsList(t *testing.T) {
	r := newIDLPagesRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/idl/apps", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("http %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			AppID            uint8  `json:"appId"`
			Name             string `json:"name"`
			Description      string `json:"description"`
			InstructionCount int    `json:"instructionCount"`
			Instructions     any    `json:"instructions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || len(resp.Data) < 5 {
		t.Fatalf("apps=%d（want ≥5）: %s", len(resp.Data), w.Body.String())
	}
	found := false
	for _, a := range resp.Data {
		if a.Instructions != nil {
			t.Fatalf("轻量清单不应携带 instructions 明细: %s", w.Body.String()[:300])
		}
		if a.Name == "token" {
			found = true
			if a.InstructionCount < 10 {
				t.Fatalf("token 方法数=%d（want ≥10）", a.InstructionCount)
			}
		}
	}
	if !found {
		t.Fatal("apps 清单应含 token")
	}
}

// TestIDLMethodsByApp：单 app 全量方法详情；未知名 400。
func TestIDLMethodsByApp(t *testing.T) {
	r := newIDLPagesRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/idl/apps/token/methods", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("http %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Name         string `json:"name"`
			Instructions []struct {
				Name string `json:"name"`
				Kind string `json:"kind"`
				Args []struct {
					Name string `json:"name"`
				} `json:"args"`
			} `json:"instructions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	hasTransfer := false
	for _, ix := range resp.Data.Instructions {
		if ix.Name == "Transfer" || ix.Name == "transfer" {
			hasTransfer = true
			if len(ix.Args) == 0 {
				t.Fatal("transfer 应携带参数明细")
			}
		}
	}
	if !hasTransfer {
		t.Fatalf("token 方法列表应含 transfer: %s", w.Body.String()[:200])
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/idl/apps/nope/methods", nil))
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("未知 app 应 400, got %d", w2.Code)
	}
}
