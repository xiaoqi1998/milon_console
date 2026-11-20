package main

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"
)

func main() {
	example(milon.DevNet)
}

// example demonstrates the full lifecycle of the sftoken contract:
// create_sft -> create_slot -> mint -> split -> transfer (partial/full) -> approve -> transfer_from
// -> freeze/unfreeze -> merge
//
// Ownership model: TOKEN shares are accounted independently by (token_id, owner); the same token_id can be held by multiple addresses at once.
//   - transfer / transfer_from only move shares, token_id stays unchanged;
//   - split detaches shares from the source TOKEN and derives a new token_id in the same SLOT (the split amount must be strictly less than the signer's shares);
//   - merge folds all of the signer's shares in the source TOKEN into the target TOKEN of the same SLOT;
//   - three-level metadata inheritance: SFT metadata <- SLOT override <- TOKEN override; view returns the resolved result.
func example(networkConfig milon.Network) {
	client := milon.MustNewClient(networkConfig)

	// Account roles:
	//   sft   : SFT resource account, only the instruction signer for create_sft (gas paid by owner, no faucet needed)
	//   owner : SFT owner; also the default mint authority / update author / freeze authority when not transferred
	//   alice : split recipient / full-transfer initiator / spender (authorized to move shares)
	//   bob   : partial-transfer recipient / approve granter / merge initiator
	sftSk, sftPk, sft := newAccount()
	ownerSk, ownerPk, owner := newAccount()
	aliceSk, alicePk, alice := newAccount()
	bobSk, bobPk, bob := newAccount()

	fmt.Printf("sft = %v \n", sft)
	fmt.Printf("owner = %v \n", owner)
	fmt.Printf("alice = %v \n", alice)
	fmt.Printf("bob = %v \n\n", bob)

	// Holder list for share queries (the same TOKEN may be held by multiple people at once).
	holders := []holder{{"owner", owner}, {"alice", alice}, {"bob", bob}}

	fmt.Printf("\n================ 1.ClaimFaucet ================\n")
	// sft is only the instruction signer for create_sft; gas is paid by owner, so no MIL needs to be claimed.
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
	// create_sft is signed by the sft resource account (owner set to the owner account), gas paid by owner:
	// sft signs only ix0, owner signs bit63 (gas) as payer.
	// metadata is a struct: name(1..=128) / symbol(1..=32) are required and immutable after creation,
	// cover_url / metadata are URIs, attribute is optional; royalty_bps is the secondary-market royalty in basis points,
	// and the royalty recipient is initially owner.
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
	txHistory, err := client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	fmt.Printf("\n================ 3.CreateSlot(owner sign, is_transferable=true) ================\n")
	// a slot can only be created by the SFT owner; SlotData.metadata is an optional MetadataOverride,
	// fields not provided (or empty strings) dynamically inherit the SFT metadata.
	// mint / update / freeze authorities are no longer specified in create_slot; they default to the SFT owner,
	// and can be transferred later via transfer_mint_authority / transfer_update_author / transfer_freeze_authority when needed.
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
	// slot_id is assigned by a global on-chain increment (starting from 1); extract it from the receipt event to avoid hardcoding.
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	slotID := receiptEventU64(client, txHistory, "SlotCreatedEvent", "slot_id")
	fmt.Printf("slot created: slot_id = %d\n", slotID)
	// A new SFT's slot_id is assigned incrementally from 1, so the first slot must be 1.
	requireU64(slotID, 1, "slot_id")

	fmt.Printf("\n================ 4.Mint 100 to owner(owner sign) ================\n")
	// mint requires the mint authority's signature (the SFT owner when not transferred); amount must be greater than 0;
	// metadata is a TOKEN-level override, fully inheriting the SLOT when nil. Shares are recorded directly on (token_id, to).
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
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	token1 := receiptEventU64(client, txHistory, "TokenMintedEvent", "token_id")
	fmt.Printf("minted: token_id = %d, owner holds 100\n", token1)
	// A new SFT's token_id is assigned incrementally from 1, so the first minted TOKEN must be 1.
	requireU64(token1, 1, "minted token_id")
	requireU64(balanceOf(client, sft, token1, owner), 100, "owner balance after mint")
	displayToken(client, sft, token1, holders...)

	fmt.Printf("\n================ 5.Split: owner splits 40 of token %d to alice(owner sign) ================\n", token1)
	// split is signed by the share holder: detach amount from the source TOKEN and create a new TOKEN in the same SLOT for to (to can be oneself).
	// Constraints: amount must be strictly less than the signer's current shares (use transfer for a full transfer-out);
	// when to is not oneself, the slot must also have is_transferable=true; the new TOKEN inherits the source TOKEN's metadata override.
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
	// split reuses TokenTransferredEvent: from_token_id is the source TOKEN, to_token_id is the newly derived TOKEN.
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	token2 := receiptEventU64(client, txHistory, "TokenTransferredEvent", "to_token_id")
	fmt.Printf("split: token %d (100) -> new token %d (40 for alice), owner keeps 60 on token %d\n", token1, token2, token1)
	// The new TOKEN derived by split gets the next token_id.
	requireU64(token2, 2, "split to_token_id")
	requireU64(balanceOf(client, sft, token1, owner), 60, "owner balance on token1 after split")
	requireU64(balanceOf(client, sft, token2, alice), 40, "alice balance on token2 after split")

	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 6.Transfer part: owner transfers 20 of token %d to bob(owner sign) ================\n", token1)
	// transfer only moves shares: token_id is unchanged, the sender decreases and the receiver increases, so the same TOKEN is co-held by multiple people.
	// Requires slot is_transferable=true, sufficient and unfrozen shares, and from != to.
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
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	requireU64(receiptEventU64(client, txHistory, "TokenTransferredEvent", "amount"), 20, "transferred amount")
	// A share transfer does not derive a new TOKEN: in the event, to_token_id equals from_token_id.
	requireU64(receiptEventU64(client, txHistory, "TokenTransferredEvent", "to_token_id"), token1, "transfer to_token_id")
	fmt.Printf("part transfer: token %d -> owner keeps 40, bob holds 20 (same token_id)\n", token1)
	requireU64(balanceOf(client, sft, token1, owner), 40, "owner balance on token1 after part transfer")
	requireU64(balanceOf(client, sft, token1, bob), 20, "bob balance on token1 after part transfer")
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 7.Transfer all: alice transfers 40 of token %d to bob(alice sign) ================\n", token2)
	// After a full transfer-out, the sender's share record on that TOKEN is deleted; only bob holds token2, and the token_id still stays unchanged.
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
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	fmt.Printf("full transfer: token %d -> bob holds 40, alice holds 0\n", token2)
	requireU64(balanceOf(client, sft, token2, alice), 0, "alice balance on token2 after full transfer")
	requireU64(balanceOf(client, sft, token2, bob), 40, "bob balance on token2 after full transfer")
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 8.Approve: bob approves alice 30 on token %d(bob sign) ================\n", token2)
	// approve is signed by the share holder; the allowance is recorded on (token_id, owner, spender),
	// so different holders of the same TOKEN can each authorize a different spender.
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
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	approval, err := milon.ViewValue(client, gen.Sftoken.ApprovalOf.Args(sft, token2, bob, alice).Encode, gen.Sftoken.ApprovalOf.DecodeView)
	if err != nil {
		panic("failed to view ApprovalOf:" + err.Error())
	}
	// After approving 30, the allowance should be 30.
	requireU64(approval, 30, "approval after approve")
	fmt.Printf("alice approval on token %d (owner=bob): %d\n", token2, approval)

	fmt.Printf("\n================ 9.TransferFrom: alice spends 30 of token %d from bob to owner(alice sign, bob pays gas) ================\n", token2)
	// transfer_from is signed by the spender and requires a sufficient allowance on (token_id, from, spender);
	// it follows the transfer path (share movement, token_id unchanged, requires is_transferable=true) and deducts the allowance on success.
	// Split-pay transaction: spender (alice) signs the instruction, payer (bob) pays gas.
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
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	fmt.Printf("transfer_from: token %d -> bob keeps 10, owner holds 30 (same token_id)\n", token2)
	requireU64(balanceOf(client, sft, token2, bob), 10, "bob balance on token2 after transfer_from")
	requireU64(balanceOf(client, sft, token2, owner), 30, "owner balance on token2 after transfer_from")

	approval, err = milon.ViewValue(client, gen.Sftoken.ApprovalOf.Args(sft, token2, bob, alice).Encode, gen.Sftoken.ApprovalOf.DecodeView)
	if err != nil {
		panic("failed to view ApprovalOf:" + err.Error())
	}
	// After spending 30, the allowance drops to zero (the record is deleted once the allowance is exhausted).
	requireU64(approval, 0, "approval after transfer_from")
	fmt.Printf("alice approval on token %d after spend: %d\n", token2, approval)
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 10.Freeze/Unfreeze: owner freezes bob on token %d(owner sign) ================\n", token2)
	// freeze / unfreeze are signed by the freeze authority (the SFT owner when not transferred);
	// the freeze granularity is (token_id, owner): frozen shares cannot be used in transfer / transfer_from / split / merge / burn.
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
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	frozen, err := milon.ViewValue(client, gen.Sftoken.FrozenOf.Args(sft, token2, bob).Encode, gen.Sftoken.FrozenOf.DecodeView)
	if err != nil {
		panic("failed to view FrozenOf:" + err.Error())
	}
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
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	frozen, err = milon.ViewValue(client, gen.Sftoken.FrozenOf.Args(sft, token2, bob).Encode, gen.Sftoken.FrozenOf.DecodeView)
	if err != nil {
		panic("failed to view FrozenOf:" + err.Error())
	}
	requireBool(frozen, false, "bob frozen on token2 after unfreeze")
	fmt.Printf("bob frozen on token %d after unfreeze: %v\n", token2, frozen)

	fmt.Printf("\n================ 11.Merge: bob merges token %d (20) into token %d(bob sign) ================\n", token1, token2)
	// merge folds all of the signer's shares in the source TOKEN into the target TOKEN: both TOKENs must belong to the same SLOT,
	// and the signer's shares in the source TOKEN must be greater than 0; after merging, that holder's share record on the source TOKEN is deleted (the TOKEN itself still exists,
	// and other holders are unaffected).
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
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait:" + err.Error())
	}
	fmt.Printf("submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	mergedAmount := receiptEventU64(client, txHistory, "TokenTransferredEvent", "amount")
	// All 20 of bob's shares on token1 are folded into token2.
	requireU64(mergedAmount, 20, "merged amount")
	requireU64(receiptEventU64(client, txHistory, "TokenTransferredEvent", "to_token_id"), token2, "merge to_token_id")
	fmt.Printf("merge: %d moved from token %d into token %d, bob's balance on token %d cleared\n",
		mergedAmount, token1, token2, token1)
	requireU64(balanceOf(client, sft, token1, bob), 0, "bob balance on token1 after merge")
	requireU64(balanceOf(client, sft, token2, bob), 30, "bob balance on token2 after merge")
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)

	fmt.Printf("\n================ 12.Final states ================\n")
	sftOwner, err := milon.ViewValue(client, gen.Sftoken.SftOwnerOf.Args(sft).Encode, gen.Sftoken.SftOwnerOf.DecodeView)
	if err != nil {
		panic("failed to view SftOwnerOf:" + err.Error())
	}
	sftMeta, err := milon.ViewValue(client, gen.Sftoken.SftMetadata.Args(sft).Encode, gen.Sftoken.SftMetadata.DecodeView)
	if err != nil {
		panic("failed to view SftMetadata:" + err.Error())
	}
	fmt.Printf("sft: owner=%v name=%q symbol=%q cover_url=%q uri=%q attribute=%s\n",
		sftOwner, sftMeta.Name, sftMeta.Symbol, sftMeta.CoverUrl, sftMeta.Metadata, optStr(sftMeta.Attribute))

	royalty, err := milon.ViewValue(client, gen.Sftoken.RoyaltyInfo.Args(sft).Encode, gen.Sftoken.RoyaltyInfo.DecodeView)
	if err != nil {
		panic("failed to view RoyaltyInfo:" + err.Error())
	}
	fmt.Printf("royalty: recipient=%v bps=%d\n", royalty.Recipient, royalty.Bps)

	// When not transferred, all three authorities fall back to the SFT owner.
	mintAuthority, err := milon.ViewValue(client, gen.Sftoken.MintAuthority.Args(sft).Encode, gen.Sftoken.MintAuthority.DecodeView)
	if err != nil {
		panic("failed to view MintAuthority:" + err.Error())
	}
	updateAuthor, err := milon.ViewValue(client, gen.Sftoken.UpdateAuthor.Args(sft).Encode, gen.Sftoken.UpdateAuthor.DecodeView)
	if err != nil {
		panic("failed to view UpdateAuthor:" + err.Error())
	}
	freezeAuthority, err := milon.ViewValue(client, gen.Sftoken.FreezeAuthority.Args(sft).Encode, gen.Sftoken.FreezeAuthority.DecodeView)
	if err != nil {
		panic("failed to view FreezeAuthority:" + err.Error())
	}
	fmt.Printf("authorities: mint=%v update=%v freeze=%v\n", mintAuthority, updateAuthor, freezeAuthority)

	transferable, err := milon.ViewValue(client, gen.Sftoken.IsSlotTransferable.Args(sft, slotID).Encode, gen.Sftoken.IsSlotTransferable.DecodeView)
	if err != nil {
		panic("failed to view IsSlotTransferable:" + err.Error())
	}
	slotInfo, err := milon.ViewValue(client, gen.Sftoken.SlotInfo.Args(sft, slotID).Encode, gen.Sftoken.SlotInfo.DecodeView)
	if err != nil {
		panic("failed to view SlotInfo:" + err.Error())
	}
	slotMeta, err := milon.ViewValue(client, gen.Sftoken.SlotMetadata.Args(sft, slotID).Encode, gen.Sftoken.SlotMetadata.DecodeView)
	if err != nil {
		panic("failed to view SlotMetadata:" + err.Error())
	}
	fmt.Printf("slot %d: is_transferable=%v override=%v\n", slotID, transferable, overrideStr(slotInfo.Metadata))
	fmt.Printf("slot %d resolved metadata: name=%q symbol=%q attribute=%s\n",
		slotID, slotMeta.Name, slotMeta.Symbol, optStr(slotMeta.Attribute))

	approval, err = milon.ViewValue(client, gen.Sftoken.ApprovalOf.Args(sft, token2, bob, alice).Encode, gen.Sftoken.ApprovalOf.DecodeView)
	if err != nil {
		panic("failed to view ApprovalOf:" + err.Error())
	}
	fmt.Printf("alice approval on token %d (owner=bob): %d\n", token2, approval)

	// Final shares: token1 has only owner with 40; token2 is co-held by bob with 30 and owner with 30.
	fmt.Printf("final holders:\n")
	displayToken(client, sft, token1, holders...)
	displayToken(client, sft, token2, holders...)
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

