package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== VC 全流程工具(POST /api/tool/vc-flow) ====================
//
// 给定证书颁发者(issuer)与个人用户(user)两方的私钥,自动完成:
//
//	[1] 双方领水(余额充足则跳过;24h 冷却但余额够也放行)
//	[2] issuer 创建 Organization 型 DID(identity.Create/CreateWithAlias,可透传别名/服务/头像)
//	[3] issuer 注册组织角色 VcIssuer + 声明凭证 schema(identity.RegisterOrganization)
//	[4] user 创建 Personal 型 DID
//	[5] issuer 链下签发 N 张键值对凭证(schema 名 prefix+序号,缺省 Test1~Test5)
//	[6] user 逐张披露上链(identity.DiscloseVcAttestation)
//	[7] view 回读验证(DisclosedVcs + HasValidVcFromIssuer)
//
// 全流程同步执行,每笔交易提交后等待确认。所有步骤幂等:DID 已创建 /
// 组织已注册 / 凭证已披露时自动跳过,可直接重跑。

const (
	vcFlowDefaultCount = 5               // 缺省签发凭证张数
	vcFlowMaxCount     = 20              // 单次最多签发张数
	vcFlowAvatarURI    = "https://milon.example/avatar.png"
	vcFlowIdentityApp  = "identity"
)

// vcFlowRequest 是 POST /api/tool/vc-flow 的请求体。
type vcFlowRequest struct {
	IssuerPrivateKey string `json:"issuerPrivateKey"` // 颁发者私钥(hex/base58),必填
	IssuerPublicKey  string `json:"issuerPublicKey"`  // 颁发者公钥;FN-DSA-512 私钥时必填
	IssuerAddress    string `json:"issuerAddress"`    // 可选;显式传入时须与私钥派生地址一致
	UserPrivateKey   string `json:"userPrivateKey"`   // 个人用户私钥,必填
	UserPublicKey    string `json:"userPublicKey"`    // 用户公钥;FN-DSA-512 私钥时必填
	UserAddress      string `json:"userAddress"`      // 必填;32字节私钥在不同曲线下派生不同地址,显式地址用于锁定正确公钥
	CredentialPrefix string `json:"credentialPrefix"` // 凭证 schema 前缀,缺省 Test
	CredentialCount  int    `json:"credentialCount"`  // 凭证张数,缺省 5,上限 20
	ValidUntilMs     *int64 `json:"validUntilMs"`     // 凭证有效期毫秒时间戳;null=永久
	IssuerDid        *vcFlowDidOptions `json:"issuerDid"` // 可选;issuer DID 的别名/服务/头像透传
	UserDid          *vcFlowDidOptions `json:"userDid"`   // 可选;user DID 的别名/服务/头像透传
}

// validateVcFlowRequest 校验请求体必填约束与有效期(纯函数,便于单测)。
// userAddress 必填:同一 32 字节私钥按 ed25519/secp256k1/bls12381 解释会派生
// 不同地址,不显式传地址时服务端只能默认按 Ed25519 解释,secp256k1 账户会被
// 静默派生成错误地址——显式地址配合曲线回退才能锁定正确公钥。
// validUntilMs 必须是未来毫秒时间戳:链端拒绝披露已过期凭证(错误 1067
// "Only a currently valid VC attestation can be accepted"),提前拦截避免
// 白跑领水/DID/组织注册等链上步骤;0 与 null 同义(不过期,对齐
// /api/util/vc-attestation 的语义)。
func validateVcFlowRequest(req vcFlowRequest, nowMs int64) error {
	if strings.TrimSpace(req.IssuerPrivateKey) == "" {
		return fmt.Errorf("issuerPrivateKey is required")
	}
	if strings.TrimSpace(req.UserPrivateKey) == "" {
		return fmt.Errorf("userPrivateKey is required")
	}
	if strings.TrimSpace(req.UserAddress) == "" {
		return fmt.Errorf("userAddress is required(必填:32字节私钥在不同曲线下派生不同地址,请传账户生成时返回的地址,服务端按其自动匹配曲线)")
	}
	if req.ValidUntilMs != nil && *req.ValidUntilMs != 0 && *req.ValidUntilMs <= nowMs {
		return fmt.Errorf("validUntilMs 已过期(%d):链端拒绝披露已过期的凭证(错误 1067 Only a currently valid VC attestation can be accepted),请传未来的毫秒时间戳;null/0 表示永久有效", *req.ValidUntilMs)
	}
	return nil
}

