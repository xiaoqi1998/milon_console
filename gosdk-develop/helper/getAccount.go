package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
)

func DisplayGetAccount(client *milon.MolinClient, addressBs58 string) {
	fmt.Printf("\n================ Display GetAccount ================\n")

	getAccountResult, err := client.GetAccount(addressBs58, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("getAccountResult : %+v \n\n", getAccountResult)
	fmt.Printf("getAccountResult.BodyAccountView : %+v \n\n", getAccountResult.BodyAccountView)

	fmt.Printf("Address : %v \n", getAccountResult.BodyAccountView.Address)
	fmt.Printf("PublicKeysBs58 : %v \n", getAccountResult.BodyAccountView.PublicKeysBs58)
	fmt.Printf("Threshold : %v \n", getAccountResult.BodyAccountView.Threshold)

	fmt.Printf("\n================ Display GetAccount ================\n")
}
