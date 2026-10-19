# 测试数据构造器 v2 设计（Seed Builder v2）

- 日期：2026-10-19
- 状态：已与用户逐节确认（8/8 节通过）
- 取代：`docs/superpowers/specs/2026-10-15-seed-console-design.md`（v1，仅 scale=full/small 两档）
- 范围：v1 不含 staking / validator / vesting（留 v2.x）

## 1. 背景与目标

现播种器（`handler/seed_*.go` 原生实现 + 控制台「测试数据」页）API 只收
`{scale: full|small, modules, seedPhrase, reset}`，矩阵全部写死在
`BuildPlan(scale, seedPhrase)` 内。无法选择：账号数量、是否领水、是否多签、
DID / 合规证书开关、代币数与精度档位、合规代币数、SFT / slot / token 数量、
余额档位。

v2 目标：**模板 + 全局旋钮**的可配置构造器——链上常规测试场景全覆盖、
可选择性创建、同 `seedPhrase + config` 完全可复现。

架构决策（方案一，已确认）：`SeedConfig` 强类型结构为唯一真相源；模板内容
定义在服务端；计划生成是纯函数 `BuildPlan(config, seedPhrase)`；预览与执行
共用同一条生成路径。

## 2. SeedConfig 配置模型

```jsonc
{
  "seedPhrase": "milon-devnet-seed-v1",
  "accounts": {
    "count": 12,                                    // 普通账号数 1..200
    "curves": ["secp256k1", "ed25519", "fndsa512"], // 非空,去重,轮转分配
    "faucet": true,                                 // 是否领水
    "balanceTiers": ["hundred", "thousand", "ten_k"], // 非空(领水时),轮转分档
    "did": true,                                    // 是否创建 DID
    "vcCert": true                                  // 是否配合规证书(VC 披露)
  },
  "multisig": {
    "count": 2,        // 0..20
    "signers": 5,      // 2..20,且 ≤ accounts.count
    "threshold": 3,    // 1..signers
    "faucet": true,    // 多签账号自身领水
    "did": true        // 多签账号自身建 DID
  },
  "tokens": {
    "count": 4,                  // 1..20
    "decimals": [0, 2, 6, 9],    // 非空,0..9,去重;代币按档位轮转分配精度
    "complianceCount": 1,        // 0..count;前 N 个代币为合规代币
    "mintTiers": "default",      // v2 固定沿用 _HUMAN_BASE 表,不开放自定义
    "freezeDemo": true           // 是否对演示用户做一笔真实冻结
  },
  "sft": {
    "count": 3,        // 0..10
    "slotsPerSft": 2,  // 1..20
    "tokensPerSlot": 3, // 1..100(每 slot 铸出的 token 数)
    "scenarios": ["transferable", "frozen", "merge", "burn", "reject_transfer"], // 非空(sft.count>0 时)
    "postOps": true    // 后置转账/合并/销毁操作
  }
}
```

- 余额档位含义（MIL，6 位精度换算最小单位）：`hundred`=100、`thousand`=1_000、
  `ten_k`=10_000。领水不足档位部分由 admin 补差转账。
- `vcCert=true` 隐含 `accounts.did=true`（证书挂 DID 主体），校验层自动纠正并
  在预览响应中提示。
- 数值上限集中在常量区（`seedConfigLimits`），若链端有更紧限制以链端报错为准。

## 3. 内置模板（服务端定义）

| 旋钮 | minimal 极简 | standard 标准 | full 全量 |
|---|---|---|---|
| 普通账号 | 3 | 8（三曲线轮转） | 12（三曲线轮转） |
| 领水 / 余额档 | ✓ / [hundred] | ✓ / [thousand] | ✓ / [hundred, thousand, ten_k] |
| DID / VC 证书 | ✗ / ✗ | ✓ / ✓ | ✓ / ✓ |
| 多签 | 0 | 1 × 3-of-5 | 2 × 3-of-5 |
| 代币 | 1（精度 6） | 3（0/6/9）含 1 合规 | 10（0~9 全档）含 2 合规 |
| freezeDemo | ✗ | ✓ | ✓ |
| SFT / slot / token | 1/1/3 | 2/2/3 场景全 | 3/2/3 场景全 |
| postOps | ✗ | ✓ | ✓ |
| 用途 | 冒烟（分钟级） | 日常开发 | 发布前全量回归 |

旧参数兼容：`scale=full` → 模板 full；`scale=small` → 模板 minimal；兼容层
保留一个小版本后移除。

## 4. 计划生成：纯函数

`BuildPlan(cfg SeedConfig, seedPhrase string) (*SeedPlan, error)`

