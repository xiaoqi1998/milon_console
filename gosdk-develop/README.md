# Milon Go SDK

Milon 区块链的 Go SDK，提供合约交互（IDL 编解码）、交易构建与签名（四种付款模式 / 多签 / 投票门控）、RPC 通信、gRPC 区块流、事件与资源解析。

## 目录

- [安装](#安装)
- [快速开始](#快速开始)
- [包结构](#包结构)
- [核心概念](#核心概念)
  - [合约调用：gen 编码](#合约调用gen-编码)
  - [交易构建与四种付款模式](#交易构建与四种付款模式)
  - [签名模式](#签名模式)
  - [多签账户签名](#多签账户签名)
  - [MIP-25 投票门控签名](#mip-25-投票门控签名)
  - [交易校验](#交易校验)
  - [上链交互：提交 / 模拟 / 等待](#上链交互提交--模拟--等待)
- [IDL 类型系统](#idl-类型系统)
- [RPC 方法与 Client 封装](#rpc-方法与-client-封装)
- [请求选项](#请求选项requestoption--waitoption)
- [区块流（gRPC）](#区块流grpc)
- [高级用法](#高级用法)
  - [只读查询（View）](#只读查询view)
  - [事件解析](#事件解析)
  - [资源解析](#资源解析)
  - [交易历史打印](#交易历史打印)
  - [自定义 IDL](#自定义-idl)
- [网络配置](#网络配置)
- [示例索引](#示例索引)

## 安装

```bash
go get github.com/milon-labs/milon-go-sdk
```

## 快速开始

```go
package main

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
)

func main() {
	// 1. 配置网络（LocalNet / DevNet / 自定义）
	client := milon.NewClient(milon.DevNet)

	// 2. 创建密钥对（Ed25519）
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk := sk.Ed25519Public()
	address, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		panic(err)
	}

	// 3. 领取测试代币
	if err := client.ClaimFaucet(sk, address, lib.PubKeySignatureMode{PublicKey: *pk}); err != nil {
		panic(err)
	}

	// 4. 查询余额（MIL）
	balance, err := client.BalanceOf(address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("balance: %d\n", balance)
}
```

`NewClient` 会自动内嵌加载全部内置 IDL（`gen.DefaultIDLs`）、绑定 `gen` 应用对象并构造 IDL 注册表，无需额外初始化。

## 包结构

```text
├── client.go                  NewClient、Client 封装方法、RequestOption / WaitOption
├── network.go                 网络配置（LocalNet / DevNet / 自定义 Network）
├── rpcClientV1.go             RPC 实现：JSON / postcard 传输、IDL 加载、各方法结果结构体
├── blockStream.go             gRPC 区块流：NewBlockStream 低层接口 + Client.StreamBlocks
├── framedHistory.go           区块流交易帧 FramedHistory 的 postcard 解码
├── printFramedHistory.go      PrintFramedHistory：打印交易帧完整明细
├── resolveResourcePaths.go    ResolveResourcePaths：批量解析资源路径
│
├── api/                       RPC 请求 / 响应结构体
│   ├── base.go                PackedInstruction、TxHash、TxId、RsHash、BlobHash、AccessRecord、PersistedValue、MILToken
│   ├── apiFailure.go          链上 ApiFailure 错误返回
│   ├── accountView.go         账户视图
│   ├── block.go               区块
│   ├── chainHead.go           链头
│   ├── eventsByTxHash.go      事件（EventEntry / TypeTagWithData）
│   ├── getAccessValue.go      外部访问值
│   ├── getResource.go         资源
│   ├── get_tx_history_proof.go 交易历史证明
│   ├── batchGetResourcePathByHash.go 批量资源路径查询
│   ├── listResourcePath.go    资源路径列表
│   ├── simulateReceipt.go     模拟执行回执
│   └── txHistory.go           交易历史与回执（TxStatePending / Success / Failed）
│
├── lib/                       交易构建与签名
│   ├── transaction.go         Transaction、TxHash、IxHashes、ValidateWire(With)
│   ├── transactionBuilder.go  TransactionBuilder：四种付款模式
│   ├── accountSignature.go    AccountSignature、AuthBit 工具、投票门控哈希
│   ├── accountSignatureBuild.go AccountSignatureBuilder：单签 / 多签 / 占位签名
│   ├── rpcRequest.go          RPC 请求信封与 MethodType 常量
│   └── rpcResponse.go         RPC 响应信封
│
├── crypto/                    密码学
│   ├── address.go             地址生成与解析（Hex / Base58 / relaxed 输入）
│   ├── secretkey.go           私钥（Ed25519 / Secp256k1 / BLS12381 / FnDSA-512）
│   ├── publickey.go           公钥
│   ├── signature.go           签名结构与验签（含批量验签）
│   ├── fn_dsa512.go           FnDSA-512 实现
│   ├── hash_domain.go         Blake3 哈希域常量
│   └── error.go
│
├── provider/                  合约 IDL 加载与编解码
│   ├── IDL/                   内置 IDL JSON（system/account/token/staking/identity/sftoken/dex/validator/keyless/social/demo）
│   ├── provider.go            单个 IDL：指令编码（Encode）、值序列化 / 反序列化
│   ├── registry.go            IDLRegistry：指令 / 事件 / 资源 / View 的统一解码与格式化
│   ├── idlTypeResolver.go     基于 type_tag 的动态类型解析（区块流、内联资源值）
│   └── types.go               IDL 数据结构（含 resources 资源段）
│
├── gen/                       由 tools/idlgen 生成的类型安全入口
│   ├── gen.go                 应用绑定注册（RegisterApp / BindAll）
│   └── idl_gen.go             自动生成：DefaultIDLs + 11 个应用对象 + 结构体类型
│
├── helper/                    常用辅助函数
│   ├── checkTxSuccess.go      CheckTxSuccess：回执非成功时 panic
│   ├── checkSimulateSuccess.go CheckSimulateSuccess：模拟失败时 panic
│   ├── eventsByTxHash.go      DisplayEventsByTxHash
│   ├── getAccount.go          DisplayGetAccount
│   ├── listSigners.go         DisplayAccountGetListSigners
│   └── txHistory.go           DisplayTxHistory
│
├── postcard/                  Postcard 序列化 / 反序列化（与链约定）
├── types/                     Bitmap64 位图（bitbap.go）
└── tools/                     HTTP 工具与 IDL 代码生成器
    ├── http.go                HTTP 客户端（连接池、5xx 重试）
    └── idlgen/                IDL → Go 代码生成器
```

## 核心概念

### 合约调用：gen 编码

所有合约调用统一通过 `gen` 包（由 IDL 生成的类型安全入口）编码，参数类型由 IDL 生成，编译期即可发现错误：

- **写入指令**（修改链上状态，需签名后提交交易）：

```go
wire, err := gen.Token.Create.Args().Encode()
```

- **只读方法**（view 查询，无需签名）：

```go
wire, err := gen.Token.BalanceOf.Args(token, account).Encode()
```

`gen` 下共有 11 个应用对象：`System`、`Account`、`Token`、`Staking`、`Identity`、`Sftoken`、`Dex`、`Validator`、`Keyless`、`Social`、`Demo`，与 `provider/IDL/index.json` 的注册应用一一对应。

### 交易构建与四种付款模式

`lib.NewTransactionBuilder(instructions)` 创建链式构建器，先 `WithPayer` 指定付款账户，再按业务需求选择付款模式添加签名。

交易中的 `AuthBit`（64 位授权位图）布局：

- **bit 0-61**：指令授权位，第 i 条指令对应 bit i（单笔交易最多 62 条指令）
- **bit 62**：投票门控位（MIP-25 vote gate），用于多签投票通过后的意图执行
- **bit 63**：gas 授权位（payer 位）

| 模式 | 说明 | 典型场景 |
|------|------|----------|
| **UnifiedPayerGasOnly** | payer 只签 gas（bit63），ix 无需签名 | 纯赞助（payer 非操作者） |
| **UnifiedPayerSignAll** | payer 在同一个签名中同时覆盖 ix bit(s) 和 gas bit（bit63） | payer 即唯一操作者 |
| **UnifiedPayerSeparateIx** | payer 只签 gas（bit63），ix 由独立 executor 账户签名 | 赞助方 + 执行方分离 |
| **SplitPayerSelfPay** | 无 payer，每个 executor 各自签自己的 ix bit(s) 和 gas bit（bit63） | 多操作者各自付账 |

#### UnifiedPayerGasOnly

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(payer).
	AddPayerSig(*payer, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPk}). // 只签 gas(bit63)
	Build()
```

#### UnifiedPayerSignAll

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(account).
	AddIxAndPayerSig(*account, accountSk, 0, lib.PubKeySignatureMode{PublicKey: *accountPk}). // ix0 + gas 一次签
	Build()
```

#### UnifiedPayerSeparateIx

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(payer).
	AddPayerSig(*payer, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPk}).                   // payer 只签 gas
	AddIxesSig(*executor, executorSk, []uint8{0}, false, lib.PubKeySignatureMode{PublicKey: *executorPk}). // executor 只签 ix0
	Build()
```

#### SplitPayerSelfPay

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	AddIxesSig(*executor, executorSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *executorPk}). // ix0 + gas 一起签
	Build()
```

#### TransactionBuilder 方法一览

| 方法 | 说明 |
|------|------|
| `NewTransactionBuilder(instructions)` | 创建构建器，`Stamp` 取当前毫秒 |
| `WithPayer(addr)` | 指定统一付款方（UnifiedPayer） |
| `WithStamp(stamp)` | 指定交易时间戳 |
| `AddPayerSig(owner, sk, mode)` | 只授权 bit63（payer） |
| `AddIxAndPayerSig(owner, sk, ix, mode)` | 授权单条 ix + bit63 |
| `AddIxesSig(owner, sk, ixs, includePayer, mode)` | 授权多条 ix，`includePayer` 控制是否附带 bit63 |
| `AddSignature(owner, acSig)` | 追加已构建好的签名（如投票门控签名） |
| `ApplySlots(slots)` | 登记签名槽位（不含私钥） |
| `SimulateSlots()` | 为已登记槽位生成同长度占位签名 |
| `SignWith(signers...)` | 按地址匹配私钥，对已登记槽位真实签名 |
| `ResetSigs()` | 清空全部签名（占位签名 → 真实签名切换） |
| `Tx()` | 取底层 `*lib.Transaction` |
| `Build()` | 返回 `*lib.Transaction`，链式过程中首个错误会在此返回 |

> 复用同一个 builder 可保持 `Stamp` 不变，因此“模拟用交易”和“真实签名交易”的 `TxHash` 完全一致，详见 [上链交互](#上链交互提交--模拟--等待)。

### 签名模式

`AccountSignatureMode` 接口有两种实现：

| 模式 | 说明 |
|------|------|
| `PubKeySignatureMode{PublicKey, SkipPubKey, SigBit}` | 单公钥签名。`SkipPubKey=true` 时公钥不上链，`SigBit` 需通过 `Client.AccountSignerBit` 从链上 signers 列表解析 |
| `MultisigKeySignatureMode{Index, PublicKey}` | 多签账户参与者签名，链上按 `SigBit = 1 << Index` 定位 signer 槽位 |

`Client.AccountSignerBit(address)` 仅支持单签名者账户（bitmap 只有一位为 1），多签账户请直接使用 `MultisigKeySignatureMode`。

### 多签账户签名

同一 `AuthBit` 下，各参与者依次追加自己的签名，最后合成一个 `AccountSignature`：

```go
txHash := tx.TxHash()
ixPart := []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}} // 授权 ix0 时必须带入 ix hash

sig, err := lib.NewAccountSignatureBuilder().
	AuthorizeIxAndPayer(0). // 授权 ix0 + gas(bit63)
	SignMultisigKey(*account, sk0, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 0, PublicKey: *pk0}).
	SignMultisigKey(*account, sk1, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pk1}).
	Build()
if err != nil {
	panic(err)
}
tx.AddSignature(*account, *sig)
```

要点：

- `SignMultisigKey` 传入的 `ixPart` 必须与 `AuthBit` 中授权的 ix 对应（本例为 ix0），否则参与方签出的消息与链上重算结果不一致，验签失败
- `MultisigKeySignatureMode.Index` 是参与者在 signers 列表中的位置（0 起），并非权重
- 占位签名：`SimulateSignMultisigKey(mode)` 只需模式参数，签名长度按公钥类型自动确定，用于免私钥估 gas

`AccountSignatureBuilder` 方法一览：`AuthorizePayer()`、`AuthorizeIxAndPayer(ix)`、`AuthorizeIxes(indices)`、`Sign(...)`、`SimulateSign(...)`、`SignMultisigKey(...)`、`SimulateSignMultisigKey(mode)`、`Build()`。

### MIP-25 投票门控签名

多签投票通过后，执行交易可由“投票门控签名 + 赞助方”完成（对应 `example/create_multisig_vote`、`example/create_multisig_vote2`）：

```go
// 1. 投票门控签名：bit62 + 授权 ix 位，无公钥 / 无签名体
voteGateAuth, err := lib.AuthVoteIxes([]uint8{0})
if err != nil {
	panic(err)
}

// 2. 执行交易：relayer 付 gas，多签账户仅提供门控授权
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{transferWire}).
	WithPayer(relayer).
	AddPayerSig(*relayer, relayerSk, lib.PubKeySignatureMode{PublicKey: *relayerPk}).
	AddSignature(*accountMultiSig, lib.Unsigned(voteGateAuth)).
	Build()
```

- `lib.Unsigned(authBit)` 构造只含 `AuthBit` 的签名（`SigBit`/`Signatures`/`PubKey` 均为空）
- `AccountSignature.IsVoteGateOnly()` 可判定该签名是否为纯门控票
- `lib.VoteBatchHash(authBit, allIxHashes)` 计算投票意图哈希，需与链上 `vote_batch_hash` 一致

### 交易校验

`Transaction.ValidateWireWith(sponsorIx)` 在上链前校验 wire 层结构，`ValidateWire()` 等价于 `ValidateWireWith([]uint8{})`：

通用校验：

- 指令非空，且不超过 62 条（`AuthReservedBit`）
- 指令哈希不得重复
- 签名 owner（账户地址）不得重复
- `AuthBit` 不得为空，ix 位不得越界（bit62 / bit63 除外）

付款模式校验：

- **UnifiedPayer**（`tx.Payer != nil`）：payer 必须签名并授权 bit63，`sponsorIx` 在此模式下不生效
- **SplitPayerSelfPay**（`tx.Payer == nil`）：每条未赞助指令都需要有人同时授权 bit63 与该 ix；仅授权 bit63 属于付款模式冲突

`Client.SubmitTx` 内部调用 `ValidateWire`，`Client.SubmitTxWithSponsorIxes(tx, sponsorIxes)` 内部调用 `ValidateWireWith(sponsorIxes)`。

### 上链交互：提交 / 模拟 / 等待

#### 提交交易

```go
client.SubmitTx(tx)                                // ValidateWire 校验后提交
client.SubmitTxWithSponsorIxes(tx, []uint8{0})     // ValidateWireWith 校验后提交（可指定赞助指令）
```

传输方式：`SubmitTx` / `SimulateTx` 使用 `application/x-milon+postcard`，其余方法使用 `application/x-milon+json`（body 仍为 postcard 字节）。

#### 模拟执行（可选，估 gas）

`SimulateTx(tx)` 模拟交易执行并估算 gas。标准流程是“占位签名 → 模拟 → 真实签名”，且必须复用同一个 builder 以保证 `TxHash` 不变：

```go
txBuilder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(payer).
	ApplySlots([]lib.SigningSlot{
		{Address: *payer, Mode: lib.PubKeySignatureMode{PublicKey: *payerPk}},                              // 只签 gas
		{Address: *executor, InstructionIndices: []uint8{0}, Mode: lib.PubKeySignatureMode{PublicKey: *executorPk}},
	}).
	SimulateSlots()

simulateResult, err := client.SimulateTx(txBuilder.Tx()) // 免私钥估算 gas
helper.CheckSimulateSuccess(simulateResult)

txBuilder.ResetSigs().
	SignWith(lib.Signer{SecretKey: payerSk, PublicKey: *payerPk}, lib.Signer{SecretKey: executorSk, PublicKey: *executorPk})
tx, err := txBuilder.Build()
```

`SigningSlot.InstructionIndices` 为空表示只签 gas；`IncludePayer` 表示在 ix 签名中一并附带 bit63。

#### 等待确认

```go
result, err := client.WaitForTransaction(tx.TxHash())
```

`WaitForTransaction` 轮询 `GetTxByHash`，直到回执状态不是 `TxStatePending`；轮询期间的网络错误不会立即返回，而是记录最后一次错误继续重试。需要校验链上执行结果时可用 `helper.CheckTxSuccess(result.BodyTxHistory)`。

## IDL 类型系统

Provider 基于 IDL JSON 实现合约方法的序列化与反序列化。动态解码（`Provider.Decode`、`IDLRegistry.Decode*`）得到的值类型如下：

| IDL 类型 | Go 类型 | 说明 |
|----------|---------|------|
| `bool` / `boolean` | `bool` | 布尔值 |
| `u8` / `u16` / `u32` / `u64` | `uint8` / `uint16` / `uint32` / `uint64` | 无符号整数 |
| `i8` / `i16` / `i32` / `i64` | `int8` / `int16` / `int32` / `int64` | 有符号整数 |
| `u128` | `*big.Int` | 128 位整数（编码接受 `*big.Int` 及各类整型） |
| `Amount` / `Epoch` / `Bitmap64` | `uint64` | 链上语义别名，按 u64 处理 |
| `Address` / `Signer` / `AnySigner` | `*crypto.Address` | 20 字节地址 |
| `PublicKey` | `*crypto.PublicKey` | 变体号 + 定长字节 |
| `Signature` | `*crypto.Signature` | 变体号 + 定长字节（依公钥类型） |
| `String` / `string` | `string` | UTF-8 字符串 |
| `bytes` | `[]byte` | 字节数组 |
| `B96` / `B144` / `B160` / `B256` | `[12]byte` / `[18]byte` / `[20]byte` / `[32]byte` | 定长字节数组 |
| `vec<T>` | `[]any` | 变长数组 |
| `option<T>` | `nil` / 具体值 | 可选值 |
| `tuple<T1,T2,...>` | `[]any` | 元组 |
| `map<K,V>` | `map[any]any` | 映射 |
| struct | `map[string]any` | 字段名 → 值 |
| enum | `map[string]any` | 含 `variant` / `index`，带数据变体另含字段 |

`gen` 编码入口的类型化方法会把上述类型映射为具体 Go 签名（如 `Address` → `*crypto.Address`，`Vec<PublicKey>` → `[]*crypto.PublicKey`）。

## RPC 方法与 Client 封装

### MethodType 表

| 常量 | 值 | 使用方 |
|------|----|--------|
| `MethodTypeChainHead` | 1 | `Client.GetChainHead` |
| `MethodTypeSubmitTx` | 5 | `Client.SubmitTx` / `SubmitTxWithSponsorIxes` |
| `MethodTypeSimulateTx` | 10 | `Client.SimulateTx` |
| `MethodTypeBatchView` | 16 | `Client.View`（单条 / 多条统一走批量接口） |
| `MethodTypeGetAccount` | 20 | `Client.GetAccount` |
| `MethodTypeEventsByTxHash` | 25 | `Client.EventsByTxHash` |
| `MethodTypeGetBlockByHeight` | 50 | `Client.GetBlockByHeight` |
| `MethodTypeGetTxByHash` | 55 | `Client.GetTxByHash` / `WaitForTransaction` |
| `MethodTypeGetTxHistoryProof` | 60 | `Client.GetTxHistoryProof` |
| `MethodTypeGetResource` | 150 | `Client.GetResource` |
| `MethodTypeGetResourcePathByHash` | 155 | `Client.GetResourcePathByHash` |
| `MethodTypeBatchGetResourcePathByHash` | 160 | `Client.BatchGetResourcePathByHash` |
| `MethodTypeGetAccessValue` | 165 | `Client.GetAccessValue` |

> `lib` 中还定义了 `MethodTypeBatchSubmitTx`(6)、`MethodTypeBatchSimulateTx`(11)、`MethodTypeView`(15) 常量，SDK 当前未封装对应方法。

### Client 封装方法

`milon.NewClient(...)` 返回的 `*milon.Client` 可直接调用：

| 分类 | 方法 | 说明 |
|------|------|------|
| 账户 | `ClaimFaucet(sk, address, mode)` | 领取测试代币 |
| | `CreateAccount(sk, pk)` | 创建链上账户 |
| | `BalanceOf(account, token ...any)` | 查询 MIL 余额（`token` 变参当前保留，实现固定查 MIL） |
| | `ListAccountSigners(address)` | 查询账户签名者列表（`[账户信息, signers]`） |
| | `AccountSignerBit(address)` | 查询单签名者账户的槽位位图（`Bitmap64`） |
| | `TokenMetadata(token)` | 查询代币元数据（名称 / 符号 / 精度 / 图标 / URI） |
| 交易 | `SubmitTx(tx, opts...)` | `ValidateWire` 校验后提交 |
| | `SubmitTxWithSponsorIxes(tx, sponsorIxes, opts...)` | `ValidateWireWith` 校验后提交 |
| | `SimulateTx(tx, opts...)` | 模拟执行（估算 gas） |
| | `WaitForTransaction(txHashOrTxId, opts...)` | 轮询等待交易确认 |
| 链 | `GetChainHead(opts...)` | 查询链头 |
| | `GetBlockByHeight(height, opts...)` | 按高度查询区块 |
| | `GetTxByHash(txHashOrTxId, opts...)` | 按哈希 / TxId 查询交易 |
| | `GetTxHistoryProof(txHashOrTxId, opts...)` | 查询交易历史证明 |
| | `GetAccount(accountRelaxed, opts...)` | 查询账户详情 |
| | `EventsByTxHash(txHash, typeTagFilter, opts...)` | 查询交易事件（可按 type_tag 过滤，`nil` 表示不过滤） |
| 资源 | `GetResource(rsHash, opts...)` | 查询资源 |
| | `GetResourcePathByHash(rsHash, opts...)` | 按哈希查询资源路径 |
| | `BatchGetResourcePathByHash(rsHashList, opts...)` | 批量查询资源路径 |
| | `GetAccessValue(blobHashList, opts...)` | 查询外部访问值 |
| 合约 | `View(wires, opts...)` | 只读查询（view 指令列表） |
| 区块流 | `StreamBlocks(startBlock, onBlockDone)` | gRPC 订阅区块流 |
| 元数据 | `GetAllPd()` | 获取全部已加载 Provider（IDL 名 → Provider） |
| | `GetProviderManager()` | 获取 `*provider.IDLRegistry`（指令 / 事件 / 资源 / View 解码） |

结果结构体统一带 `HTTPResponseBody []byte`（原始响应体，供 `DecodeView` 等后续解码）与 `BodyXxx` 字段（已解析结构）。

## 请求选项（RequestOption / WaitOption）

所有 RPC 方法接受 `...RequestOption`，`WaitForTransaction` 接受 `...WaitOption`，两者均为类型安全的函数式选项。

### RequestOption（RPC 方法通用）

| 选项 | 说明 |
|------|------|
| `milon.WithContext(ctx)` | 设置 context，支持超时 / 取消 |
| `milon.WithRequestID(id)` | 自定义 request ID（类型 `lib.RequestID`；默认由毫秒时间戳 + 进程内序列号生成） |

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
client.GetChainHead(milon.WithContext(ctx))

client.GetTxByHash(txHash, milon.WithRequestID(12345))
```

### WaitOption（WaitForTransaction 专用）

| 选项 | 说明 |
|------|------|
| `milon.WithWaitContext(ctx)` | 设置 context，支持超时 / 取消 |
| `milon.WithWaitRequestID(id)` | 自定义 request ID |
| `milon.WithWaitPollPeriod(d)` | 轮询间隔（默认 1s） |
| `milon.WithWaitPollTimeout(d)` | 轮询超时（默认 30s） |

**1. 全局默认（NewClient 级别）**

```go
client := milon.NewClient(milon.DevNet,
	milon.WithClientPollPeriod(500*time.Millisecond),
	milon.WithClientPollTimeout(30*time.Second),
)
```

**2. 单次覆盖（WaitForTransaction 级别，优先级高于全局）**

```go
result, err := client.WaitForTransaction(txHash,
	milon.WithWaitPollPeriod(200*time.Millisecond),
	milon.WithWaitPollTimeout(20*time.Second),
	milon.WithWaitContext(ctx),
)
```

超时后错误会带上最后一次失败原因（`timeout after ..., last error: ...`）；context 取消时立即返回。

## 区块流（gRPC）

`Client.StreamBlocks` 从 `startBlock` 开始订阅节点区块流，逐块回调：

```go
err := client.StreamBlocks(0, func(blockHeight uint64, histories []*milon.FramedHistory) bool {
	for _, fh := range histories {
		fmt.Printf("block %d tx %s state=%d gas=%d\n",
			blockHeight, fh.TxHashBase58(), fh.State, fh.GasCharged)
	}
	return true // true: 自动 Ack 并继续；false: 停止订阅并正常返回 nil
})
```

- 连接地址取自 `Network.GrpcAddr`，未配置时返回错误
- `consumerID` 由 SDK 生成，保证进程内唯一；`max_in_flight` 固定为 64
- 单帧接收上限提升到 32MB；单笔交易解码失败会返回包装了 `milon.ErrTxDecode` 的错误（不静默跳过），未 Ack 的区块可在重连后重扫

`FramedHistory` 关键字段：`Version`、`Stamp`、`Payer`（签名数组下标，`nil` 表示分付）、`Signatures`、`Instructions`、`TxID`、`TxHash`、`State`、`Accesses`（before/after 快照）、`Events`、`ErrorCode`、`GasCharged`。

低层控制接口（可自管 ctx 与重连策略）同样可用：

```go
stream, err := milon.NewBlockStream(ctx, addr) // addr 形如 "127.0.0.1:50051"
err = stream.Subscribe("my-consumer", 0, 64)
frame, err := stream.Next()                    // *milon.StreamFrame
err = stream.Ack(frame.BlockHeight)
defer stream.Close()
```

## 高级用法

### 只读查询（View）

编码与反序列化统一走 `gen` 包：

```go
// 1. 编码：参数类型由 IDL 生成
wire, err := gen.Token.BalanceOf.Args(api.MILToken, account).Encode()
if err != nil {
	panic(err)
}

// 2. 查询
viewResult, err := client.View([]api.PackedInstruction{wire})
if err != nil {
	panic(err)
}

// 3. 按返回类型解码为具体 Go 类型
balance, err := gen.Token.BalanceOf.DecodeView(viewResult.HTTPResponseBody)
if err != nil {
	panic(err)
}
fmt.Printf("balance: %d\n", balance) // uint64
```

- `DecodeView` 传入 `client.View` 返回的原始响应体（`HTTPResponseBody`），返回类型跟随 IDL：`u64 → uint64`、`String → string`、struct → 生成的结构体（如 `gen.TokenMetadata`）
- `DecodeView` 自动处理 Ok/Err 分支，链上返回失败（`ApiFailure`）会转为 error

多条 view 一次查询 + 批量解码（结果按 `"appName::methodName"` 顺序对应）：

```go
viewResult, err := client.View([]api.PackedInstruction{wire1, wire2})

results, err := client.GetProviderManager().DecodeViewDatas(
	[]string{"token::BalanceOf", "demo::OrderBalance"},
	viewResult.HTTPResponseBody,
)
```

### 事件解析

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

`DecodeEventDataByTag` 按 type_tag 命中 IDL `events` 段；需要人读格式时用 `FormatDecodedEvent`。

### 资源解析

链上持久化资源在 IDL `resources` 段声明（如 `token:_Balance`、`account:_Account`），按资源 type_tag 解码：

```go
txResult, err := client.GetTxByHash(txHash)

for _, record := range txResult.BodyTxHistory.Receipt.Access {
	lastWritten := record.LastWritten
	if lastWritten.Variant != 0 { // 仅 Inline 值可直接解码
		continue
	}

	decoded, err := client.GetProviderManager().DecodeResourceDataByTag(
		lastWritten.TypeTag,
		lastWritten.InlineData,
	)
	if err != nil {
		continue
	}
	fmt.Printf("Resource: %+v\n", decoded) // 含 app_name / resource_name / resource_type / data
}
```

> - 多个资源可共享同一 type_tag（如多个 `u64` 计数器），此时按 IDL 中首个声明的资源解析
> - type_tag 未命中 `resources` 段时，自动回退到 IDL `types` 段按类型解码，覆盖创世交易等以裸值类型 tag 持久化的场景（如 `Address`、`B256` builtin tag），与 Rust SDK 的注册范围（resources ∪ types）一致
> - 区块流场景可先用 `milon.ResolveResourcePaths(client, rsHashSet)` 批量解析资源路径，再交给 `PrintFramedHistory` 展示

### 交易历史打印

```go
pathMap, err := milon.ResolveResourcePaths(client, rsHashSet) // 收集到的 RsHash 集合
milon.PrintFramedHistory(client, idx, fh, pathMap)             // 指令 / 资源 before-after / 事件
```

`PrintFramedHistory` 的 `logger` 为可选变参（`milon.FramedHistoryLogger` 接口，`*log.Logger` 直接满足），不传时输出到 stdout。

### 自定义 IDL

内置 IDL 已通过 `gen.DefaultIDLs` 内嵌，`NewClient` 自动加载并绑定。需要在 SDK 之外独立使用 IDL 做编解码时：

```go
pd, err := provider.LoadProviderFromFile("./custom_idl/token.idl.json")
// 或从内存构造：pd := provider.NewProvider(idlData)

wire, err := pd.Encode("BalanceOf", provider.Args{"token": api.MILToken, "account": account})
// wire 为 []byte，可直接作为 api.PackedInstruction 使用
```

## 网络配置

```go
// 本地开发网络
client := milon.NewClient(milon.LocalNet)
//   RpcUrl:   http://127.0.0.1:6280/v1/rpc
//   GrpcAddr: 127.0.0.1:50051
//   ChainId:  900_000_001

// 开发测试网络
client := milon.NewClient(milon.DevNet)
//   RpcUrl:   https://devnet.milonlabs.com/v1/rpc
//   GrpcAddr: 8.218.101.239:50051
//   ChainId:  900_000_001

// 自定义网络
client := milon.NewClient(milon.Network{
	Name:     "myNet",
	ChainId:  900_000_001,
	RpcUrl:   "http://127.0.0.1:6280/v1/rpc",
	InxUrl:   "",                  // 可选
	GrpcAddr: "127.0.0.1:50051",   // 区块流使用，留空则 StreamBlocks 报错
})
```

> `NewClient` 会调用 `lib.SetChainId(config.ChainId)` 写入全局链 ID（影响交易哈希与签名域），同一进程内不要并发使用不同 `ChainId` 的客户端。

## 示例索引

`example/` 下每个目录都是可独立运行的完整示例：

| 目录                                 | 内容 |
|------------------------------------|------|
| `account_create_four_crypto`       | 用四种密码学算法创建账户 |
| `account_demo`                     | 账户创建与多签管理（AddSigner / SetThreshold / SetSignerWeight / RemoveSigner） |
| `chainhead_block`                  | 查询链头与按高度查询区块 |
| `create_multisig_demo`             | 多签账户创建 + 多签转账（含占位签名模拟 → 真实签名） |
| `create_multisig_vote`             | 多签投票与执行（拆分付款） |
| `create_multisig_vote2`            | 多签投票与执行（统一付款） |
| `getTxHistoryProof`                | 查询交易历史证明 |
| `grpc_demo`                        | gRPC 区块流订阅（StreamBlocks + PrintFramedHistory） |
| `identity_demo`                    | 链上身份：DID / 机构注册 / VC 合规 |
| `identity_test`                    | 身份合约多场景：VC 冻结 / 披露 / 签发者校验 |
| `multi_ix_demo`                    | 单交易多条指令 |
| `pubkey_signature_mode_demo`       | `SkipPubKey` 签名模式 |
| `sft_demo`                         | SFT 完整生命周期：create_sft / slot / mint / split / transfer / approve / transfer_from / freeze / merge |
| `sft_demo2`                        | SFT 简单场景：一次 mint，同一 token_id 由三人共持 |
| `token_create_split`               | 建币（SplitPayerSelfPay 付款模式） |
| `token_create_unified_separate_ix` | 建币（UnifiedPayerSeparateIx 付款模式） |
| `token_create_unified_sign_all`    | 建币（UnifiedPayerSignAll 付款模式） |
| `token_demo`                       | 代币创建 / 增发 / 转账 / 查询余额 |
| `view_demo`                        | 只读查询与批量解码 |
| `debug`                            | 调试用指令（EchoMode / EnsureAccount / Create / Mint） |

运行方式：

```bash
go run ./example/create_multisig_demo
```
