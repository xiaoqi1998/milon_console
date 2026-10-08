package mcpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
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
		"view_single", "view_multi",
		// Task 4：密钥与签名（5 个）
		"util_derive_address", "util_derive_public_key",
		"util_sign", "util_verify", "vc_attestation",
		// Task 5：DID 全生命周期（12 个）
		"did_create", "did_set_alias",
		"did_add_service", "did_update_service", "did_remove_service",
		"did_set_avatar_uri",
		"did_add_key", "did_update_key", "did_remove_key",
		"did_deactivate", "did_name_binding", "did_document",
		// Task 5：保存指令（4 个）
		"saved_instruction_create", "saved_instruction_list",
		"saved_instruction_get", "saved_instruction_execute",
		// Task 6：高层 flow 工具（4 个）
		"vc_flow", "sft_flow", "bulk_transfer", "bulk_transfer_status"}
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
		// Task 8 缺陷回归：keyType 在 REST 侧可缺省（account_handler.go GenerateAccount
		// 空值缺省 secp256k1），MCP schema 必填集必须与之一致——空参数应放行
		// 并以空 body 到达后端（handler 侧 ContentLength=0 同样走缺省分支）。
		{"account_generate", `{}`, "POST /api/accounts/generate? {}"},
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

// restCallCase 表驱动用例：一次 tools/call 及其期望到达假后端的 REST 请求。
// wantPathQuery 形如 "/api/read?"（无 query）或 "/api/tool/did/name-binding?name=alice-1024"，
// 与 newFakeBackend 记录的 "<method> <path>?<rawQuery> <body>" 第二段对齐；
// GET 请求不产生 body，wantBody 留空且不断言。
type restCallCase struct {
	tool          string
	args          string
	wantMethod    string
	wantPathQuery string
	wantBody      string
}

