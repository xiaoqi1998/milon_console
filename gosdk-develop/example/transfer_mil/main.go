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

	// Sender
	senderSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	senderPk := senderSk.Ed25519Public()
	sender, _ := crypto.NewAddressFromPublicKey(senderPk)

	// Recipient
	recipientSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	recipientPk := recipientSk.Ed25519Public()
	recipient, _ := crypto.NewAddressFromPublicKey(recipientPk)

	fmt.Printf("sender = %v \n", sender)
	fmt.Printf("recipient = %v \n\n", recipient)

	fmt.Printf("\n================ Initial MIL ================\n")

	// Claim faucet MIL for the sender
	if err := client.ClaimFaucet(senderSk, sender, lib.PubKeySignatureMode{PublicKey: *senderPk}); err != nil {
		panic("failed to ClaimFaucet MIL:" + err.Error())
	}
	senderBalance, err := client.BalanceOf(sender)
	if err != nil {
		panic("failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", senderBalance)

	fmt.Printf("\n================ Transfer MIL (no on-chain account) ================\n")

	// 1. Encode instruction
	wire, err := gen.Token.Transfer.Args(sender, api.MILToken, recipient, 300).Encode()
	if err != nil {
		panic("failed to encode Transfer instruction:" + err.Error())
	}

	// 2. Build once, reuse the same builder for simulate & real sign (same Stamp -> same TxHash).
	//    SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire})

	// 3. Simulate on-chain first (no private key needed, dry-run).
	simulateTx, err := builder.AddSimulateIxAndPayerSig(*sender, 0, lib.PubKeySignatureMode{PublicKey: *senderPk}).Build()
	if err != nil {
		panic("failed to simulate transaction:" + err.Error())
	}
	simulateResult, err := client.SimulateTx(simulateTx)
	if err != nil {
		panic("failed to simulate transaction on chain:" + err.Error())
	}
	helper.CheckSimulateSuccess(simulateResult)
	fmt.Printf("Simulated transaction hash: %s, gas charged: %d\n", simulateTx.TxHash(), simulateResult.BodySimulateReceipt.GasCharged)

	// 4. Real sign on the same builder (same TxHash).
	tx, err := builder.ResetSigs().AddIxAndPayerSig(*sender, senderSk, 0, lib.PubKeySignatureMode{PublicKey: *senderPk}).Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}

	// 5. Submit transaction on chain
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// 6. Wait for the transaction to complete
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	fmt.Printf("\n================ Intermediate MIL ================\n")
	senderBalance, err = client.BalanceOf(sender)
	if err != nil {
		panic("failed to get sender MIL:" + err.Error())
	}
	fmt.Printf("sender MIL: %d\n", senderBalance)
	recipientBalance, err := client.BalanceOf(recipient)
	if err != nil {
		panic("failed to get recipient MIL:" + err.Error())
	}
	fmt.Printf("recipient MIL: %d\n", recipientBalance)

	fmt.Printf("\n================ Create on-chain account ================\n")

	// CreateAccount registers the sender on chain, so later transfers can sign with SkipPubKey (the chain resolves the public key from account state).
	if err = client.CreateAccount(senderSk, senderPk); err != nil {
		panic("failed to CreateAccount:" + err.Error())
	}

	// Show the on-chain account and its signers
	helper.DisplayGetAccount(client, sender)
	helper.DisplayAccountGetListSigners(client, sender)

	fmt.Printf("\n================ Transfer MIL (with on-chain account) ================\n")

	// Resolve the sender's signer position from the on-chain signers list;
	// SkipPubKey needs it for the chain to locate the public key at verify time.
	sigBit, err := client.AccountSignerBit(sender)
	if err != nil {
		panic("failed to resolve signer bit:" + err.Error())
	}

	// 1. Encode instruction
	wire, err = gen.Token.Transfer.Args(sender, api.MILToken, recipient, 300).Encode()
	if err != nil {
		panic("failed to encode Transfer instruction:" + err.Error())
	}

	// 2. SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
	//    SkipPubKey: omit the public key (saves ~33 bytes), SigBit points to the sender's position in the on-chain signers list.
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		AddIxAndPayerSig(*sender, senderSk, 0, lib.PubKeySignatureMode{PublicKey: *senderPk, SkipPubKey: true, SigBit: sigBit}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}

	// 3. Submit transaction on chain
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}

	// 4. Wait for the transaction to complete
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	fmt.Printf("\n================ Final MIL ================\n")
	senderBalance, err = client.BalanceOf(sender)
	if err != nil {
		panic("failed to get sender MIL:" + err.Error())
	}
	fmt.Printf("sender MIL: %d\n", senderBalance)
	recipientBalance, err = client.BalanceOf(recipient)
	if err != nil {
		panic("failed to get recipient MIL:" + err.Error())
	}
	fmt.Printf("recipient MIL: %d\n", recipientBalance)
}
