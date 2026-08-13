package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/helper"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/types"
)

func main() {
	example(milon.DevNet)
}

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// Create 4 signers: signerA/B/C/D as multisig participants
	signerA := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerB := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerC := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerD := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())

	pkA := signerA.Ed25519Public()
	pkB := signerB.Ed25519Public()
	pkC := signerC.Ed25519Public()
	pkD := signerD.Ed25519Public()

	fmt.Printf("pkA = %v \n", pkA)
	fmt.Printf("pkB = %v \n", pkB)
	fmt.Printf("pkC = %v \n", pkC)
	fmt.Printf("pkD = %v \n\n", pkD)

	accountA, _ := crypto.NewAddressFromPublicKey(pkA)

	if err := client.ClaimFaucet(signerA, accountA, lib.PubKeySignatureMode{PublicKey: *pkA}); err != nil {
		panic("failed to ClaimFaucet MIL:" + err.Error())
	}

	fmt.Printf("\n================ 1.createAccount ================\n")

	createAccount(client, pkA, signerA)
	helper.DisplayAccountGetListSigners(client, accountA)

	fmt.Printf("\n================ 2.addSigner ================\n")
	addSigner(client, pkA, signerA, pkB)
	helper.DisplayAccountGetListSigners(client, accountA)

	fmt.Printf("\n================ 3.addSigners ================\n")
	addSigners(client, pkA, signerA, pkC, pkD)
	helper.DisplayAccountGetListSigners(client, accountA)

	fmt.Printf("\n================ 4.setThreshold ================\n")
	setThreshold(client, pkA, signerA, pkC, signerC)
	helper.DisplayAccountGetListSigners(client, accountA)

	fmt.Printf("\n================ 5.removeSigner ================\n")
	removeSigner(client, pkA, pkC, signerC, pkD, signerD)
	helper.DisplayAccountGetListSigners(client, accountA)

	fmt.Printf("\n================ 6.setSignerWeight ================\n")
	setSignerWeight(client, pkA, signerA, pkC, signerC, pkD, signerD)
	helper.DisplayAccountGetListSigners(client, accountA)
}

func createAccount(client *milon.Client, pkA *crypto.PublicKey, signerA crypto.SecretKeyer) {
	accountA, _ := crypto.NewAddressFromPublicKey(pkA)

	wire, err := gen.Account.Create.Args(pkA).Encode()
	if err != nil {
		panic("failed to encode Create instruction:" + err.Error())
	}

	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountA).
		AddPayerSig(*accountA, signerA, lib.PubKeySignatureMode{PublicKey: *pkA}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}

	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)
}

func addSigner(client *milon.Client, pkA *crypto.PublicKey, signerA crypto.SecretKeyer, pkB *crypto.PublicKey) {
	accountA, _ := crypto.NewAddressFromPublicKey(pkA)

	wire, err := gen.Account.AddSigner.Args(accountA, pkB, 1).Encode()
	if err != nil {
		panic("failed to encode AddSigner instruction:" + err.Error())
	}

	// Build transaction: UnifiedPayer mode, gas paid by the multisig wallet.
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountA)

	tx, err := builder.Build()
	if err != nil {
		panic("failed to build transaction:" + err.Error())
	}
	txHash := tx.TxHash()
	ixPart := []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}

	multisigSig, err := lib.NewAccountSignatureBuilder().AuthorizeIxAndPayer(0).
		Sign(*accountA, signerA, txHash, ixPart, lib.PubKeySignatureMode{PublicKey: *pkA}).
		Build()
	if err != nil {
		panic("failed to build multisig signature:" + err.Error())
	}
	tx, err = builder.AddSignature(*accountA, *multisigSig).Build()
	if err != nil {
		panic("failed to build signed transaction:" + err.Error())
	}

	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)
}

func addSigners(client *milon.Client, pkA *crypto.PublicKey, signerA crypto.SecretKeyer, pkC *crypto.PublicKey, pkD *crypto.PublicKey) {
	accountA, _ := crypto.NewAddressFromPublicKey(pkA)

	wire, err := gen.Account.AddSigners.Args(accountA, []*crypto.PublicKey{pkC, pkD}, []byte{2, 3}, 2).Encode()
	if err != nil {
		panic("failed to encode AddSigners instruction:" + err.Error())
	}

	// Build transaction: UnifiedPayer mode, gas paid by the multisig wallet.
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountA)

	tx, err := builder.Build()
	if err != nil {
		panic("failed to build transaction:" + err.Error())
	}
	txHash := tx.TxHash()
	ixPart := []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}

	multisigSig, err := lib.NewAccountSignatureBuilder().AuthorizeIxAndPayer(0).
		Sign(*accountA, signerA, txHash, ixPart, lib.PubKeySignatureMode{PublicKey: *pkA, SkipPubKey: true, SigBit: types.Bitmap64(1)}).
		Build()
	if err != nil {
		panic("failed to build multisig signature:" + err.Error())
	}
	tx, err = builder.AddSignature(*accountA, *multisigSig).Build()
	if err != nil {
		panic("failed to build signed transaction:" + err.Error())
	}

	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)
}

