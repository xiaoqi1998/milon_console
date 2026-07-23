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

	subjectSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	subjectPk := subjectSk.Ed25519Public()
	subjectAddress, _ := crypto.NewAddressFromPublicKey(subjectPk)

	fmt.Printf("subjectAddress = %v \n\n", subjectAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(subjectSk, *subjectAddress, milon.PubKeySignatureMode{PublicKey: *subjectPk}); err != nil {
		panic("Failed to ClaimFaucet subject:" + err.Error())
	}
	subjectBalance, err := client.AddressBalance(*subjectAddress)
	if err != nil {
		panic("Failed to get subject MIL:" + err.Error())
	}
	fmt.Printf("subject MIL: %d\n", subjectBalance)

	// Build the DidDocumentInput
	doc := map[string]any{
		"entity_type": "Person",
		"keys": []any{
			map[string]any{
				"key_type":   "Ed25519",
				"public_key": subjectPk.ToBase58(),
			},
		},
		"authentication": []any{
			map[string]any{
				"key_id":   "key-1",
				"key_type": "Ed25519",
			},
		},
	}

	// 1. Simulate transaction
	simulateTransactionResult, err := client.BuildAndSimulateSingleIxSplit(
		"identity",
		"Create",
		provider.Args{
			"subject": subjectAddress,
			"doc":     doc,
		},
		*subjectAddress,
		milon.PubKeySignatureMode{PublicKey: *subjectPk},
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
		"identity",
		"Create",
		provider.Args{
			"subject": subjectAddress,
			"doc":     doc,
		},
		subjectSk,
		*subjectAddress,
		milon.PubKeySignatureMode{PublicKey: *subjectPk},
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
	subjectBalance, err = client.AddressBalance(*subjectAddress)
	if err != nil {
		panic("Failed to get subject MIL:" + err.Error())
	}
	fmt.Printf("subject MIL: %d\n", subjectBalance)

	// Display TxHistory
	helper.DisplayTxHistoryAndGetResourceAndGetAccessValue(client, getTxByHashResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, submitTransactionResult.BodyTxHash, nil)
	}

	// 4. Resolve the DID
	fmt.Printf("\n================ Resolve DID ================\n")
	resolveResult, err := client.BuildAndViewSingleIx(
		"identity",
		"Resolve",
		provider.Args{
			"subject": subjectAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view Resolve: " + err.Error())
	}
	if failure, ok := resolveResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("Resolve RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("Resolve DID document: %+v \n", resolveResult.BodyValues)
	}

	// 5. Query the DID Document
	fmt.Printf("\n================ Document ================\n")
	documentResult, err := client.BuildAndViewSingleIx(
		"identity",
		"Document",
		provider.Args{
			"subject": subjectAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view Document: " + err.Error())
	}
	if failure, ok := documentResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("Document RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("DID Document: %+v \n", documentResult.BodyValues)
	}
}
