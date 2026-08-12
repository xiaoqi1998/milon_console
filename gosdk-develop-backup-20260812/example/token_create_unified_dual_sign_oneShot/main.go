package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/helper"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	tokenAddress, _ := crypto.NewAddressFromPublicKey(tokenSk.Ed25519Public())

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	ownerAddress, _ := crypto.NewAddressFromPublicKey(ownerPk)

	payerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	payerPk := payerSk.Ed25519Public()
	payerAddress, _ := crypto.NewAddressFromPublicKey(payerPk)

	fmt.Printf("tokenAddress = %v \n", tokenAddress)
	fmt.Printf("ownerAddress = %v \n", ownerAddress)
	fmt.Printf("payerAddress = %v \n\n", payerAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(payerSk, *payerAddress, lib.PubKeySignatureMode{PublicKey: *payerPk}); err != nil {
		panic("Failed to ClaimFaucet payer:" + err.Error())
	}
	payerBalance, err := client.BalanceOf(*payerAddress)
	if err != nil {
		panic("Failed to get payer MIL:" + err.Error())
	}
	if err = client.ClaimFaucet(tokenSk, *tokenAddress, lib.PubKeySignatureMode{PublicKey: *tokenPk}); err != nil {
		panic("Failed to ClaimFaucet token:" + err.Error())
	}
	tokenBalance, err := client.BalanceOf(*tokenAddress)
	if err != nil {
		panic("Failed to get token MIL:" + err.Error())
	}

	fmt.Printf("payer MIL: %d\n", payerBalance)
	fmt.Printf("token MIL: %d\n", tokenBalance)

	// 1. Encode instruction
	pd, err := client.GetPdByIDLAppName("token")
	if err != nil {
		panic("Failed to get IDL provider for 'token':" + err.Error())
	}

	wire, err := pd.Encode("Create", provider.Args{
		"token": tokenAddress,
		"owner": ownerAddress,
		"metadata": map[string]any{
			"name":     "Example Token",
			"symbol":   "Token",
			"decimals": 6,
			"icon":     "https://milon.test/token.png",
		},
	})
	if err != nil {
		panic("Failed to encode Create instruction:" + err.Error())
	}

	// 2. Define signing slots once (shared by simulate & real sign)
	slots := []lib.SigningSlot{
		{*payerAddress, nil, false, lib.PubKeySignatureMode{PublicKey: *payerPk}},
		{*tokenAddress, []uint8{0}, false, lib.PubKeySignatureMode{PublicKey: *tokenPk}},
	}

	// 3. Build transaction once, reuse the same builder for simulate & real sign (same Stamp -> same TxHash)
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(payerAddress).ApplySlots(slots)

	// 3.1 Simulate on-chain first (no private key needed, dry-run)
	simulateTx, err := builder.SimulateSlots().Build()
	if err != nil {
		panic("Failed to simulate transaction:" + err.Error())
	}
	simulateResult, err := client.SimulateTx(simulateTx)
	if err != nil {
		panic("Failed to simulate transaction on chain:" + err.Error())
	}
	if simulateResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Simulate failed on chain: error code = %d", simulateResult.BodySimulateReceipt.Error.Code))
	}
	fmt.Printf("Simulated transaction hash: %s, gas charged: %d\n", simulateTx.TxHash(), simulateResult.BodySimulateReceipt.GasCharged)

	// 3.2 Replace simulated signatures with real ones on the same transaction
	tx, err := builder.ResetSigs().
		SignWith(
			lib.Signer{SecretKey: tokenSk, PublicKey: *tokenPk},
			lib.Signer{SecretKey: payerSk, PublicKey: *payerPk},
		).
		Build()
	if err != nil {
		panic("Failed to build and sign transaction:" + err.Error())
	}

	// 4. Submit transaction on chain
	err = client.SubmitTx(tx)
	if err != nil {
		panic("Failed to submit transaction:" + err.Error())
	}

	// 5. Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash(), 1)
	if err != nil {
		panic("Failed to wait for transaction:" + err.Error())
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Transaction failed on chain: error code = %d", *getTxByHashResult.BodyTxHistory.Receipt.Error))
	}
	fmt.Printf("Submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	fmt.Printf("\n================ Final MIL ================\n")
	payerBalance, err = client.BalanceOf(*payerAddress)
	if err != nil {
		panic("Failed to get payer MIL:" + err.Error())
	}
	tokenBalance, err = client.BalanceOf(*tokenAddress)
	if err != nil {
		panic("Failed to get token MIL:" + err.Error())
	}

	fmt.Printf("payer MIL: %d\n", payerBalance)
	fmt.Printf("token MIL: %d\n", tokenBalance)

	helper.DisplayTxHistory(client, getTxByHashResult.BodyTxHistory)
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, tx.TxHash(), nil)
	}
}
