package mcpserver

// 2025-06-18 规范字段合规测试（审计整改第 1 刀，TDD 红测试先行）：
//   1. tools/list 声明层：每个工具带 title + annotations；REST 回环工具带
//      outputSchema；只读/幂等/破坏性 hint 与工具语义一致（抽样断言）
//   2. tools/call 返回层：信封回包出 structuredContent（与 content 并列），
//      字节原样透传（不经 float64 往返，大整数不失真）
//   3. 业务失败（HTTP 200 + success:false）→ isError=true，错误仍机器可读
//   4. 防御：非信封回包不撞 outputSchema 校验——structuredContent 不出、
//      isError 维持 HTTP 状态码判定（SDK 出参校验失败会废掉整个调用）
//
// 实现前本文件应全红；实现后全绿且 tools_test.go 既有用例不回归。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// complianceBackend 起一个固定回包的假 REST 后端。
func complianceBackend(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// complianceFront 把 MCP handler 指向给定假后端。
func complianceFront(t *testing.T, backendURL string) *httptest.Server {
	t.Helper()
	t.Setenv("MILON_REST_BASE_URL", backendURL)
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)
	return front
}

// listToolsRaw 取 tools/list 的原始条目（name → 条目对象）。
func listToolsRaw(t *testing.T, url string) map[string]map[string]any {
	t.Helper()
	out := rpcCall(t, url, "tools/list", map[string]any{})
	tools, _ := out["result"].(map[string]any)["tools"].([]any)
	if tools == nil {
		t.Fatalf("tools/list 无结果: %v", out)
	}
	m := make(map[string]map[string]any, len(tools))
	for _, tl := range tools {
		entry, _ := tl.(map[string]any)
		if entry == nil {
			continue
		}
		name, _ := entry["name"].(string)
		m[name] = entry
	}
	return m
}

func annOf(t *testing.T, tools map[string]map[string]any, name string) map[string]any {
	t.Helper()
	entry, ok := tools[name]
	if !ok {
		t.Fatalf("工具 %s 不在 tools/list", name)
	}
	ann, _ := entry["annotations"].(map[string]any)
	if ann == nil {
		t.Fatalf("%s: 缺 annotations（2025-06-18 规范字段）", name)
	}
	return ann
}

func TestToolDeclarationsModern(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	tools := listToolsRaw(t, srv.URL)
	if len(tools) != wantToolCount {
		t.Fatalf("工具数=%d want %d", len(tools), wantToolCount)
	}

	// 全量：title 非空 + annotations 在；REST 回环工具（除 guide 外）带 outputSchema
	for name, entry := range tools {
		title, _ := entry["title"].(string)
		if strings.TrimSpace(title) == "" {
			t.Errorf("%s: 缺 title（2025-06-18 规范字段）", name)
		}
		if _, ok := entry["annotations"]; !ok {
			t.Errorf("%s: 缺 annotations", name)
		}
		if name != "guide" {
			if _, ok := entry["outputSchema"]; !ok {
				t.Errorf("%s: 缺 outputSchema（REST 回环工具应带信封出参 schema）", name)
			}
		}
	}

	// 只读抽样：GET 工具 + POST 但零状态副作用（dry-run/视图/本地计算）+ guide
	readOnlyWant := []string{
		"network_list", "account_info", "faucet_balance", "tx_get", "idl_methods",
		"contract_read", "contract_read_multi", "contract_simulate", "contract_simulate_multi",
		"tx_simulate_raw", "tx_inspect_raw", "view_single", "view_multi",
		"util_derive_address", "util_sign", "util_verify", "vc_attestation",
		"guide",
	}
	for _, n := range readOnlyWant {
		ann := annOf(t, tools, n)
		if ann["readOnlyHint"] != true {
			t.Errorf("%s: readOnlyHint 应为 true，got %v", n, ann["readOnlyHint"])
		}
		if ann["idempotentHint"] != true {
			t.Errorf("%s: 只读工具 idempotentHint 应为 true，got %v", n, ann["idempotentHint"])
		}
	}

	// account_generate：无状态副作用（只读）但每次新密钥 → 幂等=false
	ann := annOf(t, tools, "account_generate")
	if ann["readOnlyHint"] != true {
		t.Errorf("account_generate: 本地生成密钥对，readOnlyHint 应为 true，got %v", ann["readOnlyHint"])
	}
	if ann["idempotentHint"] == true {
		t.Errorf("account_generate: 每次生成新密钥，idempotentHint 应为 false")
	}

	// 写类抽样：readOnly=false 且为增量写（destructiveHint=false）
	for _, n := range []string{
		"contract_write", "contract_write_safe", "transfer_mil", "faucet_claim",
		"did_add_key", "did_set_alias", "sft_flow", "vc_flow", "saved_instruction_create",
	} {
		ann := annOf(t, tools, n)
		if ann["readOnlyHint"] == true {
			t.Errorf("%s: 写工具 readOnlyHint 应为 false", n)
		}
		if ann["destructiveHint"] != false {
			t.Errorf("%s: 增量写 destructiveHint 应为 false，got %v", n, ann["destructiveHint"])
		}
	}

	// 移除/停用/不可逆：destructiveHint=true
	for _, n := range []string{"did_deactivate", "did_remove_key", "did_remove_service", "saved_instruction_delete"} {
		ann := annOf(t, tools, n)
		if ann["destructiveHint"] != true {
			t.Errorf("%s: 移除/停用类 destructiveHint 应为 true，got %v", n, ann["destructiveHint"])
		}
	}

	// 幂等写：同参重放终态不变
	for _, n := range []string{"did_create", "network_switch", "saved_instruction_update"} {
		ann := annOf(t, tools, n)
		if ann["idempotentHint"] != true {
			t.Errorf("%s: 幂等写 idempotentHint 应为 true，got %v", n, ann["idempotentHint"])
		}
	}
}

