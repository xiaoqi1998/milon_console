package main

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
)

// didNameSuffix 从账户地址派生一个 1000-9999 的后缀，
// 使 demo 可以用新的随机账户重复运行，
// 而不会与之前运行已绑定的名称冲突。
func didNameSuffix(addr *crypto.Address, word int) uint32 {
	return 1000 + uint32(binary.BigEndian.Uint16(addr.Bytes[word*2:word*2+2]))%9000
}

// demoDid 完整走一遍 alice 的个人 DID 生命周期：带 alias 创建、
// key / service / avatar 操作、alias 轮换，最后是不可逆的停用。
// 覆盖 did 的全部 entry 和全部 view。
func demoDid(client *milon.Client, alice *account) {
	fmt.Printf("\n================ 1.DID: create_with_alias ================\n")

	// 存储在初始文档中的额外签名密钥
	delegateSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	delegatePk := delegateSk.Ed25519Public()

	doc := gen.IdentityDidDocumentInput{
		SubjectType: gen.IdentityDidSubjectType{Variant: "Personal"},
		Keys: []gen.IdentityDidKeyInput{
			// key 按给定顺序自动编号 0..n
			{PublicKey: alice.pk, Label: strPtr("primary")},
			{PublicKey: delegatePk, Label: strPtr("delegate")},
		},
		Services: []gen.IdentityDidServiceInput{
			// service endpoint 必须是 scheme 为 https 或 wss 的绝对 URL
			{Label: "homepage", ServiceEndpoint: "https://alice.example.com"},
		},
		AvatarUri: "https://alice.example.com/avatar.png",
	}
	// alias 在链上会被归一化（去除空白 + unicode 转小写），
	// 因此 "Alice" 会以 "alice" 存储
	aliceName := gen.IdentityDidName{Alias: "Alice", Suffix: didNameSuffix(alice.addr, 0)}
	submit(client, alice, "CreateWithAlias", gen.Identity.CreateWithAlias.Args(alice.addr, doc, aliceName).Encode)

	// did view：core / document
	core := view(client, "Core", gen.Identity.Core.Args(alice.addr).Encode, gen.Identity.Core.DecodeView)
	if core.Subject.SubjectType.Variant != "Personal" || *core.Controller != *alice.addr {
		panic(fmt.Sprintf("unexpected DID core: %+v", core))
	}
	document := view(client, "Document", gen.Identity.Document.Args(alice.addr).Encode, gen.Identity.Document.DecodeView)
	if len(document.Keys) != 2 || len(document.Services) != 1 || document.Alias == nil || document.Alias.Alias != "alice" || document.Alias.Suffix != aliceName.Suffix || document.Deactivated {
		panic(fmt.Sprintf("unexpected DID document: %+v", document))
	}

	// did view：key index / keys / 单个 key
	keyIndex := view(client, "KeyIndex", gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if !keyIndex.Initialized || len(keyIndex.KeyIds) != 2 {
		panic(fmt.Sprintf("unexpected key index: %+v", keyIndex))
	}
	keys := view(client, "Keys", gen.Identity.Keys.Args(alice.addr).Encode, gen.Identity.Keys.DecodeView)
	if len(keys) != 2 || keys[0].Id != 0 || !bytes.Equal(keys[0].PublicKey.Bytes, alice.pk.Bytes) || !bytes.Equal(keys[1].PublicKey.Bytes, delegatePk.Bytes) {
		panic(fmt.Sprintf("unexpected keys: %+v", keys))
	}
	view(client, "Key(0)", gen.Identity.Key.Args(alice.addr, 0).Encode, gen.Identity.Key.DecodeView)

	// did view：service index / services / 单个 service
	serviceIndex := view(client, "ServiceIndex", gen.Identity.ServiceIndex.Args(alice.addr).Encode, gen.Identity.ServiceIndex.DecodeView)
	if !serviceIndex.Initialized || len(serviceIndex.ServiceIds) != 1 {
		panic(fmt.Sprintf("unexpected service index: %+v", serviceIndex))
	}
	services := view(client, "Services", gen.Identity.Services.Args(alice.addr).Encode, gen.Identity.Services.DecodeView)
	if len(services) != 1 || services[0].Label != "homepage" {
		panic(fmt.Sprintf("unexpected services: %+v", services))
	}
	view(client, "Service(0)", gen.Identity.Service.Args(alice.addr, 0).Encode, gen.Identity.Service.DecodeView)

	// did view：alias / avatar / updated_at / deactivated / name_binding
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
	// name_binding view 也会对其参数进行归一化
	binding := view(client, fmt.Sprintf("NameBinding(Alice-%d)", aliceName.Suffix), gen.Identity.NameBinding.Args(aliceName).Encode, gen.Identity.NameBinding.DecodeView)
	if *binding.Subject.Address != *alice.addr {
		panic(fmt.Sprintf("unexpected name binding: %+v", binding))
	}

	fmt.Printf("\n================ 2.DID: key lifecycle ================\n")
	sessionSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	sessionPk := sessionSk.Ed25519Public()

	// add_key 自动分配最小的空闲数字 id（这里是 2）
	submit(client, alice, "AddKey", gen.Identity.AddKey.Args(alice.addr, gen.IdentityDidKeyInput{PublicKey: sessionPk, Label: strPtr("session")}).Encode)
	keyIndex = view(client, "KeyIndex", gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if len(keyIndex.KeyIds) != 3 || keyIndex.KeyIds[2] != 2 {
		panic(fmt.Sprintf("unexpected key index after add_key: %+v", keyIndex))
	}

	// update_key 保留数字 id，替换 public key / label
	rotatedSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	rotatedPk := rotatedSk.Ed25519Public()
	submit(client, alice, "UpdateKey(2)", gen.Identity.UpdateKey.Args(alice.addr, 2, gen.IdentityDidKeyInput{PublicKey: rotatedPk, Label: strPtr("session-rotated")}).Encode)
	key2 := view(client, "Key(2)", gen.Identity.Key.Args(alice.addr, 2).Encode, gen.Identity.Key.DecodeView)
	if !bytes.Equal(key2.PublicKey.Bytes, rotatedPk.Bytes) || key2.Label == nil || *key2.Label != "session-rotated" {
		panic(fmt.Sprintf("unexpected key 2 after update: %+v", key2))
	}

	// remove_key 释放该 id；下一次 add_key 会复用最小空闲 id
	submit(client, alice, "RemoveKey(2)", gen.Identity.RemoveKey.Args(alice.addr, 2).Encode)
	keyIndex = view(client, "KeyIndex", gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if len(keyIndex.KeyIds) != 2 {
		panic(fmt.Sprintf("unexpected key index after remove_key: %+v", keyIndex))
	}
	submit(client, alice, "AddKey(reuse id 2)", gen.Identity.AddKey.Args(alice.addr, gen.IdentityDidKeyInput{PublicKey: sessionPk, Label: strPtr("replacement")}).Encode)
	keyIndex = view(client, "KeyIndex", gen.Identity.KeyIndex.Args(alice.addr).Encode, gen.Identity.KeyIndex.DecodeView)
	if len(keyIndex.KeyIds) != 3 || keyIndex.KeyIds[2] != 2 {
		panic(fmt.Sprintf("expected key id 2 to be reused: %+v", keyIndex))
	}

	fmt.Printf("\n================ 3.DID: service lifecycle ================\n")
	submit(client, alice, "AddService", gen.Identity.AddService.Args(alice.addr, gen.IdentityDidServiceInput{Label: "messaging", ServiceEndpoint: "https://msg.alice.example.com"}).Encode)
	serviceIndex = view(client, "ServiceIndex", gen.Identity.ServiceIndex.Args(alice.addr).Encode, gen.Identity.ServiceIndex.DecodeView)
	if len(serviceIndex.ServiceIds) != 2 || serviceIndex.ServiceIds[1] != 1 {
		panic(fmt.Sprintf("unexpected service index after add_service: %+v", serviceIndex))
	}
	submit(client, alice, "UpdateService(1)", gen.Identity.UpdateService.Args(alice.addr, 1, gen.IdentityDidServiceInput{Label: "chat", ServiceEndpoint: "wss://chat.alice.example.com"}).Encode)
	service1 := view(client, "Service(1)", gen.Identity.Service.Args(alice.addr, 1).Encode, gen.Identity.Service.DecodeView)
	if service1.Label != "chat" || service1.ServiceEndpoint != "wss://chat.alice.example.com" {
		panic(fmt.Sprintf("unexpected service 1 after update: %+v", service1))
	}
	submit(client, alice, "RemoveService(1)", gen.Identity.RemoveService.Args(alice.addr, 1).Encode)
	serviceIndex = view(client, "ServiceIndex", gen.Identity.ServiceIndex.Args(alice.addr).Encode, gen.Identity.ServiceIndex.DecodeView)
	if len(serviceIndex.ServiceIds) != 1 {
		panic(fmt.Sprintf("unexpected service index after remove_service: %+v", serviceIndex))
	}

	fmt.Printf("\n================ 4.DID: avatar ================\n")
	submit(client, alice, "SetAvatarUri", gen.Identity.SetAvatarUri.Args(alice.addr, "https://cdn.example.com/alice.png").Encode)
	avatarState = view(client, "Avatar", gen.Identity.Avatar.Args(alice.addr).Encode, gen.Identity.Avatar.DecodeView)
	if avatarState.AvatarUri != "https://cdn.example.com/alice.png" {
		panic(fmt.Sprintf("unexpected avatar after update: %+v", avatarState))
	}

	fmt.Printf("\n================ 5.DID: rotate alias ================\n")
	aliciaName := gen.IdentityDidName{Alias: "Alicia", Suffix: didNameSuffix(alice.addr, 1)}
	submit(client, alice, "SetAlias", gen.Identity.SetAlias.Args(alice.addr, aliciaName).Encode)
	aliasState = view(client, "Alias", gen.Identity.Alias.Args(alice.addr).Encode, gen.Identity.Alias.DecodeView)
	if aliasState.Alias == nil || aliasState.Alias.Alias != "alicia" || aliasState.Alias.Suffix != aliciaName.Suffix {
		panic(fmt.Sprintf("unexpected alias state after rotate: %+v", aliasState))
	}
	view(client, fmt.Sprintf("NameBinding(alicia-%d)", aliciaName.Suffix), gen.Identity.NameBinding.Args(aliciaName).Encode, gen.Identity.NameBinding.DecodeView)
	// 更改 alias 时旧的名称绑定会被释放
	expectViewFail(client, fmt.Sprintf("NameBinding(alice-%d) after rotate", aliceName.Suffix),
		gen.Identity.NameBinding.Args(aliceName).Encode,
		gen.Identity.NameBinding.DecodeView)

	fmt.Printf("\n================ 6.DID: deactivate (irreversible) ================\n")
	submit(client, alice, "Deactivate", gen.Identity.Deactivate.Args(alice.addr).Encode)
	deactivated = view(client, "Deactivated", gen.Identity.Deactivated.Args(alice.addr).Encode, gen.Identity.Deactivated.DecodeView)
	if !deactivated {
		panic("DID should be deactivated")
	}
	// 现在所有改变状态的 entry 都会报 DidDeactivated (1026)；读取仍然可用
	expectOnChainError(client, alice, "SetAvatarUri after deactivate", gen.Identity.SetAvatarUri.Args(alice.addr, "https://cdn.example.com/alice2.png").Encode)
}
