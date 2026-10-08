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
	handler := NewMCPHandler("") // 空串 = 不鉴权；内部 BuildServer(默认 baseURL)
	_ = backendURL
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// rpcCall 对 /mcp 直发 JSON-RPC POST，等价于真实 agent 的最小行为。
// Streamable HTTP 协议要求 Accept 同时含 application/json 与 text/event-stream。
func rpcCall(t *testing.T, url, method string, params any) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
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
	if len(names) != len(want) {
		t.Errorf("工具数量=%d, want %d, 实际=%v", len(names), len(want), names)
	}
}

func TestBasicToolCallMapping(t *testing.T) {
	// 假后端记录收到的 REST 请求
	backend, seen := newFakeBackend(t, 200, `{"code":0,"data":{"ok":true}}`)
	// 用环境变量把执行器指向假后端
	t.Setenv("MILON_REST_BASE_URL", backend.URL)
	handler := NewMCPHandler("")
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
