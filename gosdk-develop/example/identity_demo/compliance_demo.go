package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"
	"lukechampine.com/blake3"
)

// vcCredentialSchema is the credential schema used by the compliance demo.
// kycOrg already registered it in the entity registry demo.
const vcCredentialSchema = "KycLevel2"

// vcAttestationSigningDigest rebuilds the exact 32-byte digest that the compliance
// program verifies the issuer signature against (see vc_attestation_signing_digest in compliance.rs):
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

	expiryTag := byte(0) // validUntilMs absent
	var expiryMs uint64
	if validUntilMs != nil {
		expiryTag = 1 // validUntilMs present
		expiryMs = *validUntilMs
	}

	expiry := make([]byte, 9)
	expiry[0] = expiryTag
	binary.LittleEndian.PutUint64(expiry[1:], expiryMs)

	return crypto.Hash32([]byte("milon.identity.vc-attestation.v1"),
		chainID,
		[]byte{0x04}, // identity app id
		subject.Bytes[:],
		issuer.Bytes[:],
		[]byte{issuerKeyID},
		schemaLen,
		[]byte(credentialSchema),
		credentialHash[:],
		expiry,
	)
}

// demoCompliance runs the minimal VC outcome flow: bob creates a personal DID,
// kycOrg signs a mocked off-chain credential, bob discloses the credential hash
// on chain, then the demo walks removal, restored disclosure, freeze, unfreeze and
// finally revoke. It covers every compliance entry and every compliance view.
func demoCompliance(client *milon.Client, kycOrg, bob *account) {
	fmt.Printf("\n================ 10.Compliance: create subject DID ================\n")

	bobDoc := gen.IdentityDidDocumentInput{
		SubjectType: gen.IdentityDidSubjectType{Variant: "Personal"},
		Keys: []gen.IdentityDidKeyInput{
			{PublicKey: bob.pk, Label: strPtr("primary")},
		},
		AvatarUri: "https://bob.example.com/avatar.png",
	}

	wire, err := gen.Identity.CreateWithAlias.Args(bob.addr, bobDoc, gen.IdentityDidName{Alias: "Bob", Suffix: didNameSuffix(bob.addr, 0)}).Encode()
	if err != nil {
		panic("failed to encode Create: " + err.Error())
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(bob.addr).
		AddIxesSig(*bob.addr, bob.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *bob.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign Create: " + err.Error())
	}
	txHistory, err := client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (Create): " + err.Error())
	}
	fmt.Printf("  [tx] Create ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	fmt.Printf("\n================ 11.Compliance: disclose_vc_attestation ================\n")

	// the mock VC document is hashed off chain: the document itself is never seen on chain
	credentialDocument := []byte(`{"type":"KycLevel2","subject":"Bob","country":"SG","verified":true}`)
	credentialHash := blake3.Sum256(credentialDocument)
	validUntilMs := uint64(time.Now().Add(72 * time.Hour).UnixMilli())

	// kycOrg signs the digest with its DID key 0 (created in the entity registry demo);
	// disclose only requires the issuer to hold an active DID
	digest := vcAttestationSigningDigest(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs)
	issuerSignature := kycOrg.sk.SignEd25519(digest[:])

	wire, err = gen.Identity.DiscloseVcAttestation.Args(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs, issuerSignature).Encode()
	if err != nil {
		panic("failed to encode DiscloseVcAttestation: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(bob.addr).
		AddIxesSig(*bob.addr, bob.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *bob.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign DiscloseVcAttestation: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (DiscloseVcAttestation): " + err.Error())
	}
	fmt.Printf("  [tx] DiscloseVcAttestation ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	//11.1.VcAttestationCore ---- never changes
	vcAttestationCore, err := milon.ViewValue(client, gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationCore: " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationCore => %+v\n", vcAttestationCore)
	if vcAttestationCore.CredentialSchema != vcCredentialSchema || vcAttestationCore.CredentialHash != credentialHash || vcAttestationCore.ValidUntilMs == nil || *vcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore: %+v", vcAttestationCore))
	}

	//11.2.VcAttestationLifecycle ---- state is mutable
	vcAttestationLifecycle, err := milon.ViewValue(client, gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationLifecycle: " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationLifecycle => %+v\n", vcAttestationLifecycle)
	if vcAttestationLifecycle.Status.Variant != "Active" {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle: %+v", vcAttestationLifecycle))
	}

	//11.3.HasValidVcFromIssuer ---- depends only on VcAttestationCore.valid_until_ms, VcAttestationLifecycle.status and the issuer DID
	hasValidVcFromIssuer, err := milon.ViewValue(client, gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if err != nil {
		panic("failed to view HasValidVcFromIssuer: " + err.Error())
	}
	fmt.Printf("  [view] HasValidVcFromIssuer => %+v\n", hasValidVcFromIssuer)
	if !hasValidVcFromIssuer {
		panic("bob should have a valid VC from kycOrg")
	}

	//11.4.AcceptedVcIssuerIndexMeta ---- never changes
	acceptedVcIssuerIndexMeta, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuerIndexMeta: " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuerIndexMeta => %+v\n", acceptedVcIssuerIndexMeta)
	if acceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta: %q", acceptedVcIssuerIndexMeta))
	}

	//11.5.AcceptedVcIssuers ---- changes when a VC is added or removed
	acceptedVcIssuers, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuers: " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuers => %+v\n", acceptedVcIssuers)
	if len(acceptedVcIssuers) != 1 || *acceptedVcIssuers[0].Issuer != *kycOrg.addr || acceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuers: %+v", acceptedVcIssuers))
	}

	//11.6.DisclosedVcSchemas ---- changes when a VC is added or removed
	disclosedVcSchemas, err := milon.ViewValue(client, gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcSchemas: " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcSchemas => %+v\n", disclosedVcSchemas)
	if len(disclosedVcSchemas) != 1 || disclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas: %+v", disclosedVcSchemas))
	}

	//11.7.DisclosedVcs ---- changes when a VC is added or removed
	disclosedVcs, err := milon.ViewValue(client, gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcs: " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcs => %+v\n", disclosedVcs)
	if len(disclosedVcs) != 1 || !disclosedVcs[0].IsValid || disclosedVcs[0].Status.Variant != "Active" || *disclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected disclosedVcs: %+v", disclosedVcs))
	}

	fmt.Printf("\n================ 12.Compliance: repeated disclose_vc_attestation is idempotent ================\n")

	// an exactly repeated disclose_vc_attestation succeeds and must not refresh the lifecycle timestamp
	wire, err = gen.Identity.DiscloseVcAttestation.Args(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs, issuerSignature).Encode()
	if err != nil {
		panic("failed to encode DiscloseVcAttestation (exact repeat): " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(bob.addr).
		AddIxesSig(*bob.addr, bob.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *bob.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign DiscloseVcAttestation (exact repeat): " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (DiscloseVcAttestation repeat): " + err.Error())
	}
	fmt.Printf("  [tx] DiscloseVcAttestation(repeat) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	//12.1.VcAttestationCore ---- never changes
	repeatedVcAttestationCore, err := milon.ViewValue(client, gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationCore (after repeat): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationCore (after repeat) => %+v\n", repeatedVcAttestationCore)
	if repeatedVcAttestationCore.CredentialSchema != vcCredentialSchema || repeatedVcAttestationCore.CredentialHash != credentialHash || repeatedVcAttestationCore.ValidUntilMs == nil || *vcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after repeat): %+v", repeatedVcAttestationCore))
	}

	//12.2.VcAttestationLifecycle ---- state is mutable (the current entry has no such operations)
	repeatVcAttestationLifecycle, err := milon.ViewValue(client, gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationLifecycle (after repeat): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationLifecycle (after repeat) => %+v\n", repeatVcAttestationLifecycle)
	if repeatVcAttestationLifecycle.Status.Variant != "Active" || repeatVcAttestationLifecycle.UpdatedAtMs != vcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after repeat): %+v", repeatVcAttestationLifecycle))
	}

	//12.3.HasValidVcFromIssuer ---- depends only on VcAttestationCore.valid_until_ms, VcAttestationLifecycle.status and the issuer DID (the current entry has no such operations)
	repeatHasValidVcFromIssuer, err := milon.ViewValue(client, gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if err != nil {
		panic("failed to view HasValidVcFromIssuer (after repeat): " + err.Error())
	}
	fmt.Printf("  [view] HasValidVcFromIssuer (after repeat) => %+v\n", repeatHasValidVcFromIssuer)
	if !repeatHasValidVcFromIssuer {
		panic("the underlying attestation should stay valid (after repeat)")
	}

	//12.4.AcceptedVcIssuerIndexMeta ---- never changes
	repeatAcceptedVcIssuerIndexMeta, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuerIndexMeta (after repeat): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuerIndexMeta (after repeat) => %+v\n", repeatAcceptedVcIssuerIndexMeta)
	if repeatAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after repeat): %q", repeatAcceptedVcIssuerIndexMeta))
	}

	//12.5.AcceptedVcIssuers ---- changes when a VC is added or removed (the current entry has no such operations)
	repeatAcceptedVcIssuers, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuers (after repeat): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuers (after repeat) => %+v\n", repeatAcceptedVcIssuers)
	if len(repeatAcceptedVcIssuers) != 1 || *repeatAcceptedVcIssuers[0].Issuer != *kycOrg.addr || repeatAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after repeat): %+v", repeatAcceptedVcIssuers))
	}

	//12.6.DisclosedVcSchemas ---- changes when a VC is added or removed (the current entry has no such operations)
	repeatDisclosedVcSchemas, err := milon.ViewValue(client, gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcSchemas (after repeat): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcSchemas (after repeat) => %+v\n", repeatDisclosedVcSchemas)
	if len(repeatDisclosedVcSchemas) != 1 || repeatDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after repeat): %+v", repeatDisclosedVcSchemas))
	}

	//12.7.DisclosedVcs ---- changes when a VC is added or removed (the current entry has no such operations)
	repeatDisclosedVcs, err := milon.ViewValue(client, gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcs (after repeat): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcs (after repeat) => %+v\n", repeatDisclosedVcs)
	if len(repeatDisclosedVcs) != 1 || !repeatDisclosedVcs[0].IsValid || repeatDisclosedVcs[0].Status.Variant != "Active" || *repeatDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected disclosedVcs (after repeat): %+v", repeatDisclosedVcs))
	}

	fmt.Printf("\n================ 13.Compliance: remove_vc_disclosure ================\n")

	// remove only withdraws the public disclosure index;
	// the underlying attestation record stays on chain
	wire, err = gen.Identity.RemoveVcDisclosure.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode()
	if err != nil {
		panic("failed to encode RemoveVcDisclosure: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(bob.addr).
		AddIxesSig(*bob.addr, bob.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *bob.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign RemoveVcDisclosure: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (RemoveVcDisclosure): " + err.Error())
	}
	fmt.Printf("  [tx] RemoveVcDisclosure ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	//13.1.VcAttestationCore ---- never changes
	removeVcAttestationCore, err := milon.ViewValue(client, gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationCore (after remove): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationCore (after remove) => %+v\n", removeVcAttestationCore)
	if removeVcAttestationCore.CredentialSchema != vcCredentialSchema || removeVcAttestationCore.CredentialHash != credentialHash || removeVcAttestationCore.ValidUntilMs == nil || *removeVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after remove): %+v", removeVcAttestationCore))
	}

	//13.2.VcAttestationLifecycle ---- state is mutable (the current entry has no such operations)
	removeVcAttestationLifecycle, err := milon.ViewValue(client, gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationLifecycle (after remove): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationLifecycle (after remove) => %+v\n", removeVcAttestationLifecycle)
	if removeVcAttestationLifecycle.Status.Variant != "Active" || removeVcAttestationLifecycle.UpdatedAtMs != vcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after remove): %+v", removeVcAttestationLifecycle))
	}

	//13.3.HasValidVcFromIssuer ---- depends only on VcAttestationCore.valid_until_ms, VcAttestationLifecycle.status and the issuer DID (the current entry has no such operations)
	removeHasValidVcFromIssuer, err := milon.ViewValue(client, gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if err != nil {
		panic("failed to view HasValidVcFromIssuer (after remove): " + err.Error())
	}
	fmt.Printf("  [view] HasValidVcFromIssuer (after remove) => %+v\n", removeHasValidVcFromIssuer)
	if !removeHasValidVcFromIssuer {
		panic("the underlying attestation should stay valid (after remove)")
	}

	//13.4.AcceptedVcIssuerIndexMeta ---- never changes
	removeAcceptedVcIssuerIndexMeta, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuerIndexMeta (after remove): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuerIndexMeta (after remove) => %+v\n", removeAcceptedVcIssuerIndexMeta)
	if removeAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after remove): %q", removeAcceptedVcIssuerIndexMeta))
	}

	//13.5.AcceptedVcIssuers ---- changes when a VC is added or removed (the current entry removed a VcDisclosure)
	removeAcceptedVcIssuers, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuers (after remove): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuers (after remove) => %+v\n", removeAcceptedVcIssuers)
	if len(removeAcceptedVcIssuers) != 0 {
		panic(fmt.Sprintf("expected empty accepted AcceptedVcIssuers (after remove): %+v", removeAcceptedVcIssuers))
	}

	//13.6.DisclosedVcSchemas ---- changes when a VC is added or removed (the current entry removed a VcDisclosure)
	removeDisclosedVcSchemas, err := milon.ViewValue(client, gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcSchemas (after remove): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcSchemas (after remove) => %+v\n", removeDisclosedVcSchemas)
	if len(removeDisclosedVcSchemas) != 0 {
		panic(fmt.Sprintf("expected empty disclosedVcs DisclosedVcSchemas (after remove): %+v", removeDisclosedVcSchemas))
	}

	//13.7.DisclosedVcs ---- changes when a VC is added or removed (the current entry removed a VcDisclosure)
	removeDisclosedVcs, err := milon.ViewValue(client, gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcs (after remove): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcs (after remove) => %+v\n", removeDisclosedVcs)
	if len(removeDisclosedVcs) != 0 {
		panic(fmt.Sprintf("expected empty disclosedVcs (after remove): %+v", removeDisclosedVcs))
	}

	fmt.Printf("\n================ 14.Compliance: restore disclose_vc_attestation ================\n")

	// re-disclosing an attestation whose content is unchanged restores the disclosure index
	wire, err = gen.Identity.DiscloseVcAttestation.Args(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs, issuerSignature).Encode()
	if err != nil {
		panic("failed to encode DiscloseVcAttestation (after restore): " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(bob.addr).
		AddIxesSig(*bob.addr, bob.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *bob.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign DiscloseVcAttestation (after restore): " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (DiscloseVcAttestation restore): " + err.Error())
	}
	fmt.Printf("  [tx] DiscloseVcAttestation(restore) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	//14.1.VcAttestationCore ---- never changes
	restoreVcAttestationCore, err := milon.ViewValue(client, gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationCore (after restore): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationCore (after restore) => %+v\n", restoreVcAttestationCore)
	if restoreVcAttestationCore.CredentialSchema != vcCredentialSchema || restoreVcAttestationCore.CredentialHash != credentialHash || restoreVcAttestationCore.ValidUntilMs == nil || *restoreVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after restore): %+v", restoreVcAttestationCore))
	}

	//14.2.VcAttestationLifecycle ---- state is mutable (the current entry has no such operations)
	restoreVcAttestationLifecycle, err := milon.ViewValue(client, gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationLifecycle (after restore): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationLifecycle (after restore) => %+v\n", restoreVcAttestationLifecycle)
	if restoreVcAttestationLifecycle.Status.Variant != "Active" || restoreVcAttestationLifecycle.UpdatedAtMs != vcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after restore): %+v", restoreVcAttestationLifecycle))
	}

	//14.3.HasValidVcFromIssuer ---- depends only on VcAttestationCore.valid_until_ms, VcAttestationLifecycle.status and the issuer DID (the current entry has no such operations)
	restoreHasValidVcFromIssuer, err := milon.ViewValue(client, gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if err != nil {
		panic("failed to view HasValidVcFromIssuer (after restored): " + err.Error())
	}
	fmt.Printf("  [view] HasValidVcFromIssuer (after restored) => %+v\n", restoreHasValidVcFromIssuer)
	if !restoreHasValidVcFromIssuer {
		panic("the underlying attestation should stay valid (after restored)")
	}

	//14.4.AcceptedVcIssuerIndexMeta ---- never changes
	restoreAcceptedVcIssuerIndexMeta, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuerIndexMeta (after restore): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuerIndexMeta (after restore) => %+v\n", restoreAcceptedVcIssuerIndexMeta)
	if restoreAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after restore): %q", restoreAcceptedVcIssuerIndexMeta))
	}

	//14.5.AcceptedVcIssuers ---- changes when a VC is added or removed (the current entry restored a VcDisclosure)
	restoreAcceptedVcIssuers, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuers (after restored): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuers (after restored) => %+v\n", restoreAcceptedVcIssuers)
	if len(restoreAcceptedVcIssuers) != 1 || *restoreAcceptedVcIssuers[0].Issuer != *kycOrg.addr || restoreAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after restore): %+v", restoreAcceptedVcIssuers))
	}

	//14.6.DisclosedVcSchemas ---- changes when a VC is added or removed (the current entry restored a VcDisclosure)
	restoreDisclosedVcSchemas, err := milon.ViewValue(client, gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcSchemas (after restored): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcSchemas (after restored) => %+v\n", restoreDisclosedVcSchemas)
	if len(restoreDisclosedVcSchemas) != 1 || restoreDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after restored): %+v", restoreDisclosedVcSchemas))
	}

	//14.7.DisclosedVcs ---- changes when a VC is added or removed (the current entry restored a VcDisclosure)
	restoreDisclosedVcs, err := milon.ViewValue(client, gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcs (after restored): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcs (after restored) => %+v\n", restoreDisclosedVcs)
	if len(restoreDisclosedVcs) != 1 || !restoreDisclosedVcs[0].IsValid || restoreDisclosedVcs[0].Status.Variant != "Active" || *restoreDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected disclosedVcs (after restored): %+v", restoreDisclosedVcs))
	}

	fmt.Printf("\n================ 15.Compliance: freeze_vc_attestation ================\n")

	// freeze/unfreeze address the attestation by credential_id (not credential_schema);
	// credential_id is deterministic: derived from chain id + issuer address + schema
	credentialID, err := milon.ViewValue(client, gen.Identity.CredentialId.Args(kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.CredentialId.DecodeView)
	if err != nil {
		panic("failed to view CredentialId(KycLevel2): " + err.Error())
	}
	fmt.Printf("  [view] CredentialId(KycLevel2) => %+v\n", credentialID)

	// only the issuer can freeze; freezing only changes the lifecycle state and leaves the disclosure index built by the subject untouched
	wire, err = gen.Identity.FreezeVcAttestation.Args(kycOrg.addr, bob.addr, credentialID).Encode()
	if err != nil {
		panic("failed to encode FreezeVcAttestation: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(kycOrg.addr).
		AddIxesSig(*kycOrg.addr, kycOrg.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *kycOrg.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign FreezeVcAttestation: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (FreezeVcAttestation): " + err.Error())
	}
	fmt.Printf("  [tx] FreezeVcAttestation ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	//15.1.VcAttestationCore ---- never changes
	frozenVcAttestationCore, err := milon.ViewValue(client, gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationCore (after freeze): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationCore (after freeze) => %+v\n", frozenVcAttestationCore)
	if frozenVcAttestationCore.CredentialSchema != vcCredentialSchema || frozenVcAttestationCore.CredentialHash != credentialHash || frozenVcAttestationCore.ValidUntilMs == nil || *frozenVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after freeze): %+v", frozenVcAttestationCore))
	}

	//15.2.VcAttestationLifecycle ---- state is mutable (the current entry set Status to Frozen)
	frozenVcAttestationLifecycle, err := milon.ViewValue(client, gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationLifecycle (after freeze): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationLifecycle (after freeze) => %+v\n", frozenVcAttestationLifecycle)
	if frozenVcAttestationLifecycle.Status.Variant != "Frozen" || frozenVcAttestationLifecycle.UpdatedAtMs == restoreVcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after freeze): %+v", frozenVcAttestationLifecycle))
	}

	//15.3.HasValidVcFromIssuer ---- depends only on VcAttestationCore.valid_until_ms, VcAttestationLifecycle.status and the issuer DID (the current entry set Status to Frozen)
	frozenHasValidVcFromIssuer, err := milon.ViewValue(client, gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if err != nil {
		panic("failed to view HasValidVcFromIssuer (after freeze): " + err.Error())
	}
	fmt.Printf("  [view] HasValidVcFromIssuer (after freeze) => %+v\n", frozenHasValidVcFromIssuer)
	if frozenHasValidVcFromIssuer {
		panic("a frozen VC must not be valid (after freeze)")
	}

	//15.4.AcceptedVcIssuerIndexMeta ---- never changes
	frozenAcceptedVcIssuerIndexMeta, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuerIndexMeta (after freeze): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuerIndexMeta (after freeze) => %+v\n", frozenAcceptedVcIssuerIndexMeta)
	if frozenAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after freeze): %q", frozenAcceptedVcIssuerIndexMeta))
	}

	//15.5.AcceptedVcIssuers ---- changes when a VC is added or removed (the current entry has no such operations)
	frozenAcceptedVcIssuers, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuers (after freeze): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuers (after freeze) => %+v\n", frozenAcceptedVcIssuers)
	if len(frozenAcceptedVcIssuers) != 1 || *frozenAcceptedVcIssuers[0].Issuer != *kycOrg.addr || frozenAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after freeze): %+v", frozenAcceptedVcIssuers))
	}

	//15.6.DisclosedVcSchemas ---- changes when a VC is added or removed (the current entry has no such operations)
	frozenDisclosedVcSchemas, err := milon.ViewValue(client, gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcSchemas (after freeze): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcSchemas (after freeze) => %+v\n", frozenDisclosedVcSchemas)
	if len(frozenDisclosedVcSchemas) != 1 || frozenDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after freeze): %+v", frozenDisclosedVcSchemas))
	}

	//15.7.DisclosedVcs ---- changes when a VC is added or removed (the current entry has no such operations)
	frozenDisclosedVcs, err := milon.ViewValue(client, gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcs (after freeze): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcs (after freeze) => %+v\n", frozenDisclosedVcs)
	if len(frozenDisclosedVcs) != 1 || frozenDisclosedVcs[0].IsValid || frozenDisclosedVcs[0].Status.Variant != "Frozen" || *frozenDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected DisclosedVcs (after freeze): %+v", frozenDisclosedVcs))
	}

	fmt.Printf("\n================ 16.Compliance: unfreeze_vc_attestation ================\n")

	// only the issuer can unfreeze; unfreezing restores the lifecycle state to Active and the VC becomes valid again
	wire, err = gen.Identity.UnfreezeVcAttestation.Args(kycOrg.addr, bob.addr, credentialID).Encode()
	if err != nil {
		panic("failed to encode UnfreezeVcAttestation: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(kycOrg.addr).
		AddIxesSig(*kycOrg.addr, kycOrg.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *kycOrg.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign UnfreezeVcAttestation: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (UnfreezeVcAttestation): " + err.Error())
	}
	fmt.Printf("  [tx] UnfreezeVcAttestation ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	//16.1.VcAttestationCore ---- never changes
	unfrozenVcAttestationCore, err := milon.ViewValue(client, gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationCore (after unfreeze): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationCore (after unfreeze) => %+v\n", unfrozenVcAttestationCore)
	if unfrozenVcAttestationCore.CredentialSchema != vcCredentialSchema || unfrozenVcAttestationCore.CredentialHash != credentialHash || unfrozenVcAttestationCore.ValidUntilMs == nil || *unfrozenVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after unfreeze): %+v", unfrozenVcAttestationCore))
	}

	//16.2.VcAttestationLifecycle ---- state is mutable (the current entry set Status to Active)
	unfrozenVcAttestationLifecycle, err := milon.ViewValue(client, gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationLifecycle (after unfreeze): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationLifecycle (after unfreeze) => %+v\n", unfrozenVcAttestationLifecycle)
	if unfrozenVcAttestationLifecycle.Status.Variant != "Active" || unfrozenVcAttestationLifecycle.UpdatedAtMs == frozenVcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected VcAttestationLifecycle (after unfreeze): %+v", unfrozenVcAttestationLifecycle))
	}

	//16.3.HasValidVcFromIssuer ---- depends only on VcAttestationCore.valid_until_ms, VcAttestationLifecycle.status and the issuer DID (the current entry set Status to Active)
	unfrozenHasValidVcFromIssuer, err := milon.ViewValue(client, gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if err != nil {
		panic("failed to view HasValidVcFromIssuer (after unfreeze): " + err.Error())
	}
	fmt.Printf("  [view] HasValidVcFromIssuer (after unfreeze) => %+v\n", unfrozenHasValidVcFromIssuer)
	if !unfrozenHasValidVcFromIssuer {
		panic("the unfrozen attestation should be valid again (after unfreeze)")
	}

	//16.4.AcceptedVcIssuerIndexMeta ---- never changes
	unfrozenAcceptedVcIssuerIndexMeta, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuerIndexMeta (after unfreeze): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuerIndexMeta (after unfreeze) => %+v\n", unfrozenAcceptedVcIssuerIndexMeta)
	if unfrozenAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after unfreeze): %q", unfrozenAcceptedVcIssuerIndexMeta))
	}

	//16.5.AcceptedVcIssuers ---- changes when a VC is added or removed (the current entry has no such operations)
	unfrozenAcceptedVcIssuers, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuers (after unfreeze): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuers (after unfreeze) => %+v\n", unfrozenAcceptedVcIssuers)
	if len(unfrozenAcceptedVcIssuers) != 1 || *unfrozenAcceptedVcIssuers[0].Issuer != *kycOrg.addr || unfrozenAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after unfreeze): %+v", unfrozenAcceptedVcIssuers))
	}

	//16.6.DisclosedVcSchemas ---- changes when a VC is added or removed (the current entry has no such operations)
	unfrozenDisclosedVcSchemas, err := milon.ViewValue(client, gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcSchemas (after unfreeze): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcSchemas (after unfreeze) => %+v\n", unfrozenDisclosedVcSchemas)
	if len(unfrozenDisclosedVcSchemas) != 1 || unfrozenDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after unfreeze): %+v", unfrozenDisclosedVcSchemas))
	}

	//16.7.DisclosedVcs ---- changes when a VC is added or removed (the current entry has no such operations)
	unfrozenDisclosedVcs, err := milon.ViewValue(client, gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcs (after unfreeze): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcs (after unfreeze) => %+v\n", unfrozenDisclosedVcs)
	if len(unfrozenDisclosedVcs) != 1 || !unfrozenDisclosedVcs[0].IsValid || unfrozenDisclosedVcs[0].Status.Variant != "Active" || *unfrozenDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected DisclosedVcs (after unfreeze): %+v", unfrozenDisclosedVcs))
	}

	fmt.Printf("\n================ 17.Compliance: revoke_vc_attestation ================\n")

	// only the issuer can revoke; revoke is terminal and
	// does not affect the disclosure index built by the subject
	wire, err = gen.Identity.RevokeVcAttestation.Args(kycOrg.addr, bob.addr, vcCredentialSchema).Encode()
	if err != nil {
		panic("failed to encode RevokeVcAttestation: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(kycOrg.addr).
		AddIxesSig(*kycOrg.addr, kycOrg.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *kycOrg.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign RevokeVcAttestation: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (RevokeVcAttestation): " + err.Error())
	}
	fmt.Printf("  [tx] RevokeVcAttestation ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	//17.1.VcAttestationCore ---- never changes
	revokedVcAttestationCore, err := milon.ViewValue(client, gen.Identity.VcAttestationCore.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationCore.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationCore (after revoked): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationCore (after revoked) => %+v\n", revokedVcAttestationCore)
	if revokedVcAttestationCore.CredentialSchema != vcCredentialSchema || revokedVcAttestationCore.CredentialHash != credentialHash || revokedVcAttestationCore.ValidUntilMs == nil || *revokedVcAttestationCore.ValidUntilMs != validUntilMs {
		panic(fmt.Sprintf("unexpected VcAttestationCore (after revoke): %+v", revokedVcAttestationCore))
	}

	//17.2.VcAttestationLifecycle ---- state is mutable (the current entry set Status to Revoked)
	revokedVcAttestationLifecycle, err := milon.ViewValue(client, gen.Identity.VcAttestationLifecycle.Args(bob.addr, vcCredentialSchema, kycOrg.addr).Encode, gen.Identity.VcAttestationLifecycle.DecodeView)
	if err != nil {
		panic("failed to view VcAttestationLifecycle (after revoked): " + err.Error())
	}
	fmt.Printf("  [view] VcAttestationLifecycle (after revoked) => %+v\n", revokedVcAttestationLifecycle)
	if revokedVcAttestationLifecycle.Status.Variant != "Revoked" || revokedVcAttestationLifecycle.UpdatedAtMs == vcAttestationLifecycle.UpdatedAtMs {
		panic(fmt.Sprintf("unexpected vcAttestationLifecycle (after revoke): %+v", revokedVcAttestationLifecycle))
	}

	//17.3.HasValidVcFromIssuer ---- depends only on VcAttestationCore.valid_until_ms, VcAttestationLifecycle.status and the issuer DID (the current entry set Status to Revoked)
	revokedHasValidVcFromIssuer, err := milon.ViewValue(client, gen.Identity.HasValidVcFromIssuer.Args(bob.addr, kycOrg.addr, vcCredentialSchema).Encode, gen.Identity.HasValidVcFromIssuer.DecodeView)
	if err != nil {
		panic("failed to view HasValidVcFromIssuer (after revoked): " + err.Error())
	}
	fmt.Printf("  [view] HasValidVcFromIssuer (after revoked) => %+v\n", revokedHasValidVcFromIssuer)
	if revokedHasValidVcFromIssuer {
		panic("a revoked VC must not be valid (after revoke)")
	}

	//17.4.AcceptedVcIssuerIndexMeta ---- never changes
	revokedAcceptedVcIssuerIndexMeta, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuerIndexMeta.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuerIndexMeta.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuerIndexMeta (after revoked): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuerIndexMeta (after revoked) => %+v\n", revokedAcceptedVcIssuerIndexMeta)
	if revokedAcceptedVcIssuerIndexMeta != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected AcceptedVcIssuerIndexMeta (after revoked): %q", revokedAcceptedVcIssuerIndexMeta))
	}

	//17.5.AcceptedVcIssuers ---- changes when a VC is added or removed (the current entry has no such operations)
	revokedAcceptedVcIssuers, err := milon.ViewValue(client, gen.Identity.AcceptedVcIssuers.Args(bob.addr, vcCredentialSchema).Encode, gen.Identity.AcceptedVcIssuers.DecodeView)
	if err != nil {
		panic("failed to view AcceptedVcIssuers (after revoked): " + err.Error())
	}
	fmt.Printf("  [view] AcceptedVcIssuers (after revoked) => %+v\n", revokedAcceptedVcIssuers)
	if len(revokedAcceptedVcIssuers) != 1 || *revokedAcceptedVcIssuers[0].Issuer != *kycOrg.addr || revokedAcceptedVcIssuers[0].CredentialHash != credentialHash {
		panic(fmt.Sprintf("expected AcceptedVcIssuers (after revoked): %+v", revokedAcceptedVcIssuers))
	}

	//17.6.DisclosedVcSchemas ---- changes when a VC is added or removed (the current entry has no such operations)
	revokedDisclosedVcSchemas, err := milon.ViewValue(client, gen.Identity.DisclosedVcSchemas.Args(bob.addr).Encode, gen.Identity.DisclosedVcSchemas.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcSchemas (after revoked): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcSchemas (after revoked) => %+v\n", revokedDisclosedVcSchemas)
	if len(revokedDisclosedVcSchemas) != 1 || revokedDisclosedVcSchemas[0] != vcCredentialSchema {
		panic(fmt.Sprintf("unexpected DisclosedVcSchemas (after revoked): %+v", revokedDisclosedVcSchemas))
	}

	//17.7.DisclosedVcs ---- changes when a VC is added or removed (the current entry has no such operations)
	revokedDisclosedVcs, err := milon.ViewValue(client, gen.Identity.DisclosedVcs.Args(bob.addr, 0, 10).Encode, gen.Identity.DisclosedVcs.DecodeView)
	if err != nil {
		panic("failed to view DisclosedVcs (after revoked): " + err.Error())
	}
	fmt.Printf("  [view] DisclosedVcs (after revoked) => %+v\n", revokedDisclosedVcs)
	if len(revokedDisclosedVcs) != 1 || revokedDisclosedVcs[0].IsValid || revokedDisclosedVcs[0].Status.Variant != "Revoked" || *revokedDisclosedVcs[0].Issuer != *kycOrg.addr {
		panic(fmt.Sprintf("unexpected DisclosedVcs (after revoke): %+v", revokedDisclosedVcs))
	}

	fmt.Printf("\n================ 18.Compliance: re-disclose(after revoke) ================\n")

	// a revoked attestation can never be disclosed again
	expectOnChainError(
		client,
		bob,
		"DiscloseVcAttestation(after revoke)",
		gen.Identity.DiscloseVcAttestation.Args(bob.addr, kycOrg.addr, 0, vcCredentialSchema, credentialHash, &validUntilMs, issuerSignature).Encode,
	)
}
