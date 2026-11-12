package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
)

// DisplayGetAccount prints the account view resolved from a relaxed address.
func DisplayGetAccount(client *milon.Client, accountRelaxed any) error {
	fmt.Printf("\n================ Display GetAccount ================\n\n")

	getAccountResult, err := client.GetAccount(accountRelaxed)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	fmt.Printf("Address : %v \n", getAccountResult.BodyAccountView.Address)
	fmt.Printf("PublicKeysBs58 : %v \n", getAccountResult.BodyAccountView.PublicKeysBs58)
	fmt.Printf("Threshold : %v \n", getAccountResult.BodyAccountView.Threshold)

	fmt.Printf("\n================ Display GetAccount ================\n")
	return nil
}
