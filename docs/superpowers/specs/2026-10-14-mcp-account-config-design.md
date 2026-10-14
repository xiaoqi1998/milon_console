# 设计：MCP 客户端配置操作账户（多账户 + 抗量子支持）

- 日期：2026-10-14
- 状态：已与用户确认方向（三轮澄清：存储位置 → 用户侧 MCP 客户端配置；引用方式 → 地址即引用；接入范围 → 全部私钥入口）
- 前置背景：`872d950`（saved-instructions 私钥永不落盘）、`2026-10-09-request-scoped-network-design.md`（请求级网络头，本设计的传输通道先例）

## 1. 目标与非目标

### 目标

- 用户在自己 MCP 客户端配置（如 `~/.zcode/cli/config.json` → `mcp.servers.milon.headers`）中携带多个操作账户（地址 + 私钥，可选公钥/曲线）
- 私钥支持全部四条曲线：secp256k1（缺省）/ ed25519 / bls12381 / **fndsa512（抗量子，必须显式携带 897 字节公钥）**
- 多用户共用同一个 milon-api-server 实例时，**账户严格按用户隔离，绝不互串**
- 所有现有私钥入口（写工具、领水、DID、flow、保存指令执行）都能以「地址即引用」方式使用配置账户，免每次传私钥

### 非目标

- 服务端不做账户的持久化存储（无磁盘文件、无全局表、无跨请求缓存）
- 不做账户的增删改 MCP 工具——配置就在用户自己机器上，改配置 = 改自己的 mcp.json 后重启会话
- 不做账户级权限/审批流（签名即执行，与现有显式传私钥行为一致）

## 2. 部署与传递模型

当前 milon MCP 以 HTTP 远程服务接入（`https://milon.chuan.click/mcp`），多用户各连同一实例。隔离原则：**账户随每个用户自己的请求过境，服务端零驻留**。

### 2.1 用户侧配置形态（每个用户各自维护）

```json
"milon": {
  "type": "http",
  "url": "https://milon.chuan.click/mcp",
  "headers": {
    "X-Milon-Accounts": "<base64(JSON 数组)>"
  },
  "timeoutMs": 180000
}
```

数组元素结构：

```json
{
  "name": "main",
  "address": "<base58 地址>",
  "privateKey": "<hex 或 base58>",
  "publicKey": "<base58 或 hex 公钥>",
  "keyType": "fndsa512"
}
```

- `name`：可选，仅展示用
- `keyType`：可缺省（secp256k1）
- `publicKey`：fndsa512 **必填**（Go 无法从其私钥派生公钥）；其他曲线可省（服务端可校验派生一致性）
- base64 编码原因：JSON 含引号/非 ASCII，直接放 header 需转义；base64 传输安全
- 体积评估：单个 fndsa512 条目 ≈ 5KB（JSON+base64），Go 默认 header 上限 1MB，几十个账户无压力

通道信任级别：HTTPS header 过境，与现状「每次调用在请求体里传 `payerPrivateKey`」完全同级，只是改为客户端配置自动携带。

### 2.2 自托管单用户场景

保留 `MILON_ACCOUNTS` 环境变量（同结构 JSON 数组，无需 base64）：REST 中间件在请求未携带 header 时回落到该环境变量。共享部署不设置此变量即无串扰面。

## 3. 服务端架构

完全复用请求级网络（X-Milon-Network）的既有三段式通道：

```
用户 MCP 客户端 --header--> /mcp 外层包装（tools.go NewMCPHandler）
    → r.Context() 注入（ctxKeyAccounts，raw header 值）
    → 工具回调 ctx → Executor.Call 回环透传同名 header
    → REST /api/* 中间件 ResolveAccounts 解析+校验 → gin context
    → 各 handler 签名解析点消费
```

### 3.1 新中间件 `ResolveAccounts`（middleware/accounts.go）

挂在 `/api` 组（与 ResolveNetwork 并列）：

