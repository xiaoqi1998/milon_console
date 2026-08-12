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

	poolSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	poolPk := poolSk.Ed25519Public()
	poolAddress, _ := crypto.NewAddressFromPublicKey(poolPk)

	recipientSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	recipientPk := recipientSk.Ed25519Public()
	recipientAddress, _ := crypto.NewAddressFromPublicKey(recipientPk)

	fmt.Printf("poolAddress = %v \n", poolAddress)
	fmt.Printf("recipientAddress = %v \n\n", recipientAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(poolSk, *poolAddress, lib.PubKeySignatureMode{PublicKey: *poolPk}); err != nil {
		panic("Failed to ClaimFaucet pool:" + err.Error())
	}
	poolBalance, err := client.BalanceOf(*poolAddress)
	if err != nil {
		panic("Failed to get pool MIL:" + err.Error())
	}
	fmt.Printf("pool MIL: %d\n", poolBalance)

	// 1. Encode instructions (InitPool + BatchCredit)
	pd, err := client.GetPdByIDLAppName("demo")
	if err != nil {
		panic("Failed to get IDL provider for 'demo':" + err.Error())
	}

	initPoolWire, err := pd.Encode("InitPool", provider.Args{
		"pool":  poolAddress,
		"label": "InitPool-label",
	})
	if err != nil {
		panic("Failed to encode InitPool instruction:" + err.Error())
	}
	batchCreditWire, err := pd.Encode("BatchCredit", provider.Args{
		"pool":       poolAddress,
		"recipients": []crypto.Address{*recipientAddress},
		"amount":     123,
	})
	if err != nil {
		panic("Failed to encode BatchCredit instruction:" + err.Error())
	}

	// 2. Define signing slots once (shared by simulate & real sign)
	slots := []lib.SigningSlot{
		{*poolAddress, []uint8{0, 1}, true, lib.PubKeySignatureMode{PublicKey: *poolPk}},
	}

	// 3. Build transaction once, reuse the same builder for simulate & real sign (same Stamp -> same TxHash)
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{initPoolWire, batchCreditWire}).ApplySlots(slots)

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
			lib.Signer{SecretKey: poolSk, PublicKey: *poolPk},
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
	poolBalance, err = client.BalanceOf(*poolAddress)
	if err != nil {
		panic("Failed to get pool MIL:" + err.Error())
	}
	fmt.Printf("pool MIL: %d\n", poolBalance)

	// Display TxHistory
	helper.DisplayTxHistory(client, getTxByHashResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, tx.TxHash(), nil)
	}
}
