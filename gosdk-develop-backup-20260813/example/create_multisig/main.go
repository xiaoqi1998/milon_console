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

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// 创建 4 个签名者：signerMultiSig 为多签钱包创建者，signerA/B/C 为多签参与者
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

	// 为多签钱包领取初始代币（此时还是普通账户，由创建者单签）
	if err := client.ClaimFaucet(signerMultiSig, accountMultiSig, lib.PubKeySignatureMode{PublicKey: *pkMultiSig}); err != nil {
		panic("failed to ClaimFaucet MIL:" + err.Error())
	}

	fmt.Printf("\n================ 1.CreateMultisig ================\n")

	// 1. 编码指令：创建多签账户，签名者列表 [pkA, pkB, pkC]（位置 0/1/2），权重 [10,20,30]，阈值 40
	wire, err := gen.Account.CreateMultisig.Args(accountMultiSig, []*crypto.PublicKey{pkA, pkB, pkC}, []uint8{1, 2, 3}, 4).Encode()
	if err != nil {
		panic("failed to encode CreateMultisig instruction:" + err.Error())
	}

	// 2. 提交交易上链：统付模式，创建者单签 ix0 + gas（bit63）
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

	// 3. 等待交易完成
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

	// 1. 编码指令：从多签钱包转账 1000 MIL 给 accountA
	wire, err = gen.Token.Transfer.Args(accountMultiSig, api.MILToken, accountA, 1000).Encode()
	if err != nil {
		panic("failed to encode Transfer instruction:" + err.Error())
	}

	// 2. 构建交易：统付模式（WithPayer），gas 由多签钱包支付。
	//    同一 builder 复用（Stamp 固定）→ 模拟与真签的 TxHash 一致。
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountMultiSig)

	tx, err = builder.Build()
	if err != nil {
		panic("failed to build transaction:" + err.Error())
	}
	txHash := tx.TxHash()
	ixPart := []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}

	// 3. 先模拟：参与者生成全零占位符签名（长度与真签一致），合并为一条多签签名。
	//    统付模式：多签签名授权 ix0 + gas（bit63）。
	//    （pkB 位置 1、pkC 位置 2，权重 20+30 >= 阈值 40。）
	simSig, err := lib.NewAccountSignatureBuilder().AuthorizeIxAndPayer(0).
		SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pkB}). // 参与者 signerB（位置 1）
		SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pkC}). // 参与者 signerC（位置 2）
		Build()
	if err != nil {
		panic("failed to build simulated multisig signature:" + err.Error())
	}

	// 4. 链上模拟（dry-run，无需私钥）
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

	// 5. 模拟通过后清空占位符，换真实多签签名（同一 builder → TxHash 不变）
	multisigSig, err := lib.NewAccountSignatureBuilder().AuthorizeIxAndPayer(0).
		SignMultisigKey(*accountMultiSig, signerB, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pkB}). // 参与者 signerB（位置 1）
		SignMultisigKey(*accountMultiSig, signerC, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pkC}). // 参与者 signerC（位置 2）
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

	// 6. 等待交易完成
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)
}
