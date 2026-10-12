package middleware

import "github.com/gin-gonic/gin"

// contentSecurityPolicy 按调试台实际用到的资源来源收紧：
//   - script-src 'self'：index.html 只有外链脚本，app.js 无 eval/new Function
//   - style-src 'unsafe-inline'：el()/svgIcon 会设置内联 style 属性
//   - img-src data:：favicon 为 data URI，页面无 <img> 外链渲染
//   - frame-ancestors 'none'：阻断 iframe 点击劫持（执行交易的控制台不可被嵌套），
//     与 X-Frame-Options: DENY 双保险（XFO 兼容旧浏览器，CSP 为标准做法）
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; " +
	"font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; " +
	"frame-ancestors 'none'"

// SecurityHeaders 给所有响应（页面/静态资源/API）附加浏览器安全响应头。
// HSTS 由前置 openresty 层负责，此处不重复设置。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", contentSecurityPolicy)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}
