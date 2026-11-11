# MCP 易用性六刀 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按六刀 spec 落地 MCP 易用性改进——错误提示码表化、返回值投影、wait 缺省注入、工具合并 64→50、instructions 强化、tools/list 红线断言。

**Architecture:** REST 是唯一事实源：MCP 壳（mcpserver/）逐字镜像 handler request struct 并回环调 REST。本计划三类改动：REST handler 加可选参数（缺省不破坏现状）、MCP 壳注入/合并（新增 registerRoutedTool 多端点路由 + DefaultArgs 缺省注入）、纯文案。

**Tech Stack:** Go + gin + modelcontextprotocol/go-sdk/mcp；测试用 httptest fake RPC 后端模式（参照 handler/transfer_mil_test.go）。

**Spec:** `docs/superpowers/specs/2026-11-11-mcp-usability-design.md`

## Global Constraints

- REST 缺省行为不得改变（新参数全部 omitempty/可选，缺省=现状）
- MCP args struct 的 json tag 逐字镜像 handler 侧 request struct，注释标注来源行号
- `TestSchemaRequiredMatchesREST` 是契约事实源，改工具必同步
- jsonschema tag 首分词禁用「WORD=」形态（go-sdk 解析 panic）
- 工具描述以「一句话功能概括（细节……）」起笔（toolTitleOf 依赖）
- 提交信息中文；每任务独立提交可回滚
- 服务端单次调用总超时 300s；等待缺省 60s

---

### Task 1: 刀1a — withChainErrorHint 码表驱动化

**Files:**
- Modify: `handler/error_hints.go`
- Modify: `handler/error_lookup_handler.go`（apiErrorTable 复用，不动）
- Test: `handler/error_hints_test.go`

**Interfaces:**
- Produces: `withChainErrorHintFor(c *gin.Context, msg string) string`（Task 2 接线用）；`appendIDLErrorHint(msg string, briefs []idlErrBrief) string` 纯函数

- [ ] **Step 1: 写失败测试**（追加到 error_hints_test.go）

```go
// idlErrBrief 镜像 appendIDLErrorHint 的输入（Task 1 生产）。
type idlErrBriefForTest = idlErrBrief

func TestAppendIDLErrorHint(t *testing.T) {
	briefs := []idlErrBrief{
		{App: "credential", Code: 521, Name: "VcRequired", Message: "需要先持有合规凭证"},
		{App: "identity", Code: 1026, Name: "DidDeactivated", Message: "DID 已停用"},
	}
	cases := []struct{ name, in, wantSub string }{
		{"按错误名命中", "exec failed: VcRequired for account 2DS..", "VcRequired(code 521)"},
		{"大小写不敏感", "error: didrequired state", "DidDeactivated"}, // 注意：didrequired 含子串 didrequired≠DidDeactivated；此用例验证的是子串大小写归一，用真实形态
		{"无命中原样返回", "totally unrelated", ""},
	}
	for _, c := range cases {
		got := appendIDLErrorHint(c.in, briefs)
		if c.wantSub == "" {
			if got != c.in {
				t.Errorf("%s: 无命中应原样返回, got %q", c.name, got)
			}
			continue
		}
		if !strings.Contains(got, c.wantSub) {
			t.Errorf("%s: 应含 %q, got %q", c.name, c.wantSub, got)
		}
		if !strings.Contains(got, c.in) {
			t.Errorf("%s: 不得吞掉原始错误", c.name)
		}
	}
}
```

注意第二个用例的注释：实际写测试时用 `{"大小写不敏感", "error: vcrequired", "VcRequired(521)"}`——子串匹配按 `strings.Contains(lower(msg), lower(name))`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run TestAppendIDLErrorHint -v`
Expected: FAIL `undefined: idlErrBrief`

- [ ] **Step 3: 实现**（error_hints.go 追加）

```go
// idlErrBrief 是 IDL 错误码的最小画像（app 名 + 码 + 名 + 说明）。
type idlErrBrief struct {
	App     string
	Code    int64
	Name    string
	Message string
}

// appendIDLErrorHint 在错误消息中匹配 IDL 错误名（大小写不敏感子串），
// 命中则追加「链端错误对照」段；无匹配原样返回，永不吞错。
func appendIDLErrorHint(msg string, briefs []idlErrBrief) string {
	lower := strings.ToLower(msg)
	var hits []string
	for _, b := range briefs {
		if b.Name != "" && strings.Contains(lower, strings.ToLower(b.Name)) {
			hits = append(hits, fmt.Sprintf("%s(code %d)：%s（app:%s）", b.Name, b.Code, b.Message, b.App))
		}
	}
	if len(hits) == 0 {
		return msg
	}
	return msg + "；链端错误对照：" + strings.Join(hits, "；")
}

// withChainErrorHintFor = withChainErrorHint（高频带处置建议）+ IDL 码表自动对照。
// 从请求上下文聚合全部 app 的 IDL 错误码（与 error_lookup 同数据源）。
func withChainErrorHintFor(c *gin.Context, msg string) string {
	msg = withChainErrorHint(msg)
	mc := middleware.ClientFrom(c)
	if mc == nil {
		return msg
	}
	var briefs []idlErrBrief
	for appName, pd := range mc.GetAllPd() {
		for _, e := range pd.IDL.Errors {
			briefs = append(briefs, idlErrBrief{App: appName, Code: int64(e.Code), Name: e.Name, Message: e.Message})
		}
	}
	return appendIDLErrorHint(msg, briefs)
}
```

imports 需补 `fmt`、`milon-api-server/middleware`、`github.com/gin-gonic/gin`。
注意：`e.Code` 的类型以 SDK IDL 定义为准（可能是 uint16/uint32），编译不过就按实际类型调整 idlErrBrief.Code。

- [ ] **Step 4: 跑测试通过**

Run: `go test ./handler/ -run 'TestAppendIDLErrorHint|TestChainErrorHints' -v`
Expected: PASS（旧测试不回归）

- [ ] **Step 5: 提交**

```bash
git add handler/error_hints.go handler/error_hints_test.go
git commit -m "feat(mcp): 错误提示码表驱动——IDL 错误名自动对照（withChainErrorHintFor）"
```

---

### Task 2: 刀1b — 补接线 flow 系 handler

**Files:**
- Modify: `handler/did_handler.go`、`handler/vc_flow_handler.go`、`handler/sft_flow_handler.go`、`handler/saved_instruction_handler.go`（SDK 错误响应点）
- Test: `handler/did_handler_test.go`（追加一条）

**Interfaces:**
- Consumes: `withChainErrorHintFor(c, msg)`（Task 1）

- [ ] **Step 1: 写失败测试**（did SDK 错误路径消息含提示）。参照 transfer_mil_test.go 的 fake RPC 模式：fake 后端对 DID 写请求返回含 `DidDeactivated` 的错误 body，断言响应 message 含「链端错误对照」。

```go
// TestDidWriteSdkErrorCarriesHint：SDK 错误消息应带 IDL 错误对照提示。
func TestDidWriteSdkErrorCarriesHint(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"request_id":1,"status":6,"error":{"message":"identity rejected: DidDeactivated"}}`))
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	api.POST("/tool/did/set-alias", NewDidHandler(nm).SetAlias)
	w := httptest.NewRecorder()
	body := `{"privateKey":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","address":"2DSTx","alias":"a"}`
	req := httptest.NewRequest(http.MethodPost, "/api/tool/did/set-alias", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "链端错误对照") {
		t.Errorf("SDK 错误响应应含 IDL 对照提示: %s", w.Body.String())
	}
}
```

注意：fake RPC 的错误响应 shape 以 SDK 实际解析形态为准——先跑一次看 handler 是否走到 `logSDKError` 分支（若 fake shape 不对会 200 成功路径，测试失败信息可读）。DID handler 的构造函数名/路由注册以 `main.go` 实际为准（先 grep 确认 `did/set-alias` 的 handler 方法名）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run TestDidWriteSdkErrorCarriesHint -v`
Expected: FAIL（消息无「链端错误对照」）

