# Milon API Server 接口文档

## 概述

Milon API Server 将 Milon Go SDK 封装为一组 RESTful HTTP 接口，提供网络管理、账户管理、交易查询与提交、合约读写、RPC 访问、水龙头领水、密钥工具以及 IDL 元数据发现等能力，共计 **39** 个端点。

- **Base URL**: `http://localhost:8080`
- **默认端口**: `8080`（可通过环境变量 `SERVER_PORT` 修改）
- **默认网络**: `devNet`（可通过环境变量 `DEFAULT_NETWORK` 修改，支持 `devNet`、`localNet`）
- **Content-Type**: `application/json`（POST 请求需携带）
- **CORS**: 默认允许所有来源（可通过环境变量 `ALLOWED_ORIGINS` 配置）

### 环境变量配置

| 变量名 | 默认值 | 说明 |
| --- | --- | --- |
| `SERVER_PORT` | `8080` | 服务监听端口 |
| `ALLOWED_ORIGINS` | `*` | 允许的跨域来源 |
| `ENABLE_UTIL_SIGN` | `false` | 是否启用 `/api/util/sign` 签名接口 |
| `DEFAULT_NETWORK` | `devNet` | 默认网络 |
| `MILON_RPC_URL` | (空) | 自定义 RPC 地址 |
| `MILON_CHAIN_ID` | `0` | 自定义链 ID |

---

## 通用说明

### 统一响应格式

所有接口均返回统一的 JSON 结构，包含以下字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `success` | bool | 请求是否成功 |
| `code` | int | 业务状态码，`0` 表示成功，其余为错误码 |
| `message` | string | 状态描述信息 |
| `data` | any | 响应数据，失败时可能为错误详情 |
| `timestamp` | string | 服务器时间戳（RFC3339 格式） |

**成功响应示例：**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "chainId": 2,
    "blockHeight": 12345
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

**失败响应示例：**

```json
{
  "success": false,
  "code": 400,
  "message": "invalid request body",
  "data": "EOF",
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

### 支付模式说明

合约模拟（`/api/simulate`）与写入（`/api/write`）接口支持 6 种支付模式，通过 `paymentMode` 字段指定：

| 支付模式 | 值 | 说明 | 适用场景 |
| --- | --- | --- | --- |
| 统一支付-全签 | `unified_payer_all` | 付款方支付 gas 并签署所有指令 | 单一账户支付并签名 |
| 统一支付-双签 | `unified_dual_sign` | 付款方支付 gas，指令账户单独签名 | 付款方与指令发起方不同 |
| 统一支付-仅 gas | `unified_payer_only_gas` | 付款方仅支付 gas，不签署指令 | 指令由其他方式签名 |
| 拆分支付 | `split` | 所有者（owner）同时支付 gas 并签署指令 | 多签账户场景 |
| 多签名者 | `multi_signer` | 同一指令被多个账户签 bit0（指令执行授权） | NFT CreateUnique/CreateBatch、Staking CreateValidator 等多签场景 |
| 赞助交易 | `sponsored` | 用于 IDL 中标记 `sponsor: true` 的方法，gas 由链上赞助池支付 | demo.SponsorWithoutRegister、demo.ClaimSponsoredScore 等 |

### signatureMode 格式

`signatureMode` 用于指定账户签名方式，支持以下两种格式：

**公钥签名模式（pubkey）：**

```json
{
  "type": "pubkey",
  "publicKey": "<base58 或 hex 公钥>"
}
```

**多签模式（multisig）：**

```json
{
  "type": "multisig",
  "index": 2,
  "publicKey": "<base58 或 hex 公钥>"
}
```

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `type` | string | 是 | 签名类型：`pubkey` 或 `multisig` |
| `publicKey` | string | 是 | 公钥（hex 或 base58 编码） |
| `index` | number | multisig 模式必填 | 多签账户中的索引位置 |

**⚠️ 公钥模式（pubkey）的使用限制**

`type: "pubkey"` 只在**账户尚未在链上注册**时可用（首笔交易 / 隐式开户）。账户一旦已在链上注册（`GET /api/accounts/:address` 能返回 `data.PublicKeysBs58`），链端会拒绝公钥模式签名：

```
API returned error status 6: {Message:Account <addr> exists; pubkey mode not allowed Code:<nil> Data:<nil>}
```

对应链上 account 模块错误 `285 PubkeyModeForbidden`。此时必须改用 **multisig（签名者列表模式）**：线格式只带 `SigBit = 1<<index`、不携带公钥，链端按账户登记的 signers 列表定位公钥。

- `index` = 该公钥在账户链上 signers 位图中的位置，**单密钥账户为 `0`**。
- 获取方式：`GET /api/accounts/:address` 返回的 `PublicKeysBs58` 数组下标，或 `account::list_signers` 返回的 `bitmap`/signers 列表（SDK：`client.AccountSignerBit(addr)`）。
- 该限制同时作用于 ix 签名者与 payer（gas 付款方），每个签名账户各自判定。

### Gas 费用说明

- 交易回执（`receipt`）中包含 `gasCharged` 字段，表示该笔交易实际消耗的 gas 费用。
- 模拟交易（`/api/simulate`、`/api/transactions/simulate`）的返回结果同样透传 `gasCharged`，供调用方预估费用。
- 对于 sponsored（赞助）交易，`gasCharged` 为 `0`。

### multi_signer 多签名者模式

多签名者模式 - 同一指令被多个账户签 bit0（指令执行授权）。适用于 NFT CreateUnique/CreateBatch（mint + owner 双签）和 Staking CreateValidator（operator + consensus_account 双签）等场景。

**请求字段**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `appName` | string | 是 | 应用名称 |
| `methodName` | string | 是 | 方法名称 |
| `args` | object | 否 | 方法参数键值对 |
| `paymentMode` | string | 是 | 必须为 `multi_signer` |
| `signers` | array | 是 | 签名者列表，每项含 `{address, privateKey, signatureMode}` |
| `signers[].address` | string | 是 | 签名者地址（base58） |
| `signers[].privateKey` | string | 是 | 签名者私钥（hex 或 base58） |
| `signers[].signatureMode` | object | 是 | 签名者签名模式 |
| `gasPayer` | object | 否 | 独立 gas 付款方，含 `{address, privateKey, signatureMode}` |
| `gasPayer.address` | string | gasPayer 存在时必填 | gas 付款方地址（base58） |
| `gasPayer.privateKey` | string | gasPayer 存在时必填 | gas 付款方私钥（hex 或 base58） |
| `gasPayer.signatureMode` | object | gasPayer 存在时必填 | gas 付款方签名模式 |

**签名规则**

- **提供 `gasPayer` 时**：gasPayer 签 bit63（gas），所有 `signers` 仅签 bit0。
- **未提供 `gasPayer` 时**：`signers[0]` 同时签 bit63 + bit0，其余 signers 仅签 bit0。

**请求示例（NFT CreateUnique，含独立 gasPayer）**

```json
{
  "appName": "nft",
  "methodName": "CreateUnique",
  "args": {
    "collection": "<base58 collection address>",
    "mint": "<base58 mint address>",
    "to": "<base58 recipient address>",
    "metadata": {...},
    "royalty": {...}
  },
  "paymentMode": "multi_signer",
  "signers": [
    {
      "address": "<base58 mint address>",
      "privateKey": "<hex mint private key>",
      "signatureMode": {"type": "pubkey", "publicKey": "<base58 mint public key>"}
    },
    {
      "address": "<base58 owner address>",
      "privateKey": "<hex owner private key>",
      "signatureMode": {"type": "pubkey", "publicKey": "<base58 owner public key>"}
    }
  ],
  "gasPayer": {
    "address": "<base58 gas payer address>",
    "privateKey": "<hex gas payer private key>",
    "signatureMode": {"type": "pubkey", "publicKey": "<base58 gas payer public key>"}
  }
}
```

### sponsored 赞助交易模式

赞助交易模式 - 用于 IDL 中标记 `sponsor: true` 的方法（如 demo.SponsorWithoutRegister、demo.ClaimSponsoredScore）。ix=0 被标记为赞助指令，gas 由链上赞助池支付。

**请求字段**

与 `unified_payer_all` 一致：

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `appName` | string | 是 | 应用名称 |
| `methodName` | string | 是 | 方法名称（须为 IDL 中 `sponsor: true` 的方法） |
| `args` | object | 否 | 方法参数键值对 |
| `paymentMode` | string | 是 | 必须为 `sponsored` |
| `payerPrivateKey` | string | 是 | 付款方私钥（hex 或 base58） |
| `payerAddress` | string | 是 | 付款方地址（base58） |
| `signatureMode` | object | 是 | 付款方签名模式 |

**请求示例（Demo SponsorWithoutRegister）**

```json
{
  "appName": "demo",
  "methodName": "SponsorWithoutRegister",
  "args": {},
  "paymentMode": "sponsored",
  "payerPrivateKey": "<hex payer private key>",
  "payerAddress": "<base58 payer address>",
  "signatureMode": {"type": "pubkey", "publicKey": "<base58 payer public key>"}
}
```

### 赞助池注册前置流程

调用 `sponsor: true` 的方法（如 `SponsorWithoutRegister`）前，须先调用 `OpenGasSponsorPool` 注册 gas sponsor 地址。可通过 `/api/write` 接口调用：

```json
{
  "appName": "demo",
  "methodName": "OpenGasSponsorPool",
  "args": {
    "pool": "<base58 pool address>"
  },
  "paymentMode": "unified_payer_all",
  "payerPrivateKey": "<hex admin private key>",
  "payerAddress": "<base58 admin address>",
  "signatureMode": {"type": "pubkey", "publicKey": "<base58 admin public key>"}
}
```

> **注意**：`OpenGasSponsorPool` 的 `signer_lookups.admin` 会从 `pool` 参数解析管理员地址，该地址须与 payer 一致。

### 地址与编码说明

- **地址**：base58 编码字符串（如 `1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa`），部分接口也接受 hex 编码。
- **公钥/私钥**：支持 hex 或 base58 编码，由 `NewPublicKeyFromStringRelaxed` / `SecretKeyerFromStringRelaxed` 宽松解析。
- **交易哈希（TxHash）**：hex 编码字符串。
- **资源哈希（RsHash）**：hex 编码，固定 18 字节（36 个 hex 字符）。
- **Blob 哈希**：hex 编码，固定 32 字节（64 个 hex 字符）。
- **Postcard**：Milon 交易序列化格式，接口中以 base64 编码字符串传递。

---

## 接口列表

### 一、网络管理

#### 1. 获取网络列表

- **方法**: `GET`
- **路径**: `/api/network/list`
- **说明**: 返回所有可用网络配置，并标记当前激活的网络。

**请求参数**

无

**请求示例**

```bash
curl http://localhost:8080/api/network/list
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": [
    {
      "name": "devNet",
      "chainId": 2,
      "rpcUrl": "https://devnet-rpc.milon.io",
      "inxUrl": "https://devnet-inx.milon.io",
      "current": true
    },
    {
      "name": "localNet",
      "chainId": 0,
      "rpcUrl": "http://127.0.0.1:8080",
      "inxUrl": "http://127.0.0.1:8081",
      "current": false
    }
  ],
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 2. 获取当前网络

