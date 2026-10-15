package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"
	"lukechampine.com/blake3"
)

// vcCredentialSchema 是合规演示中使用的凭证 schema。
// kycOrg 已在实体注册表演示中注册了它。
const vcCredentialSchema = "KycLevel2"

// vcAttestationSigningDigest 重建 compliance 合约用于验证 issuer 签名的
// 精确 32 字节摘要（参见 compliance.rs 中的 vc_attestation_signing_digest）：
//
//	hash32("milon.identity.vc-attestation.v1",
//	    chain_id_le(8) || app_id(1) || subject.digest(20) || issuer.digest(20) ||
//	    issuer_key_id(1) || schema_len_le(2) || schema ||
//	    credential_hash(32) || expiry_tag(1) || expiry_le(8))
func vcAttestationSigningDigest(subject, issuer *crypto.Address, issuerKeyID uint8, credentialSchema string, credentialHash [32]byte, validUntilMs *uint64) [32]byte {
	chainID := make([]byte, 8)
	binary.LittleEndian.PutUint64(chainID, lib.GetChainId())

	schemaLen := make([]byte, 2)
	binary.LittleEndian.PutUint16(schemaLen, uint16(len(credentialSchema)))

	expiryTag := byte(0) //validUntilMs 无值
	var expiryMs uint64
	if validUntilMs != nil {
		expiryTag = 1 //validUntilMs 有值
		expiryMs = *validUntilMs
	}

	expiry := make([]byte, 9)
	expiry[0] = expiryTag
	binary.LittleEndian.PutUint64(expiry[1:], expiryMs)

	return crypto.Hash32([]byte("milon.identity.vc-attestation.v1"),
		chainID,
		[]byte{0x04}, // identity 应用 id
		subject.Bytes[:],
		issuer.Bytes[:],
		[]byte{issuerKeyID},
		schemaLen,
		[]byte(credentialSchema),
		credentialHash[:],
		expiry,
	)
}

