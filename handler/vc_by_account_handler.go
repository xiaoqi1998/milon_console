package handler

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== VC 按账户查询(GET /api/tool/vc-by-account) ====================
// 总览/账号本账户列表展开行用:查 identity.DisclosedVcs 返回该地址持有(已披露)
// 的证书摘要。"发布的证书"链端无按 issuer 枚举视图,由前端对各账户的 held
// 结果聚合(issuer==该地址)。未披露上链的证书无法枚举,属链端硬限制。

// VcByAccountRow 是一条证书摘要。
type VcByAccountRow struct {
	Schema         string  `json:"schema"`
	Issuer         string  `json:"issuer,omitempty"`
	CredentialHash string  `json:"credentialHash,omitempty"`
	ValidUntilMs   *int64  `json:"validUntilMs,omitempty"`
	Status         string  `json:"status,omitempty"`
	IsValid        bool    `json:"isValid"`
}

// VcByAccountHandler 查询处理器(viewFn 可注入以便离线单测)。
type VcByAccountHandler struct {
	viewFn func(addr string) (any, error)
}

// NewVcByAccountHandler 创建走真实链上视图的处理器。
func NewVcByAccountHandler() *VcByAccountHandler {
	return &VcByAccountHandler{}
}

// vcByAccountParseRows 解析 DisclosedVcs 视图返回(行缺字段容错,展示用途)。
func vcByAccountParseRows(value any) ([]VcByAccountRow, error) {
	rowsAny, ok := value.([]any)
	if !ok {
		if value == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("DisclosedVcs 返回形状异常: %T", value)
	}
	rows := make([]VcByAccountRow, 0, len(rowsAny))
	for _, r := range rowsAny {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		row := VcByAccountRow{}
		row.Schema, _ = m["credential_schema"].(string)
		row.Issuer = vcByAccountAddrString(m["issuer"])
		if hash, ok := m["credential_hash"]; ok && hash != nil {
			if hb, ok := hash.([]byte); ok {
				row.CredentialHash = hex.EncodeToString(hb)
			} else if h32, ok := hash.([32]byte); ok {
				row.CredentialHash = hex.EncodeToString(h32[:])
			} else if ha, ok := hash.([]any); ok {
				raw := make([]byte, len(ha))
				for i, x := range ha {
					if f, ok := x.(float64); ok {
						raw[i] = byte(f)
					}
				}
				row.CredentialHash = hex.EncodeToString(raw)
			} else {
				row.CredentialHash = fmt.Sprintf("%v", hash)
			}
		}
		if vu, ok := m["valid_until_ms"].(float64); ok {
			v := int64(vu)
			row.ValidUntilMs = &v
		}
		switch st := m["status"].(type) {
		case string:
			row.Status = st
		case map[string]any:
			row.Status, _ = st["variant"].(string)
		}
		row.IsValid, _ = m["is_valid"].(bool)
		rows = append(rows, row)
	}
	return rows, nil
}

// vcByAccountAddrString 宽容提取地址字段:字符串直接用;其他形状(解码差异)
// 兜底 %v,展示用途不因解码路径不同而丢字段。
func vcByAccountAddrString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// fetchDisclosedVcs 查链(生产路径走 flowCallView;测试注入 viewFn)。
func (h *VcByAccountHandler) fetchDisclosedVcs(mc *milon.Client, addr string) (any, error) {
	if h.viewFn != nil {
		return h.viewFn(addr)
	}
	return flowCallView(mc, vcFlowIdentityApp, "DisclosedVcs", provider.Args{
		"subject": addr,
		"offset":  0,
		"limit":   100,
	})
}

// List handles GET /api/tool/vc-by-account?address=base58。
func (h *VcByAccountHandler) List(c *gin.Context) {
	addr := strings.TrimSpace(c.Query("address"))
	if addr == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "address 必填(base58)", nil))
		return
	}
	if _, err := types.ParseAddress(addr); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid address: "+err.Error(), nil))
		return
	}
	value, err := h.fetchDisclosedVcs(middleware.ClientFrom(c), addr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "查询 DisclosedVcs 失败: "+err.Error(), nil))
		return
	}
	rows, err := vcByAccountParseRows(value)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"address": addr, "held": rows},
		"held=该地址持有且已披露的证书;发布方视角由前端聚合(链端无按 issuer 枚举视图)"))
}
