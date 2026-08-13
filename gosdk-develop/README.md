# Milon Go SDK

Milon 区块链的 Go 语言 SDK，提供合约交互（IDL 编解码）、交易构建与签名（四种付款模式）、RPC 通信、事件解析等功能。

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
	// 1. 配置网络
	client := milon.NewClient(milon.LocalNet)

	// 2. 创建密钥对
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk := sk.Ed25519Public()
	address, _ := crypto.NewAddressFromPublicKey(pk)

	// 3. 领取测试代币
	err := client.ClaimFaucet(sk, address, lib.PubKeySignatureMode{PublicKey: *pk})
	if err != nil {
		panic(err)
	}

	// 4. 查询余额
	balance, _ := client.BalanceOf(address)
	fmt.Printf("balance: %d\n", balance)
}
```

## 包结构

```text
├── client.go              客户端入口: NewClient, 统一封装 RPC 调用
├── network.go             网络配置 (LocalNet / DevNet)
├── rpcClientV1.go         RPC 客户端实现 (HTTP 通信、IDL 加载、交易提交)
│
├── api/                   RPC 响应反序列化结构体
│   ├── base.go            基础类型 (TxHash, RsHash, PersistedValue 等)
│   ├── block.go           区块相关结构体
│   ├── chainHead.go       链头结构体
│   ├── eventsByTxHash.go  事件查询结构体 (EventEntry, TypeTagWithData)
│   ├── getAccessValue.go  访问值查询结构体
│   ├── getResource.go     资源查询结构体
│   ├── listResourcePath.go 资源路径查询结构体
│   ├── simulateReceipt.go 模拟执行回执结构体
│   ├── txHistory.go       交易历史结构体
│   └── txHistoryView.go   交易历史视图结构体
│
├── lib/                   交易构建与签名
│   ├── transaction.go     交易结构体 (Transaction, TxHash, ValidateWireWith)
│   ├── transactionBuilder.go 链式构建器 (NewTransactionBuilder, SigningSlot, Signer)
│   ├── accountSignature.go 账户签名 (AccountSignature, 签名模式, AuthBit 工具)
│   └── accountSignatureBuild.go 签名链式构建器 (NewAccountSignatureBuilder)
│
├── crypto/                密码学相关
│   ├── address.go         地址生成与解析
│   ├── hash_domain.go     哈希域名常量
│   ├── secretkey.go       私钥生成与签名 (Ed25519/Secp256k1/BLS12381/FnDsa512)
│   ├── publickey.go       公钥管理
│   ├── signature.go       签名结构体
│   └── fn_dsa512.go       FnDSA-512 密码学实现
│
├── provider/              合约 IDL 加载与指令/事件编解码
│   ├── IDL/               内置 IDL JSON (account/token/staking/identity/nft/demo...)
│   ├── provider.go        Provider: IDL 加载、指令编码(Encode)、值序列化/反序列化
│   ├── registry.go        IDLRegistry: 多 IDL 统一注册、指令/事件解码与格式化
│   ├── idlTypeResolver.go 基于 type_tag 的动态类型解析器 (DecodeResource/DecodeEvent)
│   └── types.go           IDL 数据结构定义
│
├── gen/                   由 tools/idlgen 生成的 IDL 应用对象 (gen.Token.xxx.Args(...).Encode())
├── helper/                常用辅助函数 (CheckTxSuccess, DisplayGetAccount 等)
├── postcard/              与链约定的序列化/反序列化协议
├── types/                 内部数据结构 (bitbap.go: Bitmap64 64 位位图)
└── tools/                 idlgen: IDL → Go 代码生成器
```

## 核心概念

### 合约调用：gen 编码

所有合约调用统一通过 `gen` 包（由 IDL 生成的类型安全入口）编码：

- **上链的编辑指令**（写入链上状态，需签名后提交交易）：

```go
wire, err := gen.Token.Create.Args().Encode()
```

- **读取方法**（只读查询，无需签名）：

```go
wire, err := gen.Token.BalanceOf.Args(token, account).Encode()
```

### 付款模式

Milon 交易的 `AuthBit`（64 位授权位图）布局：

- **bit 0-61**：指令授权位，第 i 条指令对应 bit i（单笔交易最多 62 条指令）
- **bit 62**：保留，不可用
- **bit 63**：gas 授权位（payer 位）

SDK 支持四种付款模式：

| 模式 | 说明 | 典型场景 |
|------|------|----------|
| **UnifiedPayerGasOnly** | payer 只签 gas（bit63），ix 无需签名 | 纯赞助（payer 非操作者） |
| **UnifiedPayerSignAll** | payer 在一个签名中同时覆盖 ix bit(s) 和 gas bit（bit63） | payer 即唯一操作者 |
| **UnifiedPayerSeparateIx** | payer 只签 gas（bit63），ix 由独立 executor 账户签名 | 赞助方 + 执行方分离 |
| **SplitPayerSelfPay** | 无 payer，每个 executor 各自签自己的 ix bit(s) 和 gas bit（bit63） | 多操作者各自付账 |

#### UnifiedPayerGasOnly

payer 只签 gas（bit63），ix 无需签名（纯赞助）。

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(payer). // 指定 payer
	AddPayerSig(*payer, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPk}). // 只签 gas(bit63)
	Build()
```