- [ ] **Step 3: 接线**——四个 handler 文件中 SDK 错误的 `ErrorResponse(..., "failed to xxx: "+err.Error(), ...)` 逐个改为 `withChainErrorHintFor(c, err.Error())`。定位方式：

```bash
grep -n 'err.Error(), nil' handler/did_handler.go handler/vc_flow_handler.go handler/sft_flow_handler.go handler/saved_instruction_handler.go
```

只改 SDK/链端错误点（logSDKError 附近的），参数校验 400 不动。消息形态 `"failed to X: " + withChainErrorHintFor(c, err.Error())`。

- [ ] **Step 4: 全量测试通过**

Run: `go test ./handler/ -v 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add handler/did_handler.go handler/vc_flow_handler.go handler/sft_flow_handler.go handler/saved_instruction_handler.go handler/did_handler_test.go
git commit -m "feat(mcp): did/vc_flow/sft_flow/saved_instruction SDK 错误补挂码表提示"
```

---

### Task 3: 刀2 — vc_attestation 静默失败 spike

**Files:**
- 可能 Modify: `handler/vc_attestation_handler.go` 或 `mcpserver/`（视定位结果）
- Test: 视结果补

**Interfaces:** 无新接口；产出是结论（修复或关闭记录）。

- [ ] **Step 1: REST 直调复现**——服务跑在 18080（`SERVER_PORT=18080` 启动，或用已有进程）。用 `vc_cert_tokens_testdata.json` 里 22KH 开头账户的 ed25519 私钥：

```bash
curl -s http://127.0.0.1:18080/api/util/vc-attestation -H 'Content-Type: application/json' -d @vc_rqc_params.json | head -c 2000
```

断言：`success:true` 且 data 有完整 attestation 字段。

- [ ] **Step 2: MCP 路径对比**——同一参数走 MCP 工具 `mcp__milon__vc_attestation`（本会话即可调），对比 REST 直调结果：字段是否齐全、有无被截断/置空。

- [ ] **Step 3: 定位**——差异在 executor 回环（16MB LimitReader/信封判定）则修 mcpserver；handler 本身吞错则修 handler（错误显式化，绝不吞）。

- [ ] **Step 4: 修复 + 回归测试**（若可复现）。无法复现则在 spec 追加「2026-11-11 复测通过，疑为当时 devNet 环境」结论并跳到 Task 4。

- [ ] **Step 5: 提交**（若有修复）

```bash
git commit -m "fix(mcp): vc_attestation 经 MCP 路径静默失败——<根因>"
```

---

### Task 4: 刀3a — idl_methods 加 method 单方法过滤

**Files:**
- Modify: `handler/idl_handler.go:562`（AppMethods）
- Modify: `mcpserver/tools.go`（idlMethodsArgs 加 Method + schema 描述）
- Test: `handler/idl_filter_test.go`（追加）

**Interfaces:**
- Produces: REST `GET /api/idl/apps/:appName/methods?method=Transfer` 只返回单方法；MCP `idl_methods` 多可选参数 `method`

- [ ] **Step 1: 写失败测试**（参照 idl_filter_test.go 现有 fake 模式——先读该文件确认其 fake RPC 怎么提供 IDL 数据）

```go
// TestAppMethodsSingleMethodFilter：method=Transfer 只回一个方法；未知名 400 带可用清单。
func TestAppMethodsSingleMethodFilter(t *testing.T) {
	r := newIDLRouter(t) // 复用 idl_filter_test.go 现有组装函数（名字以文件实际为准）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/idl/apps/token/methods?method=transfer", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	var env struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	// 断言形态以现有 AppMethods 返回为准：过滤后 instructions/methods 只含 Transfer
	if len(env.Data) == 0 {
		t.Fatal("data 不应为空")
	}

	// 未知名 → 400 报可用方法
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/idl/apps/token/methods?method=NoSuch", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest || !strings.Contains(w2.Body.String(), "Transfer") {
		t.Errorf("未知名应 400 且报可用清单: %d %s", w2.Code, w2.Body.String())
	}
}
```

注意：`newIDLRouter` 及 data 结构以 idl_filter_test.go 现有代码为准（先读再写，组装函数名可能不同；返回结构可能是 `[]idlAppMeta` 而非 map——断言按实际调整，核心是「只回一个方法」+「未知名 400 带清单」）。

- [ ] **Step 2: 确认失败**

Run: `go test ./handler/ -run TestAppMethodsSingleMethodFilter -v`
Expected: FAIL（method 参数被忽略，返回全量）

- [ ] **Step 3: 实现**（AppMethods 内，pd 取到后）

