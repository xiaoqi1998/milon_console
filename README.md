# Milon API Server

Milon API Server 将 Milon Go SDK 封装成 RESTful HTTP API，并提供一个本地 Web 调试控制台。它适合用来快速调试链上账户、交易、合约调用、RPC 查询、faucet 领水，以及根据 IDL 动态发现可调用的 app 与方法。

## 功能概览

- 网络切换：内置 `devNet`、`localNet`，支持运行时切换当前网络。
- 合约读写：提供 `/api/read`、`/api/read/multi`、`/api/simulate`、`/api/write` 等通用合约接口。
- IDL 元数据发现：通过 `/api/idl/metadata` 暴露当前 SDK 已加载的 app、方法、参数、返回值与 signer 角色。
- 交易能力：支持交易查询、事件查询、等待确认、模拟、提交和 postcard 解析。
- 账户与密钥工具：支持账户生成、公钥派生、地址派生、签名与验签。
- 多种支付/签名模式：支持 `unified_payer_all`、`unified_dual_sign`、`unified_payer_only_gas`、`split`、`multi_signer`、`sponsored`。
- Faucet：支持领水和余额查询。
- Web 控制台：访问 `http://localhost:8080` 可打开调试页面。

## 快速开始

### 环境要求

- Go 1.25+
- CGO enabled
- C 编译器
  - Windows：安装 MinGW-w64，并将 `mingw64/bin` 加入 `PATH`
  - Linux：安装 `gcc`
  - macOS：执行 `xcode-select --install`

### 本地运行

```bash
go env -w CGO_ENABLED=1
go mod tidy
go run main.go
```

启动后访问：

- Web 控制台：`http://localhost:8080`
- 健康检查：`http://localhost:8080/api/health`

### Docker 运行

```bash
docker compose build
docker compose up -d
docker compose logs -f
```

停止服务：

```bash
docker compose down
```

## 配置

服务通过环境变量配置。

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `SERVER_PORT` | `8080` | HTTP 服务端口 |
| `ALLOWED_ORIGINS` | `*` | CORS 允许来源 |
| `ENABLE_UTIL_SIGN` | `false` | 是否启用 `/api/util/sign` 服务端签名接口 |
| `SIGNER_PRIVATE_KEY` | 空 | 服务端签名私钥，按需使用 |
| `DEFAULT_NETWORK` | `devNet` | 默认网络，支持 `devNet`、`localNet` |
| `MILON_RPC_URL` | 空 | 自定义 RPC 地址 |
| `MILON_CHAIN_ID` | `0` | 自定义 chain id |

示例：

```env
SERVER_PORT=8080
DEFAULT_NETWORK=devNet
ALLOWED_ORIGINS=https://your-domain.com
ENABLE_UTIL_SIGN=false
# SIGNER_PRIVATE_KEY=base58_or_hex_private_key
# MILON_RPC_URL=http://your-node:6280/milon/v1
# MILON_CHAIN_ID=2
```

## IDL 元数据

IDL 是这套 API 里最重要的动态发现能力。前端或调用方不需要把每个合约方法的参数写死在页面里，可以先调用 `/api/idl/metadata`，拿到当前网络 SDK 已加载的所有 IDL app，然后根据返回的 schema 动态生成表单、校验参数，并决定调用 `/api/read`、`/api/simulate` 还是 `/api/write`。

### 获取 IDL

```bash
curl http://localhost:8080/api/idl/metadata
```

返回值使用统一响应结构，核心数据在 `data` 字段中：

```json
{
  "success": true,
  "code": 0,
  "message": "ok",
  "data": [
    {
      "appId": 1,
      "name": "demo",
      "description": "Demo app",
      "instructions": [
        {
          "name": "GetScore",
          "kind": "view",
          "handler": "get_score",
          "discriminator": 1001,
          "args": [
            {
              "name": "account",
              "type": "PublicKey",
              "role": "input"
            }
          ],
          "returns": {
            "type": "u64"
          }
        }
      ]
    }
  ],
  "timestamp": "2026-07-23T10:00:00+08:00"
}
```

