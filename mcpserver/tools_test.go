package mcpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
		"idl_metadata",
		// Task 3：合约/视图/原始交易家族（13 个）
		"contract_read", "contract_read_multi",
		"contract_simulate", "contract_simulate_multi",
		"contract_write", "contract_write_multi",
		"contract_write_multi_agent", "contract_write_multisig",
		"tx_simulate_raw", "tx_submit_raw", "tx_inspect_raw",
		"view_single", "view_multi"}
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
		{"tx_parse", `{"hash":"h1","remote":"true"}`, "GET /api/transactions/h1/parse?remote=true "},
		{"tx_events", `{"hash":"h1","typeTag":"5"}`, "GET /api/transactions/h1/events?typeTag=5 "},
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

// jsonEqual 比较两段 JSON 文本经 unmarshal 后是否深相等（数值统一 float64，
// 规避 key 顺序与整型字面差异——Task 1 教训：勿依赖 JSON key 顺序）。
func jsonEqual(t *testing.T, got, want string) bool {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Errorf("got 不是合法 JSON: %q (err=%v)", got, err)
		return false
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Errorf("want 不是合法 JSON: %q (err=%v)", want, err)
		return false
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("JSON 不等:\n got = %s\nwant = %s", got, want)
		return false
	}
	return true
}

