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

// example demonstrates a simple SFT scenario: after a single mint, the same token_id is held by three people at once.
//
// Note: SFT shares are accounted independently by (token_id, owner), and transfer only moves shares without deriving a new TOKEN,
// so "one token_id held by three people" is a natively supported on-chain state, no longer needing split to break it into multiple TOKENs.
// (Use split only when the business requires separate token_ids.)
//
// Flow:
//  1. ClaimFaucet(owner, the account paying gas; sft only signs instructions, no MIL needed)
//  2. CreateSft(sft sign, owner pays gas)
//  3. CreateSlot(owner sign, is_transferable=true)
//  4. Mint 100 to owner              -> token1, owner holds 100
//  5. Transfer 40 to alice           -> token1: owner 60 / alice 40
//  6. Transfer 30 to bob             -> token1: owner 30 / alice 40 / bob 30
//  7. Transfer 30 to carol           -> token1: owner 0 / alice 40 / bob 30 / carol 30
//
// Finally alice / bob / carol co-hold the same token_id (same slot, same metadata).
func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// Account roles:
	//   sft   : SFT resource account, only the instruction signer for create_sft (gas paid by owner, no faucet needed)
	//   owner : SFT owner, also the default mint authority and initial share holder
	//   alice : first-share recipient (transfer)
	//   bob   : second-share recipient (transfer)
	//   carol : third-share recipient (transfer)
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

	// Holder list for share queries.
	holders := []holder{{"owner", owner}, {"alice", alice}, {"bob", bob}, {"carol", carol}}

	fmt.Printf("\n================ 1.ClaimFaucet ================\n")
	// Only owner, which initiates transactions (pays gas), needs MIL; sft only signs instructions with gas paid by owner,
	// and alice / bob / carol are just recipients, so none of them need the faucet.
	if err := client.ClaimFaucet(ownerSk, owner, lib.PubKeySignatureMode{PublicKey: *ownerPk}); err != nil {
		panic("failed to ClaimFaucet owner:" + err.Error())
	}
	ownerBalance, err := client.BalanceOf(owner)
	if err != nil {
		panic("failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	fmt.Printf("\n================ 2.CreateSft(sft sign, owner pays gas) ================\n")
	// create_sft is signed by the sft resource account (owner set to the owner account), gas paid by owner:
	// sft signs only ix0, owner signs bit63 (gas) as payer.
	// metadata is a struct: name / symbol are required, cover_url / metadata are URIs, attribute is optional;
	// royalty_bps is the secondary-market royalty in basis points, and the royalty recipient is initially owner.
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
	// a slot can only be created by the SFT owner; for shares to move between holders, is_transferable must be true.
	// SlotData.metadata is an optional override; fields not provided dynamically inherit the SFT metadata.
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
	// slot_id is assigned by a global on-chain increment; extract it from the receipt event to avoid hardcoding.
	slotID := receiptEventU64(client, submitAndWait(client, tx), "SlotCreatedEvent", "slot_id")
	fmt.Printf("slot created: slot_id = %d\n", slotID)
	requireU64(slotID, 1, "slot_id")

	fmt.Printf("\n================ 4.Mint 100 to owner(owner sign) ================\n")
	// mint requires the mint authority's signature (the SFT owner when not transferred); amount must be greater than 0.
	// Only this one TOKEN is minted here; the three people co-hold it afterwards.
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
	// transfer is signed by the share holder and records shares under the recipient: token_id is unchanged,
	// so the same TOKEN ends up with multiple holders. Requires slot is_transferable=true.
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
	// A share transfer does not derive a new TOKEN: in the event, to_token_id equals from_token_id.
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
	// After owner transfers out all remaining shares, its share record on token1 is deleted (balance_of returns 0).
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
	fmt.Printf("token %d metadata: name=%q attribute=%s (inherits slot, overrides take precedence)\n",
		token1, tokenMeta.Name, optStr(tokenMeta.Attribute))
	displayToken(client, sft, token1, holders...)
}

// **************************************** local helpers ****************************************//

// holder attaches a readable name to a share query.
type holder struct {
	name string
	addr *crypto.Address
}

// strPtr builds the string pointer required by an IDL option<String> field.
func strPtr(s string) *string { return &s }

// optStr prints an optional string: nil is shown as <none>, to distinguish "unset" from an empty string.
func optStr(v *string) string {
	if v == nil {
		return "<none>"
	}
	return *v
}

// requireU64 checks that an on-chain return value matches the expectation, panicking directly if not.
func requireU64(got, want uint64, what string) {
	if got != want {
		panic(fmt.Sprintf("unexpected %s: got %d, want %d", what, got, want))
	}
}

// newAccount generates a new account: a classical key pair whose address is derived from the Ed25519 public key.
func newAccount() (*crypto.ClassicalSecretKey, *crypto.PublicKey, *crypto.Address) {
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk := sk.Ed25519Public()
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		panic("failed to derive address from public key:" + err.Error())
	}
	return sk, pk, addr
}

// submitAndWait submits a transaction, waits for it to be committed on-chain, and returns the full receipt for event extraction.
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

// encoder abstracts the Encode method of a gen instruction-args object.
type encoder interface {
	Encode() (api.PackedInstruction, error)
}

// viewValue calls a view instruction and decodes the return value with the corresponding DecodeView.
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

// balanceOf queries an address's shares on a given TOKEN.
func balanceOf(client *milon.Client, sft *crypto.Address, tokenID uint64, owner *crypto.Address) uint64 {
	return viewValue(client, gen.Sftoken.BalanceOf.Args(sft, tokenID, owner), gen.Sftoken.BalanceOf.DecodeView)
}

// sftokenEventTypeTag looks up an event's typeTag from the bound sftoken IDL (avoiding hardcoding).
func sftokenEventTypeTag(eventName string) uint64 {
	for typeTag, event := range gen.Sftoken.Pd.EventByTypeTag {
		if event.Name == eventName {
			return typeTag
		}
	}
	panic("sftoken event not found in IDL: " + eventName)
}

// receiptEventU64 extracts a u64 field (e.g. slot_id / token_id / to_token_id) of the given event from the transaction receipt.
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

// displayToken prints a TOKEN's latest on-chain state: the inheritance-resolved metadata, plus each holder's shares and freeze status.
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
