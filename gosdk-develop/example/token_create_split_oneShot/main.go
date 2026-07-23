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

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	tokenAddress, _ := crypto.NewAddressFromPublicKey(tokenSk.Ed25519Public())

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	ownerAddress, _ := crypto.NewAddressFromPublicKey(ownerPk)

	fmt.Printf("tokenAddress = %v \n", tokenAddress)
	fmt.Printf("ownerAddress = %v \n\n", ownerAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(tokenSk, *tokenAddress, milon.PubKeySignatureMode{PublicKey: *tokenPk}); err != nil {
		panic("Failed to ClaimFaucet token:" + err.Error())
	}
	tokenBalance, err := client.AddressBalance(*tokenAddress)
	if err != nil {
		panic("Failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", tokenBalance)

	// 1. Simulate transaction
	simulateTransactionResult, err := client.BuildAndSimulateSingleIxSplit(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Multi Ix Token",
				"symbol":   "SMIX",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		*tokenAddress,
		milon.PubKeySignatureMode{PublicKey: *tokenPk},
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

	// 2. Create on-chain transaction
	submitTransactionResult, err := client.BuildAndSubmitSingleIxSplit(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Multi Ix Token",
				"symbol":   "SMIX",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		tokenSk,
		*tokenAddress,
		milon.PubKeySignatureMode{PublicKey: *tokenPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// 3. Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitTransactionResult.BodyTxHash)
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getTxByHashResult.BodyTxHistory.Receipt.Error))
	}

	fmt.Printf("\n================ Final MIL ================\n")
	tokenBalance, err = client.AddressBalance(*tokenAddress)
	if err != nil {
		panic("Failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", tokenBalance)

	// Display TxHistory
	helper.DisplayTxHistoryAndGetResourceAndGetAccessValue(client, getTxByHashResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, submitTransactionResult.BodyTxHash, nil)
	}
}
