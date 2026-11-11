# MCP 易用性六刀设计（2026-11-11）

## 背景与目标

2026-11-11 实测基线：`tools/list` 共 64 个工具、116,839 字节（≈3~4 万 token 常驻 AI 上下文），
top5（contract_write 家族 + saved_instruction_create）单个 5.6~6.5KB。返回值信封原样透传，
错误提示覆盖不全，工具选择面过大。

目标（用户已确认优先级）：**AI 用得准 + 省 token**。允许语义等价的工具合并
（旧工具名直接删，消费者是 AI，schema 描述写清即自适应）。

非目标：不动 REST 业务规则、不动 SDK、不做宿主侧（mcp.json）约定变更。

## 任务级轮次账（六刀的效率目标）

AI 每省一跳 ≈ 省 10~30s（模型生成 + 工具往返）；下表为典型任务跳数：

| 典型任务 | 现状 | 六刀后 |
|---|---|---|
| 转账 | 2 跳（transfer_mil → tx_track） | **1 跳**（wait 缺省注入） |
| 写合约（新方法） | 3~4 跳（idl_methods 全量 → write → track） | **2 跳**（单方法查询 + 一步写） |
| 复用 saved_instruction | 2 跳（execute → track） | **1 跳** |
| 出错恢复 | 3 跳（失败 → error_lookup → 重试） | **2 跳**（失败自带提示） |
| 批量读 | 1 跳 | 1 跳（不变，已最优） |

## 六刀总览（依赖序 = 实施序，每刀独立提交可回滚）

| 刀 | 内容 | 类型 | 预估 |
|---|---|---|---|
| 1 | 错误提示码表驱动 + 补接线 | 扩展现有 | 小 |
| 2 | vc_attestation 静默失败验证修复 | spike | 未知(上限小) |
| 3 | 返回值投影三处 | REST 加可选参数 | 中 |
| 4 | 写路径 wait 缺省注入（MCP 层） | 壳层机制 + transfer_mil REST 加参 | 中 |
| 5 | 工具合并 64→50 | 接口契约变更 | 大 |
| 6 | instructions 三句话地图 | 文案 | 小 |

---

## 刀 1：错误提示码表驱动 + 补接线

**现状**：`handler/error_hints.go` 的 `withChainErrorHint` 手工 2 条规则
（cooldown/insufficient），接线 9 处（contract.go×5 / faucet×2 / transfer_mil×1 等父路径）。

**改法**：
1. `withChainErrorHint` 内部改为复用 error_lookup 的码表数据源
   （`error_lookup_handler.go` 聚合的 API 层 + IDL app 错误码表）：
   错误消息中出现可识别的错误名/码（如 VcRequired、1026）即自动追加
   「码+名字+说明+处置建议」，保持无匹配原样返回、永不吞错的现有契约。
2. 补齐链端错误路径接线：did_*/vc_flow/sft_flow/saved_instruction(send) 的
   SDK 错误响应统一过 hint（对齐 contract.go 现有形态）。
3. 不改信封结构（不加 errorHint 字段）——message 内联提示对 AI 已足够。

**测试**：hint 单测扩为码表驱动断言（含 521/1026/1042/Cooldown 用例）；
新接线 handler 各补一条「SDK 错误带提示」的回归。

## 刀 2：vc_attestation 静默失败验证修复（spike）

**验收记录（2026-11-11，已修复）**：根因 = 该端点 REST 缺省返回裸
`milon-vc-disclosure` 文档（无 `success` 字段，全服务唯一非信封端点）→
MCP 壳 `parseEnvelope` 不命中 → 无 structuredContent → 宿主报
「has an output schema but did not return structured content」（即记忆中的
「MCP 静默失败」，端到端复现确认）。修复 = REST 加可选 query `envelope=true`
（缺省裸文档不变，console/脚本零影响）+ MCP 侧 DefaultArgs 缺省注入
`envelope=true`。回归：`TestVcAttestationStructuredOutput`（structured 产出）、
`TestVcAttestationEnvelopeQuery`（REST 双形态）、`TestDefaultArgsInjection`
（注入机制，刀 4b 的 executor 部分因此提前落地）。

## 刀 3：返回值投影（REST 加可选参数，缺省值不破坏现状）

| 工具 | REST 改动 | MCP 侧 |
|---|---|---|
| idl_methods | 加 `method` 可选 query：只返回该方法的参数/返回值/signer/错误码明细 | 镜像参数；schema 标注「查单方法强烈建议传」 |
| tx_get / tx_track | 加 `view` 可选 query（`full` 缺省 / `summary`）：summary 只回 status/payer/eventsCount/digest | 镜像参数；schema 标注「AI 推荐 summary」 |
| account_resources | 加 `prefix` 可选 query：资源路径前缀过滤 | 镜像参数 |

**测试**：REST 层参数化测试（缺省=现状字节等价、summary 裁剪断言）；
MCP 契约测试同步（TestSchemaRequiredMatchesREST）。

## 刀 4：写路径 wait 缺省注入（发+等并成一跳）