- **方法**: `GET`
- **路径**: `/api/network/current`
- **说明**: 返回当前激活网络的配置信息。

**请求参数**

无

**请求示例**

```bash
curl http://localhost:8080/api/network/current
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "name": "devNet",
    "chainId": 2,
    "rpcUrl": "https://devnet-rpc.milon.io",
    "inxUrl": "https://devnet-inx.milon.io",
    "current": true
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 3. 切换网络

- **方法**: `POST`
- **路径**: `/api/network/switch`
- **说明**: 切换当前激活的网络。支持 `devNet`、`localNet`。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `network` | string | 是 | 目标网络名称 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/network/switch \
  -H "Content-Type: application/json" \
  -d '{"network":"devNet"}'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "switched to devNet",
  "data": null,
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

### 二、系统

#### 4. 健康检查

- **方法**: `GET`
- **路径**: `/api/health`
- **说明**: 健康检查，返回当前链 ID 和区块高度。

**请求参数**

无

**请求示例**

```bash
curl http://localhost:8080/api/health
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "ok": true,
    "chainId": 2,
    "blockHeight": 12345,
    "timestamp": 1753230000000
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 5. 获取链头

- **方法**: `GET`
- **路径**: `/api/chain-head`
- **说明**: 获取当前链头信息，包括区块高度、区块哈希和时间戳。

**请求参数**

无

**请求示例**

```bash
curl http://localhost:8080/api/chain-head
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "chainId": 2,
    "blockHeight": 12345,
    "blockHash": "a1b2c3d4e5f6...",
    "timestampMsecs": 1753230000000
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

### 三、账户

#### 6. 获取账户信息

- **方法**: `GET`
- **路径**: `/api/accounts/:address`
- **说明**: 根据地址获取账户信息。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `address` | path | string | 是 | 账户地址（base58） |

**请求示例**

```bash
curl http://localhost:8080/api/accounts/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "balance": "1000000000"
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 7. 获取账户资源列表

- **方法**: `GET`
- **路径**: `/api/accounts/:address/resources`
- **说明**: 获取指定账户的资源列表。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `address` | path | string | 是 | 账户地址（base58） |

**请求示例**

```bash
curl http://localhost:8080/api/accounts/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa/resources
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "balance": "1000000000"
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 8. 生成账户

- **方法**: `POST`
- **路径**: `/api/accounts/generate`
- **说明**: 生成新的密钥对和地址。支持 4 种密钥算法。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `keyType` | string | 否 | 密钥类型：`secp256k1`（默认）、`ed25519`、`bls12381`、`fndsa512` |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/accounts/generate \
  -H "Content-Type: application/json" \
  -d '{"keyType":"secp256k1"}'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "privateKey": "a1b2c3d4e5f6...",
    "publicKey": "04a1b2c3d4e5f6...",
    "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

### 四、交易

#### 9. 按哈希查询交易

- **方法**: `GET`
- **路径**: `/api/transactions/:hash`
- **说明**: 根据交易哈希查询交易历史。返回的回执（receipt）中包含 `gasCharged` 字段。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `hash` | path | string | 是 | 交易哈希（hex 或 base58 编码） |

**请求示例**

```bash
curl http://localhost:8080/api/transactions/a1b2c3d4e5f6...
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "stamp": 1753230000,
    "payer": 1,
    "signatures": [
      {
        "signer": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
        "authBit": 0,
        "sigBit": 1
      }
    ],
    "instructions": ["a1b2c3..."],
    "receipt": {
      "txId": "a1b2c3d4e5f6...",
      "txHash": "a1b2c3d4e5f6...",
      "state": 2,
      "access": [],
      "events": [],
      "error": null,
      "gasCharged": 1000
    }
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 10. 解析交易输出（IDL 解码）

- **方法**: `GET`
- **路径**: `/api/transactions/:hash/parse`
- **说明**: 按哈希查询交易，并复刻 SDK `helper.DisplayTxHistory` 的解析逻辑，返回人类可读的解码结果：指令（instruction）按 IDL 解码为方法名与参数、访问记录快照按 `typeTag` 解码为 IDL 类型值、事件按 `typeTag` 解码。解码失败不会导致整包失败，单项通过 `decodeError` 字段报告。`?remote=true` 时会额外调用 RPC 获取每个 inline 写入资源的当前链上值（`current`）与 external blob 值（`accessValue`）并解码。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `hash` | path | string | 是 | 交易哈希（hex 或 base58 编码） |
| `remote` | query | bool | 否 | 是否拉取链上当前值并解码，默认 `false` |

**请求示例**

```bash
curl "http://localhost:8080/api/transactions/a1b2c3d4e5f6.../parse?remote=true"
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "tx": {
      "stamp": 1753230000,
      "payer": 1,
      "signatures": [],
      "instructions": ["a1b2c3..."],
      "receipt": {
        "txId": "a1b2c3d4e5f6...",
        "txHash": "a1b2c3d4e5f6...",
        "state": 2,
        "access": [],
        "events": [],
        "error": null,
        "gasCharged": 1000
      }
    },
    "instructions": [
      {
        "index": 0,
        "hex": "0201a1b2c3...",
        "decoded": {
          "app_id": 2,
          "app_name": "demo",
          "instruction_name": "transfer",
          "discriminator": 1,
          "args": { "to": "Base58地址", "amount": "1000000" }
        },
        "formatted": "[demo] transfer\nStruct { ... }"
      }
    ],
    "access": [
      {
        "index": 0,
        "resourceId": "a1b2c3d4e5f6...",
        "firstSnapshot": null,
        "lastWritten": {
          "variant": 0,
          "variantName": "inline",
          "typeTag": 1001,
          "idlType": "Counter",
          "dataHex": "deadbeef",
          "decoded": { "value": "42" },
          "current": {
            "typeTag": 1001,
            "dataHex": "deadbeef",
            "idlType": "Counter",
            "decoded": { "value": "43" }
          }
        }
      }
    ],
    "events": [
      {
        "index": 0,
        "typeTag": 2001,
        "valueHex": "deadbeef",
        "decoded": {
          "app_id": 2,
          "app_name": "demo",
          "event_name": "Transferred",
          "data": { "from": "Base58地址", "to": "Base58地址", "amount": "1000000" }
        },
        "formatted": "[demo] Transferred\nStruct { ... }"
      }
    ]
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

**响应字段说明**

| 字段 | 说明 |
| --- | --- |
| `tx` | 原始交易历史，结构与 `/api/transactions/:hash` 的 `data` 相同 |
| `instructions[].decoded` | 解码后的指令：`app_id`/`app_name`/`instruction_name`/`discriminator`/`args` |
| `instructions[].formatted` | 与 SDK `FormatDecodedInstruction` 输出一致的格式化文本 |
| `access[].firstSnapshot` / `lastWritten` | 快照解码结果；`variantName` 为 `inline` 或 `external`，`decoded` 为按 IDL 类型解码后的值（`u128`/`big.Int` 转十进制字符串，`Address`/`PublicKey` 转 base58，字节转 hex） |
| `lastWritten.current` | 仅 `remote=true`：该资源当前链上值及其解码结果 |
| `lastWritten.accessValue` | 仅 `remote=true`：external blob 的取回值及其解码结果 |
| `events[].decoded` | 解码后的事件：`app_id`/`app_name`/`event_name`/`data` |
| `*.decodeError` | 单项解码失败原因（如未加载对应 IDL、数据损坏），不影响其他字段 |

---

#### 11. 获取交易事件

- **方法**: `GET`
- **路径**: `/api/transactions/:hash/events`
- **说明**: 获取指定交易产生的事件列表，可按 `typeTag` 过滤。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `hash` | path | string | 是 | 交易哈希（hex 或 base58 编码） |
| `typeTag` | query | number | 否 | 事件类型标签，用于过滤 |

**请求示例**

```bash
curl "http://localhost:8080/api/transactions/a1b2c3d4e5f6.../events?typeTag=1"
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "events": [
      {
        "blockHeight": 12345,
        "txHash": "a1b2c3d4e5f6...",
        "txIndex": 0,
        "eventIndex": 0,
        "data": {
          "typeTag": 1,
          "value": "deadbeef"
        }
      }
    ]
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 12. 等待交易确认

- **方法**: `GET`
- **路径**: `/api/transactions/:hash/wait`
- **说明**: 阻塞等待指定交易被确认，返回交易历史。回执中包含 `gasCharged`。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `hash` | path | string | 是 | 交易哈希（hex 或 base58 编码） |
| `timeoutSecs` | query | number | 否 | 超时时间（秒），默认 60 |

**请求示例**

