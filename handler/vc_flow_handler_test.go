package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
)

// 与 vc_attestation_handler_test.go 一致的固定测试密钥。
const (
	vcFlowTestIssuerSKHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	vcFlowTestUserSKHex   = "202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
)

func vcFlowTestKeys(t *testing.T) (issuerSK crypto.SecretKeyer, issuerPub *crypto.PublicKey, issuerAddr *crypto.Address, userSK crypto.SecretKeyer, userPub *crypto.PublicKey, userAddr *crypto.Address) {
	t.Helper()
	issuerSK = &crypto.ClassicalSecretKey{}
	if err := issuerSK.(*crypto.ClassicalSecretKey).FromStringRelaxed(vcFlowTestIssuerSKHex); err != nil {
		t.Fatal(err)
	}
	issuerPub = issuerSK.(*crypto.ClassicalSecretKey).Ed25519Public()
	var err error
	issuerAddr, err = crypto.NewAddressFromPublicKey(issuerPub)
	if err != nil {
		t.Fatal(err)
	}

	userSK = &crypto.ClassicalSecretKey{}
	if err := userSK.(*crypto.ClassicalSecretKey).FromStringRelaxed(vcFlowTestUserSKHex); err != nil {
		t.Fatal(err)
	}
	userPub = userSK.(*crypto.ClassicalSecretKey).Ed25519Public()
	userAddr, err = crypto.NewAddressFromPublicKey(userPub)
	if err != nil {
		t.Fatal(err)
	}
	return
}

