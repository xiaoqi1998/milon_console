package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

// DisplayAccountGetListSigners prints the signer list of an account.
func DisplayAccountGetListSigners(client *milon.Client, account *crypto.Address) error {
	fmt.Printf("\n================ Display Account.GetListSigners ================\n\n")

	signers, err := client.ListAccountSigners(account)
	if err != nil {
		return fmt.Errorf("failed to get account list signers: %w", err)
	}
	fmt.Printf("ListSigners: %+v \n", signers)

	fmt.Printf("\n================ Display Account.GetListSigners ================\n")
	return nil
}
