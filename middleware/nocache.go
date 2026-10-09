package middleware

import "github.com/gin-gonic/gin"

// NoCache 让静态资源每次都向服务端校验新鲜度（未变更时 304）。
// 调试台无 CDN 层，禁用启发式缓存可避免"部署后浏览器跑旧 JS"的跨版本错配
// （新 index.html 引用的元素在旧 app.js 里无人填充，页面静默失能）。
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Next()
	}
}