// normalizeVcFlowRequest 填充缺省值并裁剪越界值。
func normalizeVcFlowRequest(req vcFlowRequest) vcFlowRequest {
	if strings.TrimSpace(req.CredentialPrefix) == "" {
		req.CredentialPrefix = "Test"
	}
	if req.CredentialCount <= 0 {
		req.CredentialCount = vcFlowDefaultCount
	}
	if req.CredentialCount > vcFlowMaxCount {
		req.CredentialCount = vcFlowMaxCount
	}
	return req
}

// vcFlowCredential 一张链下签发完成的键值对凭证。
type vcFlowCredential struct {
	Index           int    `json:"index"`
	Schema          string `json:"schema"`
	CredentialJSON  string `json:"credentialJson"`
	CredentialHash  string `json:"credentialHash"`   // 0x 前缀 64 位 hex(sha256)
	ValidUntilMs    *int64 `json:"validUntilMs"`    // null=永久
	IssuerSignature string `json:"issuerSignature"` // hex
}

// vcFlowPartyResult 单个参与方(issuer/user)的步骤明细。
type vcFlowPartyResult struct {
	Address      string          `json:"address"`
	Faucet       flowFaucetStep  `json:"faucet"`
	Did          flowStep        `json:"did"`
	Organization *flowStep       `json:"organization,omitempty"` // 仅 issuer
}

// vcFlowCredentialResult 单张凭证的签发 + 披露结果。
type vcFlowCredentialResult struct {
	vcFlowCredential
	AlreadyDisclosed bool   `json:"alreadyDisclosed"` // 幂等跳过或链端已存在
	Disclosed        bool   `json:"disclosed"`        // 链上已有该凭证披露
	DiscloseTxHash   string `json:"discloseTxHash,omitempty"`
}

// vcFlowVerifyItem 单张凭证的回读验证结果。
type vcFlowVerifyItem struct {
	Schema  string `json:"schema"`
	OnChain bool   `json:"onChain"` // 出现在 DisclosedVcs 列表
	Valid   bool   `json:"valid"`   // HasValidVcFromIssuer 为 true
}

// vcFlowResponse 全流程结果明细。
type vcFlowResponse struct {
	Issuer       vcFlowPartyResult        `json:"issuer"`
	User         vcFlowPartyResult        `json:"user"`
	Credentials  []vcFlowCredentialResult `json:"credentials"`
	Verification []vcFlowVerifyItem       `json:"verification"`
}

// buildVcFlowCredentials 为 subject 批量签发 count 张键值对凭证(纯链下计算):
// schema 名为 prefix+序号;凭证内容为 {"name": schema, "level": 序号} 的键值对 JSON;
// issuer 用 DiscloseVcAttestation 摘要逐张签名。
func buildVcFlowCredentials(chainID int, issuerAddr, subjectAddr crypto.Address, issuerSK crypto.SecretKeyer, issuerPub *crypto.PublicKey, prefix string, count int, validUntil *int64) ([]vcFlowCredential, error) {
	creds := make([]vcFlowCredential, 0, count)
	for i := 1; i <= count; i++ {
		schema := fmt.Sprintf("%s%d", prefix, i)
		credentialJSON := fmt.Sprintf(
			`{"credentialSubject":{"id":"did:milon:%s","name":"%s","level":%d},"issuer":"did:milon:%s","type":["VerifiableCredential","%s"]}`,
			subjectAddr.ToBase58(), schema, i, issuerAddr.ToBase58(), schema)
		hash := sha256.Sum256([]byte(credentialJSON))

		// 0 与 null 同义 = 不过期(对齐 /api/util/vc-attestation 语义);
		// Some(0) 是 1970 年的过期时间,链端会以 1067 拒绝披露
		var vUntil *int64
		if validUntil != nil && *validUntil != 0 {
			v := *validUntil
			vUntil = &v
		}
		digest := vcAttestationDigest(chainID, subjectAddr.Bytes[:], issuerAddr.Bytes[:], 0, schema, hash[:], vUntil)
		sig, err := signVcAttestationDigest(issuerSK, issuerPub, digest[:])
		if err != nil {
			return nil, fmt.Errorf("sign credential %s: %w", schema, err)
		}

		creds = append(creds, vcFlowCredential{
			Index:           i,
			Schema:          schema,
			CredentialJSON:  credentialJSON,
			CredentialHash:  "0x" + hex.EncodeToString(hash[:]),
			ValidUntilMs:    vUntil,
			IssuerSignature: hex.EncodeToString(sig.Bytes),
		})
	}
	return creds, nil
}

