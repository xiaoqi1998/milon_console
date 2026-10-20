# Evidence:播种器 v2 devNet 端到端（全量模板 completed）

- **日期**:2026-10-19/20
- **分支**:feat/seed-builder-v2
- **性质**:真机 devNet 端到端证据,由 Task 7/8 执行者在修复链完成后的同一场景记录(过程见 `.superpowers/sdd/2026-10-19-seed-builder-v2/task-8-report.md` §7/§8);本文件只陈述事实。

## 端到端场景

- **配置**:full 模板(12 账号 · 双曲线 secp256k1/ed25519 · 余额三档 · 2 多签(5/3) · 10 代币全精度 0..9(含 2 合规) · 3 SFT×2 slot×3 枚五场景 · VC 证书+生命周期 · 交易演示),默认 seed 短语(`milon-devnet-seed-v1`)。
- **结果**:任务状态 `completed`,七模块阶段全部通过——accounts 3 / multisig 6 / tokens 17 / sft 9 / vc 15 / compliance 14 / txs 10,返回 24 个关键地址(admin/用户/代币符号/SFT;私钥不回传)。
- **修复链(1028 → 285)**:同一全量模板场景此前先后败于两处链端交互,修复后闭环:
  1. DID 别名占绑(链端 1028 `DID name already bound: seed-admin-3001`)→ 播种器路径降级为别名可选(`AliasOptional`):别名被占时跳过绑定、DID 本体照建,日志「⚠ 别名 seed-xxx 已被占用,跳过绑定」(提交 `14147a9`)。
  2. 多签 DID 签名模式(链端 285 `PubkeyModeForbidden`,CreateMultisig 注册后单公钥签名被拒)→ 阶段顺序改为 faucet→did→create 根治;续跑兜底:create 已注册且 DID 报 285/"signers list" 时标记 skipped 不失败(提交 `a85d91b`)。
  - 修复前的另一处:SFT 状态标记与链端不一致(复用判定后建 slot 报 1285)→ create 阶段以 `SftOwnerOf` 视图+`flowViewOK` 链端核实,不一致即重建(同提交 `14147a9`)。
- **断点续跑形态(已验证的两条路径)**:ms01 走续跑兜底(create 已注册、DID 285 → skipped),ms02 走新顺序正常路径;同一次运行验证两类分支后任务继续推进至 completed。`stages` 快照按 `configHash` 命名空间过滤,重跑同配置跳过已完成阶段。

## 仍需用户手工复核的清单

以下各点端到端运行中未覆盖到对应分支/形态,需人工在 devNet 上补验:

1. **真实重复 CreateMultisig 的链端错误码**:当前 285 容忍判定靠 `seedCodeRe`/文案匹配;需要人为制造一次「multisig 已注册又重发 CreateMultisig」拿到真实链端错误码,以收窄容忍语义(避免误吞其他 285)。
2. **SFT Freeze 的 freezer 权限(非 owner 场景)**:端到端里 freeze 由持有者(owner)执行;`freeze` 后置操作在 freezer ≠ owner 的场景下链端行为/错误码未验证。
3. **faucet 冷却下的重跑体验**:端到端运行时领水未触发冷却;冷却窗口内重跑播种,领水/补差阶段的退避与跳过行为未实测。
4. **控制台浏览器全流程人工过一遍**:Task 8 冒烟已覆盖模板回填/预览/启动/轮询/地址表,但「全量模板从点开始到 completed 的一次完整人工走查」(含确认弹窗、日志滚动、失败恢复)建议由用户在浏览器中复核一次。
