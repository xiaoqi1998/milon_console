package handler

import (
	"encoding/json"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// configuredAccountHint 追加到既有「私钥必填」错误之后，引导用户走配置账户。
const configuredAccountHint = "；该地址未配置为操作账户——可在 MCP 客户端 mcp.json 的 milon.headers 配 X-Milon-Accounts（base64(JSON数组)，fndsa512 须带 publicKey）后免传私钥，或先调 account_config_list 核对已配置账户"

// accountRole 是一个签名角色在请求结构体里的字段锚点（指针指向请求字段）。
// Priv/Pub/SigMode 可为 nil（如 simulate 类请求无私钥字段，仅注入签名模式）。
type accountRole struct {
	Addr    *string
	Priv    *string
	Pub     *string
	KeyType *string
	SigMode *json.RawMessage
}

// lookupConfiguredAccount 按地址查本请求携带的配置账户集。
func lookupConfiguredAccount(c *gin.Context, addrStr string) *types.ConfiguredAccount {
	if addrStr == "" {
		return nil
	}
	accounts := middleware.AccountsFrom(c)
	for i := range accounts {
		if accounts[i].Address == addrStr {
			return &accounts[i]
		}
	}
	return nil
}

// fillFromConfiguredAccounts 「地址即引用」回填：私钥字段为空且地址命中配置账户
// 时注入私钥/公钥/曲线，签名模式为空时注入对应 pubkey 模式（fndsa512 场景关键，
// 其公钥无法从私钥派生）。显式私钥优先；未命中不动——由既有校验报错并附 hint。
func fillFromConfiguredAccounts(c *gin.Context, roles ...accountRole) {
	for _, r := range roles {
		if r.Priv != nil && *r.Priv != "" {
			continue // 显式私钥优先（用户单次意图覆盖常驻配置）
		}
		if r.Addr == nil || *r.Addr == "" {
			continue
		}
		acct := lookupConfiguredAccount(c, *r.Addr)
		if acct == nil {
			continue // 未命中：不回填，留给既有错误路径（文案已升级附 hint）
		}
		if r.Priv != nil {
			*r.Priv = acct.PrivateKey
		}
		if r.KeyType != nil && *r.KeyType == "" {
			*r.KeyType = acct.KeyType
		}
		if r.Pub != nil && *r.Pub == "" {
			*r.Pub = acct.PublicKey
		}
		if r.SigMode != nil && len(*r.SigMode) == 0 {
			m, err := json.Marshal(map[string]string{"type": "pubkey", "publicKey": acct.PublicKey})
			if err == nil {
				*r.SigMode = json.RawMessage(m)
			}
		}
	}
}
