package middleware

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
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

const (
	// bodyLogMaxBytes 单个请求/响应体最多缓存多少字节用于日志（防内存被大 body 打爆）。
	bodyLogMaxBytes = 16 * 1024
	// bodyLogMaxChars 单条日志里 body 最多输出的字符数，超出截断。
	bodyLogMaxChars = 4096
)

// sensitiveKeyRegex 匹配 JSON 中需要脱敏的 key:value，如 "privateKey":"xxx"。
// 覆盖 privateKey / private_key / private-key / PrivateKey / mnemonic / secret / password 等字段。
// 用 (?i) 忽略大小写以兼容驼峰/全大写变体；字段名前缀限定避免误伤 publicKey。
var sensitiveKeyRegex = regexp.MustCompile(`(?i)("(?:private[_-]?key|mnemonic|secret|password)"\s*:\s*")[^"]*(")`)

// responseBodyWriter 包装 gin.ResponseWriter，捕获实际写入的响应体（限量）。
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseBodyWriter) Write(data []byte) (int, error) {
	if w.body.Len() < bodyLogMaxBytes {
		w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseBodyWriter) WriteString(s string) (int, error) {
	if w.body.Len() < bodyLogMaxBytes {
		w.body.WriteString(s)
	}
	return w.ResponseWriter.WriteString(s)
}

// shouldLogBody 判断是否需要记录请求/响应体：仅对携带 body 的写类方法，
// 并排除静态资源与首页等无需关注的文件路径。
func shouldLogBody(method, path string) bool {
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
		return false
	}
	if path == "/" || strings.HasPrefix(path, "/static") {
		return false
	}
	return true
}

// redactSensitive 将 JSON body 中命中的敏感字段值替换为 ***。
func redactSensitive(s string) string {
	return sensitiveKeyRegex.ReplaceAllString(s, `${1}***${2}`)
}

// truncateBody 限制 body 输出长度，超出部分截断并标注。
func truncateBody(s string) string {
	if len(s) <= bodyLogMaxChars {
		return s
	}
	return s[:bodyLogMaxChars] + "...(truncated)"
}

// LoggerMiddleware logs each request with structured slog output.
// 当 enableBodyLog 为 true 时，额外记录请求体(reqBody)与响应体(respBody)，
// 敏感字段会脱敏，超过大小/长度限制的 body 会被截断。
func LoggerMiddleware(enableBodyLog bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		rid := generateRequestID(c)
		c.Set("requestId", rid)
		c.Header("X-Request-Id", rid)

		// 捕获请求体：读入内存后需恢复 Body，保证后续 handler 仍可正常读取。
		reqBody := ""
		if enableBodyLog && shouldLogBody(method, path) && c.Request.Body != nil {
			bodyBytes, _ := io.ReadAll(io.LimitReader(c.Request.Body, bodyLogMaxBytes))
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			reqBody = truncateBody(redactSensitive(string(bodyBytes)))
		}

		// 捕获响应体：用包装后的 writer 替换原始 writer，不改变下游行为。
		var respWriter *responseBodyWriter
		if enableBodyLog && shouldLogBody(method, path) {
			respWriter = &responseBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
			c.Writer = respWriter
		}

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
		if enableBodyLog && shouldLogBody(method, path) {
			if reqBody != "" {
				fields = append(fields, "reqBody", reqBody)
			}
			if respWriter != nil {
				respBody := truncateBody(redactSensitive(respWriter.body.String()))
				if respBody != "" {
					fields = append(fields, "respBody", respBody)
				}
			}
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
