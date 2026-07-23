package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// generateRequestID 生成请求 ID：优先复用客户端 X-Request-Id 请求头，
// 否则用 crypto/rand 生成 16 字节随机数的 hex 编码。
func generateRequestID(c *gin.Context) string {
	if id := c.GetHeader("X-Request-Id"); id != "" {
		return id
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// rand.Read 极少失败，回退用当前时间戳保证非空
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000")))
	}
	return hex.EncodeToString(b)
}

// LoggerMiddleware logs each request with structured slog output.
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		rid := generateRequestID(c)
		c.Set("requestId", rid)
		c.Header("X-Request-Id", rid)

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []any{
			"requestId", rid,
			"method", method,
			"path", path,
			"clientIP", c.ClientIP(),
			"userAgent", c.Request.UserAgent(),
			"status", status,
			"reqSize", c.Request.ContentLength,
			"respSize", c.Writer.Size(),
			"latency", latency.Milliseconds(),
		}

		switch {
		case status >= 500:
			slog.Error("request", fields...)
		case status >= 400:
			slog.Warn("request", fields...)
		default:
			slog.Info("request", fields...)
		}
	}
}