1. 读 `X-Milon-Accounts` header；缺失且 `MILON_ACCOUNTS` env 也未设置 → 空集，`c.Next()`（现有行为完全不变）
2. header 存在 → base64 解码 → JSON 解析 → 逐条校验：
   - `address` 非空且为合法 base58 地址
   - `keyType` 归一（空 = secp256k1），必须属于四曲线之一
   - `keyType=fndsa512` 时 `publicKey` 必填，否则整包拒绝
   - 公钥 → 地址一致性：解析 `publicKey`（或非 fndsa 曲线从私钥派生公钥）→ `NewAddressFromPublicKey` 派生地址 → 与 `address` 比对，不一致拒绝
   - 私钥格式校验（hex/base58 可解析、长度符合曲线）
   - 数组内地址重复 → 拒绝
3. 校验通过 → 解析结果（`[]ResolvedAccount`，含 address/privateKey/publicKey/keyType/name）放入 gin context（`CtxAccountsKey`）
4. 校验失败 → 400，错误信息明确指向 `X-Milon-Accounts`（含第几条、哪个字段、什么原因）——**快速失败**：带着坏配置的任何请求（含只读工具）都立即报错，用户第一时间修配置

校验开销全为哈希/解析级（无 fndsa 密钥生成），每请求执行无感知。

### 3.2 MCP 侧透传（mcpserver/）

- `NewMCPHandler` 外层包装新增：`X-Milon-Accounts` header 原文注入 `ctxKeyAccounts`（与 ctxKeyNetwork 并列；注入层保持在鉴权内层，401 路径行为不变）
- `Executor.Call`：ctx 有账户头原文时，回环 REST 请求带上同名 header（与网络头同一段逻辑）
- MCP 外层**不做**账户校验（校验单一发生在 REST 中间件，避免两处规则漂移）；坏配置经回环 400 透传为工具错误

### 3.3 签名解析接入：地址即引用

各私钥入口的「私钥缺失」分支统一升级（集中实现一个 `lookupAccount(c *gin.Context, addrStr string) *ResolvedAccount`，各入口一行接线）：

请求私钥字段为空 + 请求携带地址（对应角色字段）且地址命中本请求账户集 → 注入该账户的 privateKey/publicKey/keyType，后续走现有 `parsePayerAndMode` 全链路（含按链上状态自动修正签名模式）；未命中 → 报错文案升级为：`地址 <addr> 未配置为操作账户（本请求 X-Milon-Accounts 未携带该地址）：先调 account_config_list 核对已配置账户，或随请求显式传私钥`。

接入点清单（全部私钥入口）：

| 入口 | 角色/字段 |
|---|---|
| contract_write / _multi / _safe / _simulate / _simulate_multi | payer、ix、owner、signers[]、gasPayer 五角色 |
| faucet_claim | address |
| did_create / _set_alias / _set_avatar_uri / _add_key / _update_key / _remove_key / _add_service / _update_service / _remove_service / _deactivate | address（各工具的 privateKey+address 成对字段） |
| sft_flow | owner |
| vc_flow / vc_attestation | issuer、user/subject |
| saved_instruction_execute（send 模式） | payer / ix / owner / signers[] / gasPayer（复用其既有「按地址匹配注入」机制，地址命中配置账户时私钥可缺省） |
| transfer_mil | **新增可选 `fromAddress` 字段**：传了且命中配置账户 → 用配置私钥（transfer_mil 现无 from 概念，私钥自派生地址，此为「地址即引用」的自然延伸）；未命中报错同上 |
| bulk_transfer | 不接入（内部临时生成账户，语义不符） |

显式私钥优先级高于配置账户（请求带了就用请求的，配置只兜底缺省分支）。

### 3.4 新增 MCP 工具 `account_config_list`（工具数 63 → 64）

- REST：`GET /api/accounts-config`（同一 handler 双面）
- 返回：当前请求（即当前用户自己 header/env）携带的账户列表，逐条 `name / address / keyType / publicKey 指纹（sha256 前 8 hex）/ createdAt 无（无状态，改为来源标记 header|env）`
- **永不返回 privateKey 与 publicKey 全文**；空集时返回空数组 + 提示文案（如何在 mcp.json 配 headers）
- 校验失败时该端点与其他端点一致返回 400（同一中间件路径）