```bash
curl "http://localhost:8080/api/transactions/a1b2c3d4e5f6.../wait?timeoutSecs=30"
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "stamp": 1753230000,
    "payer": 1,
    "signatures": [],
    "instructions": [],
    "receipt": {
      "txId": "a1b2c3d4e5f6...",
      "txHash": "a1b2c3d4e5f6...",
      "state": 2,
      "access": [],
      "events": [],
      "error": null,
      "gasCharged": 1000
    }
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 13. 模拟交易（底层）

- **方法**: `POST`
- **路径**: `/api/transactions/simulate`
- **说明**: 使用预构建的 postcard 交易进行底层模拟（不落链），返回模拟回执（含 `gasCharged`）。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `transactionPostcard` | string | 是 | base64 编码的 postcard 交易数据 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/transactions/simulate \
  -H "Content-Type: application/json" \
  -d '{"transactionPostcard":"ZXhhbXBsZXBvc3RjYXJkZGF0YQ=="}'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "gasCharged": 1000,
    "result": "0x..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 14. 提交交易（底层）

- **方法**: `POST`
- **路径**: `/api/transactions/submit`
- **说明**: 使用预构建的 postcard 交易进行底层提交并落链。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `transactionPostcard` | string | 是 | base64 编码的 postcard 交易数据 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/transactions/submit \
  -H "Content-Type: application/json" \
  -d '{"transactionPostcard":"ZXhhbXBsZXBvc3RjYXJkZGF0YQ=="}'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "txHash": "a1b2c3d4e5f6..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 15. 检测交易

- **方法**: `POST`
- **路径**: `/api/transactions/inspect`
- **说明**: 解析 base64 编码的 postcard 交易，返回交易哈希、指令哈希、付款方及有效性，**不会提交**交易。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `transactionPostcard` | string | 是 | base64 编码的 postcard 交易数据 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/transactions/inspect \
  -H "Content-Type: application/json" \
  -d '{"transactionPostcard":"ZXhhbXBsZXBvc3RjYXJkZGF0YQ=="}'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "txHash": "a1b2c3d4e5f6...",
    "ixHashes": ["f6e5d4c3b2a1..."],
    "payer": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "valid": true
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

### 五、合约

#### 16. 读取视图函数（单返回值）

- **方法**: `POST`
- **路径**: `/api/read`
- **说明**: 读取合约视图函数，封装 SDK 的 `BuildAndViewSingleIx`，返回单条视图结果。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `appName` | string | 是 | 应用名称（如 `token`） |
| `methodName` | string | 是 | 方法名称（如 `balance_of`） |
| `args` | object | 否 | 方法参数键值对 |
| `payerAddress` | string | 否 | 付款方地址（base58） |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/read \
  -H "Content-Type: application/json" \
  -d '{
    "appName": "token",
    "methodName": "balance_of",
    "args": {"owner": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"},
    "payerAddress": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
  }'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "value": "1000000000"
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

**说明：链上载荷与 IDL 返回类型不匹配时的降级响应**

当链上部署的程序版本旧于 IDL（Ok 载荷实际形状与 `returns.type` 声明不一致）时，
`/api/read` 不再返回 500，而是返回结构化降级数据，便于调用方诊断版本漂移：

```json
{
  "success": true,
  "code": 0,
  "message": "ok (raw fallback: view payload does not match IDL return type)",
  "data": {
    "raw": "0x01000100",
    "decodeError": "failed to decode result[0]: failed to deserialize Ok value: unexpected end of input"
  },
  "timestamp": "2026-09-04T14:00:00+08:00"
}
```

典型场景：devNet `staking.EpochState`（部署程序按旧签名返回单字节 Epoch，IDL 声明 3 字段结构）。

**说明：bytes / B96 / B144 / B160 / B256 / Signature 类型参数的 JSON 传法**

args 中的字节类参数支持三种 JSON 形态（服务端自动编码为 postcard 线格式）：

| IDL 类型 | JSON 写法 | 示例 |
| --- | --- | --- |
| `bytes` | 数字数组 或 hex 字符串 | `[1,2,255]` / `"0x0102ff"` |
| `B96`/`B144`/`B160`/`B256` | hex 字符串（可带 `0x`） | `"0xab.."`（32 字节） |
| `Signature` | hex 签名字节（按长度自动识别方案：64B=Ed25519, 65B=Secp256k1, 96B=BLS12-381, 666B=FnDsa-512），线格式为 `varint(variant) + 原始字节` | `"0x..."` |

---

#### 17. 多指令视图查询

- **方法**: `POST`
- **路径**: `/api/read/multi`
- **说明**: 在单次请求中执行多个视图查询，封装 SDK 的 `BuildAndViewMultiIx`。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `instructions` | array | 是 | 指令列表，不可为空 |
| `instructions[].appName` | string | 是 | 应用名称 |
| `instructions[].methodName` | string | 是 | 方法名称 |
| `instructions[].args` | object | 否 | 方法参数键值对 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/read/multi \
  -H "Content-Type: application/json" \
  -d '{
    "instructions": [
      {
        "appName": "token",
        "methodName": "balance_of",
        "args": {"owner": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"}
      },
      {
        "appName": "token",
        "methodName": "total_supply"
      }
    ]
  }'
```

**响应示例**

`data` 为 `0x` 前缀的 hex 字符串，内容是 postcard 编码的原始返回值（可用 IDL 工具离线解码）。

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": "0x0100000002000000a0860100...",
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 18. 模拟合约调用

- **方法**: `POST`
- **路径**: `/api/simulate`
- **说明**: 构建并模拟合约调用（不落链），返回模拟回执（含 `gasCharged`）。支持 6 种支付模式。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `appName` | string | 是 | 应用名称 |
| `methodName` | string | 是 | 方法名称 |
| `args` | object | 否 | 方法参数键值对 |
| `paymentMode` | string | 是 | 支付模式：`unified_payer_all` / `unified_dual_sign` / `unified_payer_only_gas` / `split` / `multi_signer` / `sponsored` |
| `payerAddress` | string | 除 split/multi_signer 外必填 | 付款方地址（base58） |
| `signatureMode` | object | 是 | 付款方签名模式 |
| `ixAddress` | object | dual_sign 模式必填 | 指令账户地址（base58） |
| `ixSignatureMode` | object | dual_sign 模式必填 | 指令账户签名模式 |
| `ownerAddress` | string | split 模式可选 | 所有者地址（默认同 payerAddress） |
| `signers` | array | multi_signer 模式必填 | 签名者列表，详见 [multi_signer 多签名者模式](#multi_signer-多签名者模式) |
| `gasPayer` | object | multi_signer 模式可选 | 独立 gas 付款方，详见 [multi_signer 多签名者模式](#multi_signer-多签名者模式) |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/simulate \
  -H "Content-Type: application/json" \
  -d '{
    "appName": "token",
    "methodName": "transfer",
    "args": {"to": "1Bz2Qk4R9pHn...","amount": 1000},
    "paymentMode": "unified_payer_all",
    "payerAddress": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "signatureMode": {
      "type": "pubkey",
      "publicKey": "04a1b2c3d4e5f6..."
    }
  }'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "gasCharged": 1000,
    "result": "0x..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 19. 写入交易

- **方法**: `POST`
- **路径**: `/api/write`
- **说明**: 构建并提交真实签名的合约交易。支持 6 种支付模式。需提供付款方私钥。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `appName` | string | 是 | 应用名称 |
| `methodName` | string | 是 | 方法名称 |
| `args` | object | 否 | 方法参数键值对 |
| `paymentMode` | string | 是 | 支付模式：`unified_payer_all` / `unified_dual_sign` / `unified_payer_only_gas` / `split` / `multi_signer` / `sponsored` |
| `payerPrivateKey` | string | 除 split/multi_signer 外必填 | 付款方私钥（hex 或 base58） |
| `payerAddress` | string | 除 split/multi_signer 外必填 | 付款方地址（base58） |
| `signatureMode` | object | 是 | 付款方签名模式 |
| `ixPrivateKey` | string | dual_sign 模式必填 | 指令账户私钥 |
| `ixAddress` | string | dual_sign 模式必填 | 指令账户地址 |
| `ixSignatureMode` | object | dual_sign 模式必填 | 指令账户签名模式 |
| `ownerPrivateKey` | string | split 模式可选 | 所有者私钥（默认同 payerPrivateKey） |
| `ownerAddress` | string | split 模式可选 | 所有者地址（默认同 payerAddress） |
| `signers` | array | multi_signer 模式必填 | 签名者列表，详见 [multi_signer 多签名者模式](#multi_signer-多签名者模式) |
| `gasPayer` | object | multi_signer 模式可选 | 独立 gas 付款方，详见 [multi_signer 多签名者模式](#multi_signer-多签名者模式) |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/write \
  -H "Content-Type: application/json" \
  -d '{
    "appName": "token",
    "methodName": "transfer",
    "args": {"to": "1Bz2Qk4R9pHn...","amount": 1000},
    "paymentMode": "unified_payer_all",
    "payerPrivateKey": "a1b2c3d4e5f6...",
    "payerAddress": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "signatureMode": {
      "type": "pubkey",
      "publicKey": "04a1b2c3d4e5f6..."
    }
  }'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "txHash": "a1b2c3d4e5f6..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 20. 多方签名写入

- **方法**: `POST`
- **路径**: `/api/write/multi-agent`
- **说明**: 专为 `unified_dual_sign` 模式设计的多方签名写入端点。付款方与指令账户为不同账户。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `appName` | string | 是 | 应用名称 |
| `methodName` | string | 是 | 方法名称 |
| `args` | object | 否 | 方法参数键值对 |
| `paymentMode` | string | 是 | 必须为 `unified_dual_sign` |
| `payerPrivateKey` | string | 是 | 付款方私钥 |
| `payerAddress` | string | 是 | 付款方地址（base58） |
| `signatureMode` | object | 是 | 付款方签名模式 |
| `ixPrivateKey` | string | 是 | 指令账户私钥 |
| `ixAddress` | string | 是 | 指令账户地址（base58） |
| `ixSignatureMode` | object | 是 | 指令账户签名模式 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/write/multi-agent \
  -H "Content-Type: application/json" \
  -d '{
    "appName": "token",
    "methodName": "transfer",
    "args": {"to": "1Bz2Qk4R9pHn...","amount": 1000},
    "paymentMode": "unified_dual_sign",
    "payerPrivateKey": "a1b2c3d4e5f6...",
    "payerAddress": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "signatureMode": {"type": "pubkey","publicKey": "04a1b2c3d4e5f6..."},
    "ixPrivateKey": "f6e5d4c3b2a1...",
    "ixAddress": "1Bz2Qk4R9pHn...",
    "ixSignatureMode": {"type": "pubkey","publicKey": "04f6e5d4c3b2a1..."}
  }'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "txHash": "a1b2c3d4e5f6..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 21. 多签写入

- **方法**: `POST`
- **路径**: `/api/write/multisig`
- **说明**: 专为 `split` 模式设计的多签写入端点。所有者（owner）同时支付 gas 并签署指令。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `appName` | string | 是 | 应用名称 |
| `methodName` | string | 是 | 方法名称 |
| `args` | object | 否 | 方法参数键值对 |
| `paymentMode` | string | 是 | 必须为 `split` |
| `ownerPrivateKey` | string | 是 | 所有者私钥（未提供时回退到 `payerPrivateKey`） |
| `ownerAddress` | string | 是 | 所有者地址（未提供时回退到 `payerAddress`） |
| `signatureMode` | object | 是 | 所有者签名模式 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/write/multisig \
  -H "Content-Type: application/json" \
  -d '{
    "appName": "token",
    "methodName": "transfer",
    "args": {"to": "1Bz2Qk4R9pHn...","amount": 1000},
    "paymentMode": "split",
    "ownerPrivateKey": "a1b2c3d4e5f6...",
    "ownerAddress": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "signatureMode": {
      "type": "multisig",
      "index": 2,
      "publicKey": "04a1b2c3d4e5f6..."
    }
  }'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "txHash": "a1b2c3d4e5f6..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 22. 多指令模拟调用（打包）