```go
	method := strings.TrimSpace(c.Query("method"))
	meta := buildAppMeta(appName, pd)
	if method != "" {
		want := strings.ToLower(method)
		var kept []idlInstructionMeta
		for _, m := range meta.Instructions {
			if strings.ToLower(m.Name) == want {
				kept = append(kept, m)
			}
		}
		if len(kept) == 0 {
			var available []string
			for _, m := range meta.Instructions {
				available = append(available, m.Name)
			}
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
				fmt.Sprintf("unknown method %q in app %s；可用: %v", method, appName, available), nil))
			return
		}
		meta.Instructions = kept
		c.JSON(http.StatusOK, types.SuccessResponse(meta, "ok"))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(meta, "ok"))
```

（idlAppMeta 的字段名以 idl_handler.go 实际 struct 为准——若 Methods 字段名不同按实际改。）

MCP 镜像（tools.go idlMethodsArgs）：

```go
type idlMethodsArgs struct {
	AppName string `json:"appName" jsonschema:"必填：app 名（idl_apps 返回的 name，如 token）"`
	Method  string `json:"method,omitempty" jsonschema:"可选：只返回单个方法的明细（强烈建议传——省去全 app 几十个方法的返回量）；方法名大小写宽容，用 idl_methods 全量返回或 contract 错误提示里的名字"`
}
```

RESTMapping 补 `QueryParams: []string{"appName", "method"}`——注意 appName 是 PathParams，method 进 QueryParams。tools.go 注册处同步。

- [ ] **Step 4: 测试通过 + MCP 契约测试同步**

Run: `go test ./handler/ -run TestAppMethodsSingleMethodFilter -v && go test ./mcpserver/ -v 2>&1 | tail -3`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add handler/idl_handler.go handler/idl_filter_test.go mcpserver/tools.go
git commit -m "feat(idl): idl_methods 支持单方法过滤——AI 查参数不再拉全量几十个方法"
```

---

### Task 5: 刀3b — tx_get/tx_track 加 view=summary

**Files:**
- Modify: `handler/transaction_handler.go`（GetTransactionByHash:202、TrackTransaction:299、WaitForTransaction:259）
- Modify: `mcpserver/tools.go`（txGet/txTrack/txWait args 加 View 字段）
- Test: `handler/transaction_track_test.go`（追加）

**Interfaces:**
- Produces: REST `?view=summary`；MCP `view` 可选参数（schema 标注 AI 推荐 summary）

- [ ] **Step 1: 写失败测试**（参照 transaction_track_test.go 现有 fake 模式）

```go
// TestTxGetSummaryView：view=summary 只回摘要字段（无 instructions/signatures/access 原文）。
func TestTxGetSummaryView(t *testing.T) {
	r := newTrackRouter(t) // 以 transaction_track_test.go 现有组装为准
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/transactions/0123abcd/view-test?view=summary", nil) // hash 以 fake 数据为准
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, banned := range []string{`"instructions"`, `"signatures"`, `"access"`} {
		if strings.Contains(body, banned) {
			t.Errorf("summary 不应含 %s: %.200s", banned, body)
		}
	}
	for _, want := range []string{`"summary":true`, `"eventsCount"`} {
		if !strings.Contains(body, want) {
			t.Errorf("summary 应含 %s: %.200s", want, body)
		}
	}
	// 缺省 full 不破坏现状：不带 view 时应有 instructions
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/transactions/0123abcd/view-test", nil)
	r.ServeHTTP(w2, req2)
	if !strings.Contains(w2.Body.String(), `"instructions"`) {
		t.Errorf("缺省应保持全量: %.200s", w2.Body.String())
	}
}
```

- [ ] **Step 2: 确认失败**

Run: `go test ./handler/ -run TestTxGetSummaryView -v`
Expected: FAIL（view 参数被忽略，全量返回）

- [ ] **Step 3: 实现投影函数**（transaction_handler.go 追加）

```go
// txSummaryResponse 是 view=summary 的轻量投影：只留 AI 收尾判断所需字段，
// 砍掉 instructions(hex 大数组)/signatures/access 原文。
type txSummaryResponse struct {
	Summary     bool   `json:"summary"`
	Stamp       uint64 `json:"stamp"`
	Payer       *uint8 `json:"payer,omitempty"`
	Error       *uint16 `json:"error,omitempty"`
	GasCharged  uint64 `json:"gasCharged"`
	EventsCount int    `json:"eventsCount"`
	// InstructionDigest：instructions 的 sha256 前 16 hex——同交易可判等价，不占体积
	InstructionDigest string `json:"instructionDigest"`
}

func summarizeTxHistory(r txHistoryResponse) txSummaryResponse {
	d := sha256.Sum256([]byte(strings.Join(r.Instructions, ",")))
	ev := 0
	if r.Receipt.Events != nil {
		ev = len(r.Receipt.Events)
	}
	return txSummaryResponse{
		Summary: true, Stamp: r.Stamp, Payer: r.Payer, Error: r.Receipt.Error,
		GasCharged: r.Receipt.GasCharged, EventsCount: ev,
		InstructionDigest: hex.EncodeToString(d[:8]),
	}
}
```

GetTransactionByHash / WaitForTransaction / TrackTransaction 三处在 `c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))` 前加：

```go
	if c.DefaultQuery("view", "full") == "summary" {
		c.JSON(http.StatusOK, types.SuccessResponse(summarizeTxHistory(resp), "ok"))
		return
	}
```

TrackTransaction 的响应是 map（status/摘要/事件聚合）——对它只投影「交易摘要」子对象：把摘要字段的构造换为 summarizeTxHistory（保持 status/events 结构不变）。以 TrackTransaction 现有代码实际形态为准。

MCP 侧：txParseArgs 附近加 `view` 字段到 tx_get/tx_track/tx_wait 的 args（txWaitArgs/txTrackArgs/hashArgs 共用问题：hashArgs 是共享 struct——tx_get 用 hashArgs，加 View 需要独立 struct 或扩展 hashArgs。做法：新建 `txViewArgs struct { Hash string; View string }` 给 tx_get；txTrackArgs/txWaitArgs 直接加 View 字段）。RESTMapping QueryParams 补 "view"。schema 描述：`"可选：summary 只回状态/事件数/摘要（AI 推荐，省 token）；full=全量（缺省）"`。

- [ ] **Step 4: 测试通过**

Run: `go test ./handler/ -run 'TestTxGetSummaryView|TestTrack' -v && go test ./mcpserver/ 2>&1 | tail -3`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add handler/transaction_handler.go handler/transaction_track_test.go mcpserver/tools.go
git commit -m "feat(tx): tx_get/tx_wait/tx_track 支持 view=summary 轻量投影——AI 收尾省 token"
```

