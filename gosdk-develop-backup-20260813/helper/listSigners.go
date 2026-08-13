package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

func DisplayAccountGetListSigners(client *milon.Client, account *crypto.Address) {
	fmt.Printf("\n================ Display Account.GetListSigners ================\n\n")

	signers, err := client.ListAccountSigners(account)
	if err != nil {
		panic("failed to get account list signers:" + err.Error())
	}
	fmt.Printf("ListSigners: %+v \n", signers)

	fmt.Printf("\n================ Display Account.GetListSigners ================\n")
}
