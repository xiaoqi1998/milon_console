package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
)

func DisplayGetAccount(client *milon.Client, accountRelaxed any) {
	fmt.Printf("\n================ Display GetAccount ================\n\n")

	getAccountResult, err := client.GetAccount(accountRelaxed)
	if err != nil {
		panic("failed to get account: " + err.Error())
	}

	fmt.Printf("Address : %v \n", getAccountResult.BodyAccountView.Address)
	fmt.Printf("PublicKeysBs58 : %v \n", getAccountResult.BodyAccountView.PublicKeysBs58)
	fmt.Printf("Threshold : %v \n", getAccountResult.BodyAccountView.Threshold)

	fmt.Printf("\n================ Display GetAccount ================\n")
}
