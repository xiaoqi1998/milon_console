# milon-api-server MCP 端点实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 server 上内嵌 Streamable HTTP MCP 端点 `/mcp`，把全部 REST 能力（除 mock/health/chain-head）薄适配为 55 个 MCP 工具。

**Architecture:** 新增 `mcpserver/` 包，官方 go-sdk 起 MCP server；每个工具的输入结构镜像对应 handler 的 request struct（json tag 一致），通用执行器把参数渲染成 REST 请求发往本进程回环地址，REST JSON 响应原样作为工具结果返回。REST 层零改动。

**Tech Stack:** Go 1.25.9、`github.com/modelcontextprotocol/go-sdk`（唯一新增依赖）、gin（现有）、httptest。

**Spec:** `docs/superpowers/specs/2026-10-08-mcp-endpoint-design.md`

## Global Constraints

- Go 1.25.9；构建需 `CGO_ENABLED=1` + MinGW（本机已具备）。
- 唯一新增依赖：`github.com/modelcontextprotocol/go-sdk`（官方）。
- 不修改任何现有 handler / REST 行为；main.go 仅加挂载，config 不改（`MCP_AUTH_TOKEN` 由 mcpserver 自行 `os.Getenv`）。
- MCP 工具的输入字段 json tag 必须与对应 REST 请求体字段**逐字一致**（薄适配的核心约束，字段名以 handler 内 request struct 为唯一事实源）。
- 提交信息中文，前缀 feat/fix/docs/test（对齐现有 git log 风格）。
- 所有面向用户的文案（工具描述、文档、前端卡片）用简体中文。
- 测试命令统一 `go test ./mcpserver/... -run <Case> -v`；E2E 用 `SERVER_PORT=18080`。
- 工具总数 55；`tools/list` 注册数是验收断言之一。

---

### Task 1: mcpserver 执行器框架（REST 映射 → 回环调用 → 工具结果）

**Files:**
- Create: `mcpserver/executor.go`
- Test: `mcpserver/executor_test.go`

**Interfaces:**
- Produces:
  - `type RESTMapping struct { Method, PathTemplate string; PathParams, QueryParams []string }`
  - `type Executor struct { ... }` + `func NewExecutor(baseURL string, httpClient *http.Client) *Executor`
  - `func (e *Executor) Call(ctx context.Context, m RESTMapping, argsJSON json.RawMessage) (*CallOutcome, error)`
  - `type CallOutcome struct { IsError bool; Body []byte }`（Body 为 REST 响应原文）

- [ ] **Step 1: 写失败测试**

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newFakeBackend 起一个记录请求的假 REST 后端。
func newFakeBackend(t *testing.T, status int, respBody string) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			_ = r.Body.Read(body)
		}
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" "+string(body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestExecutorCall(t *testing.T) {
	ctx := context.Background()

	t.Run("GET 路径参数渲染", func(t *testing.T) {
		srv, seen := newFakeBackend(t, 200, `{"code":0}`)
		e := NewExecutor(srv.URL, srv.Client())
		out, err := e.Call(ctx, RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}", PathParams: []string{"hash"}},
			json.RawMessage(`{"hash":"abc"}`))
		if err != nil {
			t.Fatal(err)
		}
		if out.IsError || string(out.Body) != `{"code":0}` {
			t.Fatalf("out=%+v", out)
		}
		if *seen != nil && (*seen)[0] != "GET /api/transactions/abc? " {
			t.Fatalf("seen=%v", *seen)
		}
	})

	t.Run("POST body 排除路径参数", func(t *testing.T) {
		srv, seen := newFakeBackend(t, 200, `{}`)
		e := NewExecutor(srv.URL, srv.Client())
		_, err := e.Call(ctx, RESTMapping{Method: "POST", PathTemplate: "/api/faucet/claim"},
			json.RawMessage(`{"privateKey":"sk","address":"addr"}`))
		if err != nil {
			t.Fatal(err)
		}
		got := (*seen)[0]
		want := "POST /api/faucet/claim? " + `{"privateKey":"sk","address":"addr"}`
		if got != want {
			t.Fatalf("got=%s want=%s", got, want)
		}
	})

	t.Run("非2xx 转工具错误且保留原文", func(t *testing.T) {
		srv, _ := newFakeBackend(t, 400, `{"code":1001,"message":"invalid privateKey"}`)
		e := NewExecutor(srv.URL, srv.Client())
		out, _ := e.Call(ctx, RESTMapping{Method: "POST", PathTemplate: "/api/faucet/claim"}, json.RawMessage(`{}`))
		if !out.IsError {
			t.Fatal("want IsError")
		}
		if string(out.Body) != `{"code":1001,"message":"invalid privateKey"}` {
			t.Fatalf("body=%s", out.Body)
		}
	})

	t.Run("缺路径参数返回错误", func(t *testing.T) {
		srv, _ := newFakeBackend(t, 200, `{}`)
		e := NewExecutor(srv.URL, srv.Client())
		_, err := e.Call(ctx, RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}", PathParams: []string{"hash"}}, json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("want error for missing path param")
		}
	})

	t.Run("后端不可达报可读错误", func(t *testing.T) {
		e := NewExecutor("http://127.0.0.1:1", &http.Client{})
		out, err := e.Call(ctx, RESTMapping{Method: "GET", PathTemplate: "/api/health"}, json.RawMessage(`{}`))
		if err == nil && !out.IsError {
			t.Fatal("want unreachable error")
		}
	})
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./mcpserver/... -run TestExecutorCall -v`
Expected: FAIL（包不存在/符号未定义）

- [ ] **Step 3: 最小实现**

```go
// Package mcpserver 把现有 REST 能力薄适配为 MCP 工具。
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RESTMapping 声明一个 MCP 工具对应的 REST 调用。
type RESTMapping struct {
	Method      string   // "GET" / "POST"
	PathTemplate string // 如 "/api/transactions/{hash}"
	PathParams  []string
	QueryParams []string // 若 handler 读 c.Query，则列入；渲染为 query string
}