func setThreshold(client *milon.Client, pkA *crypto.PublicKey, signerA crypto.SecretKeyer, pkC *crypto.PublicKey, signerC crypto.SecretKeyer) {
	accountA, _ := crypto.NewAddressFromPublicKey(pkA)

	wire, err := gen.Account.SetThreshold.Args(accountA, 5).Encode()
	if err != nil {
		panic("failed to encode SetThreshold instruction:" + err.Error())
	}

	// Build transaction: UnifiedPayer mode, gas paid by the multisig wallet.
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountA)

	tx, err := builder.Build()
	if err != nil {
		panic("failed to build transaction:" + err.Error())
	}
	txHash := tx.TxHash()
	ixPart := []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}

	multisigSig, err := lib.NewAccountSignatureBuilder().AuthorizeIxAndPayer(0).
		SignMultisigKey(*accountA, signerA, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 0, PublicKey: *pkA}).
		SignMultisigKey(*accountA, signerC, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pkC}).
		Build()
	if err != nil {
		panic("failed to build multisig signature:" + err.Error())
	}
	tx, err = builder.AddSignature(*accountA, *multisigSig).Build()
	if err != nil {
		panic("failed to build signed transaction:" + err.Error())
	}

	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)
}

func removeSigner(client *milon.Client, pkA *crypto.PublicKey, pkC *crypto.PublicKey, signerC crypto.SecretKeyer, pkD *crypto.PublicKey, signerD crypto.SecretKeyer) {
	accountA, _ := crypto.NewAddressFromPublicKey(pkA)

	wire, err := gen.Account.RemoveSigner.Args(accountA, 1, 6).Encode()
	if err != nil {
		panic("failed to encode RemoveSigner instruction:" + err.Error())
	}

	// Build transaction: UnifiedPayer mode, gas paid by the multisig wallet.
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountA)

	tx, err := builder.Build()
	if err != nil {
		panic("failed to build transaction:" + err.Error())
	}
	txHash := tx.TxHash()
	ixPart := []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}

	multisigSig, err := lib.NewAccountSignatureBuilder().AuthorizeIxAndPayer(0).
		SignMultisigKey(*accountA, signerC, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pkC}).
		SignMultisigKey(*accountA, signerD, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 3, PublicKey: *pkD}).
		Build()
	if err != nil {
		panic("failed to build multisig signature:" + err.Error())
	}
	tx, err = builder.AddSignature(*accountA, *multisigSig).Build()
	if err != nil {
		panic("failed to build signed transaction:" + err.Error())
	}

	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)
}

func setSignerWeight(client *milon.Client, pkA *crypto.PublicKey, signerA crypto.SecretKeyer, pkC *crypto.PublicKey, signerC crypto.SecretKeyer, pkD *crypto.PublicKey, signerD crypto.SecretKeyer) {
	accountA, _ := crypto.NewAddressFromPublicKey(pkA)

	wire, err := gen.Account.SetSignerWeight.Args(accountA, 0, 5).Encode()
	if err != nil {
		panic("failed to encode SetSignerWeight instruction:" + err.Error())
	}

	// Build transaction: UnifiedPayer mode, gas paid by the multisig wallet.
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountA)

	tx, err := builder.Build()
	if err != nil {
		panic("failed to build transaction:" + err.Error())
	}
	txHash := tx.TxHash()
	ixPart := []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}

	multisigSig, err := lib.NewAccountSignatureBuilder().AuthorizeIxAndPayer(0).
		SignMultisigKey(*accountA, signerA, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 0, PublicKey: *pkA}).
		SignMultisigKey(*accountA, signerC, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pkC}).
		SignMultisigKey(*accountA, signerD, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 3, PublicKey: *pkD}).
		Build()
	if err != nil {
		panic("failed to build multisig signature:" + err.Error())
	}
	tx, err = builder.AddSignature(*accountA, *multisigSig).Build()
	if err != nil {
		panic("failed to build signed transaction:" + err.Error())
	}

	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)
}
