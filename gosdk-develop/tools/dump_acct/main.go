package main

import (
	"encoding/hex"
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

const issuerStr = "4WJqwFRPc2wCxrUwBt8ts5KEo54u"

func main() {
	issuer, err := crypto.NewAddressFromRelaxed(issuerStr)
	if err != nil {
		panic(err)
	}
	client := milon.NewClient(milon.DevNet)
	res, err := client.GetAccount(issuer)
	if err != nil {
		panic(err)
	}
	body := res.HTTPResponseBody
	fmt.Printf("GetAccount raw body length = %d\n", len(body))
	fmt.Printf("GetAccount raw body (hex) = %s\n", hex.EncodeToString(body))
	// also show as printable where possible
	fmt.Printf("GetAccount raw body (printable) = %q\n", string(body))
}
