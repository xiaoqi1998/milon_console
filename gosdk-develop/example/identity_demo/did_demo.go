package main

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"
)

// didNameSuffix derives a 1000-9999 suffix from the account address so that the demo
// can be rerun with fresh random accounts without colliding with names bound by
// previous runs.
func didNameSuffix(addr *crypto.Address, word int) uint32 {
	return 1000 + uint32(binary.BigEndian.Uint16(addr.Bytes[word*2:word*2+2]))%9000
}

// demoDid walks the full personal DID lifecycle for alice: creation with an alias,
// key / service / avatar operations, alias rotation, and finally the irreversible
// deactivation. It covers every did entry and every did view.
func demoDid(client *milon.Client, alice *account) {
	fmt.Printf("\n================ 1.DID: create_with_alias ================\n")

	// extra signing key stored in the initial document
	delegateSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	delegatePk := delegateSk.Ed25519Public()

	doc := gen.IdentityDidDocumentInput{
		SubjectType: gen.IdentityDidSubjectType{Variant: "Personal"},
		Keys: []gen.IdentityDidKeyInput{
			// keys are numbered 0..n automatically in the order given
			{PublicKey: alice.pk, Label: strPtr("primary")},
			{PublicKey: delegatePk, Label: strPtr("delegate")},
		},
		Services: []gen.IdentityDidServiceInput{
			// a service endpoint must be an absolute URL with an https or wss scheme
			{Label: "homepage", ServiceEndpoint: "https://alice.example.com"},
		},
		AvatarUri: "https://alice.example.com/avatar.png",
	}
	// aliases are normalized on chain (whitespace stripped + unicode lowercased),
	// so "Alice" is stored as "alice"
	aliceName := gen.IdentityDidName{Alias: "Alice", Suffix: didNameSuffix(alice.addr, 0)}
	wire, err := gen.Identity.CreateWithAlias.Args(alice.addr, doc, aliceName).Encode()
	if err != nil {
		panic("failed to encode CreateWithAlias: " + err.Error())
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign CreateWithAlias: " + err.Error())
	}
	txHistory, err := client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (CreateWithAlias): " + err.Error())
	}
	fmt.Printf("  [tx] CreateWithAlias ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)

	// did views: core / document
	core, err := milon.ViewValue(client, gen.Identity.Core.Args(alice.addr).Encode, gen.Identity.Core.DecodeView)
	if err != nil {
		panic("failed to view Core: " + err.Error())
	}
	fmt.Printf("  [view] Core => %+v\n", core)
	if core.Subject.SubjectType.Variant != "Personal" || *core.Controller != *alice.addr {
		panic(fmt.Sprintf("unexpected DID core: %+v", core))
	}
	document, err := milon.ViewValue(client, gen.Identity.Document.Args(alice.addr).Encode, gen.Identity.Document.DecodeView)
	if err != nil {
		panic("failed to view Document: " + err.Error())
	}
	fmt.Printf("  [view] Document => %+v\n", document)
	if len(document.Keys) != 2 || len(document.Services) != 1 || document.Alias == nil || document.Alias.Alias != "alice" || document.Alias.Suffix != aliceName.Suffix || document.Deactivated {
		panic(fmt.Sprintf("unexpected DID document: %+v", document))
	}

	// did views: key index / keys / single key
	keyIndex, err := milon.ViewValue(client, gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if err != nil {
		panic("failed to view KeyIndex: " + err.Error())
	}
	fmt.Printf("  [view] KeyIndex => %+v\n", keyIndex)
	if !keyIndex.Initialized || len(keyIndex.KeyIds) != 2 {
		panic(fmt.Sprintf("unexpected key index: %+v", keyIndex))
	}
	keys, err := milon.ViewValue(client, gen.Identity.Keys.Args(alice.addr).Encode, gen.Identity.Keys.DecodeView)
	if err != nil {
		panic("failed to view Keys: " + err.Error())
	}
	fmt.Printf("  [view] Keys => %+v\n", keys)
	if len(keys) != 2 || keys[0].Id != 0 || !bytes.Equal(keys[0].PublicKey.Bytes, alice.pk.Bytes) || !bytes.Equal(keys[1].PublicKey.Bytes, delegatePk.Bytes) {
		panic(fmt.Sprintf("unexpected keys: %+v", keys))
	}
	key0, err := milon.ViewValue(client, gen.Identity.Key.Args(alice.addr, 0).Encode, gen.Identity.Key.DecodeView)
	if err != nil {
		panic("failed to view Key(0): " + err.Error())
	}
	fmt.Printf("  [view] Key(0) => %+v\n", key0)

	// did views: service index / services / single service
	serviceIndex, err := milon.ViewValue(client, gen.Identity.ServiceIndex.Args(alice.addr).Encode, gen.Identity.ServiceIndex.DecodeView)
	if err != nil {
		panic("failed to view ServiceIndex: " + err.Error())
	}
	fmt.Printf("  [view] ServiceIndex => %+v\n", serviceIndex)
	if !serviceIndex.Initialized || len(serviceIndex.ServiceIds) != 1 {
		panic(fmt.Sprintf("unexpected service index: %+v", serviceIndex))
	}
	services, err := milon.ViewValue(client, gen.Identity.Services.Args(alice.addr).Encode, gen.Identity.Services.DecodeView)
	if err != nil {
		panic("failed to view Services: " + err.Error())
	}
	fmt.Printf("  [view] Services => %+v\n", services)
	if len(services) != 1 || services[0].Label != "homepage" {
		panic(fmt.Sprintf("unexpected services: %+v", services))
	}
	service0, err := milon.ViewValue(client, gen.Identity.Service.Args(alice.addr, 0).Encode, gen.Identity.Service.DecodeView)
	if err != nil {
		panic("failed to view Service(0): " + err.Error())
	}
	fmt.Printf("  [view] Service(0) => %+v\n", service0)

	// did views: alias / avatar / updated_at / deactivated / name_binding
	aliasState, err := milon.ViewValue(client, gen.Identity.Alias.Args(alice.addr).Encode, gen.Identity.Alias.DecodeView)
	if err != nil {
		panic("failed to view Alias: " + err.Error())
	}
	fmt.Printf("  [view] Alias => %+v\n", aliasState)
	if !aliasState.Initialized || aliasState.Alias == nil || aliasState.Alias.Alias != "alice" || aliasState.Alias.Suffix != aliceName.Suffix {
		panic(fmt.Sprintf("unexpected alias state: %+v", aliasState))
	}
	avatarState, err := milon.ViewValue(client, gen.Identity.Avatar.Args(alice.addr).Encode, gen.Identity.Avatar.DecodeView)
	if err != nil {
		panic("failed to view Avatar: " + err.Error())
	}
	fmt.Printf("  [view] Avatar => %+v\n", avatarState)
	if !avatarState.Initialized || avatarState.AvatarUri == "" {
		panic(fmt.Sprintf("unexpected avatar state: %+v", avatarState))
	}
	updatedAt, err := milon.ViewValue(client, gen.Identity.UpdatedAt.Args(alice.addr).Encode, gen.Identity.UpdatedAt.DecodeView)
	if err != nil {
		panic("failed to view UpdatedAt: " + err.Error())
	}
	fmt.Printf("  [view] UpdatedAt => %+v\n", updatedAt)
	deactivated, err := milon.ViewValue(client, gen.Identity.Deactivated.Args(alice.addr).Encode, gen.Identity.Deactivated.DecodeView)
	if err != nil {
		panic("failed to view Deactivated: " + err.Error())
	}
	fmt.Printf("  [view] Deactivated => %+v\n", deactivated)
	if deactivated {
		panic("DID should not be deactivated yet")
	}
	// the name_binding view normalizes its arguments as well
	binding, err := milon.ViewValue(client, gen.Identity.NameBinding.Args(aliceName).Encode, gen.Identity.NameBinding.DecodeView)
	if err != nil {
		panic(fmt.Sprintf("failed to view NameBinding(Alice-%d): %v", aliceName.Suffix, err))
	}
	fmt.Printf("  [view] NameBinding(Alice-%d) => %+v\n", aliceName.Suffix, binding)
	if *binding.Subject.Address != *alice.addr {
		panic(fmt.Sprintf("unexpected name binding: %+v", binding))
	}

	fmt.Printf("\n================ 2.DID: key lifecycle ================\n")
	sessionSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	sessionPk := sessionSk.Ed25519Public()

	// add_key automatically assigns the lowest free numeric id (2 here)
	wire, err = gen.Identity.AddKey.Args(alice.addr, gen.IdentityDidKeyInput{PublicKey: sessionPk, Label: strPtr("session")}).Encode()
	if err != nil {
		panic("failed to encode AddKey: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign AddKey: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (AddKey(session)): " + err.Error())
	}
	fmt.Printf("  [tx] AddKey(session) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	keyIndex, err = milon.ViewValue(client, gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if err != nil {
		panic("failed to view KeyIndex: " + err.Error())
	}
	fmt.Printf("  [view] KeyIndex => %+v\n", keyIndex)
	if len(keyIndex.KeyIds) != 3 || keyIndex.KeyIds[2] != 2 {
		panic(fmt.Sprintf("unexpected key index after add_key: %+v", keyIndex))
	}

	// update_key keeps the numeric id and replaces the public key / label
	rotatedSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	rotatedPk := rotatedSk.Ed25519Public()
	wire, err = gen.Identity.UpdateKey.Args(alice.addr, 2, gen.IdentityDidKeyInput{PublicKey: rotatedPk, Label: strPtr("session-rotated")}).Encode()
	if err != nil {
		panic("failed to encode UpdateKey: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign UpdateKey: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (UpdateKey(2)): " + err.Error())
	}
	fmt.Printf("  [tx] UpdateKey(2) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	key2, err := milon.ViewValue(client, gen.Identity.Key.Args(alice.addr, 2).Encode, gen.Identity.Key.DecodeView)
	if err != nil {
		panic("failed to view Key(2): " + err.Error())
	}
	fmt.Printf("  [view] Key(2) => %+v\n", key2)
	if !bytes.Equal(key2.PublicKey.Bytes, rotatedPk.Bytes) || key2.Label == nil || *key2.Label != "session-rotated" {
		panic(fmt.Sprintf("unexpected key 2 after update: %+v", key2))
	}

	// remove_key frees the id; the next add_key reuses the lowest free id
	wire, err = gen.Identity.RemoveKey.Args(alice.addr, 2).Encode()
	if err != nil {
		panic("failed to encode RemoveKey: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign RemoveKey: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (RemoveKey(2)): " + err.Error())
	}
	fmt.Printf("  [tx] RemoveKey(2) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	keyIndex, err = milon.ViewValue(client, gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if err != nil {
		panic("failed to view KeyIndex: " + err.Error())
	}
	fmt.Printf("  [view] KeyIndex => %+v\n", keyIndex)
	if len(keyIndex.KeyIds) != 2 {
		panic(fmt.Sprintf("unexpected key index after remove_key: %+v", keyIndex))
	}

	wire, err = gen.Identity.AddKey.Args(alice.addr, gen.IdentityDidKeyInput{PublicKey: sessionPk, Label: strPtr("replacement")}).Encode()
	if err != nil {
		panic("failed to encode AddKey: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign AddKey: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (AddKey(replacement)): " + err.Error())
	}
	fmt.Printf("  [tx] AddKey(replacement) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	keyIndex, err = milon.ViewValue(client, gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if err != nil {
		panic("failed to view KeyIndex: " + err.Error())
	}
	fmt.Printf("  [view] KeyIndex => %+v\n", keyIndex)
	if len(keyIndex.KeyIds) != 3 || keyIndex.KeyIds[2] != 2 {
		panic(fmt.Sprintf("expected key id 2 to be reused: %+v", keyIndex))
	}

	fmt.Printf("\n================ 3.DID: service lifecycle ================\n")
	wire, err = gen.Identity.AddService.Args(alice.addr, gen.IdentityDidServiceInput{Label: "messaging", ServiceEndpoint: "https://msg.alice.example.com"}).Encode()
	if err != nil {
		panic("failed to encode AddService: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign AddService: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (AddService): " + err.Error())
	}
	fmt.Printf("  [tx] AddService ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	serviceIndex, err = milon.ViewValue(client, gen.Identity.ServiceIndex.Args(alice.addr).Encode, gen.Identity.ServiceIndex.DecodeView)
	if err != nil {
		panic("failed to view ServiceIndex: " + err.Error())
	}
	fmt.Printf("  [view] ServiceIndex => %+v\n", serviceIndex)
	if len(serviceIndex.ServiceIds) != 2 || serviceIndex.ServiceIds[1] != 1 {
		panic(fmt.Sprintf("unexpected service index after add_service: %+v", serviceIndex))
	}

	wire, err = gen.Identity.UpdateService.Args(alice.addr, 1, gen.IdentityDidServiceInput{Label: "chat", ServiceEndpoint: "wss://chat.alice.example.com"}).Encode()
	if err != nil {
		panic("failed to encode UpdateService: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign UpdateService: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (UpdateService(1)): " + err.Error())
	}
	fmt.Printf("  [tx] UpdateService(1) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	service1, err := milon.ViewValue(client, gen.Identity.Service.Args(alice.addr, 1).Encode, gen.Identity.Service.DecodeView)
	if err != nil {
		panic("failed to view Service(1): " + err.Error())
	}
	fmt.Printf("  [view] Service(1) => %+v\n", service1)
	if service1.Label != "chat" || service1.ServiceEndpoint != "wss://chat.alice.example.com" {
		panic(fmt.Sprintf("unexpected service 1 after update: %+v", service1))
	}

	wire, err = gen.Identity.RemoveService.Args(alice.addr, 1).Encode()
	if err != nil {
		panic("failed to encode RemoveService: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign RemoveService: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (RemoveService(1)): " + err.Error())
	}
	fmt.Printf("  [tx] RemoveService(1) ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	serviceIndex, err = milon.ViewValue(client, gen.Identity.ServiceIndex.Args(alice.addr).Encode, gen.Identity.ServiceIndex.DecodeView)
	if err != nil {
		panic("failed to view ServiceIndex: " + err.Error())
	}
	fmt.Printf("  [view] ServiceIndex => %+v\n", serviceIndex)
	if len(serviceIndex.ServiceIds) != 1 {
		panic(fmt.Sprintf("unexpected service index after remove_service: %+v", serviceIndex))
	}

	fmt.Printf("\n================ 4.DID: avatar ================\n")
	wire, err = gen.Identity.SetAvatarUri.Args(alice.addr, "https://cdn.example.com/alice.png").Encode()
	if err != nil {
		panic("failed to encode SetAvatarUri: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign SetAvatarUri: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (SetAvatarUri): " + err.Error())
	}
	fmt.Printf("  [tx] SetAvatarUri ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	avatarState, err = milon.ViewValue(client, gen.Identity.Avatar.Args(alice.addr).Encode, gen.Identity.Avatar.DecodeView)
	if err != nil {
		panic("failed to view Avatar: " + err.Error())
	}
	fmt.Printf("  [view] Avatar => %+v\n", avatarState)
	if avatarState.AvatarUri != "https://cdn.example.com/alice.png" {
		panic(fmt.Sprintf("unexpected avatar after update: %+v", avatarState))
	}

	fmt.Printf("\n================ 5.DID: rotate alias ================\n")
	aliciaName := gen.IdentityDidName{Alias: "Alicia", Suffix: didNameSuffix(alice.addr, 1)}
	wire, err = gen.Identity.SetAlias.Args(alice.addr, aliciaName).Encode()
	if err != nil {
		panic("failed to encode SetAlias: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign SetAlias: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (SetAlias): " + err.Error())
	}
	fmt.Printf("  [tx] SetAlias ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	aliasState, err = milon.ViewValue(client, gen.Identity.Alias.Args(alice.addr).Encode, gen.Identity.Alias.DecodeView)
	if err != nil {
		panic("failed to view Alias: " + err.Error())
	}
	fmt.Printf("  [view] Alias => %+v\n", aliasState)
	if aliasState.Alias == nil || aliasState.Alias.Alias != "alicia" || aliasState.Alias.Suffix != aliciaName.Suffix {
		panic(fmt.Sprintf("unexpected alias state after rotate: %+v", aliasState))
	}
	aliciaBinding, err := milon.ViewValue(client, gen.Identity.NameBinding.Args(aliciaName).Encode, gen.Identity.NameBinding.DecodeView)
	if err != nil {
		panic(fmt.Sprintf("failed to view NameBinding(alicia-%d): %v", aliciaName.Suffix, err))
	}
	fmt.Printf("  [view] NameBinding(alicia-%d) => %+v\n", aliciaName.Suffix, aliciaBinding)
	// rotating the alias releases the previous name binding
	expectViewFail(client, fmt.Sprintf("NameBinding(alice-%d) after rotate", aliceName.Suffix),
		gen.Identity.NameBinding.Args(aliceName).Encode,
		gen.Identity.NameBinding.DecodeView)

	fmt.Printf("\n================ 6.DID: deactivate (irreversible) ================\n")
	wire, err = gen.Identity.Deactivate.Args(alice.addr).Encode()
	if err != nil {
		panic("failed to encode Deactivate: " + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(alice.addr).
		AddIxesSig(*alice.addr, alice.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *alice.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign Deactivate: " + err.Error())
	}
	txHistory, err = client.SubmitAndWait(tx)
	if err != nil {
		panic("failed to submit and wait (Deactivate): " + err.Error())
	}
	fmt.Printf("  [tx] Deactivate ok, hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
	deactivated, err = milon.ViewValue(client, gen.Identity.Deactivated.Args(alice.addr).Encode, gen.Identity.Deactivated.DecodeView)
	if err != nil {
		panic("failed to view Deactivated: " + err.Error())
	}
	fmt.Printf("  [view] Deactivated => %+v\n", deactivated)
	if !deactivated {
		panic("DID should be deactivated")
	}

	// every state-changing entry now reports DidDeactivated; reads still work
	expectOnChainError(client, alice, "SetAvatarUri after deactivate", gen.Identity.SetAvatarUri.Args(alice.addr, "https://cdn.example.com/alice2.png").Encode)
}
