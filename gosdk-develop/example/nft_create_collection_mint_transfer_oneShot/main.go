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

	// collection: Signer for CreateCollection
	collectionSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	collectionPk := collectionSk.Ed25519Public()
	collectionAddress, _ := crypto.NewAddressFromPublicKey(collectionPk)

	// mint: Signer for CreateUnique
	mintSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	mintPk := mintSk.Ed25519Public()
	mintAddress, _ := crypto.NewAddressFromPublicKey(mintPk)

	// owner: Address (collection owner, royalty recipient, transfer target)
	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	ownerAddress, _ := crypto.NewAddressFromPublicKey(ownerPk)

	// recipient: "to" in CreateUnique, "from" (Signer) in Transfer
	recipientSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	recipientPk := recipientSk.Ed25519Public()
	recipientAddress, _ := crypto.NewAddressFromPublicKey(recipientPk)

	fmt.Printf("collectionAddress = %v \n", collectionAddress)
	fmt.Printf("mintAddress = %v \n", mintAddress)
	fmt.Printf("ownerAddress = %v \n", ownerAddress)
	fmt.Printf("recipientAddress = %v \n\n", recipientAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	// Claim faucet for collection (Signer of CreateCollection)
	if err := client.ClaimFaucet(collectionSk, *collectionAddress, milon.PubKeySignatureMode{PublicKey: *collectionPk}); err != nil {
		panic("Failed to ClaimFaucet collection:" + err.Error())
	}
	collectionBalance, err := client.AddressBalance(*collectionAddress)
	if err != nil {
		panic("Failed to get collection MIL:" + err.Error())
	}
	fmt.Printf("collection MIL: %d\n", collectionBalance)

	// Claim faucet for mint (Signer of CreateUnique)
	if err := client.ClaimFaucet(mintSk, *mintAddress, milon.PubKeySignatureMode{PublicKey: *mintPk}); err != nil {
		panic("Failed to ClaimFaucet mint:" + err.Error())
	}
	mintBalance, err := client.AddressBalance(*mintAddress)
	if err != nil {
		panic("Failed to get mint MIL:" + err.Error())
	}
	fmt.Printf("mint MIL: %d\n", mintBalance)

	// Claim faucet for recipient (Signer of Transfer)
	if err := client.ClaimFaucet(recipientSk, *recipientAddress, milon.PubKeySignatureMode{PublicKey: *recipientPk}); err != nil {
		panic("Failed to ClaimFaucet recipient:" + err.Error())
	}
	recipientBalance, err := client.AddressBalance(*recipientAddress)
	if err != nil {
		panic("Failed to get recipient MIL:" + err.Error())
	}
	fmt.Printf("recipient MIL: %d\n", recipientBalance)

	// ====== 5. Simulate CreateCollection ======
	collectionMetadata := map[string]any{
		"name":     "SDK NFT Collection",
		"symbol":   "SNFT",
		"base_uri": "https://milon.test/nft/",
	}

	simulateCreateCollectionResult, err := client.BuildAndSimulateSingleIxSplit(
		"nft",
		"CreateCollection",
		provider.Args{
			"collection": collectionAddress,
			"owner":      ownerAddress,
			"metadata":   collectionMetadata,
		},
		*collectionAddress,
		milon.PubKeySignatureMode{PublicKey: *collectionPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create transaction for simulation: %v", err))
	}
	if simulateCreateCollectionResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("simulation failed: %s", simulateCreateCollectionResult.BodySimulateReceipt.Error.Message))
	}
	fmt.Printf("\n================ Simulation CreateCollection ================\n")
	fmt.Printf("Total gas fee: %d\n", simulateCreateCollectionResult.BodySimulateReceipt.GasCharged)

	// ====== 6. Submit CreateCollection ======
	submitCreateCollectionResult, err := client.BuildAndSubmitSingleIxSplit(
		"nft",
		"CreateCollection",
		provider.Args{
			"collection": collectionAddress,
			"owner":      ownerAddress,
			"metadata":   collectionMetadata,
		},
		collectionSk,
		*collectionAddress,
		milon.PubKeySignatureMode{PublicKey: *collectionPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// ====== 7. Wait for CreateCollection ======
	fmt.Printf("\nAnd we wait for the CreateCollection transaction %s to complete...\n", submitCreateCollectionResult.BodyTxHash)
	getCreateCollectionTxResult, err := client.WaitForTransaction(submitCreateCollectionResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getCreateCollectionTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getCreateCollectionTxResult.BodyTxHistory.Receipt.Error))
	}

	// ====== 8. View CollectionMetadata ======
	viewCollectionMetadataResult, err := client.BuildAndViewSingleIx(
		"nft",
		"CollectionMetadata",
		provider.Args{
			"collection": collectionAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view CollectionMetadata: " + err.Error())
	}
	if failure, ok := viewCollectionMetadataResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("CollectionMetadata RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("\n================ CollectionMetadata ================\n")
		fmt.Printf("collection metadata: %+v \n", viewCollectionMetadataResult.BodyValues)
	}

	// ====== 9. Simulate CreateUnique ======
	metadata := map[string]any{
		"name":         "SDK NFT #1",
		"symbol":       "SNFT1",
		"uri":          "https://milon.test/nft/1.json",
		"external_url": "https://milon.test/nft/1",
		"attributes": []any{
			map[string]any{
				"trait_type": "Color",
				"value":      "Blue",
			},
		},
		"properties": []any{
			map[string]any{
				"key":   "rarity",
				"value": "rare",
			},
		},
	}

	royalty := map[string]any{
		"recipient": ownerAddress,
		"bps":       uint16(500),
	}

	simulateCreateUniqueResult, err := client.BuildAndSimulateSingleIxSplit(
		"nft",
		"CreateUnique",
		provider.Args{
			"collection": collectionAddress,
			"mint":       mintAddress,
			"to":         recipientAddress,
			"metadata":   metadata,
			"royalty":    royalty,
		},
		*mintAddress,
		milon.PubKeySignatureMode{PublicKey: *mintPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create transaction for simulation: %v", err))
	}
	if simulateCreateUniqueResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("simulation failed: %s", simulateCreateUniqueResult.BodySimulateReceipt.Error.Message))
	}
	fmt.Printf("\n================ Simulation CreateUnique ================\n")
	fmt.Printf("Total gas fee: %d\n", simulateCreateUniqueResult.BodySimulateReceipt.GasCharged)

	// ====== 10. Submit CreateUnique ======
	submitCreateUniqueResult, err := client.BuildAndSubmitSingleIxSplit(
		"nft",
		"CreateUnique",
		provider.Args{
			"collection": collectionAddress,
			"mint":       mintAddress,
			"to":         recipientAddress,
			"metadata":   metadata,
			"royalty":    royalty,
		},
		mintSk,
		*mintAddress,
		milon.PubKeySignatureMode{PublicKey: *mintPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// ====== 11. Wait for CreateUnique ======
	fmt.Printf("\nAnd we wait for the CreateUnique transaction %s to complete...\n", submitCreateUniqueResult.BodyTxHash)
	getCreateUniqueTxResult, err := client.WaitForTransaction(submitCreateUniqueResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getCreateUniqueTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getCreateUniqueTxResult.BodyTxHistory.Receipt.Error))
	}

	// ====== 12. View TotalSupply ======
	viewTotalSupplyResult, err := client.BuildAndViewSingleIx(
		"nft",
		"TotalSupply",
		provider.Args{
			"mint": mintAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view TotalSupply: " + err.Error())
	}
	if failure, ok := viewTotalSupplyResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("TotalSupply RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("\n================ TotalSupply ================\n")
		fmt.Printf("total supply: %+v \n", viewTotalSupplyResult.BodyValues.(uint64))
	}

	// ====== 13. Submit Transfer ======
	submitTransferResult, err := client.BuildAndSubmitSingleIxSplit(
		"nft",
		"Transfer",
		provider.Args{
			"from":   recipientAddress,
			"mint":   mintAddress,
			"to":     ownerAddress,
			"amount": uint64(1),
		},
		recipientSk,
		*recipientAddress,
		milon.PubKeySignatureMode{PublicKey: *recipientPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// ====== 14. Wait for Transfer ======
	fmt.Printf("\nAnd we wait for the Transfer transaction %s to complete...\n", submitTransferResult.BodyTxHash)
	getTransferTxResult, err := client.WaitForTransaction(submitTransferResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getTransferTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getTransferTxResult.BodyTxHistory.Receipt.Error))
	}

	// ====== 15. View BalanceOf ======
	viewBalanceOfResult, err := client.BuildAndViewSingleIx(
		"nft",
		"BalanceOf",
		provider.Args{
			"mint":  mintAddress,
			"owner": ownerAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view BalanceOf: " + err.Error())
	}
	if failure, ok := viewBalanceOfResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("BalanceOf RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("\n================ BalanceOf ================\n")
		fmt.Printf("owner balance: %+v \n", viewBalanceOfResult.BodyValues.(uint64))
	}

	// Display TxHistory
	helper.DisplayTxHistoryAndGetResourceAndGetAccessValue(client, getTransferTxResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTransferTxResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, submitTransferResult.BodyTxHash, nil)
	}
}