// TestBuildVcFlowCredentials 验证 5 张键值对凭证的生成:
// schema 命名(prefix+序号)、凭证内容(名称/等级/双方 DID)、
// credential_hash 独立重算、issuer 签名可验签。
func TestBuildVcFlowCredentials(t *testing.T) {
	issuerSK, issuerPub, issuerAddr, _, _, userAddr := vcFlowTestKeys(t)

	creds, err := buildVcFlowCredentials(900_000_001, *issuerAddr, *userAddr, issuerSK, issuerPub, "Test", 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 5 {
		t.Fatalf("expected 5 credentials, got %d", len(creds))
	}

	for i, cred := range creds {
		wantSchema := fmt.Sprintf("Test%d", i+1)
		if cred.Schema != wantSchema {
			t.Errorf("cred[%d].Schema = %q, want %q", i, cred.Schema, wantSchema)
		}
		if !strings.Contains(cred.CredentialJSON, `"name":"`+wantSchema+`"`) {
			t.Errorf("cred[%d].CredentialJSON missing name %q: %s", i, wantSchema, cred.CredentialJSON)
		}
		if !strings.Contains(cred.CredentialJSON, fmt.Sprintf(`"level":%d`, i+1)) {
			t.Errorf("cred[%d].CredentialJSON missing level %d: %s", i, i+1, cred.CredentialJSON)
		}
		if !strings.Contains(cred.CredentialJSON, issuerAddr.ToBase58()) || !strings.Contains(cred.CredentialJSON, userAddr.ToBase58()) {
			t.Errorf("cred[%d].CredentialJSON missing issuer/user DID address: %s", i, cred.CredentialJSON)
		}

		// credential_hash 必须是凭证 JSON 的 sha256
		sum := sha256.Sum256([]byte(cred.CredentialJSON))
		wantHash := "0x" + hex.EncodeToString(sum[:])
		if !strings.EqualFold(cred.CredentialHash, wantHash) {
			t.Errorf("cred[%d].CredentialHash = %s, want %s", i, cred.CredentialHash, wantHash)
		}

		// issuer 签名必须能对 DiscloseVcAttestation 摘要验签通过
		sigBytes, err := hex.DecodeString(strings.TrimPrefix(cred.IssuerSignature, "0x"))
		if err != nil {
			t.Fatalf("cred[%d]: decode signature: %v", i, err)
		}
		digest := vcAttestationDigest(900_000_001, userAddr.Bytes[:], issuerAddr.Bytes[:], 0, cred.Schema, sum[:], nil)
		sig := &crypto.Signature{Variant: crypto.SignatureTypeEd25519, Bytes: sigBytes}
		if err := sig.Verify(digest[:], issuerPub); err != nil {
			t.Errorf("cred[%d]: signature verification failed: %v", i, err)
		}
	}
}

// TestBuildVcFlowCredentialsValidUntil 带有效期的凭证:摘要须包含 expiry,
// 生成的 ValidUntilMs 原样透传。
func TestBuildVcFlowCredentialsValidUntil(t *testing.T) {
	issuerSK, issuerPub, issuerAddr, _, _, userAddr := vcFlowTestKeys(t)
	validUntil := int64(1_900_000_000_000)

	creds, err := buildVcFlowCredentials(900_000_001, *issuerAddr, *userAddr, issuerSK, issuerPub, "T", 1, &validUntil)
	if err != nil {
		t.Fatal(err)
	}
	if creds[0].ValidUntilMs == nil || *creds[0].ValidUntilMs != validUntil {
		t.Fatalf("ValidUntilMs = %v, want %d", creds[0].ValidUntilMs, validUntil)
	}

	sum := sha256.Sum256([]byte(creds[0].CredentialJSON))
	sigBytes, _ := hex.DecodeString(creds[0].IssuerSignature)
	digest := vcAttestationDigest(900_000_001, userAddr.Bytes[:], issuerAddr.Bytes[:], 0, creds[0].Schema, sum[:], &validUntil)
	sig := &crypto.Signature{Variant: crypto.SignatureTypeEd25519, Bytes: sigBytes}
	if err := sig.Verify(digest[:], issuerPub); err != nil {
		t.Errorf("signature with expiry verification failed: %v", err)
	}
}

// TestBuildVcFlowCredentialsZeroMeansNoExpiry validUntilMs=0 与 /api/util/vc-attestation
// 语义对齐:0 表示不过期(摘要不含 expiry,输出 null),而非 1970 年的 Some(0)
// —— 链端拒绝披露已过期凭证(1067),Some(0) 必然失败。
func TestBuildVcFlowCredentialsZeroMeansNoExpiry(t *testing.T) {
	issuerSK, issuerPub, issuerAddr, _, _, userAddr := vcFlowTestKeys(t)
	zero := int64(0)

	creds, err := buildVcFlowCredentials(900_000_001, *issuerAddr, *userAddr, issuerSK, issuerPub, "Z", 1, &zero)
	if err != nil {
		t.Fatal(err)
	}
	if creds[0].ValidUntilMs != nil {
		t.Fatalf("ValidUntilMs should be nil for 0, got %d", *creds[0].ValidUntilMs)
	}

	sum := sha256.Sum256([]byte(creds[0].CredentialJSON))
	sigBytes, _ := hex.DecodeString(creds[0].IssuerSignature)
	// 摘要须按「不过期」(nil)计算——若按 Some(0) 计算,链上验签会失败
	digest := vcAttestationDigest(900_000_001, userAddr.Bytes[:], issuerAddr.Bytes[:], 0, creds[0].Schema, sum[:], nil)
	sig := &crypto.Signature{Variant: crypto.SignatureTypeEd25519, Bytes: sigBytes}
	if err := sig.Verify(digest[:], issuerPub); err != nil {
		t.Errorf("zero validUntil signature should verify against no-expiry digest: %v", err)
	}
}

// TestValidateVcFlowRequestValidUntil 已过期有效期必须在开工前拦下
// (链端拒绝披露过期凭证,错误 1067)。
func TestValidateVcFlowRequestValidUntil(t *testing.T) {
	now := int64(1_700_000_000_000) // 2023-11
	base := vcFlowRequest{
		IssuerPrivateKey: "aa",
		UserPrivateKey:   "bb",
		UserAddress:      "2MLJXUc5gMuV4L4UXNuQjMxaHWf6",
	}

	past := base
	pastMs := now - 1000
	past.ValidUntilMs = &pastMs
	if err := validateVcFlowRequest(past, now); err == nil || !strings.Contains(err.Error(), "1067") {
		t.Errorf("past validUntilMs should fail mentioning 1067, got: %v", err)
	}

	future := base
	futureMs := now + 1000
	future.ValidUntilMs = &futureMs
	if err := validateVcFlowRequest(future, now); err != nil {
		t.Errorf("future validUntilMs should pass, got: %v", err)
	}

	zero := base
	zeroMs := int64(0)
	zero.ValidUntilMs = &zeroMs
	if err := validateVcFlowRequest(zero, now); err != nil {
		t.Errorf("validUntilMs=0 means no-expiry and should pass, got: %v", err)
	}

	nilUntil := base
	nilUntil.ValidUntilMs = nil
	if err := validateVcFlowRequest(nilUntil, now); err != nil {
		t.Errorf("nil validUntilMs should pass, got: %v", err)
	}
}

// TestNormalizeVcFlowRequest prefix/count 缺省值与上限裁剪。
func TestNormalizeVcFlowRequest(t *testing.T) {
	got := normalizeVcFlowRequest(vcFlowRequest{})
	if got.CredentialPrefix != "Test" {
		t.Errorf("default prefix = %q, want Test", got.CredentialPrefix)
	}
	if got.CredentialCount != vcFlowDefaultCount {
		t.Errorf("default count = %d, want %d", got.CredentialCount, vcFlowDefaultCount)
	}

	got = normalizeVcFlowRequest(vcFlowRequest{CredentialPrefix: "Demo", CredentialCount: 3})
	if got.CredentialPrefix != "Demo" || got.CredentialCount != 3 {
		t.Errorf("explicit prefix/count not preserved: %+v", got)
	}

	got = normalizeVcFlowRequest(vcFlowRequest{CredentialCount: 99})
	if got.CredentialCount != vcFlowMaxCount {
		t.Errorf("count not clamped to max: %d", got.CredentialCount)
	}
	got = normalizeVcFlowRequest(vcFlowRequest{CredentialCount: -1})
	if got.CredentialCount != vcFlowDefaultCount {
		t.Errorf("negative count should fall back to default %d: %d", vcFlowDefaultCount, got.CredentialCount)
	}
}

// TestResolveVcFlowParty 身份解析:Ed25519 私钥推导地址、显式地址一致性校验。
func TestResolveVcFlowParty(t *testing.T) {
	_, _, issuerAddr, _, _, userAddr := vcFlowTestKeys(t)

	// 未显式传地址:由私钥推导
	_, _, addr, err := resolveVcFlowParty("issuer", vcFlowTestIssuerSKHex, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if addr.ToBase58() != issuerAddr.ToBase58() {
		t.Errorf("derived addr = %s, want %s", addr.ToBase58(), issuerAddr.ToBase58())
	}

	// 显式传地址且匹配
	_, _, addr, err = resolveVcFlowParty("user", vcFlowTestUserSKHex, "", userAddr.ToBase58())
	if err != nil {
		t.Fatal(err)
	}
	if addr.ToBase58() != userAddr.ToBase58() {
		t.Errorf("addr = %s, want %s", addr.ToBase58(), userAddr.ToBase58())
	}

	// 显式传地址但不匹配 → 报错
	if _, _, _, err := resolveVcFlowParty("issuer", vcFlowTestIssuerSKHex, "", userAddr.ToBase58()); err == nil {
		t.Error("expected error when explicit address mismatches derived address")
	}

	// 私钥缺失 → 报错
	if _, _, _, err := resolveVcFlowParty("user", "", "", ""); err == nil {
		t.Error("expected error when private key is empty")
	}
}

// TestResolveVcFlowPartyCurveFallback 同一 32 字节经典私钥在不同曲线下派生不同地址:
// 显式传入 secp256k1 派生地址时,应自动回退到 secp256k1 曲线解释并命中该地址。
func TestResolveVcFlowPartyCurveFallback(t *testing.T) {
	issuerSK := &crypto.ClassicalSecretKey{}
	if err := issuerSK.FromStringRelaxed(vcFlowTestIssuerSKHex); err != nil {
		t.Fatal(err)
	}

	secpPub, err := issuerSK.Secp256k1Public()
	if err != nil {
		t.Fatal(err)
	}
	secpAddr, err := crypto.NewAddressFromPublicKey(secpPub)
	if err != nil {
		t.Fatal(err)
	}

	sk, pub, addr, err := resolveVcFlowParty("issuer", vcFlowTestIssuerSKHex, "", secpAddr.ToBase58())
	if err != nil {
		t.Fatal(err)
	}
	if addr.ToBase58() != secpAddr.ToBase58() {
		t.Errorf("addr = %s, want secp256k1-derived %s", addr.ToBase58(), secpAddr.ToBase58())
	}
	if pub.Variant != crypto.PublicKeyTypeSecp256k1 {
		t.Errorf("pub variant = %d, want secp256k1(%d)", pub.Variant, crypto.PublicKeyTypeSecp256k1)
	}
	if sk == nil {
		t.Error("secret key should be returned alongside")
	}

	// 不传地址:默认 Ed25519 解释
	_, pub, _, err = resolveVcFlowParty("issuer", vcFlowTestIssuerSKHex, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if pub.Variant != crypto.PublicKeyTypeEd25519 {
		t.Errorf("default pub variant = %d, want ed25519(%d)", pub.Variant, crypto.PublicKeyTypeEd25519)
	}
}

// TestValidateVcFlowRequest 必填字段校验:userAddress 必填
// (32 字节私钥在不同曲线下派生不同地址,不显式传地址会默认按 Ed25519 解释)。
func TestValidateVcFlowRequest(t *testing.T) {
	full := vcFlowRequest{
		IssuerPrivateKey: "aa",
		UserPrivateKey:   "bb",
		UserAddress:      "2MLJXUc5gMuV4L4UXNuQjMxaHWf6",
	}
	if err := validateVcFlowRequest(full, 0); err != nil {
		t.Errorf("full request should pass, got: %v", err)
	}

	missingUserAddr := full
	missingUserAddr.UserAddress = ""
	err := validateVcFlowRequest(missingUserAddr, 0)
	if err == nil || !strings.Contains(err.Error(), "userAddress") {
		t.Errorf("missing userAddress should fail with clear message, got: %v", err)
	}

	noIssuer := full
	noIssuer.IssuerPrivateKey = ""
	if err := validateVcFlowRequest(noIssuer, 0); err == nil || !strings.Contains(err.Error(), "issuerPrivateKey") {
		t.Errorf("missing issuerPrivateKey should fail, got: %v", err)
	}

	noUser := full
	noUser.UserPrivateKey = ""
	if err := validateVcFlowRequest(noUser, 0); err == nil || !strings.Contains(err.Error(), "userPrivateKey") {
		t.Errorf("missing userPrivateKey should fail, got: %v", err)
	}

	blankAddr := full
	blankAddr.UserAddress = "   "
	if err := validateVcFlowRequest(blankAddr, 0); err == nil {
		t.Error("whitespace-only userAddress should fail")
	}
}

// TestResolveVcFlowPartyRoleAwareErrors 身份解析错误按角色区分:
// user 传 FN-DSA-512 私钥但缺公钥时,报错应说 userPublicKey 而非 issuerPublicKey。
func TestResolveVcFlowPartyRoleAwareErrors(t *testing.T) {
	// 1281 字节 FN-DSA-512 私钥(内容随意,只验长度分支)
	fnSk := strings.Repeat("ab", 1281)

	_, _, _, err := resolveVcFlowParty("user", fnSk, "", "")
	if err == nil {
		t.Fatal("expected error for FN-DSA-512 user key without public key")
	}
	msg := err.Error()
	if !strings.Contains(msg, "userPublicKey") {
		t.Errorf("error should mention userPublicKey, got: %s", msg)
	}
	if strings.Contains(msg, "issuerPublicKey is required") {
		t.Errorf("error should not blame issuerPublicKey for user role, got: %s", msg)
	}

	// issuer 角色同样场景:报错说 issuerPublicKey
	_, _, _, err = resolveVcFlowParty("issuer", fnSk, "", "")
	if err == nil || !strings.Contains(err.Error(), "issuerPublicKey") {
		t.Errorf("issuer role error should mention issuerPublicKey, got: %v", err)
	}

	// 私钥非法:报错带角色前缀
	_, _, _, err = resolveVcFlowParty("user", "not-a-key", "", "")
	if err == nil || !strings.Contains(err.Error(), "user") {
		t.Errorf("invalid user key error should mention user, got: %v", err)
	}
}

// TestIsAccountNotFoundErr 全新账户首次查余额的预期错误判定(code=512)。
func TestIsAccountNotFoundErr(t *testing.T) {
	yes := []string{
		`view BalanceOf failed: code=512 msg="账户不存在 (token: M11on…, account: 3DhN…)"`,
		"account not exist",
	}
	no := []string{
		"view BalanceOf failed: code=513 msg=\"other\"",
		"connection refused",
		"",
	}
	for _, m := range yes {
		if !isAccountNotFoundErr(m) {
			t.Errorf("isAccountNotFoundErr(%q) = false, want true", m)
		}
	}
	for _, m := range no {
		if isAccountNotFoundErr(m) {
			t.Errorf("isAccountNotFoundErr(%q) = true, want false", m)
		}
	}
}

// TestIsTolerableVcFlowError 幂等容忍错误判定(链端错误名/错误码)。
// 注意 identity 错误码语义:1072=VcAttestationAlreadyExists(容忍);
// 1071=VcIssuerKeyNotFound(签发方密钥问题,不容忍)。
func TestIsTolerableVcFlowError(t *testing.T) {
	cases := []struct {
		err  string
		want bool
	}{
		{"status 6: {Message:did already exists; code 1024}", true},
		{"execution failed: DidAlreadyExists", true},
		{"tx failed: OrganizationAlreadyExists (1032)", true},
		{"VcAttestationAlreadyExists: 1072", true},
		{"VcIssuerKeyNotFound: issuer key 0 (1071)", false},
		{"faucet cooldown: 24h", false},
		{"insufficient balance", false},
		{"", false},
	}
	tolerable := []string{"DidAlreadyExists", "OrganizationAlreadyExists", "VcAttestationAlreadyExists", "1024", "1032", "1072"}
	for _, tc := range cases {
		if got := isTolerableError(tc.err, tolerable...); got != tc.want {
			t.Errorf("isTolerableError(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