---

### Task 6: 刀3c — account_resources 加 prefix 过滤

**Files:**
- Modify: `handler/account_handler.go:59`（GetAccountResources）
- Modify: `mcpserver/tools.go`（account_resources 独立 args 加 Prefix）
- Test: `handler/accounts_contract_test.go`（追加，先读确认现有 fake 模式）

**Interfaces:**
- Produces: REST `GET /api/accounts/:address/resources?prefix=token`；MCP `prefix` 可选参数

- [ ] **Step 1: 写失败测试**——fake 账户资源（以 accounts_contract_test.go 现有 fake 的资源结构为准），断言 `?prefix=xxx` 只回路径含前缀的资源、无 prefix 全量（现状不变）。

- [ ] **Step 2: 确认失败** — `go test ./handler/ -run TestAccountResourcesPrefix -v` FAIL

- [ ] **Step 3: 实现**（GetAccountResources 内，拿到 result 后）

```go
	if prefix := c.Query("prefix"); prefix != "" {
		// BodyAccountView 的资源集合结构以 SDK 实际类型为准（先读 client 或
		// accounts_contract_test.go 的 fake 响应确定字段名），按资源标识前缀过滤。
		// 若 BodyAccountView 是 map[string]any / Resources 列表，过滤逻辑不同——
		// 以实测类型为准，核心是 strings.HasPrefix(资源标识, prefix)。
	}
```

（此处需要执行者先看 fake 测试里 BodyAccountView 的真实 JSON 形态再写过滤——不是占位符，是该 handler 返回结构由 SDK 定义、计划无法预知字段名；测试先行会暴露结构。）

MCP 侧：tools.go 的 account_resources 现用共享 `addressArgs`——新建 `accountResourcesArgs struct { Address, Prefix string }`，QueryParams 补 "prefix"。schema：`"可选：资源路径前缀过滤（如 token），只回命中的资源——大账户省 token"`。

- [ ] **Step 4: 测试通过** — `go test ./handler/ -run TestAccountResourcesPrefix -v && go test ./mcpserver/ 2>&1 | tail -3`

- [ ] **Step 5: 提交**

```bash
git add handler/account_handler.go handler/accounts_contract_test.go mcpserver/tools.go
git commit -m "feat(account): account_resources 支持 prefix 前缀过滤——大账户资源列表瘦身"
```

---

### Task 7: 刀4a — transfer_mil REST 加 wait

**Files:**
- Modify: `handler/transfer_mil_handler.go`（request 加 Wait + 提交后可选等待）
- Test: `handler/transfer_mil_test.go`（追加）

**Interfaces:**
- Produces: REST `POST /api/tool/transfer-mil` body 加可选 `wait bool`（缺省 false=现状）；wait=true 响应附 `confirmed`/`waitError`

- [ ] **Step 1: 写失败测试**（fake RPC：提交成功 + 等待轮询返回已确认——参照 write_wait_test.go 的 fake 模式，先读该文件）

```go
// TestTransferMilWaitTrue：wait=true 时响应含 confirmed 字段；缺省不含。
func TestTransferMilWaitTrue(t *testing.T) {
	// fake 按 write_wait_test.go 的两段式：submit → 轮询 wait 返回确认形态
	// 断言1：body {"wait":true, ...} → 200 且响应含 "confirmed"
	// 断言2：body 不带 wait → 200 且响应不含 "confirmed"（现状不变）
}
```

- [ ] **Step 2: 确认失败** — `go test ./handler/ -run TestTransferMilWaitTrue -v` FAIL

- [ ] **Step 3: 实现**

transferMilRequest 加：

```go
	// Wait=true 时提交后等待确认（缺省 false 立即返回 txHash——现状）。
	Wait bool `json:"wait,omitempty"`
```

dispatchSubmit 成功后：

```go
	respBody := gin.H{
		"txHash": txHash, "from": fromAddr, "to": req.To, "amount": req.Amount,
		"token": milTokenAddress, "rawTx": serializeTx(tx),
	}
	if req.Wait {
		_, werr := mc.WaitForTransaction(txHash, milon.WithWaitRequestID(requestId), milon.WithWaitPollTimeout(60*time.Second))
		if werr != nil {
			respBody["confirmed"] = false
			respBody["waitError"] = withChainErrorHint(werr.Error())
		} else {
			respBody["confirmed"] = true
		}
		respBody["tip"] = "已等待确认，confirmed=true 即上链成功；false 用 tx_hash 查 rawError"
		_ = txHash // keep
	} else {
		respBody["tip"] = "用 tx_track 工具（GET /api/transactions/{hash}/track）跟踪确认"
	}
```

（以 faucet_handler.go:119-128 的等待模式为准对齐；txHash 变量名以现有代码为准。）

- [ ] **Step 4: 测试通过** — `go test ./handler/ -run TestTransferMil -v`

- [ ] **Step 5: 提交**

```bash
git add handler/transfer_mil_handler.go handler/transfer_mil_test.go
git commit -m "feat(transfer): transfer_mil 支持 wait=true 同请求等确认——REST 缺省不变"
```

---

### Task 8: 刀4b — MCP DefaultArgs 缺省注入机制

**Files:**
- Modify: `mcpserver/executor.go`（RESTMapping 加 DefaultArgs + Call 合并）
- Modify: `mcpserver/tools.go`（四个写工具配 wait:true 注入 + schema 描述更新）
- Test: `mcpserver/executor_test.go`（追加）

**Interfaces:**
- Produces: `RESTMapping.DefaultArgs map[string]any`——executor.Call 在 unmarshal 后合入缺失键（用户显式值优先）

- [ ] **Step 1: 写失败测试**（executor_test.go 追加）

```go
// TestDefaultArgsInjection：DefaultArgs 只补缺失键，不覆盖用户显式值。
func TestDefaultArgsInjection(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	exec := NewExecutor(srv.URL, nil)
	m := RESTMapping{Method: "POST", PathTemplate: "/api/write", DefaultArgs: map[string]any{"wait": true}}

	// 未传 wait → 注入 true
	if _, err := exec.Call(context.Background(), m, json.RawMessage(`{"appName":"token"}`)); err != nil {
		t.Fatal(err)
	}
	if gotBody["wait"] != true {
		t.Errorf("缺省应注入 wait=true, got %v", gotBody["wait"])
	}

	// 显式 wait:false → 尊重用户值
	if _, err := exec.Call(context.Background(), m, json.RawMessage(`{"appName":"token","wait":false}`)); err != nil {
		t.Fatal(err)
	}
	if gotBody["wait"] != false {
		t.Errorf("显式 false 不得被覆盖, got %v", gotBody["wait"])
	}
}
```