// assertRestCalls 批量执行 tools/call 并断言假后端逐条收到的 REST 请求与期望一致。
// Task 5 从 Contract/Util 两份复制循环提炼（原第三份出现即提取）：
// POST body 经 map 往返 key 顺序不定，一律 jsonEqual 键值比较（Task 1 教训）。
func assertRestCalls(t *testing.T, frontURL string, seen *[]string, cases []restCallCase) {
	t.Helper()
	for _, c := range cases {
		out := rpcCall(t, frontURL, "tools/call", map[string]any{"name": c.tool, "arguments": json.RawMessage(c.args)})
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
		// seen 记录格式："<method> <path>?<query> <body>"
		parts := strings.SplitN((*seen)[i], " ", 3)
		if len(parts) != 3 || parts[0] != c.wantMethod || parts[1] != c.wantPathQuery {
			t.Errorf("%s: 请求行 got=%q want=%s %s", c.tool, (*seen)[i], c.wantMethod, c.wantPathQuery)
			continue
		}
		if c.wantMethod == "POST" && !jsonEqual(t, parts[2], c.wantBody) {
			t.Errorf("%s: body 不匹配（见上）", c.tool)
		}
	}
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

	cases := []restCallCase{
		// contract_read ← readContractRequest（handler/contract.go:43）
		{"contract_read",
			`{"appName":"identity","methodName":"1071_get_identity","args":{"id":"a1"},"payerAddress":"a1"}`,
			"POST", "/api/read?",
			`{"appName":"identity","methodName":"1071_get_identity","args":{"id":"a1"},"payerAddress":"a1"}`},
		// contract_read_multi ← readContractMultiRequest（handler/contract.go:103）
		{"contract_read_multi",
			`{"instructions":[{"appName":"identity","methodName":"m1","args":{"id":"a1"}}]}`,
			"POST", "/api/read/multi?",
			`{"instructions":[{"appName":"identity","methodName":"m1","args":{"id":"a1"}}]}`},
		// contract_simulate ← simulateContractRequest（handler/contract.go:165）
		// 可选字段标 omitempty：未传/零值键不出现在 REST body（与缺省传给 gin 等效）。
		{"contract_simulate",
			`{"appName":"nft","methodName":"mint","args":{"to":"a1"},"paymentMode":"unified_dual_sign","payerAddress":"a1","signatureMode":{"variant":0},"ixAddress":"a2","ixSignatureMode":{"variant":0}}`,
			"POST", "/api/simulate?",
			`{"appName":"nft","methodName":"mint","args":{"to":"a1"},"paymentMode":"unified_dual_sign","payerAddress":"a1","signatureMode":{"variant":0},"ixAddress":"a2","ixSignatureMode":{"variant":0}}`},
		// contract_simulate_multi ← multiContractRequest（handler/contract.go:1132，SimulateContractMulti:1186）
		{"contract_simulate_multi",
			`{"instructions":[{"appName":"nft","methodName":"mint","args":{}}],"paymentMode":"unified_payer_all","payerAddress":"a1"}`,
			"POST", "/api/simulate/multi?",
			`{"instructions":[{"appName":"nft","methodName":"mint"}],"paymentMode":"unified_payer_all","payerAddress":"a1"}`},
		// contract_write ← writeContractRequest（handler/contract.go:789）
		{"contract_write",
			`{"appName":"nft","methodName":"mint","args":{},"paymentMode":"unified_payer_all","payerPrivateKey":"sk","payerAddress":"a1"}`,
			"POST", "/api/write?",
			`{"appName":"nft","methodName":"mint","paymentMode":"unified_payer_all","payerPrivateKey":"sk","payerAddress":"a1"}`},
		// contract_write_multi ← multiContractRequest（handler/contract.go:1132，WriteContractMulti:1346）
		{"contract_write_multi",
			`{"instructions":[{"appName":"nft","methodName":"mint","args":{}}],"paymentMode":"multi_signer","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`,
			"POST", "/api/write/multi?",
			`{"instructions":[{"appName":"nft","methodName":"mint"}],"paymentMode":"multi_signer","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`},
		// contract_write_multi_agent ← writeContractRequest（WriteContractMultiAgent，handler/contract.go:845）
		{"contract_write_multi_agent",
			`{"appName":"nft","methodName":"transfer","args":{},"paymentMode":"unified_dual_sign","payerPrivateKey":"sk","payerAddress":"a1","ixPrivateKey":"sk2","ixAddress":"a2"}`,
			"POST", "/api/write/multi-agent?",
			`{"appName":"nft","methodName":"transfer","paymentMode":"unified_dual_sign","payerPrivateKey":"sk","payerAddress":"a1","ixPrivateKey":"sk2","ixAddress":"a2"}`},
		// contract_write_multisig ← writeContractRequest（WriteContractMultisig，handler/contract.go:881）
		{"contract_write_multisig",
			`{"appName":"nft","methodName":"burn","args":{},"paymentMode":"split","ownerPrivateKey":"sk","ownerAddress":"a1"}`,
			"POST", "/api/write/multisig?",
			`{"appName":"nft","methodName":"burn","paymentMode":"split","ownerPrivateKey":"sk","ownerAddress":"a1"}`},
		// tx_simulate_raw ← rawTransactionRequest（handler/transaction_handler.go:293）
		{"tx_simulate_raw",
			`{"transactionPostcard":"pc1"}`,
			"POST", "/api/transactions/simulate?",
			`{"transactionPostcard":"pc1"}`},
		// tx_submit_raw ← rawTransactionRequest（handler/transaction_handler.go:293）
		{"tx_submit_raw",
			`{"transactionPostcard":"pc1"}`,
			"POST", "/api/transactions/submit?",
			`{"transactionPostcard":"pc1"}`},
		// tx_inspect_raw ← rawTransactionRequest（InspectTransaction，handler/transaction_handler.go:399）
		{"tx_inspect_raw",
			`{"transactionPostcard":"pc1"}`,
			"POST", "/api/transactions/inspect?",
			`{"transactionPostcard":"pc1"}`},
		// view_single ← rawViewRequest（handler/view_handler.go:48）
		{"view_single",
			`{"transactionPostcard":"pc1"}`,
			"POST", "/api/view/single?",
			`{"transactionPostcard":"pc1"}`},
		// view_multi ← rawViewRequest（ViewMulti，handler/view_handler.go:93）
		{"view_multi",
			`{"transactionPostcard":"pc1"}`,
			"POST", "/api/view/multi?",
			`{"transactionPostcard":"pc1"}`},
		// ---- Task 4 顺手补：Task 3 审查遗留缺口——下述 omitempty 可选字段此前
		// 无正向传值用例，json tag 拼写无测试锁定。3 例分别锁住：
		// contractSimulateArgs 的 ownerAddress/signers/gasPayer、
		// contractMultiArgs 的 ix* 系列、contractWriteArgs 的 signers/gasPayer。----
		{"contract_simulate",
			`{"appName":"nft","methodName":"mint","paymentMode":"multi_signer","ownerAddress":"a3","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`,
			"POST", "/api/simulate?",
			`{"appName":"nft","methodName":"mint","paymentMode":"multi_signer","ownerAddress":"a3","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`},
		{"contract_simulate_multi",
			`{"instructions":[{"appName":"nft","methodName":"mint"}],"paymentMode":"unified_dual_sign","payerAddress":"a1","ixAddress":"a2","ixPrivateKey":"sk2","ixSignatureMode":{"variant":0}}`,
			"POST", "/api/simulate/multi?",
			`{"instructions":[{"appName":"nft","methodName":"mint"}],"paymentMode":"unified_dual_sign","payerAddress":"a1","ixAddress":"a2","ixPrivateKey":"sk2","ixSignatureMode":{"variant":0}}`},
		{"contract_write",
			`{"appName":"nft","methodName":"mint","paymentMode":"multi_signer","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`,
			"POST", "/api/write?",
			`{"appName":"nft","methodName":"mint","paymentMode":"multi_signer","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`},
		// ---- Task 5 顺手补：Task 4 审查遗留——contractMultiArgs 的
		// payerPrivateKey/signatureMode/ownerPrivateKey/ownerAddress 与
		// contractWriteArgs 的 ixSignatureMode 仍无正向传值用例，补 2 例锁拼写。----
		{"contract_write_multi",
			`{"instructions":[{"appName":"nft","methodName":"mint"}],"paymentMode":"split","payerPrivateKey":"sk","signatureMode":{"variant":0},"ownerPrivateKey":"sk2","ownerAddress":"a1"}`,
			"POST", "/api/write/multi?",
			`{"instructions":[{"appName":"nft","methodName":"mint"}],"paymentMode":"split","payerPrivateKey":"sk","signatureMode":{"variant":0},"ownerPrivateKey":"sk2","ownerAddress":"a1"}`},
		{"contract_write_multi_agent",
			`{"appName":"nft","methodName":"transfer","paymentMode":"unified_dual_sign","payerPrivateKey":"sk","payerAddress":"a1","ixPrivateKey":"sk2","ixAddress":"a2","ixSignatureMode":{"variant":0}}`,
			"POST", "/api/write/multi-agent?",
			`{"appName":"nft","methodName":"transfer","paymentMode":"unified_dual_sign","payerPrivateKey":"sk","payerAddress":"a1","ixPrivateKey":"sk2","ixAddress":"a2","ixSignatureMode":{"variant":0}}`},
	}
	assertRestCalls(t, front.URL, seen, cases)
}

