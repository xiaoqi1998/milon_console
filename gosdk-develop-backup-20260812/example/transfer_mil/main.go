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

	senderSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	senderPk := senderSk.Ed25519Public()
	senderAddress, _ := crypto.NewAddressFromPublicKey(senderSk.Ed25519Public())

	recipientSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	recipientPk := recipientSk.Ed25519Public()
	recipientAddress, _ := crypto.NewAddressFromPublicKey(recipientPk)

	fmt.Printf("senderAddress = %v \n", senderAddress)
	fmt.Printf("recipientAddress = %v \n\n", recipientAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(senderSk, *senderAddress, lib.PubKeySignatureMode{PublicKey: *senderPk}); err != nil {
		panic("Failed to ClaimFaucet token:" + err.Error())
	}
	senderBalance, err := client.BalanceOf(*senderAddress)
	if err != nil {
		panic("Failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", senderBalance)

	// 1. Encode instruction
	pd, err := client.GetPdByIDLAppName("token")
	if err != nil {
		panic("Failed to get IDL provider for 'token':" + err.Error())
	}

	wire, err := pd.Encode("Transfer", provider.Args{
		"from":   senderAddress,
		"token":  api.MIL,
		"to":     recipientAddress,
		"amount": 300,
	})
	if err != nil {
		panic("Failed to encode Transfer instruction:" + err.Error())
	}

	// 2. Define signing slots once (shared by simulate & real sign)
	slots := []lib.SigningSlot{
		{*senderAddress, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *senderPk}},
	}

	// 3. Build transaction once, reuse the same builder for simulate & real sign (same Stamp -> same TxHash)
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).ApplySlots(slots)

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
			lib.Signer{SecretKey: senderSk, PublicKey: *senderPk},
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
	senderBalance, err = client.BalanceOf(*senderAddress)
	if err != nil {
		panic("Failed to get sender MIL:" + err.Error())
	}
	fmt.Printf("sender MIL: %d\n", senderBalance)
	recipientBalance, err := client.BalanceOf(*recipientAddress)
	if err != nil {
		panic("Failed to get recipient MIL:" + err.Error())
	}
	fmt.Printf("recipient MIL: %d\n", recipientBalance)

	// Display TxHistory
	helper.DisplayTxHistory(client, getTxByHashResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, tx.TxHash, nil)
	}
}
