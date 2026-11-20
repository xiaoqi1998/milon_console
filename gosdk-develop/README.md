# Milon Go SDK

The Go SDK for the Milon blockchain: contract interaction (IDL encoding/decoding), transaction building and signing (four payment modes / multisig / vote gating), RPC communication, gRPC block streaming, and event and resource decoding.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Package Layout](#package-layout)
- [Core Concepts](#core-concepts)
  - [Contract Calls: gen Encoding](#contract-calls-gen-encoding)
  - [Transaction Building and the Four Payment Modes](#transaction-building-and-the-four-payment-modes)
  - [Signature Modes](#signature-modes)
  - [Multisig Account Signing](#multisig-account-signing)
  - [MIP-25 Vote Gate Signing](#mip-25-vote-gate-signing)
  - [Transaction Validation](#transaction-validation)
  - [On-Chain Interaction: Submit / Simulate / Wait](#on-chain-interaction-submit--simulate--wait)
- [IDL Type System](#idl-type-system)
- [RPC Methods and the Client Wrapper](#rpc-methods-and-the-client-wrapper)
  - [Client and RpcClientImpl](#client-and-rpcclientimpl)
  - [Method Overview](#method-overview)
  - [MethodType Table](#methodtype-table)
  - [Transport and Retries](#transport-and-retries)
- [Request Options](#request-options-requestoption--waitoption)
- [Block Stream (gRPC)](#block-stream-grpc)
- [Advanced Usage](#advanced-usage)
  - [Read-Only Queries (View)](#read-only-queries-view)
  - [Event Decoding](#event-decoding)
  - [Resource Decoding](#resource-decoding)
  - [Printing Transaction History](#printing-transaction-history)
  - [Custom IDLs](#custom-idls)
- [Network Configuration](#network-configuration)
- [Example Index](#example-index)

## Installation

```bash
go get github.com/milon-labs/milon-go-sdk
```

## Quick Start

```go
package main

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
)

func main() {
	// 1. Configure the network (LocalNet / DevNet / custom)
	client, err := milon.NewClient(milon.DevNet)
	if err != nil {
		panic(err)
	}

	// 2. Create a key pair (Ed25519)
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk := sk.Ed25519Public()
	address, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		panic(err)
	}

	// 3. Claim test tokens
	if err := client.ClaimFaucet(sk, address, lib.PubKeySignatureMode{PublicKey: *pk}); err != nil {
		panic(err)
	}

	// 4. Query the balance (MIL)
	balance, err := client.BalanceOf(address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("balance: %d\n", balance)
}
```

`NewClient` automatically loads every built-in IDL (`gen.DefaultIDLs`) as inlined data, binds the `gen` app objects and builds the IDL registry, so no extra initialization is needed. IDL load/bind failures are returned as errors and the caller decides how to handle them. For fixed configurations (examples, package-level initialization) use `milon.MustNewClient(...)`, which panics on failure. The returned `*Client` embeds `RpcClientImpl`, so every RPC method (`SubmitTx`, `GetAccount`, `BatchView`, ...) is promoted directly onto `client`.

## Package Layout

```text
├── client.go                  NewClient / MustNewClient, Client (embeds RpcClientImpl), RequestOption / WaitOption
├── network.go                 Network configuration (LocalNet / DevNet / custom Network)
├── rpcClientV1.go             RPC implementation: JSON / postcard transport, IDL loading, per-method result structs, ViewValue (package-level generic view)
├── blockStream.go             gRPC block stream: NewBlockStream low-level API + Client.StreamBlocks
├── framedHistory.go           Postcard decoding of the block-stream transaction frame FramedHistory
├── printFramedHistory.go      PrintFramedHistory: prints a full transaction-frame breakdown
├── resolveResourcePaths.go    ResolveResourcePaths: batch resource-path resolution
├── findIDLType.go             Client.FindIDLTypeByTag: resolve the Provider and IDLType by type_tag
│
├── api/                       RPC request / response structs (single + batch pairs)
│   ├── base.go                PackedInstruction, TxHash, TxId, RsHash, BlobHash, AccessRecord, PersistedValue, MILToken
│   ├── apiFailure.go          On-chain ApiFailure error return
│   ├── chainHead.go           Chain head
│   ├── getAccount.go          Account view
│   ├── getBlockByHeight.go    Block
│   ├── getTxByHash.go         Transaction query (TxHistory / TxReceipt / TxStatePending·Success·Failed)
│   ├── getTxHistoryProof.go   Transaction history proof
│   ├── eventsByTxHash.go      Events (EventEntry / TypeTagWithData)
│   ├── getResource.go         Resource
│   ├── getAccessValue.go      External access value (blob)
│   ├── simulateTx.go          Simulation receipt (SimulateReceipt / TxFailurePayload)
│   ├── nodeCapability.go      Node capability
│   └── batch*.go              Batch variants: batchSubmitTx / batchSimulateTx / batchView / batchGetAccount /
│                              batchGetBlockByHeight / batchGetTxByHash / batchGetTxHistoryProof /
│                              batchEventsByTxHash / batchGetResource / batchGetResourcePathByHash / batchGetAccessValue
│
├── lib/                       Transaction building and signing
│   ├── transaction.go         Transaction, TxHash, IxHashes, ValidateWire(With), chain ID (SetChainId / GetChainId)
│   ├── transactionBuilder.go  TransactionBuilder: the four payment modes
│   ├── accountSignature.go    AccountSignature, AuthBit helpers, vote gate hash
│   ├── accountSignatureBuild.go AccountSignatureBuilder: single-sig / multisig / placeholder signatures
│   ├── rpcRequest.go          RPC request envelope and MethodType constants
│   └── rpcResponse.go         RPC response envelope
│
├── crypto/                    Cryptography
│   ├── address.go             Address generation and parsing (Hex / Base58 / relaxed input)
│   ├── secretkey.go           Secret keys (Ed25519 / Secp256k1 / BLS12381 / FnDSA-512)
│   ├── publickey.go           Public keys
│   ├── signature.go           Signature struct and verification (including batch verification)
│   ├── fn_dsa512.go           FnDSA-512 implementation
│   ├── hash_domain.go         Blake3 hash domain constants and hasher pool (AcquireHasher / ReleaseHasher)
│   └── error.go
│
├── provider/                  Contract IDL loading and codec
│   ├── IDL/                   Built-in IDL JSON (system/account/token/staking/identity/sftoken/dex/validator/keyless/social/demo)
│   ├── provider.go            Single IDL: instruction encoding (Encode), value serialization / deserialization
│   ├── registry.go            IDLRegistry: unified decoding and formatting for instructions / events / resources / views
│   ├── idlTypeResolver.go     type_tag-driven dynamic type resolution (block stream, inline resource values)
│   └── types.go               IDL data structs (including the resources section)
│
├── gen/                       Type-safe entry points generated by tools/idlgen
│   ├── gen.go                 App binding registry (RegisterApp / BindAll)
│   └── idl_gen.go             Generated by idlgen (do not edit): DefaultIDLs + 11 app objects + struct types
│
├── helper/                    Common helpers (return errors instead of panicking)
│   ├── checkTxSuccess.go      CheckTxSuccess: returns an error when the receipt is not successful
│   ├── checkSimulateSuccess.go CheckSimulateSuccess: returns an error when simulation fails
│   ├── eventsByTxHash.go      DisplayEventsByTxHash
│   ├── getAccount.go          DisplayGetAccount
│   ├── listSigners.go         DisplayAccountGetListSigners
│   └── txHistory.go           DisplayTxHistory
│
├── postcard/                  Postcard serialization / deserialization (chain convention)
├── types/                     Bitmap64 bitmap (bitbap.go)
│
├── example/                   Standalone runnable examples (one main per directory, see "Example Index")
│   ├── account_create_four_crypto/  Create accounts with four cryptographic algorithms
│   ├── account_demo/                Account creation and multisig management
│   ├── create_multisig_demo/        Multisig account + multisig transfer
│   ├── create_multisig_vote/        Multisig voting and execution (split payment)
│   ├── grpc_demo/                   gRPC block stream subscription
│   ├── identity_demo/               DID / institution registration / VC compliance
│   ├── multi_ix_demo/               Multiple instructions in a single transaction
│   ├── pubkey_signature_mode_demo/  SkipPubKey signature mode
│   ├── sft_demo/                    Full SFT lifecycle
│   ├── token_demo/                  Token creation / minting / transfer
│   └── view_demo/                   Read-only queries and batch decoding
│
└── tools/                     HTTP utilities and the IDL code generator
    ├── http.go                HTTP client (connection pool; read requests retry on 5xx, submit-style writes do not)
    └── idlgen/                IDL → Go code generator
```

## Core Concepts

### Contract Calls: gen Encoding

All contract calls are encoded through the `gen` package (type-safe entry points generated from the IDLs); argument types come from the IDL, so mistakes surface at compile time:

- **Write instructions** (change on-chain state; must be signed and submitted as a transaction):

```go
wire, err := gen.Token.Create.Args().Encode()
```

- **Read-only methods** (view queries; no signature needed):

```go
wire, err := gen.Token.BalanceOf.Args(token, account).Encode()
```

`gen` exposes 11 app objects: `System`, `Account`, `Token`, `Staking`, `Identity`, `Sftoken`, `Dex`, `Validator`, `Keyless`, `Social` and `Demo`, matching the apps registered in `provider/IDL/index.json` one-to-one.

`gen/idl_gen.go` is a **generated artifact — do not edit it by hand** (the file header declares `DO NOT EDIT`): it is a compile-time mirror of `provider/IDL/index.json` and every IDL JSON it references. Any IDL change (adding/removing instructions, changing argument or return types, altering `types` / `resources` / `events`) requires rerunning:

```bash
go run ./tools/idlgen -index provider/IDL/index.json
```

This single command refreshes everything generated: `DefaultIDLs` (the IDL data `NewClient` loads as inlined constants), the Go types for custom structs / enums with their `ToValue` / `fromXxx` converters, the typed instruction builders `gen.Xxx.Method.Args(...).Encode()`, `DecodeView` for `view` methods, and the `RegisterApp` bindings in `init()`. If the generator is not rerun, the stale signatures still compile, but argument encoding and return-value decoding drift away from the on-chain IDL (misaligned fields / mismatched types), which shows up as `Encode` errors or on-chain `ApiFailure`. See [Custom IDLs](#custom-idls).

### Transaction Building and the Four Payment Modes

`lib.NewTransactionBuilder(instructions)` creates a chainable builder: call `WithPayer` first to set the paying account, then pick the payment mode and add signatures as the use case requires.

The `AuthBit` (64-bit authorization bitmap) layout in a transaction:

- **bit 0-61**: instruction authorization bits; instruction i maps to bit i (at most 62 instructions per transaction)
- **bit 62**: vote gate bit (MIP-25 vote gate), used to execute an intent after a multisig vote passes
- **bit 63**: gas authorization bit (payer bit)

| Mode | Description | Typical use case |
|------|-------------|------------------|
| **UnifiedPayerGasOnly** | the payer signs gas only (bit63); instructions need no signature | pure sponsorship (payer is not the operator) |
| **UnifiedPayerSignAll** | the payer covers the ix bit(s) and the gas bit (bit63) in one signature | the payer is the sole operator |
| **UnifiedPayerSeparateIx** | the payer signs gas only (bit63); instructions are signed by independent executor accounts | sponsor + executor separation |
| **SplitPayerSelfPay** | no payer; each executor signs its own ix bit(s) and the gas bit (bit63) | multiple operators paying their own way |

#### UnifiedPayerGasOnly

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(payer).
	AddPayerSig(*payer, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPk}). // signs gas (bit63) only
	Build()
```

#### UnifiedPayerSignAll

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(account).
	AddIxAndPayerSig(*account, accountSk, 0, lib.PubKeySignatureMode{PublicKey: *accountPk}). // ix0 + gas in one signature
	Build()
```

#### UnifiedPayerSeparateIx

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(payer).
	AddPayerSig(*payer, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPk}).                   // payer signs gas only
	AddIxesSig(*executor, executorSk, []uint8{0}, false, lib.PubKeySignatureMode{PublicKey: *executorPk}). // executor signs ix0 only
	Build()
```

#### SplitPayerSelfPay

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	AddIxesSig(*executor, executorSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *executorPk}). // ix0 + gas signed together
	Build()
```

#### TransactionBuilder Methods

| Method | Description |
|--------|-------------|
| `NewTransactionBuilder(instructions)` | Creates the builder; `Stamp` is set to the current millisecond |
| `WithPayer(addr)` | Sets the unified payer (UnifiedPayer) |
| `WithStamp(stamp)` | Sets the transaction timestamp |
| `AddPayerSig(owner, sk, mode)` | Authorizes bit63 (payer) only |
| `AddIxAndPayerSig(owner, sk, ix, mode)` | Authorizes a single ix + bit63 |
| `AddIxesSig(owner, sk, ixs, includePayer, mode)` | Authorizes multiple ixs; `includePayer` controls whether bit63 is included |
| `AddSignature(owner, acSig)` | Appends an already-built signature (e.g. a vote gate signature) |
| `ApplySlots(slots)` | Registers signing slots (without private keys) |
| `SimulateSlots()` | Generates placeholder signatures of matching length for the registered slots |
| `SignWith(signers...)` | Signs the registered slots for real by matching private keys via address |
| `ResetSigs()` | Clears all signatures (placeholder → real signature switch) |
| `Tx()` | Returns the underlying `*lib.Transaction` |
| `Build()` | Returns `*lib.Transaction`; the first error in the chain is surfaced here |

> Reusing the same builder keeps `Stamp` unchanged, so the "simulation transaction" and the "real-signature transaction" share an identical `TxHash`. See [On-Chain Interaction](#on-chain-interaction-submit--simulate--wait).

### Signature Modes

`AccountSignatureMode` has two implementations:

| Mode | Description |
|------|-------------|
| `PubKeySignatureMode{PublicKey, SkipPubKey, SigBit}` | Single-public-key signing. With `SkipPubKey=true` the public key is not put on chain, and `SigBit` must be resolved from the on-chain signers list via `Client.AccountSignerBit` |
| `MultisigKeySignatureMode{Index, PublicKey}` | Signing by a multisig account participant; the chain locates the signer slot via `SigBit = 1 << Index` |

`Client.AccountSignerBit(address)` only supports single-signer accounts (exactly one bit set in the bitmap); for multisig accounts use `MultisigKeySignatureMode` directly.

### Multisig Account Signing

Under the same `AuthBit`, each participant appends their own signature, which are finally combined into one `AccountSignature`:

```go
txHash := tx.TxHash()
ixPart := []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}} // the ix hash is required when authorizing ix0

sig, err := lib.NewAccountSignatureBuilder().
	AuthorizeIxAndPayer(0). // authorize ix0 + gas (bit63)
	SignMultisigKey(*account, sk0, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 0, PublicKey: *pk0}).
	SignMultisigKey(*account, sk1, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pk1}).
	Build()
if err != nil {
	panic(err)
}
tx.AddSignature(*account, *sig)
```

Key points:

- The `ixPart` passed to `SignMultisigKey` must match the ixs authorized in `AuthBit` (ix0 in this example); otherwise the message signed by the participants differs from the chain's recomputation and verification fails
- `MultisigKeySignatureMode.Index` is the participant's position in the signers list (0-based), not a weight
- Placeholder signatures: `SimulateSignMultisigKey(mode)` takes only the mode parameter, and the signature length is derived from the public key type automatically; use it to estimate gas without private keys

`AccountSignatureBuilder` methods: `AuthorizePayer()`, `AuthorizeIxAndPayer(ix)`, `AuthorizeIxes(indices)`, `Sign(...)`, `SimulateSign(...)`, `SignMultisigKey(...)`, `SimulateSignMultisigKey(mode)`, `Build()`.

### MIP-25 Vote Gate Signing

After a multisig vote passes, the execution transaction can be completed by a "vote gate signature + sponsor" (see `example/create_multisig_vote`):

```go
// 1. Vote gate signature: bit62 + authorized ix bits, no public key / no signature body
voteGateAuth, err := lib.AuthVoteIxes([]uint8{0})
if err != nil {
	panic(err)
}

// 2. Execution transaction: the relayer pays gas, the multisig account only provides the gate authorization
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{transferWire}).
	WithPayer(relayer).
	AddPayerSig(*relayer, relayerSk, lib.PubKeySignatureMode{PublicKey: *relayerPk}).
	AddSignature(*accountMultiSig, lib.Unsigned(voteGateAuth)).
	Build()
```

- `lib.Unsigned(authBit)` builds a signature containing only `AuthBit` (`SigBit` / `Signatures` / `PubKey` are all empty)
- `AccountSignature.IsVoteGateOnly()` tells whether the signature is a pure gate vote
- `lib.VoteBatchHash(authBit, allIxHashes)` computes the voting intent hash, which must match the on-chain `vote_batch_hash`

### Transaction Validation

`Transaction.ValidateWireWith(sponsorIx)` validates the wire-level structure before submission; `ValidateWire()` is equivalent to `ValidateWireWith([]uint8{})`:

General checks:

- Instructions are non-empty and no more than 62 (`AuthReservedBit`)
- Instruction hashes must not repeat
- Signature owners (account addresses) must not repeat
- `AuthBit` must not be empty, and ix bits must not be out of range (bit62 / bit63 excluded)

Payment mode checks:

- **UnifiedPayer** (`tx.Payer != nil`): the payer must sign and authorize bit63; `sponsorIx` has no effect in this mode
- **SplitPayerSelfPay** (`tx.Payer == nil`): every unsponsored instruction needs someone authorizing both bit63 and that ix; authorizing bit63 alone is a payment mode conflict

`Client.SubmitTx` calls `ValidateWire` internally, `Client.SubmitTxWithSponsorIxes(tx, sponsorIxes)` calls `ValidateWireWith(sponsorIxes)`, and `Client.SubmitAndWait` goes through `SubmitTx` first, so it carries the same wire validation.

### On-Chain Interaction: Submit / Simulate / Wait

#### Submitting a Transaction

```go
client.SubmitTx(tx)                                // validates with ValidateWire, then submits
client.SubmitTxWithSponsorIxes(tx, []uint8{0})     // validates with ValidateWireWith, then submits (sponsor ixs selectable)
```

Transport and retries: submit / simulate methods use `application/x-milon+postcard`, everything else uses `application/x-milon+json` (the body is still postcard bytes); `Submit*` never retries automatically (a retry after a lost response would submit the same transaction twice), while `Simulate*` and query methods retry on network errors / 5xx. See [Transport and Retries](#transport-and-retries) for the full matrix.

#### Submit and Wait (SubmitAndWait)

When step-by-step handling is not needed, `SubmitAndWait` combines "submit → poll → receipt check" into a single call returning one error (every transaction in `example/` uses this entry point):

```go
txHistory, err := client.SubmitAndWait(tx)
if err != nil {
	panic(err) // submit failure / wait timeout / on-chain execution failure all surface here
}
fmt.Printf("tx hash: %s, gas charged: %d\n", tx.TxHash(), txHistory.Receipt.GasCharged)
```

- Internally it calls `SubmitTx` (including `ValidateWire`) and then `WaitForTransaction`; the two error classes are wrapped as `failed to submit transaction: ...` and `failed to wait for transaction: ...`
- The receipt is checked before returning: when `Receipt.State != api.TxStateSuccess` it returns `transaction failed on chain: error code = N` (equivalent to `helper.CheckTxSuccess`), so a successful `txHistory` is guaranteed to be on chain and executed successfully; you can read `txHistory.Receipt.GasCharged` / `txHistory.Receipt.Access` directly, or fetch events with `EventsByTxHash`
- It accepts only `...WaitOption` (poll interval / timeout / context); the submit phase uses the default context. To set a dedicated context or request ID for the submit request, call `SubmitTx` + `WaitForTransaction` separately

#### Simulation (Optional, Gas Estimation)

`SimulateTx(tx)` simulates execution and estimates gas. The standard flow is "placeholder signatures → simulate → real signatures", and the same builder must be reused so that `TxHash` stays unchanged:

```go
txBuilder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(payer).
	ApplySlots([]lib.SigningSlot{
		{Address: *payer, Mode: lib.PubKeySignatureMode{PublicKey: *payerPk}},                              // signs gas only
		{Address: *executor, InstructionIndices: []uint8{0}, Mode: lib.PubKeySignatureMode{PublicKey: *executorPk}},
	}).
	SimulateSlots()

simulateResult, err := client.SimulateTx(txBuilder.Tx()) // estimate gas without private keys
if err := helper.CheckSimulateSuccess(simulateResult); err != nil {
	panic(err)
}

txBuilder.ResetSigs().
	SignWith(lib.Signer{SecretKey: payerSk, PublicKey: *payerPk}, lib.Signer{SecretKey: executorSk, PublicKey: *executorPk})
tx, err := txBuilder.Build()
```

An empty `SigningSlot.InstructionIndices` means "sign gas only"; `IncludePayer` means the ix signature also carries bit63.

#### Waiting for Confirmation

```go
result, err := client.WaitForTransaction(tx.TxHash())
```

When all you care about is "the transaction landed successfully on chain", you don't need to write these two steps by hand — use `SubmitAndWait` (see the previous section). `WaitForTransaction` polls `GetTxByHash` until the receipt state is no longer `TxStatePending`; every poll uses a fresh request ID (to avoid the node's request cache), and network errors during polling do not return immediately — the last error is recorded and retrying continues. Both `pollPeriod` and `pollTimeout` must be positive, otherwise an error is returned immediately; the timeout check happens after each query, guaranteeing that the last attempt is completed before timing out. To validate the on-chain execution result, use `helper.CheckTxSuccess(result.BodyTxHistory)`. The `helper` decode / display functions (`DisplayTxHistory`, `DisplayEventsByTxHash`, `DisplayGetAccount`, `DisplayAccountGetListSigners`) likewise return errors instead of panicking, and the examples uniformly `panic(err)`.

## IDL Type System

The provider implements contract method serialization and deserialization on top of IDL JSON. Dynamic decoding (`Provider.Decode`, `IDLRegistry.Decode*`) yields the following value types:

| IDL type | Go type | Notes |
|----------|---------|-------|
| `bool` / `boolean` | `bool` | Boolean |
| `u8` / `u16` / `u32` / `u64` | `uint8` / `uint16` / `uint32` / `uint64` | Unsigned integers |
| `i8` / `i16` / `i32` / `i64` | `int8` / `int16` / `int32` / `int64` | Signed integers |
| `u128` | `*big.Int` | 128-bit integer (encoding accepts `*big.Int` and all integer kinds) |
| `Amount` / `Epoch` / `Bitmap64` | `uint64` | On-chain semantic aliases, handled as u64 |
| `Address` / `Signer` / `AnySigner` | `*crypto.Address` | 20-byte address |
| `PublicKey` | `*crypto.PublicKey` | variant tag + fixed-length bytes |
| `Signature` | `*crypto.Signature` | variant tag + fixed-length bytes (depends on the key type) |
| `String` / `string` | `string` | UTF-8 string |
| `bytes` | `[]byte` | Byte array |
| `B96` / `B144` / `B160` / `B256` | `[12]byte` / `[18]byte` / `[20]byte` / `[32]byte` | Fixed-length byte arrays |
| `vec<T>` | `[]any` | Variable-length array |
| `option<T>` | `nil` / concrete value | Optional value |
| `tuple<T1,T2,...>` | `[]any` | Tuple |
| `map<K,V>` | `map[any]any` | Map |
| struct | `map[string]any` | field name → value |
| enum | `map[string]any` | contains `variant` / `index`; data-carrying variants also carry their fields |

The typed methods generated in `gen` map the types above to concrete Go signatures (e.g. `Address` → `*crypto.Address`, `Vec<PublicKey>` → `[]*crypto.PublicKey`).

## RPC Methods and the Client Wrapper

### Client and RpcClientImpl

The `*Client` returned by `milon.NewClient(...)` embeds the `RpcClientImpl` interface (35 methods) directly; every RPC method is promoted onto `*Client`, so no `.RpcClient` forwarding layer is needed at call sites:

```go
client, err := milon.NewClient(milon.DevNet)
if err != nil {
	panic(err)
}

client.SubmitTx(tx)             // promoted from the embedded RpcClientImpl
client.GetProviderManager()     // same
client.StreamBlocks(0, onBlock) // Client's own method (gRPC block stream)
```

- `Client` only defines `StreamBlocks` and `FindIDLTypeByTag` itself; everything else is promoted from the interface
- `var _ RpcClientImpl = (*Client)(nil)` in `client.go` is a compile-time assertion: if the interface gains a method and `Client` no longer satisfies it, the build fails immediately instead of breaking only at a call site
- Replaceable in tests: `&milon.Client{RpcClientImpl: fake}` (an interface field; `network` is the zero value, used only to swap RPC methods); production code uses `NewClient` / `MustNewClient`, which load `gen.DefaultIDLs`, build the IDL registry and bind the `gen` apps

> Migration note (relative to older versions): the `Client.RpcClient` field has been removed. `client.RpcClient.SubmitTx(tx)` → `client.SubmitTx(tx)`; custom implementations `milon.Client{RpcClient: impl}` → `milon.Client{RpcClientImpl: impl}`.

### Method Overview

`RpcClientImpl` has 35 methods, grouped by purpose:

| Category | Method | Description |
|----------|--------|-------------|
| Account | `ClaimFaucet(sk, address, mode)` | Claims test tokens |
| | `CreateAccount(sk, pk)` | Creates an on-chain account |
| | `BalanceOf(account, token ...*crypto.Address)` | Queries a token balance (`token` omitted means MIL; at most one) |
| | `ListAccountSigners(address)` | Queries the account signer list (`[account info, signers]`) |
| | `AccountSignerBit(address)` | Queries the slot bitmap of a single-signer account (`types.Bitmap64`) |
| | `TokenMetadata(token)` | Queries token metadata (name / symbol / decimals / icon / URI) |
| Transaction | `SubmitTx(tx, opts...)` | Validates with `ValidateWire`, then submits (postcard, no retry) |
| | `SubmitTxWithSponsorIxes(tx, sponsorIxes, opts...)` | Validates with `ValidateWireWith`, then submits (postcard, no retry) |
| | `SubmitAndWait(tx, opts...)` | Submit + poll + receipt check; returns `*api.TxHistory` (accepts `WaitOption` only) |
| | `BatchSubmitTx(txs, opts...)` | Batch submit (postcard, no retry; per-transaction tx hash / failure reason) |
| | `SimulateTx(tx, opts...)` | Simulates execution to estimate gas (postcard, auto retry) |
| | `BatchSimulateTx(txs, opts...)` | Batch simulation (postcard, auto retry) |
| | `WaitForTransaction(txHashOrTxId, opts...)` | Polls until the transaction is confirmed |
| Contract | `View(wire, opts...)` | Single read-only query (method 15); the result is in `ViewSingleResult.HTTPResponseBody` (raw encoded return value) |
| | `BatchView(wires, opts...)` | Batch read-only query (method 16); `BodyBatchViewItems` gives Ok / Err per request in order |
| Chain | `GetChainHead(opts...)` | Queries the chain head |
| | `GetBlockByHeight(height, opts...)` / `BatchGetBlockByHeight(heights, opts...)` | Queries blocks by height (single / batch) |
| | `GetTxByHash(txHashOrTxId, opts...)` / `BatchGetTxByHash(list, opts...)` | Queries transactions by hash / TxId |
| | `GetTxHistoryProof(txHashOrTxId, opts...)` / `BatchGetTxHistoryProof(list, opts...)` | Queries transaction history proofs |
| | `GetAccount(accountRelaxed, opts...)` / `BatchGetAccount(list, opts...)` | Queries account details (`accountRelaxed` accepts an address / Hex / Base58) |
| | `EventsByTxHash(txHash, typeTagFilter, opts...)` / `BatchEventsByTxHash(reqList, opts...)` | Queries transaction events (`typeTagFilter` of `nil` means no filtering) |
| | `GetNodeCapability(opts...)` | Queries node capabilities (method 30) |
| Resource | `GetResource(rsHash, opts...)` / `BatchGetResource(rsHashList, opts...)` | Queries resources |
| | `GetResourcePathByHash(rsHash, opts...)` / `BatchGetResourcePathByHash(rsHashList, opts...)` | Queries resource paths by hash |
| | `GetAccessValue(blobHash, opts...)` / `BatchGetAccessValue(blobHashList, opts...)` | Queries external access values (blobs) |
| Block stream | `StreamBlocks(startBlock, onBlockDone)` | Subscribes to the gRPC block stream (`Client`'s own method, not an RPC) |
| Metadata | `GetAllPd()` | Returns all loaded providers (IDL name → provider) |
| | `GetProviderManager()` | Returns the `*provider.IDLRegistry` (instruction / event / resource / view decoding) |
| | `FindIDLTypeByTag(typeTag)` | Resolves `(*provider.Provider, *provider.IDLType)` by type_tag (`Client`'s own method) |

Beyond the interface there are two one-call entry points: the package-level generic function `milon.ViewValue` (encode + `View` + decode, see [Read-Only Queries](#read-only-queries-view)) and the method `client.SubmitAndWait` (submit + wait + receipt check).

Result structs uniformly carry `HTTPResponseBody []byte` (the raw response body, for later decoding such as `DecodeView`) and `BodyXxx` fields (the parsed struct). Batch methods map items to the **request order** one-to-one, and each item is exactly one of Ok / Failure (`Value` / `Block` / `Data` + `Failure *api.ApiFailure`); a single failure does not affect the other items.

### MethodType Table

| Constant | Value | Used by |
|----------|-------|---------|
| `MethodTypeChainHead` | 1 | `GetChainHead` |
| `MethodTypeSubmitTx` | 5 | `SubmitTx` / `SubmitTxWithSponsorIxes` |
| `MethodTypeBatchSubmitTx` | 6 | `BatchSubmitTx` |
| `MethodTypeSimulateTx` | 10 | `SimulateTx` |
| `MethodTypeBatchSimulateTx` | 11 | `BatchSimulateTx` |
| `MethodTypeView` | 15 | `View` (single) |
| `MethodTypeBatchView` | 16 | `BatchView` |
| `MethodTypeGetAccount` | 20 | `GetAccount` |
| `MethodTypeBatchGetAccount` | 21 | `BatchGetAccount` |
| `MethodTypeEventsByTxHash` | 25 | `EventsByTxHash` |
| `MethodTypeBatchEventsByTxHash` | 26 | `BatchEventsByTxHash` |
| `MethodTypeNodeCapability` | 30 | `GetNodeCapability` |
| `MethodTypeGetBlockByHeight` | 50 | `GetBlockByHeight` |
| `MethodTypeBatchGetBlockByHeight` | 51 | `BatchGetBlockByHeight` |
| `MethodTypeGetTxByHash` | 55 | `GetTxByHash` / `WaitForTransaction` |
| `MethodTypeBatchGetTxByHash` | 56 | `BatchGetTxByHash` |
| `MethodTypeGetTxHistoryProof` | 60 | `GetTxHistoryProof` |
| `MethodTypeBatchGetTxHistoryProof` | 61 | `BatchGetTxHistoryProof` |
| `MethodTypeGetResource` | 150 | `GetResource` |
| `MethodTypeBatchGetResource` | 151 | `BatchGetResource` |
| `MethodTypeGetResourcePathByHash` | 155 | `GetResourcePathByHash` |
| `MethodTypeBatchGetResourcePathByHash` | 160 | `BatchGetResourcePathByHash` |
| `MethodTypeGetAccessValue` | 165 | `GetAccessValue` |
| `MethodTypeBatchGetAccessValue` | 166 | `BatchGetAccessValue` |

### Transport and Retries

| Method | Content-Type | Auto retry |
|--------|--------------|------------|
| `SubmitTx` / `SubmitTxWithSponsorIxes` / `BatchSubmitTx` | `application/x-milon+postcard` | No |
| `SimulateTx` / `BatchSimulateTx` | `application/x-milon+postcard` | Yes |
| Other query methods (including batch) | `application/x-milon+json` (body is still postcard bytes) | Yes (network errors and 5xx) |

Submit-style requests never retry automatically: the node may already have accepted the transaction while the response was lost, and retrying would submit the same transaction twice — whether to retry is up to the caller, based on `request_id` / `txHash`. `WaitForTransaction` actively rotates `request_id` on every poll so that it never gets stuck on the node's request cache returning a stale pending receipt.

## Request Options (RequestOption / WaitOption)

All RPC methods accept `...RequestOption`; `WaitForTransaction` and `SubmitAndWait` accept `...WaitOption`. Both are type-safe functional options.

### RequestOption (Common to RPC Methods)

| Option | Description |
|--------|-------------|
| `milon.WithContext(ctx)` | Sets the context, supporting timeout / cancellation |
| `milon.WithRequestID(id)` | Custom request ID (type `lib.RequestID`; by default generated from a millisecond timestamp plus an in-process sequence number) |

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
client.GetChainHead(milon.WithContext(ctx))

client.GetTxByHash(txHash, milon.WithRequestID(12345))
```

### WaitOption (Specific to WaitForTransaction / SubmitAndWait)

| Option | Description |
|--------|-------------|
| `milon.WithWaitContext(ctx)` | Sets the context, supporting timeout / cancellation |
| `milon.WithWaitRequestID(id)` | Custom request ID (by default every poll uses a fresh request ID; once set it is reused) |
| `milon.WithWaitPollPeriod(d)` | Poll interval (default 1s) |
| `milon.WithWaitPollTimeout(d)` | Poll timeout (default 30s) |

**1. Global defaults (NewClient level)**

```go
client, err := milon.NewClient(milon.DevNet,
	milon.WithClientPollPeriod(500*time.Millisecond),
	milon.WithClientPollTimeout(30*time.Second),
)
if err != nil {
	panic(err)
}
```

**2. Per-call override (WaitForTransaction / SubmitAndWait level, takes precedence over the global defaults)**

```go
result, err := client.WaitForTransaction(txHash,
	milon.WithWaitPollPeriod(200*time.Millisecond),
	milon.WithWaitPollTimeout(20*time.Second),
	milon.WithWaitContext(ctx),
)

txHistory, err := client.SubmitAndWait(tx,
	milon.WithWaitPollTimeout(60*time.Second), // affects the wait phase only; the submit phase uses the default context
)
```

On timeout the error carries the last failure reason (`timeout after ..., last error: ...`); a cancelled context returns immediately.

## Block Stream (gRPC)

`Client.StreamBlocks` subscribes to the node's block stream starting at `startBlock` and invokes the callback per block:

```go
err := client.StreamBlocks(0, func(blockHeight uint64, histories []*milon.FramedHistory) bool {
	for _, fh := range histories {
		fmt.Printf("block %d tx %s state=%d gas=%d\n",
			blockHeight, fh.TxHashBase58(), fh.State, fh.GasCharged)
	}
	return true // true: auto-Ack and continue; false: stop subscribing and return nil
})
```

- The address comes from `Network.GrpcAddr`; an error is returned when it is not configured
- `consumerID` is generated by the SDK and guaranteed unique within the process; `max_in_flight` is fixed at 64
- The per-frame receive limit is raised to 32MB; a transaction that fails to decode yields an error wrapping `milon.ErrTxDecode` (it is not silently skipped), and unacked blocks can be rescanned after reconnecting

Key `FramedHistory` fields: `Version`, `Stamp`, `Payer` (index into the signature array; `nil` means split payment), `Signatures`, `Instructions`, `TxID`, `TxHash`, `State`, `Accesses` (before/after snapshots), `Events`, `ErrorCode`, `GasCharged`.

The low-level control API (managing ctx and reconnection strategy yourself) is available as well:

```go
stream, err := milon.NewBlockStream(ctx, addr) // addr looks like "127.0.0.1:50051"
err = stream.Subscribe("my-consumer", 0, 64)
frame, err := stream.Next()                    // *milon.StreamFrame
err = stream.Ack(frame.BlockHeight)
defer stream.Close()
```

## Advanced Usage

### Read-Only Queries (View)

Encoding and deserialization both go through the `gen` package:

```go
// 1. Encode: argument types come from the IDL
wire, err := gen.Token.BalanceOf.Args(api.MILToken, account).Encode()
if err != nil {
	panic(err)
}

// 2. Query (single view)
viewResult, err := client.View(wire)
if err != nil {
	panic(err)
}

// 3. Decode into the concrete Go type according to the return type
balance, err := gen.Token.BalanceOf.DecodeView(viewResult.HTTPResponseBody)
if err != nil {
	panic(err)
}
fmt.Printf("balance: %d\n", balance) // uint64
```

```go
// Encode + query + decode in one step (equivalent to the three steps above)
balance, err := milon.ViewValue(client,
	gen.Token.BalanceOf.Args(api.MILToken, account).Encode,
	gen.Token.BalanceOf.DecodeView,
)
if err != nil {
	panic(err)
}
```

- `milon.ViewValue[T](client, encode, decode, opts...)` is a package-level generic function (Go has no generic methods); it performs encode + `client.View` + `DecodeView`, and any failing step returns a phase-wrapped error: `failed to encode view instruction` / `failed to call view` / `failed to decode view`
- The first parameter has type `milon.ViewCaller` (it only requires `View(wire, opts...)`); both `*milon.Client` and concrete RPC implementations satisfy it, which makes it easy to swap implementations in unit tests; if `client`, `encode` or `decode` is nil, an error is returned directly
- `DecodeView` takes the raw response body returned by `client.View` / `client.BatchView` (`HTTPResponseBody`) and returns a type that follows the IDL: `u64 → uint64`, `String → string`, struct → the generated struct (e.g. `gen.TokenMetadata`)
- `DecodeView` handles the Ok/Err branches automatically; a failure returned on chain (`ApiFailure`) becomes an error; `RequestOption` values are forwarded to `client.View` as variadic arguments

Query multiple views at once and decode them in batch (results correspond to the `"appName::methodName"` order):

```go
viewResult, err := client.BatchView([]api.PackedInstruction{wire1, wire2})

results, err := client.GetProviderManager().DecodeViewDatas(
	[]string{"token::BalanceOf", "demo::OrderBalance"},
	viewResult.HTTPResponseBody,
)
```

### Event Decoding

```go
eventsResult, err := client.EventsByTxHash(txHash, nil)
if err != nil {
	panic(err)
}

for _, event := range eventsResult.BodyEventsByTxHash.Events {
	decoded, err := client.GetProviderManager().DecodeEventDataByTag(
		event.Data.TypeTag,
		event.Data.Value,
	)
	if err != nil {
		continue
	}
	fmt.Printf("Event: %+v\n", decoded)
}
```

`DecodeEventDataByTag` matches the IDL `events` section by type_tag; use `FormatDecodedEvent` when a human-readable format is needed.

### Resource Decoding

On-chain persisted resources are declared in the IDL `resources` section (e.g. `token:_Balance`, `account:_Account`) and decoded by resource type_tag:

```go
txResult, err := client.GetTxByHash(txHash)

for _, record := range txResult.BodyTxHistory.Receipt.Access {
	lastWritten := record.LastWritten
	if lastWritten.Variant != 0 { // only Inline values can be decoded directly
		continue
	}

	decoded, err := client.GetProviderManager().DecodeResourceDataByTag(
		lastWritten.TypeTag,
		lastWritten.InlineData,
	)
	if err != nil {
		continue
	}
	fmt.Printf("Resource: %+v\n", decoded) // includes app_name / resource_name / resource_type / data
}
```

> - Several resources may share the same type_tag (e.g. multiple `u64` counters); in that case the first resource declared in the IDL is used
> - When a type_tag does not match the `resources` section, decoding falls back to the IDL `types` section, covering scenarios where bare value types are persisted (e.g. genesis transactions with `Address` or `B256` builtin tags); this matches the Rust SDK's registration scope (resources ∪ types)
> - In block-stream scenarios, resolve resource paths in batch with `milon.ResolveResourcePaths(client, rsHashSet)` and hand them to `PrintFramedHistory` for display

### Printing Transaction History

```go
pathMap, err := milon.ResolveResourcePaths(client, rsHashSet) // the collected set of RsHash values
milon.PrintFramedHistory(client, idx, fh, pathMap)             // instructions / resource before-after / events
```

`PrintFramedHistory`'s `logger` is an optional variadic argument (the `milon.FramedHistoryLogger` interface, which `*log.Logger` satisfies directly); when omitted, output goes to stdout.

### Custom IDLs

The built-in IDLs are inlined through `gen.DefaultIDLs`, and `NewClient` loads and binds them automatically. To use IDLs standalone for encoding/decoding outside the SDK:

```go
pd, err := provider.LoadProviderFromFile("./custom_idl/token.idl.json")
// or build from memory: pd := provider.NewProvider(idlData)

wire, err := pd.Encode("BalanceOf", provider.Args{"token": api.MILToken, "account": account})
// wire is []byte and can be used directly as an api.PackedInstruction
```

#### Regenerating gen After an IDL Change

`tools/idlgen` translates every IDL registered in `index.json` into `gen/idl_gen.go` in one pass; this is the only manually triggered code-generation step in the SDK:

```bash
go run ./tools/idlgen -index provider/IDL/index.json
# idlgen: wrote gen/idl_gen.go (11 apps)
```

| Flag | Default | Description |
|------|---------|-------------|
| `-index` | `provider/IDL/index.json` | Entry index; each IDL JSON is read from `apps[].idl` relative to the directory of this file |
| `-out` | `gen/idl_gen.go` | Output file path |
| `-pkg` | `gen` | Generated package name (for any package other than `gen`, bindings are emitted as `gen.RegisterApp`) |

The generated file contains four parts:

- `DefaultIDLs`: every IDL inlined as Go literals; `NewClient` uses it to initialize the providers, so nothing is read from disk at runtime
- Go types for custom structs / enums (e.g. `TokenMetadata`, `TokenComplianceMode`) plus their `ToValue` / `fromXxx` / `toXxxValue` converters
- Typed instruction builders for every app object: `Args(...)` → `Encode()`; `view` instructions also get `DecodeView(body)`
- `RegisterApp` bindings in `init()`, so every `NewClient` rebinds the generated objects to the freshly loaded providers

Therefore, after changing any `provider/IDL/*.json` you must rerun the command and commit the `gen/idl_gen.go` diff: manual edits are overwritten on the next generation, and an out-of-sync file makes encoding/decoding behavior unpredictable.

## Network Configuration

```go
// Local development network
client, err := milon.NewClient(milon.LocalNet)
//   RpcUrl:   http://127.0.0.1:6280/v1/rpc
//   GrpcAddr: 127.0.0.1:50051
//   ChainId:  900_000_001

// Development test network
client, err = milon.NewClient(milon.DevNet)
//   RpcUrl:   https://dev-api.milonlabs.com/v1/rpc
//   GrpcAddr: dev-api-grpc.milonlabs.com:443
//   ChainId:  100_10

// Custom network
client, err = milon.NewClient(milon.Network{
	Name:     "myNet",
	ChainId:  900_000_001,         // required: participates in transaction / instruction / authorization hashing
	RpcUrl:   "http://127.0.0.1:6280/v1/rpc",
	GrpcAddr: "127.0.0.1:50051",   // used by the block stream; StreamBlocks errors when empty
})
if err != nil {
	panic(err)
}
```

> `NewClient` calls `lib.SetChainId(config.ChainId)` to write the global chain ID (it affects transaction hashes and signature domains). Both the chain ID and the `gen` instruction objects are process-wide, so **one process supports only one `Network`**: creating another client with a different `ChainId` returns an error directly, preventing silent transaction hashes for the wrong chain.
>
> `Network.ChainId` is required and the SDK **provides no default chain ID**: `NewClient` errors out when it is missing, and if you bypass the client and build transactions with `lib` directly, `lib.GetChainId()` panics with `ErrChainIDNotSet` rather than silently computing with some fixed chain ID.

## Example Index

Every directory under `example/` is a standalone runnable example; the table lists the main ones (local debugging examples are omitted):

| Directory                         | Contents |
|-----------------------------------|----------|
| `account_create_four_crypto`      | Create accounts with four cryptographic algorithms |
| `account_demo`                    | Account creation and multisig management (AddSigner / SetThreshold / SetSignerWeight / RemoveSigner) |
| `create_multisig_demo`            | Multisig account creation + multisig transfer (including placeholder-signature simulation → real signing) |
| `create_multisig_vote`            | Multisig voting and execution (split payment) |
| `grpc_demo`                       | gRPC block stream subscription (StreamBlocks + PrintFramedHistory) |
| `identity_demo`                   | On-chain identity: DID / institution registration / VC compliance |
| `multi_ix_demo`                   | Multiple instructions in a single transaction |
| `pubkey_signature_mode_demo`      | `SkipPubKey` signature mode |
| `sft_demo`                        | Full SFT lifecycle: create_sft / slot / mint / split / transfer / approve / transfer_from / freeze / merge |
| `token_demo`                      | Token creation / minting / transfer / balance query |
| `view_demo`                       | Read-only queries and batch decoding |

How to run:

```bash
go run ./example/create_multisig_demo
```