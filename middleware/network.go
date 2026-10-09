package middleware

import (
	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
)

// 请求级网络选择（spec: 2026-10-09-request-scoped-network）：
// 客户端可携带 X-Milon-Network 头指定本次请求的网络；缺省走服务端默认
// 网络（启动配置或 network_switch 所设），与历史行为一致。

const (
	// NetworkHeader 是请求级网络选择头名。
	NetworkHeader = "X-Milon-Network"
	// CtxClientKey / CtxNetworkNameKey 是 context 注入键。
	CtxClientKey      = "milon_network_client"
	CtxNetworkNameKey = "milon_network_name"
)

// ResolveNetwork 读网络头并按名解析 client 注入 context；未知网络名 400。
// 挂在整个 /api 组；/api/network/* 三端点不读 client，统一挂载无副作用。
func ResolveNetwork(nm *client.NetworkManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		mc, cfg, err := nm.ClientFor(c.GetHeader(NetworkHeader))
		if err != nil {
			c.AbortWithStatusJSON(400, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
			return
		}
		c.Set(CtxClientKey, mc)
		c.Set(CtxNetworkNameKey, cfg.Name)
		c.Next()
	}
}

// ClientFrom 取当前请求解析出的网络 client；未经过 ResolveNetwork 时返回 nil。
func ClientFrom(c *gin.Context) *milon.Client {
	if v, ok := c.Get(CtxClientKey); ok {
		if mc, ok := v.(*milon.Client); ok {
			return mc
		}
	}
	return nil
}

// NetworkNameFrom 取解析后的网络名（空头时即默认网络名）。
func NetworkNameFrom(c *gin.Context) string {
	if v, ok := c.Get(CtxNetworkNameKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
