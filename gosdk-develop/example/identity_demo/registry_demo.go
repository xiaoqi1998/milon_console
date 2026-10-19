package main

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/gen"
)

// demoEntityRegistry 走完开放机构目录流程：kycOrg 以 VcIssuer 角色和
// 两个凭证 schema 入驻、更新能力声明，tempOrg 自我停用以展示不可逆状态。
// 覆盖 entity_registry 的全部 entry 和全部 view。
func demoEntityRegistry(client *milon.Client, kycOrg, tempOrg *account) {
	fmt.Printf("\n================ 7.Entity registry: register_organization ================\n")

	// 注册要求拥有一个 subject_type 为 Organization 的活跃 DID
	orgDoc := gen.IdentityDidDocumentInput{
		SubjectType: gen.IdentityDidSubjectType{Variant: "Organization"},
		Keys: []gen.IdentityDidKeyInput{
			// key 0 用于在合规演示中签名 VC attestation
			{PublicKey: kycOrg.pk, Label: strPtr("issuer-key")},
		},
		AvatarUri: "https://kyc.example.org/logo.png",
	}
	//submit(
	//	client,
	//	kycOrg,
	//	"Create (organization DID)",
	//	gen.Identity.Create.Args(kycOrg.addr, orgDoc).Encode,
	//)

	submit(
		client,
		kycOrg,
		"Create (organization DID)",
		gen.Identity.CreateWithAlias.Args(kycOrg.addr, orgDoc, gen.IdentityDidName{Alias: "Kyc", Suffix: didNameSuffix(kycOrg.addr, 0)}).Encode,
	)

	// VcIssuer / KycProvider 角色要求至少声明一个凭证 schema
	submit(
		client,
		kycOrg,
		"RegisterOrganization",
		gen.Identity.RegisterOrganization.Args(
			kycOrg.addr,
			[]gen.IdentityEntityRole{{Variant: "VcIssuer"}},
			[]string{"KycLevel2", "AgeOver18"},
		).Encode,
	)

	capabilities := view(client, "OrganizationCapabilities", gen.Identity.OrganizationCapabilities.Args(kycOrg.addr).Encode, gen.Identity.OrganizationCapabilities.DecodeView)
	if len(capabilities.Roles) != 1 || capabilities.Roles[0].Variant != "VcIssuer" || len(capabilities.CredentialSchemas) != 2 || capabilities.CredentialSchemas[0] != "KycLevel2" || capabilities.CredentialSchemas[1] != "AgeOver18" {
		panic(fmt.Sprintf("unexpected capabilities: %+v", capabilities))
	}

	orgStatus := view(client, "OrganizationStatus", gen.Identity.OrganizationStatus.Args(kycOrg.addr).Encode, gen.Identity.OrganizationStatus.DecodeView)
	if orgStatus.Variant != "Active" {
		panic(fmt.Sprintf("unexpected organization status: %+v", orgStatus))
	}

	view(client, "OrganizationUpdatedAt", gen.Identity.OrganizationUpdatedAt.Args(kycOrg.addr).Encode, gen.Identity.OrganizationUpdatedAt.DecodeView)

	// credential_id 是确定性的：chain id + issuer 地址 + schema
	credentialID := view(client, "CredentialId(KycLevel2)", gen.Identity.CredentialId.Args(kycOrg.addr, "KycLevel2").Encode, gen.Identity.CredentialId.DecodeView)
	if credentialID == "" {
		panic("credential id must not be empty")
	}

	credentialDefinition := view(client, "CredentialDefinition", gen.Identity.CredentialDefinition.Args(credentialID).Encode, gen.Identity.CredentialDefinition.DecodeView)
	if *credentialDefinition.Issuer != *kycOrg.addr || credentialDefinition.CredentialSchema != "KycLevel2" {
		panic(fmt.Sprintf("unexpected credential definition: %+v", credentialDefinition))
	}

	// 机构只能注册一次
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

	// update 会完全替换 roles 和 credential schema
	submit(
		client,
		kycOrg,
		"UpdateOrganizationCapabilities",
		gen.Identity.UpdateOrganizationCapabilities.Args(
			kycOrg.addr,
			[]gen.IdentityEntityRole{{Variant: "VcIssuer"}, {Variant: "KycProvider"}},
			[]string{"KycLevel2", "AgeOver18", "IncomeRange"},
		).Encode,
	)

	capabilities = view(client, "OrganizationCapabilities", gen.Identity.OrganizationCapabilities.Args(kycOrg.addr).Encode, gen.Identity.OrganizationCapabilities.DecodeView)
	if len(capabilities.Roles) != 2 || len(capabilities.CredentialSchemas) != 3 {
		panic(fmt.Sprintf("unexpected capabilities after update: %+v", capabilities))
	}

	incomeRangeID := view(client, "CredentialId(IncomeRange)", gen.Identity.CredentialId.Args(kycOrg.addr, "IncomeRange").Encode, gen.Identity.CredentialId.DecodeView)
	incomeRangeDefinition := view(client, "CredentialDefinition", gen.Identity.CredentialDefinition.Args(incomeRangeID).Encode, gen.Identity.CredentialDefinition.DecodeView)
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
	//submit(
	//	client,
	//	tempOrg,
	//	"Create (organization DID)",
	//	gen.Identity.Create.Args(tempOrg.addr, tempDoc).Encode,
	//)

	submit(
		client,
		tempOrg,
		"Create (organization DID)",
		gen.Identity.CreateWithAlias.Args(tempOrg.addr, tempDoc, gen.IdentityDidName{Alias: "Kyc-Temp", Suffix: didNameSuffix(tempOrg.addr, 0)}).Encode,
	)

	submit(client,
		tempOrg,
		"RegisterOrganization",
		gen.Identity.RegisterOrganization.Args(
			tempOrg.addr,
			[]gen.IdentityEntityRole{{Variant: "KycProvider"}},
			[]string{"EmploymentStatus"},
		).Encode)
	submit(client, tempOrg, "DeactivateOrganization", gen.Identity.DeactivateOrganization.Args(tempOrg.addr).Encode)

	tempStatus := view(client, "OrganizationStatus(tempOrg)", gen.Identity.OrganizationStatus.Args(tempOrg.addr).Encode, gen.Identity.OrganizationStatus.DecodeView)
	if tempStatus.Variant != "Deactivated" {
		panic(fmt.Sprintf("unexpected tempOrg status: %+v", tempStatus))
	}

	// 已停用的机构无法再更新其能力声明
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