- **方法**: `POST`
- **路径**: `/api/simulate/multi`
- **说明**: 一次请求携带多条指令，打包成单笔交易做链上模拟（dry-run，不落链），返回模拟回执（含 `gasCharged`）。支持 6 种支付模式。指令按数组顺序编号，签名账户默认对**全部指令**授权，与 multi_ix_demo 的多指令打包逻辑一致。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `instructions` | array | 是 | 指令数组，每条含 `appName` / `methodName` / `args` |
| `paymentMode` | string | 是 | 支付模式：`unified_payer_all` / `unified_dual_sign` / `unified_payer_only_gas` / `split` / `multi_signer` / `sponsored` |
| `payerAddress` | string | 除 split/multi_signer 外必填 | 付款方地址（base58） |
| `signatureMode` | object | 是 | 付款方签名模式 |
| `ixAddress` | object | dual_sign 模式必填 | 指令账户地址（base58） |
| `ixSignatureMode` | object | dual_sign 模式必填 | 指令账户签名模式 |
| `ownerAddress` | string | split 模式可选 | 所有者地址（默认同 payerAddress） |
| `signers` | array | multi_signer 模式必填 | 签名者列表，详见 [multi_signer 多签名者模式](#multi_signer-多签名者模式) |
| `gasPayer` | object | multi_signer 模式可选 | 独立 gas 付款方，详见 [multi_signer 多签名者模式](#multi_signer-多签名者模式) |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/simulate/multi \
  -H "Content-Type: application/json" \
  -d '{
    "instructions": [
      {"appName": "token", "methodName": "transfer", "args": {"to": "1Bz2Qk4R9pHn...", "amount": 1000}},
      {"appName": "token", "methodName": "transfer", "args": {"to": "1C3fWm7aKpXv...", "amount": 2000}},
      {"appName": "demo", "methodName": "batch_credit", "args": {"recipients": ["1Bz2Qk4R9pHn..."], "amount": 42}}
    ],
    "paymentMode": "unified_payer_all",
    "payerAddress": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "signatureMode": {
      "type": "pubkey",
      "publicKey": "04a1b2c3d4e5f6..."
    }
  }'
```

**响应示例**：与 [`/api/simulate`](#17-模拟合约调用) 相同，返回模拟回执（含 `gasCharged`）。

---

#### 23. 多指令打包写入

- **方法**: `POST`
- **路径**: `/api/write/multi`
- **说明**: 一次请求携带多条指令，打包成**单笔交易**签名后提交上链，多条指令原子执行（全部成功或全部失败）。支持 6 种支付模式。实现方式与 multi_ix_demo 相同：每条指令独立 Encode 为 `PackedInstruction`，再通过 `lib.NewTransactionBuilder` 装入同一交易，按支付模式分配签名。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `instructions` | array | 是 | 指令数组，每条含 `appName` / `methodName` / `args` |
| `paymentMode` | string | 是 | 支付模式（同 `/api/write`） |
| `payerPrivateKey` | string | 除 split/multi_signer 外必填 | 付款方私钥（hex 或 base58） |
| `payerAddress` | string | 除 split/multi_signer 外必填 | 付款方地址（base58） |
| `signatureMode` | object | 是 | 付款方签名模式 |
| `ixPrivateKey` | string | dual_sign 模式必填 | 指令账户私钥 |
| `ixAddress` | string | dual_sign 模式必填 | 指令账户地址 |
| `ixSignatureMode` | object | dual_sign 模式必填 | 指令账户签名模式 |
| `ownerPrivateKey` | string | split 模式可选 | 所有者私钥（默认同 payerPrivateKey） |
| `ownerAddress` | string | split 模式可选 | 所有者地址（默认同 payerAddress） |
| `signers` | array | multi_signer 模式必填 | 签名者列表，详见 [multi_signer 多签名者模式](#multi_signer-多签名者模式) |
| `gasPayer` | object | multi_signer 模式可选 | 独立 gas 付款方，详见 [multi_signer 多签名者模式](#multi_signer-多签名者模式) |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/write/multi \
  -H "Content-Type: application/json" \
  -d '{
    "instructions": [
      {"appName": "token", "methodName": "transfer", "args": {"to": "1Bz2Qk4R9pHn...", "amount": 1000}},
      {"appName": "token", "methodName": "transfer", "args": {"to": "1C3fWm7aKpXv...", "amount": 2000}}
    ],
    "paymentMode": "unified_payer_all",
    "payerPrivateKey": "a1b2c3d4e5f6...",
    "payerAddress": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "signatureMode": {
      "type": "pubkey",
      "publicKey": "04a1b2c3d4e5f6..."
    }
  }'
```

**响应示例**：与 [`/api/write`](#18-写入交易) 相同，返回 `txHash`。

---

#### 24. 底层单指令视图

- **方法**: `POST`
- **路径**: `/api/view/single`
- **说明**: 使用预构建的 postcard 执行底层单指令视图查询。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `transactionPostcard` | string | 是 | base64 编码的 postcard 数据 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/view/single \
  -H "Content-Type: application/json" \
  -d '{"transactionPostcard":"ZXhhbXBsZXBvc3RjYXJkZGF0YQ=="}'
```

**响应示例**

`data` 为 `0x` 前缀的 hex 字符串，内容是 postcard 编码的原始返回值。

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": "0x0100000002000000a0860100...",
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 25. 底层多指令视图

- **方法**: `POST`
- **路径**: `/api/view/multi`
- **说明**: 使用预构建的 postcard 执行底层多指令视图查询。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `transactionPostcard` | string | 是 | base64 编码的 postcard 数据 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/view/multi \
  -H "Content-Type: application/json" \
  -d '{"transactionPostcard":"ZXhhbXBsZXBvc3RjYXJkZGF0YQ=="}'
```

**响应示例**

`data` 为 `0x` 前缀的 hex 字符串，内容是 postcard 编码的原始返回值（多条指令的结果按序拼接）。

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": "0x0100000002000000a0860100...",
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

### 六、RPC

#### 26. 获取区块

- **方法**: `GET`
- **路径**: `/api/rpc/blocks/:height`
- **说明**: 根据区块高度获取区块信息。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `height` | path | number | 是 | 区块高度 |

**请求示例**

```bash
curl http://localhost:8080/api/rpc/blocks/12345
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "number": 12345,
    "hash": "a1b2c3d4e5f6...",
    "prevHash": "f6e5d4c3b2a1...",
    "stateHash": "c3d4e5f6a1b2...",
    "txRoot": "d4e5f6a1b2c3...",
    "txCount": 12,
    "timestamp": 1753230000000
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 27. 获取资源

- **方法**: `GET`
- **路径**: `/api/rpc/resources/:hash`
- **说明**: 根据资源哈希获取资源内容。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `hash` | path | string | 是 | 资源哈希（hex 编码，固定 18 字节 / 36 字符） |

**请求示例**

```bash
curl http://localhost:8080/api/rpc/resources/a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "typeTag": 1,
    "value": "deadbeef"
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 28. 获取访问值

- **方法**: `POST`
- **路径**: `/api/rpc/access-value`
- **说明**: 根据 Blob 哈希列表批量获取访问值。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `blobHashes` | string[] | 是 | Blob 哈希列表（每个为 hex 编码，固定 32 字节 / 64 字符） |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/rpc/access-value \
  -H "Content-Type: application/json" \
  -d '{
    "blobHashes": [
      "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
      "f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5d4c3b2a1f6e5"
    ]
  }'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": [
    {
      "blobHash": "a1b2c3d4e5f6...",
      "data": {
        "typeTag": 1,
        "value": "deadbeef"
      }
    },
    {
      "blobHash": "f6e5d4c3b2a1...",
      "data": null
    }
  ],
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 29. 按哈希查询资源路径

- **方法**: `GET`
- **路径**: `/api/rpc/resource-paths/:hash`
- **说明**: 根据资源哈希查询其资源路径。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `hash` | path | string | 是 | 资源哈希（hex 编码，固定 18 字节 / 36 字符） |

**请求示例**

```bash
curl http://localhost:8080/api/rpc/resource-paths/a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "rsHash": "a1b2c3d4e5f6...",
    "path": "token.balance_of.owner"
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

### 七、水龙头

#### 30. 领水

- **方法**: `POST`
- **路径**: `/api/faucet/claim`
- **说明**: 从水龙头领取 gas 代币。会提交交易并等待确认，返回领取地址、是否成功及交易哈希。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `privateKey` | string | 是 | 领取方私钥（hex 或 base58） |
| `address` | string | 是 | 领取方地址（base58） |
| `signatureMode` | object | 是 | 签名模式 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/faucet/claim \
  -H "Content-Type: application/json" \
  -d '{
    "privateKey": "a1b2c3d4e5f6...",
    "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "signatureMode": {
      "type": "pubkey",
      "publicKey": "04a1b2c3d4e5f6..."
    }
  }'
```

**响应示例（领取成功）：**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "claimed": true,
    "txHash": "a1b2c3d4e5f6..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

**响应示例（已提交但等待确认失败）：**

```json
{
  "success": true,
  "code": 0,
  "message": "submitted",
  "data": {
    "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "claimed": false,
    "txHash": "a1b2c3d4e5f6...",
    "error": "transaction submitted but wait failed: timeout"
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 31. 查询 MIL 余额

- **方法**: `GET`
- **路径**: `/api/faucet/balance/:address`
- **说明**: 查询指定地址的 MIL 代币余额。

**请求参数**

| 字段 | 位置 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- | --- |
| `address` | path | string | 是 | 账户地址（base58） |

**请求示例**

```bash
curl http://localhost:8080/api/faucet/balance/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "balance": "1000000000"
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

### 八、工具

#### 32. 从公钥派生地址

- **方法**: `POST`
- **路径**: `/api/util/address/derive`
- **说明**: 从公钥派生出对应地址。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `publicKey` | string | 是 | 公钥（hex 或 base58） |
| `keyType` | string | 否 | 密钥类型，未提供时自动识别 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/util/address/derive \
  -H "Content-Type: application/json" \
  -d '{"publicKey":"04a1b2c3d4e5f6...","keyType":"secp256k1"}'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "publicKey": "04a1b2c3d4e5f6...",
    "keyType": "secp256k1"
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 33. 从私钥派生公钥

- **方法**: `POST`
- **路径**: `/api/util/key/derive-public`
- **说明**: 从私钥派生出对应公钥。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `privateKey` | string | 是 | 私钥（hex 或 base58） |
| `keyType` | string | 是 | 密钥类型：`secp256k1`、`ed25519`、`bls12381`、`fndsa512` |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/util/key/derive-public \
  -H "Content-Type: application/json" \
  -d '{"privateKey":"a1b2c3d4e5f6...","keyType":"secp256k1"}'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "publicKey": "04a1b2c3d4e5f6...",
    "keyType": "secp256k1",
    "privateKey": "a1b2c3d4e5f6..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 34. 签名消息

- **方法**: `POST`
- **路径**: `/api/util/sign`
- **说明**: 使用私钥对消息进行签名。**需要环境变量 `ENABLE_UTIL_SIGN=true` 才会启用。**

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `privateKey` | string | 是 | 私钥（hex 或 base58） |
| `message` | string | 是 | 待签名消息：优先按 hex 解码；若 hex 解析失败则按 UTF-8 明文处理（支持中文等非 ASCII 文本） |
| `keyType` | string | 是 | 密钥类型：`secp256k1`、`ed25519`、`bls12381`、`fndsa512` |

> **注意**：`message` 同时支持 **hex 编码** 与 **明文** 两种形式。服务端先尝试 hex 解码，失败则回退为 UTF-8 字节直接签名。因此纯 hex 字符组成的明文（如 `"deadbeef"`）存在歧义，会被当作 hex 处理。

**请求示例（hex 编码）**