// TestUtilToolCallMapping 覆盖 Task 4 的 5 个密钥/签名/VC 工具：
// 断言每个工具把参数镜像为正确的 POST method + path + JSON body。
// 多字段 body 一律 jsonEqual 键值比较（勿依赖 key 顺序——实现经 map 往返）。
// vc_attestation 用三例分别锁住：最小必填集（omitempty 字段不出现）、
// 全 13 字段正向传值（json tag 拼写全锁定）、指针字段 0 值透传
// （validUntilMs=0 表示"不过期"，是业务语义，绝不能被 omitempty 吞掉）。
func TestUtilToolCallMapping(t *testing.T) {
	backend, seen := newFakeBackend(t, 200, `{"code":0}`)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)

	cases := []restCallCase{
		// util_derive_address ← deriveAddressRequest（handler/util.go:25）
		{"util_derive_address",
			`{"publicKey":"pk","keyType":"ed25519"}`,
			"POST", "/api/util/address/derive?",
			`{"publicKey":"pk","keyType":"ed25519"}`},
		// util_derive_address：keyType 可选（omitempty），不传时 body 不含该键
		{"util_derive_address",
			`{"publicKey":"pk"}`,
			"POST", "/api/util/address/derive?",
			`{"publicKey":"pk"}`},
		// util_derive_public_key ← derivePublicKeyRequest（handler/util.go:81）
		{"util_derive_public_key",
			`{"privateKey":"sk","keyType":"ed25519"}`,
			"POST", "/api/util/key/derive-public?",
			`{"privateKey":"sk","keyType":"ed25519"}`},
		// util_sign ← signMessageRequest（handler/util.go:138）
		{"util_sign",
			`{"privateKey":"sk","message":"deadbeef","keyType":"ed25519"}`,
			"POST", "/api/util/sign?",
			`{"privateKey":"sk","message":"deadbeef","keyType":"ed25519"}`},
		// util_verify ← verifySignatureRequest（handler/util.go:214）
		{"util_verify",
			`{"publicKey":"pk","message":"deadbeef","signature":"ab"}`,
			"POST", "/api/util/verify?",
			`{"publicKey":"pk","message":"deadbeef","signature":"ab"}`},
		// vc_attestation ← generateVcAttestationRequest（handler/vc_attestation_handler.go:39）
		// 最小集：issuerPrivateKey + credentialJson，其余 omitempty 字段不得出现
		{"vc_attestation",
			`{"issuerPrivateKey":"sk","credentialJson":"{}"}`,
			"POST", "/api/util/vc-attestation?",
			`{"issuerPrivateKey":"sk","credentialJson":"{}"}`},
		// vc_attestation：全 13 字段正向传值，锁住全部 json tag 拼写
		{"vc_attestation",
			`{"issuerPrivateKey":"sk","issuerPublicKey":"pk","chainId":900000001,"subjectPrivateKey":"ssk","subjectAddress":"subj","issuerKeyId":2,"credentialSchema":"KycLevelCredential","credentialJson":"{}","validUntilMs":1900000000000,"validUntil":"2027-08-24T00:00:00.000Z","credentialName":"n","credentialDesc":"d","issuedAt":"2026-01-01T00:00:00.000Z"}`,
			"POST", "/api/util/vc-attestation?",
			`{"issuerPrivateKey":"sk","issuerPublicKey":"pk","chainId":900000001,"subjectPrivateKey":"ssk","subjectAddress":"subj","issuerKeyId":2,"credentialSchema":"KycLevelCredential","credentialJson":"{}","validUntilMs":1900000000000,"validUntil":"2027-08-24T00:00:00.000Z","credentialName":"n","credentialDesc":"d","issuedAt":"2026-01-01T00:00:00.000Z"}`},
		// vc_attestation：指针字段显式 0 值必须透传（validUntilMs=0 = 不过期）
		{"vc_attestation",
			`{"issuerPrivateKey":"sk","subjectAddress":"subj","validUntilMs":0}`,
			"POST", "/api/util/vc-attestation?",
			`{"issuerPrivateKey":"sk","subjectAddress":"subj","validUntilMs":0}`},
	}
	assertRestCalls(t, front.URL, seen, cases)
}

