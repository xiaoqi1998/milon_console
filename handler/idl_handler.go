package handler

import (
	"net/http"
	"sort"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// IDLHandler 暴露 IDL 元数据端点，供前端动态发现 app/方法/参数 schema。
type IDLHandler struct {
	nm *client.NetworkManager
}

// NewIDLHandler 创建绑定到 NetworkManager 的 IDLHandler。
func NewIDLHandler(nm *client.NetworkManager) *IDLHandler {
	return &IDLHandler{nm: nm}
}

// idlAppMeta 描述一个 IDL app 的元数据。
type idlAppMeta struct {
	AppID        uint8                `json:"appId"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Instructions []idlInstructionMeta `json:"instructions"`
	Types        []idlTypeMeta        `json:"types,omitempty"`
	Constants    []idlConstantMeta    `json:"constants,omitempty"`
	Errors       []idlErrorMeta       `json:"errors,omitempty"`
}

// idlInstructionMeta 描述一个 IDL 方法（指令）的元数据。
type idlInstructionMeta struct {
	Name          string                          `json:"name"`
	Kind          string                          `json:"kind"` // "entry" | "view"
	Handler       string                          `json:"handler"`
	Discriminator uint16                          `json:"discriminator"`
	Description   string                          `json:"description"` // 方法中文说明
	Args          []idlArgMeta                    `json:"args"`
	Returns       *idlReturnMeta                  `json:"returns,omitempty"` // view 必有
	Sponsor       bool                            `json:"sponsor,omitempty"`  // entry 可有
	SignerLookups map[string]idlSignerLookupMeta  `json:"signerLookups,omitempty"` // entry 可有，签名者角色 -> 参数映射
}

// idlArgMeta 描述一个方法参数。
type idlArgMeta struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // 原始 IDL 类型字符串，如 "vec<PublicKey>"
	Role        string `json:"role"` // "input" | "signer" | "any_signer"
	Description string `json:"description"` // 参数中文说明
}

// idlReturnMeta 描述 view 方法的返回值类型。
type idlReturnMeta struct {
	Type string `json:"type"`
}

// idlTypeMeta 描述一个自定义类型（struct/enum/builtin/tuple/unit）。
type idlTypeMeta struct {
	Name     string               `json:"name"`
	Kind     string               `json:"kind"` // struct | enum | builtin | tuple | unit
	Fields   []idlStructFieldMeta `json:"fields,omitempty"`
	Variants []idlEnumVariantMeta `json:"variants,omitempty"`
}

// idlStructFieldMeta 描述 struct 的一个字段。
type idlStructFieldMeta struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// idlEnumVariantMeta 描述 enum 的一个变体。
type idlEnumVariantMeta struct {
	Name   string               `json:"name"`
	Kind   string               `json:"kind"`
	Fields []idlStructFieldMeta `json:"fields,omitempty"`
}

// idlConstantMeta 描述 IDL 中的一个常量。
type idlConstantMeta struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value,omitempty"`
}

// idlErrorMeta 描述 IDL 中定义的一个错误。
type idlErrorMeta struct {
	Code    int    `json:"code"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// idlSignerLookupMeta 描述单个签名者的查找配置。
type idlSignerLookupMeta struct {
	Arg  string `json:"arg"`
	Type string `json:"type"`
	Res  uint8  `json:"res"`
}

// GetIDLMetadata handles GET /api/idl/metadata
// 返回所有已加载 IDL app 的元数据（app 列表 + 每个方法名/类型/参数/角色）。
func (h *IDLHandler) GetIDLMetadata(c *gin.Context) {
	mc, _ := h.nm.GetCurrent()
	if mc == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "no active network client", nil))
		return
	}

	allPd := mc.GetAllPd()

	apps := make([]idlAppMeta, 0, len(allPd))
	for name, pd := range allPd {
		apps = append(apps, buildAppMeta(name, pd))
	}

	// 按 app_id 升序排序，保证返回顺序稳定
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].AppID < apps[j].AppID
	})

	c.JSON(http.StatusOK, types.SuccessResponse(apps, "ok"))
}

// buildAppMeta 从 Provider 构建单个 app 的元数据。
func buildAppMeta(name string, pd *provider.Provider) idlAppMeta {
	idl := pd.IDL
	instructions := make([]idlInstructionMeta, 0, len(idl.Instructions))

	for _, ix := range idl.Instructions {
		args := make([]idlArgMeta, 0, len(ix.Args))
		for _, a := range ix.Args {
			args = append(args, idlArgMeta{
				Name:        a.Name,
				Type:        a.Type,
				Role:        a.Role,
				Description: describeArg(a.Name, a.Role),
			})
		}

		ixDesc := describeInstruction(name, ix.Name, ix.Handler, ix.Kind)

		meta := idlInstructionMeta{
			Name:          ix.Name,
			Kind:          ix.Kind,
			Handler:       ix.Handler,
			Discriminator: ix.Discriminator,
			Description:   ixDesc,
			Args:          args,
			Sponsor:       ix.Sponsor,
		}

		if ix.Returns.Type != "" {
			meta.Returns = &idlReturnMeta{Type: ix.Returns.Type}
		}

		if len(ix.SignerLookups) > 0 {
			lookups := make(map[string]idlSignerLookupMeta, len(ix.SignerLookups))
			for role, sl := range ix.SignerLookups {
				lookups[role] = idlSignerLookupMeta{
					Arg:  sl.Path.Arg,
					Type: sl.Path.Type,
					Res:  sl.Res,
				}
			}
			meta.SignerLookups = lookups
		}

		instructions = append(instructions, meta)
	}

	// 构建自定义类型、常量、错误元数据
	types := make([]idlTypeMeta, 0, len(idl.Types))
	for _, t := range idl.Types {
		tm := idlTypeMeta{Name: t.Name, Kind: t.Kind}
		if len(t.Fields) > 0 {
			tm.Fields = make([]idlStructFieldMeta, 0, len(t.Fields))
			for _, f := range t.Fields {
				tm.Fields = append(tm.Fields, idlStructFieldMeta{Name: f.Name, Type: f.Type})
			}
		}
		if len(t.Variants) > 0 {
			tm.Variants = make([]idlEnumVariantMeta, 0, len(t.Variants))
			for _, v := range t.Variants {
				vm := idlEnumVariantMeta{Name: v.Name, Kind: v.Kind}
				if len(v.Fields) > 0 {
					vm.Fields = make([]idlStructFieldMeta, 0, len(v.Fields))
					for _, f := range v.Fields {
						vm.Fields = append(vm.Fields, idlStructFieldMeta{Name: f.Name, Type: f.Type})
					}
				}
				tm.Variants = append(tm.Variants, vm)
			}
		}
		types = append(types, tm)
	}

	constants := make([]idlConstantMeta, 0, len(idl.Constants))
	for _, c := range idl.Constants {
		constants = append(constants, idlConstantMeta{Name: c.Name, Type: c.Type, Value: c.Value})
	}

	errors := make([]idlErrorMeta, 0, len(idl.Errors))
	for _, e := range idl.Errors {
		errors = append(errors, idlErrorMeta{Code: e.Code, Name: e.Name, Message: e.Message})
	}

	return idlAppMeta{
		AppID:        idl.Metadata.AppID,
		Name:         name,
		Description:  idl.Metadata.Description,
		Instructions: instructions,
		Types:        types,
		Constants:    constants,
		Errors:       errors,
	}
}

// describeInstruction 根据 app/方法名/handler/kind 推断方法的中文说明。
func describeInstruction(app, name, handler, kind string) string {
	h := handler
	verb := ""
	switch {
	case hasPrefix(h, "create"):
		verb = "创建"
	case hasPrefix(h, "mint"):
		verb = "铸造/增发"
	case hasPrefix(h, "burn"):
		verb = "销毁"
	case hasPrefix(h, "transfer"):
		verb = "转账"
	case hasPrefix(h, "freeze"):
		verb = "冻结"
	case hasPrefix(h, "unfreeze"):
		verb = "解冻"
	case hasPrefix(h, "approve"):
		verb = "授权"
	case hasPrefix(h, "revoke"):
		verb = "撤销授权"
	case hasPrefix(h, "set"):
		verb = "设置"
	case hasPrefix(h, "update"):
		verb = "更新"
	case hasPrefix(h, "add"):
		verb = "添加"
	case hasPrefix(h, "remove"):
		verb = "移除"
	case hasPrefix(h, "open"):
		verb = "开启/创建"
	case hasPrefix(h, "close"):
		verb = "关闭"
	case hasPrefix(h, "init"):
		verb = "初始化"
	case hasPrefix(h, "pause"):
		verb = "暂停"
	case hasPrefix(h, "unpause"):
		verb = "恢复"
	case hasPrefix(h, "upgrade"):
		verb = "升级"
	case hasPrefix(h, "claim"):
		verb = "领取"
	case hasPrefix(h, "deposit"):
		verb = "存入"
	case hasPrefix(h, "withdraw"):
		verb = "取出"
	case hasPrefix(h, "batch"):
		verb = "批量处理"
	case hasPrefixAny(h, "get_", "query_", "list_", "fetch_", "of", "balance", "total"):
		verb = "查询"
	case hasPrefix(h, "disclose"):
		verb = "披露"
	case hasPrefix(h, "register"):
		verb = "注册"
	case hasPrefix(h, "submit"):
		verb = "提交"
	case hasPrefix(h, "vote"):
		verb = "投票"
	case hasPrefix(h, "delegate"):
		verb = "委托"
	case hasPrefix(h, "lock"):
		verb = "锁定"
	case hasPrefix(h, "unlock"):
		verb = "解锁"
	case hasPrefix(h, "echo"):
		verb = "回显"
	}

	obj := appNameCN(app) + "对象"
	switch app {
	case "system":
		obj = "系统"
	case "account":
		obj = "账户"
	case "token":
		obj = "代币"
	case "staking":
		obj = "质押"
	case "identity":
		obj = "身份（DID）"
	case "nft":
		obj = "NFT"
	case "randomness":
		obj = "随机数（VRF）信标"
	case "demo":
		obj = "示例/demo"
	}

	if verb != "" {
		if kind == "view" {
			return "（只读查询）" + verb + obj + "相关信息。"
		}
		return verb + obj + "（handler: " + h + "）。"
	}
	return "调用 " + obj + " 的 " + name + " 方法（handler: " + h + "）。"
}

// describeArg 根据参数名/角色推断中文含义。
func describeArg(name, role string) string {
	switch role {
	case "signer":
		return "签名者（发起方账户，需签名授权）"
	case "any_signer":
		return "任意签名者"
	}
	switch name {
	case "token":
		return "代币地址"
	case "account", "owner", "holder", "subject", "to", "from", "recipient", "payer", "operator", "claimer", "spender", "issuer", "validator", "delegator", "collection", "pool", "dex", "signer", "mint", "addr", "address":
		return "地址"
	case "amount", "value", "balance", "score", "cap", "threshold", "royalty", "max_supply", "total":
		return "数量（u64）"
	case "amounts":
		return "数量列表（vec<u64>）"
	case "metadata":
		return "元数据"
	case "key", "keys":
		return "密钥"
	case "epoch":
		return "纪元（epoch）编号"
	case "nonce":
		return "随机数 nonce"
	case "seq", "sequence":
		return "序列号"
	case "name":
		return "名称"
	case "id":
		return "编号/ID"
	case "label":
		return "标签"
	case "doc", "input":
		return "内容/输入数据"
	case "hash", "credential_hash", "genesis_hash", "parent_block_hash", "seed", "entropy":
		return "哈希/种子值"
	case "uri", "avatar_uri", "icon_url":
		return "URI 地址"
	case "data", "payload", "proof", "signature", "issuer_signature":
		return "数据/签名"
	case "attributes", "properties":
		return "属性/特性"
	case "config", "initial_config":
		return "配置"
	}
	return ""
}

func appNameCN(app string) string {
	m := map[string]string{
		"system":     "系统",
		"account":    "账户",
		"token":      "代币",
		"staking":    "质押",
		"identity":   "身份",
		"nft":        "NFT",
		"randomness": "随机数",
		"demo":       "示例",
	}
	if v, ok := m[app]; ok {
		return v
	}
	return app
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasPrefixAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if hasPrefix(s, p) {
			return true
		}
	}
	return false
}
