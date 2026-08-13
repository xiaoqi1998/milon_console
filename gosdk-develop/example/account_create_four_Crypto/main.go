package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/helper"
	"github.com/milon-labs/milon-go-sdk/lib"
)

func main() {
	example(milon.DevNet)
}

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	//********************************* Create Account 1 Secp256k1

	account1Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	account1Pk, _ := account1Sk.Secp256k1Public()
	account1, _ := crypto.NewAddressFromPublicKey(account1Pk)

	fmt.Printf("account1Pk = %v \n", account1Pk)
	fmt.Printf("account1 = %v \n\n", account1)

	if err := client.ClaimFaucet(account1Sk, account1, lib.PubKeySignatureMode{PublicKey: *account1Pk}); err != nil {
		panic("failed to ClaimFaucet account1:" + err.Error())
	}
	accountBalance, err := client.BalanceOf(account1)
	if err != nil {
		panic("failed to get account1 MIL:" + err.Error())
	}
	fmt.Printf("account1 MIL: %d\n", accountBalance)

	wire, err := gen.Account.Create.Args(account1Pk).Encode()
	if err != nil {
		panic("failed to encode Create instruction:" + err.Error())
	}

	// UnifiedPayerGasOnly mode: account1 signs gas (bit63) only, ix needs no signature (pure sponsorship).
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(account1).
		AddPayerSig(*account1, account1Sk, lib.PubKeySignatureMode{PublicKey: *account1Pk}).
		Build()

	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// Wait for the transaction to complete
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// Show the on-chain account and its signers (the source for AccountSignerBit below).
	helper.DisplayGetAccount(client, account1)
	helper.DisplayAccountGetListSigners(client, account1)

	//********************************* Create Account 2 Ed25519

	account2Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	account2Pk := account2Sk.Ed25519Public()
	account2, _ := crypto.NewAddressFromPublicKey(account2Pk)

	fmt.Printf("account2Pk = %v \n", account2Pk)
	fmt.Printf("account2 = %v \n\n", account2)

	if err = client.ClaimFaucet(account2Sk, account2, lib.PubKeySignatureMode{PublicKey: *account2Pk}); err != nil {
		panic("failed to ClaimFaucet account2:" + err.Error())
	}
	accountBalance, err = client.BalanceOf(account2)
	if err != nil {
		panic("failed to get account2 MIL:" + err.Error())
	}
	fmt.Printf("account2 MIL: %d\n", accountBalance)

	wire, err = gen.Account.Create.Args(account2Pk).Encode()
	if err != nil {
		panic("failed to encode Create instruction:" + err.Error())
	}

	// UnifiedPayerSignAll mode: account2 signs the ix bit(s) and gas bit (bit63) in a single signature.
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(account2).
		AddIxAndPayerSig(*account2, account2Sk, 0, lib.PubKeySignatureMode{PublicKey: *account2Pk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// Wait for the transaction to complete
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// Show the on-chain account and its signers (the source for AccountSignerBit below).
	helper.DisplayGetAccount(client, account2)
	helper.DisplayAccountGetListSigners(client, account2)

	//********************************* Create Account 3 BLS12381

	account3Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	account3Pk := account3Sk.BLS12381Public()
	account3, _ := crypto.NewAddressFromPublicKey(account3Pk)

	payerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	payerPk := payerSk.Ed25519Public()
	payer, _ := crypto.NewAddressFromPublicKey(payerPk)

	fmt.Printf("account3Pk = %v \n", account3Pk)
	fmt.Printf("account3 = %v \n", account3)
	fmt.Printf("payerPk = %v \n", payerPk)
	fmt.Printf("payer = %v \n\n", payer)

	if err = client.ClaimFaucet(payerSk, payer, lib.PubKeySignatureMode{PublicKey: *payerPk}); err != nil {
		panic("failed to ClaimFaucet payer:" + err.Error())
	}
	accountBalance, err = client.BalanceOf(payer)
	if err != nil {
		panic("failed to get payer MIL:" + err.Error())
	}
	fmt.Printf("payer MIL: %d\n", accountBalance)

	wire, err = gen.Account.Create.Args(account3Pk).Encode()
	if err != nil {
		panic("failed to encode Create instruction:" + err.Error())
	}

	// UnifiedPayerSeparateIx mode: payer signs gas (bit63) only, ix signed by a separate executor account.
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(payer).
		AddPayerSig(*payer, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPk}).
		AddIxesSig(*account3, account3Sk, []uint8{0}, false, lib.PubKeySignatureMode{PublicKey: *account3Pk}).
		Build()

	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// Wait for the transaction to complete
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// Show the on-chain account and its signers (the source for AccountSignerBit below).
	helper.DisplayGetAccount(client, account3.ToHex())
	helper.DisplayAccountGetListSigners(client, account3)

	//********************************* Create Account 4 FnDsa512

	account4Sker, account4Pk, _ := crypto.NewFnDsa512SecretKey()
	account4Sk := crypto.AsFnDsa512SecretKey(account4Sker)
	account4, _ := crypto.NewAddressFromPublicKey(account4Pk)

	fmt.Printf("account4Pk = %v \n", account4Pk)
	fmt.Printf("account4 = %v \n\n", account4)

	if err = client.ClaimFaucet(account4Sk, account4, lib.PubKeySignatureMode{PublicKey: *account4Pk}); err != nil {
		panic("failed to ClaimFaucet account4:" + err.Error())
	}
	accountBalance, err = client.BalanceOf(account4)
	if err != nil {
		panic("failed to get account4 MIL:" + err.Error())
	}
	fmt.Printf("account4 MIL: %d\n", accountBalance)

	wire, err = gen.Account.Create.Args(account4Pk).Encode()
	if err != nil {
		panic("failed to encode Create instruction:" + err.Error())
	}

	// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		AddIxesSig(*account4, account4Sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *account4Pk}).
		Build()

	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// Wait for the transaction to complete
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// Show the on-chain account and its signers (the source for AccountSignerBit below).
	helper.DisplayGetAccount(client, account4.ToBase58())
	helper.DisplayAccountGetListSigners(client, account4)
}
