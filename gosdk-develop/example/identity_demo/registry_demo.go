package main

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"
)

// demoEntityRegistry walks the open organization directory flow: kycOrg onboards with
// the VcIssuer role and two credential schemas, then updates its capabilities, while
// tempOrg deactivates itself to show the irreversible state.
// It covers every entity_registry entry and every entity_registry view.
func demoEntityRegistry(client *milon.Client, kycOrg, tempOrg *account) {
	fmt.Printf("\n================ 7.Entity registry: register_organization ================\n")

	// registration requires an active DID whose subject_type is Organization
	orgDoc := gen.IdentityDidDocumentInput{
		SubjectType: gen.IdentityDidSubjectType{Variant: "Organization"},
		Keys: []gen.IdentityDidKeyInput{
			// key 0 is used to sign the VC attestation in the compliance demo
			{PublicKey: kycOrg.pk, Label: strPtr("issuer-key")},
		},
		AvatarUri: "https://kyc.example.org/logo.png",
	}

	wire, err := gen.Identity.CreateWithAlias.Args(kycOrg.addr, orgDoc, gen.IdentityDidName{Alias: "Kyc", Suffix: didNameSuffix(kycOrg.addr, 0)}).Encode()
	if err != nil {
		panic("failed to encode CreateWithAlias: " + err.Error())
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(kycOrg.addr).
		AddIxesSig(*kycOrg.addr, kycOrg.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *kycOrg.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign CreateWithAlias: " + err.Error())
	}
	txHistory, err := client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (CreateWithAlias): " + err.Error())
	}
	fmt.Printf("  [tx] CreateWithAlias ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	// the VcIssuer / KycProvider roles require at least one declared credential schema
	wire, err = gen.Identity.RegisterOrganization.Args(
		kycOrg.addr,
		[]gen.IdentityEntityRole{{Variant: "VcIssuer"}},
		[]string{"KycLevel2", "AgeOver18"},
	).Encode()
	if err != nil {
		panic("failed to encode RegisterOrganization: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(kycOrg.addr).
		AddIxesSig(*kycOrg.addr, kycOrg.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *kycOrg.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign RegisterOrganization: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (RegisterOrganization): " + err.Error())
	}
	fmt.Printf("  [tx] RegisterOrganization ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	capabilities, err := milon.ViewValue(client, gen.Identity.OrganizationCapabilities.Args(kycOrg.addr).Encode, gen.Identity.OrganizationCapabilities.DecodeView)
	if err != nil {
		panic("failed to view OrganizationCapabilities: " + err.Error())
	}
	fmt.Printf("  [view] OrganizationCapabilities => %+v\n", capabilities)
	if len(capabilities.Roles) != 1 || capabilities.Roles[0].Variant != "VcIssuer" || len(capabilities.CredentialSchemas) != 2 || capabilities.CredentialSchemas[0] != "KycLevel2" || capabilities.CredentialSchemas[1] != "AgeOver18" {
		panic(fmt.Sprintf("unexpected capabilities: %+v", capabilities))
	}

	orgStatus, err := milon.ViewValue(client, gen.Identity.OrganizationStatus.Args(kycOrg.addr).Encode, gen.Identity.OrganizationStatus.DecodeView)
	if err != nil {
		panic("failed to view OrganizationStatus: " + err.Error())
	}
	fmt.Printf("  [view] OrganizationStatus => %+v\n", orgStatus)
	if orgStatus.Variant != "Active" {
		panic(fmt.Sprintf("unexpected organization status: %+v", orgStatus))
	}

	orgUpdatedAt, err := milon.ViewValue(client, gen.Identity.OrganizationUpdatedAt.Args(kycOrg.addr).Encode, gen.Identity.OrganizationUpdatedAt.DecodeView)
	if err != nil {
		panic("failed to view OrganizationUpdatedAt: " + err.Error())
	}
	fmt.Printf("  [view] OrganizationUpdatedAt => %+v\n", orgUpdatedAt)

	// credential_id is deterministic: chain id + issuer address + schema
	credentialID, err := milon.ViewValue(client, gen.Identity.CredentialId.Args(kycOrg.addr, "KycLevel2").Encode, gen.Identity.CredentialId.DecodeView)
	if err != nil {
		panic("failed to view CredentialId(KycLevel2): " + err.Error())
	}
	fmt.Printf("  [view] CredentialId(KycLevel2) => %+v\n", credentialID)
	if credentialID == "" {
		panic("credential id must not be empty")
	}

	credentialDefinition, err := milon.ViewValue(client, gen.Identity.CredentialDefinition.Args(credentialID).Encode, gen.Identity.CredentialDefinition.DecodeView)
	if err != nil {
		panic("failed to view CredentialDefinition: " + err.Error())
	}
	fmt.Printf("  [view] CredentialDefinition => %+v\n", credentialDefinition)
	if *credentialDefinition.Issuer != *kycOrg.addr || credentialDefinition.CredentialSchema != "KycLevel2" {
		panic(fmt.Sprintf("unexpected credential definition: %+v", credentialDefinition))
	}

	// an organization can only be registered once
	expectOnChainError(
		client,
		kycOrg, "RegisterOrganization (already registered)",
		gen.Identity.RegisterOrganization.Args(
			kycOrg.addr,
			[]gen.IdentityEntityRole{{Variant: "VcIssuer"}},
			[]string{"KycLevel2"},
		).Encode,
	)

	fmt.Printf("\n================ 8.Entity registry: update_organization_capabilities ================\n")

	// update fully replaces the roles and the credential schemas
	wire, err = gen.Identity.UpdateOrganizationCapabilities.Args(
		kycOrg.addr,
		[]gen.IdentityEntityRole{{Variant: "VcIssuer"}, {Variant: "KycProvider"}},
		[]string{"KycLevel2", "AgeOver18", "IncomeRange"},
	).Encode()
	if err != nil {
		panic("failed to encode UpdateOrganizationCapabilities: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(kycOrg.addr).
		AddIxesSig(*kycOrg.addr, kycOrg.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *kycOrg.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign UpdateOrganizationCapabilities: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (UpdateOrganizationCapabilities): " + err.Error())
	}
	fmt.Printf("  [tx] UpdateOrganizationCapabilities ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	capabilities, err = milon.ViewValue(client, gen.Identity.OrganizationCapabilities.Args(kycOrg.addr).Encode, gen.Identity.OrganizationCapabilities.DecodeView)
	if err != nil {
		panic("failed to view OrganizationCapabilities: " + err.Error())
	}
	fmt.Printf("  [view] OrganizationCapabilities => %+v\n", capabilities)
	if len(capabilities.Roles) != 2 || len(capabilities.CredentialSchemas) != 3 {
		panic(fmt.Sprintf("unexpected capabilities after update: %+v", capabilities))
	}

	incomeRangeID, err := milon.ViewValue(client, gen.Identity.CredentialId.Args(kycOrg.addr, "IncomeRange").Encode, gen.Identity.CredentialId.DecodeView)
	if err != nil {
		panic("failed to view CredentialId(IncomeRange): " + err.Error())
	}
	fmt.Printf("  [view] CredentialId(IncomeRange) => %+v\n", incomeRangeID)
	incomeRangeDefinition, err := milon.ViewValue(client, gen.Identity.CredentialDefinition.Args(incomeRangeID).Encode, gen.Identity.CredentialDefinition.DecodeView)
	if err != nil {
		panic("failed to view CredentialDefinition: " + err.Error())
	}
	fmt.Printf("  [view] CredentialDefinition => %+v\n", incomeRangeDefinition)
	if *incomeRangeDefinition.Issuer != *kycOrg.addr || incomeRangeDefinition.CredentialSchema != "IncomeRange" {
		panic(fmt.Sprintf("unexpected credential definition: %+v", incomeRangeDefinition))
	}

	fmt.Printf("\n================ 9.Entity registry: deactivate_organization (irreversible) ================\n")

	tempDoc := gen.IdentityDidDocumentInput{
		SubjectType: gen.IdentityDidSubjectType{Variant: "Organization"},
		Keys: []gen.IdentityDidKeyInput{
			{PublicKey: tempOrg.pk, Label: strPtr("issuer-key")},
		},
		AvatarUri: "https://temp.example.org/logo.png",
	}

	wire, err = gen.Identity.CreateWithAlias.Args(tempOrg.addr, tempDoc, gen.IdentityDidName{Alias: "Kyc-Temp", Suffix: didNameSuffix(tempOrg.addr, 0)}).Encode()
	if err != nil {
		panic("failed to encode CreateWithAlias: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(tempOrg.addr).
		AddIxesSig(*tempOrg.addr, tempOrg.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *tempOrg.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign CreateWithAlias: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (CreateWithAlias tempOrg): " + err.Error())
	}
	fmt.Printf("  [tx] CreateWithAlias(tempOrg) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	wire, err = gen.Identity.RegisterOrganization.Args(
		tempOrg.addr,
		[]gen.IdentityEntityRole{{Variant: "KycProvider"}},
		[]string{"EmploymentStatus"},
	).Encode()
	if err != nil {
		panic("failed to encode RegisterOrganization: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(tempOrg.addr).
		AddIxesSig(*tempOrg.addr, tempOrg.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *tempOrg.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign RegisterOrganization: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (RegisterOrganization tempOrg): " + err.Error())
	}
	fmt.Printf("  [tx] RegisterOrganization(tempOrg) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	wire, err = gen.Identity.DeactivateOrganization.Args(tempOrg.addr).Encode()
	if err != nil {
		panic("failed to encode DeactivateOrganization: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(tempOrg.addr).
		AddIxesSig(*tempOrg.addr, tempOrg.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *tempOrg.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign DeactivateOrganization: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (DeactivateOrganization): " + err.Error())
	}
	fmt.Printf("  [tx] DeactivateOrganization ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	tempStatus, err := milon.ViewValue(client, gen.Identity.OrganizationStatus.Args(tempOrg.addr).Encode, gen.Identity.OrganizationStatus.DecodeView)
	if err != nil {
		panic("failed to view OrganizationStatus(tempOrg): " + err.Error())
	}
	fmt.Printf("  [view] OrganizationStatus(tempOrg) => %+v\n", tempStatus)
	if tempStatus.Variant != "Deactivated" {
		panic(fmt.Sprintf("unexpected tempOrg status: %+v", tempStatus))
	}

	// a deactivated organization can no longer update its capabilities
	expectOnChainError(
		client,
		tempOrg,
		"UpdateOrganizationCapabilities after deactivate",
		gen.Identity.UpdateOrganizationCapabilities.Args(
			tempOrg.addr,
			[]gen.IdentityEntityRole{{Variant: "KycProvider"}},
			[]string{"EmploymentStatus", "IncomeRange"},
		).Encode,
	)
}