// VcFlowHandler 提供 VC 签发-披露全流程工具。
type VcFlowHandler struct {
	nm *client.NetworkManager
}

// NewVcFlowHandler 创建绑定到 NetworkManager 的 VcFlowHandler。
func NewVcFlowHandler(nm *client.NetworkManager) *VcFlowHandler {
	return &VcFlowHandler{nm: nm}
}

// VcFlow 处理 POST /api/tool/vc-flow,同步执行全流程并返回每步明细。
func (h *VcFlowHandler) VcFlow(c *gin.Context) {
	var req vcFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "VcFlow", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}
	req = normalizeVcFlowRequest(req)
	if err := validateVcFlowRequest(req, time.Now().UnixMilli()); err != nil {
		logParamError(c, "VcFlow", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	issuerSK, issuerPub, issuerAddr, err := resolveFlowParty("issuer", req.IssuerPrivateKey, req.IssuerPublicKey, req.IssuerAddress)
	if err != nil {
		logParamError(c, "VcFlow", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid issuer identity: "+err.Error(), nil))
		return
	}
	userSK, userPub, userAddr, err := resolveFlowParty("user", req.UserPrivateKey, req.UserPublicKey, req.UserAddress)
	if err != nil {
		logParamError(c, "VcFlow", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid user identity: "+err.Error(), nil))
		return
	}
	if issuerAddr.ToBase58() == userAddr.ToBase58() {
		logParamError(c, "VcFlow", fmt.Errorf("issuer and user must be different addresses"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "issuer 和 user 不能是同一个地址", nil))
		return
	}
	// VC 凭证签名算法仅支持 Ed25519 / FN-DSA-512(见 signVcAttestationDigest),
	// issuer 公钥落在其他曲线(如 secp256k1)时提前给出明确报错
	if issuerPub.Variant != crypto.PublicKeyTypeEd25519 && !issuerPub.IsFnDsa512() {
		logParamError(c, "VcFlow", fmt.Errorf("issuer key type %d cannot sign VC credentials", issuerPub.Variant))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
			"issuer 私钥类型不支持 VC 凭证签名(仅支持 Ed25519 / FN-DSA-512);请改用 keyType=ed25519 的账户作为颁发者,或显式传入 ed25519 派生的 issuerAddress", nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	if mc == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "no current network", nil))
		return
	}

	resp := vcFlowResponse{
		Issuer:      vcFlowPartyResult{Address: issuerAddr.ToBase58()},
		User:        vcFlowPartyResult{Address: userAddr.ToBase58()},
		Credentials: []vcFlowCredentialResult{},
	}
	fail := func(stage string, err error) {
		logSDKError(c, "VcFlow", fmt.Errorf("%s: %w", stage, err))
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, stage+": "+err.Error(), resp))
	}

	// [1] 双方领水(幂等:余额充足即跳过)
	if resp.Issuer.Faucet, err = flowEnsureFaucet(mc, issuerAddr, issuerSK, issuerPub); err != nil {
		fail("issuer faucet", err)
		return
	}
	if resp.User.Faucet, err = flowEnsureFaucet(mc, userAddr, userSK, userPub); err != nil {
		fail("user faucet", err)
		return
	}

	// 凭证 schema 列表(RegisterOrganization 声明 + 签发共用)
	schemas := make([]string, 0, req.CredentialCount)
	for i := 1; i <= req.CredentialCount; i++ {
		schemas = append(schemas, fmt.Sprintf("%s%d", req.CredentialPrefix, i))
	}

	// [2] issuer 建 Organization 型 DID(可选透传别名/服务/头像)
	if resp.Issuer.Did, err = vcFlowEnsureDID(mc, issuerAddr, issuerSK, issuerPub, "Organization", req.IssuerDid); err != nil {
		fail("issuer did create", err)
		return
	}
	// [3] issuer 注册组织(VcIssuer 角色 + 声明 schema)
	{
		step, serr := vcFlowEnsureOrganization(mc, issuerAddr, issuerSK, issuerPub, schemas)
		resp.Issuer.Organization = &step
		if serr != nil {
			fail("issuer register organization", serr)
			return
		}
	}
	// [4] user 建 Personal 型 DID(可选透传别名/服务/头像)
	if resp.User.Did, err = vcFlowEnsureDID(mc, userAddr, userSK, userPub, "Personal", req.UserDid); err != nil {
		fail("user did create", err)
		return
	}

	// [5] 链下签发 N 张键值对凭证
	chainID, err := flowChainID(mc)
	if err != nil {
		fail("get chain id", err)
		return
	}
	creds, err := buildVcFlowCredentials(chainID, issuerAddr, userAddr, issuerSK, issuerPub, req.CredentialPrefix, req.CredentialCount, req.ValidUntilMs)
	if err != nil {
		fail("build credentials", err)
		return
	}

	// [6] user 逐张披露上链(幂等:已披露相同凭证则跳过)
	for _, cred := range creds {
		result := vcFlowCredentialResult{vcFlowCredential: cred}
		disclosed, already, txHash, derr := vcFlowDisclose(mc, userAddr, userSK, userPub, issuerAddr, cred)
		result.Disclosed = disclosed
		result.AlreadyDisclosed = already
		result.DiscloseTxHash = txHash
		resp.Credentials = append(resp.Credentials, result)
		if derr != nil {
			fail(fmt.Sprintf("disclose credential %s", cred.Schema), derr)
			return
		}
	}

	// [7] view 回读验证
	resp.Verification = vcFlowVerify(mc, userAddr, issuerAddr, creds)

	logBusinessInfo(c, "VcFlow",
		"issuer", resp.Issuer.Address, "user", resp.User.Address,
		"credentials", len(resp.Credentials))
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// ==================== 链上步骤辅助 ====================

