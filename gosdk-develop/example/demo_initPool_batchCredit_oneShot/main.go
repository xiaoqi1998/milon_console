package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/helper"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func example(networkConfig milon.NetworkConfig) {
	client := milon.NewMilonClient(networkConfig)

	poolSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	poolPk := poolSk.Ed25519Public()
	poolAddress, _ := crypto.NewAddressFromPublicKey(poolPk)

	recipientSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	recipientPk := recipientSk.Ed25519Public()
	recipientAddress, _ := crypto.NewAddressFromPublicKey(recipientPk)

	fmt.Printf("poolAddress = %v \n", poolAddress)
	fmt.Printf("recipientAddress = %v \n\n", recipientAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(poolSk, *poolAddress, milon.PubKeySignatureMode{PublicKey: *poolPk}); err != nil {
		panic("Failed to ClaimFaucet pool:" + err.Error())
	}
	poolBalance, err := client.AddressBalance(*poolAddress)
	if err != nil {
		panic("Failed to get pool MIL:" + err.Error())
	}
	fmt.Printf("pool MIL: %d\n", poolBalance)

	// 1. Look up "demo" IDL provider (loaded during NewMilonClient)
	pd, err := client.GetPdByIDLAppName("demo")
	if err != nil {
		panic(fmt.Sprintf("failed to get demo IDL provider: %v", err))
	}

	// 2. Encode instructions (InitPool + BatchCredit)
	initPoolWire, err := pd.Encode("InitPool", provider.Args{
		"pool":  poolAddress,
		"label": "InitPool-label",
	})
	if err != nil {
		panic(fmt.Sprintf("failed to encode InitPool instruction: %v", err))
	}
	batchCreditWire, err := pd.Encode("BatchCredit", provider.Args{
		"pool":       poolAddress,
		"recipients": []crypto.Address{*recipientAddress},
		"amount":     123,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to encode BatchCredit instruction: %v", err))
	}

	// 3. Simulate transaction
	simulateTransactionResult, err := client.BuildAndSimulateMultiIxUnified(
		[]api.PackedInstruction{
			initPoolWire,
			batchCreditWire,
		},
		*poolAddress,
		milon.PubKeySignatureMode{PublicKey: *poolPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create transaction for simulation: %v", err))
	}
	if simulateTransactionResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("simulation failed: %s", simulateTransactionResult.BodySimulateReceipt.Error.Message))
	}
	fmt.Printf("\n================ Simulation ================\n")
	fmt.Printf("Total gas fee: %d\n", simulateTransactionResult.BodySimulateReceipt.GasCharged)

	// 4. Create on-chain transaction
	submitTransactionResult, err := client.BuildAndSubmitMultiIxUnified(
		[]api.PackedInstruction{
			initPoolWire,
			batchCreditWire,
		},
		poolSk,
		*poolAddress,
		milon.PubKeySignatureMode{PublicKey: *poolPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// 5. Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitTransactionResult.BodyTxHash)
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getTxByHashResult.BodyTxHistory.Receipt.Error))
	}

	fmt.Printf("\n================ Final MIL ================\n")
	poolBalance, err = client.AddressBalance(*poolAddress)
	if err != nil {
		panic("Failed to get pool MIL:" + err.Error())
	}
	fmt.Printf("pool MIL: %d\n", poolBalance)

	// Display TxHistory
	helper.DisplayTxHistoryAndGetResourceAndGetAccessValue(client, getTxByHashResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, submitTransactionResult.BodyTxHash, nil)
	}
}