- [ ] **Step 2: 确认失败** — `go test ./mcpserver/ -run TestDefaultArgsInjection -v` FAIL（无 DefaultArgs 字段）

- [ ] **Step 3: 实现**（executor.go）

```go
type RESTMapping struct {
	Method       string
	PathTemplate string
	PathParams   []string
	QueryParams  []string
	// DefaultArgs：MCP 侧参数缺省注入（用户显式值优先）。键 = 参数名。
	// 仅用于呈现层便利（如 wait 缺省 true），REST 缺省行为不受影响。
	DefaultArgs map[string]any
}
```

Call 里 unmarshal 之后、PathParams 处理之前：

```go
	for k, v := range m.DefaultArgs {
		if _, exists := args[k]; !exists {
			if args == nil {
				args = map[string]any{}
			}
			args[k] = v
		}
	}
```

- [ ] **Step 4: 配置注入 + schema 描述**

tools.go 注册处给四个工具加 DefaultArgs：

```go
	RESTMapping{Method: "POST", PathTemplate: "/api/write", DefaultArgs: map[string]any{"wait": true}}
	RESTMapping{Method: "POST", PathTemplate: "/api/write/multi", DefaultArgs: map[string]any{"wait": true}}
	RESTMapping{Method: "POST", PathTemplate: "/api/tool/transfer-mil", DefaultArgs: map[string]any{"wait": true}}
	RESTMapping{Method: "POST", PathTemplate: "/api/saved-instructions/{id}/execute", PathParams: []string{"id"}, QueryParams: []string{"mode", "wait"}, DefaultArgs: map[string]any{"wait": true}}
```

saved_instruction_execute 的 wait 是 QueryParam——注入进 args 后 QueryParams 处理会自然带上（Call 里 QueryParams 从 args 取）。四个工具对应 args struct 的 Wait 字段 jsonschema 描述改为「可选：MCP 侧缺省 true（同请求等确认省一轮 tx_track）；不等待显式传 false」。transfer_mil 的 transferMilArgs 需加 Wait 字段（Task 7 已加 REST 支持）：

```go
	Wait bool `json:"wait,omitempty" jsonschema:"可选：MCP 侧缺省 true（同请求等确认）；REST 缺省 false；不等待显式传 false"`
```

注意：saved_instruction_execute 的 wait REST 是 string query（"true"/"false"）——DefaultArgs 注入 bool true 后 `fmt.Sprintf("%v", v)` 渲染为 "true"，与 handler 的 DefaultQuery 解析对齐（先 grep handler 侧 wait 解析确认 bool/string）。

- [ ] **Step 5: 测试 + 契约同步通过**

Run: `go test ./mcpserver/ -v 2>&1 | tail -3`
Expected: PASS（TestSchemaRequiredMatchesREST 若因 Wait 字段 required 集变化而红，按 REST 事实源同步——Wait 是 omitempty 不进 required，应无变化）

- [ ] **Step 6: 提交**

```bash
git add mcpserver/executor.go mcpserver/executor_test.go mcpserver/tools.go
git commit -m "feat(mcp): 写路径 wait 缺省注入——发+等并成一跳（REST 缺省不变）"
```

---

### Task 9: 刀5a — DID 细粒度 8 个合并为 did_update

**Files:**
- Modify: `mcpserver/tools.go`（新增 didUpdateArgs + registerRoutedTool；删 8 个旧注册与旧 args struct）
- Test: `mcpserver/tools_test.go`（追加 did_update 契约测试；改 TestToolInventory 期望数）

**Interfaces:**
- Produces: `registerRoutedTool[In any](srv, exec, name, desc string, route func(args In) (RESTMapping, map[string]any, error))`——按参数路由到不同 REST 端点（Task 10/11 复用）
- Produces: MCP 工具 `did_update`（action 八选一）；工具数 64 → 57

- [ ] **Step 1: 写失败测试**（tools_test.go 追加——参照现有契约测试模式，先读 tools_test.go 里 DID 工具的现有测试怎么断言 schema）

```go
// TestDidUpdateToolContract：action 枚举/字段组齐备；旧 8 工具不再注册。
func TestDidUpdateToolContract(t *testing.T) {
	srv := BuildServer("http://127.0.0.1:1")
	// 用 SDK 列工具（参照 tools_test.go 现有取工具 schema 的辅助函数）
	// 断言1：did_update 存在，description 含全部 8 个 action 名
	// 断言2：did_set_alias/did_add_service/did_update_service/did_remove_service/
	//        did_set_avatar_uri/did_add_key/did_update_key/did_remove_key 不存在
	// 断言3：did_update 的 inputSchema 有 action 字段
}
```

- [ ] **Step 2: 确认失败** — `go test ./mcpserver/ -run TestDidUpdateToolContract -v` FAIL

- [ ] **Step 3: 实现 registerRoutedTool**（tools.go，registerTool 旁）

```go
// registerRoutedTool 注册按参数路由到不同 REST 端点的合并工具：route 回调
// 依据 args 产出目标映射与 body（body 里不含路由键）。inventory/annotations/
// outputSchema 与 registerTool 同规则。
func registerRoutedTool[In any](srv *mcp.Server, exec *Executor, name, desc string,
	route func(args In) (RESTMapping, map[string]any, error)) {
	if !inventorySeen[name] {
		inventorySeen[name] = true
		inventory = append(inventory, ToolInfo{
			Name: name, Description: desc, Group: toolGroupOf(name),
			Method: "ROUTED", Path: "(multi)",
		})
	}
	mcp.AddTool[In, any](srv, &mcp.Tool{
		Name: name, Title: toolTitleOf(name, desc), Description: desc,
		Annotations: toolAnnotationsFor(name, RESTMapping{Method: "POST"}),
		OutputSchema: envelopeOutputSchema,
	},
		func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, any, error) {
			m, body, err := route(args)
			if err != nil {
				return &mcp.CallToolResult{IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil, nil
			}
			raw, _ := json.Marshal(body)
			out, err := exec.Call(ctx, m, raw)
			if err != nil {
				return nil, nil, err
			}
			isError := out.IsError
			var structured any
			if success, ok := parseEnvelope(out.Body); ok {
				isError = isError || !success
				structured = json.RawMessage(out.Body)
			}
			return &mcp.CallToolResult{IsError: isError,
				Content: []mcp.Content{&mcp.TextContent{Text: string(out.Body)}}}, structured, nil
		})
}
```