```bash
curl -X POST http://localhost:8080/api/util/sign \
  -H "Content-Type: application/json" \
  -d '{
    "privateKey":"a1b2c3d4e5f6...",
    "message":"deadbeef",
    "keyType":"secp256k1"
  }'
```

**请求示例（明文，含中文）**

```bash
curl -X POST http://localhost:8080/api/util/sign \
  -H "Content-Type: application/json" \
  -d '{
    "privateKey":"a1b2c3d4e5f6...",
    "message":"编码",
    "keyType":"secp256k1"
  }'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "signature": "3045...",
    "publicKey": "04a1b2c3d4e5f6..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

> 若未启用 `ENABLE_UTIL_SIGN`，返回 `401` 错误：`{"success":false,"code":401,"message":"sign endpoint is disabled","data":null,"timestamp":"..."}`

---

#### 35. 验签

- **方法**: `POST`
- **路径**: `/api/util/verify`
- **说明**: 验证签名是否有效。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `publicKey` | string | 是 | 公钥（hex 或 base58） |
| `message` | string | 是 | 原始消息（hex 编码或明文，解码规则同 `/api/util/sign`） |
| `signature` | string | 是 | 签名（hex 编码） |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/util/verify \
  -H "Content-Type: application/json" \
  -d '{
    "publicKey":"04a1b2c3d4e5f6...",
    "message":"deadbeef",
    "signature":"3045..."
  }'
```

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "valid": true
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 36. 生成 VC 凭证参数（DiscloseVcAttestation）

- **方法**: `POST`
- **路径**: `/api/util/vc-attestation`
- **说明**: 生成 Milon KYC 认证凭证（DiscloseVcAttestation）参数。算法与 TS 参考实现（`examples/identity/disclose_vc_attestation.ts`）及 `scripts/generate_vc_attestation.py` 严格一致，可直接用于构造链上 `/api/write` 的凭证上传参数。返回 `milon-vc-disclosure` 包装格式。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `issuerPrivateKey` | string | 是 | issuer 私钥（hex/base58）：32 字节为经典 Ed25519 密钥；1281 字节为 FN-DSA-512 后量子密钥，按长度自动识别 |
| `issuerPublicKey` | string | 条件必填 | issuer 公钥（hex/base58，897 字节）。issuer 为 FN-DSA-512 密钥时必填（SDK 无法从签名密钥反推公钥），可用 `/api/account/generate?keyType=fndsa512` 与私钥成对生成；Ed25519 时无需传 |
| `subjectPrivateKey` | string | 条件必填 | subject 私钥（hex），与 `subjectAddress` 二选一 |
| `subjectAddress` | string | 条件必填 | subject 地址（base58，20 字节），与 `subjectPrivateKey` 二选一 |
| `chainId` | number | 否 | 链 ID，缺省 `900000001` |
| `issuerKeyId` | number | 否 | issuer 密钥索引，缺省 `0` |
| `credentialSchema` | string | 否 | 凭证 schema，缺省 `KycLevelCredential` |
| `credentialJson` | string | 否 | 凭证规范化 JSON 字符串（对其做 sha256 得到 `credential_hash`），缺省为内置 KYC 测试凭证 |
| `validUntilMs` | number | 否 | 有效期**毫秒**时间戳（13 位），显式传入时优先于 `validUntil`（传 `0` 表示不过期）。**必须为未来毫秒值**：传 10 位秒级时间戳会被 400 拦截（提示疑似秒级——链端按毫秒解释恒为 1970 年，披露报 1067，用户侧显示 Invalid disclosed VC data）；毫秒但已过期同样 400 拦截 |
| `validUntil` | string | 否 | 有效期 ISO8601（如 `2027-08-24T00:00:00.000Z`），与 `validUntilMs` 二选一；两者都不传时缺省为**当前时间 + 1 年**（动态计算） |
| `credentialName` | string | 否 | 凭证展示名称（写入 `credential.name`） |
| `credentialDesc` | string | 否 | 凭证描述（写入 `credential.description`） |
| `issuedAt` | string | 否 | 签发时间 ISO8601，缺省取当前 UTC |

**请求示例（私钥模式）**

```bash
curl -X POST http://localhost:8080/api/util/vc-attestation \
  -H "Content-Type: application/json" \
  -d '{
    "issuerPrivateKey":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
    "subjectPrivateKey":"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f",
    "credentialSchema":"KycLevelCredential",
    "validUntil":"2027-08-24T00:00:00.000Z",
    "credentialName":"KycLevel Credential",
    "credentialDesc":"A mock KYC credential for testing the DID web upload flow."
  }'
```

**请求示例（地址模式）**

```bash
curl -X POST http://localhost:8080/api/util/vc-attestation \
  -H "Content-Type: application/json" \
  -d '{
    "issuerPrivateKey":"b8df85948dbd37335d137f6772b3c90bf1868d5e3288ce2242f3357624422555",
    "subjectAddress":"48QWpGsZpXJV3rdRvsiQb4iGzBW",
    "credentialSchema":"asdasd"
  }'
```

**请求示例（FN-DSA-512 后量子 issuer）**

issuer 组织使用后量子密钥时，`issuerPrivateKey` 为 1281 字节（hex 2562 字符），且必须同时传 `issuerPublicKey`（897 字节，hex 1794 字符），两者由 `/api/account/generate?keyType=fndsa512` 成对生成：

```bash
curl -X POST http://localhost:8080/api/util/vc-attestation \
  -H "Content-Type: application/json" \
  -d '{
    "issuerPrivateKey":"<1281字节 FN-DSA-512 私钥 hex>",
    "issuerPublicKey":"<897字节 FN-DSA-512 公钥 hex>",
    "subjectAddress":"3pbWorV6iS7Mv8iCs36JUi18RfFy",
    "credentialSchema":"Change",
    "credentialName":"Change Credential",
    "credentialDesc":"A mock Credential credential for testing the DID web upload flow.",
    "validUntil":"2027-08-31T00:00:00.000Z"
  }'
```

**响应示例**

成功响应为**裸 JSON 文档**（无 `success/code/data` 包装），结构与 `milon-vc-disclosure` 产物文件完全一致，可将响应体直接保存为 `.json` 文件使用：

```json
{
  "format": "milon-vc-disclosure",
  "version": 1,
  "credential": {
    "name": "asdasd Credential",
    "description": "A mock asdasd credential for testing the DID web upload flow.",
    "issued_at": "2026-08-24T00:00:00.000Z",
    "valid_until": "2027-08-24T00:00:00.000Z"
  },
  "disclosure": {
    "app": "Identity",
    "method": "DiscloseVcAttestation",
    "args": {
      "subject": "yCinBpqBNjzNtpcdzh9WApHYne6",
      "issuer": "3pbWorV6iS7Mv8iCs36JUi18RfFy",
      "issuer_key_id": 0,
      "credential_schema": "asdasd",
      "credential_hash": [207, 192, 54, 52, 222, 3, 142, 255, 94, 254, 101, 124, 255, 7, 166, 9, 151, 50, 145, 56, 67, 0, 99, 177, 66, 93, 145, 212, 221, 53, 128, 127],
      "valid_until_ms": 1819065600000,
      "issuer_signature": "a33e6bdc66a19dbc822adb39104ce21480ef934130856ada56fbe76ed02fc4b8bde810db0cb4bf1b89931328b35c6e94f692bcf30ba7d28c739d1d5a6980650e"
    }
  }
}
```

> 注：`valid_until` / `valid_until_ms` 为 `null` 表示凭证永不过期（请求传 `validUntilMs: 0` 时）。`issuer_signature` 长度随 issuer 密钥算法而定：Ed25519 为 64 字节（128 hex 字符），FN-DSA-512 为 666 字节（1332 hex 字符）。错误响应仍为统一的 `success/code/message` 包装结构。

---

#### 37. 设置 Mock 返回内容

- **方法**: `POST`
- **路径**: `/api/util/mock/set`
- **说明**: 测试用接口。请求体携带任意合法 JSON，服务端**字节级原样保存**（字段顺序、数字格式、缩进均保留），成功后返回一个**专属链接**（路径携带唯一 `id`），访问该链接即可原样获取本次设置的 JSON。内容保存在进程内存中，服务重启后失效。

**请求参数**

请求体为任意合法 JSON（对象、数组、字符串、数字、布尔、`null` 均可），无固定字段。

**请求示例**

```bash
curl -X POST http://localhost:8080/api/util/mock/set \
  -H "Content-Type: application/json" \
  -d '{"anyKey":"任意JSON，原样返回","nested":{"a":1}}'
```

**响应示例**

