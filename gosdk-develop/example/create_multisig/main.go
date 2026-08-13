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

	// Create 4 signers: signerMultiSig is the multisig wallet creator, signerA/B/C are the participants
	signerMultiSig := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerA := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerB := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerC := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())

	pkMultiSig := signerMultiSig.Ed25519Public()
	pkA := signerA.Ed25519Public()
	pkB := signerB.Ed25519Public()
	pkC := signerC.Ed25519Public()

	accountA, _ := crypto.NewAddressFromPublicKey(pkA)
	accountMultiSig, _ := crypto.NewAddressFromPublicKey(pkMultiSig)

	fmt.Printf("pkA = %v \n", pkA)
	fmt.Printf("pkB = %v \n", pkB)
	fmt.Printf("pkC = %v \n\n", pkC)

	fmt.Printf("accountMultiSig = %v \n\n", accountMultiSig)

	// Claim initial MIL for the multisig wallet (still a regular account, signed solo by the creator)
	if err := client.ClaimFaucet(signerMultiSig, accountMultiSig, lib.PubKeySignatureMode{PublicKey: *pkMultiSig}); err != nil {
		panic("failed to ClaimFaucet MIL:" + err.Error())
	}

	fmt.Printf("\n================ 1.CreateMultisig ================\n")

	// 1. Encode instruction: create multisig account with signers [pkA, pkB, pkC] (positions 0/1/2), weights [1,2,3], threshold 4
	wire, err := gen.Account.CreateMultisig.Args(accountMultiSig, []*crypto.PublicKey{pkA, pkB, pkC}, []uint8{1, 2, 3}, 4).Encode()
	if err != nil {
		panic("failed to encode CreateMultisig instruction:" + err.Error())
	}

	// 2. Submit transaction: unified payer mode, creator signs ix0 + gas (bit63) solo
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountMultiSig).
		AddIxAndPayerSig(*accountMultiSig, signerMultiSig, 0, lib.PubKeySignatureMode{PublicKey: *pkMultiSig}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// 3. Wait for the transaction to complete
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	helper.DisplayGetAccount(client, accountMultiSig)
	helper.DisplayAccountGetListSigners(client, accountMultiSig)

	fmt.Printf("\n================ 2.transfer MIL ================\n")

	// 1. Encode instruction: transfer 1000 MIL from the multisig wallet to accountA
	wire, err = gen.Token.Transfer.Args(accountMultiSig, api.MILToken, accountA, 1000).Encode()
	if err != nil {
		panic("failed to encode Transfer instruction:" + err.Error())
	}

	// 2. Build transaction: unified payer mode (WithPayer), gas paid by the multisig wallet.
	//    Reuse the same builder (fixed Stamp) -> simulate and real sign produce the same TxHash.
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountMultiSig)

	tx, err = builder.Build()
	if err != nil {
		panic("failed to build transaction:" + err.Error())
	}
	txHash := tx.TxHash()
	ixPart := []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}

	// 3. Simulate first: participants generate all-zero placeholder signatures (same length as real ones),
	//    merged into one multisig signature.
	//    Unified payer mode: the multisig signature authorizes ix0 + gas (bit63).
	//    (pkB at position 1, pkC at position 2; weights 2+3 >= threshold 4.)
	simSig, err := lib.NewAccountSignatureBuilder().AuthorizeIxAndPayer(0).
		SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pkB}). // participant signerB (position 1)
		SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pkC}). // participant signerC (position 2)
		Build()
	if err != nil {
		panic("failed to build simulated multisig signature:" + err.Error())
	}

	// 4. Simulate on-chain (dry-run, no private key needed)
	simulateTx, err := builder.AddSignature(*accountMultiSig, *simSig).Build()
	if err != nil {
		panic("failed to build simulated transaction:" + err.Error())
	}
	simulateResult, err := client.SimulateTx(simulateTx)
	if err != nil {
		panic("failed to simulate transaction on chain:" + err.Error())
	}
	helper.CheckSimulateSuccess(simulateResult)
	fmt.Printf("simulated transaction hash: %s, gas charged: %d\n", simulateTx.TxHash(), simulateResult.BodySimulateReceipt.GasCharged)

	// 5. After simulation passes, reset placeholders and sign with the real multisig signature (same builder -> same TxHash)
	multisigSig, err := lib.NewAccountSignatureBuilder().AuthorizeIxAndPayer(0).
		SignMultisigKey(*accountMultiSig, signerB, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pkB}). // participant signerB (position 1)
		SignMultisigKey(*accountMultiSig, signerC, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pkC}). // participant signerC (position 2)
		Build()
	if err != nil {
		panic("failed to build multisig signature:" + err.Error())
	}
	tx, err = builder.ResetSigs().AddSignature(*accountMultiSig, *multisigSig).Build()
	if err != nil {
		panic("failed to build signed transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// 6. Wait for the transaction to complete
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)
}
