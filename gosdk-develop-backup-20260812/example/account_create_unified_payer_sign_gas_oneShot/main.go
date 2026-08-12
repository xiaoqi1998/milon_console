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

	userSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	userPk := userSk.Ed25519Public()
	userAddress, _ := crypto.NewAddressFromPublicKey(userPk)

	fmt.Printf("userPk = %v \n", userPk)
	fmt.Printf("userAddress = %v \n\n", userAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(userSk, *userAddress, lib.PubKeySignatureMode{PublicKey: *userPk}); err != nil {
		panic("Failed to ClaimFaucet user:" + err.Error())
	}
	userBalance, err := client.BalanceOf(*userAddress)
	if err != nil {
		panic("Failed to get user MIL:" + err.Error())
	}
	fmt.Printf("user MIL: %d\n", userBalance)

	// 1. Encode instruction
	pd, err := client.GetPdByIDLAppName("account")
	if err != nil {
		panic("Failed to get IDL provider for 'account':" + err.Error())
	}

	wire, err := pd.Encode("Create", provider.Args{
		"owner_pk": userPk,
	})
	if err != nil {
		panic("Failed to encode Create instruction:" + err.Error())
	}

	// 2. Define signing slots once (shared by simulate & real sign)
	slots := []lib.SigningSlot{
		{*userAddress, nil, false, lib.PubKeySignatureMode{PublicKey: *userPk}},
	}
	// 3. Build transaction once, reuse the same builder for simulate & real sign (same Stamp -> same TxHash)
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(userAddress).ApplySlots(slots)

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
			lib.Signer{SecretKey: userSk, PublicKey: *userPk},
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
	userBalance, err = client.BalanceOf(*userAddress)
	if err != nil {
		panic("Failed to get user MIL:" + err.Error())
	}
	fmt.Printf("user MIL: %d\n", userBalance)

	// Display TxHistory
	helper.DisplayTxHistory(client, getTxByHashResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, tx.TxHash(), nil)
	}

	// Display GetAccount
	helper.DisplayGetAccount(client, userAddress.ToBase58())
}