- [ ] **Step 4: 实现 did_update**（args + 注册；route 内 switch action）

```go
// didUpdateArgs 合并 DID 八个细粒度写操作（action 路由）。公共身份字段
// 镜像 didMutateArgs；各 action 专属字段如下，handler 侧校验规则不变。
type didUpdateArgs struct {
	didMutateArgs
	Action string `json:"action" jsonschema:"必填：八选一——set_alias/add_service/update_service/remove_service/set_avatar_uri/add_key/update_key/remove_key"`
	// set_alias 专用
	Alias  string  `json:"alias,omitempty" jsonschema:"set_alias 必填：别名主体（最终「alias-数字」）"`
	Suffix *uint32 `json:"suffix,omitempty" jsonschema:"可选：数字后缀（set_alias）；缺省服务端代填，撞名换号重试"`
	// service 专用
	Label           string `json:"label,omitempty" jsonschema:"add_service/update_service 必填：服务标签"`
	ServiceEndpoint string `json:"serviceEndpoint,omitempty" jsonschema:"add_service/update_service 必填：绝对 URI"`
	// id 定位（update/remove service、update/remove key）
	Id *uint8 `json:"id,omitempty" jsonschema:"update_service/remove_service/update_key/remove_key 必填：目标 id（从 did_document 取）"`
	// 头像
	AvatarURI string `json:"avatarUri,omitempty" jsonschema:"set_avatar_uri 必填：头像 URI（1-512 字节）"`
	// 密钥
	NewPublicKey string  `json:"newPublicKey,omitempty" jsonschema:"add_key/update_key 必填：新公钥（base58）"`
	Lbl          *string `json:"label,omitempty" jsonschema:"可选：密钥标签（add_key/update_key）；注意与 service 的 label 区分，密钥标签仅展示用"`
}
```

冲突注意：service 的 label（string）与 key 的 label（*string）——上面合并为两个字段有 json tag 冲突风险（都想要 "label"）。决断：key 标签放弃独立字段，add_key/update_key 场景 label 复用同一 `Label string`（handler 侧 didAddKeyArgs.Label 是 *string 可选——透传 string 即可，空串不传）。route 构造 body 时按 action 组装，只带该 action 的字段。

route 实现（八分支，每支给出端点+body）：

```go
func didUpdateRoute(a didUpdateArgs) (RESTMapping, map[string]any, error) {
	base := map[string]any{"privateKey": a.PrivateKey, "publicKey": a.PublicKey, "address": a.Address}
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "set_alias":
		return RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/set-alias"},
			mergeBody(base, map[string]any{"alias": a.Alias, "suffix": a.Suffix}), nil
	case "add_service":
		return RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/add-service"},
			mergeBody(base, map[string]any{"label": a.Label, "serviceEndpoint": a.ServiceEndpoint}), nil
	case "update_service":
		return RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/update-service"},
			mergeBody(base, map[string]any{"id": a.Id, "label": a.Label, "serviceEndpoint": a.ServiceEndpoint}), nil
	case "remove_service":
		return RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/remove-service"},
			mergeBody(base, map[string]any{"id": a.Id}), nil
	case "set_avatar_uri":
		return RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/set-avatar-uri"},
			mergeBody(base, map[string]any{"avatarUri": a.AvatarURI}), nil
	case "add_key":
		return RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/add-key"},
			mergeBody(base, map[string]any{"newPublicKey": a.NewPublicKey, "label": a.Lbl}), nil
	case "update_key":
		return RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/update-key"},
			mergeBody(base, map[string]any{"id": a.Id, "newPublicKey": a.NewPublicKey, "label": a.Lbl}), nil
	case "remove_key":
		return RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/remove-key"},
			mergeBody(base, map[string]any{"id": a.Id}), nil
	}
	return RESTMapping{}, nil, fmt.Errorf(
		"未知 action %q；可用：set_alias/add_service/update_service/remove_service/set_avatar_uri/add_key/update_key/remove_key", a.Action)
}

// mergeBody 合并 base 与 extra，跳过零值键（nil 指针/空串不透传，交 handler 缺省）。
func mergeBody(base, extra map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		if v == nil || v == "" {
			continue
		}
		out[k] = v
	}
	return out
}
```

Lbl 字段上面提到的冲突决断落为：args 只留一个 `Label string`（json:"label"），add_key/update_key 也读它。修正上面 struct：删掉 Lbl，route 的 add_key/update_key 用 `a.Label`。

注册（替换旧 8 个 registerTool 调用）：

```go
	registerRoutedTool[didUpdateArgs](srv, exec, "did_update",
		"DID 文档细粒度更新（action 八选一）：set_alias 绑定/换别名 · add_service/update_service/remove_service 服务端点管理 · set_avatar_uri 换头像 · add_key/update_key/remove_key 公钥管理（remove 系不可逆；id 从 did_document 取）；响应均带最新文档",
		didUpdateRoute)
```

删除旧 8 个注册调用与对应 args struct（didSetAliasArgs/didAddServiceArgs/didUpdateServiceArgs/didRemoveServiceArgs/didSetAvatarUriArgs/didAddKeyArgs/didUpdateKeyArgs/didRemoveKeyArgs）。destructiveWrites：did_update 不入静态名单（action 动态），描述已注明 remove 不可逆。guide.go workflows/limits 页若有提及旧工具名同步改为 did_update。

- [ ] **Step 5: 测试通过 + 契约测试同步**

Run: `go test ./mcpserver/ -v 2>&1 | tail -5`
Expected: PASS（TestToolInventory 等期望工具数的断言按 57 更新；guide_test 若断言旧工具名同步）

- [ ] **Step 6: 提交**

```bash
git add mcpserver/tools.go mcpserver/tools_test.go mcpserver/guide.go
git commit -m "feat(mcp): DID 细粒度 8 工具合并为 did_update（action 路由）——工具数 64→57"
```

---

### Task 10: 刀5b — saved_instruction 6 → 4

**Files:**
- Modify: `mcpserver/tools.go`
- Test: `mcpserver/tools_test.go`

