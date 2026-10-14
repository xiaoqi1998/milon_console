package middleware

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// 请求级配置账户（spec: 2026-10-14-mcp-account-config）：
// 客户端在 MCP 配置 headers 里携带 X-Milon-Accounts（base64(JSON 数组)），
// 服务端按请求过境解析——零持久化、零跨请求共享，多用户天然隔离。
// 自托管单用户可用 MILON_ACCOUNTS 环境变量（raw JSON）兜底；header 优先。

const (
	// AccountsHeader 是客户端配置账户头名。
	AccountsHeader = "X-Milon-Accounts"
	// AccountsEnv 是自托管环境变量名（raw JSON，非 base64）。
	AccountsEnv = "MILON_ACCOUNTS"
	// CtxAccountsKey / CtxAccountsSourceKey 是 context 注入键。
	CtxAccountsKey       = "milon_configured_accounts"
	CtxAccountsSourceKey = "milon_configured_accounts_source"
)

// ResolveAccounts 解析账户头（或 env 兜底）注入 context；校验失败 400 快速失败
// （错误定位到 X-Milon-Accounts[i] 条目）。挂在 /api 组，与 ResolveNetwork 并列。
func ResolveAccounts() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader(AccountsHeader)
		source := "header"
		sourceName := AccountsHeader // 错误文案前缀：指向真实来源，防 env 配错时误导排障
		if raw == "" {
			raw = os.Getenv(AccountsEnv)
			if raw == "" {
				c.Next() // 无配置账户：现有行为完全不变
				return
			}
			source = "env"
			sourceName = AccountsEnv
		} else {
			decoded, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest,
					types.ErrorResponse(types.ERR_INVALID_PARAMETER, "X-Milon-Accounts 不是合法 base64（应为 base64(JSON 数组)）: "+err.Error(), nil))
				return
			}
			raw = string(decoded)
		}
		accounts, err := types.ParseConfiguredAccounts(sourceName, []byte(raw))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
			return
		}
		c.Set(CtxAccountsKey, accounts)
		c.Set(CtxAccountsSourceKey, source)
		c.Next()
	}
}

// AccountsFrom 取当前请求的配置账户集；未携带时返回 nil。
func AccountsFrom(c *gin.Context) []types.ConfiguredAccount {
	if v, ok := c.Get(CtxAccountsKey); ok {
		if a, ok := v.([]types.ConfiguredAccount); ok {
			return a
		}
	}
	return nil
}

// AccountsSourceFrom 取账户来源（"header"/"env"/""）。
func AccountsSourceFrom(c *gin.Context) string {
	if v, ok := c.Get(CtxAccountsSourceKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// redirectLogOutput 把默认 slog 输出重定向到 buf，返回恢复函数。
func redirectLogOutput(buf *strings.Builder) func() {
	old := slog.Default()
	logger := slog.New(slog.NewTextHandler(buf, nil))
	slog.SetDefault(logger)
	return func() { slog.SetDefault(old) }
}
