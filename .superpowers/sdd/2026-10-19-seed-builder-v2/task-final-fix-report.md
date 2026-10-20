# 终审修复波报告(task-final-fix)

- 日期:2026-10-20
- 分支:feat/seed-builder-v2
- 提交:5a620dd `fix(seed-v2): 终审修复波——非法 scale 报 400(v1 回归)+vc-flow 禁注 AliasOptional+前端死代码清理`
- 背景:评审终审一次清完六条(2 Important + 1 Important 前端 + 3 Minor/DOC),全部有精确修法。

## diff 摘要(6 文件,+21/-11)

1. **Important 1** `handler/seed_handler.go` — `seedResolveConfig` switch 新增 case(default 前):`req.Scale != "" && req.Scale != "full" && req.Scale != "small"` → 返回 `fmt.Errorf("未知 scale %q(可选 full/small)", req.Scale)`。非法 scale 不再静默落默认,对齐 v1 scale 校验(400)。位置在 preset/scale 合法分支之后,保持「Config > Preset+Overrides > Scale 映射 > 默认」优先级:preset 显式给出时 scale 兼容字段不参与判定。
2. **Important 2** `handler/vc_flow_handler.go` — `VcFlow` 在 `ShouldBindJSON` 后立即对 `req.IssuerDid` / `req.UserDid` 显式清零 `AliasOptional`(带注释「外部端点禁止注入播种器专用降级开关」),客户端传 `aliasOptional:true` 不再把显式别名静默降级。**绑定面核实**(grep `vcFlowDidOptions|AliasOptional` 全仓):外部 HTTP 直绑该类型仅 VcFlow 一处——`did/create` 端点自行构造 options(不含 AliasOptional)、播种器内部路径(`seed_runner*.go`)程序内置 `AliasOptional:true` 不经 HTTP、`mcpserver/tools.go` 镜像结构 `vcFlowDidOptionsArgs` 无该字段,均无需处理;未改 struct tag(类型被多路径复用)。
3. **Important 3** `static/js/seed.js` — `SEED_CHIPS.seedCurvesChips.list` 删 `'fndsa512'`(9d1fc13 起服务端校验恒 400,UI 不提供永非法选项),行尾注释注明原因。
4. **Minor/DOC 4** `API.md` — SeedConfig 字段表 `tokens.decimals` 行约束列补注:POST 请求侧接受数组,GET 响应为 base64 字符串(Go `[]uint8` 经 `encoding/json` 线上序列化为 base64,前端 `seedDecimalsToArr` 已兼容)。
5. **Minor 5** `handler/seed_plan_v2.go` — 删死变量 `warnMerge`(声明 + merge 场景置位 + `_ = warnMerge` 三段),块注释改为说明 merge 警告由 `SeedPlanSummary` 重算(已正确)。plan 构建输出不变。
6. **Minor 6** `static/js/seed.js` — `startSeedJob` 非 202 分支删 `res.body.data.jobId` 死代码(服务端 409 的 data 为 nil,永不为真及其「接管轮询」注释),保留校验错误列表分支;409 等失败直接把服务端 message 显示为错误 toast。

## TDD 红绿证据(Important 1)

红(实现前,`handler/seed_handler_test.go` `TestSeedResolveConfig_Priority` 新增用例先落地):

```
--- FAIL: TestSeedResolveConfig_Priority (0.00s)
    seed_handler_test.go:115: 非法 scale 应报错,不得静默落默认(对齐 v1 scale 校验)
FAIL
FAIL	milon-api-server/handler	2.559s
```

绿(实现后同测试):`ok milon-api-server/handler 2.524s`

## 全量验证

- `go build ./...` → 零错误
- `go vet ./...` → 零告警
- `go test ./...` → 5 个含测试包全部 ok,零失败
- `node test_seed_v2_section.js` → **ALL PASS**(含「精度 chips」等段,seed.js 改动未破坏前端契约)

## Concerns

- AliasOptional 清零为运行时防注入,未加专门的 HTTP 层测试(vc-flow 全流程依赖链上环境,现有单测覆盖 didEnsureDocument 纯逻辑;如需可后续补 bind 层表驱动用例)。
- seed.js 409 分支改动为纯前端行为,`test_seed_v2_section.js` 为静态段断言,409 路径无自动化覆盖(手工验证路径:任务运行中重复点启动)。
