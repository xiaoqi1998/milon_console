package main

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
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
	submit(
		client,
		alice,
		"CreateWithAlias",
		gen.Identity.CreateWithAlias.Args(alice.addr, doc, aliceName).Encode,
	)

	// did views: core / document
	core := view(client, "Core", gen.Identity.Core.Args(alice.addr).Encode, gen.Identity.Core.DecodeView)
	if core.Subject.SubjectType.Variant != "Personal" || *core.Controller != *alice.addr {
		panic(fmt.Sprintf("unexpected DID core: %+v", core))
	}
	document := view(client, "Document", gen.Identity.Document.Args(alice.addr).Encode, gen.Identity.Document.DecodeView)
	if len(document.Keys) != 2 || len(document.Services) != 1 || document.Alias == nil || document.Alias.Alias != "alice" || document.Alias.Suffix != aliceName.Suffix || document.Deactivated {
		panic(fmt.Sprintf("unexpected DID document: %+v", document))
	}

	// did views: key index / keys / single key
	keyIndex := view(client, "KeyIndex", gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if !keyIndex.Initialized || len(keyIndex.KeyIds) != 2 {
		panic(fmt.Sprintf("unexpected key index: %+v", keyIndex))
	}
	keys := view(client, "Keys", gen.Identity.Keys.Args(alice.addr).Encode, gen.Identity.Keys.DecodeView)
	if len(keys) != 2 || keys[0].Id != 0 || !bytes.Equal(keys[0].PublicKey.Bytes, alice.pk.Bytes) || !bytes.Equal(keys[1].PublicKey.Bytes, delegatePk.Bytes) {
		panic(fmt.Sprintf("unexpected keys: %+v", keys))
	}
	view(client, "Key(0)", gen.Identity.Key.Args(alice.addr, 0).Encode, gen.Identity.Key.DecodeView)

	// did views: service index / services / single service
	serviceIndex := view(client, "ServiceIndex", gen.Identity.ServiceIndex.Args(alice.addr).Encode, gen.Identity.ServiceIndex.DecodeView)
	if !serviceIndex.Initialized || len(serviceIndex.ServiceIds) != 1 {
		panic(fmt.Sprintf("unexpected service index: %+v", serviceIndex))
	}
	services := view(client, "Services", gen.Identity.Services.Args(alice.addr).Encode, gen.Identity.Services.DecodeView)
	if len(services) != 1 || services[0].Label != "homepage" {
		panic(fmt.Sprintf("unexpected services: %+v", services))
	}
	view(client, "Service(0)", gen.Identity.Service.Args(alice.addr, 0).Encode, gen.Identity.Service.DecodeView)

	// did views: alias / avatar / updated_at / deactivated / name_binding
	aliasState := view(client, "Alias", gen.Identity.Alias.Args(alice.addr).Encode, gen.Identity.Alias.DecodeView)
	if !aliasState.Initialized || aliasState.Alias == nil || aliasState.Alias.Alias != "alice" || aliasState.Alias.Suffix != aliceName.Suffix {
		panic(fmt.Sprintf("unexpected alias state: %+v", aliasState))
	}
	avatarState := view(client, "Avatar", gen.Identity.Avatar.Args(alice.addr).Encode, gen.Identity.Avatar.DecodeView)
	if !avatarState.Initialized || avatarState.AvatarUri == "" {
		panic(fmt.Sprintf("unexpected avatar state: %+v", avatarState))
	}
	view(client, "UpdatedAt", gen.Identity.UpdatedAt.Args(alice.addr).Encode, gen.Identity.UpdatedAt.DecodeView)
	deactivated := view(client, "Deactivated", gen.Identity.Deactivated.Args(alice.addr).Encode, gen.Identity.Deactivated.DecodeView)
	if deactivated {
		panic("DID should not be deactivated yet")
	}
	// the name_binding view normalizes its arguments as well
	binding := view(client, fmt.Sprintf("NameBinding(Alice-%d)", aliceName.Suffix), gen.Identity.NameBinding.Args(aliceName).Encode, gen.Identity.NameBinding.DecodeView)
	if *binding.Subject.Address != *alice.addr {
		panic(fmt.Sprintf("unexpected name binding: %+v", binding))
	}

	fmt.Printf("\n================ 2.DID: key lifecycle ================\n")
	sessionSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	sessionPk := sessionSk.Ed25519Public()

	// add_key automatically assigns the lowest free numeric id (2 here)
	submit(
		client,
		alice,
		"AddKey",
		gen.Identity.AddKey.Args(alice.addr, gen.IdentityDidKeyInput{PublicKey: sessionPk, Label: strPtr("session")}).Encode,
	)
	keyIndex = view(client, "KeyIndex", gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if len(keyIndex.KeyIds) != 3 || keyIndex.KeyIds[2] != 2 {
		panic(fmt.Sprintf("unexpected key index after add_key: %+v", keyIndex))
	}

	// update_key keeps the numeric id and replaces the public key / label
	rotatedSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	rotatedPk := rotatedSk.Ed25519Public()
	submit(
		client,
		alice,
		"UpdateKey(2)",
		gen.Identity.UpdateKey.Args(alice.addr, 2, gen.IdentityDidKeyInput{PublicKey: rotatedPk, Label: strPtr("session-rotated")}).Encode,
	)
	key2 := view(client, "Key(2)", gen.Identity.Key.Args(alice.addr, 2).Encode, gen.Identity.Key.DecodeView)
	if !bytes.Equal(key2.PublicKey.Bytes, rotatedPk.Bytes) || key2.Label == nil || *key2.Label != "session-rotated" {
		panic(fmt.Sprintf("unexpected key 2 after update: %+v", key2))
	}

	// remove_key frees the id; the next add_key reuses the lowest free id
	submit(
		client,
		alice,
		"RemoveKey(2)",
		gen.Identity.RemoveKey.Args(alice.addr, 2).Encode,
	)
	keyIndex = view(client, "KeyIndex", gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if len(keyIndex.KeyIds) != 2 {
		panic(fmt.Sprintf("unexpected key index after remove_key: %+v", keyIndex))
	}

	submit(
		client,
		alice,
		"AddKey(reuse id 2)",
		gen.Identity.AddKey.Args(alice.addr, gen.IdentityDidKeyInput{PublicKey: sessionPk, Label: strPtr("replacement")}).Encode,
	)
	keyIndex = view(client, "KeyIndex", gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if len(keyIndex.KeyIds) != 3 || keyIndex.KeyIds[2] != 2 {
		panic(fmt.Sprintf("expected key id 2 to be reused: %+v", keyIndex))
	}

	fmt.Printf("\n================ 3.DID: service lifecycle ================\n")
	submit(
		client,
		alice,
		"AddService",
		gen.Identity.AddService.Args(alice.addr, gen.IdentityDidServiceInput{Label: "messaging", ServiceEndpoint: "https://msg.alice.example.com"}).Encode,
	)
	serviceIndex = view(client, "ServiceIndex", gen.Identity.ServiceIndex.Args(alice.addr).Encode, gen.Identity.ServiceIndex.DecodeView)
	if len(serviceIndex.ServiceIds) != 2 || serviceIndex.ServiceIds[1] != 1 {
		panic(fmt.Sprintf("unexpected service index after add_service: %+v", serviceIndex))
	}

	submit(
		client,
		alice,
		"UpdateService(1)",
		gen.Identity.UpdateService.Args(alice.addr, 1, gen.IdentityDidServiceInput{Label: "chat", ServiceEndpoint: "wss://chat.alice.example.com"}).Encode,
	)
	service1 := view(client, "Service(1)", gen.Identity.Service.Args(alice.addr, 1).Encode, gen.Identity.Service.DecodeView)
	if service1.Label != "chat" || service1.ServiceEndpoint != "wss://chat.alice.example.com" {
		panic(fmt.Sprintf("unexpected service 1 after update: %+v", service1))
	}

	submit(
		client,
		alice,
		"RemoveService(1)",
		gen.Identity.RemoveService.Args(alice.addr, 1).Encode,
	)
	serviceIndex = view(client, "ServiceIndex", gen.Identity.ServiceIndex.Args(alice.addr).Encode, gen.Identity.ServiceIndex.DecodeView)
	if len(serviceIndex.ServiceIds) != 1 {
		panic(fmt.Sprintf("unexpected service index after remove_service: %+v", serviceIndex))
	}

	fmt.Printf("\n================ 4.DID: avatar ================\n")
	submit(
		client,
		alice,
		"SetAvatarUri",
		gen.Identity.SetAvatarUri.Args(alice.addr, "https://cdn.example.com/alice.png").Encode,
	)
	avatarState = view(client, "Avatar", gen.Identity.Avatar.Args(alice.addr).Encode, gen.Identity.Avatar.DecodeView)
	if avatarState.AvatarUri != "https://cdn.example.com/alice.png" {
		panic(fmt.Sprintf("unexpected avatar after update: %+v", avatarState))
	}

	fmt.Printf("\n================ 5.DID: rotate alias ================\n")
	aliciaName := gen.IdentityDidName{Alias: "Alicia", Suffix: didNameSuffix(alice.addr, 1)}
	submit(
		client,
		alice,
		"SetAlias",
		gen.Identity.SetAlias.Args(alice.addr, aliciaName).Encode,
	)
	aliasState = view(client, "Alias", gen.Identity.Alias.Args(alice.addr).Encode, gen.Identity.Alias.DecodeView)
	if aliasState.Alias == nil || aliasState.Alias.Alias != "alicia" || aliasState.Alias.Suffix != aliciaName.Suffix {
		panic(fmt.Sprintf("unexpected alias state after rotate: %+v", aliasState))
	}
	view(client, fmt.Sprintf("NameBinding(alicia-%d)", aliciaName.Suffix), gen.Identity.NameBinding.Args(aliciaName).Encode, gen.Identity.NameBinding.DecodeView)
	// rotating the alias releases the previous name binding
	expectViewFail(client, fmt.Sprintf("NameBinding(alice-%d) after rotate", aliceName.Suffix),
		gen.Identity.NameBinding.Args(aliceName).Encode,
		gen.Identity.NameBinding.DecodeView)

	fmt.Printf("\n================ 6.DID: deactivate (irreversible) ================\n")
	submit(
		client,
		alice,
		"Deactivate",
		gen.Identity.Deactivate.Args(alice.addr).Encode,
	)
	deactivated = view(client, "Deactivated", gen.Identity.Deactivated.Args(alice.addr).Encode, gen.Identity.Deactivated.DecodeView)
	if !deactivated {
		panic("DID should be deactivated")
	}

	// every state-changing entry now reports DidDeactivated; reads still work
	expectOnChainError(client, alice, "SetAvatarUri after deactivate", gen.Identity.SetAvatarUri.Args(alice.addr, "https://cdn.example.com/alice2.png").Encode)
}