// vcFlowEnsureDID 确保 issuer/user 已建「完整」DID:默认自动生成全局唯一别名
// (角色前缀 org-/user- + 地址片段 + 服务端代填数字后缀,撞名自动换号重试),
// 服务/头像经 issuerDid/userDid 透传;核心逻辑在 didEnsureDocument,
// 与 /api/tool/did/create 共用。结果汇总为单个 flowStep。
func vcFlowEnsureDID(mc *milon.Client, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey, subjectType string, opts *vcFlowDidOptions) (flowStep, error) {
	rolePrefix := "user"
	if subjectType == "Organization" {
		rolePrefix = "org"
	}
	if opts == nil {
		opts = &vcFlowDidOptions{}
	}
	if alias, bind := resolveVcFlowDidAlias(opts, rolePrefix, addr); bind {
		opts.Alias = alias
	}
	res, err := didEnsureDocument(mc, addr, sk, pub, subjectType, opts)
	if err != nil {
		return flowStep{}, err
	}
	return summarizeDidEnsure(res), nil
}

// vcFlowEnsureOrganization 确保 issuer 已注册组织(VcIssuer 角色 + 声明 schema);
// 已注册(view OrganizationCapabilities 可读)则跳过,链端报
// OrganizationAlreadyExists(1032)亦视为已注册。
func vcFlowEnsureOrganization(mc *milon.Client, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey, schemas []string) (flowStep, error) {
	step := flowStep{}
	if value, err := flowCallView(mc, vcFlowIdentityApp, "OrganizationCapabilities", provider.Args{"subject": addr.ToBase58()}); err == nil && flowViewOK(value) {
		step.Skipped = true
		step.Detail = "organization already registered"
		return step, nil
	}

	args := provider.Args{
		"subject":             addr.ToBase58(),
		"roles":               []string{"VcIssuer"},
		"credential_schemas":  schemas,
	}
	tx, err := flowBuildTx(mc, vcFlowIdentityApp, "RegisterOrganization", args, addr, sk, pub)
	if err != nil {
		return step, err
	}
	txHash, err := flowSendTx(mc, tx)
	if err != nil {
		if isTolerableChainError(err.Error(), "OrganizationAlreadyExists", "1032") {
			step.Skipped = true
			step.Detail = "organization already registered on chain: " + err.Error()
			return step, nil
		}
		return step, err
	}
	step.TxHash = txHash
	return step, nil
}

