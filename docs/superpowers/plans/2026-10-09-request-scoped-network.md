# 请求级网络参数（方案 A）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 多用户共享同一 server 时，各自请求可通过 `X-Milon-Network` 头（MCP 工具用可选 `network` 参数）指定不同网络，互不干扰；不带头的请求行为与现状一致。

**Architecture:** gin 中间件 `ResolveNetwork` 读头 → `NetworkManager.ClientFor(name)` 按网解析（空名回落默认）→ 注入 context；35 处 handler 调用点改从 context 取 client；MCP 侧用泛型包装 `networkScoped[In]` 让 57 个工具 schema 自动获得可选 `network` 字段，executor 回环时转为请求头；前端顶栏选择器改本地偏好（localStorage）。

**Tech Stack:** Go + gin + modelcontextprotocol/go-sdk/mcp；原生 JS 单页前端。

**Spec:** `docs/superpowers/specs/2026-10-09-request-scoped-network-design.md`（本计划从 spec 出发，执行者需同时阅读 spec）

## Global Constraints

- 请求头名固定 `X-Milon-Network`（常量 `middleware.NetworkHeader`）；MCP 参数名 `network`。
- 不带头 = 走服务端默认网络，行为与现状逐字节一致（既有测试全部保持绿）。
- `network_switch` 端点与 MCP 工具保留，语义 =「设置服务端默认网络」。
- 未知网络名 → 400 `types.ERR_INVALID_PARAMETER`（=400），message 含未知名。
- 不改任何端点的 body/path/query 契约；网络只走头。
- 全程 TDD：先写测试看它失败，再实现看它通过；每任务独立 commit，中文 commit message。
- 前端静态资源版本号从 `?v=20261009` bump 到 `?v=20261010`。
- 网络集合仍由代码内置（localNet/devNet，`MILON_RPC_URL` 可覆盖 devNet RPC）——不做动态注册。

---

### Task 1: client 层 `ClientFor`

**Files:**
- Modify: `client/network_manager.go`（在 `Switch` 函数后新增方法）
- Test: `client/network_manager_test.go`（新建，client 包首个测试文件）

**Interfaces:**
- Consumes: 现有 `getOrCreateClient(name)`（已带缓存 + panic-recover）、`nm.currentNetwork`、`nm.networks`。
- Produces: `func (nm *NetworkManager) ClientFor(name string) (*milon.Client, milon.Network, error)`——Task 2 的中间件依赖它。注意锁纪律：**不能**持 `nm.mu` 调 `getOrCreateClient`（它自己拿写锁）。

- [ ] **Step 1: 写失败测试**

新建 `client/network_manager_test.go`：

```go
package client

import (
	"strings"
	"testing"
)

// TestClientFor 锁定请求级网络解析的三态 + 缓存复用 + RPC 覆盖。
// 注意 milon.NewClient 只加载 IDL 不拨号，localNet(127.0.0.1:6280 无服务)
// 也能创建成功，后续请求才失败——测试环境安全。
func TestClientFor(t *testing.T) {
	nm := NewNetworkManager("devNet", "")

	// 空名 → 回落默认网络 devNet
	c1, cfg1, err := nm.ClientFor("")
	if err != nil {
		t.Fatalf("空名不应报错: %v", err)
	}
	if c1 == nil || cfg1.Name != "devNet" {
		t.Fatalf("空名应回落 devNet, got %q", cfg1.Name)
	}

	// 合法名 → 对应网络
	_, cfg2, err := nm.ClientFor("localNet")
	if err != nil {
		t.Fatalf("localNet 不应报错: %v", err)
	}
	if cfg2.Name != "localNet" {
		t.Fatalf("应返回 localNet, got %q", cfg2.Name)
	}

	// 同网复用同一 client 实例（缓存）
	c3, _, _ := nm.ClientFor("")
	if c1 != c3 {
		t.Fatal("同网络两次解析应返回同一 client 实例")
	}

	// 未知名 → 报错
	if _, _, err := nm.ClientFor("mainNet"); err == nil || !strings.Contains(err.Error(), "mainNet") {
		t.Fatalf("未知名应报错且含名字, got %v", err)
	}

	// MILON_RPC_URL 覆盖 devNet 端点后，ClientFor 返回覆盖后的配置
	nm2 := NewNetworkManager("devNet", "http://127.0.0.1:19999")
	_, cfg3, _ := nm2.ClientFor("devNet")
	if cfg3.RpcUrl != "http://127.0.0.1:19999" {
		t.Fatalf("devNet RpcUrl 应被覆盖, got %q", cfg3.RpcUrl)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./client/ -run TestClientFor -count=1 -v`