#### UnifiedPayerSignAll

payer 在一个签名中同时签 ix bit(s) 和 gas bit（bit63）。

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(account).
	AddIxAndPayerSig(*account, accountSk, 0, lib.PubKeySignatureMode{PublicKey: *accountPk}). // 一次签名覆盖 ix0 + gas
	Build()
```

#### UnifiedPayerSeparateIx

payer 只签 gas（bit63），ix 由独立 executor 账户签名。

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(payer).
	AddPayerSig(*payer, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPk}). // payer 只签 gas
	AddIxesSig(*executor, executorSk, []uint8{0}, false, lib.PubKeySignatureMode{PublicKey: *executorPk}). // executor 只签 ix0
	Build()
```

#### SplitPayerSelfPay

无 payer，每个 executor 各自签自己的 ix bit(s) 和 gas bit（bit63）。

```go
tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	AddIxesSig(*executor, executorSk, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *executorPk}). // ix0 + gas 一起签
	Build()
```

### 签名模式

`AccountSignatureMode` 接口的两种实现：

| 模式 | 说明 |
|------|------|
| `PubKeySignatureMode{PublicKey, SkipPubKey, SigBit}` | 单公钥签名。`SkipPubKey=true` 时省略公钥上链，`SigBit` 需通过 `Client.AccountSignerBit` 从链上 signers 列表解析 |
| `MultisigKeySignatureMode{Index, PublicKey}` | 多签账户参与者签名，公钥按 `SigBit = 1 << Index` 在链上 signers 列表中定位 |

### 交易流程

```
1. Encode  → gen.xxx.Args(...).Encode() 或 Provider.Encode() 将合约调用编码为字节
2. Build   → NewTransactionBuilder(instructions) + WithPayer + 按付款模式添加签名
3. Simulate → client.SimulateTx (可选，可先用 SimulateSlots 占位签名估 gas)
4. Submit  → client.SubmitTx
5. Wait    → client.WaitForTransaction
```

模拟签名（不接触私钥）→ 真实签名：

```go
txBuilder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
	WithPayer(payer).
	ApplySlots([]lib.SigningSlot{ // 声明各账户授权范围
		{Address: *payer, IncludePayer: true, Mode: lib.PubKeySignatureMode{PublicKey: *payerPk}},
		{Address: *executor, InstructionIndices: []uint8{0}, Mode: lib.PubKeySignatureMode{PublicKey: *executorPk}},
	}).
	SimulateSlots() // 零值占位签名，wire 尺寸与真实签名一致

client.SimulateTx(txBuilder.Tx()) // 精确估算 gas

txBuilder.ResetSigs(). // 清掉占位签名
	SignWith(lib.Signer{SecretKey: payerSk, PublicKey: *payerPk}, lib.Signer{SecretKey: executorSk, PublicKey: *executorPk}) // 真实签名
tx, err := txBuilder.Build()
```

### 交易校验

`Transaction.ValidateWireWith(sponsorIx)` 在上链前校验 wire 层结构：指令数/哈希、签名 owner 去重、AuthBit 越界，并按付款模式校验 gas 签名：

