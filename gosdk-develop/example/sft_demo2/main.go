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

// example 演示 SFT 简单场景：mint 一次后，同一个 token_id 由三个人同时持有。
//
// 说明：SFT 的份额按 (token_id, owner) 独立记账，transfer 只搬运份额而不派生新 TOKEN，
// 因此“同 1 个 token_id 由三人持有”是链上原生支持的状态，不再需要靠 split 拆成多个 TOKEN。
// （若业务上需要各自独立的 token_id，才使用 split。）
//
// 流程：
//  1. ClaimFaucet(owner，付 gas 的账户；sft 仅签指令，无需 MIL)
//  2. CreateSft(sft sign, owner pays gas)
//  3. CreateSlot(owner sign, is_transferable=true)
//  4. Mint 100 to owner              -> token1，owner 持有 100
//  5. Transfer 40 to alice           -> token1：owner 60 / alice 40
//  6. Transfer 30 to bob             -> token1：owner 30 / alice 40 / bob 30
//  7. Transfer 30 to carol           -> token1：owner 0 / alice 40 / bob 30 / carol 30
//
// 最终 alice / bob / carol 三人共持同一个 token_id（同 slot、同 metadata）。
func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// 账户角色：
	//   sft   : SFT 资源账户，仅作为 create_sft 的指令签名者（gas 由 owner 代付，无需领水）
	//   owner : SFT owner，同时是默认的 mint authority，初始份额持有人
	//   alice : 第一份接收者（transfer）
	//   bob   : 第二份接收者（transfer）
	//   carol : 第三份接收者（transfer）
	sftSk, sftPk, sft := newAccount()
	ownerSk, ownerPk, owner := newAccount()
	_, _, alice := newAccount()
	_, _, bob := newAccount()
	_, _, carol := newAccount()

	fmt.Printf("sft = %v \n", sft)
	fmt.Printf("owner = %v \n", owner)
	fmt.Printf("alice = %v \n", alice)
	fmt.Printf("bob = %v \n", bob)
	fmt.Printf("carol = %v \n\n", carol)

	// 份额查询用的持有人清单。
	holders := []holder{{"owner", owner}, {"alice", alice}, {"bob", bob}, {"carol", carol}}

	fmt.Printf("\n================ 1.ClaimFaucet ================\n")
	// 只有发起交易（付 gas）的 owner 需要 MIL；sft 仅签指令且 gas 由 owner 代付，
	// alice / bob / carol 仅作为接收方，均无需 faucet。
	if err := client.ClaimFaucet(ownerSk, owner, lib.PubKeySignatureMode{PublicKey: *ownerPk}); err != nil {
		panic("failed to ClaimFaucet owner:" + err.Error())
	}
	ownerBalance, err := client.BalanceOf(owner)
	if err != nil {
		panic("failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	fmt.Printf("\n================ 2.CreateSft(sft sign, owner pays gas) ================\n")
	// create_sft 由 sft 资源账户签名指令（owner 指定为 owner 账户），gas 由 owner 代付：
	// sft 仅签 ix0，owner 作为 payer 签 bit63(gas)。
	// metadata 是结构体：name / symbol 必填，cover_url / metadata 为 URI，attribute 可选；
	// royalty_bps 是二级市场版税万分比，版税接收人初始为 owner。
	wire, err := gen.Sftoken.CreateSft.Args(sft, owner, gen.SftokenMetadata{
		Name:      "Three Holders Demo",
		Symbol:    "THD",
		CoverUrl:  "https://milon.test/sft.png",
		Metadata:  "https://milon.test/sft.json",
		Attribute: strPtr("demo=multi-holder"),
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
	// slot 只能由 SFT owner 创建；份额要能在持有人之间流转，is_transferable 必须为 true。
	// SlotData.metadata 是可选覆盖项，未提供的字段动态继承 SFT metadata。
	wire, err = gen.Sftoken.CreateSlot.Args(owner, sft, gen.SftokenSlotData{
		Metadata: &gen.SftokenMetadataOverride{
			Name:      strPtr("VIP Card"),
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
	// slot_id 由链上全局递增分配，从回执事件中提取，避免硬编码。
	slotID := receiptEventU64(client, submitAndWait(client, tx), "SlotCreatedEvent", "slot_id")
	fmt.Printf("slot created: slot_id = %d\n", slotID)
	requireU64(slotID, 1, "slot_id")

	fmt.Printf("\n================ 4.Mint 100 to owner(owner sign) ================\n")
	// mint 需要 mint authority 签名（未转移时即 SFT owner）；amount 必须大于 0。
	// 本次只 mint 这一份 TOKEN，后面三人共持的都是它。
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
	fmt.Printf("minted: token_id = %d (owner holds 100)\n", token1)
	requireU64(token1, 1, "minted token_id")
	requireU64(balanceOf(client, sft, token1, owner), 100, "owner balance after mint")
	displayToken(client, sft, token1, holders...)

	fmt.Printf("\n================ 5.Transfer 40 of token %d to alice(owner sign) ================\n", token1)
	// transfer 由份额持有人签名，把份额记到接收方名下：token_id 不变，
	// 于是同一个 TOKEN 出现多个持有人。要求 slot is_transferable=true。
	wire, err = gen.Sftoken.Transfer.Args(owner, sft, token1, alice, 40).Encode()
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
	// 份额转移不派生新 TOKEN：事件里 to_token_id 与 from_token_id 相同。
	requireU64(receiptEventU64(client, txHistory, "TokenTransferredEvent", "to_token_id"), token1, "transfer to_token_id")
	fmt.Printf("transfer: token %d -> owner 60, alice 40\n", token1)
	requireU64(balanceOf(client, sft, token1, owner), 60, "owner balance after transfer to alice")
	requireU64(balanceOf(client, sft, token1, alice), 40, "alice balance after transfer")
	displayToken(client, sft, token1, holders...)

	fmt.Printf("\n================ 6.Transfer 30 of token %d to bob(owner sign) ================\n", token1)
	wire, err = gen.Sftoken.Transfer.Args(owner, sft, token1, bob, 30).Encode()
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
	submitAndWait(client, tx)
	fmt.Printf("transfer: token %d -> owner 30, alice 40, bob 30\n", token1)
	requireU64(balanceOf(client, sft, token1, owner), 30, "owner balance after transfer to bob")
	requireU64(balanceOf(client, sft, token1, bob), 30, "bob balance after transfer")
	displayToken(client, sft, token1, holders...)

	fmt.Printf("\n================ 7.Transfer 30 of token %d to carol(owner sign) ================\n", token1)
	// owner 把剩余份额全部转出后，它在 token1 上的份额记录被删除（balance_of 返回 0）。
	wire, err = gen.Sftoken.Transfer.Args(owner, sft, token1, carol, 30).Encode()
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
	submitAndWait(client, tx)
	fmt.Printf("transfer: token %d -> owner 0, alice 40, bob 30, carol 30\n", token1)
	requireU64(balanceOf(client, sft, token1, owner), 0, "owner balance after transfer to carol")
	requireU64(balanceOf(client, sft, token1, carol), 30, "carol balance after transfer")

	fmt.Printf("\n================ Final: three holders of one token_id ================\n")
	fmt.Printf("alice holds: token %d, balance %d\n", token1, balanceOf(client, sft, token1, alice))
	fmt.Printf("bob holds:   token %d, balance %d\n", token1, balanceOf(client, sft, token1, bob))
	fmt.Printf("carol holds: token %d, balance %d\n", token1, balanceOf(client, sft, token1, carol))

	slotMeta := viewValue(client, gen.Sftoken.SlotMetadata.Args(sft, slotID), gen.Sftoken.SlotMetadata.DecodeView)
	tokenMeta := viewValue(client, gen.Sftoken.Token.Args(sft, token1), gen.Sftoken.Token.DecodeView)
	fmt.Printf("slot %d metadata: name=%q attribute=%s\n", slotID, slotMeta.Name, optStr(slotMeta.Attribute))
	fmt.Printf("token %d metadata: name=%q attribute=%s (继承 slot，覆盖项优先生效)\n",
		token1, tokenMeta.Name, optStr(tokenMeta.Attribute))
	displayToken(client, sft, token1, holders...)
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

// receiptEventU64 从交易回执中提取指定事件的 u64 字段（如 slot_id / token_id / to_token_id）。
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
	fmt.Printf("  token_id=%d slot_id=%d name=%q symbol=%q cover_url=%q attribute=%s\n",
		tokenID, slotID, meta.Name, meta.Symbol, meta.CoverUrl, optStr(meta.Attribute))
	for _, h := range holders {
		balance := balanceOf(client, sft, tokenID, h.addr)
		frozen := viewValue(client, gen.Sftoken.FrozenOf.Args(sft, tokenID, h.addr), gen.Sftoken.FrozenOf.DecodeView)
		fmt.Printf("    holder=%-6s addr=%v balance=%d frozen=%v\n", h.name, h.addr, balance, frozen)
	}
}