// demoCompliance 运行最小 VC 结果流程：bob 创建个人 DID，
// kycOrg 对一份 mock 的链下凭证签名，bob 将该凭证的哈希上链披露，
// 随后依次演示移除、恢复披露、冻结、解冻，最后 revoke。
// 覆盖 compliance 的全部 entry 和全部 view。
func demoCompliance(client *milon.Client, kycOrg, bob *account) {
	fmt.Printf("\n================ 10.Compliance: create subject DID ================\n")

	bobDoc := gen.IdentityDidDocumentInput{
		SubjectType: gen.IdentityDidSubjectType{Variant: "Personal"},
		Keys: []gen.IdentityDidKeyInput{
			{PublicKey: bob.pk, Label: strPtr("primary")},
		},
		AvatarUri: "https://bob.example.com/avatar.png",
	}
	submit(
		client,
		bob,
		"Create (personal DID)",
		gen.Identity.Create.Args(bob.addr, bobDoc).Encode,
	)

	fmt.Printf("\n================ 11.Compliance: disclose_vc_attestation ================\n")

	// mock VC 文档在链下哈希：链上永远不会看到文档本身
	credentialDocument := []byte(`{"type":"KycLevel2","subject":"Bob","country":"SG","verified":true}`)
	credentialHash := blake3.Sum256(credentialDocument)
	validUntilMs := uint64(time.Now().Add(72 * time.Hour).UnixMilli())

	// kycOrg 用其 DID key 0（在实体注册表演示中创建）对摘要签名；
	// disclose 只要求 issuer 拥有活跃 DID
	digest := vcAttestationSigningDigest(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs)
	issuerSignature := kycOrg.sk.SignEd25519(digest[:])

	submit(
		client,
		bob,
		"DiscloseVcAttestation",
		gen.Identity.DiscloseVcAttestation.Args(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs, issuerSignature).Encode,
	)

	//11.1.VcAttestationCore----永不变
	vcAttestationCore := view(client, "VcAttestationCore", gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if vcAttestationCore.CredentialSchema != vcCredentialSchema || vcAttestationCore.CredentialHash != credentialHash || vcAttestationCore.ValidUntilMs == nil || *vcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore: %+v", vcAttestationCore))
	}

	//11.2.VcAttestationLifecycle----永远存在（但状态可变）
	vcAttestationLifecycle := view(client, "VcAttestationLifecycle", gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if vcAttestationLifecycle.Status.Variant != "Active" {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle: %+v", vcAttestationLifecycle))
	}

	//11.3.AcceptedVcIssuerIndexMeta----永不变
	acceptedVcIssuerIndexMeta := view(client, "AcceptedVcIssuerIndexMeta", gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if acceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta: %q", acceptedVcIssuerIndexMeta))
	}

	//11.4.AcceptedVcIssuers----vc的删除和新增有变化
	acceptedVcIssuers := view(client, "AcceptedVcIssuers", gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if len(acceptedVcIssuers) != 1 || *acceptedVcIssuers[0].Issuer != *kycOrg.addr || acceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuers: %+v", acceptedVcIssuers))
	}

	//11.5.DisclosedVcSchemas----vc的删除和新增有变化
	disclosedVcSchemas := view(client, "DisclosedVcSchemas", gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if len(disclosedVcSchemas) != 1 || disclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas: %+v", disclosedVcSchemas))
	}

	//11.6.DisclosedVcs----vc的删除和新增有变化
	disclosedVcs := view(client, "DisclosedVcs", gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if len(disclosedVcs) != 1 || !disclosedVcs[0].IsValid || disclosedVcs[0].Status.Variant != "Active" || *disclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected disclosedVcs: %+v", disclosedVcs))
	}

	//11.7.HasValidVcFromIssuer----只和VcAttestationCore.valid_until_ms和VcAttestationLifecycle.status有关
	hasValidVcFromIssuer := view(client, "HasValidVcFromIssuer", gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if !hasValidVcFromIssuer {
		panic("bob should have a valid VC from kycOrg")
	}

	fmt.Printf("\n================ 12.Compliance: repeated disclose is idempotent ================\n")

	// 完全重复的 disclose_vc_attestation 会成功，且不得刷新生命周期时间戳
	submit(
		client,
		bob,
		"DiscloseVcAttestation (exact repeat)",
		gen.Identity.DiscloseVcAttestation.Args(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs, issuerSignature).Encode,
	)

	//12.1.VcAttestationCore----永不变
	repeatedVcAttestationCore := view(client, "VcAttestationCore (after repeat)", gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if repeatedVcAttestationCore.CredentialSchema != vcCredentialSchema || repeatedVcAttestationCore.CredentialHash != credentialHash || repeatedVcAttestationCore.ValidUntilMs == nil || *vcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after repeat): %+v", repeatedVcAttestationCore))
	}

	//12.2.VcAttestationLifecycle----永远存在（但状态可变）
	repeatVcAttestationLifecycle := view(client, "VcAttestationLifecycle (after repeat)", gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if repeatVcAttestationLifecycle.Status.Variant != "Active" || repeatVcAttestationLifecycle.UpdatedAtMs != vcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after repeat): %+v", repeatVcAttestationLifecycle))
	}

	//12.3.AcceptedVcIssuerIndexMeta----永不变
	repeatAcceptedVcIssuerIndexMeta := view(client, "AcceptedVcIssuerIndexMeta (after repeat)", gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if repeatAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after repeat): %q", repeatAcceptedVcIssuerIndexMeta))
	}

	//12.4.AcceptedVcIssuers----vc的删除和新增有变化（当前entry无这些操作）
	repeatAcceptedVcIssuers := view(client, "AcceptedVcIssuers (after repeat)", gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if len(repeatAcceptedVcIssuers) != 1 || *repeatAcceptedVcIssuers[0].Issuer != *kycOrg.addr || repeatAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after repeat): %+v", repeatAcceptedVcIssuers))
	}

	//12.5.DisclosedVcSchemas----vc的删除和新增有变化（当前entry无这些操作）
	repeatDisclosedVcSchemas := view(client, "DisclosedVcSchemas (after repeat)", gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if len(repeatDisclosedVcSchemas) != 1 || repeatDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after repeat): %+v", repeatDisclosedVcSchemas))
	}

	//12.6.DisclosedVcs----vc的删除和新增有变化（当前entry无这些操作）
	repeatDisclosedVcs := view(client, "DisclosedVcs (after repeat)", gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if len(repeatDisclosedVcs) != 1 || !repeatDisclosedVcs[0].IsValid || repeatDisclosedVcs[0].Status.Variant != "Active" || *repeatDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected disclosedVcs (after repeat): %+v", repeatDisclosedVcs))
	}

	//12.7.HasValidVcFromIssuer----只和VcAttestationCore.valid_until_ms和VcAttestationLifecycle.status有关
	repeatHasValidVcFromIssuer := view(client, "HasValidVcFromIssuer (after repeat)", gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if !repeatHasValidVcFromIssuer {
		panic("the underlying attestation should stay valid (after repeat)")
	}

	fmt.Printf("\n================ 13.Compliance: remove_vc_disclosure ================\n")

	// remove 只撤回公开的展示索引；
	// 底层的 attestation 记录仍保留在链上
	submit(
		client,
		bob,
		"RemoveVcDisclosure",
		gen.Identity.RemoveVcDisclosure.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode,
	)

	//13.1.VcAttestationCore----永不变
	removeVcAttestationCore := view(client, "VcAttestationCore (after remove)", gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if removeVcAttestationCore.CredentialSchema != vcCredentialSchema || removeVcAttestationCore.CredentialHash != credentialHash || removeVcAttestationCore.ValidUntilMs == nil || *removeVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after remove): %+v", removeVcAttestationCore))
	}

	//13.2.VcAttestationLifecycle----永远存在（但状态可变）
	removeVcAttestationLifecycle := view(client, "VcAttestationLifecycle (after remove)", gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if removeVcAttestationLifecycle.Status.Variant != "Active" || removeVcAttestationLifecycle.UpdatedAtMs != vcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after remove): %+v", removeVcAttestationLifecycle))
	}

	//13.3.AcceptedVcIssuerIndexMeta----永不变
	removeAcceptedVcIssuerIndexMeta := view(client, "AcceptedVcIssuerIndexMeta (after remove)", gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if removeAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after remove): %q", removeAcceptedVcIssuerIndexMeta))
	}

	//13.4.AcceptedVcIssuers----vc的删除和新增有变化（这里删除了）
	removeAcceptedVcIssuers := view(client, "AcceptedVcIssuers (after remove)", gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if len(removeAcceptedVcIssuers) != 0 {
		panic(fmt.Sprintf("expected empty accepted AcceptedVcIssuers (after remove): %+v", removeAcceptedVcIssuers))
	}

	//13.5.DisclosedVcSchemas----vc的删除和新增有变化（这里删除了）
	removeDisclosedVcSchemas := view(client, "DisclosedVcSchemas (after remove)", gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if len(removeDisclosedVcSchemas) != 0 {
		panic(fmt.Sprintf("expected empty disclosedVcs DisclosedVcSchemas (after remove): %+v", removeDisclosedVcSchemas))
	}

	//13.6.DisclosedVcs----vc的删除和新增有变化（这里删除了）
	removeDisclosedVcs := view(client, "DisclosedVcs (after remove)", gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if len(removeDisclosedVcs) != 0 {
		panic(fmt.Sprintf("expected empty disclosedVcs (after remove): %+v", removeDisclosedVcs))
	}

	//13.7.HasValidVcFromIssuer----只和VcAttestationCore.valid_until_ms和VcAttestationLifecycle.status有关
	removeHasValidVcFromIssuer := view(client, "HasValidVcFromIssuer (after remove)", gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if !removeHasValidVcFromIssuer {
		panic("the underlying attestation should stay valid (after remove)")
	}

	fmt.Printf("\n================ 14.Compliance: re-disclose ================\n")

	// 重新披露内容未变的 attestation 会恢复展示索引
	submit(
		client,
		bob,
		"DiscloseVcAttestation (after restore)",
		gen.Identity.DiscloseVcAttestation.Args(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs, issuerSignature).Encode,
	)

	//14.1.VcAttestationCore----永不变
	restoreVcAttestationCore := view(client, "VcAttestationCore (after restore)", gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if restoreVcAttestationCore.CredentialSchema != vcCredentialSchema || restoreVcAttestationCore.CredentialHash != credentialHash || restoreVcAttestationCore.ValidUntilMs == nil || *restoreVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after restore): %+v", restoreVcAttestationCore))
	}

	//14.2.VcAttestationLifecycle----永远存在（但状态可变）
	restoreVcAttestationLifecycle := view(client, "VcAttestationLifecycle (after restore)", gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if restoreVcAttestationLifecycle.Status.Variant != "Active" || restoreVcAttestationLifecycle.UpdatedAtMs != vcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after restore): %+v", restoreVcAttestationLifecycle))
	}

	//14.3.AcceptedVcIssuerIndexMeta----永不变
	restoreAcceptedVcIssuerIndexMeta := view(client, "AcceptedVcIssuerIndexMeta (after restore)", gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if restoreAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after restore): %q", restoreAcceptedVcIssuerIndexMeta))
	}

	//14.4.AcceptedVcIssuers----vc的删除和新增有变化（这里恢复了）
	restoreAcceptedVcIssuers := view(client, "AcceptedVcIssuers (after restored)", gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if len(restoreAcceptedVcIssuers) != 1 || *restoreAcceptedVcIssuers[0].Issuer != *kycOrg.addr || restoreAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after restore): %+v", restoreAcceptedVcIssuers))
	}

	//14.5.DisclosedVcSchemas----vc的删除和新增有变化（这里恢复了）
	restoreDisclosedVcSchemas := view(client, "DisclosedVcSchemas (after restored)", gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if len(restoreDisclosedVcSchemas) != 1 || restoreDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after restored): %+v", restoreDisclosedVcSchemas))
	}

	//14.7.DisclosedVcs----vc的删除和新增有变化（这里恢复了）
	restoreDisclosedVcs := view(client, "DisclosedVcs (after restored)", gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if len(restoreDisclosedVcs) != 1 || !restoreDisclosedVcs[0].IsValid || restoreDisclosedVcs[0].Status.Variant != "Active" || *restoreDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected disclosedVcs (after restored): %+v", restoreDisclosedVcs))
	}

	//14.6.HasValidVcFromIssuer----只和VcAttestationCore.valid_until_ms和VcAttestationLifecycle.status有关
	restoreHasValidVcFromIssuer := view(client, "HasValidVcFromIssuer (after restored)", gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if !restoreHasValidVcFromIssuer {
		panic("the underlying attestation should stay valid (after restored)")
	}

	fmt.Printf("\n================ 15.Compliance: freeze_vc_attestation ================\n")

	// freeze/unfreeze 通过 credential_id 定位 attestation（不是 credential_schema）；
	// credential_id 是确定性的：由 chain id + issuer 地址 + schema 派生
	credentialID := view(client, "CredentialId(KycLevel2)", gen.Identity.CredentialId.Args(kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.CredentialId.DecodeView)

	// 只有 issuer 能执行 freeze；冻结只改变生命周期状态，不会影响 subject 构建的展示索引
	submit(
		client,
		kycOrg,
		"FreezeVcAttestation",
		gen.Identity.FreezeVcAttestation.Args(kycOrg.addr, bob.addr, credentialID).Encode,
	)

	//15.1.VcAttestationCore----永不变
	frozenVcAttestationCore := view(client, "VcAttestationCore (after freeze)", gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if frozenVcAttestationCore.CredentialSchema != vcCredentialSchema || frozenVcAttestationCore.CredentialHash != credentialHash || frozenVcAttestationCore.ValidUntilMs == nil || *frozenVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after freeze): %+v", frozenVcAttestationCore))
	}

	//15.2.VcAttestationLifecycle----永远存在（但状态可变）
	frozenVcAttestationLifecycle := view(client, "VcAttestationLifecycle (after freeze)", gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if frozenVcAttestationLifecycle.Status.Variant != "Frozen" || frozenVcAttestationLifecycle.UpdatedAtMs == restoreVcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after freeze): %+v", frozenVcAttestationLifecycle))
	}

	//15.3.AcceptedVcIssuerIndexMeta----永不变
	frozenAcceptedVcIssuerIndexMeta := view(client, "AcceptedVcIssuerIndexMeta (after freeze)", gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if frozenAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after freeze): %q", frozenAcceptedVcIssuerIndexMeta))
	}

	//15.4.AcceptedVcIssuers----vc的删除和新增有变化（当前entry无这些操作）
	frozenAcceptedVcIssuers := view(client, "AcceptedVcIssuers (after freeze)", gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if len(frozenAcceptedVcIssuers) != 1 || *frozenAcceptedVcIssuers[0].Issuer != *kycOrg.addr || frozenAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after freeze): %+v", frozenAcceptedVcIssuers))
	}

	//15.5.DisclosedVcSchemas----vc的删除和新增有变化（当前entry无这些操作）
	frozenDisclosedVcSchemas := view(client, "DisclosedVcSchemas (after freeze)", gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if len(frozenDisclosedVcSchemas) != 1 || frozenDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after freeze): %+v", frozenDisclosedVcSchemas))
	}

	//15.6.DisclosedVcs----vc的删除和新增有变化（当前entry无这些操作）
	frozenDisclosedVcs := view(client, "DisclosedVcs (after freeze)", gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if len(frozenDisclosedVcs) != 1 || frozenDisclosedVcs[0].IsValid || frozenDisclosedVcs[0].Status.Variant != "Frozen" || *frozenDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected DisclosedVcs (after freeze): %+v", frozenDisclosedVcs))
	}

	//15.7.HasValidVcFromIssuer----只和VcAttestationCore.valid_until_ms和VcAttestationLifecycle.status有关
	frozenHasValidVcFromIssuer := view(client, "HasValidVcFromIssuer (after freeze)", gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if frozenHasValidVcFromIssuer {
		panic("a frozen VC must not be valid (after freeze)")
	}

	fmt.Printf("\n================ 16.Compliance: unfreeze_vc_attestation ================\n")

	// 只有 issuer 能执行 unfreeze；解冻把生命周期状态恢复为 Active，VC 重新有效
	submit(
		client,
		kycOrg,
		"UnfreezeVcAttestation",
		gen.Identity.UnfreezeVcAttestation.Args(kycOrg.addr, bob.addr, credentialID).Encode,
	)

	//16.1.VcAttestationCore----永不变
	unfrozenVcAttestationCore := view(client, "VcAttestationCore (after unfreeze)", gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if unfrozenVcAttestationCore.CredentialSchema != vcCredentialSchema || unfrozenVcAttestationCore.CredentialHash != credentialHash || unfrozenVcAttestationCore.ValidUntilMs == nil || *unfrozenVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after unfreeze): %+v", unfrozenVcAttestationCore))
	}

	//16.2.VcAttestationLifecycle----永远存在（但状态可变）
	unfrozenVcAttestationLifecycle := view(client, "VcAttestationLifecycle (after unfreeze)", gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if unfrozenVcAttestationLifecycle.Status.Variant != "Active" || unfrozenVcAttestationLifecycle.UpdatedAtMs == frozenVcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after unfreeze): %+v", unfrozenVcAttestationLifecycle))
	}

	//16.3.AcceptedVcIssuerIndexMeta----永不变
	unfrozenAcceptedVcIssuerIndexMeta := view(client, "AcceptedVcIssuerIndexMeta (after unfreeze)", gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if unfrozenAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after unfreeze): %q", unfrozenAcceptedVcIssuerIndexMeta))
	}

	//16.4.AcceptedVcIssuers----vc的删除和新增有变化（当前entry无这些操作）
	unfrozenAcceptedVcIssuers := view(client, "AcceptedVcIssuers (after unfreeze)", gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if len(unfrozenAcceptedVcIssuers) != 1 || *unfrozenAcceptedVcIssuers[0].Issuer != *kycOrg.addr || unfrozenAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after unfreeze): %+v", unfrozenAcceptedVcIssuers))
	}

	//16.5.DisclosedVcSchemas----vc的删除和新增有变化（当前entry无这些操作）
	unfrozenDisclosedVcSchemas := view(client, "DisclosedVcSchemas (after unfreeze)", gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if len(unfrozenDisclosedVcSchemas) != 1 || unfrozenDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after unfreeze): %+v", unfrozenDisclosedVcSchemas))
	}

	//16.6.DisclosedVcs----vc的删除和新增有变化（当前entry无这些操作）
	unfrozenDisclosedVcs := view(client, "DisclosedVcs (after unfreeze)", gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if len(unfrozenDisclosedVcs) != 1 || !unfrozenDisclosedVcs[0].IsValid || unfrozenDisclosedVcs[0].Status.Variant != "Active" || *unfrozenDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected DisclosedVcs (after unfreeze): %+v", unfrozenDisclosedVcs))
	}

	//16.7.HasValidVcFromIssuer----只和VcAttestationCore.valid_until_ms和VcAttestationLifecycle.status有关
	unfrozenHasValidVcFromIssuer := view(client, "HasValidVcFromIssuer (after unfreeze)", gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if !unfrozenHasValidVcFromIssuer {
		panic("the unfrozen attestation should be valid again (after unfreeze)")
	}

	fmt.Printf("\n================ 17.Compliance: revoke_vc_attestation ================\n")

	// 只有 issuer 能执行 revoke；revoke 是终态，
	// 不会影响 subject 构建的展示索引
	submit(
		client,
		kycOrg,
		"RevokeVcAttestation",
		gen.Identity.RevokeVcAttestation.Args(kycOrg.addr, bob.addr, vcCredentialSchema).Encode,
	)

	//17.1.VcAttestationCore----永不变
	revokedVcAttestationCore := view(client, "VcAttestationCore (after revoked)", gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if revokedVcAttestationCore.CredentialSchema != vcCredentialSchema || revokedVcAttestationCore.CredentialHash != credentialHash || revokedVcAttestationCore.ValidUntilMs == nil || *revokedVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after revoke): %+v", revokedVcAttestationCore))
	}

	//17.2.VcAttestationLifecycle----永远存在（但状态可变）
	revokedVcAttestationLifecycle := view(client, "VcAttestationLifecycle (after revoked)", gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if revokedVcAttestationLifecycle.Status.Variant != "Revoked" || revokedVcAttestationLifecycle.UpdatedAtMs == vcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected vcAttestationLifecycle (after revoke): %+v", revokedVcAttestationLifecycle))
	}

	//17.3.AcceptedVcIssuerIndexMeta----永不变
	revokedAcceptedVcIssuerIndexMeta := view(client, "AcceptedVcIssuerIndexMeta (after revoked)", gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if revokedAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after revoked): %q", revokedAcceptedVcIssuerIndexMeta))
	}

	//17.4.AcceptedVcIssuers----vc的删除和新增有变化（当前entry无这些操作）
	revokedAcceptedVcIssuers := view(client, "AcceptedVcIssuers (after revoked)", gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if len(revokedAcceptedVcIssuers) != 1 || *revokedAcceptedVcIssuers[0].Issuer != *kycOrg.addr || revokedAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after revoked): %+v", revokedAcceptedVcIssuers))
	}

	//17.5.DisclosedVcSchemas----vc的删除和新增有变化（当前entry无这些操作）
	revokedDisclosedVcSchemas := view(client, "DisclosedVcSchemas (after revoked)", gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if len(revokedDisclosedVcSchemas) != 1 || revokedDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after revoked): %+v", revokedDisclosedVcSchemas))
	}

	//17.6.DisclosedVcs----vc的删除和新增有变化（当前entry无这些操作）
	revokedDisclosedVcs := view(client, "DisclosedVcs (after revoked)", gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if len(revokedDisclosedVcs) != 1 || revokedDisclosedVcs[0].IsValid || revokedDisclosedVcs[0].Status.Variant != "Revoked" || *revokedDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected DisclosedVcs (after revoke): %+v", revokedDisclosedVcs))
	}

	//17.7.HasValidVcFromIssuer----只和VcAttestationCore.valid_until_ms和VcAttestationLifecycle.status有关
	revokedHasValidVcFromIssuer := view(client, "HasValidVcFromIssuer (after revoked)", gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if revokedHasValidVcFromIssuer {
		panic("a revoked VC must not be valid (after revoke)")
	}

	fmt.Printf("\n================ 18.Compliance: re-disclose(after revoke) ================\n")

	// 已 revoke 的 attestation 永远无法再次 disclose
	expectOnChainError(
		client,
		bob,
		"DiscloseVcAttestation(after revoke)",
		gen.Identity.DiscloseVcAttestation.Args(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs, issuerSignature).Encode,
	)
}