// overrideStr prints a metadata override; nil means fully inheriting the level above.
func overrideStr(override *gen.SftokenMetadataOverride) string {
	if override == nil {
		return "<none, inherit>"
	}
	return fmt.Sprintf("name=%s symbol=%s attribute=%s",
		optStr(override.Name), optStr(override.Symbol), optStr(override.Attribute))
}

// requireU64 checks that an on-chain return value matches the expectation, panicking directly if not.
func requireU64(got, want uint64, what string) {
	if got != want {
		panic(fmt.Sprintf("unexpected %s: got %d, want %d", what, got, want))
	}
}

// requireBool checks that an on-chain returned boolean matches the expectation, panicking directly if not.
func requireBool(got, want bool, what string) {
	if got != want {
		panic(fmt.Sprintf("unexpected %s: got %v, want %v", what, got, want))
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

// balanceOf queries an address's shares on a given TOKEN.
func balanceOf(client *milon.Client, sft *crypto.Address, tokenID uint64, owner *crypto.Address) uint64 {
	balance, err := milon.ViewValue(client, gen.Sftoken.BalanceOf.Args(sft, tokenID, owner).Encode, gen.Sftoken.BalanceOf.DecodeView)
	if err != nil {
		panic("failed to view BalanceOf:" + err.Error())
	}
	return balance
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

// receiptEventU64 extracts a u64 field (e.g. slot_id / token_id / to_token_id / amount) of the given event from the transaction receipt.
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
	slotID, err := milon.ViewValue(client, gen.Sftoken.SlotOf.Args(sft, tokenID).Encode, gen.Sftoken.SlotOf.DecodeView)
	if err != nil {
		panic("failed to view SlotOf:" + err.Error())
	}
	meta, err := milon.ViewValue(client, gen.Sftoken.Token.Args(sft, tokenID).Encode, gen.Sftoken.Token.DecodeView)
	if err != nil {
		panic("failed to view Token:" + err.Error())
	}
	fmt.Printf("  token_id=%d slot_id=%d name=%q symbol=%q cover_url=%q attribute=%s\n", tokenID, slotID, meta.Name, meta.Symbol, meta.CoverUrl, optStr(meta.Attribute))

	for _, h := range holders {
		balance := balanceOf(client, sft, tokenID, h.addr)
		frozen, err := milon.ViewValue(client, gen.Sftoken.FrozenOf.Args(sft, tokenID, h.addr).Encode, gen.Sftoken.FrozenOf.DecodeView)
		if err != nil {
			panic("failed to view FrozenOf:" + err.Error())
		}
		fmt.Printf("    holder=%-6s addr=%v balance=%d frozen=%v\n", h.name, h.addr, balance, frozen)
	}
}