### 字段说明

| 字段 | 说明 |
| --- | --- |
| `appId` | IDL app 的链上应用编号 |
| `name` | app 名称，也就是调用合约接口时的 `appName` |
| `description` | app 描述 |
| `instructions` | 当前 app 暴露的方法列表 |
| `instructions[].name` | 方法名，也就是调用合约接口时的 `methodName` |
| `instructions[].kind` | 方法类型：`view` 表示只读查询，`entry` 表示会构造交易的写入方法 |
| `instructions[].handler` | SDK/链上 handler 名称 |
| `instructions[].discriminator` | 方法判别码，用于底层指令识别 |
| `instructions[].args` | 方法参数列表 |
| `instructions[].args[].name` | 参数名，对应请求体 `args` 对象里的 key |
| `instructions[].args[].type` | 参数类型，例如 `u64`、`string`、`PublicKey`、`vec<PublicKey>` |
| `instructions[].args[].role` | 参数角色：`input` 普通入参，`signer` 签名账户，`any_signer` 多签/任一签名账户 |
| `instructions[].returns` | `view` 方法的返回类型；写入方法通常没有该字段 |
| `instructions[].sponsor` | 是否为 sponsored 方法；为 `true` 时可配合 `paymentMode: "sponsored"` |

### 如何根据 IDL 调用接口

1. 先调用 `GET /api/idl/metadata` 获取 app 和方法列表。
2. 用户选择一个 `appName` 和 `methodName`。
3. 根据 `args` 生成请求参数表单。
4. 如果 `kind` 是 `view`，调用 `/api/read` 或 `/api/read/multi`。
5. 如果 `kind` 是 `entry`，先调用 `/api/simulate` 预估执行结果和 gas，再调用 `/api/write` 提交交易。
6. 如果方法标记了 `sponsor: true`，可以使用 `paymentMode: "sponsored"`。

只读调用示例：

```json
{
  "appName": "demo",
  "methodName": "GetScore",
  "args": {
    "account": "<base58 account address>"
  }
}
```

写入调用示例：

```json
{
  "appName": "demo",
  "methodName": "SetScore",
  "args": {
    "account": "<base58 account address>",
    "score": 100
  },
  "paymentMode": "unified_payer_all",
  "payerPrivateKey": "<hex or base58 private key>",
  "payerAddress": "<base58 payer address>",
  "signatureMode": {
    "type": "pubkey",
    "publicKey": "<base58 public key>"
  }
}
```

## API 列表

### 网络

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/network/list` | 获取可用网络列表 |
| `GET` | `/api/network/current` | 获取当前网络 |
| `POST` | `/api/network/switch` | 切换当前网络 |

### 系统

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/health` | 健康检查 |
| `GET` | `/api/chain-head` | 获取链头信息 |

### 账户

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/accounts/generate` | 生成账户 |
| `GET` | `/api/accounts/:address` | 获取账户信息 |
| `GET` | `/api/accounts/:address/resources` | 获取账户资源列表 |

### 交易

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/transactions/:hash` | 按 hash 查询交易 |
| `GET` | `/api/transactions/:hash/events` | 查询交易事件 |
| `GET` | `/api/transactions/:hash/wait` | 等待交易确认 |
| `POST` | `/api/transactions/simulate` | 模拟底层 postcard 交易 |
| `POST` | `/api/transactions/submit` | 提交底层 postcard 交易 |
| `POST` | `/api/transactions/inspect` | 解析和检查底层 postcard 交易 |

