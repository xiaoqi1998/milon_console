# Milon Go SDK

Milon 区块链的 Go 语言 SDK，提供合约交互、交易构建与签名、RPC 通信等功能。

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
	"github.com/milon-labs/milon-go-sdk/api" 
	"github.com/milon-labs/milon-go-sdk/crypto" 
)

func main() {
    // 1. 配置网络 
    client := milon.NewMilonClient(milon.LocalNetConfig)
	
    // 2. 创建密钥对
    sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
    pk := sk.Ed25519Public()
    address, _ := crypto.NewAddressFromPublicKey(pk)
    
    // 3. 领取测试代币
    client.ClaimFaucet(sk, *address, milon.PubKeySignatureMode{PublicKey: *pk})
    
    // 4. 查询余额
    balance, _ := client.AddressBalance(*address)
    fmt.Printf("balance: %d\n", balance)
}
```

## 包结构

```text
    ├── api/ RPC 响应反序列化结构体
    │   ├── base.go 基础类型 (TxHash, RsHash, PersistedValue 等)
    │   ├── block.go 区块相关结构体
    │   ├── chainHead.go 链头结构体
    │   ├── eventsByTxHash.go 事件查询结构体
    │   ├── getAccessValue.go 访问值查询结构体
    │   ├── getResource.go 资源查询结构体
    │   ├── listResourcePath.go 资源路径查询结构体
    │   ├── rpcResponse.go RPC 响应反序列化
    │   ├── simulateReceipt.go 模拟执行回执结构体
    │   ├── txHistory.go 交易历史结构体
    │   └── txHistoryView.go 交易历史视图结构体
    │
    ├── crypto/ 密码学相关
    │   ├── address.go 地址生成与解析
    │   ├── hash_domain.go 哈希域名常量
    │   ├── secretkey.go 私钥生成与签名 (Ed25519/Secp256k1/BLS/FnDSA)
    │   ├── publickey.go 公钥管理
    │   ├── signature.go 签名结构体
    │   └── fn_dsa512.go FnDSA-512 密码学实现
    │
    ├── example/ 使用示例
    │   ├── token_create_mint_transfer_multi/ Token 创建 + Mint + Transfer + Batch View
    │   ├── token_create_unified_payer_sign_all_oneShot/ 统付模式: payer 签所有 ix
    │   ├── token_create_unified_dual_sign_oneShot/ 统付模式: payer + ix 各自签名
    │   ├── token_create_unified_dual_sign_step_by_step/ 统付双签(分步)
    │   ├── token_create_split_oneShot/ 分账模式: 各自签名自己的 ix
    │   ├── transfer_mil/ 转账 MIL
    │   ├── account_create_four_Crypto/ 创建四种密码学类型的账户
    │   ├── account_create_unified_payer_sign_gas_oneShot/ 统付模式: payer 仅付 gas
    │   ├── demo_initPool_batchCredit_oneShot/ Demo 池初始化 + 批量授信
    │   ├── chanhead_block/ 查询链头与区块
    │   └── listResourcePath_getResourcePathByHash/ 查询资源路径
    │
    ├── postcard/ 与链约定的序列化/反序列化协议
    │   ├── serializer.go 序列化器
    │   ├── deserializer.go 反序列化器
    │   ├── postcard.go 通用接口 (Marshaler/Unmarshaler, Seq, Option 等)
    │   └── postcard_test.go 编解码测试
    │
    ├── provider/ 合约 IDL 加载与指令编解码
    │   ├── IDL/ 合约 IDL JSON 定义
    │   │   ├── index.json IDL 索引文件
    │   │   ├── account.idl.json
    │   │   ├── token.idl.json
    │   │   ├── staking.idl.json
    │   │   ├── identity.idl.json
    │   │   ├── nft.idl.json
    │   │   └── demo.idl.json
    │   ├── provider.go Provider: IDL 加载、指令编码(Encode)、值序列化/反序列化
    │   ├── manager.go IDLManager: 多 IDL 统一管理、指令/事件解码
    │   ├── idlTypeResolver.go 基于 type_tag 的动态类型解析器
    │   └── types.go IDL 数据结构定义
    │
    ├── types/ 内部数据结构
    │   └── bitbap.go Bitmap64: 64 位位图
    │
    └── tools/
    │   └── http.go HTTP 请求工具
    │
    ├── accountSignature.go 账户签名模块 (AccountSignature, 签名模式, AuthMessage)
    ├── client.go MolinClient 客户端，统一封装 RPC 调用
    ├── config.go 网络配置 (LocalNet / DevNet)
    ├── rpcClientV1.go RPC 客户端实现 (HTTP 通信、IDL 加载、交易构建)
    ├── submitTransaction.go 提交交易结构体 (SubmitTransaction, MethodType)
    ├── transaction.go 交易结构体 (Transaction, 签名流程)
```


## 核心概念

### 交易模式

SDK 支持四种交易模式：

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **统付全签 (UnifiedPayerSignAll)** | 一个 payer 支付 gas，payer 签署所有 ix 的 hash | payer 即唯一操作者 |
| **统付双签 (UnifiedDualSign)** | 一个 payer 支付 gas，payer 签 payer 位 + ix owner 签各自的 ix | payer 与操作者分离 |
| **统付仅 Gas (UnifiedPayerOnlyGas)** | 一个 payer 仅支付 gas，不签名任何 ix | 赞助 gas 场景 |
| **分账 (Split)** | 无 payer，每个 ix 由各自的 owner 签名 | 多操作者独立签名 |

### 签名模式

| 模式 | 说明 |
|------|------|
| `PubKeySignatureMode` | 单公钥签名（直接传入公钥） |
| `MultisigKeySignatureMode` | 多签密钥签名（指定 key index + 公钥） |

### 交易流程

```
1.Encode → 通过 Provider.Encode() 将合约调用编码为字节
2.Build → 组装 Transaction 结构体 (Stamp, Payer, Instructions)
3.Sign → 对交易进行签名 (SignPayer / SignIx)
4.Simulate → 模拟执行 (可选)
5.Submit → 提交上链
6.Wait → 等待交易确认

```

### IDL 类型系统

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

### RPC 方法

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

## 高级用法

### 自定义 IDL 路径

默认 IDL 文件通过 `//go:embed` 嵌入在二进制中。如果需要使用自定义 IDL：

```
    go client := milon.NewMilonClient( milon.DevNetConfig, milon.WithIDLPath("./custom_idl/index.json"), )
```


### 多签账户

```go
    //创建多签签名 
    sig, err := accountSignature.NewAccountSignature(index, signature) 
	
    // 追加多签 key	
    sig.Add(keyIndex, anotherSignature)
	
    // 多签签名模式
    mode := milon.MultisigKeySignatureMode{ Index: 0, PublicKey: *pk, }

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

## 网络配置

```go
    // 本地开发网络
    client := milon.NewMilonClient(milon.LocalNetConfig)
	
    // 开发测试网络
    client := milon.NewMilonClient(milon.DevNetConfig)
```