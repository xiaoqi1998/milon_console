# Milon API Server 接口文档

## 概述

Milon API Server 将 Milon Go SDK 封装为一组 RESTful HTTP 接口，提供网络管理、账户管理、交易查询与提交、合约读写、RPC 访问、水龙头领水以及密钥工具等能力，共计 **32** 个端点。

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

合约模拟（`/api/simulate`）与写入（`/api/write`）接口支持 4 种支付模式，通过 `paymentMode` 字段指定：

| 支付模式 | 值 | 说明 | 适用场景 |
| --- | --- | --- | --- |
| 统一支付-全签 | `unified_payer_all` | 付款方支付 gas 并签署所有指令 | 单一账户支付并签名 |
| 统一支付-双签 | `unified_dual_sign` | 付款方支付 gas，指令账户单独签名 | 付款方与指令发起方不同 |
| 统一支付-仅 gas | `unified_payer_only_gas` | 付款方仅支付 gas，不签署指令 | 指令由其他方式签名 |
| 拆分支付 | `split` | 所有者（owner）同时支付 gas 并签署指令 | 多签账户场景 |

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

### Gas 费用说明

- 交易回执（`receipt`）中包含 `gasCharged` 字段，表示该笔交易实际消耗的 gas 费用。
- 模拟交易（`/api/simulate`、`/api/transactions/simulate`）的返回结果同样透传 `gasCharged`，供调用方预估费用。
- 对于 sponsored（赞助）交易，`gasCharged` 为 `0`。

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

#### 10. 获取交易事件

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

#### 11. 等待交易确认

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

#### 12. 模拟交易（底层）

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

#### 13. 提交交易（底层）

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

#### 14. 检测交易

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

#### 15. 读取视图函数（单返回值）

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

---

#### 16. 多指令视图查询

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

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "results": [
      {"value": "1000000000"},
      {"value": "100000000000"}
    ]
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 17. 模拟合约调用

- **方法**: `POST`
- **路径**: `/api/simulate`
- **说明**: 构建并模拟合约调用（不落链），返回模拟回执（含 `gasCharged`）。支持 4 种支付模式。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `appName` | string | 是 | 应用名称 |
| `methodName` | string | 是 | 方法名称 |
| `args` | object | 否 | 方法参数键值对 |
| `paymentMode` | string | 是 | 支付模式：`unified_payer_all` / `unified_dual_sign` / `unified_payer_only_gas` / `split` |
| `payerAddress` | string | 除 split 外必填 | 付款方地址（base58） |
| `signatureMode` | object | 是 | 付款方签名模式 |
| `ixAddress` | object | dual_sign 模式必填 | 指令账户地址（base58） |
| `ixSignatureMode` | object | dual_sign 模式必填 | 指令账户签名模式 |
| `ownerAddress` | string | split 模式可选 | 所有者地址（默认同 payerAddress） |

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

#### 18. 写入交易

- **方法**: `POST`
- **路径**: `/api/write`
- **说明**: 构建并提交真实签名的合约交易。支持 4 种支付模式。需提供付款方私钥。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `appName` | string | 是 | 应用名称 |
| `methodName` | string | 是 | 方法名称 |
| `args` | object | 否 | 方法参数键值对 |
| `paymentMode` | string | 是 | 支付模式 |
| `payerPrivateKey` | string | 除 split 外必填 | 付款方私钥（hex 或 base58） |
| `payerAddress` | string | 除 split 外必填 | 付款方地址（base58） |
| `signatureMode` | object | 是 | 付款方签名模式 |
| `ixPrivateKey` | string | dual_sign 模式必填 | 指令账户私钥 |
| `ixAddress` | string | dual_sign 模式必填 | 指令账户地址 |
| `ixSignatureMode` | object | dual_sign 模式必填 | 指令账户签名模式 |
| `ownerPrivateKey` | string | split 模式可选 | 所有者私钥（默认同 payerPrivateKey） |
| `ownerAddress` | string | split 模式可选 | 所有者地址（默认同 payerAddress） |

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

