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
}

// idlInstructionMeta 描述一个 IDL 方法（指令）的元数据。
type idlInstructionMeta struct {
	Name          string        `json:"name"`
	Kind          string        `json:"kind"` // "entry" | "view"
	Handler       string        `json:"handler"`
	Discriminator uint16        `json:"discriminator"`
	Args          []idlArgMeta  `json:"args"`
	Returns       *idlReturnMeta `json:"returns,omitempty"` // view 必有
	Sponsor       bool          `json:"sponsor,omitempty"`  // entry 可有
}

// idlArgMeta 描述一个方法参数。
type idlArgMeta struct {
	Name string `json:"name"`
	Type string `json:"type"` // 原始 IDL 类型字符串，如 "vec<PublicKey>"
	Role string `json:"role"` // "input" | "signer" | "any_signer"
}

// idlReturnMeta 描述 view 方法的返回值类型。
type idlReturnMeta struct {
	Type string `json:"type"`
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
				Name: a.Name,
				Type: a.Type,
				Role: a.Role,
			})
		}

		meta := idlInstructionMeta{
			Name:          ix.Name,
			Kind:          ix.Kind,
			Handler:       ix.Handler,
			Discriminator: ix.Discriminator,
			Args:          args,
			Sponsor:       ix.Sponsor,
		}

		if ix.Returns != nil {
			meta.Returns = &idlReturnMeta{Type: ix.Returns.Type}
		}

		instructions = append(instructions, meta)
	}

	return idlAppMeta{
		AppID:        idl.Metadata.AppID,
		Name:         name,
		Description:  idl.Metadata.Description,
		Instructions: instructions,
	}
}
