package handler

import (
	"fmt"
	"net/http"
	"strings"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
)

// ==================== 发证书:给派生账户签发 VC(总览账户卡行操作) ====================
// 复用播种器 vc 模块的幂等七步函数链(领水/DID/组织/schema/签发/披露/验证),
// 抽出 IssueVcToAccount 供 seed_runner_flows.seedVcFlow 与本端点共用。
// 响应不含私钥——issuer/subject 私钥由 seedPhrase 本地派生(sha256(短语:角色:名)),服务端只回地址与结果。

const (
	vcIssueCountMin = 1
	vcIssueCountMax = 10
)

type vcIssueRequest struct {
	Config       *SeedConfig `json:"config"`     // 派生命名空间(总览当前旋钮);缺省用默认配置
	SeedPhrase   string      `json:"seedPhrase"` // config 缺省时的短语兜底
	IssuerName   string      `json:"issuerName"` // 签发方账户名(须为 role=issuer,如 kycorg)
	SubjectName  string      `json:"subjectName"`
	SchemaPrefix string      `json:"schemaPrefix"` // 如 SeedKyc → 签发 SeedKyc1..count
	Count        int         `json:"count"`
}

// resolveVcIssueAccounts config+账户名 → 派生账户对(纯派生,离线可测)。
func resolveVcIssueAccounts(cfg SeedConfig, issuerName, subjectName string) (*SeedPlanAccount, *SeedPlanAccount, error) {
	plan, err := BuildSeedPlanV2(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("计划构建失败: %w", err)
	}
	accts, err := SeedBuildAccounts(plan)
	if err != nil {
		return nil, nil, fmt.Errorf("账户派生失败: %w", err)
	}
	issuer, ok := accts[issuerName]
	if !ok {
		return nil, nil, fmt.Errorf("签发方 %q 不在当前派生命名空间(vcCert=false 时无签发方;可先在播种器开 vcCert)", issuerName)
	}
	if issuer.Role != "issuer" {
		return nil, nil, fmt.Errorf("%s 角色为 %s,只有签发方(issuer)能签发证书", issuerName, issuer.Role)
	}
	subject, ok := accts[subjectName]
	if !ok {
		return nil, nil, fmt.Errorf("目标账户 %q 不在当前派生命名空间(检查 accounts.count/seedPhrase)", subjectName)
	}
	return issuer, subject, nil
}

