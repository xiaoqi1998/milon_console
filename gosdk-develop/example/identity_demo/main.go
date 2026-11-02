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

// account combines a signing key pair with its derived address.
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

	// alice: the personal DID used by the DID document demo (deactivated at the end).
	alice := newAccount()
	// kycOrg: an organization DID registered in the entity registry and issuing
	// VCs in the compliance demo. It stays active for the whole run.
	kycOrg := newAccount()
	// tempOrg: an organization used to demo the irreversible deactivate_organization.
	tempOrg := newAccount()
	// bob: the subject of the VC in the compliance demo.
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

// submit builds a single-instruction transaction signed by signer (authorizing both
// the instruction and the gas payment), submits it, and waits until the receipt is no
// longer pending. It panics with the error code when on-chain execution fails.
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

// expectOnChainError submits an instruction that is expected to fail and prints the
// error instead of panicking so the demo can continue. Nodes simulate the transaction
// at submission time, so deterministic failures (e.g. DidDeactivated) are rejected by
// SubmitTx itself; non-deterministic failures surface in the receipt status.
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

// view executes a read-only view instruction and decodes its response.
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

// expectViewFail asserts that a view fails, for example because the resource does not
// exist. On-chain failures are serialized into the response body and surface at decode
// time.
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
