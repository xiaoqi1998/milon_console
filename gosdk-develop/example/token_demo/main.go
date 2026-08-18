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

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	token, _ := crypto.NewAddressFromPublicKey(tokenPk)

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	owner, _ := crypto.NewAddressFromPublicKey(ownerPk)

	account1Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	account1Pk := account1Sk.Ed25519Public()
	account1, _ := crypto.NewAddressFromPublicKey(account1Pk)

	account2Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	account2Pk := account2Sk.Ed25519Public()
	account2, _ := crypto.NewAddressFromPublicKey(account2Pk)

	spenderSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	spenderPk := spenderSk.Ed25519Public()
	spender, _ := crypto.NewAddressFromPublicKey(spenderPk)

	fmt.Printf("token = %v \n", token)
	fmt.Printf("owner = %v \n", owner)
	fmt.Printf("spender = %v \n", spender)
	fmt.Printf("account1 = %v \n", account1)
	fmt.Printf("account2 = %v \n", account2)
	fmt.Printf("spender = %v \n\n", spender)

	fmt.Printf("\n================ 1.Initial MIL ================\n")
	if err := client.ClaimFaucet(tokenSk, token, lib.PubKeySignatureMode{PublicKey: *tokenPk}); err != nil {
		panic("failed to ClaimFaucet MIL:" + err.Error())
	}
	tokenBalance, err := client.BalanceOf(token)
	if err != nil {
		panic("failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", tokenBalance)

	if err = client.ClaimFaucet(ownerSk, owner, lib.PubKeySignatureMode{PublicKey: *ownerPk}); err != nil {
		panic("failed to ClaimFaucet owner:" + err.Error())
	}
	ownerBalance, err := client.BalanceOf(owner)
	if err != nil {
		panic("failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	if err = client.ClaimFaucet(account1Sk, account1, lib.PubKeySignatureMode{PublicKey: *account1Pk}); err != nil {
		panic("failed to ClaimFaucet account1:" + err.Error())
	}
	account1Balance, err := client.BalanceOf(account1)
	if err != nil {
		panic("failed to get account1 MIL:" + err.Error())
	}
	fmt.Printf("account1 MIL: %d\n", account1Balance)

	fmt.Printf("\n================ 2.Create(token sign) ================\n")

	wire, err := gen.Token.Create.Args(token, owner, gen.TokenMetadata{
		Name:     "Example Token",
		Symbol:   "Token",
		Decimals: 6,
		Icon:     "https://milon.test/token.png",
	}).Encode()
	if err != nil {
		panic("failed to encode Create instruction:" + err.Error())
	}

	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(token).
		AddIxesSig(*token, tokenSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *tokenPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	fmt.Printf("\n================ 3.Mint(owner mint 1000 to account1 + owner sign) ================\n")

	wire, err = gen.Token.Mint.Args(token, account1, 1000).Encode()
	if err != nil {
		panic("failed to encode Mint instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(owner).
		AddIxesSig(*owner, ownerSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *ownerPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	wire, err = gen.Token.BalanceOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err := client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wireViewDecode, err := gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 balance query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 balance: %+v \n", wireViewDecode)
	}

	fmt.Printf("\n================ 4.Transfer(account1 transfer 100 to account2  + account1 sign) ================\n")

	wire, err = gen.Token.Transfer.Args(account1, token, account2, 100).Encode()
	if err != nil {
		panic("failed to encode Transfer instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(account1).
		AddIxesSig(*account1, account1Sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *account1Pk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	wire, err = gen.Token.BalanceOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 balance query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 balance: %+v \n", wireViewDecode)
	}

	fmt.Printf("\n================ 5.Burn(account1 burn 200 + account1 sign) ================\n")

	wire, err = gen.Token.Burn.Args(account1, token, 200).Encode()
	if err != nil {
		panic("failed to encode Burn instruction:" + err.Error())
	}

	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(account1).
		AddIxesSig(*account1, account1Sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *account1Pk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	wire, err = gen.Token.BalanceOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 balance query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 balance: %+v \n", wireViewDecode)
	}

	fmt.Printf("\n================ 6.Freeze(account1 freeze 300 + owner sign) ================\n")

	wire, err = gen.Token.Freeze.Args(token, account1, 300).Encode()
	if err != nil {
		panic("failed to encode Freeze instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(owner).
		AddIxesSig(*owner, ownerSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *ownerPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	wire, err = gen.Token.BalanceOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 balance query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 balance: %+v \n", wireViewDecode)
	}

	wire, err = gen.Token.FrozenOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode FrozenOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view FrozenOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.FrozenOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 frozen query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 frozen: %+v \n", wireViewDecode)
	}

	fmt.Printf("\n================ 7.Unfreeze(account1 unfreeze 300 + owner sign) ================\n")

	wire, err = gen.Token.Unfreeze.Args(token, account1, 300).Encode()
	if err != nil {
		panic("failed to encode Unfreeze instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(owner).
		AddIxesSig(*owner, ownerSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *ownerPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	wire, err = gen.Token.BalanceOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 balance query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 balance: %+v \n", wireViewDecode)
	}

	wire, err = gen.Token.FrozenOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode FrozenOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view FrozenOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.FrozenOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 frozen query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 frozen: %+v \n", wireViewDecode)
	}

	fmt.Printf("\n================ 8.Approve(account1 Approve 400 to spender + account1 sign) ================\n")

	wire, err = gen.Token.Approve.Args(account1, token, spender, 400).Encode()
	if err != nil {
		panic("failed to encode TransferFrom instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(account1).
		AddIxesSig(*account1, account1Sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *account1Pk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	wire, err = gen.Token.BalanceOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 balance query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 balance: %+v \n", wireViewDecode)
	}

	wire, err = gen.Token.ApprovalOf.Args(token, account1, spender).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.ApprovalOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token spender approval query error:%v \n", err.Error())
	} else {
		fmt.Printf("token spender approval: %+v \n", wireViewDecode)
	}

	fmt.Printf("\n================ 9.TransferFrom(account1 TransferFrom 200 to spender + spender sign and account1 sign+sponsored) ================\n")

	wire, err = gen.Token.TransferFrom.Args(spender, token, account1, 200).Encode()
	if err != nil {
		panic("failed to encode TransferFrom instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(account1).
		AddIxesSig(*spender, spenderSk, []uint8{0}, false, lib.PubKeySignatureMode{PublicKey: *spenderPk}).
		AddPayerSig(*account1, account1Sk, lib.PubKeySignatureMode{PublicKey: *account1Pk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	wire, err = gen.Token.BalanceOf.Args(token, account1).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token account1 balance query error:%v \n", err.Error())
	} else {
		fmt.Printf("token account1 balance: %+v \n", wireViewDecode)
	}

	wire, err = gen.Token.ApprovalOf.Args(token, account1, spender).Encode()
	if err != nil {
		panic("failed to encode BalanceOf:" + err.Error())
	}
	viewTxResult, err = client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	wireViewDecode, err = gen.Token.ApprovalOf.DecodeView(viewTxResult.HTTPResponseBody)
	if err != nil {
		fmt.Printf("token spender approval query error:%v \n", err.Error())
	} else {
		fmt.Printf("token spender approval: %+v \n", wireViewDecode)
	}
}
