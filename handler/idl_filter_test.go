package handler

// 2026-10 AI 提速（Step 5）：idl_metadata 支持 ?apps=a,b 过滤。
// 全量 metadata 约 200KB，AI 只需一两个 app 时整包返回既慢又撑上下文——
// 过滤后配合 idl_apps 先看清单再按需取，响应可缩一个数量级。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

func newIDLRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	ih := NewIDLHandler(nm)
	api.GET("/idl/metadata", ih.GetIDLMetadata)
	return r
}

// TestIDLMetadataAppsFilter：?apps=token 只返回 token 一个 app；
// 未知名以 400 报出全部可用名（AI 一轮自纠）；不传时行为不变（全部）。
func TestIDLMetadataAppsFilter(t *testing.T) {
	r := newIDLRouter(t)

	get := func(q string) (int, []string, string) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/idl/metadata"+q, nil))
		var resp struct {
			Success bool `json:"success"`
			Data    []struct {
				Name string `json:"name"`
			} `json:"data"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		names := make([]string, 0, len(resp.Data))
		for _, d := range resp.Data {
			names = append(names, d.Name)
		}
		return w.Code, names, w.Body.String()
	}

	code, names, _ := get("?apps=token")
	if code != http.StatusOK || len(names) != 1 || names[0] != "token" {
		t.Fatalf("?apps=token 应只返回 token: %d %v", code, names)
	}

	code, names, _ = get("")
	if code != http.StatusOK || len(names) < 5 {
		t.Fatalf("不传应返回全部 app（实测 %d 个）: %d %v", len(names), code, names)
	}

	code, _, body := get("?apps=token,nonexistent_app")
	if code != http.StatusBadRequest || !containsAny(body, "nonexistent_app") {
		t.Fatalf("未知名应 400 且报出名字与可用清单: %d %s", code, body[:min(len(body), 200)])
	}
}

func containsAny(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ==================== method 单方法过滤（2026-11 六刀刀3a） ====================

// newIDLMethodsRouter 组装 GET /api/idl/apps/:appName/methods 路由。
func newIDLMethodsRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	ih := NewIDLHandler(nm)
	api.GET("/idl/apps/:appName/methods", ih.AppMethods)
	return r
}

// TestAppMethodsSingleMethodFilter：?method= 只返回单个方法（大小写宽容）；
// 未知名 400 报出全部可用方法名（错误路径即发现路径）；缺省全量不变。
func TestAppMethodsSingleMethodFilter(t *testing.T) {
	r := newIDLMethodsRouter(t)

	get := func(q string) (int, []string, string) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/idl/apps/token/methods"+q, nil))
		var resp struct {
			Success bool `json:"success"`
			Data    struct {
				Instructions []struct {
					Name string `json:"name"`
				} `json:"instructions"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		names := make([]string, 0, len(resp.Data.Instructions))
		for _, i := range resp.Data.Instructions {
			names = append(names, i.Name)
		}
		return w.Code, names, w.Body.String()
	}

	// 全量基线：token app 不止一个方法
	code, all, _ := get("")
	if code != http.StatusOK || len(all) < 2 {
		t.Fatalf("缺省应全量（token 实测 %d 个方法）: %d %v", len(all), code, all)
	}

	// 单方法（小写传入，PascalCase 命中——与 contract_write 的方法名宽容一致）
	code, one, _ := get("?method=transfer")
	if code != http.StatusOK || len(one) != 1 || one[0] != "Transfer" {
		t.Fatalf("?method=transfer 应只回 Transfer: %d %v", code, one)
	}

	// 未知名：400 且报可用方法清单
	code, _, body := get("?method=NoSuchMethod")
	if code != http.StatusBadRequest || !containsAny(body, "Transfer") {
		t.Fatalf("未知名应 400 且报可用清单: %d %.200s", code, body)
	}
}
