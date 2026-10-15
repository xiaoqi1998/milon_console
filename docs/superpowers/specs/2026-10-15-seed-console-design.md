# Milon 控制台一键播种(测试数据)设计

日期:2026-10-15。状态:已批准(对话内设计确认)。

## 目标

把 `tools/seeder/seed.py` 的播种能力原生化进 milon-api-server,控制台页面可发起、可看进度、可拿结果;重新创世后一键还原整条 devNet 测试环境。语义与 Python 版逐条对齐,断点续跑状态与 Python CLI 共用。

## 后端(handler/seed 新包)

- `plan.go` — 确定性计划:full(12 用户/10 代币精度 0~9/2 SFT 集合 10 场景 slot/2 issuer 17 人持证/SCUSD 合规/8+1 交易)与 small(4/3/1×3/1×4/…/3+1)。纯数据,零 IO。
- `keys.go` — `sha256("{seed}:{role}:{name}")` 确定性 32 字节私钥;issuer 用 ed25519(VC 签名限制),SFT 资源账户 ed25519,其余 secp256k1;地址经 server 既有派生口径。
- `runner.go` — 六模块执行器:
  - accounts:领水(先查余额+`FaucetCooldownRemaining`,冷却中/余额足则 skip,单次尝试)+ DID `/api/tool/did/create` 语义(幂等补齐,显式后缀保证确定性)。
  - tokens:`token.Create` 双签(516=已存在跳过)→ MintBatch **余额对账补铸**(假阳性标记自愈)→ SEED8 冻结演示(按链上余额 1/3,为 0 跳过)。
  - sft:复用 `sftFlowEnsureSft/EnsureSlot/MintOne` 自由函数;六个场景(no_transfer 附拒绝实测、metadata_full、token_override、burn_shell 后置烧卡、multi_holder 后置双转、empty)。
  - vc:抽 `vcFlowCore` 供 HTTP handler 与播种共用;按 issuer×user 矩阵跑,幂等;生命周期 freeze(credential_id)/revoke(credential_schema)。
  - compliance:`CreateWithCompliance`(owner=admin 须持证,已由发证矩阵保证)→ SetComplianceMode(Any)→ AddComplianceRequirement → 余额守卫 Mint → 持证收款(必须成功)→ 无证转账(必须被 521/522 拦截)。
  - txs:MIL 互转 + TransferBatch + 513 预期失败(断言错误码)。
- `state.go` — 断点续跑:`data/seed_state.json`,schema 同 Python(`{module: {stage: data}}`),原子写;两个实现可互相接续。
- `job.go`/`handler.go` — 仿 bulk-transfer 异步任务:同刻仅允许一个 seed job;快照含模块/阶段进度、日志环形缓冲(定长)、完成后关键地址表。

### 端点

- `POST /api/tool/seed` `{scale, modules[], seedPhrase?, reset?}` → `{jobId}`;modules 含 compliance 时自动补 vc;重复启动返回 409。
- `GET /api/tool/seed/status/:jobId` → 进度快照。
- `GET /api/tool/seed/plan?scale=full` → 计划预览(不上链)。

## 前端(控制台新 tab「测试数据」)

规模下拉、模块勾选、seed 短语输入(默认 `milon-devnet-seed-v1`)、计划预览、开始按钮(确认弹窗)、2s 轮询进度(阶段+日志)、完成地址表复制。`static/js/seed.js` + `view-seed` section,复用现有卡片/按钮样式。

## 测试

- `plan_test.go`:确定性、规模一致性、精度集 {0..9}、场景键、VC 矩阵、u64 上限——与 Python 版对拍。
- runner 单测:payload/幂等分支纯函数;链上验证走端到端。
- job 单测:启动互斥、快照。
- 端到端:devNet 真跑 small → full,exit 0 + 抽验(FrozenOf/HasValidVc/Lifecycle/Compliance)。

## 风险与对策

- vc-flow 抽 core 需重构:保持 HTTP 行为不变,现有 vc_flow_handler_test.go 回归。
- 与 Python 版 state 并写:同 schema 同文件,阶段命名一致;文档注明。
- 长任务(30-45min):job 常驻内存 + 落盘可续;server 重启后 status 丢失属可接受(按钮重按即续)。
