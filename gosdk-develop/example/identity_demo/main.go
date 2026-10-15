package main

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/helper"
	"github.com/milon-labs/milon-go-sdk/lib"
)

func main() {
	example(milon.DevNet)
}

// account 组合了一个签名密钥对及其派生地址。
type account struct {
	sk   *crypto.ClassicalSecretKey
	pk   *crypto.PublicKey
	addr *crypto.Address
}

func newAccount() *account {
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk := sk.Ed25519Public()
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		panic("failed to derive address from public key: " + err.Error())
	}
	return &account{sk: sk, pk: pk, addr: addr}
}

func strPtr(s string) *string { return &s }

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// alice：DID 文档演示使用的个人 DID（最后会被停用）。
	alice := newAccount()
	// kycOrg：在实体注册表中注册的机构 DID，并在合规演示中签发 VC。
	// 整个运行期间保持活跃。
	kycOrg := newAccount()
	// tempOrg：用于演示不可逆的 deactivate_organization 的机构。
	tempOrg := newAccount()
	// bob：合规演示中 VC 的 subject。
	bob := newAccount()

	fmt.Printf("alice  = %v\nkycOrg = %v\ntempOrg = %v\nbob    = %v\n", alice.addr, kycOrg.addr, tempOrg.addr, bob.addr)

	fmt.Printf("\n================ 0.Fund test accounts ================\n")
	for _, acc := range []*account{alice, kycOrg, tempOrg, bob} {
		if err := client.ClaimFaucet(acc.sk, acc.addr, lib.PubKeySignatureMode{PublicKey: *acc.pk}); err != nil {
			panic("failed to ClaimFaucet: " + err.Error())
		}
		balance, err := client.BalanceOf(acc.addr)
		if err != nil {
			panic("failed to get balance: " + err.Error())
		}
		fmt.Printf("  %v MIL: %d\n", acc.addr, balance)
	}

	demoDid(client, alice)
	demoEntityRegistry(client, kycOrg, tempOrg)
	demoCompliance(client, kycOrg, bob)

	fmt.Printf("\nidentity demo finished: did / entity_registry / compliance all covered.\n")
}

// submit 构建一笔由 signer 签名的单指令交易（同时授权该指令和 gas 支付），
// 提交后等待回执不再处于 pending 状态。
// 链上执行失败时会 panic 并携带错误码。
func submit(client *milon.Client, signer *account, what string, encode func() (api.PackedInstruction, error)) {
	wire, err := encode()
	if err != nil {
		panic("failed to encode " + what + ": " + err.Error())
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(signer.addr).
		AddIxesSig(*signer.addr, signer.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *signer.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign " + what + ": " + err.Error())
	}
	if err = client.SubmitTx(tx); err != nil {
		panic("failed to submit " + what + ": " + err.Error())
	}
	result, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for tx.TxHash()=" + tx.TxHash().String() + "|" + what + ": " + err.Error())
	}
	helper.CheckTxSuccess(result.BodyTxHistory)
	fmt.Printf("  [tx] %s ok, hash: %s, gas charged: %d\n", what, tx.TxHash(), result.BodyTxHistory.Receipt.GasCharged)
}

// expectOnChainError 提交一条预期失败的指令，打印错误而不 panic，
// 以便 demo 继续执行。节点在提交时对交易进行模拟，
// 因此确定性失败（例如 DidDeactivated）会被 SubmitTx 本身拒绝；
// 非确定性失败则体现在回执状态中。
func expectOnChainError(client *milon.Client, signer *account, what string, encode func() (api.PackedInstruction, error)) {
	wire, err := encode()
	if err != nil {
		panic("failed to encode " + what + ": " + err.Error())
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(signer.addr).
		AddIxesSig(*signer.addr, signer.sk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *signer.pk}).
		Build()
	if err != nil {
		panic("failed to build and sign " + what + ": " + err.Error())
	}
	if err = client.SubmitTx(tx); err != nil {
		fmt.Printf("  [tx] %s rejected as expected: %v\n", what, err)
		return
	}
	result, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for " + what + ": " + err.Error())
	}
	if result.BodyTxHistory.Receipt.State == api.TxStateSuccess {
		panic("expected " + what + " to fail on chain, but it succeeded")
	}
	code := uint16(0)
	if result.BodyTxHistory.Receipt.Error != nil {
		code = *result.BodyTxHistory.Receipt.Error
	}
	fmt.Printf("  [tx] %s rejected as expected, on-chain error code = %d\n", what, code)
}

// view 执行只读 view 指令并解码其响应。
func view[T any](client *milon.Client, what string, encode func() (api.PackedInstruction, error), decode func([]byte) (T, error)) T {
	wire, err := encode()
	if err != nil {
		panic("failed to encode view " + what + ": " + err.Error())
	}
	result, err := client.View([]api.PackedInstruction{wire})
	if err != nil {
		panic("failed to view " + what + ": " + err.Error())
	}
	out, err := decode(result.HTTPResponseBody)
	if err != nil {
		panic("failed to decode view " + what + ": " + err.Error())
	}
	fmt.Printf("  [view] %s => %+v\n", what, out)
	return out
}

// expectViewFail 断言某个 view 会失败，例如因为资源不存在。
// 链上失败会被序列化到响应体中，在解码时暴露出来。
func expectViewFail[T any](client *milon.Client, what string, encode func() (api.PackedInstruction, error), decode func([]byte) (T, error)) {
	wire, err := encode()
	if err != nil {
		panic("failed to encode view " + what + ": " + err.Error())
	}
	result, err := client.View([]api.PackedInstruction{wire})
	if err != nil {
		fmt.Printf("  [view] %s failed as expected: %v\n", what, err)
		return
	}
	if _, err = decode(result.HTTPResponseBody); err == nil {
		panic("expected view " + what + " to fail, but it succeeded")
	}
	fmt.Printf("  [view] %s failed as expected: %v\n", what, err)
}