```json
{
  "id": "1f2a3b4c5d6e7f8090a1b2c3d4e5f6070",
  "url": "http://localhost:8080/api/util/mock/1f2a3b4c5d6e7f8090a1b2c3d4e5f6070"
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 本次设置的唯一标识（32 位 hex） |
| `url` | string | 专属访问链接，GET 该链接原样返回本次设置的 JSON |

> 错误响应（如请求体为空或非法 JSON）仍为统一的 `success/code/message` 包装结构。

---

#### 38. 获取 Mock 返回内容

- **方法**: `GET`
- **路径**: `/api/util/mock/:id`
- **说明**: 测试用接口。通过 `POST /api/util/mock/set` 返回的专属链接（或 `id`）获取对应的内容，**字节级原样返回**（与设置时一致，无 `success/code/data` 包装）。

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | string | 是 | `POST /api/util/mock/set` 返回的唯一标识 |

**请求示例**

```bash
curl http://localhost:8080/api/util/mock/1f2a3b4c5d6e7f8090a1b2c3d4e5f6070
```

**响应示例**

即对应 id 设置的原始 JSON，例如：

```json
{"anyKey":"任意JSON，原样返回","nested":{"a":1}}
```

> 若 `id` 不存在，返回 `404`，响应为 `{"success":false,"code":404,"message":"mock response \"xxx\" not found: call POST /api/util/mock/set first"}`。

---

### 九、IDL 元数据

#### 39. 获取 IDL 元数据

- **方法**: `GET`
- **路径**: `/api/idl/metadata`
- **说明**: 返回当前网络下所有已加载 IDL app 的元数据，包括 app 列表、每个 app 的方法（指令）清单、参数 schema、返回值类型等。前端 IDL Tab 据此动态渲染方法树与参数表单，新增 IDL 方法无需修改前后端代码。

**请求参数**

无

**请求示例**

```bash
curl http://localhost:8080/api/idl/metadata
```

**响应结构**

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `data` | array | app 元数据列表，按 `appId` 升序排列 |
| `data[].appId` | number | app 的数字 ID（u8） |
| `data[].name` | string | app 名称（如 `token`、`nft`、`staking`） |
| `data[].description` | string | app 描述 |
| `data[].instructions` | array | 方法（指令）列表 |
| `data[].instructions[].name` | string | 方法名（PascalCase，如 `Transfer`） |
| `data[].instructions[].kind` | string | 方法类型：`entry`（写入）或 `view`（只读） |
| `data[].instructions[].handler` | string | 方法处理器名（snake_case，如 `transfer`、`balance_of`） |
| `data[].instructions[].discriminator` | number | 方法判别符（u16） |
| `data[].instructions[].args` | array | 参数列表 |
| `data[].instructions[].args[].name` | string | 参数名 |
| `data[].instructions[].args[].type` | string | 参数类型（原始 IDL 类型字符串，如 `Address`、`u64`、`vec<PublicKey>`） |
| `data[].instructions[].args[].role` | string | 参数角色：`input`（普通输入）、`signer`（必需签名者）、`any_signer`（任意签名者） |
| `data[].instructions[].returns` | object | 返回值类型（仅 `view` 方法存在） |
| `data[].instructions[].returns.type` | string | 返回值类型字符串 |
| `data[].instructions[].sponsor` | bool | 是否为赞助交易（仅 `entry` 方法可能存在） |

**响应示例**

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": [
    {
      "appId": 1,
      "name": "account",
      "description": "Milon account app IDL",
      "instructions": [
        {
          "name": "Create",
          "kind": "entry",
          "handler": "create",
          "discriminator": 1234,
          "args": [
            {"name": "owner", "type": "Address", "role": "input"}
          ],
          "sponsor": true
        }
      ]
    },
    {
      "appId": 2,
      "name": "token",
      "description": "Milon token app IDL",
      "instructions": [
        {
          "name": "Transfer",
          "kind": "entry",
          "handler": "transfer",
          "discriminator": 18518,
          "args": [
            {"name": "token", "type": "Address", "role": "input"},
            {"name": "to", "type": "Address", "role": "input"},
            {"name": "amount", "type": "u64", "role": "input"}
          ]
        },
        {
          "name": "BalanceOf",
          "kind": "view",
          "handler": "balance_of",
          "discriminator": 25810,
          "args": [
            {"name": "owner", "type": "Address", "role": "input"}
          ],
          "returns": {"type": "u64"}
        }
      ]
    }
  ],
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

**使用说明**

- 前端 IDL Tab 加载此端点后，按 `name` 分组展示方法列表，选中方法时根据 `args` 动态渲染参数表单。
- `kind=view` 的方法通过 `/api/read`（单指令）或 `/api/read/multi`（多指令）调用，`methodName` 使用 `handler` 字段。
- `kind=entry` 的方法通过 `/api/simulate`（模拟）或 `/api/write`（落链）调用，需额外提供付款方与签名信息。
- `role=signer` 的参数需在 `signatureMode` 中提供对应签名模式；`role=any_signer` 表示可由任一签名者代签。

---

### 十、VC 全流程工具

#### 40. VC 签发披露全流程

- **方法**: `POST`
- **路径**: `/api/tool/vc-flow`
- **说明**: 给定证书颁发者（issuer）与个人用户（user）两方的私钥，**同步**自动完成整个 VC 链路：双方领水 → issuer 创建 Organization 型 DID → issuer 注册组织（`VcIssuer` 角色 + 声明凭证 schema）→ user 创建 Personal 型 DID → issuer 链下签发 N 张键值对凭证（schema 名 `prefix+序号`，缺省 `Test1~Test5`）→ user 逐张披露上链（`identity.DiscloseVcAttestation`）→ view 回读验证（`DisclosedVcs` + `HasValidVcFromIssuer`）。全流程约 10 笔交易，devNet 上预计 30 秒~2 分钟。**所有步骤幂等**：DID 已创建 / 组织已注册 / 凭证已披露时自动跳过（余额充足也跳过领水），可直接重跑。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `issuerPrivateKey` | string | 是 | 颁发者私钥（hex/base58）：32 字节为经典 Ed25519；1281 字节为 FN-DSA-512（须同时传 `issuerPublicKey`） |
| `issuerPublicKey` | string | 条件必填 | 颁发者公钥，issuer 为 FN-DSA-512 私钥时必填 |
| `issuerAddress` | string | 否 | 颁发者地址；显式传入时须与私钥派生地址一致（防呆校验） |
| `userPrivateKey` | string | 是 | 个人用户私钥，格式同 issuer |
| `userPublicKey` | string | 条件必填 | 用户公钥，FN-DSA-512 私钥时必填 |
| `userAddress` | string | 是 | 用户地址（base58），取 `/api/accounts/generate` 返回的 `address`。必填：同一 32 字节私钥按 ed25519/secp256k1/bls12381 解释会派生**不同地址**，服务端按显式地址自动匹配曲线并锁定正确公钥；与私钥任何曲线派生都不一致时报 400 |
| `credentialPrefix` | string | 否 | 凭证 schema 前缀，缺省 `Test`（生成 `Test1`、`Test2`…） |
| `credentialCount` | number | 否 | 凭证张数，缺省 `5`，上限 `20` |
| `validUntilMs` | number | 否 | 凭证有效期毫秒时间戳，**必须为未来时间**——链端拒绝披露已过期凭证（错误 1067 Only a currently valid VC attestation can be accepted），过期值在开工前即被 400 拦截；`null`/`0`/不传 = 永久有效 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/tool/vc-flow \
  -H "Content-Type: application/json" \
  -d '{
    "issuerPrivateKey":"<颁发者私钥 hex>",
    "userPrivateKey":"<用户私钥 hex>",
    "userAddress":"<用户地址 base58,取账户生成接口返回的 address>"
  }'
```

**响应示例（data 字段）**

```json
{
  "issuer": {
    "address": "3pHqrfVpw4ziiWZ2S6graADk8sXu",
    "faucet": { "skipped": false, "claimed": true, "txHash": "0x…", "balanceBefore": "0", "balanceAfter": "10000000000" },
    "did":    { "skipped": false, "txHash": "0x…" },
    "organization": { "skipped": false, "txHash": "0x…" }
  },
  "user": {
    "address": "48QWpGsZpXJV3rdRvsiQb4iGzBW",
    "faucet": { "skipped": true, "claimed": false, "balanceBefore": "9800000000", "balanceAfter": "9800000000", "detail": "balance is sufficient, skip faucet" },
    "did":    { "skipped": false, "txHash": "0x…" }
  },
  "credentials": [
    {
      "index": 1,
      "schema": "Test1",
      "credentialJson": "{\"credentialSubject\":{\"id\":\"did:milon:…\",\"name\":\"Test1\",\"level\":1},…}",
      "credentialHash": "0x…",
      "validUntilMs": null,
      "issuerSignature": "…",
      "alreadyDisclosed": false,
      "disclosed": true,
      "discloseTxHash": "0x…"
    }
  ],
  "verification": [
    { "schema": "Test1", "onChain": true, "valid": true }
  ]
}
```

**使用说明**

- 幂等重跑：任一步已完成时对应步骤返回 `skipped: true` 并附 `detail`；链端报 `DidAlreadyExists(1024)` / `OrganizationAlreadyExists(1032)` / `VcAttestationAlreadyExists(1072)` 亦视为已完成。
- 领水策略：余额 ≥ 100 MIL 直接跳过；24h 冷却被拒但余额足够时放行并在 `detail` 说明。
- 中途失败：返回 500，`message` 标明失败阶段（如 `disclose credential Test3: …`），`data` 携带已完成步骤明细，可修复后直接重跑（幂等）。
- 凭证内容为确定性 JSON（不含时间戳），同参数重跑生成的 `credentialHash` 一致，满足链上幂等披露。
- 验证结果 `verification[]` 中 `onChain` 表示出现在 `DisclosedVcs` 列表，`valid` 为 `HasValidVcFromIssuer` 当前判定；验证失败不中断流程。
- **DID 默认完整创建**：双方建 DID 默认走 `CreateWithAlias` 完整创建——未显式给别名时服务端自动生成全局唯一别名（issuer 为 `org+地址前8位`、user 为 `user+地址前8位`，数字后缀随机代填，撞名自动换号重试），头像使用占位 URI；即**不传任何 DID 参数，产出的也是带别名的完整 DID**。
- **DID 选项透传**（可选）：请求体可带 `issuerDid` / `userDid` 对象覆盖默认行为——`alias` 显式指定别名主体；`autoAlias: false` 关闭自动别名；`services` 登记服务端点；`avatarUri` 自定义头像。幂等重跑时按差异补齐。示例：

```json
{
  "issuerPrivateKey": "<hex>",
  "userPrivateKey": "<hex>",
  "userAddress": "<base58>",
  "issuerDid": { "alias": "myorg", "services": [{ "label": "portal", "serviceEndpoint": "https://org.example.com" }] },
  "userDid": { "autoAlias": true, "avatarUri": "https://cdn.example.com/me.png" }
}
```

**DID 选项字段**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `alias` | string | 否 | 别名主体（不含数字后缀）；缺省自动生成（`org/user`+地址前 8 位） |
| `autoAlias` | bool | 否 | 缺省 `true`；显式 `false` 且未给 `alias` 时该方 DID 不绑定别名 |
| `suffix` | number | 否 | 显式数字后缀，缺省随机代填（撞名自动换号重试至多 3 次） |
| `services` | array | 否 | 服务端点列表 `[{ "label", "serviceEndpoint" }]` |
| `avatarUri` | string | 否 | 头像 URI（1-512 字节），缺省占位 URI |

---

### 十一、SFT 全流程工具

#### 41. SFT 创建-分发-合并-转移全流程

