package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"milon-api-server/mcpserver"

	"github.com/gin-gonic/gin"
)

// TestMcpToolsEndpoint 锁定 GET /api/mcp/tools：标准成功响应包裹全部
// MCP 工具清单（与 mcpserver 注册表同源），五字段齐备且总数 = 57。
// handler 包测试不经过 main 的 /mcp 挂载，同时验证 ToolInventory 的
// 惰性初始化在 REST-only 场景下可用。
func TestMcpToolsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/mcp/tools", NewMcpToolsHandler().ListTools)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/mcp/tools", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("http %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Group       string `json:"group"`
			Method      string `json:"method"`
			Path        string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("success=false: %s", w.Body.String())
	}
	// 数量与 ToolInventory 同源对账，另设下限防工具意外丢失（批次全部落地后升 64）
	if len(resp.Data) != len(mcpserver.ToolInventory()) {
		t.Fatalf("端点返回 %d 个 != inventory %d 个", len(resp.Data), len(mcpserver.ToolInventory()))
	}
	if len(resp.Data) < 64 {
		t.Fatalf("工具数=%d, want ≥64", len(resp.Data))
	}
	seen := map[string]bool{}
	for _, ti := range resp.Data {
		if ti.Name == "" || ti.Description == "" || ti.Group == "" || ti.Method == "" || ti.Path == "" {
			t.Errorf("%+v 存在空字段", ti)
		}
		if seen[ti.Name] {
			t.Errorf("重复工具 %s", ti.Name)
		}
		seen[ti.Name] = true
	}
}