// TestDidSavedInstructionToolCallMapping 覆盖 Task 5 的 DID 12 工具 + 保存指令 4 工具：
// 断言每个工具把参数镜像为正确的 method + path(+query/路径参数) + JSON body。
// 字段镜像自 handler/did_handler.go 与 handler/saved_instruction_handler.go 的
// request struct（json tag 逐字一致）；GET+query 工具（did_name_binding、
// saved_instruction_execute 的 mode/wait）有带 query 的正向断言；
// saved_instruction_create 全 17 字段正向传值锁住全部 json tag 拼写。
func TestDidSavedInstructionToolCallMapping(t *testing.T) {
	backend, seen := newFakeBackend(t, 200, `{"code":0}`)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)

	cases := []restCallCase{
		// did_create ← didCreateRequest（handler/did_handler.go:48）
		// 最小集：privateKey/address 必填（handler validateDidCreateRequest:188 校验），
		// 其余可选字段 omitempty 不出现。
		{"did_create",
			`{"privateKey":"sk","address":"a1"}`,
			"POST", "/api/tool/did/create?",
			`{"privateKey":"sk","address":"a1"}`},
		// did_create：全 8 字段正向传值（services 元素为 {label,serviceEndpoint}）
		{"did_create",
			`{"privateKey":"sk","publicKey":"pk","address":"a1","subjectType":"Organization","alias":"alice","suffix":1024,"services":[{"label":"blog","serviceEndpoint":"https://example.com"}],"avatarUri":"https://example.com/me.png"}`,
			"POST", "/api/tool/did/create?",
			`{"privateKey":"sk","publicKey":"pk","address":"a1","subjectType":"Organization","alias":"alice","suffix":1024,"services":[{"label":"blog","serviceEndpoint":"https://example.com"}],"avatarUri":"https://example.com/me.png"}`},
		// did_set_alias ← SetAlias 匿名 request（handler/did_handler.go:465）
		{"did_set_alias",
			`{"privateKey":"sk","address":"a1","alias":"alice","suffix":1024}`,
			"POST", "/api/tool/did/set-alias?",
			`{"privateKey":"sk","address":"a1","alias":"alice","suffix":1024}`},
		// did_add_service ← AddService 匿名 request（handler/did_handler.go:502）
		{"did_add_service",
			`{"privateKey":"sk","address":"a1","label":"blog","serviceEndpoint":"https://example.com"}`,
			"POST", "/api/tool/did/add-service?",
			`{"privateKey":"sk","address":"a1","label":"blog","serviceEndpoint":"https://example.com"}`},
		// did_update_service ← UpdateService 匿名 request（handler/did_handler.go:540）
		{"did_update_service",
			`{"privateKey":"sk","address":"a1","id":2,"label":"blog","serviceEndpoint":"https://new.example.com"}`,
			"POST", "/api/tool/did/update-service?",
			`{"privateKey":"sk","address":"a1","id":2,"label":"blog","serviceEndpoint":"https://new.example.com"}`},
		// did_remove_service ← RemoveService 匿名 request（handler/did_handler.go:584）
		{"did_remove_service",
			`{"privateKey":"sk","address":"a1","id":2}`,
			"POST", "/api/tool/did/remove-service?",
			`{"privateKey":"sk","address":"a1","id":2}`},
		// did_set_avatar_uri ← SetAvatarUri 匿名 request（handler/did_handler.go:619）
		{"did_set_avatar_uri",
			`{"privateKey":"sk","address":"a1","avatarUri":"https://example.com/a.png"}`,
			"POST", "/api/tool/did/set-avatar-uri?",
			`{"privateKey":"sk","address":"a1","avatarUri":"https://example.com/a.png"}`},
		// did_add_key ← AddKey 匿名 request（handler/did_handler.go:655），label 可选指针
		{"did_add_key",
			`{"privateKey":"sk","address":"a1","newPublicKey":"pk2","label":"backup"}`,
			"POST", "/api/tool/did/add-key?",
			`{"privateKey":"sk","address":"a1","newPublicKey":"pk2","label":"backup"}`},
		// did_add_key：label 缺省（omitempty 吞 nil 指针，handler 侧 = option<String>::None）
		{"did_add_key",
			`{"privateKey":"sk","address":"a1","newPublicKey":"pk2"}`,
			"POST", "/api/tool/did/add-key?",
			`{"privateKey":"sk","address":"a1","newPublicKey":"pk2"}`},
		// did_update_key ← UpdateKey 匿名 request（handler/did_handler.go:698）
		{"did_update_key",
			`{"privateKey":"sk","address":"a1","id":1,"newPublicKey":"pk2"}`,
			"POST", "/api/tool/did/update-key?",
			`{"privateKey":"sk","address":"a1","id":1,"newPublicKey":"pk2"}`},
		// did_remove_key ← RemoveKey 匿名 request（handler/did_handler.go:742）
		{"did_remove_key",
			`{"privateKey":"sk","address":"a1","id":1}`,
			"POST", "/api/tool/did/remove-key?",
			`{"privateKey":"sk","address":"a1","id":1}`},
		// did_deactivate ← didMutateBase（handler/did_handler.go:60，Deactivate:776 直接绑定）；
		// publicKey 可选（omitempty），不传时 body 只含两个必填字段
		{"did_deactivate",
			`{"privateKey":"sk","address":"a1"}`,
			"POST", "/api/tool/did/deactivate?",
			`{"privateKey":"sk","address":"a1"}`},
		// did_name_binding ← NameBinding（handler/did_handler.go:834）：
		// GET + c.Query("name")，query 正向渲染
		{"did_name_binding",
			`{"name":"alice-1024"}`,
			"GET", "/api/tool/did/name-binding?name=alice-1024",
			""},
		// did_document ← Document（handler/did_handler.go:803）：
		// GET + c.Param("address")，路径参数渲染
		{"did_document",
			`{"address":"a1"}`,
			"GET", "/api/tool/did/a1/document?",
			""},
		// saved_instruction_create ← createSavedInstructionRequest
		//（handler/saved_instruction_handler.go:176）最小必填集
		{"saved_instruction_create",
			`{"name":"n1","appName":"identity","methodName":"m1"}`,
			"POST", "/api/saved-instructions?",
			`{"name":"n1","appName":"identity","methodName":"m1"}`},
		// saved_instruction_create：全 17 字段正向传值，锁住全部 json tag 拼写
		{"saved_instruction_create",
			`{"name":"n1","description":"d","appName":"identity","methodName":"m1","args":{"id":"a1"},"paymentMode":"unified_dual_sign","payerAddress":"a1","payerPrivateKey":"sk","signatureMode":{"variant":0},"ixAddress":"a2","ixPrivateKey":"sk2","ixSignatureMode":{"variant":0},"ownerAddress":"a3","ownerPrivateKey":"sk3","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`,
			"POST", "/api/saved-instructions?",
			`{"name":"n1","description":"d","appName":"identity","methodName":"m1","args":{"id":"a1"},"paymentMode":"unified_dual_sign","payerAddress":"a1","payerPrivateKey":"sk","signatureMode":{"variant":0},"ixAddress":"a2","ixPrivateKey":"sk2","ixSignatureMode":{"variant":0},"ownerAddress":"a3","ownerPrivateKey":"sk3","signers":[{"address":"a1","privateKey":"sk1","signatureMode":{"variant":0}}],"gasPayer":{"address":"a2","privateKey":"sk2","signatureMode":{"variant":0}}}`},
		// saved_instruction_list ← ListSavedInstructions（handler/saved_instruction_handler.go:278）：
		// handler 不读任何 query/param，无参 GET
		{"saved_instruction_list",
			`{}`,
			"GET", "/api/saved-instructions?",
			""},
		// saved_instruction_get ← GetSavedInstruction（handler/saved_instruction_handler.go:284）：
		// GET + c.Param("id")
		{"saved_instruction_get",
			`{"id":"abcd1234"}`,
			"GET", "/api/saved-instructions/abcd1234?",
			""},
		// saved_instruction_execute ← ExecuteSavedInstruction（handler/saved_instruction_handler.go:383）：
		// POST + c.Param("id")；mode/wait 可选 query（DefaultQuery:395/515），缺省不带
		{"saved_instruction_execute",
			`{"id":"abcd1234"}`,
			"POST", "/api/saved-instructions/abcd1234/execute?",
			`{}`},
		// saved_instruction_execute：mode/wait 正向传值（query 渲染，handler 不读 body）
		{"saved_instruction_execute",
			`{"id":"abcd1234","mode":"send","wait":"false"}`,
			"POST", "/api/saved-instructions/abcd1234/execute?mode=send&wait=false",
			`{}`},
	}
	assertRestCalls(t, front.URL, seen, cases)
}