- **方法**: `POST`
- **路径**: `/api/tool/sft-flow`
- **说明**: 给定 owner 私钥，**同步**自动完成 sftoken 全生命周期：owner 领水（余额 ≥ 100 MIL 跳过；SFT 资源账户与份额接收者无需 gas）→ 创建 SFT（缺省服务端生成 Ed25519 资源账户并在响应中返回私钥；传 `sft.address` 且链上已存在则跳过创建）→ 创建 slot（slot_id 链上递增分配，从 `SlotCreatedEvent` 回执提取；传 `slot.slotId` 则复用已有 slot）→ 分发（对 `distributions` 逐笔 Mint 直发，每笔 `(to, amount)` 铸出独立 token_id，从 `TokenMintedEvent` 回执提取）→ 合并（`merge.fromTokenId → toTokenId`，仅合并 owner 自己持有的份额，合并数量从 `TokenTransferredEvent` 提取）→ 转移（`transfer.tokenId → to`，`amount` 缺省为该 token 全额份额）→ 回读验证（`SftMetadata` / `SlotInfo` / `BalanceOf`）。**步骤开关 = 参数存在性**：不传某步参数即跳过该步，可自由组合（如仅对已有 SFT 做一次转移）。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `ownerPrivateKey` | string | 是 | owner 私钥（hex/base58）：32 字节为经典 Ed25519；1281 字节为 FN-DSA-512（须同时传 `ownerPublicKey`）。slot/mint/merge/transfer 均由 owner 签名并支付 gas |
| `ownerPublicKey` | string | 条件必填 | owner 公钥，owner 为 FN-DSA-512 私钥时必填 |
| `ownerAddress` | string | 是 | owner 地址（base58）。必填：同一 32 字节私钥按 ed25519/secp256k1/bls12381 解释会派生**不同地址**，服务端按显式地址自动匹配曲线并锁定正确公钥 |
| `sft` | object | 否 | SFT 资源账户选项；缺省由服务端生成新资源账户（Ed25519，私钥随响应返回） |
| `sft.address` | string | 否 | 已有 SFT 地址：链上已存在（`SftOwnerOf` 可读）则跳过创建；不存在则须传 `sft.privateKey` 签名创建 |
| `sft.privateKey` | string | 条件必填 | SFT 资源账户私钥——仅在 `sft.address` 指向的 SFT 链上不存在时必填；资源账户仅签 `create_sft` 指令，gas 由 owner 代付 |
| `sft.publicKey` | string | 条件必填 | SFT 资源账户公钥，其为 FN-DSA-512 私钥时必填 |
| `sftMetadata` | object | 条件必填 | 创建新 SFT 时的元数据（需要创建时必填）；`name`（1..=128 字符）与 `symbol`（1..=32 字符）链端强制必填；`coverUrl` / `metadata` 为 URI 字符串；`attribute` 可选字符串。复用已有 SFT 时可省略 |
| `royaltyBps` | number | 否 | 二级市场版税万分比（u16），缺省 `0`；版税接收人初始为 owner |
| `slot` | object | 否 | slot 选项；缺省跳过 slot 步骤。`slotId > 0` 表示复用已有 slot（与 `metadata`/`isTransferable` 互斥）；否则创建新 slot |
| `slot.slotId` | number | 否 | 复用的 slot_id |
| `slot.metadata` | object | 否 | slot 元数据覆盖 `{ "name", "symbol", "coverUrl", "metadata", "attribute" }`，全可选；未提供的字段动态继承 SFT metadata |
| `slot.isTransferable` | bool | 否 | slot 下 token 是否可转移，缺省 `true` |
| `distributions` | array | 否 | 分发列表，每笔 `{ "to", "amount", "metadata?" }` 一次 Mint 直发（独立 token_id），上限 20 笔。依赖 slot（复用或新建） |
| `merge` | object | 否 | 合并选项 `{ "fromTokenId", "toTokenId" }`，两字段必须同时提供；源与目标不能相同。**仅合并 owner 自己持有的份额**（链端语义） |
| `transfer` | object | 否 | 转移选项 `{ "tokenId", "to", "amount?" }`；`amount` 缺省（不传）= 该 token 在 owner 名下的全额份额（链上回读），显式传必须为正数 |
| `steps` | - | - | 无此参数——步骤开关即各参数是否存在：不传 `sft` 建新 SFT、传 `sft.address` 复用；不传 `slot` 跳过 slot；`distributions` 空不分发；`merge`/`transfer` 缺省不执行 |

**请求示例（全流程一把梭）**

```bash
curl -X POST http://localhost:8080/api/tool/sft-flow   -H "Content-Type: application/json"   -d '{
    "ownerPrivateKey": "<owner 私钥 hex>",
    "ownerAddress": "<owner 地址 base58>",
    "sftMetadata": { "name": "Milon SFT Demo", "symbol": "MSFT", "coverUrl": "https://milon.test/sft.png", "attribute": "series=2026" },
    "royaltyBps": 50,
    "slot": { "metadata": { "name": "Level-1 VIP Card" }, "isTransferable": true },
    "distributions": [
      { "to": "<接收者A地址>", "amount": 40 },
      { "to": "<接收者B地址>", "amount": 60, "metadata": { "attribute": "batch=b1" } },
      { "to": "<owner自己地址>", "amount": 30 }
    ],
    "merge": { "fromTokenId": 3, "toTokenId": 1 },
    "transfer": { "tokenId": 1, "to": "<接收者C地址>" }
  }'
```

**响应示例（data 字段）**

```json
{
  "owner": "3pHqrfVpw4ziiWZ2S6graADk8sXu",
  "faucet": { "skipped": true, "claimed": false, "balanceBefore": "9800000000", "balanceAfter": "9800000000", "detail": "balance is sufficient, skip faucet" },
  "sft": {
    "address": "5FsdeLbnTcHmYpqT8ubYb5yLpYjmaxrfVPyD7Ky3ky6rKxgN",
    "owner": "3pHqrfVpw4ziiWZ2S6graADk8sXu",
    "privateKey": "0x…",
    "created": true,
    "skipped": false,
    "txHash": "0x…",
    "detail": "generated new sft resource account (ed25519)"
  },
  "slot": { "slotId": 1, "created": true, "reused": false, "txHash": "0x…" },
  "distributions": [
    { "to": "5DhThCgzEw3qTQNTGwcgpPWyQmEMGcB3mcLqMsZRE4AcdSUf", "amount": 40, "tokenId": 1, "txHash": "0x…" },
    { "to": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "amount": 60, "tokenId": 2, "txHash": "0x…" },
    { "to": "3pHqrfVpw4ziiWZ2S6graADk8sXu", "amount": 30, "tokenId": 3, "txHash": "0x…" }
  ],
  "merge": { "fromTokenId": 3, "toTokenId": 1, "mergedAmount": 30, "txHash": "0x…" },
  "transfer": { "tokenId": 1, "to": "5DAQdpPavkNQAwj4p7cYRmH3DmZVJVk9RW5L8cEYdPvWbpHKmQL5", "amount": 30, "txHash": "0x…" },
  "verification": {
    "sftMetadata": { "name": "Milon SFT Demo", "symbol": "MSFT", "cover_url": "https://milon.test/sft.png", "metadata": "", "attribute": "series=2026" },
    "slotInfo": { "metadata": { "name": "Level-1 VIP Card" } },
    "balances": [
      { "tokenId": 1, "holder": "5DhThCgzEw3qTQNTGwcgpPWyQmEMGcB3mcLqMsZRE4AcdSUf", "balance": 100 },
      { "tokenId": 2, "holder": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", "balance": 0 }
    ]
  }
}
```

**使用说明**

- **步骤组合**：参数存在性即开关——只传 `sft.address` + `transfer` 即是对存量 SFT 的一次转移；只传 `sftMetadata` + `slot` 则只建 SFT 和 slot 不分发。
- **幂等边界**：`sft.address` 链上已存在则跳过创建、`slot.slotId` 复用、领水余额充足跳过，可幂等重跑；**分发/合并/转移是链上状态变更操作，重复调用会重复生效**，重跑前须确认（merge 的源 token 份额并空后链端会报错兜底）。
- **资源账户私钥**：服务端新生成的 SFT 资源账户私钥在 `sft.privateKey` 返回，请妥善保存——后续对该 SFT 重跑创建（如链上回滚后）需要它；仅复用已有 SFT 时无需私钥。
- **分发语义**：每笔 Mint 直发产生独立 token_id（份额按 `(token_id, owner)` 独立记账，同一 token_id 可被多地址同时持有）；如需「先整铸再拆分」请用 `/api/write` 直接调用 `Split` 指令（拆出量必须严格小于签名者份额）。
- **合并方向**：`merge` 把 owner 在 `fromTokenId` 的**全部**份额并入 `toTokenId`（同 slot），链端执行后源 token 份额归零。注意：分发直发给其他接收者的 token，owner 在其上份额为 0，无法作为 `fromTokenId`（链端报 1286 amount must be greater than zero）——请让 `distributions` 里包含一笔发给 owner 自己，或对存量 token 操作。
- **中途失败**：返回 500，`message` 标明失败阶段（如 `distributions[1]: …`），`data` 携带已完成步骤明细（含各步 txHash 与已生成的 `sft.privateKey` / `slotId`），修复后把 `sft.address`、`slot.slotId` 显式传回即可续跑，无需重复已完成步骤。
- **gas**：全程仅 owner 支付 gas（领水/创建 SFT/slot/mint/merge/transfer），SFT 资源账户与份额接收者均无需余额。

---

### 十二、DID 工具

把「建 DID」补成一整套：创建 + 别名（`identity.CreateWithAlias`/`SetAlias`）+ 服务 URI（`AddService` 等）+ 头像（`SetAvatarUri`）+ 密钥管理（`AddKey` 等）+ 停用（`Deactivate`）+ 文档查询（`Document`/`NameBinding`）。

**别名规则**：链上别名为「`alias-数字`」格式（如 `alice-1024`），**完整 DidName（alias+suffix 整体）全局唯一**；`suffix` 未指定时由服务端随机代填（4 位数字），代填撞上已占名（链端 1028）时自动换号重试至多 3 次，显式 `suffix` 撞名则直接报错。`address` 一律必填：同一 32 字节私钥按不同曲线解释会派生不同地址，显式地址用于锁定正确公钥（与 vc-flow 的 `userAddress` 同理）。

#### 42. DID 一键创建（聚合）

- **方法**: `POST`
- **路径**: `/api/tool/did/create`
- **说明**: 一步完成 DID 创建 + 别名绑定 + 服务登记 + 头像设置（有别名走链上 `CreateWithAlias`，否则 `Create`）。**幂等**：链上已有 DID 则跳过创建，按请求**补齐差异**——别名不同→`SetAlias`、缺失服务→逐条 `AddService`、显式头像与链上不同→`SetAvatarUri`（未传头像绝不覆盖链上值），可直接重跑。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `privateKey` | string | 是 | 私钥（hex/base58）；FN-DSA-512 私钥须同时传 `publicKey` |
| `publicKey` | string | 条件必填 | 公钥，FN-DSA-512 私钥时必填 |
| `address` | string | 是 | 账户地址（base58），曲线消歧 |
| `subjectType` | string | 否 | `Personal`（缺省）/ `Organization`（后续注册组织必须为 Organization） |
| `alias` | string | 否 | 别名字符串（不含数字后缀）；非空时创建即绑定别名 |
| `suffix` | number | 否 | 显式指定别名数字后缀；缺省服务端随机代填 |
| `services` | array | 否 | 服务端点列表，元素 `{ "label": "website", "serviceEndpoint": "https://…" }`，endpoint 须为绝对 URI |
| `avatarUri` | string | 否 | 头像 URI（1-512 字节）；创建时缺省用占位 URI，补齐时空值不覆盖链上 |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/tool/did/create \
  -H "Content-Type: application/json" \
  -d '{
    "privateKey": "<私钥 hex>",
    "address": "<账户地址 base58>",
    "subjectType": "Personal",
    "alias": "alice",
    "services": [{ "label": "website", "serviceEndpoint": "https://alice.example.com" }],
    "avatarUri": "https://cdn.example.com/alice.png"
  }'
