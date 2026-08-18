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

	signerA := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerB := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerC := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerD := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())

	pkA := signerA.Ed25519Public()
	pkB := signerB.Ed25519Public()
	pkC := signerC.Ed25519Public()
	pkD := signerD.Ed25519Public()

	accountA, _ := crypto.NewAddressFromPublicKey(pkA)
	accountB, _ := crypto.NewAddressFromPublicKey(pkB)
	accountC, _ := crypto.NewAddressFromPublicKey(pkC)
	accountD, _ := crypto.NewAddressFromPublicKey(pkD)

	demoSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	demoPk := demoSk.Ed25519Public()
	demoPool, _ := crypto.NewAddressFromPublicKey(demoPk)

	demoRecipientSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	demoRecipientPk := demoRecipientSk.Ed25519Public()
	demoRecipient, _ := crypto.NewAddressFromPublicKey(demoRecipientPk)

	modeA := lib.PubKeySignatureMode{PublicKey: *pkA}
	modeB := lib.PubKeySignatureMode{PublicKey: *pkB}
	modeC := lib.PubKeySignatureMode{PublicKey: *pkC}
	modeD := lib.PubKeySignatureMode{PublicKey: *pkD}
	modeDemo := lib.PubKeySignatureMode{PublicKey: *demoPk}

	fmt.Printf("accountA = %v \n", accountA)
	fmt.Printf("accountB = %v \n", accountB)
	fmt.Printf("accountC = %v \n", accountC)
	fmt.Printf("accountD = %v \n", accountD)
	fmt.Printf("demoPool = %v \n", demoPool)
	fmt.Printf("demoRecipient = %v \n\n", demoRecipient)

	// 1. Encode 7 instructions (ix0 - ix6)
	var transferAmount uint64 = 1

	// ix0: ClaimFaucet(accountA) - claim initial MIL for accountA
	wire0, err := gen.Token.ClaimFaucet.Args(accountA).Encode()
	if err != nil {
		panic("failed to encode ClaimFaucet instruction:" + err.Error())
	}

	// ix1: Transfer(accountA -> accountB, 1)
	wire1, err := gen.Token.Transfer.Args(accountA, api.MILToken, accountB, transferAmount).Encode()
	if err != nil {
		panic("failed to encode Transfer[1] instruction:" + err.Error())
	}

	// ix2: Transfer(accountB -> accountC, 1)
	wire2, err := gen.Token.Transfer.Args(accountB, api.MILToken, accountC, transferAmount).Encode()
	if err != nil {
		panic("failed to encode Transfer[2] instruction:" + err.Error())
	}

	// ix3: InitPool(demoPool, "simulate batch credit") - initialize the credit pool
	wire3, err := gen.Demo.InitPool.Args(demoPool, "simulate batch credit").Encode()
	if err != nil {
		panic("failed to encode InitPool instruction:" + err.Error())
	}

	// ix4: BatchCredit(demoPool, [demoRecipient], 42) - batch credit
	wire4, err := gen.Demo.BatchCredit.Args(demoPool, []*crypto.Address{demoRecipient}, 42).Encode()
	if err != nil {
		panic("failed to encode BatchCredit instruction:" + err.Error())
	}

	// ix5: Transfer(accountC -> accountD, 1)
	wire5, err := gen.Token.Transfer.Args(accountC, api.MILToken, accountD, transferAmount).Encode()
	if err != nil {
		panic("failed to encode Transfer[5] instruction:" + err.Error())
	}

	// ix6: Transfer(accountD -> accountA, 1)
	wire6, err := gen.Token.Transfer.Args(accountD, api.MILToken, accountA, transferAmount).Encode()
	if err != nil {
		panic("failed to encode Transfer[6] instruction:" + err.Error())
	}

	// ==================== Simple mode: direct chained signing (no ApplySlots) ====================

	// 3.1 Build once, reuse the same builder for simulate & real sign (same Stamp -> same TxHash)
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire0, wire1, wire2, wire3, wire4, wire5, wire6}).WithPayer(accountA)

	// 3.2 Simulate on-chain first (no private key needed, dry-run)
	//    UnifiedPayerSeparateIx mode: payer signs gas (bit63) only, ix signed by a separate executor account.
	simulateTx, err := builder.
		AddSimulateIxesSig(*accountA, []uint8{0, 1}, true, modeA).     // accountA: ix0 + ix1 + gas (bit63)
		AddSimulateIxesSig(*accountB, []uint8{2}, false, modeB).       // accountB: ix2 only
		AddSimulateIxesSig(*demoPool, []uint8{3, 4}, false, modeDemo). // demoPool: ix3 + ix4 only
		AddSimulateIxesSig(*accountC, []uint8{5}, false, modeC).       // accountC: ix5 only
		AddSimulateIxesSig(*accountD, []uint8{6}, false, modeD).       // accountD: ix6 only
		Build()
	if err != nil {
		panic("failed to simulate transaction:" + err.Error())
	}
	simulateResult, err := client.SimulateTx(simulateTx)
	if err != nil {
		panic("failed to simulate transaction on chain:" + err.Error())
	}
	helper.CheckSimulateSuccess(simulateResult)
	fmt.Printf("simple mode - Simulated transaction hash: %s, gas charged: %d\n", simulateTx.TxHash(), simulateResult.BodySimulateReceipt.GasCharged)

	// 3.3 Real sign on the same builder (same TxHash)
	tx, err := builder.ResetSigs().
		AddIxesSig(*accountA, signerA, []uint8{0, 1}, true, modeA).    // accountA: ix0 + ix1 + gas (bit63)
		AddIxesSig(*accountB, signerB, []uint8{2}, false, modeB).      // accountB: ix2 only
		AddIxesSig(*demoPool, demoSk, []uint8{3, 4}, false, modeDemo). // demoPool: ix3 + ix4 only
		AddIxesSig(*accountC, signerC, []uint8{5}, false, modeC).      // accountC: ix5 only
		AddIxesSig(*accountD, signerD, []uint8{6}, false, modeD).      // accountD: ix6 only
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}

	// 3.4 Submit transaction on chain
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// 3.5 Wait for the transaction to complete
	fmt.Printf("simple mode - wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("simple mode - submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// ==================== Complex mode: SigningSlot + ApplySlots ====================

	// 4.1 Define signing slots once (shared by simulate & real sign)
	slots := []lib.SigningSlot{
		{Address: *accountA, InstructionIndices: []uint8{0}, IncludePayer: true, Mode: modeA},  // accountA: ix0 + gas (bit63)
		{Address: *accountB, InstructionIndices: []uint8{1}, IncludePayer: false, Mode: modeB}, // accountB: ix1 only
		{Address: *accountC, InstructionIndices: []uint8{2}, IncludePayer: false, Mode: modeC}, // accountC: ix2 only
		{Address: *accountD, InstructionIndices: []uint8{3}, IncludePayer: false, Mode: modeD}, // accountD: ix3 only
	}

	// 4.2 Build transaction once with ApplySlots, reuse the same builder for simulate & real sign (same Stamp -> same TxHash)
	builderComplex := lib.NewTransactionBuilder([]api.PackedInstruction{wire1, wire2, wire5, wire6}).WithPayer(accountA).ApplySlots(slots)

	// 4.3 Simulate on-chain first (no private key needed, dry-run)
	simulateTxComplex, err := builderComplex.SimulateSlots().Build()
	if err != nil {
		panic("failed to simulate transaction:" + err.Error())
	}
	simulateResultComplex, err := client.SimulateTx(simulateTxComplex)
	if err != nil {
		panic("failed to simulate transaction on chain:" + err.Error())
	}
	helper.CheckSimulateSuccess(simulateResultComplex)
	fmt.Printf("complex mode - Simulated transaction hash: %s, gas charged: %d\n", simulateTxComplex.TxHash(), simulateResultComplex.BodySimulateReceipt.GasCharged)

	// 4.4 Real sign: private keys are matched to slots by address
	txComplex, err := builderComplex.ResetSigs().
		SignWith(
			lib.Signer{SecretKey: signerA, PublicKey: *pkA},
			lib.Signer{SecretKey: signerB, PublicKey: *pkB},
			lib.Signer{SecretKey: signerC, PublicKey: *pkC},
			lib.Signer{SecretKey: signerD, PublicKey: *pkD},
		).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}

	// 4.5 Submit transaction on chain
	err = client.SubmitTx(txComplex)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// 4.6 Wait for the transaction to complete
	fmt.Printf("complex mode - wait for the transaction %s to complete...\n", txComplex.TxHash())
	getTxByHashResultComplex, err := client.WaitForTransaction(txComplex.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResultComplex.BodyTxHistory)
	fmt.Printf("complex mode - submit transaction hash: %s, gas charged: %d\n", txComplex.TxHash(), getTxByHashResultComplex.BodyTxHistory.Receipt.GasCharged)

	// Display TxHistory
	helper.DisplayTxHistory(client, getTxByHashResultComplex.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResultComplex.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, txComplex.TxHash(), nil)
	}
}