Expected: 编译失败 `nm.ClientFor undefined`

- [ ] **Step 3: 实现 `ClientFor`**

在 `client/network_manager.go` 的 `Switch` 函数之后插入：

```go
// ClientFor 返回指定网络的 client 与配置（请求级网络解析的入口）。
// name 为空时回落到默认网络（currentNetwork，即启动配置或 network_switch
// 所设）；未知网络名返回错误。client 经 getOrCreateClient 缓存复用。
// 注意：不能持 nm.mu 调 getOrCreateClient（它自己拿写锁），分步加锁。
func (nm *NetworkManager) ClientFor(name string) (*milon.Client, milon.Network, error) {
	if name == "" {
		nm.mu.RLock()
		name = nm.currentNetwork
		nm.mu.RUnlock()
	} else {
		nm.mu.RLock()
		_, known := nm.networks[name]
		nm.mu.RUnlock()
		if !known {
			return nil, milon.Network{}, fmt.Errorf("unknown network: %s", name)
		}
	}
	client, err := nm.getOrCreateClient(name)
	if err != nil {
		return nil, milon.Network{}, err
	}
	nm.mu.RLock()
	cfg := nm.networks[name]
	nm.mu.RUnlock()
	return client, cfg, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./client/ -run TestClientFor -count=1 -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add client/network_manager.go client/network_manager_test.go
git commit -m "feat(client): NetworkManager 新增 ClientFor 请求级网络解析（空名回落默认/未知报错/缓存复用）"
```

---

### Task 2: middleware 层 `ResolveNetwork` / `ClientFrom`

**Files:**
- Create: `middleware/network.go`
- Test: `middleware/network_test.go`（新建）

**Interfaces:**
- Consumes: Task 1 的 `nm.ClientFor(name)`。
- Produces（Task 3 依赖）:
  - `const NetworkHeader = "X-Milon-Network"`、`const CtxClientKey`、`const CtxNetworkNameKey`
  - `func ResolveNetwork(nm *client.NetworkManager) gin.HandlerFunc`
  - `func ClientFrom(c *gin.Context) *milon.Client`（缺失时返回 nil）
  - `func NetworkNameFrom(c *gin.Context) string`（返回**解析后**的网络名，空头=默认网名）

- [ ] **Step 1: 写失败测试**

新建 `middleware/network_test.go`：

