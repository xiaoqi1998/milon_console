package handler

import (
	"net/http"
	"strconv"
	"strings"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// ErrorLookupHandler 暴露错误码查询（GET /api/errors/:query），聚合 API 层
// 码表与 IDL 各 app 的链上错误码，供 AI 碰到错误时一步翻译。
type ErrorLookupHandler struct{}

// NewErrorLookupHandler creates an ErrorLookupHandler.
func NewErrorLookupHandler() *ErrorLookupHandler {
	return &ErrorLookupHandler{}
}

// apiErrorCodeTable 是服务层（非链上）错误码表，与前端 ERROR_CODES 同源。
type apiErrorCode struct {
	Code     int    `json:"code"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Solution string `json:"solution"`
}

var apiErrorTable = []apiErrorCode{
	{0, "SUCCESS", "请求成功", "操作已成功完成，无需额外处理。"},
	{400, "INVALID_ARGUMENT", "请求参数无效或格式错误", "请检查请求参数是否完整、格式是否正确。"},
	{401, "UNAUTHENTICATED", "未认证，缺少有效的 API 密钥", "请在请求头中添加有效的 Authorization 字段。"},
	{403, "PERMISSION_DENIED", "权限不足", "确认账户权限，或联系管理员。"},
	{404, "NOT_FOUND", "请求的资源不存在", "检查 URL 和参数是否正确。"},
	{409, "CONFLICT", "资源冲突（例如账户已存在）", "检查数据状态后重试。"},
	{429, "TOO_MANY_REQUESTS", "请求频率超限", "稍后重试。"},
	{500, "INTERNAL_ERROR", "服务器内部错误", "稍后重试；持续存在请联系技术支持。"},
	{503, "UNAVAILABLE", "服务暂时不可用", "稍后重试。"},
	{-32000, "INVALID_PARAMS", "RPC 调用参数无效", "检查 RPC 方法参数类型与数量。"},
	{-32601, "METHOD_NOT_FOUND", "RPC 方法不存在", "核对方法名。"},
	{-32602, "INVALID_PARAMS_RPC", "RPC 参数格式错误", "检查参数结构。"},
}

// Lookup handles GET /api/errors/:query
// query 为十进制错误码或名字子串（大小写不敏感）；命中返回
// [{source, code, name, message, solution?}]，source 为 "api" 或 "app:<名>"。
func (h *ErrorLookupHandler) Lookup(c *gin.Context) {
	query := strings.TrimSpace(c.Param("query"))
	if query == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "query is required", nil))
		return
	}

	type hit struct {
		Source   string `json:"source"`
		Code     int    `json:"code"`
		Name     string `json:"name"`
		Message  string `json:"message"`
		Solution string `json:"solution,omitempty"`
	}
	var hits []hit

	queryNum, numErr := strconv.Atoi(query)
	needle := strings.ToLower(query)

	for _, e := range apiErrorTable {
		match := (numErr == nil && e.Code == queryNum) ||
			strings.Contains(strings.ToLower(e.Name), needle)
		if match {
			hits = append(hits, hit{Source: "api", Code: e.Code, Name: e.Name, Message: e.Desc, Solution: e.Solution})
		}
	}

	if mc := middleware.ClientFrom(c); mc != nil {
		for appName, pd := range mc.GetAllPd() {
			for _, e := range pd.IDL.Errors {
				match := (numErr == nil && int(e.Code) == queryNum) ||
					strings.Contains(strings.ToLower(e.Name), needle)
				if match {
					hits = append(hits, hit{Source: "app:" + appName, Code: int(e.Code), Name: e.Name, Message: e.Message})
				}
			}
		}
	}
	if hits == nil {
		hits = []hit{}
	}

	c.JSON(http.StatusOK, types.SuccessResponse(hits, "ok"))
}
