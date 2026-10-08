# milon-api-server 内嵌 MCP 端点设计

- 日期：2026-10-08
- 状态：已与用户对齐，待实施
- 类型：架构级（新增对外暴露层）

## 背景与动机

milon-api-server 目前以 RESTful HTTP + Web 调试台对外暴露 Milon Go SDK 的链上能力（账户、交易、合约读写、IDL 发现、DID/VC/SFT flow、faucet 等）。用户希望 AI 编程代理（ZCode/Claude 等）能**直接以自然语言驱动这些链能力**，因此要在现有 server 上增加一个 MCP（Model Context Protocol）端点，把现有 REST 能力包成 MCP 工具。

**关键决策记录**（与用户逐项确认）：

1. 动机：AI 代理直接用链能力（不是对外标准接口、不是替代 Playground）。
2. 范围：把**现有**能力包成 MCP，不是只包未来新功能。
3. 形态：挂在现有 server 上，Streamable HTTP 端点 `/mcp`，单进程部署。
4. 复用方式：**方案 A——HTTP 自调用薄适配层**（否决了抽 service 层重构与 MCP 直调 SDK）。
5. 工具面：**全量映射**——除 `mock`（前端调试辅助）与 `health`/`chain-head`（运维探活）外，全部 REST 端点一比一映射为 MCP 工具，共 57 个。能力对称，不留缺口。（勘误：原稿计 55，系 saved-instructions 的 update/delete 两端点转写漏计，2026-10-08 终审修正。）
6. 鉴权：默认关闭，可选 `MCP_AUTH_TOKEN` 环境变量开启 Bearer 校验。

## 目标

- ZCode/Claude 等 MCP 客户端连接 `http://<host>:<port>/mcp` 即可调用全部链上能力。
- MCP 与 REST 行为 100% 一致（同一份 handler 逻辑），REST 侧零改动。
- 新增 REST 端点后，MCP 侧只需在映射表加一行声明。

## 非目标

- 不重构现有 handler 层（不抽 service 层）。
- 不做 stdio 传输模式（未来可加，见"开放问题"）。
- 不改变 REST API 与 Web 调试台的任何行为。
- 不做 MCP 采样（sampling）、资源（resources）、提示（prompts）能力，仅工具（tools）。

## 架构

```
ZCode/Claude (MCP client)
   │ Streamable HTTP (POST /mcp, JSON-RPC)
   ▼
gin 路由 r.Any("/mcp")
   │ modelcontextprotocol/go-sdk StreamableHTTPHandler（stateless）
   ▼
mcpserver 包 · 工具执行器
   │ 参数校验 → 组 REST 请求 → 回环 HTTP
   ▼
http://127.0.0.1:${SERVER_PORT}/api/...（现有 REST handler）
   ▼
gosdk → Milon 链
```

新增 `mcpserver/` 包，职责：

- `tools.go`：57 个工具的静态声明（名称、描述、JSON Schema、REST 映射：method、path 模板、body 组装规则）。
- `executor.go`：统一执行器——校验参数、渲染 path、组装 body、发回环请求、把 REST JSON 响应原样包成 MCP 工具结果（text content）。
- `server.go`：构造 StreamableHTTPHandler、可选 Bearer 鉴权中间件、挂载到 gin。
- `main.go` 增加约 3 行挂载代码，`config` 增读 `MCP_AUTH_TOKEN`。

工具 Schema 手写（现有 REST 无 OpenAPI），描述文字面向 agent：写清参数含义、signer 角色要求、易错点（如私钥曲线、支付模式）。

## 工具清单（57 个，全量映射）

按 REST 路由分组列出。命名 `分组_动作`，与 REST 端点一一对应。

### 网络（3）

| 工具 | REST 端点 |
|---|---|
| `network_list` | GET /api/network/list |
| `network_current` | GET /api/network/current |
| `network_switch` | POST /api/network/switch |

### 账户（3）

| 工具 | REST 端点 |
|---|---|
| `account_generate` | POST /api/accounts/generate |
| `account_info` | GET /api/accounts/{address} |
| `account_resources` | GET /api/accounts/{address}/resources |

### 交易查询（4）