## 4. 隔离与安全不变量

1. **零持久化**：账户不落盘、不进任何跨请求共享结构（无缓存层）；只活在单次请求的 context 中——结构上杜绝多用户互串
2. `X-Milon-Accounts` 值与一切私钥/公钥明文**永不进日志**：核查 `LoggerMiddleware`（含 `EnableBodyLog=true` 路径）与 MCP 侧日志，将此 header 加入脱敏清单（若现有日志不记 header 则以测试锁死不变量）
3. `account_config_list` 及任何响应不回私钥；公钥只回指纹
4. fndsa512 无 publicKey 在中间件前置拦截（而非签名时才炸）
5. 威胁模型明示：**能携带该 header 调用本服务 = 能用其中账户签名**（与能传 privateKey 等价，非新增向量）；密钥明文存于用户本机客户端配置文件（与存 API key 同级实践），文档明示
6. 显式私钥优先于配置账户（用户单次意图覆盖常驻配置）

## 5. 文档同步

- `README.md` / `API.md`：新 header、`MILON_ACCOUNTS` env、`account_config_list`、`fromAddress` 字段、四曲线账户配置示例（含 fndsa512 完整示例）
- `mcpserver/guide.go`：keys 页补「客户端配置账户」小节（怎么配 header、地址即引用用法、fndsa512 必带公钥），workflows 页补一条典型流（配账户 → 免私钥调用），doc_version bump
- 启动横幅工具数 63 → 64（main.go 与 tools.go 注释同步）

## 6. 测试计划（TDD，红→绿→重构）

### 中间件（middleware/accounts_test.go）

- base64/JSON 解析成功/失败、坏 header 400 文案含 `X-Milon-Accounts` 定位
- fndsa512 缺 publicKey 拒绝；keyType 非法拒绝；地址重复拒绝
- 公钥派生地址与 address 不一致拒绝（secp256k1 派生路径 + fndsa512 显式公钥路径）
- header 缺失 + env 缺失 → 空集放行；header 缺失 + env 有 → env 生效（来源标记 env）；header 有 → header 优先于 env
- **日志无私钥断言**：开启 EnableBodyLog 打一轮请求，日志输出不含任何私钥/公钥/header 值片段

### 解析接线（handler 层）

- contract_write 无 payerPrivateKey + payerAddress 命中 → 注入后走通签名组装（mock client 断言拿到的是配置私钥派生的签名模式）
- 未命中 → 错误文案含 `account_config_list` 提示
- 显式 privateKey + 地址也命中配置 → 用显式私钥（优先级测试）
- signers[]/gasPayer/ix/owner 多角色混合命中
- transfer_mil 带 fromAddress 命中 → 走通；fromAddress 未命中 → 报错
- saved_instruction_execute send 模式地址命中 → 私钥字段可缺省
- did/sft/vc flow 各取一个代表工具接线测试

### MCP 面（mcpserver）

- 镜像测试：`account_config_list` schema 断言；工具计数 64
- e2e（回环，MILON_REST_BASE_URL 注入）：带 X-Milon-Accounts 的 /mcp 调用 → account_config_list 回显脱敏列表；坏 header → 工具错误透传 400 信息
- 隔离测试：两次请求携带不同账户集，各自 account_config_list 只见各自账户；A 的地址在 B 的写调用中未命中报错

## 7. 交付物清单

- `middleware/accounts.go` + 测试
- `handler/`：`lookupAccount` 助手 + 各入口接线 + `accounts_config_handler.go`（list 端点）+ 测试
- `mcpserver/tools.go`：ctxKeyAccounts 注入、executor 透传、新工具注册 + 镜像测试
- `handler/transfer_mil_handler.go`：fromAddress 字段
- `main.go`：路由挂载 `/api/accounts-config`、横幅 64
- `README.md` / `API.md` / `guide.go` 文档同步