func TestStructuredContentOnSuccess(t *testing.T) {
	// 大整数余额：98_000_000_000（98,000 MIL 最小单位）——若实现走
	// map[string]any 解析再序列化，float64 往返会变 9.8e+10 丢精度形态。
	const body = `{"success":true,"message":"ok","code":0,"timestamp":"2026-10-15T00:00:00Z","data":{"balance":98000000000}}`
	front := complianceFront(t, complianceBackend(t, 200, body).URL)

	out := rpcCall(t, front.URL, "tools/call", map[string]any{
		"name": "faucet_balance", "arguments": json.RawMessage(`{"address":"AddrXXXXXXXX"}`),
	})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("无 result: %v", out)
	}
	if v, _ := res["isError"].(bool); v {
		t.Fatalf("isError 应为 false: %v", res)
	}
	sc, _ := res["structuredContent"].(map[string]any)
	if sc == nil {
		t.Fatalf("缺 structuredContent（2025-06-18：与 content 并列返回）: %v", res)
	}
	if sc["success"] != true || sc["message"] != "ok" || sc["code"] != float64(0) {
		t.Errorf("structuredContent 信封字段不符: %v", sc)
	}
	data, _ := sc["data"].(map[string]any)
	if data == nil {
		t.Fatalf("structuredContent.data 缺失: %v", sc)
	}
	// json 解码侧 float64 能精确表示 9.8e10，数值相等即可；关键是原文形态
	// 不带指数——直接检查 wire 文本。
	raw, _ := json.Marshal(sc["data"])
	if !strings.Contains(string(raw), "98000000000") {
		t.Errorf("data.balance 应保持整数形态（非 9.8e+10）: %s", raw)
	}
	// content 与 structuredContent 并列保留（人读 + 机读同帧）
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		t.Errorf("content 应与 structuredContent 并列保留")
	}
}

func TestIsErrorOnBusinessFailure(t *testing.T) {
	// REST 层业务失败的典型形态：HTTP 200 + success:false（链端错误码 512）
	const body = `{"success":false,"message":"账户不存在 (code 512)","code":512,"timestamp":"2026-10-15T00:00:00Z","data":null}`
	front := complianceFront(t, complianceBackend(t, 200, body).URL)

	out := rpcCall(t, front.URL, "tools/call", map[string]any{
		"name": "faucet_balance", "arguments": json.RawMessage(`{"address":"AddrXXXXXXXX"}`),
	})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("无 result: %v", out)
	}
	if v, _ := res["isError"].(bool); !v {
		t.Fatalf("业务失败（HTTP 200 + success:false）应 isError=true（否则错误信号藏在散文里）: %v", res)
	}
	// 错误结果同样机器可读：structuredContent 携带 code/message
	sc, _ := res["structuredContent"].(map[string]any)
	if sc == nil {
		t.Fatalf("错误结果应带 structuredContent: %v", res)
	}
	if sc["code"] != float64(512) || sc["success"] != false {
		t.Errorf("structuredContent 应含 code=512/success=false: %v", sc)
	}
}

func TestHTTPErrorStillIsError(t *testing.T) {
	// HTTP >=300（REST 层真错误）：原有判定不回归
	const body = `{"success":false,"message":"bad request","code":400,"timestamp":"2026-10-15T00:00:00Z","data":null}`
	front := complianceFront(t, complianceBackend(t, 400, body).URL)

	out := rpcCall(t, front.URL, "tools/call", map[string]any{
		"name": "faucet_balance", "arguments": json.RawMessage(`{"address":"AddrXXXXXXXX"}`),
	})
	res, _ := out["result"].(map[string]any)
	if v, _ := res["isError"].(bool); !v {
		t.Fatalf("HTTP 400 应 isError=true: %v", res)
	}
}

func TestNonEnvelopeBodyDoesNotBreakCall(t *testing.T) {
	// 非信封回包（无 success 字段）：structuredContent 不出（防撞出参校验），
	// isError 维持 HTTP 状态码判定，content 原样——老端点/裸文本兼容路径。
	const body = `{"code":0,"data":{"ok":true}}`
	front := complianceFront(t, complianceBackend(t, 200, body).URL)

	out := rpcCall(t, front.URL, "tools/call", map[string]any{
		"name": "faucet_balance", "arguments": json.RawMessage(`{"address":"AddrXXXXXXXX"}`),
	})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("无 result: %v", out)
	}
	if v, _ := res["isError"].(bool); v {
		t.Errorf("非信封回包 + HTTP 200：isError 应维持 false: %v", res)
	}
	if _, has := res["structuredContent"]; has {
		t.Errorf("非信封回包不应出 structuredContent（避免撞 outputSchema 校验）")
	}
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		t.Errorf("content 应原样保留")
	}
}