```

**响应示例（data 字段）**

```json
{
  "address": "QffKfGk3Jnp4k4qHJtbA8fwrW8E",
  "didId": "did:milon:QffKfGk3Jnp4k4qHJtbA8fwrW8E",
  "created": true,
  "steps": {
    "create":   { "skipped": false, "txHash": "0x…", "detail": "created with alias" },
    "alias":    null,
    "services": null,
    "avatar":   null
  },
  "document": { "subject": { "subject_type": {"index":0,"variant":"Personal"}, "address": "…" }, "keys": [], "services": [], "alias": {"alias":"alice","suffix":9386}, "avatar_uri": "…", "deactivated": false }
}
```

> `steps.alias`/`steps.services`/`steps.avatar` 仅在「已存在→补齐差异」路径出现；`document` 为创建/补齐后的链上最新文档。

#### 43. 设置/更换 DID 别名

- **方法**: `POST`；**路径**: `/api/tool/did/set-alias`
- **说明**: 为已有 DID 设置或更换全局唯一别名（链上 `SetAlias`）。代填 suffix 撞名自动换号重试。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `privateKey`/`publicKey`/`address` | string | 是 | 同上 |
| `alias` | string | 是 | 别名字符串（不含后缀） |
| `suffix` | number | 否 | 显式数字后缀，缺省随机代填 |

```bash
curl -X POST http://localhost:8080/api/tool/did/set-alias \
  -H "Content-Type: application/json" \
  -d '{"privateKey":"<hex>","address":"<base58>","alias":"bob"}'
```

响应 `data`：`{ "address", "txHash", "name": {"alias","suffix"}, "nameBinding" }`（`nameBinding` 为按名反查结果，失败时 `null`）。

#### 44. 添加 DID 服务端点

- **方法**: `POST`；**路径**: `/api/tool/did/add-service`
- **说明**: 为 DID 添加服务端点（链上 `AddService`），service id 由链上分配。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `privateKey`/`publicKey`/`address` | string | 是 | 同上 |
| `label` | string | 是 | 服务标签（非空） |
| `serviceEndpoint` | string | 是 | 绝对 URI（`https://…` 等） |

响应 `data`：`{ "address", "txHash", "document" }`——`document` 为最新文档，从 `services[]` 中取链上分配的 `id`。

#### 45. 更新 DID 服务端点

- **方法**: `POST`；**路径**: `/api/tool/did/update-service`

在 43 的基础上增加：`id`（number，必填，待更新的服务 id）。响应同 43。

#### 46. 移除 DID 服务端点

- **方法**: `POST`；**路径**: `/api/tool/did/remove-service`

请求仅 `privateKey`/`publicKey`/`address` + `id`（number，必填）。响应同 43。

#### 47. 设置 DID 头像 URI

- **方法**: `POST`；**路径**: `/api/tool/did/set-avatar-uri`

请求为公共字段 + `avatarUri`（string，必填，1-512 字节）。响应同 43。

#### 48. 添加 DID 密钥

- **方法**: `POST`；**路径**: `/api/tool/did/add-key`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `privateKey`/`publicKey`/`address` | string | 是 | 同上 |
| `newPublicKey` | string | 是 | 要加入文档的新公钥（base58） |
| `label` | string | 否 | 密钥标签，缺省 `null` |

响应 `data`：`{ "address", "txHash", "document" }`，从 `keys[]` 中取链上分配的 `id`。

#### 49. 更新 DID 密钥

- **方法**: `POST`；**路径**: `/api/tool/did/update-key`

在 47 的基础上增加：`id`（number，必填，待更新的密钥 id）。

#### 50. 移除 DID 密钥

- **方法**: `POST`；**路径**: `/api/tool/did/remove-key`

请求为公共字段 + `id`（number，必填）。最后一把密钥链端拒绝（错误 1042 CannotRemoveLastKey）。

#### 51. 停用 DID

- **方法**: `POST`；**路径**: `/api/tool/did/deactivate`
- **说明**: 停用该 DID（链上 `Deactivate`），停用后 identity 全部写操作被拒（错误 1026），**不可逆恢复需谨慎**。请求仅公共字段，响应 `{ "address", "txHash" }`。

#### 52. 查询 DID 文档

- **方法**: `GET`；**路径**: `/api/tool/did/:address/document`
- **说明**: 查询完整 DID 文档（链上 `Document` view）。未创建返回 404（`DidNotFound`/1025）。

```bash
curl http://localhost:8080/api/tool/did/QffKfGk3Jnp4k4qHJtbA8fwrW8E/document
```

响应 `data`：`{ "address", "didId", "document": { subject, controller, keys[], services[], alias, avatar_uri, updated_at_ms, deactivated } }`。

#### 53. 按别名反查绑定

- **方法**: `GET`；**路径**: `/api/tool/did/name-binding?name=alice-1024`
- **说明**: 按完整 DidName 反查绑定（链上 `NameBinding` view）。未绑定返回 404（`NameNotFound`/1029）。

响应 `data`：`{ "name": {"alias","suffix"}, "binding": { "name": …, "subject": {"subject_type","address"} } }`。

---

## 错误码

| 错误码 | 常量名 | HTTP 状态码 | 说明 |
| --- | --- | --- | --- |
| `0` | - | 200 | 成功 |
| `400` | `ERR_INVALID_PARAMETER` | 400 | 请求参数错误（如必填字段缺失、格式不合法、body 解析失败） |
| `401` | `ERR_UNAUTHORIZED` | 403 | 未授权（如 `/api/util/sign` 未启用时调用） |
| `404` | `ERR_NOT_FOUND` | 404 | 资源不存在（如资源路径未找到） |
| `500` | `ERR_INTERNAL` | 500 | 服务器内部错误 |
| `5001` | `ERR_SDK_ERROR` | 500 | Milon SDK 调用错误（如链上请求失败、签名/派生失败） |
| `5002` | `ERR_NETWORK_ERROR` | 400 | 网络错误（如切换到不存在的网络） |
| `5003` | `ERR_TRANSACTION_FAILED` | 500 | 交易失败 |

> **说明**：业务错误码 `code` 与 HTTP 状态码通常对应（参数类错误返回 400，未授权返回 403，未找到返回 404，SDK/内部错误返回 500）。所有错误响应的 `success` 字段为 `false`，`message` 字段包含具体错误描述，`data` 字段可能携带额外错误详情。

---

## 端点总览

| 序号 | 方法 | 路径 | 说明 |
| --- | --- | --- | --- |
| 1 | GET | `/api/network/list` | 获取网络列表 |
| 2 | GET | `/api/network/current` | 获取当前网络 |
| 3 | POST | `/api/network/switch` | 切换网络 |
| 4 | GET | `/api/health` | 健康检查 |
| 5 | GET | `/api/chain-head` | 获取链头 |
| 6 | GET | `/api/accounts/:address` | 获取账户信息 |
| 7 | GET | `/api/accounts/:address/resources` | 获取账户资源列表 |
| 8 | POST | `/api/accounts/generate` | 生成账户 |
| 9 | GET | `/api/transactions/:hash` | 按哈希查询交易 |
| 10 | GET | `/api/transactions/:hash/parse` | 解析交易输出（IDL 解码） |
| 11 | GET | `/api/transactions/:hash/events` | 获取交易事件 |
| 12 | GET | `/api/transactions/:hash/wait` | 等待交易确认 |
| 13 | POST | `/api/transactions/simulate` | 模拟交易（底层） |
| 14 | POST | `/api/transactions/submit` | 提交交易（底层） |
| 15 | POST | `/api/transactions/inspect` | 检测交易 |
| 16 | POST | `/api/read` | 读取视图函数 |
| 17 | POST | `/api/read/multi` | 多指令视图查询 |
| 18 | POST | `/api/simulate` | 模拟合约调用 |
| 19 | POST | `/api/write` | 写入交易 |
| 20 | POST | `/api/write/multi-agent` | 多方签名写入 |
| 21 | POST | `/api/write/multisig` | 多签写入 |
| 22 | POST | `/api/simulate/multi` | 多指令模拟调用（打包） |
| 23 | POST | `/api/write/multi` | 多指令打包写入 |
| 24 | POST | `/api/view/single` | 底层单指令视图 |
| 25 | POST | `/api/view/multi` | 底层多指令视图 |
| 26 | GET | `/api/rpc/blocks/:height` | 获取区块 |
| 27 | GET | `/api/rpc/resources/:hash` | 获取资源 |
| 28 | POST | `/api/rpc/access-value` | 获取访问值 |
| 29 | GET | `/api/rpc/resource-paths/:hash` | 按哈希查询资源路径 |
| 30 | POST | `/api/faucet/claim` | 领水 |
| 31 | GET | `/api/faucet/balance/:address` | 查询 MIL 余额 |
| 32 | POST | `/api/util/address/derive` | 从公钥派生地址 |
| 33 | POST | `/api/util/key/derive-public` | 从私钥派生公钥 |
| 34 | POST | `/api/util/sign` | 签名消息 |
| 35 | POST | `/api/util/verify` | 验签 |
| 36 | POST | `/api/util/vc-attestation` | 生成 VC 凭证参数（DiscloseVcAttestation） |
| 37 | POST | `/api/util/mock/set` | 设置 Mock 返回内容，返回专属链接（测试用） |
| 38 | GET | `/api/util/mock/:id` | 按 ID 返回 Mock 内容（原样返回） |
| 39 | GET | `/api/idl/metadata` | 获取 IDL 元数据 |
| 40 | POST | `/api/tool/vc-flow` | VC 签发披露全流程（领水+DID+组织+凭证+披露，同步幂等） |
| 41 | POST | `/api/tool/sft-flow` | SFT 全流程（创建SFT+slot+分发+合并+转移，参数控制步骤） |
| 42 | POST | `/api/tool/did/create` | DID 一键创建（别名+服务+头像聚合，幂等补齐） |
| 43 | POST | `/api/tool/did/set-alias` | 设置/更换 DID 别名（suffix 可代填+撞名重试） |
| 44 | POST | `/api/tool/did/add-service` | 添加 DID 服务端点 |
| 45 | POST | `/api/tool/did/update-service` | 更新 DID 服务端点 |
| 46 | POST | `/api/tool/did/remove-service` | 移除 DID 服务端点 |
| 47 | POST | `/api/tool/did/set-avatar-uri` | 设置 DID 头像 URI |
| 48 | POST | `/api/tool/did/add-key` | 添加 DID 密钥 |
| 49 | POST | `/api/tool/did/update-key` | 更新 DID 密钥 |
| 50 | POST | `/api/tool/did/remove-key` | 移除 DID 密钥 |
| 51 | POST | `/api/tool/did/deactivate` | 停用 DID |
| 52 | GET | `/api/tool/did/:address/document` | 查询 DID 文档 |
| 53 | GET | `/api/tool/did/name-binding` | 按别名反查 DID 绑定 |

**统计**：共 53 个端点，分布于 12 个功能组（网络管理 3、系统 2、账户 3、交易 7、合约 9、RPC 4、水龙头 2、工具 20、IDL 元数据 1、VC 全流程 1、SFT 全流程 1、DID 12）。此外提供 Web 控制台（`GET /`）与静态资源（`GET /static/*`）。