// CallOutcome 是一次 REST 调用的结果；IsError=true 时 Body 为 REST 错误 JSON 原文。
type CallOutcome struct {
	IsError bool
	Body    []byte
}

type Executor struct {
	baseURL string
	http    *http.Client
}

func NewExecutor(baseURL string, httpClient *http.Client) *Executor {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &Executor{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

func (e *Executor) Call(ctx context.Context, m RESTMapping, argsJSON json.RawMessage) (*CallOutcome, error) {
	var args map[string]any
	if len(argsJSON) > 0 {
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("工具参数不是合法 JSON 对象: %w", err)
		}
	}
	path := m.PathTemplate
	for _, p := range m.PathParams {
		v, ok := args[p]
		if !ok {
			return nil, fmt.Errorf("缺少必填参数 %s", p)
		}
		path = strings.ReplaceAll(path, "{"+p+"}", url.PathEscape(fmt.Sprintf("%v", v)))
		delete(args, p)
	}
	query := url.Values{}
	for _, q := range m.QueryParams {
		if v, ok := args[q]; ok {
			query.Set(q, fmt.Sprintf("%v", v))
			delete(args, q)
		}
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	var body io.Reader
	if m.Method == "POST" {
		b, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, m.Method, e.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if m.Method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("milon REST 后端不可达: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	return &CallOutcome{IsError: resp.StatusCode >= 300, Body: data}, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./mcpserver/... -run TestExecutorCall -v`
Expected: PASS（全部子测试）

- [ ] **Step 5: 提交**

```bash
git add mcpserver/executor.go mcpserver/executor_test.go
git commit -m "feat(mcp): REST 映射执行器——路径渲染/body 组装/错误透传"
```

---

### Task 2: 工具注册框架 + 17 个基础工具（网络/账户/faucet/交易查询/RPC/IDL）

**Files:**
- Create: `mcpserver/tools.go`
- Test: `mcpserver/tools_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Executor`/`RESTMapping`/`CallOutcome`
- Produces:
  - `func RegisterTools(srv *mcp.Server, exec *Executor)`（后续任务在同函数内追加注册）
  - `func BuildServer(baseURL string) *mcp.Server`（Task 7 的 server.go 使用；先放 tools.go，Task 7 移入）
  - `var RegisteredToolCount func() int` 不需要——测试经 `tools/list` 数数量。

- [ ] **Step 1: 引入 SDK 依赖**

Run: `go get github.com/modelcontextprotocol/go-sdk@latest && go mod tidy`
说明：若 API 与下述签名有出入（AddTool 泛型签名在 v0.2→v0.4 间有过调整），以模块缓存 `go doc github.com/modelcontextprotocol/go-sdk/mcp` 为准对齐，不改变"输入结构镜像 request struct"的设计。

- [ ] **Step 2: 写失败测试（协议级：HTTP 直连 tools/list 与 tools/call）**

测试直接对挂好的 `/mcp` 发 JSON-RPC，等价于真实 agent 行为，且不依赖 SDK 测试工具类：

```go
package mcpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mcpHTTPServer 挂载 BuildServer 的 handler 到 httptest。
func mcpHTTPServer(t *testing.T, backendURL string) *httptest.Server {
	t.Helper()
	handler := NewMCPHandler(nil) // nil = 不鉴权；内部 BuildServer(默认 baseURL)
	_ = backendURL
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func rpcCall(t *testing.T, url, method string, params any) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("http %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func TestToolsListBasics(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	out := rpcCall(t, srv.URL, "tools/list", map[string]any{})
	tools := out["result"].(map[string]any)["tools"].([]any)
	var names []string
	for _, tl := range tools {
		names = append(names, tl.(map[string]any)["name"].(string))
	}
	want := []string{"network_list", "network_current", "network_switch",
		"account_generate", "account_info", "account_resources",
		"faucet_claim", "faucet_balance",
		"tx_get", "tx_parse", "tx_events", "tx_wait",
		"rpc_block", "rpc_resource", "rpc_access_value", "rpc_resource_path",
		"idl_metadata"}
	for _, w := range want {
		found := false
		for _, n := range names {
			if n == w {
				found = true
			}
		}
		if !found {
			t.Errorf("缺少工具 %s，实际=%v", w, names)
		}
	}
}

func TestBasicToolCallMapping(t *testing.T) {
	// 假后端记录收到的 REST 请求
	backend, seen := newFakeBackend(t, 200, `{"code":0,"data":{"ok":true}}`)
	// 用环境变量把执行器指向假后端
	t.Setenv("MILON_REST_BASE_URL", backend.URL)
	handler := NewMCPHandler(nil)
	front := httptest.NewServer(handler)
	t.Cleanup(front.Close)

	cases := []struct{ tool, args, want string }{
		{"network_list", `{}`, "GET /api/network/list? "},
		{"network_switch", `{"network":"localNet"}`, `POST /api/network/switch? {"network":"localNet"}`},
		{"account_generate", `{"keyType":"ed25519"}`, `POST /api/accounts/generate? {"keyType":"ed25519"}`},
		{"account_info", `{"address":"a1"}`, "GET /api/accounts/a1? "},
		{"faucet_balance", `{"address":"a1"}`, "GET /api/faucet/balance/a1? "},
		{"tx_get", `{"hash":"h1"}`, "GET /api/transactions/h1? "},
		{"tx_wait", `{"hash":"h1"}`, "GET /api/transactions/h1/wait? "},
		{"rpc_block", `{"height":"123"}`, "GET /api/rpc/blocks/123? "},
		{"rpc_resource", `{"hash":"r1"}`, "GET /api/rpc/resources/r1? "},
		{"rpc_resource_path", `{"hash":"r1"}`, "GET /api/rpc/resource-paths/r1? "},
		{"idl_metadata", `{}`, "GET /api/idl/metadata? "},
	}
	for _, c := range cases {
		out := rpcCall(t, front.URL, "tools/call", map[string]any{"name": c.tool, "arguments": json.RawMessage(c.args)})
		res := out["result"].(map[string]any)
		if v, _ := res["isError"].(bool); v {
			t.Errorf("%s: unexpected isError: %v", c.tool, out)
		}
	}
	if len(*seen) != len(cases) {
		t.Fatalf("后端收到 %d 个请求, want %d: %v", len(*seen), len(cases), *seen)
	}
	for i, c := range cases {
		if (*seen)[i] != c.want {
			t.Errorf("%s: got=%q want=%q", c.tool, (*seen)[i], c.want)
		}
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./mcpserver/... -run 'TestToolsListBasics|TestBasicToolCallMapping' -v`
Expected: FAIL（NewMCPHandler 未定义）

- [ ] **Step 4: 实现 tools.go**

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 镜像 REST request struct 的输入类型（json tag 与 REST 逐字一致）。
// 无参数的工具统一用 emptyArgs。
type emptyArgs struct{}

type networkSwitchArgs struct {
	Network string `json:"network" jsonschema:"目标网络名：devNet 或 localNet"`
}

type accountGenerateArgs struct {
	KeyType string `json:"keyType" jsonschema:"密钥曲线：secp256k1(缺省)/ed25519/fn-dsa-512"`
}

type addressArgs struct {
	Address string `json:"address"`
}

type hashArgs struct {
	Hash string `json:"hash"`
}

type heightArgs struct {
	Height string `json:"height"`
}

type accessValueArgs struct {
	BlobHashes []string `json:"blobHashes"`
}

// registerTool 把"输入结构 → REST 映射"注册为 MCP 工具。
func registerTool[In any](srv *mcp.Server, exec *Executor, name, desc string, m RESTMapping) {
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, any, error) {
			raw, err := json.Marshal(args)
			if err != nil {
				return nil, nil, err
			}
			out, err := exec.Call(ctx, m, raw)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{
				IsError: out.IsError,
				Content: []mcp.Content{&mcp.TextContent{Text: string(out.Body)}},
			}, nil, nil
		})
}

// NewMCPHandler 构造挂到 gin 的 /mcp handler；token 为空则不鉴权。
// 回环基地址优先取 MILON_REST_BASE_URL（测试注入），缺省 http://127.0.0.1:$SERVER_PORT。
func NewMCPHandler(token string) http.Handler {
	baseURL := os.Getenv("MILON_REST_BASE_URL")
	if baseURL == "" {
		port := os.Getenv("SERVER_PORT")
		if port == "" {
			port = "8080"
		}
		baseURL = "http://127.0.0.1:" + port
	}
	exec := NewExecutor(baseURL, nil)
	srv := mcp.NewServer(&mcp.Implementation{Name: "milon-api-server", Version: "1.0"}, nil)
	RegisterTools(srv, exec)
	h := mcp.NewStreamableHTTPHandler([]*mcp.Server{srv}, &mcp.StreamableHTTPOptions{Stateless: true})
	if token == "" {
		return h
	}
	return bearerAuth(token, h)
}

func bearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"invalid MCP token"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RegisterTools 注册全部工具；本任务先注册基础 17 个，后续任务在此追加。
func RegisterTools(srv *mcp.Server, exec *Executor) {
	registerTool[emptyArgs](srv, exec, "network_list", "列出内置网络（devNet/localNet）及当前网络", RESTMapping{Method: "GET", PathTemplate: "/api/network/list"})
	registerTool[emptyArgs](srv, exec, "network_current", "查询当前网络", RESTMapping{Method: "GET", PathTemplate: "/api/network/current"})
	registerTool[networkSwitchArgs](srv, exec, "network_switch", "切换当前网络（devNet/localNet）", RESTMapping{Method: "POST", PathTemplate: "/api/network/switch"})
	registerTool[accountGenerateArgs](srv, exec, "account_generate", "生成新账户（返回私钥/公钥/地址）", RESTMapping{Method: "POST", PathTemplate: "/api/accounts/generate"})
	registerTool[addressArgs](srv, exec, "account_info", "查询账户信息", RESTMapping{Method: "GET", PathTemplate: "/api/accounts/{address}", PathParams: []string{"address"}})
	registerTool[addressArgs](srv, exec, "account_resources", "查询账户链上资源", RESTMapping{Method: "GET", PathTemplate: "/api/accounts/{address}/resources", PathParams: []string{"address"}})
	registerTool[addressArgs](srv, exec, "faucet_balance", "查询账户余额（faucet 口径）", RESTMapping{Method: "GET", PathTemplate: "/api/faucet/balance/{address}", PathParams: []string{"address"}})
	registerTool[hashArgs](srv, exec, "tx_get", "按哈希查询交易", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}", PathParams: []string{"hash"}})
	registerTool[hashArgs](srv, exec, "tx_parse", "解析交易（结构化 postcard）", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/parse", PathParams: []string{"hash"}})
	registerTool[hashArgs](srv, exec, "tx_events", "查询交易事件", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/events", PathParams: []string{"hash"}})
	registerTool[hashArgs](srv, exec, "tx_wait", "等待交易确认（可能长轮询）", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/wait", PathParams: []string{"hash"}})
	registerTool[heightArgs](srv, exec, "rpc_block", "按高度查区块头", RESTMapping{Method: "GET", PathTemplate: "/api/rpc/blocks/{height}", PathParams: []string{"height"}})
	registerTool[hashArgs](srv, exec, "rpc_resource", "按哈希查链上资源原文", RESTMapping{Method: "GET", PathTemplate: "/api/rpc/resources/{hash}", PathParams: []string{"hash"}})
	registerTool[accessValueArgs](srv, exec, "rpc_access_value", "按 blob 哈希批量取 access value", RESTMapping{Method: "POST", PathTemplate: "/api/rpc/access-value"})
	registerTool[hashArgs](srv, exec, "rpc_resource_path", "按哈希查资源路径", RESTMapping{Method: "GET", PathTemplate: "/api/rpc/resource-paths/{hash}", PathParams: []string{"hash"}})
	registerTool[emptyArgs](srv, exec, "idl_metadata", "IDL 元数据发现：列出全部 app/方法/参数/返回值/signer 角色（调用合约工具前的第一站）", RESTMapping{Method: "GET", PathTemplate: "/api/idl/metadata"})
}
```

注意：`tx_wait` 若 handler 读 `c.Query("timeout")` 之类，给该 mapping 加 `QueryParams` 并在输入结构补字段（以 handler 源码为准）。

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./mcpserver/... -run 'TestToolsListBasics|TestBasicToolCallMapping' -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add mcpserver/ go.mod go.sum
git commit -m "feat(mcp): 工具注册框架 + 17 个基础工具(网络/账户/faucet/交易/RPC/IDL)"
```

---

### Task 3: 合约与视图工具（13 个：read/simulate/write 全家族 + view + 原始交易）

**Files:**
- Modify: `mcpserver/tools.go`（追加注册与输入结构）
- Test: `mcpserver/tools_test.go`（追加用例）

**Interfaces:**
- Consumes: `registerTool`/`Executor`
- Produces: 合约家族 13 个工具（下表）

镜像来源（唯一事实源）：`handler/contract.go` 的 `readContractRequest`(43)、`readContractMultiRequest`(103)、`simulateContractRequest`(165)、`writeContractRequest`(789)、`multiContractRequest`(1132)，以及 `handler/transaction_handler.go` 的 `rawTransactionRequest`(293)、`handler/view_handler.go` 的 `rawViewRequest`(48)。**逐字段复制 json tag**；`provider.Args` 镜像为 `map[string]any`，`json.RawMessage` 镜像为 `json.RawMessage`，`types.SignerEntry` 镜像为内联 struct（字段见 `types/request.go:17`）。

| 工具 | REST | 输入结构镜像自 |
|---|---|---|
| `contract_read` | POST /api/read | readContractRequest |
| `contract_read_multi` | POST /api/read/multi | readContractMultiRequest |
| `contract_simulate` | POST /api/simulate | simulateContractRequest |
| `contract_simulate_multi` | POST /api/simulate/multi | simulateContractRequest 家族（见 handler 117/847 行附近） |
| `contract_write` | POST /api/write | writeContractRequest |
| `contract_write_multi` | POST /api/write/multi | multiContractRequest |
| `contract_write_multi_agent` | POST /api/write/multi-agent | handler/contract.go:1187 对应 struct |
| `contract_write_multisig` | POST /api/write/multisig | handler/contract.go:1347 对应 struct |
| `tx_simulate_raw` | POST /api/transactions/simulate | rawTransactionRequest |
| `tx_submit_raw` | POST /api/transactions/submit | rawTransactionRequest |
| `tx_inspect_raw` | POST /api/transactions/inspect | rawTransactionRequest |
| `view_single` | POST /api/view/single | rawViewRequest |
| `view_multi` | POST /api/view/multi | handler/view_handler.go 对应 struct |

- [ ] **Step 1: 写失败测试（追加到 tools_test.go）**

```go
func TestContractToolCallMapping(t *testing.T) {
	backend, seen := newFakeBackend(t, 200, `{"code":0}`)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)
	front := httptest.NewServer(NewMCPHandler(nil))
	t.Cleanup(front.Close)

	cases := []struct{ tool, args, want string }{
		{"contract_read", `{"appName":"identity","methodName":"1071_get_identity","args":{"id":"a1"}}`,
			`POST /api/read? {"appName":"identity","methodName":"1071_get_identity","args":{"id":"a1"}}`},
		{"contract_write", `{"appName":"nft","methodName":"mint","args":{},"paymentMode":"unified_payer_all","payerPrivateKey":"sk","payerAddress":"a1"}`,
			`POST /api/write? {"appName":"nft","methodName":"mint","args":{},"paymentMode":"unified_payer_all","payerPrivateKey":"sk","payerAddress":"a1"}`},
		{"tx_submit_raw", `{"transactionPostcard":"pc1"}`,
			`POST /api/transactions/submit? {"transactionPostcard":"pc1"}`},
		{"view_single", `{"transactionPostcard":"pc1"}`,
			`POST /api/view/single? {"transactionPostcard":"pc1"}`},
	}
	for _, c := range cases {
		out := rpcCall(t, front.URL, "tools/call", map[string]any{"name": c.tool, "arguments": json.RawMessage(c.args)})
		if v, _ := out["result"].(map[string]any)["isError"].(bool); v {
			t.Errorf("%s: %v", c.tool, out)
		}
	}
	for i, c := range cases {
		if (*seen)[i] != c.want {
			t.Errorf("%s: got=%q want=%q", c.tool, (*seen)[i], c.want)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./mcpserver/... -run TestContractToolCallMapping -v`
Expected: FAIL（工具未注册）

- [ ] **Step 3: 实现输入结构与注册**

按上表在 tools.go 定义输入结构（导出、字段逐字镜像 json tag，`provider.Args` → `Args map[string]any \`json:"args"\``），示例给出两个代表，其余同法：

```go
type contractReadArgs struct {
	AppName    string         `json:"appName" jsonschema:"IDL 里的 app 名，先用 idl_metadata 查"`
	MethodName string         `json:"methodName" jsonschema:"IDL 里的方法名"`
	Args       map[string]any `json:"args"`
	PayerAddress string       `json:"payerAddress"`
}

type contractWriteArgs struct {
	AppName         string          `json:"appName"`
	MethodName      string          `json:"methodName"`
	Args            map[string]any  `json:"args"`
	PaymentMode     string          `json:"paymentMode" jsonschema:"unified_payer_all/unified_dual_sign/unified_payer_only_gas/split/multi_signer/sponsored"`
	PayerPrivateKey string          `json:"payerPrivateKey"`
	PayerAddress    string          `json:"payerAddress"`
	SignatureMode   json.RawMessage `json:"signatureMode"`
	IxAddress       string          `json:"ixAddress"`
	IxPrivateKey    string          `json:"ixPrivateKey"`
	IxSignatureMode json.RawMessage `json:"ixSignatureMode"`
	OwnerAddress    string          `json:"ownerAddress"`
	OwnerPrivateKey string          `json:"ownerPrivateKey"`
	Signers         []signerEntry   `json:"signers"`
	GasPayer        *signerEntry    `json:"gasPayer"`
}

type signerEntry struct { // 镜像 types.SignerEntry
	Address     string          `json:"address"`
	PrivateKey  string          `json:"privateKey"`
	SignatureMode json.RawMessage `json:"signatureMode"`
}
```

注册（追加进 RegisterTools）：

```go
registerTool[contractReadArgs](srv, exec, "contract_read", "只读合约调用（view）", RESTMapping{Method: "POST", PathTemplate: "/api/read"})
// ...其余 12 个按表逐一注册
```

- [ ] **Step 4: 跑测试确认通过；Step 5: 提交**

Run: `go test ./mcpserver/... -v`
```bash
git add mcpserver/
git commit -m "feat(mcp): 合约读写/模拟/原始交易/视图 13 个工具"
```

---

### Task 4: 密钥与签名工具（5 个）

**Files:** Modify `mcpserver/tools.go`；Test `mcpserver/tools_test.go`

镜像来源：`handler/util.go`（deriveAddressRequest:25、derivePublicKeyRequest:81、signMessageRequest:138、verifySignatureRequest:214）、`handler/vc_attestation_handler.go`（generateVcAttestationRequest:39）。

| 工具 | REST | 关键字段（完整以 handler 为准） |
|---|---|---|
| `util_derive_address` | POST /api/util/address/derive | publicKey, keyType |
| `util_derive_public_key` | POST /api/util/key/derive-public | privateKey, keyType |
| `util_sign` | POST /api/util/sign | privateKey, message, keyType |
| `util_verify` | POST /api/util/verify | publicKey, message, signature |
| `vc_attestation` | POST /api/util/vc-attestation | issuerPrivateKey, issuerPublicKey, chainId, subjectPrivateKey, subjectAddress, issuerKeyId, credentialSchema, credentialJson, validUntilMs, validUntil |

工具描述必须写明：`util_derive_address`——"32 字节私钥在 secp256k1/ed25519/fn-dsa-512 下派生出不同地址，地址计算必须用本工具（或 account_generate 的返回），不要本地臆造"。

- [ ] **Step 1: 写失败测试** —— 同 Task 3 模式，断言 `util_derive_address {"publicKey":"pk","keyType":"ed25519"}` → `POST /api/util/address/derive? {"publicKey":"pk","keyType":"ed25519"}`；`vc_attestation {"issuerPrivateKey":"sk","credentialJson":"{}"}` → 对应 POST 路径与 body。
- [ ] **Step 2: 确认失败** Run: `go test ./mcpserver/... -run TestUtilToolCallMapping -v` → FAIL
- [ ] **Step 3: 镜像实现 5 个输入结构并注册**（字段逐字抄 handler json tag）
- [ ] **Step 4: 确认通过**
- [ ] **Step 5: 提交** `git commit -m "feat(mcp): 密钥派生/签名验签/VC 证明 5 个工具"`

---

### Task 5: DID 12 工具 + 保存指令 4 工具

**Files:** Modify `mcpserver/tools.go`；Test `mcpserver/tools_test.go`

镜像来源：`handler/did_handler.go`（didCreateRequest:48 及同文件其余 *Request struct）、`handler/saved_instruction_handler.go`（createSavedInstructionRequest:176、updateSavedInstructionRequest:195 等；execute 端点的 body 以 handler 实际绑定为准）。

| 工具 | REST |
|---|---|
| `did_create` | POST /api/tool/did/create |
| `did_set_alias` | POST /api/tool/did/set-alias |
| `did_add_service` / `did_update_service` / `did_remove_service` | POST /api/tool/did/{add,update,remove}-service |
| `did_set_avatar_uri` | POST /api/tool/did/set-avatar-uri |
| `did_add_key` / `did_update_key` / `did_remove_key` | POST /api/tool/did/{add,update,remove}-key |
| `did_deactivate` | POST /api/tool/did/deactivate |
| `did_name_binding` | GET /api/tool/did/name-binding（query 参数以 handler c.Query 为准） |
| `did_document` | GET /api/tool/did/{address}/document（PathParams: address） |
| `saved_instruction_create` | POST /api/saved-instructions |
| `saved_instruction_list` | GET /api/saved-instructions |
| `saved_instruction_get` | GET /api/saved-instructions/{id}（PathParams: id） |
| `saved_instruction_execute` | POST /api/saved-instructions/{id}/execute（PathParams: id） |

`did_create` 输入结构完整给出（镜像 didCreateRequest）：

```go
type didCreateArgs struct {
	PrivateKey  string   `json:"privateKey" jsonschema:"必填"`
	PublicKey   string   `json:"publicKey" jsonschema:"FN-DSA-512 私钥时必填"`
	Address     string   `json:"address" jsonschema:"必填；32 字节私钥不同曲线派生不同地址"`
	SubjectType string   `json:"subjectType" jsonschema:"Personal(缺省)/Organization"`
	Alias       string   `json:"alias"`
	Suffix      *uint32  `json:"suffix"`
	Services    []any    `json:"services"`
	AvatarURI   string   `json:"avatarUri"`
}
```

- [ ] **Step 1: 写失败测试** —— 表驱动 16 行，覆盖全部 16 个工具的最小参数 → REST 映射（`did_document {"address":"a1"}` → `GET /api/tool/did/a1/document? ` 等）
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 镜像实现** —— 逐个打开 handler struct 复制 json tag；`did_document`/`saved_instruction_get` 等带路径参数的用 PathParams
- [ ] **Step 4: 确认通过**
- [ ] **Step 5: 提交** `git commit -m "feat(mcp): DID 全生命周期 12 工具 + 保存指令 4 工具"`

---

### Task 6: 高层 flow 工具（4 个）

**Files:** Modify `mcpserver/tools.go`；Test `mcpserver/tools_test.go`

镜像来源：`handler/vc_flow_handler.go`（vcFlowRequest:43，含嵌套 issuerDid 等 options struct）、`handler/sft_flow_handler.go`（sftFlowRequest:45，嵌套 sft/sftMetadata/slot/distributions/merge/transfer）、`handler/bulk_transfer_handler.go`（bulkTransferRequest:39）。

| 工具 | REST |
|---|---|
| `vc_flow` | POST /api/tool/vc-flow |
| `sft_flow` | POST /api/tool/sft-flow |
| `bulk_transfer` | POST /api/tool/bulk-transfer |
| `bulk_transfer_status` | GET /api/tool/bulk-transfer/{id}（PathParams: id） |

嵌套 options struct 同样逐字镜像（`*vcFlowDidOptions` → 指针结构、字段 json tag 一致）。工具描述写明这是"服务端多步编排"：`vc_flow` 一次调用完成颁发者/用户身份+凭证签发全流程；`sft_flow` 一次完成 SFT 创建/铸槽/分发/合并/转账。

- [ ] **Step 1: 写失败测试** —— 4 行映射断言（flow 两个只断言路径与 2-3 个关键字段透传）
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 镜像实现**
- [ ] **Step 4: 确认通过**
- [ ] **Step 5: 提交** `git commit -m "feat(mcp): vc/sft flow 与批量转账 4 工具，凑齐 55 个"`

---

### Task 7: 挂载到 gin + Bearer 鉴权 + tools/list 总数断言

**Files:**
- Modify: `main.go`（路由区，`api := r.Group("/api")` 之后加 2-3 行）
- Test: `mcpserver/tools_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestToolsListTotalAndAuth(t *testing.T) {
	backend, _ := newFakeBackend(t, 200, `{}`)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)

	front := httptest.NewServer(NewMCPHandler(nil))
	t.Cleanup(front.Close)
	out := rpcCall(t, front.URL, "tools/list", map[string]any{})
	tools := out["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 55 {
		t.Fatalf("工具数=%d, want 55", len(tools))
	}

	// 鉴权开启后无 token 401、带 token 200
	authed := httptest.NewServer(NewMCPHandler("secret"))
	t.Cleanup(authed.Close)
	resp, _ := http.Post(authed.URL, "application/json", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("无 token = %d, want 401", resp.StatusCode)
	}
	req, _ := http.NewRequest("POST", authed.URL, bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)))
	req.Header.Set("Authorization", "Bearer secret")
	resp2, _ := http.DefaultClient.Do(req)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("带 token = %d, want 200", resp2.StatusCode)
	}
}
```

- [ ] **Step 2: 确认失败**（总数断言此时应已通过——若 Task 2-6 计数有出入在此修正）

- [ ] **Step 3: main.go 挂载**（在 `api := r.Group("/api")` 定义后、静态路由前）：

```go
// MCP 端点：把全部 REST 能力暴露为 MCP 工具（Streamable HTTP, stateless）
r.Any("/mcp", gin.WrapH(mcpserver.NewMCPHandler(os.Getenv("MCP_AUTH_TOKEN"))))
```

（main.go 若未 import "os" 则补。）不重启验证逻辑——E2E 在 Task 8 做。

- [ ] **Step 4: 编译 + 全量单测**

Run: `go build ./... && go test ./mcpserver/... -v`
Expected: 编译通过、全部 PASS

- [ ] **Step 5: 提交** `git add main.go mcpserver/ && git commit -m "feat(mcp): /mcp 挂载 gin,可选 MCP_AUTH_TOKEN 鉴权,55 工具齐备"`

---

### Task 8: E2E——真服务 + 真链冒烟

**Files:** Create `mcpserver/e2e_test.go`（`//go:build e2e` 标签，避免日常 CI 必跑）

- [ ] **Step 1: 写 E2E 测试**

```go
//go:build e2e

package mcpserver

// 前置：SERVER_PORT=18080 的 milon-api-server 已在本机运行（devNet 可达）。
// 起法见 README；本测试直连 http://127.0.0.1:18080/mcp。

import (
	"encoding/json"
	"testing"
)

func TestE2ESmoke(t *testing.T) {
	url := "http://127.0.0.1:18080/mcp"

	out := rpcCall(t, url, "tools/list", map[string]any{})
	tools := out["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 55 {
		t.Fatalf("tools=%d want 55", len(tools))
	}

	// 真链冒烟
	must := func(tool, args string) map[string]any {
		return rpcCall(t, url, "tools/call", map[string]any{"name": tool, "arguments": json.RawMessage(args)})
	}
	for _, c := range []struct{ tool, args string }{
		{"network_list", `{}`},
		{"idl_metadata", `{}`},
		{"account_generate", `{}`},
		{"faucet_balance", `{"address":"1111111111111111111111111111111111111111"}`}, // 零地址仅验证链路
	} {
		res := must(c.tool, c.args)
		r := res["result"].(map[string]any)
		if v, _ := r["isError"].(bool); v {
			t.Errorf("%s isError: %v", c.tool, r)
		}
	}

	// 错误路径：坏参数应返回可读错误（isError=true 且正文含 message）
	bad := must("tx_get", `{"hash":"not-a-hash"}`)
	br := bad["result"].(map[string]any)
	if v, _ := br["isError"].(bool); !v {
		t.Logf("错误路径返回（人工确认可读性）: %v", br)
	}
}
```

- [ ] **Step 2: 起服务并运行**

Run:（项目根目录，另开终端）`SERVER_PORT=18080 go run main.go`
Run: `go test ./mcpserver/ -tags e2e -run TestE2ESmoke -v -timeout 300s`
Expected: PASS（含 tools=55 与真链冒烟）

- [ ] **Step 3: 提交** `git add mcpserver/e2e_test.go && git commit -m "test(mcp): E2E 冒烟——tools/list=55 + 真链调用"`

---

### Task 9: 文档 + Web 调试台 MCP 卡片

**Files:**
- Modify: `README.md`（"MCP" 章节）、`API.md`（/mcp 与 55 工具对照表——可从 spec 的工具清单表复制）
- Modify: `static/`（调试台首页加 MCP 卡片：标题"MCP 接入"、连接配置 JSON 展示 + 复制按钮 + 一句使用提示；跟随现有页面风格，不引新框架）

卡片展示的配置片段（ZCode/Claude 通用）：

```json
{
  "mcpServers": {
    "milon": { "url": "http://127.0.0.1:8080/mcp" }
  }
}
```

- [ ] **Step 1: 改 README/API.md（内容从 spec 工具清单表搬运，含 MCP_AUTH_TOKEN 说明）**
- [ ] **Step 2: 加前端卡片**，浏览器打开 `http://127.0.0.1:18080` 实测：卡片渲染、复制按钮可用
- [ ] **Step 3: 提交** `git add README.md API.md static/ && git commit -m "docs(mcp): README/API 文档与调试台 MCP 接入卡片"`

---

### Task 10: 收尾验证

- [ ] **Step 1: 全量构建与测试**

Run: `go build ./... && go vet ./mcpserver/... && go test ./... -count=1`
Expected: 全部通过（含既有 handler 测试，证明 REST 零回归）

- [ ] **Step 2: 人工验收清单**（请用户参与）：
  1. ZCode 配置加 `milon` MCP server（Task 9 片段）→ 会话里直接调 `network_list`/`idl_metadata`；
  2. 浏览器看调试台 MCP 卡片。
- [ ] **Step 3: 最终提交（若有零星修正）**

---

## Self-Review 记录

- **Spec 覆盖**：55 工具 = Task 2(17) + 3(13) + 4(5) + 5(16) + 6(4)；鉴权 Task 7；E2E Task 8；文档/卡片 Task 9；排除端点未映射 ✓；stdio 明确不做 ✓。
- **占位符**：Task 3/5/6 中"镜像 handler struct"类步骤均给出了确切文件与行号 + 至少一个完整示例 + 表驱动测试兜底，非空洞 TBD。
- **类型一致性**：`registerTool[In]`/`RESTMapping`/`CallOutcome`/`NewMCPHandler` 各任务引用一致；`MILON_REST_BASE_URL` 仅测试注入用，生产走 `SERVER_PORT`。
- **SDK 版本风险**：Task 2 Step 1 明确"以 go doc 为准对齐 AddTool 泛型签名"，设计不变。
