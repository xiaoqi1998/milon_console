// Package handler 内：配置账户脱敏回显。
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// AccountsConfigHandler 回显当前请求（即当前用户客户端配置）携带的操作账户。
type AccountsConfigHandler struct{}

// NewAccountsConfigHandler 创建 handler。
func NewAccountsConfigHandler() *AccountsConfigHandler { return &AccountsConfigHandler{} }

// List handles GET /api/accounts-config（MCP 工具 account_config_list 同源）。
// 不变量：永不返回 privateKey 与 publicKey 全文——公钥只回 sha256 指纹前 8 hex。
func (h *AccountsConfigHandler) List(c *gin.Context) {
	accounts := middleware.AccountsFrom(c)
	source := middleware.AccountsSourceFrom(c)
	rows := make([]gin.H, 0, len(accounts))
	for _, a := range accounts {
		sum := sha256.Sum256([]byte(a.PublicKey))
		rows = append(rows, gin.H{
			"name": a.Name, "address": a.Address, "keyType": a.KeyType,
			"publicKeyFingerprint": hex.EncodeToString(sum[:4]),
			"source":               source,
		})
	}
	msg := "ok"
	if len(accounts) == 0 {
		msg = "本请求未携带配置账户：在 MCP 客户端 mcp.json 的 milon.headers 加 X-Milon-Accounts（base64(JSON数组)，元素 address/privateKey/keyType，fndsa512 须带 publicKey），自托管可设 MILON_ACCOUNTS"
	}
	c.JSON(http.StatusOK, types.SuccessResponse(rows, msg))
}
