package handler

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== DID 工具端点(/api/tool/did/*) ====================
//
// 把「建 DID」补成一整套:创建 + 别名(identity.CreateWithAlias/SetAlias) +
// 服务 URI(AddService/UpdateService/RemoveService) + 头像(SetAvatarUri) +
// 密钥管理(AddKey/UpdateKey/RemoveKey) + 停用(Deactivate) + 文档查询(Document/NameBinding)。
//
// 聚合 create 幂等:链上已有 DID 则跳过创建,按请求补齐差异(缺别名 SetAlias /
// 缺服务 AddService / 头像不同 SetAvatarUri),可直接重跑。
// 别名全局唯一,格式为「alias-数字」(链端 1035-1039 校验);suffix 未指定时由
// 服务端代填(didAliasSuffixMin/Max,链端报 1038 时按实测调整范围)。
// 复用 vc_flow_handler 的身份解析与交易设施(resolveVcFlowParty/vcFlowBuildTx/...)。

const (
	didAliasSuffixMin = 1000  // 服务端代填 suffix 的下界(含)
	didAliasSuffixMax = 10000 // 服务端代填 suffix 的上界(不含)

	didAliasMaxRetries = 3 // 服务端代填 suffix 撞名(1028)时的最大重试次数

	didDefaultKeyLabel = "primary"
)

// didServiceSpec 一条服务端点输入(label + 绝对 URI)。
type didServiceSpec struct {
	Label    string `json:"label"`
	Endpoint string `json:"serviceEndpoint"`
}

// didCreateRequest 是 POST /api/tool/did/create 的请求体。
type didCreateRequest struct {
	PrivateKey  string           `json:"privateKey"`  // 必填
	PublicKey   string           `json:"publicKey"`   // FN-DSA-512 私钥时必填
	Address     string           `json:"address"`     // 必填;32字节私钥在不同曲线下派生不同地址,用于锁定正确公钥
	SubjectType string           `json:"subjectType"` // Personal(缺省)/Organization
	Alias       string           `json:"alias"`       // 可选;非空时创建即绑定别名(CreateWithAlias)
	Suffix      *uint32          `json:"suffix"`      // 可选;覆盖服务端代填的数字后缀
	Services    []didServiceSpec `json:"services"`    // 可选;创建时一并登记的服务端点
	AvatarURI   string           `json:"avatarUri"`   // 可选;创建时缺省用占位 URI,补齐时空值绝不覆盖链上
}

// didMutateBase 是细粒度管理端点的公共请求字段。
type didMutateBase struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"` // FN-DSA-512 私钥时必填
	Address    string `json:"address"`   // 必填;曲线消歧
}

// didBase 让端点请求结构体(匿名嵌入 didMutateBase)暴露公共字段,
// 供 bindMutate 做整包绑定后统一校验。
func (b *didMutateBase) didBase() *didMutateBase { return b }

// didMutateProvider 是 bindMutate 接受的请求类型约束。
type didMutateProvider interface {
	didBase() *didMutateBase
}

// vcFlowDidOptions 是 vc-flow 透传给某一方 DID 创建的可选项。
type vcFlowDidOptions struct {
	Alias     string           `json:"alias"`
	Suffix    *uint32          `json:"suffix"`
	Services  []didServiceSpec `json:"services"`
	AvatarURI string           `json:"avatarUri"`
	AutoAlias *bool            `json:"autoAlias"` // 缺省 true:未显式给别名时由服务端按角色+地址自动生成
}

// autoDidAlias 生成 vc-flow 默认别名主体:角色前缀 + 地址 base58 前 8 位,
// 纯字母数字(避开「alias-数字」格式的分隔歧义);数字后缀仍由 didBindAlias 代填。
func autoDidAlias(rolePrefix string, addr crypto.Address) string {
	base := addr.ToBase58()
	if len(base) > 8 {
		base = base[:8]
	}
	return rolePrefix + base
}

// resolveVcFlowDidAlias 决定 vc-flow 某一方 DID 的别名:(别名主体, 是否绑定)。
// 显式 alias > 自动生成(缺省);autoAlias=false 且未给别名时不绑定。
func resolveVcFlowDidAlias(opts *vcFlowDidOptions, rolePrefix string, addr crypto.Address) (string, bool) {
	if opts != nil && strings.TrimSpace(opts.Alias) != "" {
		return strings.TrimSpace(opts.Alias), true
	}
	if opts != nil && opts.AutoAlias != nil && !*opts.AutoAlias {
		return "", false
	}
	return autoDidAlias(rolePrefix, addr), true
}

// didBackfillPlan 链上已有 DID 时,按请求计算出的补齐动作(纯函数结果)。
type didBackfillPlan struct {
	NeedCreate  bool
	SetAlias    bool
	AddServices []didServiceSpec
	SetAvatar   bool
}

// didEnsureResult 一次「确保 DID 就绪」的分步结果。
type didEnsureResult struct {
	Create   flowStep
	Alias    *flowStep
	Services []didServiceStep
	Avatar   *flowStep
	Created  bool
}

// didServiceStep 单条服务的登记结果。
type didServiceStep struct {
	Label    string `json:"label"`
	Endpoint string `json:"serviceEndpoint"`
	Skipped  bool   `json:"skipped"`
	TxHash   string `json:"txHash,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// ==================== 纯函数(可单测) ====================

// buildDidName 组装链上 DidName {alias, suffix};alias 为空(或全空白)表示不设别名,
// 返回 nil。suffix 未指定时由 rnd 代填。
func buildDidName(alias string, suffix *uint32, rnd func() uint32) (map[string]any, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return nil, nil
	}
	if suffix == nil {
		s := rnd()
		suffix = &s
	}
	return map[string]any{"alias": alias, "suffix": *suffix}, nil
}

// randomDidSuffix 服务端代填的数字后缀,范围 [didAliasSuffixMin, didAliasSuffixMax)。
func randomDidSuffix() uint32 {
	return didAliasSuffixMin + uint32(rand.Intn(int(didAliasSuffixMax-didAliasSuffixMin)))
}

// buildDidDocInput 组装链上 DidDocumentInput wire map。
// 注意:struct 参数必须用 map[string]any —— args_coerce 的 struct 分支按
// 精确类型 map[string]any 断言(与 vcFlowEnsureDID 相同的坑)。
func buildDidDocInput(subjectType, pubBase58 string, services []didServiceSpec, avatarURI string) map[string]any {
	wireServices := make([]any, 0, len(services))
	for _, s := range services {
		wireServices = append(wireServices, map[string]any{"label": s.Label, "service_endpoint": s.Endpoint})
	}
	return map[string]any{
		"subject_type": subjectType,
		"keys":         []any{map[string]any{"public_key": pubBase58, "label": didDefaultKeyLabel}},
		"services":     wireServices,
		"avatar_uri":   avatarURI,
	}
}

// normalizeDidCreateRequest 填充缺省值并裁剪空白。
func normalizeDidCreateRequest(req didCreateRequest) didCreateRequest {
	req.SubjectType = strings.TrimSpace(req.SubjectType)
	if req.SubjectType == "" {
		req.SubjectType = "Personal"
	}
	req.Alias = strings.TrimSpace(req.Alias)
	for i := range req.Services {
		req.Services[i].Label = strings.TrimSpace(req.Services[i].Label)
		req.Services[i].Endpoint = strings.TrimSpace(req.Services[i].Endpoint)
	}
	req.AvatarURI = strings.TrimSpace(req.AvatarURI)
	return req
}

// validateDidCreateRequest 校验必填与基本格式。address 必填的理由与 vc-flow 的
// userAddress 相同:32 字节私钥在不同曲线下派生不同地址,显式地址配合曲线回退
// 才能锁定正确公钥,避免在错误地址上建 DID。serviceEndpoint 只做「绝对 URI」
// 预检,scheme 白名单等更严校验交链端(1048-1050)。
func validateDidCreateRequest(req didCreateRequest) error {
	if strings.TrimSpace(req.PrivateKey) == "" {
		return fmt.Errorf("privateKey is required")
	}
	if strings.TrimSpace(req.Address) == "" {
		return fmt.Errorf("address is required(必填:32字节私钥在不同曲线下派生不同地址,请传账户生成时返回的地址,服务端按其自动匹配曲线)")
	}
	if req.SubjectType != "Personal" && req.SubjectType != "Organization" {
		return fmt.Errorf("subjectType must be Personal or Organization, got %q", req.SubjectType)
	}
	for i, s := range req.Services {
		if strings.TrimSpace(s.Label) == "" {
			return fmt.Errorf("services[%d].label is required(链端拒绝空 label,错误 1047)", i)
		}
		if strings.TrimSpace(s.Endpoint) == "" {
			return fmt.Errorf("services[%d].serviceEndpoint is required(链端要求绝对 URI,错误 1048)", i)
		}
		u, err := url.Parse(s.Endpoint)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("services[%d].serviceEndpoint %q must be an absolute URI (e.g. https://example.com)", i, s.Endpoint)
		}
	}
	return nil
}

// planDidBackfill 链上已有 DID 时,比对请求与链上文档得出补齐动作。
// 语义:只补齐缺失,不覆盖已有(services 同 label+endpoint 视为已存在;
// avatar 仅在显式传入且不同时更新;alias 按别名字符串比对,忽略后缀)。
func planDidBackfill(doc map[string]any, req didCreateRequest) didBackfillPlan {
	plan := didBackfillPlan{}
	if doc == nil {
		plan.NeedCreate = true
		return plan
	}
	if alias := strings.TrimSpace(req.Alias); alias != "" && didDocumentAliasName(doc) != alias {
		plan.SetAlias = true
	}
	chainServices, _ := doc["services"].([]any)
	for _, s := range req.Services {
		found := false
		for _, row := range chainServices {
			m, ok := row.(map[string]any)
			if !ok {
				continue
			}
			if m["label"] == s.Label && m["service_endpoint"] == s.Endpoint {
				found = true
				break
			}
		}
		if !found {
			plan.AddServices = append(plan.AddServices, s)
		}
	}
	if req.AvatarURI != "" {
		if chainAvatar, _ := doc["avatar_uri"].(string); chainAvatar != req.AvatarURI {
			plan.SetAvatar = true
		}
	}
	return plan
}

// didDocumentAliasName 从 Document view 值提取当前别名字符串(无别名返回空串)。
func didDocumentAliasName(doc map[string]any) string {
	if doc == nil {
		return ""
	}
	aliasMap, ok := doc["alias"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := aliasMap["alias"].(string)
	return name
}

// isDidNotFoundErr 判断链端错误是否为 DID 未创建(DidNotFound/1025)。
func isDidNotFoundErr(msg string) bool {
	return isTolerableChainError(msg, "DidNotFound", "1025")
}

// isDidNameFormatErr 判断链端错误是否属于别名格式/冲突类(1035-1039 与 1028),
// 仅这类错误才附加「alias-数字」格式提示,避免把余额不足等无关错误误包。
func isDidNameFormatErr(msg string) bool {
	return isTolerableChainError(msg,
		"NameAlreadyBound", "NameInvalidLength", "NameInvalidFormat",
		"NameAliasInvalidLength", "NameSuffixInvalid", "NameHashCollision")
}

// formatDidNameError 包装链端别名相关错误,附上格式提示。
func formatDidNameError(err error) error {
	return fmt.Errorf("%v(别名须为「alias-数字」格式且全局唯一,如 alice-1024;错误 1035-1039 为格式/长度/后缀/冲突类错误)", err)
}

// didAliasOutcome 一次别名绑定的结果。
type didAliasOutcome struct {
	TxHash string
	Name   map[string]any // 最终使用的 DidName wire 值
	Err    error
}

// didBindAlias 把别名绑定到 DID(经 send 发起链上写,CreateWithAlias/SetAlias 均适用)。
// 链上唯一性按完整 DidName(alias+suffix 整体)判定:服务端代填的随机 suffix 撞上
// 已占名(1028)时自动换号重试至多 didAliasMaxRetries 次;显式 suffix 撞名直接报错。
// 格式/冲突类错误(1035-1039/1028)附加格式提示,其余错误(余额不足等)原样透传。
func didBindAlias(alias string, suffix *uint32, send func(name map[string]any) (string, error)) didAliasOutcome {
	for attempt := 0; ; attempt++ {
		name, err := buildDidName(alias, suffix, randomDidSuffix)
		if err != nil {
			return didAliasOutcome{Err: err}
		}
		txHash, err := send(name)
		if err == nil {
			return didAliasOutcome{TxHash: txHash, Name: name}
		}
		if isTolerableChainError(err.Error(), "NameAlreadyBound", "1028") {
			// 显式指定的 suffix 被占 → 不换号直接报错;代填 → 换号重试
			if suffix == nil && attempt < didAliasMaxRetries {
				continue
			}
			return didAliasOutcome{Name: name, Err: formatDidNameError(err)}
		}
		if isDidNameFormatErr(err.Error()) {
			return didAliasOutcome{Name: name, Err: formatDidNameError(err)}
		}
		return didAliasOutcome{Name: name, Err: err}
	}
}

// parseDidNameString 把 "alice-1024" 拆成 DidName 所需的 (alias, suffix)。
// 以最后一个 '-' 分隔,suffix 必须为纯数字。
func parseDidNameString(s string) (string, uint32, error) {
	s = strings.TrimSpace(s)
	idx := strings.LastIndex(s, "-")
	if idx <= 0 || idx == len(s)-1 {
		return "", 0, fmt.Errorf("invalid name %q: 链上别名为「alias-数字」格式,如 alice-1024", s)
	}
	alias, suffixStr := s[:idx], s[idx+1:]
	suffix, err := strconv.ParseUint(suffixStr, 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("invalid name %q: 后缀 %q 必须是数字", s, suffixStr)
	}
	return alias, uint32(suffix), nil
}

// normalizeVcFlowDidOptions vc-flow 透传选项的裁剪。
func normalizeVcFlowDidOptions(opts *vcFlowDidOptions) *vcFlowDidOptions {
	if opts == nil {
		return nil
	}
	opts.Alias = strings.TrimSpace(opts.Alias)
	for i := range opts.Services {
		opts.Services[i].Label = strings.TrimSpace(opts.Services[i].Label)
		opts.Services[i].Endpoint = strings.TrimSpace(opts.Services[i].Endpoint)
	}
	opts.AvatarURI = strings.TrimSpace(opts.AvatarURI)
	return opts
}

// summarizeDidEnsure 把分步结果汇总成 vc-flow 单步展示。
func summarizeDidEnsure(res didEnsureResult) flowStep {
	step := flowStep{}
	if res.Created {
		step.Detail = res.Create.Detail
		if step.Detail == "" {
			step.Detail = "DID created"
		}
	} else {
		step.Detail = "DID already exists"
	}
	if res.Create.TxHash != "" {
		step.TxHash = res.Create.TxHash
	}

	allSkipped := !res.Created
	var actions []string
	if res.Alias != nil && !res.Alias.Skipped {
		allSkipped = false
		actions = append(actions, "alias set")
	}
	for _, s := range res.Services {
		if !s.Skipped {
			allSkipped = false
		}
	}
	if len(res.Services) > 0 {
		actions = append(actions, fmt.Sprintf("%d service(s) added", len(res.Services)))
	}
	if res.Avatar != nil && !res.Avatar.Skipped {
		allSkipped = false
		actions = append(actions, "avatar updated")
	}
	step.Skipped = allSkipped
	if len(actions) > 0 {
		step.Detail += "; " + strings.Join(actions, ", ")
	}
	return step
}

// ==================== Handler ====================

// DidHandler 提供 DID 完整生命周期工具端点。
type DidHandler struct {
	nm *client.NetworkManager
}

// NewDidHandler 创建绑定到 NetworkManager 的 DidHandler。
func NewDidHandler(nm *client.NetworkManager) *DidHandler {
	return &DidHandler{nm: nm}
}

// didClient 获取当前网络客户端;失败时已写好 500 响应,返回 nil。
func (h *DidHandler) didClient(c *gin.Context, endpoint string) *milon.Client {
	mc, _ := h.nm.GetCurrent()
	if mc == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "no current network", nil))
		return nil
	}
	return mc
}

// Create 处理 POST /api/tool/did/create:一步完成 创建(+别名)+服务+头像;
// 链上已有 DID 时按请求补齐差异。
func (h *DidHandler) Create(c *gin.Context) {
	var req didCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "DidCreate", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}
	req = normalizeDidCreateRequest(req)
	if err := validateDidCreateRequest(req); err != nil {
		logParamError(c, "DidCreate", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	sk, pub, addr, err := resolveFlowParty("did", req.PrivateKey, req.PublicKey, req.Address)
	if err != nil {
		logParamError(c, "DidCreate", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid identity: "+err.Error(), nil))
		return
	}
	mc := h.didClient(c, "DidCreate")
	if mc == nil {
		return
	}

	res, err := didEnsureDocument(mc, addr, sk, pub, req.SubjectType, &vcFlowDidOptions{
		Alias:     req.Alias,
		Suffix:    req.Suffix,
		Services:  req.Services,
		AvatarURI: req.AvatarURI,
	})
	if err != nil {
		logSDKError(c, "DidCreate", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "did create: "+err.Error(), nil))
		return
	}

	document := didQueryDocument(mc, addr)
	resp := gin.H{
		"address": addr.ToBase58(),
		"didId":   "did:milon:" + addr.ToBase58(),
		"created": res.Created,
		"steps": gin.H{
			"create":   res.Create,
			"alias":    res.Alias,
			"services": res.Services,
			"avatar":   res.Avatar,
		},
		"document": document,
	}
	logBusinessInfo(c, "DidCreate", "address", addr.ToBase58(), "created", res.Created)
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// SetAlias 处理 POST /api/tool/did/set-alias。
func (h *DidHandler) SetAlias(c *gin.Context) {
	var req struct {
		didMutateBase
		Alias  string  `json:"alias"`
		Suffix *uint32 `json:"suffix"`
	}
	if !h.bindMutate(c, "DidSetAlias", &req) {
		return
	}
	req.Alias = strings.TrimSpace(req.Alias)
	if req.Alias == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "alias is required", nil))
		return
	}
	sk, pub, addr, ok := h.resolve(c, "DidSetAlias", req.didBase())
	if !ok {
		return
	}
	mc := h.didClient(c, "DidSetAlias")
	if mc == nil {
		return
	}

	out := didBindAlias(req.Alias, req.Suffix, func(name map[string]any) (string, error) {
		return didSend(mc, "SetAlias", provider.Args{"subject": addr.ToBase58(), "name": name}, addr, sk, pub)
	})
	if out.Err != nil {
		h.fail(c, "DidSetAlias", out.Err)
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": addr.ToBase58(), "txHash": out.TxHash,
		"name": out.Name, "nameBinding": didQueryNameBinding(mc, out.Name),
	}, "ok"))
}

// AddService 处理 POST /api/tool/did/add-service,响应带最新文档(从中取链上分配的 service id)。
func (h *DidHandler) AddService(c *gin.Context) {
	var req struct {
		didMutateBase
		Label           string `json:"label"`
		ServiceEndpoint string `json:"serviceEndpoint"`
	}
	if !h.bindMutate(c, "DidAddService", &req) {
		return
	}
	sk, pub, addr, ok := h.resolve(c, "DidAddService", req.didBase())
	if !ok {
		return
	}
	if req.Label == "" || req.ServiceEndpoint == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "label 与 serviceEndpoint 均必填(链端错误 1047/1048)", nil))
		return
	}
	mc := h.didClient(c, "DidAddService")
	if mc == nil {
		return
	}

	args := provider.Args{
		"subject": addr.ToBase58(),
		"input":   map[string]any{"label": req.Label, "service_endpoint": req.ServiceEndpoint},
	}
	txHash, err := didSend(mc, "AddService", args, addr, sk, pub)
	if err != nil {
		h.fail(c, "DidAddService", err)
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": addr.ToBase58(), "txHash": txHash,
		"document": didQueryDocument(mc, addr),
	}, "ok"))
}

// UpdateService 处理 POST /api/tool/did/update-service。
func (h *DidHandler) UpdateService(c *gin.Context) {
	var req struct {
		didMutateBase
		Id              *uint8 `json:"id"`
		Label           string `json:"label"`
		ServiceEndpoint string `json:"serviceEndpoint"`
	}
	if !h.bindMutate(c, "DidUpdateService", &req) {
		return
	}
	if req.Id == nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "id is required(从 GET /api/tool/did/:address/document 的 services 列表获取)", nil))
		return
	}
	if req.Label == "" || req.ServiceEndpoint == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "label 与 serviceEndpoint 均必填(链端错误 1047/1048)", nil))
		return
	}
	sk, pub, addr, ok := h.resolve(c, "DidUpdateService", req.didBase())
	if !ok {
		return
	}
	mc := h.didClient(c, "DidUpdateService")
	if mc == nil {
		return
	}

	args := provider.Args{
		"subject": addr.ToBase58(),
		"id":      *req.Id,
		"input":   map[string]any{"label": req.Label, "service_endpoint": req.ServiceEndpoint},
	}
	txHash, err := didSend(mc, "UpdateService", args, addr, sk, pub)
	if err != nil {
		h.fail(c, "DidUpdateService", err)
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": addr.ToBase58(), "txHash": txHash,
		"document": didQueryDocument(mc, addr),
	}, "ok"))
}

// RemoveService 处理 POST /api/tool/did/remove-service。
func (h *DidHandler) RemoveService(c *gin.Context) {
	var req struct {
		didMutateBase
		Id *uint8 `json:"id"`
	}
	if !h.bindMutate(c, "DidRemoveService", &req) {
		return
	}
	if req.Id == nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "id is required", nil))
		return
	}
	sk, pub, addr, ok := h.resolve(c, "DidRemoveService", req.didBase())
	if !ok {
		return
	}
	mc := h.didClient(c, "DidRemoveService")
	if mc == nil {
		return
	}

	txHash, err := didSend(mc, "RemoveService", provider.Args{
		"subject": addr.ToBase58(), "id": *req.Id,
	}, addr, sk, pub)
	if err != nil {
		h.fail(c, "DidRemoveService", err)
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": addr.ToBase58(), "txHash": txHash,
		"document": didQueryDocument(mc, addr),
	}, "ok"))
}

// SetAvatarUri 处理 POST /api/tool/did/set-avatar-uri。
func (h *DidHandler) SetAvatarUri(c *gin.Context) {
	var req struct {
		didMutateBase
		AvatarURI string `json:"avatarUri"`
	}
	if !h.bindMutate(c, "DidSetAvatarUri", &req) {
		return
	}
	req.AvatarURI = strings.TrimSpace(req.AvatarURI)
	if req.AvatarURI == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "avatarUri is required(链端要求长度 1-512 字节,错误 1045)", nil))
		return
	}
	sk, pub, addr, ok := h.resolve(c, "DidSetAvatarUri", req.didBase())
	if !ok {
		return
	}
	mc := h.didClient(c, "DidSetAvatarUri")
	if mc == nil {
		return
	}

	txHash, err := didSend(mc, "SetAvatarUri", provider.Args{
		"subject": addr.ToBase58(), "avatar_uri": req.AvatarURI,
	}, addr, sk, pub)
	if err != nil {
		h.fail(c, "DidSetAvatarUri", err)
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": addr.ToBase58(), "txHash": txHash,
		"document": didQueryDocument(mc, addr),
	}, "ok"))
}

// AddKey 处理 POST /api/tool/did/add-key,响应带最新文档(从中取链上分配的 key id)。
func (h *DidHandler) AddKey(c *gin.Context) {
	var req struct {
		didMutateBase
		NewPublicKey string  `json:"newPublicKey"` // 要加入文档的公钥(base58)
		Label        *string `json:"label"`
	}
	if !h.bindMutate(c, "DidAddKey", &req) {
		return
	}
	req.NewPublicKey = strings.TrimSpace(req.NewPublicKey)
	if req.NewPublicKey == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "newPublicKey is required", nil))
		return
	}
	sk, pub, addr, ok := h.resolve(c, "DidAddKey", req.didBase())
	if !ok {
		return
	}
	mc := h.didClient(c, "DidAddKey")
	if mc == nil {
		return
	}

	input := map[string]any{"public_key": req.NewPublicKey}
	if req.Label != nil && strings.TrimSpace(*req.Label) != "" {
		input["label"] = strings.TrimSpace(*req.Label)
	} else {
		input["label"] = nil // option<String>:None
	}
	txHash, err := didSend(mc, "AddKey", provider.Args{
		"subject": addr.ToBase58(), "input": input,
	}, addr, sk, pub)
	if err != nil {
		h.fail(c, "DidAddKey", err)
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": addr.ToBase58(), "txHash": txHash,
		"document": didQueryDocument(mc, addr),
	}, "ok"))
}

// UpdateKey 处理 POST /api/tool/did/update-key。
func (h *DidHandler) UpdateKey(c *gin.Context) {
	var req struct {
		didMutateBase
		Id           *uint8  `json:"id"`
		NewPublicKey string  `json:"newPublicKey"`
		Label        *string `json:"label"`
	}
	if !h.bindMutate(c, "DidUpdateKey", &req) {
		return
	}
	req.NewPublicKey = strings.TrimSpace(req.NewPublicKey)
	if req.Id == nil || req.NewPublicKey == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "id 与 newPublicKey 均必填", nil))
		return
	}
	sk, pub, addr, ok := h.resolve(c, "DidUpdateKey", req.didBase())
	if !ok {
		return
	}
	mc := h.didClient(c, "DidUpdateKey")
	if mc == nil {
		return
	}

	input := map[string]any{"public_key": req.NewPublicKey}
	if req.Label != nil && strings.TrimSpace(*req.Label) != "" {
		input["label"] = strings.TrimSpace(*req.Label)
	} else {
		input["label"] = nil
	}
	txHash, err := didSend(mc, "UpdateKey", provider.Args{
		"subject": addr.ToBase58(), "id": *req.Id, "input": input,
	}, addr, sk, pub)
	if err != nil {
		h.fail(c, "DidUpdateKey", err)
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": addr.ToBase58(), "txHash": txHash,
		"document": didQueryDocument(mc, addr),
	}, "ok"))
}

// RemoveKey 处理 POST /api/tool/did/remove-key(最后一把密钥链端拒绝,错误 1042)。
func (h *DidHandler) RemoveKey(c *gin.Context) {
	var req struct {
		didMutateBase
		Id *uint8 `json:"id"`
	}
	if !h.bindMutate(c, "DidRemoveKey", &req) {
		return
	}
	if req.Id == nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "id is required", nil))
		return
	}
	sk, pub, addr, ok := h.resolve(c, "DidRemoveKey", req.didBase())
	if !ok {
		return
	}
	mc := h.didClient(c, "DidRemoveKey")
	if mc == nil {
		return
	}

	txHash, err := didSend(mc, "RemoveKey", provider.Args{
		"subject": addr.ToBase58(), "id": *req.Id,
	}, addr, sk, pub)
	if err != nil {
		h.fail(c, "DidRemoveKey", err)
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": addr.ToBase58(), "txHash": txHash,
		"document": didQueryDocument(mc, addr),
	}, "ok"))
}

// Deactivate 处理 POST /api/tool/did/deactivate(停用后 identity 写操作均被拒,错误 1026)。
func (h *DidHandler) Deactivate(c *gin.Context) {
	var req didMutateBase
	if !h.bindMutate(c, "DidDeactivate", &req) {
		return
	}
	sk, pub, addr, ok := h.resolve(c, "DidDeactivate", &req)
	if !ok {
		return
	}
	mc := h.didClient(c, "DidDeactivate")
	if mc == nil {
		return
	}

	txHash, err := didSend(mc, "Deactivate", provider.Args{
		"subject": addr.ToBase58(),
	}, addr, sk, pub)
	if err != nil {
		h.fail(c, "DidDeactivate", err)
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": addr.ToBase58(), "txHash": txHash,
	}, "ok"))
}

// Document 处理 GET /api/tool/did/:address/document:查询完整 DID 文档。
func (h *DidHandler) Document(c *gin.Context) {
	addrStr := c.Param("address")
	mc := h.didClient(c, "DidDocument")
	if mc == nil {
		return
	}
	value, err := flowCallView(mc, vcFlowIdentityApp, "Document", provider.Args{"subject": addrStr})
	if err != nil {
		logSDKError(c, "DidDocument", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, err.Error(), nil))
		return
	}
	if !flowViewOK(value) {
		msg := fmt.Sprintf("%v", value)
		if isDidNotFoundErr(msg) {
			c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_NOT_FOUND,
				fmt.Sprintf("DID not found for %s(尚未创建,请先调用 POST /api/tool/did/create)", addrStr), nil))
			return
		}
		logSDKError(c, "DidDocument", fmt.Errorf("%s", msg))
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, msg, nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address":  addrStr,
		"didId":    "did:milon:" + addrStr,
		"document": value,
	}, "ok"))
}

// NameBinding 处理 GET /api/tool/did/name-binding?name=alice-1024:按别名反查绑定。
func (h *DidHandler) NameBinding(c *gin.Context) {
	nameStr := strings.TrimSpace(c.Query("name"))
	if nameStr == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "name is required(query 参数,格式 alias-数字,如 alice-1024)", nil))
		return
	}
	alias, suffix, err := parseDidNameString(nameStr)
	if err != nil {
		logParamError(c, "DidNameBinding", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	mc := h.didClient(c, "DidNameBinding")
	if mc == nil {
		return
	}
	name := map[string]any{"alias": alias, "suffix": suffix}
	value, err := flowCallView(mc, vcFlowIdentityApp, "NameBinding", provider.Args{"name": name})
	if err != nil {
		logSDKError(c, "DidNameBinding", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, err.Error(), nil))
		return
	}
	if !flowViewOK(value) {
		msg := fmt.Sprintf("%v", value)
		if isTolerableChainError(msg, "NameNotFound", "1029") {
			c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_NOT_FOUND,
				fmt.Sprintf("name %q 未绑定", nameStr), nil))
			return
		}
		logSDKError(c, "DidNameBinding", fmt.Errorf("%s", msg))
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, msg, nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"name": name, "binding": value}, "ok"))
}

// ==================== 内部辅助 ====================

// bindMutate 绑定并校验细粒度端点的完整请求体(含端点特有字段);失败时已写好 400 响应。
func (h *DidHandler) bindMutate(c *gin.Context, endpoint string, req didMutateProvider) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		logParamError(c, endpoint, err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return false
	}
	base := req.didBase()
	if strings.TrimSpace(base.PrivateKey) == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "privateKey is required", nil))
		return false
	}
	if strings.TrimSpace(base.Address) == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
			"address is required(必填:32字节私钥在不同曲线下派生不同地址,请传账户生成时返回的地址,服务端按其自动匹配曲线)", nil))
		return false
	}
	return true
}

// resolve 解析细粒度端点的身份;失败时已写好 400 响应。
func (h *DidHandler) resolve(c *gin.Context, endpoint string, base *didMutateBase) (crypto.SecretKeyer, *crypto.PublicKey, crypto.Address, bool) {
	sk, pub, addr, err := resolveFlowParty("did", base.PrivateKey, base.PublicKey, base.Address)
	if err != nil {
		logParamError(c, endpoint, err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid identity: "+err.Error(), nil))
		return nil, nil, crypto.Address{}, false
	}
	return sk, pub, addr, true
}

// fail 细粒度端点的统一 500 出口。
func (h *DidHandler) fail(c *gin.Context, endpoint string, err error) {
	logSDKError(c, endpoint, err)
	c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, err.Error(), nil))
}

// didSend 构建并发送一笔 identity 写交易,返回 txHash。
func didSend(mc *milon.Client, method string, args provider.Args, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey) (string, error) {
	tx, err := flowBuildTx(mc, vcFlowIdentityApp, method, args, addr, sk, pub)
	if err != nil {
		return "", err
	}
	return flowSendTx(mc, tx)
}

// didQueryDocument 查询 DID 文档;不存在或失败时返回 nil(诊断用途,不作为错误出口)。
func didQueryDocument(mc *milon.Client, addr crypto.Address) any {
	value, err := flowCallView(mc, vcFlowIdentityApp, "Document", provider.Args{"subject": addr.ToBase58()})
	if err != nil || !flowViewOK(value) {
		return nil
	}
	return value
}

// didQueryNameBinding 查询别名绑定(诊断用途,失败返回 nil)。
func didQueryNameBinding(mc *milon.Client, name map[string]any) any {
	value, err := flowCallView(mc, vcFlowIdentityApp, "NameBinding", provider.Args{"name": name})
	if err != nil || !flowViewOK(value) {
		return nil
	}
	return value
}

// didEnsureDocument 确保 addr 已建 DID 且别名/服务/头像与请求一致:
//   - 链上无文档:Create(带齐 keys/services/avatar)或 CreateWithAlias(alias 非空时)
//   - 链上已有:按 planDidBackfill 的差异执行 SetAlias/AddService/SetAvatarUri
//
// 幂等容忍:DidAlreadyExists(1024);NameAlreadyBound(1028) 仅在别名绑在本人时容忍,
// 绑在他人则原样报错。供 POST /api/tool/did/create 与 vc-flow DID 步骤共用。
func didEnsureDocument(mc *milon.Client, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey, subjectType string, opts *vcFlowDidOptions) (didEnsureResult, error) {
	res := didEnsureResult{}
	opts = normalizeVcFlowDidOptions(opts)
	if opts == nil {
		opts = &vcFlowDidOptions{}
	}

	// 链上文档现状
	var doc map[string]any
	if value, err := flowCallView(mc, vcFlowIdentityApp, "Document", provider.Args{"subject": addr.ToBase58()}); err == nil && flowViewOK(value) {
		if d, ok := value.(map[string]any); ok {
			doc = d
		}
	}

	// [A] 未创建 → 一步建齐
	if doc == nil {
		avatarURI := opts.AvatarURI
		if avatarURI == "" {
			avatarURI = vcFlowAvatarURI
		}
		docInput := buildDidDocInput(subjectType, pub.ToBase58(), opts.Services, avatarURI)
		if opts.Alias != "" {
			// CreateWithAlias 一步建齐 + 绑名;代填 suffix 撞名时 didBindAlias 自动换号重试
			out := didBindAlias(opts.Alias, opts.Suffix, func(name map[string]any) (string, error) {
				return didSend(mc, "CreateWithAlias", provider.Args{
					"subject": addr.ToBase58(), "doc": docInput, "name": name,
				}, addr, sk, pub)
			})
			if out.Err != nil {
				return res, out.Err
			}
			res.Created = true
			res.Create = flowStep{TxHash: out.TxHash, Detail: "created with alias"}
			return res, nil
		}
		txHash, err := didSend(mc, "Create", provider.Args{"subject": addr.ToBase58(), "doc": docInput}, addr, sk, pub)
		if err != nil {
			if isTolerableChainError(err.Error(), "DidAlreadyExists", "1024") {
				res.Create = flowStep{Skipped: true, Detail: "DID already exists on chain: " + err.Error()}
				return res, nil
			}
			return res, err
		}
		res.Created = true
		res.Create = flowStep{TxHash: txHash}
		return res, nil
	}

	// [B] 已创建 → 按差异补齐
	res.Create = flowStep{Skipped: true, Detail: "DID document already exists"}
	plan := planDidBackfill(doc, didCreateRequest{
		Alias:     opts.Alias,
		Suffix:    opts.Suffix,
		Services:  opts.Services,
		AvatarURI: opts.AvatarURI,
	})

	if plan.SetAlias {
		out := didBindAlias(opts.Alias, opts.Suffix, func(name map[string]any) (string, error) {
			return didSend(mc, "SetAlias", provider.Args{"subject": addr.ToBase58(), "name": name}, addr, sk, pub)
		})
		if out.Err != nil {
			return res, out.Err
		}
		res.Alias = &flowStep{TxHash: out.TxHash}
	}

	for _, s := range plan.AddServices {
		step := didServiceStep{Label: s.Label, Endpoint: s.Endpoint}
		txHash, err := didSend(mc, "AddService", provider.Args{
			"subject": addr.ToBase58(),
			"input":   map[string]any{"label": s.Label, "service_endpoint": s.Endpoint},
		}, addr, sk, pub)
		if err != nil {
			return res, fmt.Errorf("add service %s: %w", s.Label, err)
		}
		step.TxHash = txHash
		res.Services = append(res.Services, step)
	}

	if plan.SetAvatar {
		txHash, err := didSend(mc, "SetAvatarUri", provider.Args{
			"subject": addr.ToBase58(), "avatar_uri": opts.AvatarURI,
		}, addr, sk, pub)
		if err != nil {
			return res, fmt.Errorf("set avatar uri: %w", err)
		}
		res.Avatar = &flowStep{TxHash: txHash}
	}

	return res, nil
}