// IssueVcToAccount 单个 (issuer, subject) 的发证书链路(与 seed_runner_flows.seedVcFlow 同构)。
// 幂等:领水/DID/组织注册/披露全部可重入;重复发同一 schema → already。
// 全部凭证上链返回 nil;HasValidVcFromIssuer=false(冻结/撤销)不算失败,是正常生命周期状态。
func IssueVcToAccount(mc *milon.Client, issuer, subject *SeedPlanAccount, schemaPrefix string, count int, logf func(string, ...any)) ([]vcFlowVerifyItem, error) {
	log := func(format string, a ...any) {
		if logf != nil {
			logf(format, a...)
		}
	}
	// [1] 双方领水(幂等)
	if _, err := flowEnsureFaucet(mc, *issuer.ADR, issuer.SK, issuer.PUB); err != nil {
		return nil, fmt.Errorf("issuer faucet: %w", err)
	}
	if _, err := flowEnsureFaucet(mc, *subject.ADR, subject.SK, subject.PUB); err != nil {
		return nil, fmt.Errorf("subject faucet: %w", err)
	}
	// [2][3] issuer 建 Organization DID + 注册组织/schema(幂等)
	issuerOpts := &vcFlowDidOptions{Alias: issuer.Alias, Suffix: &issuer.AliasSuffix,
		AvatarURI: issuer.AvatarURL, AliasOptional: true}
	issuerDidStep, err := vcFlowEnsureDID(mc, *issuer.ADR, issuer.SK, issuer.PUB, "Organization", issuerOpts)
	if err != nil {
		return nil, fmt.Errorf("issuer did: %w", err)
	}
	if strings.Contains(issuerDidStep.Detail, "alias occupied") {
		log("    ⚠ 别名 %s 已被占用,跳过绑定(DID 本体不受影响)", issuer.Alias)
	}
	schemas := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		schemas = append(schemas, fmt.Sprintf("%s%d", schemaPrefix, i))
	}
	if _, err := vcFlowEnsureOrganization(mc, *issuer.ADR, issuer.SK, issuer.PUB, schemas); err != nil {
		return nil, fmt.Errorf("register organization: %w", err)
	}
	// [4] subject 建 Personal DID(幂等)
	subjectOpts := &vcFlowDidOptions{Alias: subject.Alias, Suffix: &subject.AliasSuffix,
		AvatarURI: subject.AvatarURL, AliasOptional: true}
	subjectDidStep, err := vcFlowEnsureDID(mc, *subject.ADR, subject.SK, subject.PUB, "Personal", subjectOpts)
	if err != nil {
		return nil, fmt.Errorf("subject did: %w", err)
	}
	if strings.Contains(subjectDidStep.Detail, "alias occupied") {
		log("    ⚠ 别名 %s 已被占用,跳过绑定(DID 本体不受影响)", subject.Alias)
	}
	// [5] 链下签发 count 张
	chainID, err := flowChainID(mc)
	if err != nil {
		return nil, fmt.Errorf("chain id: %w", err)
	}
	creds, err := buildVcFlowCredentials(chainID, *issuer.ADR, *subject.ADR,
		issuer.SK, issuer.PUB, schemaPrefix, count, nil)
	if err != nil {
		return nil, fmt.Errorf("build credentials: %w", err)
	}
	// [6] 逐张披露(幂等)
	for _, cred := range creds {
		if _, _, _, err := vcFlowDisclose(mc, *subject.ADR, subject.SK, subject.PUB, *issuer.ADR, cred); err != nil {
			return nil, fmt.Errorf("disclose %s: %w", cred.Schema, err)
		}
	}
	// [7] 回读验证:披露上链即链路完成
	items := vcFlowVerify(mc, *subject.ADR, *issuer.ADR, creds)
	allOnChain := len(items) > 0
	for _, it := range items {
		if !it.OnChain {
			allOnChain = false
			break
		}
	}
	if !allOnChain {
		return nil, fmt.Errorf("凭证披露丢失: %v", items)
	}
	return items, nil
}

// VcIssue POST /api/tool/vc-issue — 给派生账户发证书。
func (h *SeedJobHandler) VcIssue(c *gin.Context) {
	var req vcIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "VcIssue", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}
	cfg := SeedConfigDefaults()
	if req.Config != nil {
		cfg = *req.Config
	}
	if cfg.SeedPhrase == "" {
		cfg.SeedPhrase = req.SeedPhrase
	}
	if cfg.SeedPhrase == "" {
		cfg.SeedPhrase = SeedDefaultPhrase
	}
	if strings.TrimSpace(req.SchemaPrefix) == "" {
		logParamError(c, "VcIssue", fmt.Errorf("schemaPrefix 为空"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "schemaPrefix 必填(如 SeedKyc)", nil))
		return
	}
	if req.IssuerName == "" || req.SubjectName == "" {
		logParamError(c, "VcIssue", fmt.Errorf("issuerName/subjectName 为空"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "issuerName 与 subjectName 必填", nil))
		return
	}
	count := req.Count
	if count == 0 {
		count = 1
	}
	if count < vcIssueCountMin || count > vcIssueCountMax {
		logParamError(c, "VcIssue", fmt.Errorf("count=%d", count))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
			fmt.Sprintf("count 须在 %d..%d, got %d", vcIssueCountMin, vcIssueCountMax, count), nil))
		return
	}
	issuer, subject, err := resolveVcIssueAccounts(cfg, req.IssuerName, req.SubjectName)
	if err != nil {
		logParamError(c, "VcIssue", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	mc := middleware.ClientFrom(c)
	if mc == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "no current network", nil))
		return
	}
	items, err := IssueVcToAccount(mc, issuer, subject, req.SchemaPrefix, count, nil)
	if err != nil {
		logSDKError(c, "VcIssue", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "发证书失败: "+err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"issuer":  gin.H{"name": issuer.Name, "address": issuer.Address},
		"subject": gin.H{"name": subject.Name, "address": subject.Address},
		"schemaPrefix": req.SchemaPrefix,
		"credentials":  items,
	}, "ok"))
}
