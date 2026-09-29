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
// create_sft -> create_slot -> mint -> split -> transfer(部分/全量) -> approve -> transfer_from
// -> freeze/unfreeze -> merge
//
// 所有权模型：TOKEN 份额按 (token_id, owner) 独立记账，同一个 token_id 可以被多个地址同时持有。
//   - transfer / transfer_from 只搬运份额，token_id 保持不变；
//   - split 从源 TOKEN 拆出份额并派生同 SLOT 的新 token_id（拆出量必须严格小于签名者份额）；
//   - merge 把签名者在源 TOKEN 的全部份额并入同 SLOT 的目标 TOKEN；
//   - 元数据三级继承：SFT metadata <- SLOT 覆盖 <- TOKEN 覆盖，view 返回解析后的结果。
func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// 账户角色：
	//   sft   : SFT 资源账户，仅作为 create_sft 的指令签名者（gas 由 owner 代付，无需领水）
	//   owner : SFT owner；mint authority / update author / freeze authority 未转移时默认也是它
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

	// 份额查询用的持有人清单（同一 TOKEN 可能同时由多人持有）。
	holders := []holder{{"owner", owner}, {"alice", alice}, {"bob", bob}}

	fmt.Printf("\n================ 1.ClaimFaucet ================\n")
	// sft 仅作为 create_sft 的指令签名者，gas 由 owner 代付，无需领取 MIL。
	if err := client.ClaimFaucet(ownerSk, owner, lib.PubKeySignatureMode{PublicKey: *ownerPk}); err != nil {
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

	fmt.Printf("\n================ 2.CreateSft(sft sign, owner pays gas) ================\n")
	// create_sft 由 sft 资源账户签名指令（owner 指定为 owner 账户），gas 由 owner 代付：
	// sft 仅签 ix0，owner 作为 payer 签 bit63(gas)。
	// metadata 是结构体：name(1..=128) / symbol(1..=32) 必填且创建后不可变，
	// cover_url / metadata 为 URI，attribute 可选；royalty_bps 是二级市场版税万分比，
	// 版税接收人初始为 owner。
	wire, err := gen.Sftoken.CreateSft.Args(sft, owner, gen.SftokenMetadata{
		Name:      "Milon SFT Demo",
		Symbol:    "MSFT",
		CoverUrl:  "https://milon.test/sft.png",
		Metadata:  "https://milon.test/sft.json",
		Attribute: strPtr("series=2026"),
	}, 50).Encode()
	if err != nil {
		panic("failed to encode CreateSft instruction:" + err.Error())
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(owner).
		AddIxesSig(*sft, sftSk, []uint8{0}, false, lib.PubKeySignatureMode{PublicKey: *sftPk}).
		AddPayerSig(*owner, ownerSk, lib.PubKeySignatureMode{PublicKey: *ownerPk}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	submitAndWait(client, tx)

	fmt.Printf("\n================ 3.CreateSlot(owner sign, is_transferable=true) ================\n")
	// slot 只能由 SFT owner 创建；SlotData.metadata 是可选的 MetadataOverride，
	// 未提供（或为空串）的字段动态继承 SFT metadata。
	// mint / update / freeze 权限不再在 create_slot 里指定，默认归属 SFT owner，
	// 需要时再用 transfer_mint_authority / transfer_update_author / transfer_freeze_authority 转移。
	wire, err = gen.Sftoken.CreateSlot.Args(owner, sft, gen.SftokenSlotData{
		Metadata: &gen.SftokenMetadataOverride{
			Name:      strPtr("Level-1 VIP Card"),
			Attribute: strPtr("level=1"),
		},
	}, true).Encode()
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
	// mint 需要 mint authority 签名（未转移时即 SFT owner）；amount 必须大于 0；
	// metadata 是 TOKEN 级覆盖，传 nil 时完全继承 SLOT。份额直接记在 (token_id, to) 上。
	wire, err = gen.Sftoken.Mint.Args(owner, sft, slotID, owner, 100, &gen.SftokenMetadataOverride{
		Name:      strPtr("Gold Card #1"),
		Attribute: strPtr("grade=A"),
	}).Encode()
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
	fmt.Printf("minted: token_id = %d, owner holds 100\n", token1)
	// 新 SFT 的 token_id 从 1 开始递增分配，第一个 mint 的 TOKEN 必然是 1。
	requireU64(token1, 1, "minted token_id")
	requireU64(balanceOf(client, sft, token1, owner), 100, "owner balance after mint")
	displayToken(client, sft, token1, holders...)

	fmt.Printf("\n================ 5.Split: owner splits 40 of token %d to alice(owner sign) ================\n", token1)
	// split 由份额持有人签名：从源 TOKEN 拆出 amount，创建同 SLOT 的新 TOKEN 给 to（to 可以是自己）。
	// 约束：amount 必须严格小于签名者当前份额（全额转出请用 transfer），
	// to 不是自己时还要求 slot is_transferable=true；新 TOKEN 继承源 TOKEN 的元数据覆盖。
	wire, err = gen.Sftoken.Split.Args(owner, sft, token1, alice, 40).Encode()
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
	// split 复用 TokenTransferredEvent：from_token_id 是源 TOKEN，to_token_id 是派生出的新 TOKEN。
	token2 := receiptEventU64(client, submitAndWait(client, tx), "TokenTransferredEvent", "to_token_id")
	fmt.Printf("split: token %d (100) -> new token %d (40 for alice), owner keeps 60 on token %d\n", token1, token2, token1)
	// split 派生的新 TOKEN 拿到下一个 token_id。
	requireU64(token2, 2, "split to_token_id")
	requireU64(balanceOf(client, sft, token1, owner), 60, "owner balance on token1 after split")
	requireU64(balanceOf(client, sft, token2, alice), 40, "alice balance on token2 after split")

	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 6.Transfer part: owner transfers 20 of token %d to bob(owner sign) ================\n", token1)
	// transfer 只搬运份额：token_id 不变，转出方减少、转入方增加，于是同一个 TOKEN 由多人共持。
	// 要求 slot is_transferable=true、份额充足且未被冻结，from != to。
	wire, err = gen.Sftoken.Transfer.Args(owner, sft, token1, bob, 20).Encode()
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
	txHistory := submitAndWait(client, tx)
	requireU64(receiptEventU64(client, txHistory, "TokenTransferredEvent", "amount"), 20, "transferred amount")
	// 份额转移不派生新 TOKEN：事件里 to_token_id 与 from_token_id 相同。
	requireU64(receiptEventU64(client, txHistory, "TokenTransferredEvent", "to_token_id"), token1, "transfer to_token_id")
	fmt.Printf("part transfer: token %d -> owner keeps 40, bob holds 20 (same token_id)\n", token1)
	requireU64(balanceOf(client, sft, token1, owner), 40, "owner balance on token1 after part transfer")
	requireU64(balanceOf(client, sft, token1, bob), 20, "bob balance on token1 after part transfer")
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 7.Transfer all: alice transfers 40 of token %d to bob(alice sign) ================\n", token2)
	// 全量转出后，转出方在该 TOKEN 上的份额记录被删除，token2 只剩 bob 持有；token_id 依然不变。
	wire, err = gen.Sftoken.Transfer.Args(alice, sft, token2, bob, 40).Encode()
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
	fmt.Printf("full transfer: token %d -> bob holds 40, alice holds 0\n", token2)
	requireU64(balanceOf(client, sft, token2, alice), 0, "alice balance on token2 after full transfer")
	requireU64(balanceOf(client, sft, token2, bob), 40, "bob balance on token2 after full transfer")
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 8.Approve: bob approves alice 30 on token %d(bob sign) ================\n", token2)
	// approve 由份额持有人签名，额度记录在 (token_id, owner, spender) 上，
	// 因此同一个 TOKEN 的不同持有人可以各自授权不同的 spender。
	wire, err = gen.Sftoken.Approve.Args(bob, sft, token2, alice, 30).Encode()
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
	approval := viewValue(client, gen.Sftoken.ApprovalOf.Args(sft, token2, bob, alice), gen.Sftoken.ApprovalOf.DecodeView)
	// approve 30 后授权额度应为 30。
	requireU64(approval, 30, "approval after approve")
	fmt.Printf("alice approval on token %d (owner=bob): %d\n", token2, approval)

	fmt.Printf("\n================ 9.TransferFrom: alice spends 30 of token %d from bob to owner(alice sign, bob pays gas) ================\n", token2)
	// transfer_from 由 spender 签名，要求 (token_id, from, spender) 上有足额授权；
	// 它走 transfer 路径（份额搬运，token_id 不变，要求 is_transferable=true），成功后扣减授权额度。
	// 分账交易：spender(alice) 签指令，payer(bob) 付 gas。
	wire, err = gen.Sftoken.TransferFrom.Args(alice, sft, token2, bob, owner, 30).Encode()
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
	submitAndWait(client, tx)
	fmt.Printf("transfer_from: token %d -> bob keeps 10, owner holds 30 (same token_id)\n", token2)
	requireU64(balanceOf(client, sft, token2, bob), 10, "bob balance on token2 after transfer_from")
	requireU64(balanceOf(client, sft, token2, owner), 30, "owner balance on token2 after transfer_from")

	approval = viewValue(client, gen.Sftoken.ApprovalOf.Args(sft, token2, bob, alice), gen.Sftoken.ApprovalOf.DecodeView)
	// 花费 30 后授权额度归零（额度耗尽时记录被删除）。
	requireU64(approval, 0, "approval after transfer_from")
	fmt.Printf("alice approval on token %d after spend: %d\n", token2, approval)
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 10.Freeze/Unfreeze: owner freezes bob on token %d(owner sign) ================\n", token2)
	// freeze / unfreeze 由 freeze authority 签名（未转移时即 SFT owner），
	// 冻结粒度是 (token_id, owner)：被冻结的份额无法 transfer / transfer_from / split / merge / burn。
	wire, err = gen.Sftoken.Freeze.Args(owner, sft, token2, bob).Encode()
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
	submitAndWait(client, tx)
	frozen := viewValue(client, gen.Sftoken.FrozenOf.Args(sft, token2, bob), gen.Sftoken.FrozenOf.DecodeView)
	requireBool(frozen, true, "bob frozen on token2 after freeze")
	fmt.Printf("bob frozen on token %d: %v\n", token2, frozen)

	wire, err = gen.Sftoken.Unfreeze.Args(owner, sft, token2, bob).Encode()
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
	submitAndWait(client, tx)
	frozen = viewValue(client, gen.Sftoken.FrozenOf.Args(sft, token2, bob), gen.Sftoken.FrozenOf.DecodeView)
	requireBool(frozen, false, "bob frozen on token2 after unfreeze")
	fmt.Printf("bob frozen on token %d after unfreeze: %v\n", token2, frozen)

	fmt.Printf("\n================ 11.Merge: bob merges token %d (20) into token %d(bob sign) ================\n", token1, token2)
	// merge 由签名者把自己在源 TOKEN 的全部份额并入目标 TOKEN：两个 TOKEN 必须属于同一个 SLOT，
	// 且签名者在源 TOKEN 上份额大于 0；合并后源 TOKEN 上该持有人的份额记录被删除（TOKEN 本身仍存在，
	// 其他持有人不受影响）。
	wire, err = gen.Sftoken.Merge.Args(bob, sft, token1, token2).Encode()
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
	txHistory = submitAndWait(client, tx)
	mergedAmount := receiptEventU64(client, txHistory, "TokenTransferredEvent", "amount")
	// bob 在 token1 上的 20 全部并入 token2。
	requireU64(mergedAmount, 20, "merged amount")
	requireU64(receiptEventU64(client, txHistory, "TokenTransferredEvent", "to_token_id"), token2, "merge to_token_id")
	fmt.Printf("merge: %d moved from token %d into token %d, bob's balance on token %d cleared\n",
		mergedAmount, token1, token2, token1)
	requireU64(balanceOf(client, sft, token1, bob), 0, "bob balance on token1 after merge")
	requireU64(balanceOf(client, sft, token2, bob), 30, "bob balance on token2 after merge")
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 12.Final states ================\n")
	sftOwner := viewValue(client, gen.Sftoken.SftOwnerOf.Args(sft), gen.Sftoken.SftOwnerOf.DecodeView)
	sftMeta := viewValue(client, gen.Sftoken.SftMetadata.Args(sft), gen.Sftoken.SftMetadata.DecodeView)
	fmt.Printf("sft: owner=%v name=%q symbol=%q cover_url=%q uri=%q attribute=%s\n",
		sftOwner, sftMeta.Name, sftMeta.Symbol, sftMeta.CoverUrl, sftMeta.Metadata, optStr(sftMeta.Attribute))

	royalty := viewValue(client, gen.Sftoken.RoyaltyInfo.Args(sft), gen.Sftoken.RoyaltyInfo.DecodeView)
	fmt.Printf("royalty: recipient=%v bps=%d\n", royalty.Recipient, royalty.Bps)

	// 三个权限未转移时都回落到 SFT owner。
	mintAuthority := viewValue(client, gen.Sftoken.MintAuthority.Args(sft), gen.Sftoken.MintAuthority.DecodeView)
	updateAuthor := viewValue(client, gen.Sftoken.UpdateAuthor.Args(sft), gen.Sftoken.UpdateAuthor.DecodeView)
	freezeAuthority := viewValue(client, gen.Sftoken.FreezeAuthority.Args(sft), gen.Sftoken.FreezeAuthority.DecodeView)
	fmt.Printf("authorities: mint=%v update=%v freeze=%v\n", mintAuthority, updateAuthor, freezeAuthority)

	transferable := viewValue(client, gen.Sftoken.IsSlotTransferable.Args(sft, slotID), gen.Sftoken.IsSlotTransferable.DecodeView)
	slotInfo := viewValue(client, gen.Sftoken.SlotInfo.Args(sft, slotID), gen.Sftoken.SlotInfo.DecodeView)
	slotMeta := viewValue(client, gen.Sftoken.SlotMetadata.Args(sft, slotID), gen.Sftoken.SlotMetadata.DecodeView)
	fmt.Printf("slot %d: is_transferable=%v override=%v\n", slotID, transferable, overrideStr(slotInfo.Metadata))
	fmt.Printf("slot %d resolved metadata: name=%q symbol=%q attribute=%s\n",
		slotID, slotMeta.Name, slotMeta.Symbol, optStr(slotMeta.Attribute))

	approval = viewValue(client, gen.Sftoken.ApprovalOf.Args(sft, token2, bob, alice), gen.Sftoken.ApprovalOf.DecodeView)
	fmt.Printf("alice approval on token %d (owner=bob): %d\n", token2, approval)

	// 最终份额：token1 只剩 owner 40；token2 由 bob 30 与 owner 30 共持。
	fmt.Printf("final holders:\n")
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)
}

// **************************************** local helpers ****************************************//

// holder 给份额查询附带可读名字。
type holder struct {
	name string
	addr *crypto.Address
}

// strPtr 构造 IDL option<String> 字段需要的字符串指针。
func strPtr(s string) *string { return &s }

// optStr 打印可选字符串：nil 显示为 <none>，便于区分“未设置”与空串。
func optStr(v *string) string {
	if v == nil {
		return "<none>"
	}
	return *v
}

// overrideStr 打印元数据覆盖项，nil 表示完全继承上一级。
func overrideStr(override *gen.SftokenMetadataOverride) string {
	if override == nil {
		return "<none, inherit>"
	}
	return fmt.Sprintf("name=%s symbol=%s attribute=%s",
		optStr(override.Name), optStr(override.Symbol), optStr(override.Attribute))
}

// requireU64 校验链上返回值符合预期，不符合直接 panic。
func requireU64(got, want uint64, what string) {
	if got != want {
		panic(fmt.Sprintf("unexpected %s: got %d, want %d", what, got, want))
	}
}

// requireBool 校验链上返回的布尔值符合预期，不符合直接 panic。
func requireBool(got, want bool, what string) {
	if got != want {
		panic(fmt.Sprintf("unexpected %s: got %v, want %v", what, got, want))
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

// balanceOf 查询某个地址在指定 TOKEN 上的份额。
func balanceOf(client *milon.Client, sft *crypto.Address, tokenID uint64, owner *crypto.Address) uint64 {
	return viewValue(client, gen.Sftoken.BalanceOf.Args(sft, tokenID, owner), gen.Sftoken.BalanceOf.DecodeView)
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

// receiptEventU64 从交易回执中提取指定事件的 u64 字段（如 slot_id / token_id / to_token_id / amount）。
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

// displayToken 打印 TOKEN 的链上最新状态：继承解析后的元数据，以及每个持有人的份额与冻结状态。
func displayToken(client *milon.Client, sft *crypto.Address, tokenID uint64, holders ...holder) {
	slotID := viewValue(client, gen.Sftoken.SlotOf.Args(sft, tokenID), gen.Sftoken.SlotOf.DecodeView)
	meta := viewValue(client, gen.Sftoken.Token.Args(sft, tokenID), gen.Sftoken.Token.DecodeView)
	fmt.Printf("  token_id=%d slot_id=%d name=%q symbol=%q cover_url=%q attribute=%s\n", tokenID, slotID, meta.Name, meta.Symbol, meta.CoverUrl, optStr(meta.Attribute))

	for _, h := range holders {
		balance := balanceOf(client, sft, tokenID, h.addr)
		frozen := viewValue(client, gen.Sftoken.FrozenOf.Args(sft, tokenID, h.addr), gen.Sftoken.FrozenOf.DecodeView)
		fmt.Printf("    holder=%-6s addr=%v balance=%d frozen=%v\n", h.name, h.addr, balance, frozen)
	}
}