```go
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"

	"github.com/gin-gonic/gin"
)

// newNetworkRouter 组装 ResolveNetwork + 探针路由（回显解析结果）。
// fake RPC 后端经 MILON_RPC_URL 注入为 devNet 端点（NewClient 不拨号，安全）。
func newNetworkRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(fake.Close)
	t.Setenv("MILON_RPC_URL", fake.URL)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	nm := client.NewNetworkManager("devNet", fake.URL)
	r.Use(func(c *gin.Context) { c.Set("nm", nm); c.Next() })
	r.Use(ResolveNetwork(nm))
	r.GET("/probe", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"hasClient": ClientFrom(c) != nil,
			"network":   NetworkNameFrom(c),
		})
	})
	return r
}

func TestResolveNetwork(t *testing.T) {
	r := newNetworkRouter(t)

	cases := []struct {
		header string
		want   string
	}{
		{"", "devNet"},          // 不带 → 默认网络
		{"devNet", "devNet"},    // 显式 devNet
		{"localNet", "localNet"}, // 显式 localNet
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		if tc.header != "" {
			req.Header.Set(NetworkHeader, tc.header)
		}
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("header=%q http %d: %s", tc.header, w.Code, w.Body.String())
		}
		var out struct {
			HasClient bool   `json:"hasClient"`
			Network   string `json:"network"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if !out.HasClient || out.Network != tc.want {
			t.Errorf("header=%q got client=%v network=%q, want %q", tc.header, out.HasClient, out.Network, tc.want)
		}
	}

	// 未知名 → 400 ERR_INVALID_PARAMETER
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set(NetworkHeader, "mainNet")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未知网络应 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "mainNet") || !strings.Contains(w.Body.String(), "400") {
		t.Fatalf("错误响应应含未知名与错误码: %s", w.Body.String())
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./middleware/ -run TestResolveNetwork -count=1 -v`
Expected: 编译失败 `undefined: ResolveNetwork`

- [ ] **Step 3: 实现 `middleware/network.go`**

```go
package middleware

import (
	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
)

// 请求级网络选择（spec: 2026-10-09-request-scoped-network）：
// 客户端可携带 X-Milon-Network 头指定本次请求的网络；缺省走服务端默认
// 网络（启动配置或 network_switch 所设），与历史行为一致。

const (
	// NetworkHeader 是请求级网络选择头名。
	NetworkHeader = "X-Milon-Network"
	// CtxClientKey / CtxNetworkNameKey 是 context 注入键。
	CtxClientKey      = "milon_network_client"
	CtxNetworkNameKey = "milon_network_name"
)

// ResolveNetwork 读网络头并按名解析 client 注入 context；未知网络名 400。
// 挂在整个 /api 组；/api/network/* 三端点不读 client，统一挂载无副作用。
func ResolveNetwork(nm *client.NetworkManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		mc, cfg, err := nm.ClientFor(c.GetHeader(NetworkHeader))
		if err != nil {
			c.AbortWithStatusJSON(400, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
			return
		}
		c.Set(CtxClientKey, mc)
		c.Set(CtxNetworkNameKey, cfg.Name)
		c.Next()
	}
}

// ClientFrom 取当前请求解析出的网络 client；未经过 ResolveNetwork 时返回 nil。
func ClientFrom(c *gin.Context) *milon.Client {
	if v, ok := c.Get(CtxClientKey); ok {
		if mc, ok := v.(*milon.Client); ok {
			return mc
		}
	}
	return nil
}

// NetworkNameFrom 取解析后的网络名（空头时即默认网络名）。
func NetworkNameFrom(c *gin.Context) string {
	if v, ok := c.Get(CtxNetworkNameKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./middleware/ -count=1 -v`
Expected: PASS（含既有 nocache/redact 测试）

- [ ] **Step 5: Commit**

```bash
git add middleware/network.go middleware/network_test.go
git commit -m "feat(middleware): ResolveNetwork 中间件——X-Milon-Network 头解析注入 client，未知网络 400"
```

---

### Task 3: handler 层 35 处替换 + main.go 挂载 + 路由 E2E

**Files:**
- Modify: `handler/` 下 18 个文件（全部 `mc, _ := h.nm.GetCurrent()` 调用点）
- Modify: `main.go`（`api := r.Group("/api")` 处挂中间件）
- Modify: `handler/network.go`（仅改 `GetCurrentNetwork` 的 message 文案为「服务端默认网络」；`SwitchNetwork` 代码不动）
- Test: `handler/network_routing_test.go`（新建）

**Interfaces:**
- Consumes: Task 2 的 `middleware.ClientFrom(c)`。
- Produces: 全部链上 handler 从请求上下文取 client——后续任务不依赖新符号。
- ⚠️ import 方向 `handler → middleware → client` 无环；**不要**在 middleware 包的测试里 import handler（会成环）——路由 E2E 测试放 handler 包。

- [ ] **Step 1: 写失败的路由 E2E 测试**

新建 `handler/network_routing_test.go`：

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

// TestNetworkHeaderRouting 是方案 A 的核心证明：fake RPC 后端充当 devNet
// 端点，同一 REST 端点带不同 X-Milon-Network 头，请求必须落到对应网络。
// localNet(127.0.0.1:6280) 在测试环境无服务，用「fake 计数不增加」做反向证明。
func TestNetworkHeaderRouting(t *testing.T) {
	var devNetHits int64
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&devNetHits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"number":"1"}}`))
	}))
	t.Cleanup(fake.Close)

	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	api.GET("/chain-head", NewSystemHandler(nm).GetChainHead)

	// 不带头（默认=devNet）→ 落 fake
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/chain-head", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("默认网络请求失败: http %d: %s", w.Code, w.Body.String())
	}
	if atomic.LoadInt64(&devNetHits) == 0 {
		t.Fatal("默认网络(缺省头)应路由到 devNet fake 后端")
	}

	// 带 devNet 头 → 落 fake
	before := atomic.LoadInt64(&devNetHits)
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/chain-head", nil)
	req.Header.Set(middleware.NetworkHeader, "devNet")
	r.ServeHTTP(w, req)
	if atomic.LoadInt64(&devNetHits) <= before {
		t.Fatal("X-Milon-Network: devNet 应路由到 devNet fake 后端")
	}

	// 带 localNet 头 → 不落 fake（去了 localNet 端点），响应非 200（127.0.0.1:6280 无服务）
	before = atomic.LoadInt64(&devNetHits)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/chain-head", nil)
	req.Header.Set(middleware.NetworkHeader, "localNet")
	r.ServeHTTP(w, req)
	if atomic.LoadInt64(&devNetHits) != before {
		t.Fatal("X-Milon-Network: localNet 不应路由到 devNet fake 后端")
	}
	if w.Code == http.StatusOK {
		t.Fatalf("localNet 无服务应失败, got %d: %s", w.Code, w.Body.String())
	}

	// 未知网络 → 400
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/chain-head", nil)
	req.Header.Set(middleware.NetworkHeader, "nope")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未知网络应 400, got %d", w.Code)
	}
}
```

先跑一次：Run: `go test ./handler/ -run TestNetworkHeaderRouting -count=1 -v`
Expected: FAIL——`api.GET` 组还没挂中间件的场景在测试里已直接组装（本测试自组装 mini router，**不依赖 main.go**），但 handler 内部仍是 `nm.GetCurrent()`，localNet 头用例会失败（请求仍落 devNet fake → 计数增加）。这就是红。

- [ ] **Step 2: 批量替换 35 处调用点**

```bash
# 1) 替换调用（幂等，可重复跑）
grep -rl 'mc, _ := h.nm.GetCurrent()' handler/ | xargs sed -i 's/mc, _ := h.nm.GetCurrent()/mc := middleware.ClientFrom(c)/g'
# 2) 编译驱动补 import：逐个修到 build 通过
go build ./handler/ 2>&1 | head -30
```

对报「undefined: middleware」的每个文件，在 import 块加 `"milon-api-server/middleware"`（保持字母序）。

- [ ] **Step 3: main.go 挂中间件**

`api := r.Group("/api")` 之后紧跟一行：

```go
	// 请求级网络选择：X-Milon-Network 头 → 对应网络 client；缺省走服务端默认网络
	api.Use(middleware.ResolveNetwork(nm))