// vcFlowDisclose 由 user(subject)披露一张凭证。幂等:该 (subject, schema, issuer)
// 已有链上凭证(view VcAttestationCore 可读)则跳过;链端报
// VcAttestationAlreadyExists(1072)亦视为已披露。
func vcFlowDisclose(mc *milon.Client, userAddr crypto.Address, userSK crypto.SecretKeyer, userPub *crypto.PublicKey, issuerAddr crypto.Address, cred vcFlowCredential) (disclosed bool, already bool, txHash string, err error) {
	coreArgs := provider.Args{
		"subject":            userAddr.ToBase58(),
		"credential_schema":  cred.Schema,
		"issuer":             issuerAddr.ToBase58(),
	}
	if value, verr := flowCallView(mc, vcFlowIdentityApp, "VcAttestationCore", coreArgs); verr == nil && flowViewOK(value) {
		return true, true, "", nil
	}

	var validUntil any // option<u64>:nil → null(永久)
	if cred.ValidUntilMs != nil {
		validUntil = *cred.ValidUntilMs
	}
	args := provider.Args{
		"subject":            userAddr.ToBase58(),
		"issuer":             issuerAddr.ToBase58(),
		"issuer_key_id":      0,
		"credential_schema":  cred.Schema,
		"credential_hash":    cred.CredentialHash,
		"valid_until_ms":     validUntil,
		"issuer_signature":   cred.IssuerSignature,
	}
	tx, err := flowBuildTx(mc, vcFlowIdentityApp, "DiscloseVcAttestation", args, userAddr, userSK, userPub)
	if err != nil {
		return false, false, "", err
	}
	txHash, err = flowSendTx(mc, tx)
	if err != nil {
		if isTolerableChainError(err.Error(), "VcAttestationAlreadyExists", "1072") {
			return true, true, txHash, nil
		}
		return false, false, txHash, err
	}
	return true, false, txHash, nil
}

// vcFlowVerify 回读验证:DisclosedVcs 列表确认披露可见,
// HasValidVcFromIssuer 逐张确认当前有效。验证失败不中断流程,仅标记结果。
func vcFlowVerify(mc *milon.Client, userAddr, issuerAddr crypto.Address, creds []vcFlowCredential) []vcFlowVerifyItem {
	onChain := map[string]bool{}
	if value, err := flowCallView(mc, vcFlowIdentityApp, "DisclosedVcs", provider.Args{
		"subject": userAddr.ToBase58(),
		"offset":  0,
		"limit":   100,
	}); err == nil && flowViewOK(value) {
		if rows, ok := value.([]any); ok {
			for _, row := range rows {
				if m, ok := row.(map[string]any); ok {
					if schema, ok := m["credential_schema"].(string); ok {
						onChain[schema] = true
					}
				}
			}
		}
	}

	items := make([]vcFlowVerifyItem, 0, len(creds))
	for _, cred := range creds {
		item := vcFlowVerifyItem{Schema: cred.Schema, OnChain: onChain[cred.Schema]}
		if value, err := flowCallView(mc, vcFlowIdentityApp, "HasValidVcFromIssuer", provider.Args{
			"subject":            userAddr.ToBase58(),
			"issuer":             issuerAddr.ToBase58(),
			"credential_schema":  cred.Schema,
		}); err == nil && flowViewOK(value) {
			if valid, ok := value.(bool); ok {
				item.Valid = valid
			}
		}
		items = append(items, item)
	}
	return items
}