**现状**：`wait` 缺省 false，AI 发完写交易须再调 `tx_track` 等确认——发与等
天然同件却拆两跳。`faucet_claim` 已内置等待（本就 1 跳，不动）。

**改法**：
1. REST：仅 `transfer_mil` 加可选 `wait`（缺省 false 不破坏现状；true 时
   服务端等确认后返回，复用 tx_wait 的等待逻辑）。其余写端点
   （/api/write、/api/write/multi、saved-instructions execute）REST 请求体
   已有 wait 字段，不动。
2. MCP 壳：`registerTool` 支持 per-tool 参数缺省注入——以下工具 `wait`
   未传时注入 true 再回环：`contract_write` / `contract_multi`（刀 5 合并后）
   / `transfer_mil` / `saved_instruction_execute(send)`。REST 缺省保持 false，
   存量脚本与 console 前端零影响。
3. schema 描述标明「MCP 侧缺省 true（省一轮 tx_track）；不等待显式传 false」。

**代价**：单次调用阻塞至确认（缺省 60s 上限，executor 300s 内），AI 会话可接受。

**测试**：注入机制单测（未传注入 true、显式 false 尊重）；transfer_mil REST
wait 参数化测试（缺省不等、true 等待）；MCP 契约测试同步。

## 刀 5：工具合并 64 → 50（语义等价，旧名删除）

只合并参数同构/近同构的工具，全部走「action/mode 切换 + schema 描述标明字段组」：

| 合并 | 新工具形态 | 省 |
|---|---|---|
| DID 细粒度 8 个 → `did_update` | action 八选一：set_alias / add_service / update_service / remove_service / set_avatar_uri / add_key / update_key / remove_key | 7 |
| saved_instruction_create + update | `id` 不传=新建，传=部分更新（两者本就共用大量字段） | 1 |
| saved_instruction_list + get | `id` 不传=清单，传=单条 | 1 |
| tx_simulate_raw + tx_submit_raw + tx_inspect_raw → `tx_raw` | action 三选一（参数同一 struct） | 2 |
| view_single + view_multi → `view_call` | mode 二选一 | 1 |
| contract_simulate_multi + contract_write_multi → `contract_multi` | action 二选一（参数 struct 本就完全相同 contractMultiArgs） | 1 |
| contract_write_safe 并入 contract_write | `safe` 字段缺省 **true**（用户已确认）：默认先模拟再上链；显式 safe:false 跳过 | 1 |

**REST 端点全部保留**（只删 MCP 壳）；REST 是唯一事实源，MCP 镜像映射到
对应端点（如 tx_raw 按 action 映射 /api/transactions/{simulate|submit|inspect}）。

**保留不动**：rpc_* 4 个（参数异构）、util_* 4 个、transfer_mil、全部编排工具
（vc_flow/sft_flow/bulk_transfer*）、guide、error_lookup。

**契约纪律**：
- 每刀合并同步更新 `TestSchemaRequiredMatchesREST` 与全部受影响契约测试；
- 新工具 schema 的 action 枚举与字段组描述齐全（AI 只看 schema 即会用）；
- `postButReadOnly` / `destructiveWrites` / `idempotentWrites` 名单同步
  （tx_raw 按 action 动态判定只读性：simulate/inspect 只读、submit 写；
  did_update 的 remove_* 三 action 进 destructive）；
- tools.go 的 args struct 注释仍逐字镜像、标注来源行号。

**预期**：64 → 50 个，tools/list 116.8KB → ≈80KB 以内（见红线）。

**测试**：合并工具逐 action 的契约测试；devNet 冒烟（did_update 八个 action
逐个实测 + tx_raw 三 action + contract_write safe:true/false 路径）。

## 刀 6：instructions 三句话工具地图

`serverInstructions`（initialize 响应）追加高频三招：
查状态 `idl_methods(带 method) → contract_read`；转账 `transfer_mil`（自带等待）；
写合约 `contract_write`（safe/wait 缺省开，一步到位）。

## 防回弹红线（写进回归测试，非测量脚本）

```
tools/list 总字节 ≤ 80,000（现 116,839）
工具数 ≤ 52（现 64，合并后 50 留 2 余量）
```

实现方式：把 measure_test.go 的逻辑转为正式断言测试（无 MILON_MEASURE 门槛），
超红线即测试失败。

## 风险与回滚

- 刀 5 风险最高（接口契约变更）：逐合并独立提交（DID / saved_instruction /
  raw 三批），单批回滚不影响他批；ZCode 侧需重连 MCP 生效。
- safe:true 翻转的副作用：每次写多一轮模拟（≈1-2s），显式 safe:false 可跳过；
  描述写明。
- 刀 4 wait 注入的副作用：写调用阻塞至确认（≤60s 缺省）；显式 wait:false 逃生；
  仅影响 MCP 路径，REST 消费者行为不变。
- 刀 2 若为环境因素不可复现：记录并关闭，不阻塞。

## 实施序

刀1 → 刀2(spike) → 刀3 → 刀4 → 刀5(三批) → 刀6 → 红线断言收尾。
全程 TDD（红→绿→重构），每刀独立提交。