```

- [ ] **Step 4: handler/network.go 文案**

`GetCurrentNetwork` 的响应 message 保持 `"ok"`；把函数头注释改为：

```go
// GetCurrentNetwork handles GET /api/network/current —— 返回服务端默认网络
// （未携带 X-Milon-Network 头的请求所走的网络）。
```

`SwitchNetwork` 函数头注释改为「设置服务端默认网络（仅影响未携带网络头的请求）」，代码不动。

- [ ] **Step 5: 路由 E2E 转绿 + 全量回归**

Run: `go test ./handler/ -run TestNetworkHeaderRouting -count=1 -v`
Expected: PASS（localNet 用例现在真的去了 localNet）
Run: `go build ./... && go test ./... -count=1`
Expected: 全绿（既有测试不回归——不带头的请求行为不变）

- [ ] **Step 6: Commit**

```bash
git add handler/ main.go
git commit -m "feat(handler): 链上端点全部改走请求级网络——35 处 GetCurrent 替换为中间件注入，/api 组挂 ResolveNetwork"
```

---

### Task 4: MCP 层 `networkScoped` 包装 + 回环带网络

**Files:**
- Modify: `mcpserver/tools.go`（`registerTool` 泛型包装 + `network_switch` 描述）
- Modify: `mcpserver/executor.go`（`Call` 签名加 `network string`）
- Modify: `mcpserver/tools_test.go` / `executor_test.go` / `e2e_test.go`（既有 `exec.Call` 调用点补空串参数）
- Test: 追加到 `mcpserver/tools_test.go`

**Interfaces:**
- Consumes: 现有 `registerTool[In]`、`Executor.Call`。
- Produces:
  - `type networkScoped[In any] struct { Network string; In }`（仅 mcpserver 内部）
  - `func (e *Executor) Call(ctx context.Context, m RESTMapping, argsJSON json.RawMessage, network string)`——network 非空时回环请求带 `X-Milon-Network` 头
  - 57 个工具的 inputSchema 均含非必填 `network` 字段

- [ ] **Step 1: 写失败的 schema 测试**

追加到 `mcpserver/tools_test.go` 末尾：

```go
// TestNetworkParamInSchema 锁定方案 A 的 MCP 面：全部工具 schema 含非必填
// network 字段且描述正确——同时锁死 go-sdk jsonschema 对泛型嵌入字段的
// inline 展开（若 SDK 行为变化此测试立即红）。
func TestNetworkParamInSchema(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	out := rpcCall(t, srv.URL, "tools/list", map[string]any{})
	tools := out["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 57 {
		t.Fatalf("工具数=%d, want 57", len(tools))
	}
	for _, tl := range tools {
		tm := tl.(map[string]any)
		schema := tm["inputSchema"].(map[string]any)
		props := schema["properties"].(map[string]any)
		net, ok := props["network"].(map[string]any)
		if !ok {
			t.Errorf("%s schema 缺 network 字段", tm["name"])
			continue
		}
		if req, ok := schema["required"].([]any); ok {
			for _, r := range req {
				if r == "network" {
					t.Errorf("%s 的 network 不应是 required", tm["name"])
				}
			}
		}
		if d, _ := net["description"].(string); d == "" {
			t.Errorf("%s 的 network 描述为空", tm["name"])
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./mcpserver/ -run TestNetworkParamInSchema -count=1 -v`
Expected: FAIL（大量「缺 network 字段」）

- [ ] **Step 3: 实现——networkScoped 包装 + Call 加 network**

`mcpserver/tools.go` 中 `registerTool` 改为：

```go
// networkScoped 给任意工具参数叠加可选 network 字段（请求级网络选择，
// spec: 2026-10-09）。嵌入 In 使 jsonschema 把原字段 inline 展开。
type networkScoped[In any] struct {
	Network string `json:"network,omitempty" jsonschema:"可选：目标网络（devNet/localNet），缺省用服务端默认网络"`
	In
}

// registerTool 把"输入结构 → REST 映射"注册为 MCP 工具。
func registerTool[In any](srv *mcp.Server, exec *Executor, name, desc string, m RESTMapping) {
	if !inventorySeen[name] {
		inventorySeen[name] = true
		inventory = append(inventory, ToolInfo{
			Name: name, Description: desc, Group: toolGroupOf(name),
			Method: m.Method, Path: m.PathTemplate,
		})
	}
	mcp.AddTool[networkScoped[In], any](srv, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, req *mcp.CallToolRequest, args networkScoped[In]) (*mcp.CallToolResult, any, error) {
			raw, err := json.Marshal(args)
			if err != nil {
				return nil, nil, err
			}
			var all map[string]any
			if err := json.Unmarshal(raw, &all); err != nil {
				return nil, nil, err
			}
			network, _ := all["network"].(string)
			delete(all, "network")
			rest, err := json.Marshal(all)
			if err != nil {
				return nil, nil, err
			}
			out, err := exec.Call(ctx, m, rest, network)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{
				IsError: out.IsError,
				Content: []mcp.Content{&mcp.TextContent{Text: string(out.Body)}},
			}, nil, nil
		})
}
```

`mcpserver/executor.go` 的 `Call` 签名与请求构造改为：

```go
func (e *Executor) Call(ctx context.Context, m RESTMapping, argsJSON json.RawMessage, network string) (*CallOutcome, error) {
```

（函数体不变，在 `req.Header.Set("Content-Type", ...)` 同级、`e.http.Do(req)` 之前加：）

```go
	if network != "" {
		req.Header.Set("X-Milon-Network", network)
	}
```

然后修全部既有 `exec.Call(` / `.Call(ctx` 调用点（executor_test.go、e2e_test.go、tools_test.go 里若有直接调用），末参补 `""`。

`network_switch` 的注册行描述改为：
`"设置服务端默认网络（仅影响未携带 network 参数/头的请求；devNet/localNet）"`

- [ ] **Step 4: 跑测试确认通过 + 全量回归**

Run: `go test ./mcpserver/ -run 'TestNetworkParamInSchema|TestToolInventory|TestToolsListTotalAndAuth' -count=1 -v`
Expected: PASS（inventory 与 schema 同步变化无回归）
Run: `go test ./... -count=1`
Expected: 全绿

- [ ] **Step 5: 写回环带头的集成测试**

追加到 `mcpserver/tools_test.go`：

```go
// TestMcpNetworkHeaderPassthrough 证明：工具参数 network → 回环 REST 请求的
// X-Milon-Network 头，且 network 键已从 body 剥离。
func TestMcpNetworkHeaderPassthrough(t *testing.T) {
	var gotNetwork, gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotNetwork = r.Header.Get("X-Milon-Network")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
	}))
	t.Cleanup(backend.Close)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)

	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)

	out := rpcCall(t, front.URL, "tools/call", map[string]any{
		"name": "network_current",
		"arguments": map[string]any{"network": "localNet"},
	})
	if out["error"] != nil {
		t.Fatalf("tools/call 失败: %v", out["error"])
	}
	if gotNetwork != "localNet" {
		t.Fatalf("回环应带 X-Milon-Network: localNet, got %q (path=%s)", gotNetwork, gotPath)
	}
	if gotPath != "/api/network/current" {
		t.Fatalf("回环路径不对: %s", gotPath)
	}
}
```

Run: `go test ./mcpserver/ -run TestMcpNetworkHeaderPassthrough -count=1 -v`
Expected: PASS（若红，检查 body 剥离与头设置逻辑）

- [ ] **Step 6: Commit**

```bash
git add mcpserver/
git commit -m "feat(mcp): 57 工具全部获得可选 network 参数——networkScoped 泛型包装入 schema，executor 回环转 X-Milon-Network 头"
```

---

### Task 5: 前端本地偏好 + 统一带头

**Files:**
- Modify: `static/js/app.js`
- Modify: `static/js/saved-instructions.js`
- Modify: `static/index.html`（版本号 bump）

**Interfaces:**
- Consumes: REST 端点的 `X-Milon-Network` 头语义（Task 3）。
- Produces: `function apiFetch(url, opt)`（app.js 全局，saved-instructions.js 同页面可直接用）；localStorage 键 `milonNetwork`。

- [ ] **Step 1: app.js 加 `apiFetch` 并替换链上 fetch 点**

在 app.js 顶部（`const ENDPOINTS` 之前）加：

```js
// apiFetch：链上 API 的统一 fetch 封装——携带本地网络偏好（顶栏选择器
// 所选，存 localStorage['milonNetwork']）为 X-Milon-Network 头；无偏好不带头，
// 走服务端默认网络。网络管理/健康检查等服务元数据端点不走此封装。
function apiFetch(url, opt) {
  opt = opt || {};
  var net = localStorage.getItem('milonNetwork');
  if (net) {
    opt.headers = Object.assign({}, opt.headers, { 'X-Milon-Network': net });
  }
  return fetch(url, opt);
}
```

替换规则：`grep -n "fetch(" static/js/app.js static/js/saved-instructions.js` 清点全部 18 处，其中**链上数据端点**换 `apiFetch`（Playground 发送与轮询、`/api/accounts/*`、`/api/util/key/derive-public`、`/api/idl/metadata`、saved-instructions 的全部 `/api/saved-instructions*`）；**不换**：`/api/network/list|current|switch`、`/api/health`、`/api/mcp/tools`（服务元数据，与本会话网络无关）。注意各处 `fetch(` 调用的参数形态保持不变只换函数名，`await`/`.then` 均不动。

- [ ] **Step 2: 顶栏选择器改本地偏好**

`loadNetworks`（app.js 约 1420 行）回显逻辑改为：`localStorage['milonNetwork']` 有值且在列表中 → 选中它；否则沿用现有「服务器默认」回显。

`switchNetwork(name)` 整体替换为：

```js
async function switchNetwork(name) {
  if (!name) return;
  localStorage.setItem('milonNetwork', name);
  showToast('本地网络偏好已设为 ' + name + '（后续请求携带 X-Milon-Network 头）', 'success');
}
```

（不再调用 `/api/network/switch`；该端点在 Playground 里仍可用。）

- [ ] **Step 3: index.html 版本号 bump**

三处 `?v=20261009` → `?v=20261010`。

- [ ] **Step 4: 语法检查 + 起服务浏览器实测**

Run: `node --check static/js/app.js && node --check static/js/saved-instructions.js`
Expected: 无输出（通过）

起服务 `SERVER_PORT=18080 ./milon-api-server.exe`，浏览器实测（步骤见 Task 6 的实测清单第 1-3 条，可并入 Task 6 一起做）。

- [ ] **Step 5: Commit**

```bash
git add static/js/app.js static/js/saved-instructions.js static/index.html
git commit -m "feat(web): 顶栏网络选择器改本地偏好（localStorage）+ apiFetch 统一携带 X-Milon-Network，静态资源 bump v=20261010"
```

---

### Task 6: 文档同步 + 浏览器实测收尾

**Files:**
- Modify: `API.md`（「通用说明」加「网络选择」小节；`network/switch` 条目说明）
- Modify: `README.md`（MCP 章节一句 + 快速开始一句）
- Modify: `static/index.html`（MCP 页工具清单区描述提 network 参数——若 Task 5 已顺手完成则跳过）

**Interfaces:** 无代码接口；产出为文档与实测证据。

- [ ] **Step 1: API.md「网络选择」小节**

在「## 通用说明」的「### 统一响应格式」之前插入：

```markdown
### 网络选择（请求级）

除服务端默认网络外，任何链上端点都可通过请求头指定本次请求的网络：

```bash
curl -H 'X-Milon-Network: devNet' http://localhost:8080/api/chain-head
```

- 头缺失：走**服务端默认网络**（启动配置，或 `POST /api/network/switch` 最近所设）——与历史行为一致。
- 头为已知网络名（`localNet` / `devNet`，未来 `mainNet`）：该次请求路由到对应网络，client 按网缓存复用。
- 头为未知网络名：返回 400（`ERR_INVALID_PARAMETER`），message 指明未知名。
- 多用户共享同一 server 时各带各的头，互不影响；`network_switch` 只改默认网络，不再影响显式指定网络的请求。
- MCP 侧等价能力：全部 57 个工具均有可选 `network` 参数（缺省同上）。
```

`### 一、网络管理` 的 switch 条目「说明」追加：「方案 A 后语义为设置服务端默认网络——仅影响未携带 `X-Milon-Network` 头的请求」。

- [ ] **Step 2: README 两句**

MCP 章节首段末尾追加：「全部工具均支持可选 `network` 参数（`devNet`/`localNet`），供多网络并存时按请求指定；REST 侧等价头为 `X-Milon-Network`。」快速开始或环境变量表附近补一句默认网络行为。

- [ ] **Step 3: 全量回归**

Run: `go build ./... && go test ./... -count=1`
Expected: 全绿

- [ ] **Step 4: 浏览器实测清单**

起服务（`SERVER_PORT=18080`），逐项验证并记录：
1. 顶栏切到 localNet → toast 提示本地偏好；刷新页面偏好保持。
2. Playground 发 `/api/chain-head`（localNet 偏好下）→ 响应为连接失败（127.0.0.1:6280 无服务）即证明请求带了头走 localNet；切回 devNet 再发 → 200。
3. 请求历史/服务端日志确认请求正常。
4. MCP 页工具清单正常（57 个），ENDPOINTS 计数不变。

- [ ] **Step 5: Commit**

```bash
git add API.md README.md
git commit -m "docs: 请求级网络选择文档——X-Milon-Network 头用法、network_switch 新语义、MCP network 参数"
```

---

## Self-Review 记录

1. **Spec 覆盖**：spec 六节组件设计 ↔ Task 1（client）/Task 2（middleware）/Task 3（handler+main）/Task 4（MCP）/Task 5（前端）/Task 6（文档）；spec 测试策略六条 ↔ Task 1 Step1 / Task 2 Step1 / Task 3 Step1（E2E 核心）/ Task 4 Step1+Step5 / Task 6 Step4。✅
2. **占位符**：无 TBD/「适当处理」类表述；所有代码步骤给了完整代码。✅
3. **类型一致性**：`ClientFor(name) (*milon.Client, milon.Network, error)` 在 Task 1 定义、Task 2 消费一致；`ResolveNetwork`/`ClientFrom`/`NetworkNameFrom`/`NetworkHeader` 在 Task 2 定义、Task 3 消费一致；`Call(..., network string)` 在 Task 4 内部自洽。✅
4. **已知风险内嵌**：jsonschema 泛型嵌入展开由 Task 4 Step1 测试锁死；middleware↔handler import 环在 Task 3 接口注记里显式警告。
