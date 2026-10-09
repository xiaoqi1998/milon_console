package handler

import (
	"net/http"

	"milon-api-server/mcpserver"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// McpToolsHandler 暴露 MCP 工具清单（GET /api/mcp/tools），数据与 /mcp
// 端点注册表同源（mcpserver.ToolInventory），供前端「MCP 接入」页面与
// 外部集成方程序化发现全部工具。
type McpToolsHandler struct{}

// NewMcpToolsHandler creates a McpToolsHandler.
func NewMcpToolsHandler() *McpToolsHandler {
	return &McpToolsHandler{}
}

// ListTools handles GET /api/mcp/tools
func (h *McpToolsHandler) ListTools(c *gin.Context) {
	tools := mcpserver.ToolInventory()
	c.JSON(http.StatusOK, types.SuccessResponse(tools, "ok"))
}