- **UnifiedPayer**（`tx.Payer != nil`）：payer 必须签名并授权 bit63
- **SplitPayerSelfPay**（`tx.Payer == nil`）：每条未赞助指令都需要有人同时授权 bit63 与该 ix；只授权 bit63 属于付款模式冲突

## IDL 类型系统

Provider 基于 IDL JSON 实现合约方法的序列化与反序列化，支持以下类型：

| IDL 类型 | Go 类型 | 说明 |
|----------|---------|------|
| `bool` / `boolean` | `bool` | 布尔值 |
| `u8` / `u16` / `u32` / `u64` | `uint8/16/32/64` | 无符号整数 |
| `i8` / `i16` / `i32` / `i64` | `int8/16/32/64` | 有符号整数 |
| `u128` | `uint64` | 128 位整数（实际按 uint64 处理） |
| `Address` / `Signer` | `crypto.Address` | 20 字节地址 |
| `PublicKey` | `crypto.PublicKey` | 公钥（含 variant + 变长字节）|
| `String` / `string` | `string` | 字符串 |
| `bytes` | `[]byte` | 字节数组 |
| `Bitmap64` | `uint64` | 64 位位图 |
| `B96` / `B144` / `B160` / `B256` | `[N]byte` | 固定长度字节数组 |
| `vec<T>` | `[]any` | 变长数组 |
| `option<T>` | `any` / `nil` | 可选值 |
| `tuple<T1,T2,...>` | `[]any` | 元组 |
| `map<K,V>` | `map[any]any` | 映射 |
| struct / enum | `map[string]any` | 自定义结构体/枚举 |

## RPC 方法

| 方法 | MethodType | 说明 |
|------|-----------|------|
| `ChainHead` | 1 | 查询链头 |
| `SubmitTx` | 2 | 提交交易 |
| `SimulateTx` | 3 | 模拟执行 |
| `View` | 4 | 只读查询 |
| `GetResource` | 5 | 查询资源 |
| `GetBlockByHeight` | 6 | 按高度查询区块 |
| `GetTxByHash` | 7 | 按哈希查询交易 |
| `GetAccount` | 8 | 查询账户 |
| `EventsByTxHash` | 9 | 查询交易事件 |
| `ListResourcePath` | 10 | 资源路径列表 |
| `GetResourcePathByHash` | 11 | 按哈希查询资源路径 |
| `GetAccessValue` | 12 | 查询外部访问值 |
| `BatchGetResourcePathByHash` | 13 | 批量查询资源路径 |

### Client 封装方法（可直接调用）

`milon.NewClient(...)` 返回的 `*milon.Client` 已封装全部 RPC 方法，直接调用即可：

| 分类 | 方法 | 说明 |
|------|------|------|
| 账户 | `ClaimFaucet(sk, address, mode)` | 领取测试代币 |
| | `CreateAccount(sk, pk)` | 创建链上账户 |
| | `BalanceOf(address)` | 查询账户余额 |
| | `ListAccountSigners(address)` | 查询账户签名者列表 |
| | `AccountSignerBit(address)` | 查询签名者槽位位图（Bitmap64） |
| 交易 | `SubmitTx(tx)` | 提交交易上链 |
| | `SimulateTx(tx)` | 模拟执行（估算 gas） |
| | `WaitForTransaction(txHash)` | 轮询等待交易确认 |
| 链 | `GetChainHead()` | 查询链头 |
| | `GetBlockByHeight(height)` | 按高度查询区块 |
| | `GetTxByHash(txHash)` | 按哈希查询交易 |
| | `GetAccount(address)` | 查询账户详情 |
| | `EventsByTxHash(txHash, typeTagFilter)` | 查询交易事件（可按 type_tag 过滤） |
| 资源 | `GetResource(rsHash)` | 查询资源 |
| | `ListResourcePath()` | 资源路径列表 |
| | `GetResourcePathByHash(rsHash)` | 按哈希查询资源路径 |
| | `BatchGetResourcePathByHash(rsHashList)` | 批量查询资源路径 |
| | `GetAccessValue(blobHashList)` | 查询外部访问值 |
| 合约 | `View(wires)` | 只读查询（view 指令列表） |
| 元数据 | `GetAllPd()` | 获取全部已加载的 Provider |
| | `GetProviderManager()` | 获取 IDLRegistry（指令/事件解码） |

