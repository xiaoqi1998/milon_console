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

	pd, err := client.GetPdByIDLAppName("account")
	if err != nil {
		panic("Failed to get IDL provider for 'account':" + err.Error())
	}

	//********************************* Create Account 1 Secp256k1

	user1Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user1Pk, _ := user1Sk.Secp256k1Public()
	user1Address, _ := crypto.NewAddressFromPublicKey(user1Pk)

	fmt.Printf("user1Pk = %v \n", user1Pk)
	fmt.Printf("user1Address = %v \n\n", user1Address)

	if err = client.ClaimFaucet(user1Sk, *user1Address, lib.PubKeySignatureMode{PublicKey: *user1Pk}); err != nil {
		panic("Failed to ClaimFaucet user1:" + err.Error())
	}
	userBalance, err := client.BalanceOf(*user1Address)
	if err != nil {
		panic("Failed to get user1 MIL:" + err.Error())
	}
	fmt.Printf("user1 MIL: %d\n", userBalance)

	wire, err := pd.Encode("Create", provider.Args{
		"owner_pk": user1Pk,
	})
	if err != nil {
		panic("Failed to encode Create instruction:" + err.Error())
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(user1Address).
		AddPayerSig(*user1Address, user1Sk, lib.PubKeySignatureMode{PublicKey: *user1Pk}).
		Build()

	if err != nil {
		panic("Failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("Failed to submit transaction:" + err.Error())
	}

	// Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash(), 1)
	if err != nil {
		panic("Failed to wait for transaction:" + err.Error())
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Transaction failed on chain: error code = %d", *getTxByHashResult.BodyTxHistory.Receipt.Error))
	}
	fmt.Printf("Submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// Display GetAccount
	helper.DisplayGetAccount(client, user1Address.ToBase58())

	//********************************* Create Account 2 Ed25519 	BuildAndSubmitSingleIxUnifiedPayerSignAll

	user2Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user2Pk := user2Sk.Ed25519Public()
	user2Address, _ := crypto.NewAddressFromPublicKey(user2Pk)

	fmt.Printf("user2Pk = %v \n", user2Pk)
	fmt.Printf("user2Address = %v \n\n", user2Address)

	if err = client.ClaimFaucet(user2Sk, *user2Address, lib.PubKeySignatureMode{PublicKey: *user2Pk}); err != nil {
		panic("Failed to ClaimFaucet user2:" + err.Error())
	}
	userBalance, err = client.BalanceOf(*user2Address)
	if err != nil {
		panic("Failed to get user2 MIL:" + err.Error())
	}
	fmt.Printf("user2 MIL: %d\n", userBalance)

	wire, err = pd.Encode("Create", provider.Args{
		"owner_pk": user2Pk,
	})
	if err != nil {
		panic("Failed to encode Create instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(user2Address).
		AddIxAndPayerSig(*user2Address, user2Sk, 0, lib.PubKeySignatureMode{PublicKey: *user2Pk}).
		Build()

	if err != nil {
		panic("Failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("Failed to submit transaction:" + err.Error())
	}

	// Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash(), 1)
	if err != nil {
		panic("Failed to wait for transaction:" + err.Error())
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Transaction failed on chain: error code = %d", *getTxByHashResult.BodyTxHistory.Receipt.Error))
	}
	fmt.Printf("Submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// Display GetAccount
	helper.DisplayGetAccount(client, user2Address.ToBase58())

	//********************************* Create Account 3 BLS12381

	user3Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user3Pk := user3Sk.BLS12381Public()
	user3Address, _ := crypto.NewAddressFromPublicKey(user3Pk)

	fmt.Printf("user3Pk = %v \n", user3Pk)
	fmt.Printf("user3Address = %v \n\n", user3Address)

	if err = client.ClaimFaucet(user3Sk, *user3Address, lib.PubKeySignatureMode{PublicKey: *user3Pk}); err != nil {
		panic("Failed to ClaimFaucet user3:" + err.Error())
	}
	userBalance, err = client.BalanceOf(*user3Address)
	if err != nil {
		panic("Failed to get user3 MIL:" + err.Error())
	}
	fmt.Printf("user3 MIL: %d\n", userBalance)

	wire, err = pd.Encode("Create", provider.Args{
		"owner_pk": user3Pk,
	})
	if err != nil {
		panic("Failed to encode Create instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		AddIxesSig(*user2Address, user2Sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *user2Pk}).
		Build()

	if err != nil {
		panic("Failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("Failed to submit transaction:" + err.Error())
	}

	// Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash(), 1)
	if err != nil {
		panic("Failed to wait for transaction:" + err.Error())
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Transaction failed on chain: error code = %d", *getTxByHashResult.BodyTxHistory.Receipt.Error))
	}
	fmt.Printf("Submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

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
	fmt.Printf("user4Address = %v \n", user4Address)
	fmt.Printf("payerPk = %v \n", payerPk)
	fmt.Printf("payerAddress = %v \n\n", payerAddress)

	if err = client.ClaimFaucet(payerSk, *payerAddress, lib.PubKeySignatureMode{PublicKey: *payerPk}); err != nil {
		panic("Failed to ClaimFaucet payer:" + err.Error())
	}
	userBalance, err = client.BalanceOf(*payerAddress)
	if err != nil {
		panic("Failed to get payer MIL:" + err.Error())
	}
	fmt.Printf("payer MIL: %d\n", userBalance)

	wire, err = pd.Encode("Create", provider.Args{
		"owner_pk": user4Pk,
	})
	if err != nil {
		panic("Failed to encode Create instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(payerAddress).
		AddPayerSig(*payerAddress, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPk}).
		AddIxesSig(*user4Address, user4Sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *user4Pk}).
		Build()

	if err != nil {
		panic("Failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("Failed to submit transaction:" + err.Error())
	}

	// Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash(), 1)
	if err != nil {
		panic("Failed to wait for transaction:" + err.Error())
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Transaction failed on chain: error code = %d", *getTxByHashResult.BodyTxHistory.Receipt.Error))
	}
	fmt.Printf("Submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// Display GetAccount
	helper.DisplayGetAccount(client, user4Address.ToBase58())
}