- 复用现有 Spec 结构（AccountSpec / TokenSpec / SftSpec / ScenarioSpec /
  VcIssuerSpec），矩阵由 cfg 推导：
  - 命名确定性轮转：alice/bob/…、TKA/TKB/…、SFT 键名、别名 `alias-<序号>`；
  - 曲线按 `curves` 轮转；余额档按 `balanceTiers` 轮转；
  - 代币精度按 `decimals` 轮转；前 `complianceCount` 个标记合规，引用 VC issuer
    主 schema；**执行归属**：合规代币由 compliance 模块创建（沿用现状
    credential→create→mode→requirements→mint→accept 流程），tokens 模块跳过
    标记为合规的条目，普通代币仍在 tokens 模块创建；
  - SFT 场景按 `scenarios` 轮转铺到 slot；`postOps` 生成后置操作序列；
  - 多签组：普通账号轮转取 n 个签者（等权重各 1），threshold=m。
- 确定性：同 `seedPhrase + cfg` 两次构建逐字段相等；地址在运行期由密钥派生，
  预览返回规格矩阵 + 计数 + 预估交易数，不含地址。

## 5. 执行器：七模块

模块顺序：`accounts → multisig → tokens → sft → vc → compliance → txs`
（vc/compliance 依赖 issuer 组织账号，顺序与现状一致）。

- multisig 模块（新增，插在 accounts 后）：轮选取签者 → `AccountCreateMultisigIx`
  （owner + signers + 等权 weights + threshold）→ 按开关领水 / 建 DID →
  正常参与代币 / SFT 分发。
- 每步 `IsDone / Mark` 落盘 `data/seed_state.json`；状态 key 前缀加
  `configHash`（= sha256(seedPhrase + 规范化 cfg JSON) 前 16 hex）：
  同配置续跑幂等；改配置自动开新命名空间，互不污染。
- 失败即停：job 标 failed + 断点日志；同配置重跑自动跳过已完成步骤。
- 单任务并发限制保留：同刻仅一个播种任务，重复启动返回 409。

## 6. API 契约

```
GET  /api/tool/seed/templates    → [{key, name, description, config}]
POST /api/tool/seed/plan         → body: SeedConfig（或 {preset, overrides} 简写）
                                    → 完整计划预览（不上链）
POST /api/tool/seed              → 启动任务 → {jobId}；body 同上 + reset?
GET  /api/tool/seed/status/:id   → 进度快照 + 本次 config 回显
```

- 校验错误逐字段返回路径与原因：`tokens.decimals[3]: 超出 0~9`。
- 旧 `GET /tool/seed/plan?scale=` 与 `POST {scale}` 走兼容映射，保留一个小版本。

## 7. 控制台页面

- 模板条：极简 / 标准 / 全量 三按钮；一动旋钮切「自定义」标签。
- 分组卡片：账号 / 多签 / 代币 / SFT——每组对应 SeedConfig 一段；开关 +
  数字步进器 + 档位 chips 多选（精度 0-9 十个 chip、余额档三个 chip、
  SFT 场景五个 chip）。
- 计划预览：调任何旋钮防抖 500ms 调 `POST /plan` 刷新；分组树 + 顶部计数
  （N 账号 · M 代币 · K token · 预估 X 笔交易）。
- 执行区：开始播种 → 进度条 + 阶段清单 + 日志 tail；完成后地址表可复制 /
  导出 JSON。
- 配置存 localStorage 自动回填；种子短语用默认值，页面不接触私钥。
- 预估交易数 > 800 时开始前弹确认。

## 8. 错误处理与边界

- 服务端校验（唯一权威）：数值上下限、`complianceCount ≤ count`、
  `sft.count>0 ⇒ scenarios 非空`、`signers ≤ accounts.count`、
  decimals ∈ 0..9 去重、curves 合法三选一去重。
- faucet 冷却：指数退避重试 3 次；交易超时：记断点停任务可续跑；不静默吞错。
- Python 旧 CLI（`tools/seeder/`）：文件头加 DEPRECATED 注释指向新 API，
  不再维护。

## 9. 测试策略（TDD：红 → 绿 → 重构）

| 层 | 测试 |
|---|---|
| 配置校验 | 表驱动：合法 / 越界 / 逻辑冲突；错误信息含字段路径 |
| 模板展开 | 三模板展开快照断言（账号数 / 精度集 / 合规数） |
| 计划生成 | 纯函数确定性（同输入两次构建逐字段相等）；改单旋钮只动对应段 |
| 执行器 | 现有各模块测试保留；multisig 模块新增（注册 / 领水 / DID 分支）；configHash 命名空间隔离 |
| API | plan / seed / status 契约测试 + 校验错误格式 |
| 前端 | 源码直抽测试（同 `test_mcp_accounts_section.js` 模式）：模板按钮渲染、chips 事件、预览防抖 |
