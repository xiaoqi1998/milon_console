package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"milon-api-server/types"

	"github.com/btcsuite/btcutil/base58"
	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

// VC attestation 相关常量（与 Python scripts/generate_vc_attestation.py 一致，勿改）
const (
	// ATTESTATION_DOMAIN 凭证断言签名域
	attestationDomain = "milon.identity.vc-attestation.v1"
	// identityAppID DiscloseVcAttestation 的 app id
	identityAppID = 4
	// defaultChainID devNet 默认链 ID
	defaultChainID = 900_000_001
)

// VcAttestationHandler 生成 DiscloseVcAttestation 凭证参数（milon-vc-disclosure 包装格式）。
type VcAttestationHandler struct {
}

// NewVcAttestationHandler creates a VcAttestationHandler.
func NewVcAttestationHandler() *VcAttestationHandler {
	return &VcAttestationHandler{}
}

// generateVcAttestationRequest 是 GenerateVcAttestation 的请求体。
// 字段与 Python 脚本 generate_vc_attestation() 参数一一对应。
type generateVcAttestationRequest struct {
	IssuerPrivateKey   string `json:"issuerPrivateKey"`             // issuer 私钥（hex, 32 字节，必填）
	ChainID            *int64 `json:"chainId"`                      // 链 ID，缺省 900000001
	SubjectPrivateKey  string `json:"subjectPrivateKey"`            // subject 私钥（hex）—— 与 subjectAddress 二选一
	SubjectAddress     string `json:"subjectAddress"`               // subject 地址（bs58）—— 与 subjectPrivateKey 二选一
	IssuerKeyID        *int   `json:"issuerKeyId"`                  // issuer 密钥索引，缺省 0
	CredentialSchema   string `json:"credentialSchema"`             // 凭证 schema，缺省 KycLevelCredential
	CredentialJson     string `json:"credentialJson"`               // 凭证规范化 JSON 字符串（sha256 作为 credential_hash）
	ValidUntilMs       *int64 `json:"validUntilMs"`                 // 有效期毫秒时间戳；传 0 表示不过期（与 validUntil 二选一，显式传入优先）
	ValidUntil         string `json:"validUntil"`                   // 有效期 ISO8601（如 2027-08-24T00:00:00.000Z）
	CredentialName     string `json:"credentialName"`               // 凭证展示名称
	CredentialDesc     string `json:"credentialDesc"`               // 凭证描述
	IssuedAt           string `json:"issuedAt"`                     // 签发时间 ISO8601；缺省取当前 UTC
}

// generateVcAttestationResponse 是 GenerateVcAttestation 的响应体。
type generateVcAttestationResponse struct {
	Format     string         `json:"format"`
	Version    int            `json:"version"`
	Credential map[string]any `json:"credential"`
	Disclosure map[string]any `json:"disclosure"`
}