### 合约

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/read` | 调用单返回值 view 方法 |
| `POST` | `/api/read/multi` | 调用多返回值 view 方法 |
| `POST` | `/api/simulate` | 模拟合约写入交易 |
| `POST` | `/api/write` | 提交合约写入交易 |
| `POST` | `/api/write/multi-agent` | 多方签名写入 |
| `POST` | `/api/write/multisig` | 多签写入 |
| `POST` | `/api/view/single` | 底层单指令 view |
| `POST` | `/api/view/multi` | 底层多指令 view |

### RPC

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/rpc/blocks/:height` | 获取区块 |
| `GET` | `/api/rpc/resources/:hash` | 获取资源 |
| `POST` | `/api/rpc/access-value` | 获取 access value |
| `GET` | `/api/rpc/resource-paths/:hash` | 按资源 hash 查询资源路径 |

### Faucet

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/faucet/claim` | 领水 |
| `GET` | `/api/faucet/balance/:address` | 查询 MIL 余额 |

### 工具

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/util/address/derive` | 从公钥派生地址 |
| `POST` | `/api/util/key/derive-public` | 从私钥派生公钥 |
| `POST` | `/api/util/sign` | 服务端签名，需启用 `ENABLE_UTIL_SIGN=true` |
| `POST` | `/api/util/verify` | 验证签名 |

### IDL

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/idl/metadata` | 获取当前 SDK 已加载的 IDL app、方法、参数和返回值 schema |

## 支付模式

`/api/simulate` 和 `/api/write` 通过 `paymentMode` 指定 gas 支付与签名方式。

| paymentMode | 说明 | 适用场景 |
| --- | --- | --- |
| `unified_payer_all` | payer 支付 gas，并签署 payer 与指令权限 | 单账户支付并签名 |
| `unified_dual_sign` | payer 支付 gas，指令 signer 单独签名 | gas 支付方和业务账户不同 |
| `unified_payer_only_gas` | payer 只支付 gas，不签署指令权限 | 指令已由其他方式签名 |
| `split` | owner 自付 gas 并签署指令 | 独立账户或多签拆分场景 |
| `multi_signer` | 同一指令需要多个 signer 签署 bit0 | NFT、staking 等多 signer 场景 |
| `sponsored` | gas 由链上 sponsor pool 支付 | IDL 中标记 `sponsor: true` 的方法 |

`signatureMode` 常用格式：

```json
{
  "type": "pubkey",
  "publicKey": "<base58 or hex public key>"
}
```

多签格式：

```json
{
  "type": "multisig",
  "index": 2,
  "publicKey": "<base58 or hex public key>"
}
```

## Gas 费用

模拟和交易回执相关接口会透传 `gasCharged` 字段：

- `/api/simulate`
- `/api/transactions/simulate`
- `/api/transactions/:hash`
- `/api/transactions/:hash/wait`

对于 sponsored 交易，`gasCharged` 通常为 `0`，因为 gas 由 sponsor pool 支付。

## 项目结构

```text
milon-api-server/
├── main.go                       # 服务入口和路由注册
├── go.mod                        # Go 模块依赖，本地 replace 到 gosdk-develop
├── Dockerfile
├── docker-compose.yml
├── API.md                        # 更完整的接口文档
├── client/
│   └── network_manager.go        # 网络和 SDK client 管理
├── config/
│   └── config.go                 # 环境变量配置
├── handler/                      # API handler
│   ├── account_handler.go
│   ├── contract.go
│   ├── faucet_handler.go
│   ├── idl_handler.go
│   ├── network.go
│   ├── resource_path_handler.go
│   ├── rpc_read.go
│   ├── system_handler.go
│   ├── transaction_handler.go
│   ├── util.go
│   └── view_handler.go
├── middleware/                   # CORS 和请求日志
├── types/                        # 请求、响应和转换辅助类型
├── static/                       # Web 调试控制台
└── gosdk-develop/                # 内置 Milon Go SDK
```

## 备注

- `gosdk-develop/` 是当前项目直接依赖的本地 SDK 源码目录。
- `gosdk-develop-backup-20260812/` 是备份目录，不参与 `go.mod` 的 replace。
- 生产环境建议关闭 `ENABLE_UTIL_SIGN`，并将 `ALLOWED_ORIGINS` 配置为明确域名。
