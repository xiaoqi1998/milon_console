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

	//********************************* Create Account 1 Secp256k1 		BuildAndSubmitSingleIxUnifiedPayerOnlyGas

	user1Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user1Pk, _ := user1Sk.Secp256k1Public()
	user1Address, _ := crypto.NewAddressFromPublicKey(user1Pk)

	fmt.Printf("user1Pk = %v \n", user1Pk)
	fmt.Printf("user1Address = %v \n\n", user1Address)

	if err := client.ClaimFaucet(user1Sk, *user1Address, milon.PubKeySignatureMode{PublicKey: *user1Pk}); err != nil {
		panic("Failed to ClaimFaucet user1:" + err.Error())
	}
	userBalance, err := client.AddressBalance(*user1Address)
	if err != nil {
		panic("Failed to get user1 MIL:" + err.Error())
	}
	fmt.Printf("user1 MIL: %d\n", userBalance)

	// Create on-chain transaction
	submitTransactionResult, err := client.BuildAndSubmitSingleIxUnifiedPayerOnlyGas(
		"account",
		"Create",
		provider.Args{
			"owner_pk": user1Pk,
		},
		user1Sk,
		*user1Address,
		milon.PubKeySignatureMode{PublicKey: *user1Pk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitTransactionResult.BodyTxHash)
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getTxByHashResult.BodyTxHistory.Receipt.Error))
	}

	// Display GetAccount
	helper.DisplayGetAccount(client, user1Address.ToBase58())

	//********************************* Create Account 2 Ed25519 	BuildAndSubmitSingleIxUnifiedPayerSignAll

	user2Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user2Pk := user2Sk.Ed25519Public()
	user2Address, _ := crypto.NewAddressFromPublicKey(user2Pk)

	fmt.Printf("user2Pk = %v \n", user2Pk)
	fmt.Printf("user2Address = %v \n\n", user2Address)

	if err = client.ClaimFaucet(user2Sk, *user2Address, milon.PubKeySignatureMode{PublicKey: *user2Pk}); err != nil {
		panic("Failed to ClaimFaucet user2:" + err.Error())
	}
	userBalance, err = client.AddressBalance(*user2Address)
	if err != nil {
		panic("Failed to get user2 MIL:" + err.Error())
	}
	fmt.Printf("user2 MIL: %d\n", userBalance)

	// Create on-chain transaction
	submitTransactionResult, err = client.BuildAndSubmitSingleIxUnifiedPayerSignAll(
		"account",
		"Create",
		provider.Args{
			"owner_pk": user2Pk,
		},
		user2Sk,
		*user2Address,
		milon.PubKeySignatureMode{PublicKey: *user2Pk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitTransactionResult.BodyTxHash)
	getTxByHashResult, err = client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getTxByHashResult.BodyTxHistory.Receipt.Error))
	}

	// Display GetAccount
	helper.DisplayGetAccount(client, user2Address.ToBase58())

	//********************************* Create Account 3 BLS12381 	BuildAndSubmitSingleIxSplit

	user3Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user3Pk := user3Sk.BLS12381Public()
	user3Address, _ := crypto.NewAddressFromPublicKey(user3Pk)

	fmt.Printf("user3Pk = %v \n", user3Pk)
	fmt.Printf("user3Address = %v \n\n", user3Address)

	if err = client.ClaimFaucet(user3Sk, *user3Address, milon.PubKeySignatureMode{PublicKey: *user3Pk}); err != nil {
		panic("Failed to ClaimFaucet user3:" + err.Error())
	}
	userBalance, err = client.AddressBalance(*user3Address)
	if err != nil {
		panic("Failed to get user3 MIL:" + err.Error())
	}
	fmt.Printf("user3 MIL: %d\n", userBalance)

	// Create on-chain transaction
	submitTransactionResult, err = client.BuildAndSubmitSingleIxSplit(
		"account",
		"Create",
		provider.Args{
			"owner_pk": user3Pk,
		},
		user3Sk,
		*user3Address,
		milon.PubKeySignatureMode{PublicKey: *user3Pk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitTransactionResult.BodyTxHash)
	getTxByHashResult, err = client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getTxByHashResult.BodyTxHistory.Receipt.Error))
	}

	// Display GetAccount
	helper.DisplayGetAccount(client, user3Address.ToBase58())

	//********************************* Create Account 4 FnDsa512 		BuildAndSubmitSingleIxUnifiedDualSign

	user4Sker, user4Pk, _ := crypto.NewFnDsa512SecretKey()
	user4Sk := crypto.AsFnDsa512SecretKey(user4Sker)
	user4Address, _ := crypto.NewAddressFromPublicKey(user4Pk)

	payerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	payerPk := payerSk.Ed25519Public()
	payerAddress, _ := crypto.NewAddressFromPublicKey(payerPk)

	fmt.Printf("user4Pk = %v \n", user4Pk)
	fmt.Printf("user4Address = %v \n\n", user4Address)

	fmt.Printf("payerPk = %v \n", payerPk)
	fmt.Printf("payerAddress = %v \n\n", payerAddress)

	if err = client.ClaimFaucet(payerSk, *payerAddress, milon.PubKeySignatureMode{PublicKey: *payerPk}); err != nil {
		panic("Failed to ClaimFaucet payer:" + err.Error())
	}
	userBalance, err = client.AddressBalance(*payerAddress)
	if err != nil {
		panic("Failed to get payer MIL:" + err.Error())
	}
	fmt.Printf("payer MIL: %d\n", userBalance)

	// Create on-chain transaction
	submitTransactionResult, err = client.BuildAndSubmitSingleIxUnifiedDualSign(
		"account",
		"Create",
		provider.Args{
			"owner_pk": user4Pk,
		},
		payerSk,
		*payerAddress,
		milon.PubKeySignatureMode{PublicKey: *payerPk},
		user4Sk,
		*user4Address,
		milon.PubKeySignatureMode{PublicKey: *user4Pk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitTransactionResult.BodyTxHash)
	getTxByHashResult, err = client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getTxByHashResult.BodyTxHistory.Receipt.Error))
	}

	// Display GetAccount
	helper.DisplayGetAccount(client, user4Address.ToBase58())
}
