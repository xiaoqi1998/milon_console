package main

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
)

const issuerStr = "4WJqwFRPc2wCxrUwBt8ts5KEo54u"

func main() {
	issuer, err := crypto.NewAddressFromRelaxed(issuerStr)
	if err != nil {
		panic(err)
	}
	client := milon.NewClient(milon.DevNet)

	// 1) DID Core
	{
		wire, _ := gen.Identity.Core.Args(issuer).Encode()
		res, err := client.View([]api.PackedInstruction{wire})
		if err != nil {
			fmt.Println("Core view err:", err)
		} else if core, err := gen.Identity.Core.DecodeView(res.HTTPResponseBody); err != nil {
			fmt.Println("Core decode err (DID likely absent):", err)
		} else {
			fmt.Printf("DID Core: subject=%v controller=%v\n", core.Subject, core.Controller)
		}
	}

	// 2) Organization status
	{
		wire, _ := gen.Identity.OrganizationStatus.Args(issuer).Encode()
		res, err := client.View([]api.PackedInstruction{wire})
		if err != nil {
			fmt.Println("OrganizationStatus view err:", err)
		} else if st, err := gen.Identity.OrganizationStatus.DecodeView(res.HTTPResponseBody); err != nil {
			fmt.Println("OrganizationStatus decode err (org likely absent):", err)
		} else {
			fmt.Printf("OrganizationStatus: %s (index %d)\n", st.Variant, st.Index)
		}
	}

	// 3) Organization capabilities (declared roles + credential_schemas)
	{
		wire, _ := gen.Identity.OrganizationCapabilities.Args(issuer).Encode()
		res, err := client.View([]api.PackedInstruction{wire})
		if err != nil {
			fmt.Println("OrganizationCapabilities view err:", err)
		} else if cap, err := gen.Identity.OrganizationCapabilities.DecodeView(res.HTTPResponseBody); err != nil {
			fmt.Println("OrganizationCapabilities decode err (org likely absent):", err)
		} else {
			fmt.Printf("Roles: %v\n", cap.Roles)
			fmt.Printf("CredentialSchemas: %v\n", cap.CredentialSchemas)
		}
	}
}