| 工具 | REST 端点 |
|---|---|
| `tx_get` | GET /api/transactions/{hash} |
| `tx_parse` | GET /api/transactions/{hash}/parse |
| `tx_events` | GET /api/transactions/{hash}/events |
| `tx_wait` | GET /api/transactions/{hash}/wait |

### 原始交易（3）

| 工具 | REST 端点 |
|---|---|
| `tx_simulate_raw` | POST /api/transactions/simulate |
| `tx_submit_raw` | POST /api/transactions/submit |
| `tx_inspect_raw` | POST /api/transactions/inspect |

### RPC 底层查询（4）

| 工具 | REST 端点 |
|---|---|
| `rpc_block` | GET /api/rpc/blocks/{height} |
| `rpc_resource` | GET /api/rpc/resources/{hash} |
| `rpc_access_value` | POST /api/rpc/access-value |
| `rpc_resource_path` | GET /api/rpc/resource-paths/{hash} |

### 合约与 IDL 发现（9）

| 工具 | REST 端点 |
|---|---|
| `idl_metadata` | GET /api/idl/metadata（agent 的发现入口：app/方法/参数/返回/signer） |
| `contract_read` | POST /api/read |
| `contract_read_multi` | POST /api/read/multi |
| `contract_simulate` | POST /api/simulate |
| `contract_simulate_multi` | POST /api/simulate/multi |
| `contract_write` | POST /api/write |
| `contract_write_multi` | POST /api/write/multi |
| `contract_write_multi_agent` | POST /api/write/multi-agent |
| `contract_write_multisig` | POST /api/write/multisig |

### 高层 flow 工具（4）

| 工具 | REST 端点 |
|---|---|
| `vc_flow` | POST /api/tool/vc-flow |
| `sft_flow` | POST /api/tool/sft-flow |
| `bulk_transfer` | POST /api/tool/bulk-transfer |
| `bulk_transfer_status` | GET /api/tool/bulk-transfer/{id} |

### DID（12）

| 工具 | REST 端点 |
|---|---|
| `did_create` | POST /api/tool/did/create |
| `did_set_alias` | POST /api/tool/did/set-alias |
| `did_add_service` | POST /api/tool/did/add-service |
| `did_update_service` | POST /api/tool/did/update-service |
| `did_remove_service` | POST /api/tool/did/remove-service |
| `did_set_avatar_uri` | POST /api/tool/did/set-avatar-uri |
| `did_add_key` | POST /api/tool/did/add-key |
| `did_update_key` | POST /api/tool/did/update-key |
| `did_remove_key` | POST /api/tool/did/remove-key |
| `did_deactivate` | POST /api/tool/did/deactivate |
| `did_name_binding` | GET /api/tool/did/name-binding |
| `did_document` | GET /api/tool/did/{address}/document |

### 保存的指令（6）

| 工具 | REST 端点 |
|---|---|
| `saved_instruction_create` | POST /api/saved-instructions |
| `saved_instruction_list` | GET /api/saved-instructions |
| `saved_instruction_get` | GET /api/saved-instructions/{id} |
| `saved_instruction_update` | PUT /api/saved-instructions/{id}（部分更新：只改传入字段，未传保持不变） |
| `saved_instruction_delete` | DELETE /api/saved-instructions/{id} |
| `saved_instruction_execute` | POST /api/saved-instructions/{id}/execute |

### Faucet（2）

| 工具 | REST 端点 |
|---|---|
| `faucet_claim` | POST /api/faucet/claim |
| `faucet_balance` | GET /api/faucet/balance/{address} |

### 密钥与签名工具（5）

| 工具 | REST 端点 |
|---|---|
| `util_derive_address` | POST /api/util/address/derive（32 字节私钥三曲线派生不同地址，地址派生必须走本口径） |
| `util_derive_public_key` | POST /api/util/key/derive-public |
| `util_sign` | POST /api/util/sign |
| `util_verify` | POST /api/util/verify |
| `vc_attestation` | POST /api/util/vc-attestation |

### 视图查询（2）

| 工具 | REST 端点 |
|---|---|
| `view_single` | POST /api/view/single |
| `view_multi` | POST /api/view/multi |

### 明确排除（4 个端点，不映射）