// GenerateVcAttestation handles POST /api/util/vc-attestation
func (h *VcAttestationHandler) GenerateVcAttestation(c *gin.Context) {
	var req generateVcAttestationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "GenerateVcAttestation", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	if strings.TrimSpace(req.IssuerPrivateKey) == "" {
		logParamError(c, "GenerateVcAttestation", fmt.Errorf("issuerPrivateKey is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "issuerPrivateKey is required", nil))
		return
	}

	if strings.TrimSpace(req.SubjectPrivateKey) == "" && strings.TrimSpace(req.SubjectAddress) == "" {
		logParamError(c, "GenerateVcAttestation", fmt.Errorf("subjectPrivateKey or subjectAddress is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "subjectPrivateKey 与 subjectAddress 必须提供其一", nil))
		return
	}

	// ---- 有效期：显式 validUntilMs > validUntil(ISO 推导) > 默认 1.9e12 ms ----
	validUntilMs := req.ValidUntilMs
	if validUntilMs == nil {
		if req.ValidUntil != "" {
			ms, err := isoToMs(req.ValidUntil)
			if err != nil {
				logParamError(c, "GenerateVcAttestation", err)
				c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid validUntil: "+err.Error(), nil))
				return
			}
			validUntilMs = &ms
		} else {
			def := int64(1_900_000_000_000)
			validUntilMs = &def
		}
	}

	chainID := defaultChainID
	if req.ChainID != nil {
		chainID = int(*req.ChainID)
	}

	issuerKeyID := 0
	if req.IssuerKeyID != nil {
		issuerKeyID = *req.IssuerKeyID
	}

	credentialSchema := strings.TrimSpace(req.CredentialSchema)
	if credentialSchema == "" {
		credentialSchema = "KycLevelCredential"
	}

	credentialJSON := req.CredentialJson
	if strings.TrimSpace(credentialJSON) == "" {
		credentialJSON = `{"credentialSubject":{"id":"did:milon:subject","kycLevel":2},"id":"urn:uuid:12345678-1234-5678-1234-567812345678","issuer":"did:milon:issuer","type":["VerifiableCredential","KycLevelCredential"]}`
	}

	credentialName := req.CredentialName
	if credentialName == "" {
		credentialName = "KycLevel Credential"
	}
	credentialDesc := req.CredentialDesc
	if credentialDesc == "" {
		credentialDesc = "A mock KYC credential for testing the DID web upload flow."
	}

	issuedAt := req.IssuedAt
	if strings.TrimSpace(issuedAt) == "" {
		issuedAt = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}

	// ---- issuer 密钥与地址 ----
	issuerSK := &crypto.ClassicalSecretKey{}
	if err := issuerSK.FromStringRelaxed(req.IssuerPrivateKey); err != nil {
		logSDKError(c, "GenerateVcAttestation", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid issuerPrivateKey: "+err.Error(), nil))
		return
	}
	issuerPub := issuerSK.Ed25519Public()
	issuerAddr, err := crypto.NewAddressFromPublicKey(issuerPub)
	if err != nil {
		logSDKError(c, "GenerateVcAttestation", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to derive issuer address: "+err.Error(), nil))
		return
	}

	// ---- subject 地址：私钥优先，否则 base58 解码地址 ----
	var subjectAddr *crypto.Address
	if strings.TrimSpace(req.SubjectPrivateKey) != "" {
		subjectSK := &crypto.ClassicalSecretKey{}
		if err := subjectSK.FromStringRelaxed(req.SubjectPrivateKey); err != nil {
			logSDKError(c, "GenerateVcAttestation", err)
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid subjectPrivateKey: "+err.Error(), nil))
			return
		}
		subjectAddr, err = crypto.NewAddressFromPublicKey(subjectSK.Ed25519Public())
		if err != nil {
			logSDKError(c, "GenerateVcAttestation", err)
			c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to derive subject address: "+err.Error(), nil))
			return
		}
	} else {
		decoded := base58.Decode(req.SubjectAddress)
		if len(decoded) != crypto.AddressRawLen {
			logParamError(c, "GenerateVcAttestation", fmt.Errorf("subject 地址解码后必须为 20 字节, got %d", len(decoded)))
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, fmt.Sprintf("subject 地址解码后必须为 20 字节, got %d", len(decoded)), nil))
			return
		}
		subjectAddr, err = crypto.NewAddressFromBytes(decoded)
		if err != nil {
			logSDKError(c, "GenerateVcAttestation", err)
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid subjectAddress: "+err.Error(), nil))
			return
		}
	}

	// ---- credential_hash = sha256(canonical_credential) ----
	credentialHash := sha256.Sum256([]byte(credentialJSON))

	// ---- 签名摘要（与 gosdk DiscloseVcAttestation digest 一致） ----
	signingDigest := vcAttestationDigest(
		chainID,
		subjectAddr.Bytes[:],
		issuerAddr.Bytes[:],
		issuerKeyID,
		credentialSchema,
		credentialHash[:],
		validUntilMs,
	)

	// ---- issuer ed25519 签名（raw R||S, 64 字节） ----
	sig := issuerSK.SignEd25519(signingDigest[:])
	issuerSignature := hex.EncodeToString(sig.Bytes)

	// ---- 本地验签（失败会返回错误） ----
	if err := sig.Verify(signingDigest[:], issuerPub); err != nil {
		logSDKError(c, "GenerateVcAttestation", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "local signature verification failed: "+err.Error(), nil))
		return
	}

	// ---- 组装 milon-vc-disclosure 包装格式 ----
	var validUntilISO *string
	if validUntilMs != nil {
		s := msToIso(*validUntilMs)
		validUntilISO = &s
	}

	credentialHashList := make([]int, len(credentialHash))
	for i, b := range credentialHash {
		credentialHashList[i] = int(b)
	}

	discloseArgs := map[string]any{
		"subject":           subjectAddr.ToBase58(),
		"issuer":            issuerAddr.ToBase58(),
		"issuer_key_id":     issuerKeyID,
		"credential_schema": credentialSchema,
		"credential_hash":   credentialHashList,
		"valid_until_ms":    validUntilMs,
		"issuer_signature":  issuerSignature,
	}

	resp := generateVcAttestationResponse{
		Format:  "milon-vc-disclosure",
		Version: 1,
		Credential: map[string]any{
			"name":        credentialName,
			"description": credentialDesc,
			"issued_at":   issuedAt,
			"valid_until": validUntilISO,
		},
		Disclosure: map[string]any{
			"app":    "Identity",
			"method": "DiscloseVcAttestation",
			"args":   discloseArgs,
		},
	}

	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// vcAttestationDigest 构造 DiscloseVcAttestation 签名摘要。
// 摘要 = Blake3(Milon-blake3 || milon.identity.vc-attestation.v1
//
//	|| u64le(chain_id) || 0x04 || subject || issuer || u8(issuer_key_id)
//	|| u16le(len(schema)) || schema || credential_hash
//	|| u8(has_expiry) || u64le(valid_until_ms or 0))
func vcAttestationDigest(
	chainID int,
	subject []byte,
	issuer []byte,
	issuerKeyID int,
	credentialSchema string,
	credentialHash []byte,
	validUntilMs *int64,
) [32]byte {
	schema := []byte(credentialSchema)
	hasExpiry := byte(0)
	var vUntil uint64
	if validUntilMs != nil {
		hasExpiry = 1
		vUntil = uint64(*validUntilMs)
	}
	return crypto.Hash32(
		[]byte(attestationDomain),
		u64le(uint64(chainID)),
		[]byte{byte(identityAppID)},
		subject,
		issuer,
		[]byte{byte(issuerKeyID)},
		u16le(len(schema)),
		schema,
		credentialHash,
		[]byte{hasExpiry},
		u64le(vUntil),
	)
}

// u16le 编码 16 位小端
func u16le(value int) []byte {
	return []byte{byte(value), byte(value >> 8)}
}

// u64le 编码 64 位小端
func u64le(value uint64) []byte {
	b := make([]byte, 8)
	for i := 0; i < 8; i++ {
		b[i] = byte(value >> (8 * i))
	}
	return b
}

// isoToMs 将 ISO8601 时间字符串（如 2027-08-24T00:00:00.000Z）转为毫秒时间戳。
// 兼容无时区（视为 UTC）与 "+00:00" 偏移格式。
func isoToMs(iso string) (int64, error) {
	s := strings.TrimSpace(iso)
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().UnixMilli(), nil
		}
	}
	return 0, fmt.Errorf("无法解析时间: %s", iso)
}

// msToIso 将毫秒时间戳转为 ISO8601 UTC 字符串（如 2027-08-24T00:00:00.000Z）。
func msToIso(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z")
}