// TestFlowToolCallMapping 覆盖 Task 6 的 4 个高层 flow 工具：
// 断言每个工具把参数镜像为正确的 method + path + JSON body。
// 字段镜像自 handler/vc_flow_handler.go（vcFlowRequest:43 及嵌套
// vcFlowDidOptions[did_handler.go:76]/didServiceSpec[did_handler.go:42]）、
// handler/sft_flow_handler.go（sftFlowRequest:45 及嵌套 sft/sftMetadata/
// slot/distributions/merge/transfer）、handler/bulk_transfer_handler.go
// （bulkTransferRequest:39；GetBulkTransferStatus:196 只读 c.Param("id")，无 query）。
// vc_flow/sft_flow 各有一例正向传一层嵌套 options，锁住嵌套字段 json tag 拼写。
func TestFlowToolCallMapping(t *testing.T) {
	backend, seen := newFakeBackend(t, 200, `{"code":0}`)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)

	cases := []restCallCase{
		// vc_flow ← vcFlowRequest（handler/vc_flow_handler.go:43）
		// 最小集：issuerPrivateKey/userPrivateKey/userAddress 必填
		//（validateVcFlowRequest:66-73 校验），其余 omitempty 不出现。
		{"vc_flow",
			`{"issuerPrivateKey":"sk1","userPrivateKey":"sk2","userAddress":"a2"}`,
			"POST", "/api/tool/vc-flow?",
			`{"issuerPrivateKey":"sk1","userPrivateKey":"sk2","userAddress":"a2"}`},
		// vc_flow：全 11 字段正向传值，issuerDid 传全 5 字段（含 services 元素
		// {label,serviceEndpoint}）、userDid 传一层，锁住全部嵌套 json tag 拼写
		{"vc_flow",
			`{"issuerPrivateKey":"sk1","issuerPublicKey":"pk1","issuerAddress":"a1","userPrivateKey":"sk2","userPublicKey":"pk2","userAddress":"a2","credentialPrefix":"Kyc","credentialCount":3,"validUntilMs":1900000000000,"issuerDid":{"alias":"org-abc","suffix":7,"services":[{"label":"blog","serviceEndpoint":"https://example.com"}],"avatarUri":"https://example.com/a.png","autoAlias":false},"userDid":{"alias":"alice"}}`,
			"POST", "/api/tool/vc-flow?",
			`{"issuerPrivateKey":"sk1","issuerPublicKey":"pk1","issuerAddress":"a1","userPrivateKey":"sk2","userPublicKey":"pk2","userAddress":"a2","credentialPrefix":"Kyc","credentialCount":3,"validUntilMs":1900000000000,"issuerDid":{"alias":"org-abc","suffix":7,"services":[{"label":"blog","serviceEndpoint":"https://example.com"}],"avatarUri":"https://example.com/a.png","autoAlias":false},"userDid":{"alias":"alice"}}`},
		// sft_flow ← sftFlowRequest（handler/sft_flow_handler.go:45）最小必填集
		{"sft_flow",
			`{"ownerPrivateKey":"sk","ownerAddress":"a1"}`,
			"POST", "/api/tool/sft-flow?",
			`{"ownerPrivateKey":"sk","ownerAddress":"a1"}`},
		// sft_flow：全 9 字段正向传值，嵌套 sft（3 字段）/sftMetadata（5 字段）/
		// slot（slotId + metadata 5 字段全指针 + isTransferable）/distributions
		//（含 token 级 metadata 覆盖）/merge/transfer 全展开，锁全部嵌套 tag
		{"sft_flow",
			`{"ownerPrivateKey":"sk","ownerPublicKey":"pk","ownerAddress":"a1","sft":{"address":"sft1","privateKey":"ssk","publicKey":"spk"},"sftMetadata":{"name":"MySFT","symbol":"MSF","coverUrl":"https://example.com/c.png","metadata":"m","attribute":"attr"},"royaltyBps":500,"slot":{"slotId":3,"metadata":{"name":"slot-n","symbol":"slot-s","coverUrl":"c","metadata":"m","attribute":"a"},"isTransferable":false},"distributions":[{"to":"a2","amount":100,"metadata":{"name":"tok-n"}}],"merge":{"fromTokenId":1,"toTokenId":2},"transfer":{"tokenId":5,"to":"a3","amount":40}}`,
			"POST", "/api/tool/sft-flow?",
			`{"ownerPrivateKey":"sk","ownerPublicKey":"pk","ownerAddress":"a1","sft":{"address":"sft1","privateKey":"ssk","publicKey":"spk"},"sftMetadata":{"name":"MySFT","symbol":"MSF","coverUrl":"https://example.com/c.png","metadata":"m","attribute":"attr"},"royaltyBps":500,"slot":{"slotId":3,"metadata":{"name":"slot-n","symbol":"slot-s","coverUrl":"c","metadata":"m","attribute":"a"},"isTransferable":false},"distributions":[{"to":"a2","amount":100,"metadata":{"name":"tok-n"}}],"merge":{"fromTokenId":1,"toTokenId":2},"transfer":{"tokenId":5,"to":"a3","amount":40}}`},
		// bulk_transfer ← bulkTransferRequest（handler/bulk_transfer_handler.go:39）
		// count/toAddress 带 binding:"required"；concurrency 可选（缺省 16，上限 128）
		{"bulk_transfer",
			`{"count":10,"toAddress":"a1","concurrency":32}`,
			"POST", "/api/tool/bulk-transfer?",
			`{"count":10,"toAddress":"a1","concurrency":32}`},
		// bulk_transfer_status ← GetBulkTransferStatus（handler/bulk_transfer_handler.go:196）：
		// GET + c.Param("id")，不读 query；返回任务进度与逐账户结果
		{"bulk_transfer_status",
			`{"id":"1760000000000000000"}`,
			"GET", "/api/tool/bulk-transfer/1760000000000000000?",
			""},
	}
	assertRestCalls(t, front.URL, seen, cases)
}

