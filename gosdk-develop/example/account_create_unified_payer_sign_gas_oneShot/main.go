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

	userSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	userPk := userSk.Ed25519Public()
	userAddress, _ := crypto.NewAddressFromPublicKey(userPk)

	fmt.Printf("userPk = %v \n", userPk)
	fmt.Printf("userAddress = %v \n\n", userAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(userSk, *userAddress, milon.PubKeySignatureMode{PublicKey: *userPk}); err != nil {
		panic("Failed to ClaimFaucet user:" + err.Error())
	}
	userBalance, err := client.AddressBalance(*userAddress)
	if err != nil {
		panic("Failed to get user MIL:" + err.Error())
	}
	fmt.Printf("user MIL: %d\n", userBalance)

	// 1. Simulate transaction
	simulateTransactionResult, err := client.BuildAndSimulateSingleIxUnifiedPayerOnlyGas(
		"account",
		"Create",
		provider.Args{
			"owner_pk": userPk,
		},
		*userAddress,
		milon.PubKeySignatureMode{PublicKey: *userPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create transaction for simulation: %v", err))
	}
	if simulateTransactionResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Failed to simulate transaction: %s", simulateTransactionResult.BodySimulateReceipt.Error.Message))
	}
	fmt.Printf("\n================ Simulation ================\n")
	fmt.Printf("Total gas fee: %d\n", simulateTransactionResult.BodySimulateReceipt.GasCharged)

	// 2. Create on-chain transaction
	submitTransactionResult, err := client.BuildAndSubmitSingleIxUnifiedPayerOnlyGas(
		"account",
		"Create",
		provider.Args{
			"owner_pk": userPk,
		},
		userSk,
		*userAddress,
		milon.PubKeySignatureMode{PublicKey: *userPk},
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
	userBalance, err = client.AddressBalance(*userAddress)
	if err != nil {
		panic("Failed to get user MIL:" + err.Error())
	}
	fmt.Printf("user MIL: %d\n", userBalance)

	// Display TxHistory
	helper.DisplayTxHistoryAndGetResourceAndGetAccessValue(client, getTxByHashResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, submitTransactionResult.BodyTxHash, nil)
	}

	// Display GetAccount
	helper.DisplayGetAccount(client, userAddress.ToBase58())
}
