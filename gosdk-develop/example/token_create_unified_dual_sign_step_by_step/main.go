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
	tokenAddress, _ := crypto.NewAddressFromPublicKey(tokenPk)

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
	if err := client.ClaimFaucet(payerSk, *payerAddress, milon.PubKeySignatureMode{PublicKey: *payerPk}); err != nil {
		panic("Failed to ClaimFaucet payer:" + err.Error())
	}
	payerBalance, err := client.AddressBalance(*payerAddress)
	if err != nil {
		panic("Failed to get payer MIL:" + err.Error())
	}
	fmt.Printf("payer MIL: %d\n", payerBalance)

	// 1. Look up token IDL provider (loaded during NewMilonClient)
	pd, err := client.GetPdByIDLAppName("token")
	if err != nil {
		panic(fmt.Sprintf("failed to get IDL provider for 'token': %v", err))
	}

	// 2. Encode a single instruction
	wire, err := pd.Encode(
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
	)
	if err != nil {
		panic(fmt.Sprintf("failed to encode Create instruction: %v", err))
	}

	// 3. Simulate transaction
	simulateTransaction, err := client.CreateTransactionWithParam([]api.PackedInstruction{wire}, payerAddress)
	if err != nil {
		panic(fmt.Sprintf("failed to create transaction for simulation: %v", err))
	}

	payerSig, err := simulateTransaction.SimulateSignPayer(*payerAddress, milon.PubKeySignatureMode{PublicKey: *payerPk})
	if err != nil {
		panic(fmt.Sprintf("failed to simulate payer signature: %v", err))
	}
	simulateTransaction.AddSignature(*payerAddress, *payerSig)

	tokenSig, err := simulateTransaction.SimulateSignIx(*tokenAddress, 0, milon.PubKeySignatureMode{PublicKey: *tokenPk})
	if err != nil {
		panic(fmt.Sprintf("failed to simulate ix signature for token: %v", err))
	}
	simulateTransaction.AddSignature(*tokenAddress, *tokenSig)

	simulateTransactionPostcard, err := simulateTransaction.ToBytes()
	if err != nil {
		panic(fmt.Sprintf("failed to serialize simulated transaction: %v", err))
	}
	simulateTransactionResult, err := client.SimulateTx(simulateTransactionPostcard, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to submit simulation: %v", err))
	}
	if simulateTransactionResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("simulation failed: %s", simulateTransactionResult.BodySimulateReceipt.Error.Message))
	}
	fmt.Printf("\n================ Simulation ================\n")
	fmt.Printf("Total gas fee: %d\n", simulateTransactionResult.BodySimulateReceipt.GasCharged)

	// 4. Create on-chain transaction
	transaction, err := client.CreateTransactionWithParam([]api.PackedInstruction{wire}, payerAddress)
	if err != nil {
		panic(fmt.Sprintf("failed to create on-chain transaction: %v", err))
	}

	// 5. Sign as payer and attach signature
	err = client.SignPayerAndAddSignature(transaction, payerSk, *payerAddress, milon.PubKeySignatureMode{PublicKey: *payerPk})
	if err != nil {
		panic(fmt.Sprintf("failed to sign transaction as payer: %v", err))
	}

	// 6. Sign as instruction signer and attach signature
	err = client.SignIxAndAddSignature(transaction, 0, tokenSk, *tokenAddress, milon.PubKeySignatureMode{PublicKey: *tokenPk})
	if err != nil {
		panic(fmt.Sprintf("failed to sign instruction 0 as token: %v", err))
	}

	// 7. Validate transaction structure
	err = transaction.ValidateWire()
	if err != nil {
		panic(fmt.Sprintf("transaction validation failed: %v", err))
	}

	// 8. Serialize and submit to chain
	transactionPostcard, err := transaction.ToBytes()
	if err != nil {
		panic(fmt.Sprintf("failed to serialize transaction: %v", err))
	}

	submitTransactionResult, err := client.SubmitTx(transactionPostcard, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to submit transaction: %v", err))
	}
	fmt.Println("txHash =", submitTransactionResult.BodyTxHash)

	// 9. Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitTransactionResult.BodyTxHash)
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getTxByHashResult.BodyTxHistory.Receipt.Error))
	}

	fmt.Printf("\n================ Final MIL ================\n")
	payerBalance, err = client.AddressBalance(*payerAddress)
	if err != nil {
		panic("Failed to get payer MIL:" + err.Error())
	}
	fmt.Printf("payer MIL: %d\n", payerBalance)

	// Display TxHistory
	helper.DisplayTxHistoryAndGetResourceAndGetAccessValue(client, getTxByHashResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, submitTransactionResult.BodyTxHash, nil)
	}
}