// TestSchemaRequiredMatchesREST 把 Task 8 的"schema 必填集 ↔ REST 契约"扫描结论
// 固化为断言：tools/list 返回的每个工具 inputSchema.required 必须与 handler
// 事实源（binding:"required" / 空值 400 / 路由 param）一致。此后任何新工具或
// 字段改动改了必填集都会在这里红灯，倒逼回 handler 核对。
// 依据逐工具注明（文件:行为）；排序后比较，不依赖 required 数组顺序。
func TestSchemaRequiredMatchesREST(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	out := rpcCall(t, srv.URL, "tools/list", map[string]any{})
	tools := out["result"].(map[string]any)["tools"].([]any)
	got := map[string][]string{}
	for _, tl := range tools {
		m := tl.(map[string]any)
		var req []string
		if raw, ok := m["inputSchema"].(map[string]any)["required"].([]any); ok {
			for _, r := range raw {
				req = append(req, r.(string))
			}
		}
		sort.Strings(req)
		got[m["name"].(string)] = req
	}

	// 事实源：handler 侧必填证据（binding:"required" 或空值 400 或路由 param 必填）。
	want := map[string][]string{
		// 基础 17：network.go(binding required)、account_handler.go(空值 400/
		// keyType 缺省 secp256k1)、faucet_handler.go(binding required/空值 400)、
		// transaction_handler.go(hash 空值 400)、rpc_read.go(binding required/
		// height param ParseUint)、resource_path_handler.go(空值 400)
		"network_list": {}, "network_current": {},
		"network_switch":          {"network"},
		"account_generate":        {}, // keyType 缺省 secp256k1——Task 8 修复点
		"account_info":            {"address"},
		"account_resources":       {"address"},
		"faucet_claim":            {"privateKey", "address", "signatureMode"},
		"faucet_balance":          {"address"},
		"tx_get":                  {"hash"},
		"tx_parse":                {"hash"},
		"tx_events":               {"hash"},
		"tx_wait":                 {"hash"},
		"rpc_block":               {"height"},
		"rpc_resource":            {"hash"},
		"rpc_access_value":        {"blobHashes"},
		"rpc_resource_path":       {"hash"},
		"idl_metadata":            {},
		// Task 3：contract.go appName/methodName/paymentMode/instructions 均
		// binding:"required"；transaction_handler.go/view_handler.go postcard
		// binding:"required"
		"contract_read":              {"appName", "methodName"},
		"contract_read_multi":        {"instructions"},
		"contract_simulate":          {"appName", "methodName", "paymentMode"},
		"contract_simulate_multi":    {"instructions", "paymentMode"},
		"contract_write":             {"appName", "methodName", "paymentMode"},
		"contract_write_multi":       {"instructions", "paymentMode"},
		"contract_write_multi_agent": {"appName", "methodName", "paymentMode"},
		"contract_write_multisig":    {"appName", "methodName", "paymentMode"},
		"tx_simulate_raw":            {"transactionPostcard"},
		"tx_submit_raw":              {"transactionPostcard"},
		"tx_inspect_raw":             {"transactionPostcard"},
		"view_single":                {"transactionPostcard"},
		"view_multi":                 {"transactionPostcard"},
		// Task 4：util.go 四 handler 均空值 400；vc_attestation_handler.go
		// issuerPrivateKey 空值 400（subject 二选一无法用 required 表达，保持可选）
		"util_derive_address":    {"publicKey"},
		"util_derive_public_key": {"privateKey", "keyType"},
		"util_sign":              {"privateKey", "message", "keyType"},
		"util_verify":            {"publicKey", "message", "signature"},
		"vc_attestation":         {"issuerPrivateKey"},
		// Task 5：did_handler.go validateDidCreateRequest/bindMutate 空值 400，
		// 细粒度端点 alias/label/serviceEndpoint/id/avatarUri/newPublicKey 空 400；
		// saved_instruction_handler.go name/appName/methodName binding required，
		// id 为路由 param
		"did_create":             {"privateKey", "address"},
		"did_set_alias":          {"privateKey", "address", "alias"},
		"did_add_service":        {"privateKey", "address", "label", "serviceEndpoint"},
		"did_update_service":     {"privateKey", "address", "id", "label", "serviceEndpoint"},
		"did_remove_service":     {"privateKey", "address", "id"},
		"did_set_avatar_uri":     {"privateKey", "address", "avatarUri"},
		"did_add_key":            {"privateKey", "address", "newPublicKey"},
		"did_update_key":         {"privateKey", "address", "id", "newPublicKey"},
		"did_remove_key":         {"privateKey", "address", "id"},
		"did_deactivate":         {"privateKey", "address"},
		"did_name_binding":       {"name"},
		"did_document":           {"address"},
		"saved_instruction_create": {"name", "appName", "methodName"},
		"saved_instruction_list":   {},
		"saved_instruction_get":    {"id"},
		"saved_instruction_execute": {"id"},
		// Task 6：vc_flow_handler.go/sft_flow_handler.go validate* 空值 400；
		// bulk_transfer_handler.go binding required；status id 为路由 param
		"vc_flow":              {"issuerPrivateKey", "userPrivateKey", "userAddress"},
		"sft_flow":             {"ownerPrivateKey", "ownerAddress"},
		"bulk_transfer":        {"count", "toAddress"},
		"bulk_transfer_status": {"id"},
	}
	if len(want) != 55 {
		t.Fatalf("用例表=%d, want 55（新工具须回 handler 事实源核对后补行）", len(want))
	}
	for name, w := range want {
		sort.Strings(w)
		g := got[name]
		if g == nil {
			g = []string{} // 无 required（nil）与空集（[]string{}）语义等价，归一后比较
		}
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s: required=%v want %v", name, g, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("工具数=%d want %d：多出的工具=%v", len(got), len(want), func() []string {
			var extra []string
			for n := range got {
				if _, ok := want[n]; !ok {
					extra = append(extra, n)
				}
			}
			return extra
		}())
	}
}