## 高级用法

### 多签账户

```go
// 每个参与者都在同一授权下追加自己的签名
sig, err := lib.NewAccountSignatureBuilder().
	AuthorizeIxAndPayer(0). // 授权 ix0 + gas
	SignMultisigKey(*account, sk0, tx.TxHash(), nil, lib.MultisigKeySignatureMode{Index: 0, PublicKey: *pk0}).
	SignMultisigKey(*account, sk1, tx.TxHash(), nil, lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pk1}).
	Build()
tx.AddSignature(*account, *sig)
```

### 事件解析

SDK 自动将链上事件反序列化为结构化数据：

```go
eventsResult, _ := client.EventsByTxHash(txHash, nil)

for _, event := range eventsResult.BodyEventsByTxHash.Events {
	decoded, _ := client.GetProviderManager().DecodeEventDataByTag(
		event.Data.TypeTag,
		event.Data.Value,
	)
	fmt.Printf("Event: %+v\n", decoded)
}
```

### 只读查询（View）

**编码与反序列化统一走 `gen` 包**（由 IDL 生成的类型安全入口）：

```go
// 1. 编码：gen.xxx.xxx.Args(...).Encode() 生成 PackedInstruction（参数类型由 IDL 生成，编译期检查）
wire, err := gen.Token.BalanceOf.Args(token, account).Encode()
if err != nil {
	panic(err)
}

// 2. 查询：client.View 提交只读查询
viewTxResult, err := client.View([]api.PackedInstruction{wire})
if err != nil {
	panic(err)
}

// 3. 反序列化：gen.xxx.xxx.DecodeView() 读取 View 返回的数据，按返回类型解码为具体 Go 类型
balance, err := gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
if err != nil {
	panic(err)
}
fmt.Printf("balance: %d\n", balance) // balance 为 uint64
```

**要点**：

- 编码统一用 `gen.xxx.xxx.Args(...).Encode()`，参数类型由 IDL 生成，编译期即可发现错误
- `DecodeView` 传入 `client.View` 返回的原始响应体（`HTTPResponseBody`），返回类型跟随 IDL：`u64 → uint64`、`String → string`、struct → 对应结构体
- `DecodeView` 自动处理 Ok/Err 分支：链上返回失败（`TxFailurePayload`）会转为 error

多条 view 一次查询 + 批量解码（结果按 `"appName::methodName"` 列表顺序对应）：

```go
wires := []api.PackedInstruction{wire1, wire2}
viewResult, err := client.View(wires)

results, err := client.GetProviderManager().DecodeViewDatas(
	[]string{"token::BalanceOf", "demo::OrderBalance"},
	viewResult.HTTPResponseBody,
)
```

### 自定义 IDL

SDK 内置 IDL 已通过 `gen.DefaultIDLs` 内嵌，`NewClient` 自动加载并绑定。如需使用自定义 IDL：

```go
pd, err := provider.LoadProviderFromFile("./custom_idl/token.idl.json")
// 或
pd := provider.NewProvider(idlData)
```

### 轮询配置

`WaitForTransaction` 的轮询参数可通过 ClientOption 调整，默认每 1 秒轮询一次、超时 10 秒：

```go
client := milon.NewClient(milon.DevNet,
	milon.WithPollPeriod(milon.PollPeriod(500*time.Millisecond)),  // 每 500ms 轮询一次
	milon.WithPollTimeout(milon.PollTimeout(30*time.Second)),     // 30s 超时
)
```

`PollPeriod` / `PollTimeout` 是命名类型（`type PollPeriod time.Duration`），传参时需要显式转换。

## 网络配置

```go
// 本地开发网络
client := milon.NewClient(milon.LocalNet)

// 开发测试网络
client := milon.NewClient(milon.DevNet)

// 自定义网络
client := milon.NewClient(milon.Network{
	Name:    "myNet",
	ChainId: 900_000_001,
	RpcUrl:  "http://127.0.0.1:6280/milon/v1",
})
```