// TestContractToolCallMapping 覆盖 Task 3 的 13 个合约/视图/原始交易工具：
// 断言每个工具把参数镜像为正确的 POST method + path + JSON body。
// 输入结构无 omitempty，故 body 应包含镜像 struct 的全部 json tag
// （未传字段以零值 ""/null 出现）——这同时是"字段镜像无遗漏"的强断言。
func TestContractToolCallMapping(t *testing.T) {
	backend, seen := newFakeBackend(t, 200, `{"code":0}`)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)

	cases := []struct{ tool, args, wantPath, wantBody string }{
		// contract_read ← readContractRequest（handler/contract.go:43）
		{"contract_read",
			`{"appName":"identity","methodName":"1071_get_identity","args":{"id":"a1"},"payerAddress":"a1"}`,
			"/api/read",
			`{"appName":"identity","methodName":"1071_get_identity","args":{"id":"a1"},"payerAddress":"a1"}`},
		// contract_read_multi ← readContractMultiRequest（handler/contract.go:103）
		{"contract_read_multi",
			`{"instructions":[{"appName":"identity","methodName":"m1","args":{"id":"a1"}}]}`,
			"/api/read/multi",
			`{"instructions":[{"appName":"identity","methodName":"m1","args":{"id":"a1"}}]}`},
		// contract_simulate ← simulateContractRequest（handler/contract.go:165）
		// 可选字段标 omitempty：未传/零值键不出现在 REST body（与缺省传给 gin 等效）。
		{"contract_simulate",
			`{"appName":"nft","methodName":"mint","args":{"to":"a1"},"paymentMode":"unified_dual_sign","payerAddress":"a1","signatureMode":{"variant":0},"ixAddress":"a2","ixSignatureMode":{"variant":0}}`,
			"/api/simulate",
			`{"appName":"nft","methodName":"mint","args":{"to":"a1"},"paymentMode":"unified_dual_sign","payerAddress":"a1","signatureMode":{"variant":0},"ixAddress":"a2","ixSignatureMode":{"variant":0}}`},
		// contract_simulate_multi ← multiContractRequest（handler/contract.go:1132，SimulateContractMulti:1186）
		{"contract_simulate_multi",
			`{"instructions":[{"appName":"nft","methodName":"mint","args":{}}],"paymentMode":"unified_payer_all","payerAddress":"a1"}`,
			"/api/simulate/multi",
			`{"instructions":[{"appName":"nft","methodName":"mint"}],"paymentMode":"unified_payer_all","payerAddress":"a1"}`},
		// contract_write ← writeContractRequest（handler/contract.go:789）
		{"contract_write",
			`{"appName":"nft","methodName":"mint","args":{},"paymentMode":"unified_payer_all","payerPrivateKey":"sk","payerAddress":"a1"}`,
			"/api/write",
			`{"appName":"nft","methodName":"mint","paymentMode":"unified_payer_all","payerPrivateKey":"sk","payerAddress":"a1"}`},
		// contract_write_multi ← multiContractRequest（handler/contract.go:1132，WriteContractMulti:1346）
		{"contract_write_multi",
			`{"instructions":[{"appName":"nft","methodName":"mint","args":{}}],"paymentMode":"multi_signer","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`,
			"/api/write/multi",
			`{"instructions":[{"appName":"nft","methodName":"mint"}],"paymentMode":"multi_signer","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`},
		// contract_write_multi_agent ← writeContractRequest（WriteContractMultiAgent，handler/contract.go:845）
		{"contract_write_multi_agent",
			`{"appName":"nft","methodName":"transfer","args":{},"paymentMode":"unified_dual_sign","payerPrivateKey":"sk","payerAddress":"a1","ixPrivateKey":"sk2","ixAddress":"a2"}`,
			"/api/write/multi-agent",
			`{"appName":"nft","methodName":"transfer","paymentMode":"unified_dual_sign","payerPrivateKey":"sk","payerAddress":"a1","ixPrivateKey":"sk2","ixAddress":"a2"}`},
		// contract_write_multisig ← writeContractRequest（WriteContractMultisig，handler/contract.go:881）
		{"contract_write_multisig",
			`{"appName":"nft","methodName":"burn","args":{},"paymentMode":"split","ownerPrivateKey":"sk","ownerAddress":"a1"}`,
			"/api/write/multisig",
			`{"appName":"nft","methodName":"burn","paymentMode":"split","ownerPrivateKey":"sk","ownerAddress":"a1"}`},
		// tx_simulate_raw ← rawTransactionRequest（handler/transaction_handler.go:293）
		{"tx_simulate_raw",
			`{"transactionPostcard":"pc1"}`,
			"/api/transactions/simulate",
			`{"transactionPostcard":"pc1"}`},
		// tx_submit_raw ← rawTransactionRequest（handler/transaction_handler.go:293）
		{"tx_submit_raw",
			`{"transactionPostcard":"pc1"}`,
			"/api/transactions/submit",
			`{"transactionPostcard":"pc1"}`},
		// tx_inspect_raw ← rawTransactionRequest（InspectTransaction，handler/transaction_handler.go:399）
		{"tx_inspect_raw",
			`{"transactionPostcard":"pc1"}`,
			"/api/transactions/inspect",
			`{"transactionPostcard":"pc1"}`},
		// view_single ← rawViewRequest（handler/view_handler.go:48）
		{"view_single",
			`{"transactionPostcard":"pc1"}`,
			"/api/view/single",
			`{"transactionPostcard":"pc1"}`},
		// view_multi ← rawViewRequest（ViewMulti，handler/view_handler.go:93）
		{"view_multi",
			`{"transactionPostcard":"pc1"}`,
			"/api/view/multi",
			`{"transactionPostcard":"pc1"}`},
	}
	for _, c := range cases {
		out := rpcCall(t, front.URL, "tools/call", map[string]any{"name": c.tool, "arguments": json.RawMessage(c.args)})
		res, _ := out["result"].(map[string]any)
		if res == nil {
			t.Errorf("%s: 调用未返回 result（工具未注册或参数被拒）: %v", c.tool, out)
			continue
		}
		if v, _ := res["isError"].(bool); v {
			t.Errorf("%s: unexpected isError: %v", c.tool, out)
		}
	}
	if len(*seen) != len(cases) {
		t.Fatalf("后端收到 %d 个请求, want %d: %v", len(*seen), len(cases), *seen)
	}
	for i, c := range cases {
		// seen 记录格式："POST <path>?<query> <body>"
		parts := strings.SplitN((*seen)[i], " ", 3)
		if len(parts) != 3 || parts[0] != "POST" || parts[1] != c.wantPath+"?" {
			t.Errorf("%s: 请求行 got=%q want=POST %s?", c.tool, (*seen)[i], c.wantPath)
			continue
		}
		if !jsonEqual(t, parts[2], c.wantBody) {
			t.Errorf("%s: body 不匹配（见上）", c.tool)
		}
	}
}
