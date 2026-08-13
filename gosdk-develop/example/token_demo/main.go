package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/helper"
	"github.com/milon-labs/milon-go-sdk/lib"
)

func main() {
	example(milon.DevNet)
}

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	token, _ := crypto.NewAddressFromPublicKey(tokenPk)

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	owner, _ := crypto.NewAddressFromPublicKey(ownerPk)

	account1Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	account1Pk := account1Sk.Ed25519Public()
	account1, _ := crypto.NewAddressFromPublicKey(account1Pk)

	account2Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	account2Pk := account2Sk.Ed25519Public()
	account2, _ := crypto.NewAddressFromPublicKey(account2Pk)

	account3Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	account3Pk := account3Sk.Ed25519Public()
	account3, _ := crypto.NewAddressFromPublicKey(account3Pk)

	fmt.Printf("token = %v \n", token)
	fmt.Printf("owner = %v \n", owner)
	fmt.Printf("account1 = %v \n", account1)
	fmt.Printf("account2 = %v \n", account2)
	fmt.Printf("account3 = %v \n\n", account3)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(tokenSk, token, lib.PubKeySignatureMode{PublicKey: *tokenPk}); err != nil {
		panic("failed to ClaimFaucet MIL:" + err.Error())
	}
	tokenBalance, err := client.BalanceOf(token)
	if err != nil {
		panic("failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", tokenBalance)

	if err = client.ClaimFaucet(ownerSk, owner, lib.PubKeySignatureMode{PublicKey: *ownerPk}); err != nil {
		panic("failed to ClaimFaucet owner:" + err.Error())
	}
	ownerBalance, err := client.BalanceOf(owner)
	if err != nil {
		panic("failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	if err = client.ClaimFaucet(account1Sk, account1, lib.PubKeySignatureMode{PublicKey: *account1Pk}); err != nil {
		panic("failed to ClaimFaucet account1:" + err.Error())
	}
	account1Balance, err := client.BalanceOf(account1)
	if err != nil {
		panic("failed to get account1 MIL:" + err.Error())
	}
	fmt.Printf("account1 MIL: %d\n", account1Balance)

	// 1. Encode instructions (Create + Mint + Transfer)
	createWire, err := gen.Token.Create.Args(token, owner, gen.TokenMetadata{
		Name:     "Example Token",
		Symbol:   "Token",
		Decimals: 6,
		Icon:     "https://milon.test/token.png",
	}).Encode()
	if err != nil {
		panic("failed to encode Create instruction:" + err.Error())
	}

	mintWire, err := gen.Token.Mint.Args(token, account1, 1000).Encode()
	if err != nil {
		panic("failed to encode Mint instruction:" + err.Error())
	}

	mintWire2, err := gen.Token.Mint.Args(token, account1, 2000).Encode()
	if err != nil {
		panic("failed to encode Mint instruction:" + err.Error())
	}

	transferWire, err := gen.Token.Transfer.Args(account1, token, account2, 300).Encode()
	if err != nil {
		panic("failed to encode Transfer instruction:" + err.Error())
	}

	// 2. Build transaction once, reuse the same builder for simulate & real sign (same Stamp -> same TxHash)
	//    SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{createWire, mintWire, mintWire2, transferWire})

	// 3. Simulate on-chain first (no private key needed, dry-run)
	simulateTx, err := builder.
		AddSimulateIxesSig(*token, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *tokenPk}).
		AddSimulateIxesSig(*owner, []uint8{1, 2}, true, lib.PubKeySignatureMode{PublicKey: *ownerPk}).
		AddSimulateIxesSig(*account1, []uint8{3}, true, lib.PubKeySignatureMode{PublicKey: *account1Pk}).
		Build()
	if err != nil {
		panic("failed to simulate transaction:" + err.Error())
	}
	simulateResult, err := client.SimulateTx(simulateTx)
	if err != nil {
		panic("failed to simulate transaction on chain:" + err.Error())
	}
	helper.CheckSimulateSuccess(simulateResult)
	fmt.Printf("Simulated transaction hash: %s, gas charged: %d\n", simulateTx.TxHash(), simulateResult.BodySimulateReceipt.GasCharged)

	// 4. Real sign on the same builder (same TxHash)
	tx, err := builder.ResetSigs().
		AddIxesSig(*token, tokenSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *tokenPk}).
		AddIxesSig(*owner, ownerSk, []uint8{1, 2}, true, lib.PubKeySignatureMode{PublicKey: *ownerPk}).
		AddIxesSig(*account1, account1Sk, []uint8{3}, true, lib.PubKeySignatureMode{PublicKey: *account1Pk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}

	// 5. Submit transaction on chain
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// 6. Wait for the transaction to complete
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	fmt.Printf("\n================ Final MIL ================\n")
	tokenBalance, err = client.BalanceOf(token)
	if err != nil {
		panic("failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", tokenBalance)

	ownerBalance, err = client.BalanceOf(owner)
	if err != nil {
		panic("failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	account1Balance, err = client.BalanceOf(account1)
	if err != nil {
		panic("failed to get account1 MIL:" + err.Error())
	}
	fmt.Printf("account1 MIL: %d\n", account1Balance)

	fmt.Printf("\n================ token Balances ================\n")

	balanceAccount1Wire, err := gen.Token.BalanceOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err := client.View([]api.PackedInstruction{balanceAccount1Wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wire0ViewDecode, err := gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 balance query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 balance: %+v \n", wire0ViewDecode)
	}

	balanceAccount2Wire, err := gen.Token.BalanceOf.Args(token, account2).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{balanceAccount2Wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wire1ViewDecode, err := gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account2 balance DecodeView error:%v \n", err.Error())
	} else {
		fmt.Printf("token account2 balance: %+v \n", wire1ViewDecode)
	}

	balanceAccount3Wire, err := gen.Token.BalanceOf.Args(token, account3).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{balanceAccount3Wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wire2ViewDecode, err := gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account3 balance DecodeView error:%v \n", err.Error())
	} else {
		fmt.Printf("token account3 balance: %+v \n", wire2ViewDecode)
	}

	metadataWire, err := gen.Token.Metadata.Args(token).Encode()
	if err != nil {
		panic("failed to encode Metadata:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{metadataWire})
	if err != nil {
		panic("failed to view Metadata:" + err.Error())
	}
	wire3ViewDecode, err := gen.Token.Metadata.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token Metadata DecodeView error:%v \n", err.Error())
	} else {
		fmt.Printf("token Metadata : %+v \n", wire3ViewDecode)
	}

	totalSupplyWire, err := gen.Token.TotalSupply.Args(token).Encode()
	if err != nil {
		panic("failed to encode TotalSupply:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{totalSupplyWire})
	if err != nil {
		panic("failed to view TotalSupply:" + err.Error())
	}
	wire4ViewDecode, err := gen.Token.TotalSupply.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token TotalSupply DecodeView error:%v \n", err.Error())
	} else {
		fmt.Printf("token TotalSupply : %+v \n", wire4ViewDecode)
	}

	fmt.Printf("\n================ Now do it again, but with a different method ================\n")

	viewTxResult, err = client.View([]api.PackedInstruction{balanceAccount1Wire, balanceAccount2Wire, balanceAccount3Wire, metadataWire, totalSupplyWire})
	if err != nil {
		panic("failed to view: " + err.Error())
	}

	decodedTaggedValueList, err := client.GetProviderManager().DecodeViewDatas(
		[]string{
			"token::BalanceOf",
			"token::BalanceOf",
			"token::BalanceOf",
			"token::Metadata",
			"token::TotalSupply",
		},
		viewTxResult.HTTPResponseBody,
	)
	if err != nil {
		panic("failed to decode view data: " + err.Error())
	}

	for i, decodedTaggedValue := range decodedTaggedValueList {
		fmt.Printf("view[%d] : %+v \n", i, decodedTaggedValue)
		if failure, ok := decodedTaggedValue.Value.(*api.TxFailurePayload); ok {
			fmt.Printf("❌ err = %+v \n", failure)
		}
	}
}
