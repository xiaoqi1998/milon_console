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

	// --- 获取 IDL 提供者 ---
	tokenPd, err := client.GetPdByIDLAppName("token")
	if err != nil {
		panic("获取 token IDL 失败: %w" + err.Error())
	}
	demoPd, err := client.GetPdByIDLAppName("demo")
	if err != nil {
		panic("获取 demo IDL 失败: %w" + err.Error())
	}

	fmt.Printf("accountA = %v \n", accountA)
	fmt.Printf("accountB = %v \n", accountB)
	fmt.Printf("accountC = %v \n", accountC)
	fmt.Printf("accountD = %v \n", accountD)
	fmt.Printf("demoPool = %v \n", demoPool)
	fmt.Printf("demoRecipient = %v \n\n", demoRecipient)

	// --- 构建 7 条指令 ---

	transferAmount := 1

	// 指令 0: ClaimFaucet(account_a) — 领取代币
	wire0, err := tokenPd.Encode("ClaimFaucet", provider.Args{
		"claimer": accountA,
	})
	if err != nil {
		panic("编码 ClaimFaucet 失败: %w" + err.Error())
	}

	// 指令 1: Transfer(account_a → account_b, 1)
	wire1, err := tokenPd.Encode("Transfer", provider.Args{
		"from":   accountA,
		"token":  api.MIL,
		"to":     accountB,
		"amount": transferAmount,
	})
	if err != nil {
		panic("编码 Transfer[1] 失败: %w" + err.Error())
	}

	// 指令 2: Transfer(account_b → account_c, 1)
	wire2, err := tokenPd.Encode("Transfer", provider.Args{
		"from":   accountB,
		"token":  api.MIL,
		"to":     accountC,
		"amount": transferAmount,
	})
	if err != nil {
		panic("编码 Transfer[2] 失败: %w" + err.Error())
	}

	// 指令 3: InitPool(demo_pool, "simulate batch credit") — 初始化积分池
	wire3, err := demoPd.Encode("InitPool", provider.Args{
		"pool":  demoPool,
		"label": "simulate batch credit",
	})
	if err != nil {
		panic("编码 InitPool 失败: %w" + err.Error())
	}

	// 指令 4: BatchCredit(demo_pool, [demo_recipient], 42) — 批量积分
	wire4, err := demoPd.Encode("BatchCredit", provider.Args{
		"pool":       demoPool,
		"recipients": []crypto.Address{*demoRecipient},
		"amount":     42,
	})
	if err != nil {
		panic("编码 BatchCredit 失败: %w" + err.Error())
	}

	// 指令 5: Transfer(account_c → account_d, 1)
	wire5, err := tokenPd.Encode("Transfer", provider.Args{
		"from":   accountC,
		"token":  api.MIL,
		"to":     accountD,
		"amount": transferAmount,
	})
	if err != nil {
		panic("编码 Transfer[5] 失败: %w" + err.Error())
	}

	// 指令 6: Transfer(account_d → account_a, 1)
	wire6, err := tokenPd.Encode("Transfer", provider.Args{
		"from":   accountD,
		"token":  api.MIL,
		"to":     accountA,
		"amount": transferAmount,
	})
	if err != nil {
		panic("编码 Transfer[6] 失败: %w" + err.Error())
	}

	// 2. Define signing slots once (shared by simulate & real sign)
	slots := []lib.SigningSlot{
		{*accountA, []uint8{0, 1}, true, lib.PubKeySignatureMode{PublicKey: *pkA}},
		{*accountB, []uint8{2}, false, lib.PubKeySignatureMode{PublicKey: *pkB}},
		{*demoPool, []uint8{3, 4}, false, lib.PubKeySignatureMode{PublicKey: *demoPk}},
		{*accountC, []uint8{5}, false, lib.PubKeySignatureMode{PublicKey: *pkC}},
		{*accountD, []uint8{6}, false, lib.PubKeySignatureMode{PublicKey: *pkD}},
	}

	// 3. Build transaction once, reuse the same builder for simulate & real sign (same Stamp -> same TxHash)
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire0, wire1, wire2, wire3, wire4, wire5, wire6}).WithPayer(accountA).ApplySlots(slots)

	// 3.1 Simulate on-chain first (no private key needed, dry-run)
	simulateTx, err := builder.SimulateSlots().Build()
	if err != nil {
		panic("Failed to simulate transaction:" + err.Error())
	}
	simulateResult, err := client.SimulateTx(simulateTx)
	if err != nil {
		panic("Failed to simulate transaction on chain:" + err.Error())
	}
	if simulateResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Simulate failed on chain: error code = %d", simulateResult.BodySimulateReceipt.Error.Code))
	}
	fmt.Printf("Simulated transaction hash: %s, gas charged: %d\n", simulateTx.TxHash(), simulateResult.BodySimulateReceipt.GasCharged)

	// 3.2 Replace simulated signatures with real ones on the same transaction
	tx, err := builder.ResetSigs().
		SignWith(
			lib.Signer{SecretKey: signerA, PublicKey: *pkA},
			lib.Signer{SecretKey: signerB, PublicKey: *pkB},
			lib.Signer{SecretKey: signerC, PublicKey: *pkC},
			lib.Signer{SecretKey: signerD, PublicKey: *pkD},
			lib.Signer{SecretKey: demoSk, PublicKey: *demoPk},
		).
		Build()
	if err != nil {
		panic("Failed to build and sign transaction:" + err.Error())
	}

	// 4. Submit transaction on chain
	err = client.SubmitTx(tx)
	if err != nil {
		panic("Failed to submit transaction:" + err.Error())
	}

	// 5. Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash(), 1)
	if err != nil {
		panic("Failed to wait for transaction:" + err.Error())
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Transaction failed on chain: error code = %d", *getTxByHashResult.BodyTxHistory.Receipt.Error))
	}
	fmt.Printf("Submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// Display TxHistory
	helper.DisplayTxHistory(client, getTxByHashResult.BodyTxHistory)

	// Display EventsByTxHash
	if len(getTxByHashResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, tx.TxHash(), nil)
	}
}