**Interfaces:**
- Consumes: `registerRoutedTool`、`mergeBody`（Task 9）
- Produces: `saved_instruction_upsert`（id 空=新建，传=部分更新）、`saved_instruction_query`（id 空=清单，传=单条）；工具数 57 → 55

- [ ] **Step 1: 写失败测试**——同 Task 9 模式：断言新两工具存在且 description 说明 id 语义；旧 create/update/list/get 四名不存在；execute/delete 保留。

- [ ] **Step 2: 确认失败**

- [ ] **Step 3: 实现**

```go
// savedInstructionUpsertArgs 合并 create（Task 9 前的 create）与 update：id 空=新建，
// 传=部分更新（update 的全字段 nil-able 语义由 omitempty 保持）。
type savedInstructionUpsertArgs struct {
	Id              string             `json:"id,omitempty" jsonschema:"可选：传入=部分更新既有指令（未传字段不变）；缺省=新建"`
	Name            string             `json:"name,omitempty" jsonschema:"新建必填：展示名称；更新可选"`
	Description     string             `json:"description,omitempty" jsonschema:"可选：描述"`
	AppName         string             `json:"appName,omitempty" jsonschema:"新建必填：IDL app 名"`
	MethodName      string             `json:"methodName,omitempty" jsonschema:"新建必填：IDL 方法名"`
	Args            map[string]any     `json:"args,omitempty" jsonschema:"可选：方法参数对象"`
	PaymentMode     string             `json:"paymentMode,omitempty" jsonschema:"可选：六模式之一；entry 缺省 unified_payer_all"`
	PayerAddress    string             `json:"payerAddress,omitempty" jsonschema:"可选：payer 地址（私钥永不落盘）"`
	SignatureMode   *signatureModeSpec `json:"signatureMode,omitempty" jsonschema:"规则同 contract_write.signatureMode"`
	IxAddress       string             `json:"ixAddress,omitempty" jsonschema:"unified_dual_sign 专用"`
	IxSignatureMode *signatureModeSpec `json:"ixSignatureMode,omitempty" jsonschema:"规则同 signatureMode"`
	OwnerAddress    string             `json:"ownerAddress,omitempty" jsonschema:"split 专用"`
	Signers         []signerEntry      `json:"signers,omitempty" jsonschema:"multi_signer 专用（仅地址/签名模式入库）"`
	GasPayer        *signerEntry       `json:"gasPayer,omitempty" jsonschema:"multi_signer 可选：独立 gas 代付"`
}
```

route：id 空 → POST /api/saved-instructions（body 全量）；非空 → PUT /api/saved-instructions/{id}（PathParams 走不了（routed body 由我们构造）——route 返回的 RESTMapping 带 PathTemplate 已含 id：直接 `PathTemplate: "/api/saved-instructions/" + url.PathEscape(a.Id)`，PathParams 空）。

query 工具：

```go
type savedInstructionQueryArgs struct {
	Id string `json:"id,omitempty" jsonschema:"可选：传=查单条；缺省=列全部"`
}
```

route：id 空 → GET /api/saved-instructions；非空 → GET /api/saved-instructions/{id}。注意 GET 无 body——route 返回 body=nil，executor Call 对 GET 无 body 已处理（hasBody 仅 POST/PUT）。

- [ ] **Step 4: 测试通过**（工具数断言 55；受影响契约测试同步）

- [ ] **Step 5: 提交**

```bash
git commit -m "feat(mcp): saved_instruction create+update→upsert、list+get→query——工具数 57→55"
```

---

### Task 11: 刀5c — raw/view/multi 合并 + safe 并入

**Files:**
- Modify: `mcpserver/tools.go`
- Test: `mcpserver/tools_test.go`、`mcpserver/compliance_test.go`（annotations 断言同步）

**Interfaces:**
- Consumes: `registerRoutedTool`（Task 9）、`DefaultArgs`（Task 8）
- Produces: `tx_raw`（action: simulate/submit/inspect）、`view_call`（mode: single/multi）、`contract_multi`（action: simulate/write）、contract_write 加 `safe`（缺省 true，路由 write/write-safe）；工具数 55 → 50

- [ ] **Step 1: 写失败测试**——断言四组新形态存在、旧名（tx_simulate_raw/tx_submit_raw/tx_inspect_raw/view_single/view_multi/contract_simulate_multi/contract_write_multi/contract_write_safe）不存在；contract_write schema 有 safe 字段且描述注明缺省 true。

- [ ] **Step 2: 确认失败**

- [ ] **Step 3: 实现**

```go
// txRawArgs：三合一（action 路由）。参数镜像 rawTransactionRequest（同一 struct）。
type txRawArgs struct {
	Action             string `json:"action" jsonschema:"必填：三选一——simulate（干跑不耗 gas）/submit（真实上链）/inspect（解析不上链）"`
	TransactionPostcard string `json:"transactionPostcard" jsonschema:"base64 编码的交易 postcard 原文"`
}
// route: simulate→/api/transactions/simulate; submit→/submit; inspect→/inspect
// 注意 postButReadOnly 名单更新：删 tx_simulate_raw/tx_inspect_raw，加 tx_raw
// ——但 tx_raw 含 submit（写）！annotations 不能静态只读。决断：tx_raw 不入
// postButReadOnly（按最危险 action 计），description 注明 simulate/inspect 无副作用。

type viewCallArgs struct {
	Mode                string `json:"mode" jsonschema:"必填：single（单条 wire）/multi（多条打包）"`
	TransactionPostcard string `json:"transactionPostcard" jsonschema:"base64 编码的视图 postcard 原文"`
}
// route: single→/api/view/single; multi→/api/view/multi；入 postButReadOnly

type contractMultiActionArgs struct {
	contractMultiArgs
	Action string `json:"action" jsonschema:"必填：simulate（原子 dry-run）/write（真实上链）"`
}
// route: 剥掉 action 后透传 contractMultiArgs 全量 body → /api/simulate/multi 或 /api/write/multi
// 入 postButReadOnly？含 write——不入，description 注明。
```

contract_write safe 并入：contractWriteArgs 加

```go
	Safe bool `json:"safe,omitempty" jsonschema:"可选：true（缺省）先模拟再上链——模拟失败不上链只耗一次模拟；false 直接提交"`
```

