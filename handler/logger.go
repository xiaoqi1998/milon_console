package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// requestID 从 gin.Context 中读取 requestId，若为空则返回 "unknown"。
func requestID(c *gin.Context) string {
	rid := c.GetString("requestId")
	if rid == "" {
		return "unknown"
	}
	return rid
}

// logParamError 记录参数解析失败日志（warn 级别）。
func logParamError(c *gin.Context, endpoint string, err error) {
	rid := requestID(c)
	slog.Warn("param error", "requestId", rid, "endpoint", endpoint, "error", err.Error())
}

// logSDKError 记录 SDK 调用失败日志（error 级别）。
func logSDKError(c *gin.Context, endpoint string, err error) {
	rid := requestID(c)
	slog.Error("sdk error", "requestId", rid, "endpoint", endpoint, "error", err.Error())
}

// logBusinessInfo 记录关键业务操作日志（info 级别），fields 为额外的 key-value 对。
func logBusinessInfo(c *gin.Context, endpoint string, fields ...any) {
	rid := requestID(c)
	slog.Info("business", append([]any{"requestId", rid, "endpoint", endpoint}, fields...)...)
}
