package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== VC 全流程工具(POST /api/tool/vc-flow) ====================
//
// 给定证书颁发者(issuer)与个人用户(user)两方的私钥,自动完成:
//
//	[1] 双方领水(余额充足则跳过;24h 冷却但余额够也放行)
//	[2] issuer 创建 Organization 型 DID(identity.Create)
//	[3] issuer 注册组织角色 VcIssuer + 声明凭证 schema(identity.RegisterOrganization)
//	[4] user 创建 Personal 型 DID
//	[5] issuer 链下签发 N 张键值对凭证(schema 名 prefix+序号,缺省 Test1~Test5)
//	[6] user 逐张披露上链(identity.DiscloseVcAttestation)
//	[7] view 回读验证(DisclosedVcs + HasValidVcFromIssuer)
//
// 全流程同步执行,每笔交易提交后等待确认。所有步骤幂等:DID 已创建 /
// 组织已注册 / 凭证已披露时自动跳过,可直接重跑。

const (
	vcFlowDefaultCount = 5                 // 缺省签发凭证张数
	vcFlowMaxCount     = 20                // 单次最多签发张数
	vcFlowGasThreshold = 100 * 1_000_000   // 余额 ≥ 100 MIL 视为无需领水
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
}

// validateVcFlowRequest 校验请求体必填约束(纯函数,便于单测)。
// userAddress 必填:同一 32 字节私钥按 ed25519/secp256k1/bls12381 解释会派生
// 不同地址,不显式传地址时服务端只能默认按 Ed25519 解释,secp256k1 账户会被
// 静默派生成错误地址——显式地址配合曲线回退才能锁定正确公钥。
func validateVcFlowRequest(req vcFlowRequest) error {
	if strings.TrimSpace(req.IssuerPrivateKey) == "" {
		return fmt.Errorf("issuerPrivateKey is required")
	}
	if strings.TrimSpace(req.UserPrivateKey) == "" {
		return fmt.Errorf("userPrivateKey is required")
	}
	if strings.TrimSpace(req.UserAddress) == "" {
		return fmt.Errorf("userAddress is required(必填:32字节私钥在不同曲线下派生不同地址,请传账户生成时返回的地址,服务端按其自动匹配曲线)")
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

// vcFlowFaucetStep 领水步骤结果。
type vcFlowFaucetStep struct {
	Skipped       bool   `json:"skipped"`
	Claimed       bool   `json:"claimed"`
	TxHash        string `json:"txHash,omitempty"`
	BalanceBefore string `json:"balanceBefore"`
	BalanceAfter  string `json:"balanceAfter"`
	Detail        string `json:"detail,omitempty"`
}

// vcFlowStep 一步链上写操作的幂等执行结果。
type vcFlowStep struct {
	Skipped bool   `json:"skipped"`
	TxHash  string `json:"txHash,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// vcFlowPartyResult 单个参与方(issuer/user)的步骤明细。
type vcFlowPartyResult struct {
	Address       string          `json:"address"`
	Faucet        vcFlowFaucetStep `json:"faucet"`
	Did           vcFlowStep      `json:"did"`
	Organization  *vcFlowStep     `json:"organization,omitempty"` // 仅 issuer
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

		var vUntil *int64
		if validUntil != nil {
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

// resolveVcFlowParty 解析一方(issuer/user)身份:私钥必填。
// role 用于错误信息区分角色("issuer"/"user")——同一错误在两方身上措辞不同,
// 避免出现「user 传 FN-DSA-512 私钥缺公钥,报错却让补 issuerPublicKey」的误导。
// 32 字节经典私钥默认按 Ed25519 解释(与 /api/util/vc-attestation 一致);
// 显式传入的地址与 Ed25519 派生地址不一致时,自动尝试 secp256k1 / bls12381
// 曲线解释(同一 32 字节私钥在不同曲线下派生不同地址,以显式地址为准)。
// FN-DSA-512(1281 字节)私钥须额外传公钥(SDK 无法从签名密钥反推)。
func resolveVcFlowParty(role, privKey, pubKey, addrStr string) (crypto.SecretKeyer, *crypto.PublicKey, crypto.Address, error) {
	// FN-DSA-512 缺公钥的前置检查:resolveIssuerIdentity 的错误措辞固定为 issuer,
	// 这里按角色先行给出准确报错
	if fk, err := crypto.SecretKeyerFromStringRelaxed(privKey); err == nil && crypto.AsFnDsa512SecretKey(fk) != nil && strings.TrimSpace(pubKey) == "" {
		return nil, nil, crypto.Address{}, fmt.Errorf(
			"%sPublicKey is required when %s private key is FN-DSA-512(1281字节后量子私钥无法反推公钥,请传账户生成时返回的 %sPublicKey)",
			role, role, role)
	}

	sk, pub, addr, err := resolveIssuerIdentity(privKey, pubKey)
	if err != nil {
		// 其余错误措辞同样按角色改写(resolveIssuerIdentity 的文案均以 issuer 主语)
		rewritten := strings.ReplaceAll(err.Error(), "issuer", role)
		return nil, nil, crypto.Address{}, errors.New(rewritten)
	}
	explicit := strings.TrimSpace(addrStr)
	if explicit == "" {
		return sk, pub, *addr, nil
	}
	want, perr := types.ParseAddress(explicit)
	if perr != nil {
		return nil, nil, crypto.Address{}, fmt.Errorf("invalid address %q: %w", explicit, perr)
	}
	if want.ToBase58() == addr.ToBase58() {
		return sk, pub, *addr, nil
	}

	// 曲线回退:经典私钥换 secp256k1 / bls12381 解释再比对
	if ck := crypto.AsClassicalSecretKey(sk); ck != nil {
		blsDerive := func() (*crypto.PublicKey, error) { return ck.BLS12381Public(), nil }
		for _, derive := range []func() (*crypto.PublicKey, error){ck.Secp256k1Public, blsDerive} {
			altPub, derr := derive()
			if derr != nil {
				continue
			}
			altAddr, aerr := crypto.NewAddressFromPublicKey(altPub)
			if aerr != nil {
				continue
			}
			if altAddr.ToBase58() == want.ToBase58() {
				return sk, altPub, *altAddr, nil
			}
		}
	}
	return nil, nil, crypto.Address{}, fmt.Errorf(
		"address %s does not match any address derived from the private key (ed25519: %s, plus secp256k1/bls12381 variants)",
		explicit, addr.ToBase58())
}

// isTolerableError 判断链端错误是否属于幂等容忍范围(错误名或错误码子串匹配)。
func isTolerableError(errMsg string, tolerable ...string) bool {
	if errMsg == "" {
		return false
	}
	for _, sub := range tolerable {
		if strings.Contains(errMsg, sub) {
			return true
		}
	}
	return false
}

// isAccountNotFoundErr 判断是否为「账户不存在」错误(code=512):
// 全新账户首次上链前查余额的预期错误,应视为余额 0 而非失败。
func isAccountNotFoundErr(msg string) bool {
	return strings.Contains(msg, "code=512") ||
		strings.Contains(msg, "账户不存在") ||
		strings.Contains(msg, "account not exist")
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
	if err := validateVcFlowRequest(req); err != nil {
		logParamError(c, "VcFlow", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	issuerSK, issuerPub, issuerAddr, err := resolveVcFlowParty("issuer", req.IssuerPrivateKey, req.IssuerPublicKey, req.IssuerAddress)
	if err != nil {
		logParamError(c, "VcFlow", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid issuer identity: "+err.Error(), nil))
		return
	}
	userSK, userPub, userAddr, err := resolveVcFlowParty("user", req.UserPrivateKey, req.UserPublicKey, req.UserAddress)
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
	if resp.Issuer.Faucet, err = vcFlowEnsureFaucet(mc, issuerAddr, issuerSK, issuerPub); err != nil {
		fail("issuer faucet", err)
		return
	}
	if resp.User.Faucet, err = vcFlowEnsureFaucet(mc, userAddr, userSK, userPub); err != nil {
		fail("user faucet", err)
		return
	}

	// 凭证 schema 列表(RegisterOrganization 声明 + 签发共用)
	schemas := make([]string, 0, req.CredentialCount)
	for i := 1; i <= req.CredentialCount; i++ {
		schemas = append(schemas, fmt.Sprintf("%s%d", req.CredentialPrefix, i))
	}

	// [2] issuer 建 Organization 型 DID
	if resp.Issuer.Did, err = vcFlowEnsureDID(mc, issuerAddr, issuerSK, issuerPub, "Organization"); err != nil {
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
	// [4] user 建 Personal 型 DID
	if resp.User.Did, err = vcFlowEnsureDID(mc, userAddr, userSK, userPub, "Personal"); err != nil {
		fail("user did create", err)
		return
	}

	// [5] 链下签发 N 张键值对凭证
	chainID, err := vcFlowChainID(mc)
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

// vcFlowChainID 从链头取 chain ID(凭证签名摘要的组成部分)。
func vcFlowChainID(mc *milon.Client) (int, error) {
	result, err := mc.GetChainHead(milon.WithRequestID(lib.RequestID(time.Now().UnixMilli())))
	if err != nil {
		return 0, fmt.Errorf("failed to get chain head: %w", err)
	}
	return int(result.BodyChainHead.ChainId), nil
}

// vcFlowCallView 执行一次 view 调用并解码。返回值可能是链端 Err 载荷
// (*api.TxFailurePayload),用 vcFlowViewOK 区分。
func vcFlowCallView(mc *milon.Client, appName, method string, args provider.Args) (any, error) {
	pd, ok := mc.GetAllPd()[appName]
	if !ok {
		return nil, fmt.Errorf("IDL app %q not found", appName)
	}
	wire, err := encodeWithCoercion(pd, method, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode view %s.%s: %w", appName, method, err)
	}
	result, err := mc.View([]api.PackedInstruction{wire}, milon.WithRequestID(lib.RequestID(time.Now().UnixMilli())))
	if err != nil {
		return nil, fmt.Errorf("view %s.%s failed: %w", appName, method, err)
	}
	value, err := pd.DecodeViewData(method, result.HTTPResponseBody)
	if err != nil {
		return nil, fmt.Errorf("failed to decode view %s.%s: %w", appName, method, err)
	}
	return value, nil
}

// vcFlowViewOK 判断 view 解码值是否为 Ok 载荷(而非链端 Err 载荷)。
func vcFlowViewOK(value any) bool {
	_, isFailure := value.(*api.TxFailurePayload)
	return !isFailure
}

// vcFlowBuildTx 构建一笔「自付 gas + 签 ix0」的交易(等价 /api/write 的 unified_payer_all)。
// 签名模式按账户链上状态自动修正(pubkey → 签名者列表)。
func vcFlowBuildTx(mc *milon.Client, appName, method string, args provider.Args, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey) (*lib.Transaction, error) {
	pd, ok := mc.GetAllPd()[appName]
	if !ok {
		return nil, fmt.Errorf("IDL app %q not found", appName)
	}
	wire, err := encodeWithCoercion(pd, method, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s.%s: %w", appName, method, err)
	}
	mode, err := normalizeSignatureModeForAccount(mc, addr, lib.PubKeySignatureMode{PublicKey: *pub})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve signature mode for %s: %w", addr.ToBase58(), err)
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(&addr).
		AddIxAndPayerSig(addr, sk, 0, mode).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %s.%s: %w", appName, method, err)
	}
	return tx, nil
}

// vcFlowSendTx 提交交易并等待确认,返回 txHash;链端执行错误以 error 返回
// (txHash 一并返回,便于追踪已提交但执行失败的交易)。
func vcFlowSendTx(mc *milon.Client, tx *lib.Transaction) (string, error) {
	if err := mc.SubmitTx(tx, milon.WithRequestID(lib.RequestID(time.Now().UnixMilli()))); err != nil {
		return "", err
	}
	txHash := txHashHex(tx)
	if _, err := mc.WaitForTransaction(txHash, milon.WithWaitRequestID(lib.RequestID(1))); err != nil {
		return txHash, err
	}
	return txHash, nil
}

// vcFlowEnsureFaucet 确保地址有足够 gas:
//   - 余额 ≥ 阈值:跳过领水
//   - 领水失败(典型:24h 冷却)但余额 ≥ 阈值:放行并在 Detail 说明
//   - 领水成功:确认到账后返回
func vcFlowEnsureFaucet(mc *milon.Client, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey) (vcFlowFaucetStep, error) {
	step := vcFlowFaucetStep{}
	before, err := mc.BalanceOf(&addr)
	if err != nil {
		// 全新账户首次上链前链上无 Account 资源,查余额报「账户不存在」→ 视为余额 0
		if !isAccountNotFoundErr(err.Error()) {
			return step, fmt.Errorf("failed to query balance of %s: %w", addr.ToBase58(), err)
		}
		before = 0
	}
	step.BalanceBefore = fmt.Sprint(before)
	step.BalanceAfter = step.BalanceBefore

	if before >= vcFlowGasThreshold {
		step.Skipped = true
		step.Detail = "balance is sufficient, skip faucet"
		return step, nil
	}

	mode, err := normalizeSignatureModeForAccount(mc, addr, lib.PubKeySignatureMode{PublicKey: *pub})
	if err != nil {
		return step, fmt.Errorf("faucet signature mode: %w", err)
	}
	tx, err := buildClaimFaucetTx(mc, addr, sk, mode)
	if err != nil {
		return step, fmt.Errorf("build faucet tx: %w", err)
	}
	if err := submitClaimFaucetTx(mc, tx); err != nil {
		// 常见幂等场景:24h 内重复领水被拒,但余额足够 → 放行
		if after, berr := mc.BalanceOf(&addr); berr == nil && after >= vcFlowGasThreshold {
			step.Skipped = true
			step.BalanceAfter = fmt.Sprint(after)
			step.Detail = "faucet rejected but balance is sufficient: " + err.Error()
			return step, nil
		}
		return step, fmt.Errorf("claim faucet: %w", err)
	}

	txHash := txHashHex(tx)
	if _, err := mc.WaitForTransaction(txHash, milon.WithWaitRequestID(lib.RequestID(1))); err != nil {
		if after, berr := mc.BalanceOf(&addr); berr == nil && after >= vcFlowGasThreshold {
			step.Claimed = true
			step.TxHash = txHash
			step.BalanceAfter = fmt.Sprint(after)
			step.Detail = "faucet submitted, balance confirms funding (wait error ignored): " + err.Error()
			return step, nil
		}
		return step, fmt.Errorf("wait faucet tx %s: %w", txHash, err)
	}

	after, err := mc.BalanceOf(&addr)
	if err != nil {
		return step, fmt.Errorf("re-query balance: %w", err)
	}
	step.Claimed = true
	step.TxHash = txHash
	step.BalanceAfter = fmt.Sprint(after)
	return step, nil
}

// vcFlowEnsureDID 确保地址已建 DID;已存在(view Document 可读)则跳过,
// 链端报 DidAlreadyExists(1024)亦视为已存在。
func vcFlowEnsureDID(mc *milon.Client, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey, subjectType string) (vcFlowStep, error) {
	step := vcFlowStep{}
	if value, err := vcFlowCallView(mc, vcFlowIdentityApp, "Document", provider.Args{"subject": addr.ToBase58()}); err == nil && vcFlowViewOK(value) {
		step.Skipped = true
		step.Detail = "DID document already exists"
		return step, nil
	}

	// 注意:struct 参数必须用 map[string]any 而非 gin.H ——
	// args_coerce 的 struct 分支按精确类型 map[string]any 断言,自定义 map 类型会失败
	args := provider.Args{
		"subject": addr.ToBase58(),
		"doc": map[string]any{
			"subject_type": subjectType,
			"keys":         []any{map[string]any{"public_key": pub.ToBase58(), "label": "primary"}},
			"services":     []any{},
			"avatar_uri":   vcFlowAvatarURI,
		},
	}
	tx, err := vcFlowBuildTx(mc, vcFlowIdentityApp, "Create", args, addr, sk, pub)
	if err != nil {
		return step, err
	}
	txHash, err := vcFlowSendTx(mc, tx)
	if err != nil {
		if isTolerableError(err.Error(), "DidAlreadyExists", "1024") {
			step.Skipped = true
			step.Detail = "DID already exists on chain: " + err.Error()
			return step, nil
		}
		return step, err
	}
	step.TxHash = txHash
	return step, nil
}

// vcFlowEnsureOrganization 确保 issuer 已注册组织(VcIssuer 角色 + 声明 schema);
// 已注册(view OrganizationCapabilities 可读)则跳过,链端报
// OrganizationAlreadyExists(1032)亦视为已注册。
func vcFlowEnsureOrganization(mc *milon.Client, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey, schemas []string) (vcFlowStep, error) {
	step := vcFlowStep{}
	if value, err := vcFlowCallView(mc, vcFlowIdentityApp, "OrganizationCapabilities", provider.Args{"subject": addr.ToBase58()}); err == nil && vcFlowViewOK(value) {
		step.Skipped = true
		step.Detail = "organization already registered"
		return step, nil
	}

	args := provider.Args{
		"subject":             addr.ToBase58(),
		"roles":               []string{"VcIssuer"},
		"credential_schemas":  schemas,
	}
	tx, err := vcFlowBuildTx(mc, vcFlowIdentityApp, "RegisterOrganization", args, addr, sk, pub)
	if err != nil {
		return step, err
	}
	txHash, err := vcFlowSendTx(mc, tx)
	if err != nil {
		if isTolerableError(err.Error(), "OrganizationAlreadyExists", "1032") {
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
	if value, verr := vcFlowCallView(mc, vcFlowIdentityApp, "VcAttestationCore", coreArgs); verr == nil && vcFlowViewOK(value) {
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
	tx, err := vcFlowBuildTx(mc, vcFlowIdentityApp, "DiscloseVcAttestation", args, userAddr, userSK, userPub)
	if err != nil {
		return false, false, "", err
	}
	txHash, err = vcFlowSendTx(mc, tx)
	if err != nil {
		if isTolerableError(err.Error(), "VcAttestationAlreadyExists", "1072") {
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
	if value, err := vcFlowCallView(mc, vcFlowIdentityApp, "DisclosedVcs", provider.Args{
		"subject": userAddr.ToBase58(),
		"offset":  0,
		"limit":   100,
	}); err == nil && vcFlowViewOK(value) {
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
		if value, err := vcFlowCallView(mc, vcFlowIdentityApp, "HasValidVcFromIssuer", provider.Args{
			"subject":            userAddr.ToBase58(),
			"issuer":             issuerAddr.ToBase58(),
			"credential_schema":  cred.Schema,
		}); err == nil && vcFlowViewOK(value) {
			if valid, ok := value.(bool); ok {
				item.Valid = valid
			}
		}
		items = append(items, item)
	}
	return items
}