注册改为 registerRoutedTool + DefaultArgs 双料：safe=true → POST /api/write-safe（body 剥 safe），false → POST /api/write。DefaultArgs 在 RESTMapping 上，但 routed 工具的 mapping 是 route 里动态产出的——决断：safe 缺省不走 DefaultArgs，而是 args struct 的 Safe 用 *bool：nil=缺省 true（route 里 nil 或 true → write-safe），显式 false → write。改用 `Safe *bool` 保持三态：

```go
	Safe *bool `json:"safe,omitempty" jsonschema:"可选：缺省 true 先模拟再上链（失败不上链）；显式 false 直接提交"`
```

writeSafeArgs 别名与 contract_write_safe 注册删除。compliance_test 的 annotations 断言（contract_write_safe 此前在 postButReadOnly 吗？不在——它是写）同步。

Wait 字段照旧在 body 透传（write/write-safe 的 REST body 已支持 wait——Task 8 已配 DefaultArgs；routed 后 DefaultArgs 挂在 route 返回的 mapping 上，两个分支都带）。

- [ ] **Step 4: 全部 mcpserver 测试通过**（工具数断言 50）

Run: `go test ./mcpserver/ -v 2>&1 | tail -5`

- [ ] **Step 5: 提交**

```bash
git commit -m "feat(mcp): tx_raw/view_call/contract_multi 合并+write_safe 并入 contract_write(safe 缺省开)——工具数 55→50"
```

---

### Task 12: 刀5d — devNet 冒烟 + 体积测量

**Files:**
- 无代码；产出验证记录（可写入 spec 尾部「验收记录」）

- [ ] **Step 1: 起服务** — `SERVER_PORT=18080` 启动新构建（`go build -o milon-api-server-test.exe && SERVER_PORT=18080 ./milon-api-server-test.exe`）

- [ ] **Step 2: 冒烟清单**（用 sft_1000_testdata.json / vc_cert_tokens_testdata.json 的既有账户私钥）：
  - did_update 八个 action 逐个（set_alias→add_service→update_service→remove_service→set_avatar_uri→add_key→update_key→remove_key，用测试账户）
  - tx_raw 三 action（先 inspect 一笔已有交易 postcard）
  - view_call 两 mode
  - contract_multi（simulate 一笔 token 查询类）
  - contract_write：safe 缺省路径 + 显式 safe:false 路径各一笔小额 transfer
  - transfer_mil：不传 wait（REST 缺省行为）+ MCP 路径 wait 注入（MCP 客户端连 18080/mcp 或直接 curl 模拟 tools/call）
  - idl_methods method=Transfer 单方法
  - tx_get view=summary
  - 错误路径：故意触发 Cooldown/未知 action，确认错误提示带对照

- [ ] **Step 3: 体积测量** — `MILON_MEASURE=1 go test ./mcpserver/ -run TestMeasureToolsListSize -v`：确认 TOOLS=50、BYTES ≤ 80000。不达标回 Task 9-11 查 schema 冗余。

- [ ] **Step 4: 提交冒烟记录**（追加到 spec 验收记录节）

---

### Task 13: 刀6 — instructions 强化 + 红线断言

**Files:**
- Modify: `mcpserver/tools.go`（serverInstructions）
- Modify: `mcpserver/measure_test.go`（转正式断言测试 TestToolsListBudget）
- Test: 同上

- [ ] **Step 1: 写失败测试**（measure_test.go 追加正式测试）

```go
// TestToolsListBudget：tools/list 红线——工具数 ≤52、总字节 ≤80000。
// 超线即失败：新增工具必须先瘦身（spec 防回弹红线）。
func TestToolsListBudget(t *testing.T) {
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{}})
	req, _ := http.NewRequest("POST", front.URL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(buf.Bytes(), &out)
	tools := out["result"].(map[string]any)["tools"].([]any)
	if len(tools) > 52 {
		t.Errorf("工具数 %d 超红线 52——新工具需先合并瘦身", len(tools))
	}
	total := 0
	for _, tl := range tools {
		b, _ := json.Marshal(tl)
		total += len(b)
	}
	if total > 80000 {
		t.Errorf("tools/list %d 字节超红线 80000——schema 描述需瘦身", total)
	}
}
```

- [ ] **Step 2: 跑测试**（Task 12 已保证 50/≤80000，应直接 PASS——它是防将来回弹的护栏）

- [ ] **Step 3: instructions 更新**

```go
const serverInstructions = `本服务（milon-api-server）完全开源，源代码：` + sourceRepoURL + `。
高频三招：查合约状态 = idl_methods(app, method 单方法) → contract_read；
转账 = transfer_mil（自动等确认）；写合约 = contract_write（safe 缺省开：先模拟再上链；wait 缺省开：同请求等确认）。
拿不准用法先调 guide 取主题页（overview/chain/workflows/limits/performance/keys），
错误码用 error_lookup 翻译（多数链端错误已内联对照提示）；手册或工具描述仍没讲清的，
可到上述仓库读源码对照——REST 端点在 handler/、MCP 壳与手册在 mcpserver/、链 SDK 在 gosdk-develop/`
```

guide.go 的 workflows/overview 页同步三招口径（transfer_mil 不再建议手动 tx_track、contract_write 提 safe/wait 缺省、did 细粒度改 did_update、idl_methods 提 method 过滤、tx_get 提 summary）。

- [ ] **Step 4: 全量测试 + guide 测试同步**

Run: `go test ./mcpserver/ ./handler/ 2>&1 | tail -3`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add mcpserver/tools.go mcpserver/measure_test.go mcpserver/guide.go mcpserver/guide_test.go
git commit -m "feat(mcp): instructions 三招地图+tools/list 红线断言（≤52 工具/≤80KB）——六刀收尾"
```

---

## Self-Review 记录

- Spec 覆盖：六刀 → Task 1-2（刀1）/ Task 3（刀2）/ Task 4-6（刀3）/ Task 7-8（刀4）/ Task 9-11+12（刀5）/ Task 13（刀6+红线）。无遗漏。
- 占位符：Task 6 Step 3 与 Task 3 存在「以实际结构为准」的探索步骤——因 SDK 返回结构由上游定义，测试先行会暴露结构，属明确的探索指令而非 TBD。
- 类型一致：registerRoutedTool/mergeBody（Task 9 产、10/11 用）；DefaultArgs（Task 8 产、11 用）；withChainErrorHintFor（Task 1 产、2 用）。工具数链：64→57（T9）→55（T10）→50（T11）。