// TestToolsListTotalAndAuth 锁定 Task 7 的两件事：
//  1. tools/list 总数 = 55（基础 17 + Task3 13 + Task4 5 + Task5 16 + Task6 4）；
//  2. Bearer 鉴权三态：无 token 401 / 带对 token 200 / 带错 token 401。
//
// 假后端经 MILON_REST_BASE_URL 注入（tools/list 不触达后端，注入只为与
// 生产同构）；鉴权部分的请求直发 JSON-RPC，不经 rpcCall（它断言 200）。
func TestToolsListTotalAndAuth(t *testing.T) {
	backend, _ := newFakeBackend(t, 200, `{}`)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)

	// 不鉴权：tools/list 总数断言
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)
	out := rpcCall(t, front.URL, "tools/list", map[string]any{})
	tools := out["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 55 {
		t.Fatalf("工具数=%d, want 55", len(tools))
	}

	// 鉴权开启后的三态
	authed := httptest.NewServer(NewMCPHandler("secret"))
	t.Cleanup(authed.Close)
	listReq := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`

	resp, err := http.Post(authed.URL, "application/json", bytes.NewReader([]byte(listReq)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("无 token = %d, want 401", resp.StatusCode)
	}

	req, _ := http.NewRequest("POST", authed.URL, bytes.NewReader([]byte(listReq)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer secret")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("带对 token = %d, want 200", resp2.StatusCode)
	}

	reqWrong, _ := http.NewRequest("POST", authed.URL, bytes.NewReader([]byte(listReq)))
	reqWrong.Header.Set("Content-Type", "application/json")
	reqWrong.Header.Set("Accept", "application/json, text/event-stream")
	reqWrong.Header.Set("Authorization", "Bearer wrong")
	resp3, err := http.DefaultClient.Do(reqWrong)
	if err != nil {
		t.Fatal(err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("带错 token = %d, want 401", resp3.StatusCode)
	}
}