#### 19. 多方签名写入

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

#### 20. 多签写入

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

#### 21. 底层单指令视图

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

---

#### 22. 底层多指令视图

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

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": {
    "results": [
      {"value": "1000000000"},
      {"value": "100000000000"}
    ]
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

### 六、RPC

#### 23. 获取区块

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
    "timestamp": 1753230000000,
    "txProofIdentifiers": ["a1b2c3..."],
    "witnessAddress": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "witnessSignature": "deadbeef..."
  },
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

---

#### 24. 获取资源

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

#### 25. 获取访问值

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

#### 26. 按哈希查询资源路径

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

#### 27. 领水

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

#### 28. 查询 MIL 余额

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

#### 29. 从公钥派生地址

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

#### 30. 从私钥派生公钥

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

#### 31. 签名消息

- **方法**: `POST`
- **路径**: `/api/util/sign`
- **说明**: 使用私钥对消息进行签名。**需要环境变量 `ENABLE_UTIL_SIGN=true` 才会启用。**

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `privateKey` | string | 是 | 私钥（hex 或 base58） |
| `message` | string | 是 | 待签名消息（hex 编码） |
| `keyType` | string | 是 | 密钥类型：`secp256k1`、`ed25519`、`bls12381`、`fndsa512` |

**请求示例**

```bash
curl -X POST http://localhost:8080/api/util/sign \
  -H "Content-Type: application/json" \
  -d '{
    "privateKey":"a1b2c3d4e5f6...",
    "message":"deadbeef",
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

#### 32. 验签

- **方法**: `POST`
- **路径**: `/api/util/verify`
- **说明**: 验证签名是否有效。

**请求参数**

| 字段 | 类型 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `publicKey` | string | 是 | 公钥（hex 或 base58） |
| `message` | string | 是 | 原始消息（hex 编码） |
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
| 10 | GET | `/api/transactions/:hash/events` | 获取交易事件 |
| 11 | GET | `/api/transactions/:hash/wait` | 等待交易确认 |
| 12 | POST | `/api/transactions/simulate` | 模拟交易（底层） |
| 13 | POST | `/api/transactions/submit` | 提交交易（底层） |
| 14 | POST | `/api/transactions/inspect` | 检测交易 |
| 15 | POST | `/api/read` | 读取视图函数 |
| 16 | POST | `/api/read/multi` | 多指令视图查询 |
| 17 | POST | `/api/simulate` | 模拟合约调用 |
| 18 | POST | `/api/write` | 写入交易 |
| 19 | POST | `/api/write/multi-agent` | 多方签名写入 |
| 20 | POST | `/api/write/multisig` | 多签写入 |
| 21 | POST | `/api/view/single` | 底层单指令视图 |
| 22 | POST | `/api/view/multi` | 底层多指令视图 |
| 23 | GET | `/api/rpc/blocks/:height` | 获取区块 |
| 24 | GET | `/api/rpc/resources/:hash` | 获取资源 |
| 25 | POST | `/api/rpc/access-value` | 获取访问值 |
| 26 | GET | `/api/rpc/resource-paths/:hash` | 按哈希查询资源路径 |
| 27 | POST | `/api/faucet/claim` | 领水 |
| 28 | GET | `/api/faucet/balance/:address` | 查询 MIL 余额 |
| 29 | POST | `/api/util/address/derive` | 从公钥派生地址 |
| 30 | POST | `/api/util/key/derive-public` | 从私钥派生公钥 |
| 31 | POST | `/api/util/sign` | 签名消息 |
| 32 | POST | `/api/util/verify` | 验签 |

**统计**：共 32 个端点，分布于 8 个功能组（网络管理 3、系统 2、账户 3、交易 6、合约 7、RPC 4、水龙头 2、工具 4）。此外提供 Web 控制台（`GET /`）与静态资源（`GET /static/*`）。
