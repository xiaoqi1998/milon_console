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

// example 演示 sftoken 合约的完整生命周期：
// create_sft -> create_slot -> mint -> split -> transfer(部分/全量) -> approve -> transfer_from -> merge
func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// 账户角色：
	//   sft   : SFT 资源账户，仅作为 create_sft 的签名者
	//   owner : SFT owner / slot owner / mint authority / update author / freeze authority
	//   alice : split 接收者 / 全量 transfer 发起者 / spender（被授权划转）
	//   bob   : 部分 transfer 接收者 / approve 授权者 / merge 发起者
	sftSk, sftPk, sft := newAccount()
	ownerSk, ownerPk, owner := newAccount()
	aliceSk, alicePk, alice := newAccount()
	bobSk, bobPk, bob := newAccount()

	fmt.Printf("sft = %v \n", sft)
	fmt.Printf("owner = %v \n", owner)
	fmt.Printf("alice = %v \n", alice)
	fmt.Printf("bob = %v \n\n", bob)

	fmt.Printf("\n================ 1.ClaimFaucet ================\n")
	if err := client.ClaimFaucet(sftSk, sft, lib.PubKeySignatureMode{PublicKey: *sftPk}); err != nil {
		panic("failed to ClaimFaucet sft:" + err.Error())
	}
	sftBalance, err := client.BalanceOf(sft)
	if err != nil {
		panic("failed to get sft MIL:" + err.Error())
	}
	fmt.Printf("sft MIL: %d\n", sftBalance)

	if err = client.ClaimFaucet(ownerSk, owner, lib.PubKeySignatureMode{PublicKey: *ownerPk}); err != nil {
		panic("failed to ClaimFaucet owner:" + err.Error())
	}
	ownerBalance, err := client.BalanceOf(owner)
	if err != nil {
		panic("failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	if err = client.ClaimFaucet(aliceSk, alice, lib.PubKeySignatureMode{PublicKey: *alicePk}); err != nil {
		panic("failed to ClaimFaucet alice:" + err.Error())
	}
	aliceBalance, err := client.BalanceOf(alice)
	if err != nil {
		panic("failed to get alice MIL:" + err.Error())
	}
	fmt.Printf("alice MIL: %d\n", aliceBalance)

	if err = client.ClaimFaucet(bobSk, bob, lib.PubKeySignatureMode{PublicKey: *bobPk}); err != nil {
		panic("failed to ClaimFaucet bob:" + err.Error())
	}
	bobBalance, err := client.BalanceOf(bob)
	if err != nil {
		panic("failed to get bob MIL:" + err.Error())
	}
	fmt.Printf("bob MIL: %d\n", bobBalance)

	fmt.Printf("\n================ 2.CreateSft(sft sign) ================\n")
	// create_sft 由 sft 资源账户签名，owner 指定为 owner 账户。
	wire, err := gen.Sftoken.CreateSft.Args(sft, owner, `{"name":"Milon SFT Demo","desc":"semi-fungible token"}`).Encode()
	if err != nil {
		panic("failed to encode CreateSft instruction:" + err.Error())
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(sft).
		AddIxesSig(*sft, sftSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *sftPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	submitAndWait(client, tx)

	fmt.Printf("\n================ 3.CreateSlot(owner sign, is_transferable=true) ================\n")
	// slot 只能由 SFT owner 创建；slot_owner / mint_authority / update_author / freeze_authority
	// 均为可选项，这里全部指定为 owner。
	authority := owner
	wire, err = gen.Sftoken.CreateSlot.Args(sft, owner, "Level-1 VIP Card", "level=1", true, &authority, &authority, &authority, &authority).Encode()
	if err != nil {
		panic("failed to encode CreateSlot instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(owner).
		AddIxesSig(*owner, ownerSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *ownerPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	// slot_id 由链上全局递增分配（从 1 开始），从回执事件中提取，避免硬编码。
	slotID := receiptEventU64(client, submitAndWait(client, tx), "SlotCreatedEvent", "slot_id")
	fmt.Printf("slot created: slot_id = %d\n", slotID)
	// 新 SFT 的 slot_id 从 1 开始递增分配，第一个 slot 必然是 1。
	requireU64(slotID, 1, "slot_id")

	fmt.Printf("\n================ 4.Mint 100 to owner(owner sign) ================\n")
	// mint 需要 mint_authority 签名；initial_value 必须大于 0。
	wire, err = gen.Sftoken.Mint.Args(sft, owner, slotID, owner, 100, "Gold Card #1", "grade=A").Encode()
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
	token1 := receiptEventU64(client, submitAndWait(client, tx), "TokenMintedEvent", "token_id")
	fmt.Printf("minted: token_id = %d\n", token1)
	// 新 SFT 的 token_id 从 1 开始递增分配，第一个 mint 的 TOKEN 必然是 1。
	requireU64(token1, 1, "minted token_id")
	displayToken(client, sft, token1)

	fmt.Printf("\n================ 5.Split: owner splits 40 of token %d to alice(owner sign) ================\n", token1)
	// split 只要求签名者是源 TOKEN 的 owner，不要求 slot is_transferable；
	// 新 TOKEN 继承 slot_id / metadata / attribute，源 TOKEN 保留剩余 60。
	wire, err = gen.Sftoken.Split.Args(sft, owner, token1, alice, 40).Encode()
	if err != nil {
		panic("failed to encode Split instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(owner).
		AddIxesSig(*owner, ownerSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *ownerPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	token2 := receiptEventU64(client, submitAndWait(client, tx), "TokenSplitEvent", "new_token_id")
	fmt.Printf("split: token %d (100) -> new token %d (40 for alice), remaining 60\n", token1, token2)
	// split 为 alice 新建的 TOKEN 拿到下一个 token_id。
	requireU64(token2, 2, "split new_token_id")

	displayToken(client, sft, token1)
	displayToken(client, sft, token2)

	fmt.Printf("\n================ 6.Transfer part: owner transfers 20 of token %d to bob(owner sign) ================\n", token1)
	// 部分 value 转移走拆分路径（要求 is_transferable=true），为 bob 新建一个 TOKEN。
	wire, err = gen.Sftoken.Transfer.Args(sft, owner, token1, bob, 20).Encode()
	if err != nil {
		panic("failed to encode Transfer instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(owner).
		AddIxesSig(*owner, ownerSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *ownerPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	token3 := receiptEventU64(client, submitAndWait(client, tx), "TokenSplitEvent", "new_token_id")
	fmt.Printf("part transfer: token %d (60) -> new token %d (20 for bob), remaining 40\n", token1, token3)
	// 部分转移为 bob 新建的 TOKEN 拿到下一个 token_id。
	requireU64(token3, 3, "transfer new_token_id")
	displayToken(client, sft, token1)
	displayToken(client, sft, token2)
	displayToken(client, sft, token3)

	fmt.Printf("\n================ 7.Transfer all: alice transfers token %d to bob(alice sign) ================\n", token2)
	// 全量 value 转移保留 token_id，仅变更 owner（TokenTransferredEvent），不要求 is_transferable。
	wire, err = gen.Sftoken.Transfer.Args(sft, alice, token2, bob, 40).Encode()
	if err != nil {
		panic("failed to encode Transfer instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice).
		AddIxesSig(*alice, aliceSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alicePk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	submitAndWait(client, tx)
	fmt.Printf("full transfer: token %d owner is now bob, token_id preserved\n", token2)
	displayToken(client, sft, token1)
	displayToken(client, sft, token2)
	displayToken(client, sft, token3)

	fmt.Printf("\n================ 8.Approve: bob approves alice 30 on token %d(bob sign) ================\n", token2)
	// approve 需要 TOKEN owner 签名，额度记录在 (token_id, spender) 上。
	wire, err = gen.Sftoken.Approve.Args(sft, bob, token2, alice, 30).Encode()
	if err != nil {
		panic("failed to encode Approve instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(bob).
		AddIxesSig(*bob, bobSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *bobPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	submitAndWait(client, tx)
	approval := viewValue(client, gen.Sftoken.ApprovalOf.Args(sft, token2, alice), gen.Sftoken.ApprovalOf.DecodeView)
	// approve 30 后授权额度应为 30。
	requireU64(approval, 30, "approval after approve")
	fmt.Printf("alice approval on token %d: %d\n", token2, approval)
	displayToken(client, sft, token1)
	displayToken(client, sft, token2)
	displayToken(client, sft, token3)

	fmt.Printf("\n================ 9.TransferFrom: alice spends 30 of token %d to owner(alice sign, bob pays gas) ================\n", token2)
	// transfer_from 要求 spender 拥有足额 approve；它始终走拆分路径（要求 is_transferable=true）
	// 并扣减授权额度。分账交易：spender(alice) 签指令，payer(bob) 付 gas。
	wire, err = gen.Sftoken.TransferFrom.Args(sft, alice, token2, owner, 30).Encode()
	if err != nil {
		panic("failed to encode TransferFrom instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(bob).
		AddIxesSig(*alice, aliceSk, []uint8{0}, false, lib.PubKeySignatureMode{PublicKey: *alicePk}).
		AddPayerSig(*bob, bobSk, lib.PubKeySignatureMode{PublicKey: *bobPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	token4 := receiptEventU64(client, submitAndWait(client, tx), "TokenSplitEvent", "new_token_id")
	// transfer_from 为 owner 新建的 TOKEN 拿到下一个 token_id。
	requireU64(token4, 4, "transfer_from new_token_id")
	fmt.Printf("transfer_from: token %d (40) -> new token %d (30 for owner), remaining 10\n", token2, token4)
	displayToken(client, sft, token1)
	displayToken(client, sft, token2)
	displayToken(client, sft, token3)
	displayToken(client, sft, token4)

	approval = viewValue(client, gen.Sftoken.ApprovalOf.Args(sft, token2, alice), gen.Sftoken.ApprovalOf.DecodeView)
	// 花费 30 后授权额度归零（consume_approval 扣减）。
	requireU64(approval, 0, "approval after transfer_from")
	fmt.Printf("alice approval on token %d after spend: %d\n", token2, approval)

	fmt.Printf("\n================ 10.Merge: bob merges token %d (20) into token %d(bob sign) ================\n", token3, token2)
	// merge 要求签名者是源 TOKEN 的 owner，两个 TOKEN 属于同一 slot 且 is_transferable=true；
	// 源 TOKEN 全量并入目标 TOKEN 后销毁。
	wire, err = gen.Sftoken.Merge.Args(sft, bob, token3, token2).Encode()
	if err != nil {
		panic("failed to encode Merge instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(bob).
		AddIxesSig(*bob, bobSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *bobPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	mergedValue := receiptEventU64(client, submitAndWait(client, tx), "TokensMergedEvent", "value")
	// token3 的 20 全部并入 token2。
	requireU64(mergedValue, 20, "merged value")

	fmt.Printf("merge: %d value moved from token %d into token %d, source token burned\n", mergedValue, token3, token2)
	displayToken(client, sft, token1)
	displayToken(client, sft, token2)
	displayToken(client, sft, token4)

	fmt.Printf("\n================ 11.Final states ================\n")
	sftInfo := viewValue(client, gen.Sftoken.SftInfo.Args(sft), gen.Sftoken.SftInfo.DecodeView)
	fmt.Printf("sft: owner=%v metadata=%q\n", sftInfo.Owner, sftInfo.Metadata)

	slotInfo := viewValue(client, gen.Sftoken.SlotInfo.Args(sft, slotID), gen.Sftoken.SlotInfo.DecodeView)
	fmt.Printf("slot %d: metadata=%v attribute=%v is_transferable=%v\n", slotID, slotInfo.Metadata, slotInfo.Attribute, slotInfo.IsTransferable)

	frozen := viewValue(client, gen.Sftoken.IsSlotFrozen.Args(sft, slotID), gen.Sftoken.IsSlotFrozen.DecodeView)
	fmt.Printf("slot %d frozen: %v\n", slotID, frozen)

	approval = viewValue(client, gen.Sftoken.ApprovalOf.Args(sft, token2, alice), gen.Sftoken.ApprovalOf.DecodeView)
	fmt.Printf("alice approval on token %d: %d\n", token2, approval)

	fmt.Printf("owner tokens:\n")
	displayToken(client, sft, token1)
	displayToken(client, sft, token4)
	fmt.Printf("bob tokens:\n")
	displayToken(client, sft, token2)
}

// **************************************** local helpers ****************************************//

// requireU64 校验链上返回值符合预期，不符合直接 panic。
func requireU64(got, want uint64, what string) {
	if got != want {
		panic(fmt.Sprintf("unexpected %s: got %d, want %d", what, got, want))
	}
}

// newAccount 生成一个新账户：由 Ed25519 公钥派生地址的 classical 密钥对。
func newAccount() (*crypto.ClassicalSecretKey, *crypto.PublicKey, *crypto.Address) {
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk := sk.Ed25519Public()
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		panic("failed to derive address from public key:" + err.Error())
	}
	return sk, pk, addr
}

// submitAndWait 提交交易、等待上链成功，并返回回执全文供事件提取。
func submitAndWait(client *milon.Client, tx *lib.Transaction) *api.TxHistory {
	if err := client.SubmitTx(tx); err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	fmt.Printf("and we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)
	return getTxByHashResult.BodyTxHistory
}

// encoder 抽象 gen 指令参数对象的 Encode 方法。
type encoder interface {
	Encode() (api.PackedInstruction, error)
}

// viewValue 调用 view 指令并用对应的 DecodeView 解码返回值。
func viewValue[T any](client *milon.Client, args encoder, decode func([]byte) (T, error)) T {
	wire, err := args.Encode()
	if err != nil {
		panic("failed to encode view instruction:" + err.Error())
	}
	viewTxResult, err := client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to call view:" + err.Error())
	}
	value, err := decode(viewTxResult.HTTPResponseBody)
	if err != nil {
		panic("failed to decode view result:" + err.Error())
	}
	return value
}

// sftokenEventTypeTag 从已绑定的 sftoken IDL 中查询事件 typeTag（避免硬编码）。
func sftokenEventTypeTag(eventName string) uint64 {
	for typeTag, event := range gen.Sftoken.Pd.EventByTypeTag {
		if event.Name == eventName {
			return typeTag
		}
	}
	panic("sftoken event not found in IDL: " + eventName)
}

// receiptEventU64 从交易回执中提取指定事件的 u64 字段（如 slot_id / token_id / new_token_id）。
func receiptEventU64(client *milon.Client, txHistory *api.TxHistory, eventName, fieldName string) uint64 {
	typeTag := sftokenEventTypeTag(eventName)
	for _, event := range txHistory.Receipt.Events {
		if event.TypeTag != typeTag {
			continue
		}
		decoded, err := client.GetProviderManager().DecodeEventDataByTag(event.TypeTag, event.Value)
		if err != nil {
			panic("failed to decode event " + eventName + ":" + err.Error())
		}
		data, ok := decoded["data"].(map[string]any)
		if !ok {
			panic("decoded event " + eventName + " has no data")
		}
		value, ok := data[fieldName].(uint64)
		if !ok {
			panic("event " + eventName + " has no u64 field " + fieldName)
		}
		return value
	}
	panic("event " + eventName + " not found in receipt")
}

// displayToken 打印 TOKEN 的链上最新状态。
func displayToken(client *milon.Client, sft *crypto.Address, tokenID uint64) {
	token := viewValue(client, gen.Sftoken.Token.Args(sft, tokenID), gen.Sftoken.Token.DecodeView)
	value := viewValue(client, gen.Sftoken.ValueOf.Args(sft, tokenID), gen.Sftoken.ValueOf.DecodeView)
	fmt.Printf("  token_id=%d slot_id=%d owner=%v value=%d metadata=%q attribute=%q\n",
		token.TokenId, token.SlotId, token.Owner, value, token.Metadata, token.Attribute)
}