| 端点 | 排除理由 |
|---|---|
| POST /api/util/mock/set、GET /api/util/mock/{id} | 前端调试辅助，agent 无使用场景 |
| GET /api/health、GET /api/chain-head | 运维探活；链头信息 agent 可经 `rpc_block`/`tx_get` 获得 |

## 数据流

1. agent 发起 `tools/call`，携带 JSON 参数。
2. 执行器按工具声明校验必填参数（缺参直接返回 MCP 错误，不发请求）。
3. 渲染 path 模板（如 `/api/transactions/{hash}`）、组装 query/body。
4. 经共享的 `http.Client` 请求 `http://127.0.0.1:${SERVER_PORT}/api/...`。
5. REST 响应（JSON）原样作为工具结果的 text content 返回，agent 可继续解析嵌套字段。

回环基地址取进程自身的 `SERVER_PORT` 配置，复用同一 server 实例，不经外部网络。

## 错误处理

- REST 返回非 2xx：工具结果 `isError=true`，正文为 REST 错误 JSON 原文（含 code/message/详情）。agent 能读到具体失败原因（如链上 512、身份解析错误）自行修参重试。
- 回环连接失败：返回明确错误"milon REST 后端不可达：<原因>"。
- 回环 http.Client 超时 120s（覆盖 `tx_wait` 长轮询与批量交易场景）。
- 参数 schema 校验失败：由 MCP SDK 返回标准协议错误。

## 鉴权与安全

- 环境变量 `MCP_AUTH_TOKEN`：默认空 = `/mcp` 不鉴权（与 REST 现状一致，本机/内网使用）；设置后 `/mcp` 要求 `Authorization: Bearer <token>`，不匹配返回 401。
- Streamable HTTP 以 stateless 模式运行（无 session 状态），多 client 并发安全。
- 私钥仅作为工具参数在内存中转发至回环请求，不落盘。日志风险须如实陈述：`ENABLE_BODY_LOG` 开启时 /mcp 工具入参（请求体）会写入日志，而现有脱敏正则只覆盖裸 `privateKey` 等词、不覆盖 `payerPrivateKey` 等带前缀字段，存在泄露缺口（详见 README MCP 章节的风险提示：仅限 devNet 调试，生产勿开）。工具描述中明确建议 agent 使用临时测试账户，勿投入主网资产私钥。

## 测试策略（TDD）

1. **单测（mcpserver 包）**：以 `httptest.Server` 模拟 REST 后端，逐工具断言：
   - 参数 → REST 请求的映射（method、path、query、body）正确；
   - 2xx 响应 → 工具结果 text content 透传；
   - 非 2xx 响应 → `isError=true` 且正文保留错误 JSON；
   - 缺参 → 本地校验错误，不发请求。
2. **E2E**：`SERVER_PORT=18080` 启动真实 server，用官方 SDK 的 MCP client 走完整协议（initialize → tools/list → tools/call）：
   - `tools/list` 断言 57 个工具全部注册；
   - 真链冒烟：`idl_metadata`、`account_generate`、`contract_read`、`network_list`；
   - 错误路径：故意传错参数，断言错误信息可读。
3. **实测验收**：提供 ZCode 的 `mcpServers` 配置片段，用户加入配置后由 ZCode 在会话中实际调用工具验证。

## 文档与 Playground

- README 新增 "MCP" 章节：启用方式、配置片段、鉴权说明。
- API.md 增补 `/mcp` 说明与 57 工具对照表。
- Web 调试台新增 MCP 卡片：展示连接配置 JSON + 一键复制 + 使用提示（满足"新能力要有前端可玩入口"的项目惯例）。

## 依赖

- 新增唯一 Go 依赖：`github.com/modelcontextprotocol/go-sdk`（官方 SDK，Go 1.25.9 满足其版本要求）。
- 对现有代码的改动仅限：`main.go` 挂载路由（约 3 行）、config 读取 `MCP_AUTH_TOKEN`。

## 开放问题（本期不做）

- stdio 传输模式：如未来需要本机纯 agent 场景（无 HTTP server），可在同一条工具声明上复用，另加入口即可。
- mock/health 类端点：若 agent 场景出现需求再补。
- 工具分页/分组：MCP 协议目前工具列表一次性下发，若 57 个工具对客户端上下文造成压力，再考虑按需裁剪配置。
