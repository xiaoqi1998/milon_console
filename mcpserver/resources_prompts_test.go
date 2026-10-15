package mcpserver

// resources / prompts 摆货测试（审计整改第 3 刀，TDD 红测试先行）：
//   1. resources/list 出静态资源（network / tools / idl/apps），
//      resources/templates/list 出两个模板（idl/apps/{app} / guide/{topic}）
//   2. resources/read：本地资源（guide 页、tools 清单）与回环资源
//      （network、idl/apps/{app} 按 REST 路径取数）都出 contents
//   3. 不存在的资源 → JSON-RPC error（ResourceNotFound），不是空成功
//   4. prompts/list 出六个模板；prompts/get 参数渲染进消息文本
//
// 实现前本文件应全红；实现后全绿且既有用例不回归。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// recordingBackend 记录收到的每个 REST 请求（方法+路径），固定回包。
func recordingBackend(t *testing.T, status int, body string) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func listResourcesOf(t *testing.T, url string) []map[string]any {
	t.Helper()
	out := rpcCall(t, url, "resources/list", map[string]any{})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("resources/list 无 result: %v", out)
	}
	items, _ := res["resources"].([]any)
	m := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if e, ok := it.(map[string]any); ok {
			m = append(m, e)
		}
	}
	return m
}

func listResourceTemplatesOf(t *testing.T, url string) []map[string]any {
	t.Helper()
	out := rpcCall(t, url, "resources/templates/list", map[string]any{})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("resources/templates/list 无 result: %v", out)
	}
	items, _ := res["resourceTemplates"].([]any)
	m := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if e, ok := it.(map[string]any); ok {
			m = append(m, e)
		}
	}
	return m
}

func readResource(t *testing.T, url, uri string) map[string]any {
	t.Helper()
	return rpcCall(t, url, "resources/read", map[string]any{"uri": uri})
}

func firstContentText(t *testing.T, out map[string]any) string {
	t.Helper()
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("resources/read 无 result: %v", out)
	}
	contents, _ := res["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("contents 为空: %v", res)
	}
	c, _ := contents[0].(map[string]any)
	text, _ := c["text"].(string)
	return text
}

func TestResourcesListAndTemplates(t *testing.T) {
	srv := mcpHTTPServer(t, "")

	statics := listResourcesOf(t, srv.URL)
	got := map[string]bool{}
	for _, r := range statics {
		got[r["uri"].(string)] = true
		if _, ok := r["name"].(string); !ok || strings.TrimSpace(r["name"].(string)) == "" {
			t.Errorf("资源 %v: 缺 name", r["uri"])
		}
	}
	for _, want := range []string{"milon://network", "milon://tools", "milon://idl/apps"} {
		if !got[want] {
			t.Errorf("resources/list 缺静态资源 %s，实际=%v", want, got)
		}
	}

	tmpls := listResourceTemplatesOf(t, srv.URL)
	tgot := map[string]bool{}
	for _, r := range tmpls {
		tgot[r["uriTemplate"].(string)] = true
	}
	for _, want := range []string{"milon://idl/apps/{app}", "milon://guide/{topic}"} {
		if !tgot[want] {
			t.Errorf("resources/templates/list 缺模板 %s，实际=%v", want, tgot)
		}
	}
}

func TestResourceReadLocalGuide(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	out := readResource(t, srv.URL, "milon://guide/chain")
	text := firstContentText(t, out)
	if len(text) < 200 {
		t.Errorf("guide/chain 页应为大段手册文本，got %d 字节", len(text))
	}
	if !strings.Contains(text, "paymentMode") && !strings.Contains(text, "链") {
		t.Errorf("guide/chain 内容不符（应含链特性/paymentMode）")
	}
}

func TestResourceReadLocalTools(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	text := firstContentText(t, readResource(t, srv.URL, "milon://tools"))
	var inv []map[string]any
	if err := json.Unmarshal([]byte(text), &inv); err != nil {
		t.Fatalf("milon://tools 应为工具清单 JSON: %v\n%s", err, text[:min(200, len(text))])
	}
	if len(inv) != wantToolCount {
		t.Errorf("milon://tools 条目=%d want %d", len(inv), wantToolCount)
	}
}

func TestResourceReadLoopback(t *testing.T) {
	const body = `{"success":true,"message":"ok","code":0,"data":{"apps":["token","identity"]}}`
	backend, seen := recordingBackend(t, 200, body)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)

	// 静态：milon://network → GET /api/network/current
	if text := firstContentText(t, readResource(t, front.URL, "milon://network")); text != body {
		t.Errorf("milon://network 应透传 REST 回包: %s", text)
	}
	// 模板：milon://idl/apps/token → GET /api/idl/apps/token/methods
	if text := firstContentText(t, readResource(t, front.URL, "milon://idl/apps/token")); text != body {
		t.Errorf("milon://idl/apps/token 应透传 REST 回包: %s", text)
	}
	joined := strings.Join(*seen, "\n")
	for _, want := range []string{"GET /api/network/current", "GET /api/idl/apps/token/methods"} {
		if !strings.Contains(joined, want) {
			t.Errorf("回环 REST 缺 %s，实际=\n%s", want, joined)
		}
	}
}

func TestResourceReadNotFound(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	for _, uri := range []string{"milon://guide/nope", "milon://idl/apps/", "milon://bogus"} {
		out := readResource(t, srv.URL, uri)
		if out["error"] == nil {
			t.Errorf("读不存在资源 %s 应返回 JSON-RPC error，got: %v", uri, out)
		}
	}
}

// ---- prompts ----

func listPromptsOf(t *testing.T, url string) map[string]map[string]any {
	t.Helper()
	out := rpcCall(t, url, "prompts/list", map[string]any{})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("prompts/list 无 result: %v", out)
	}
	items, _ := res["prompts"].([]any)
	m := make(map[string]map[string]any, len(items))
	for _, it := range items {
		if e, ok := it.(map[string]any); ok {
			m[e["name"].(string)] = e
		}
	}
	return m
}

func TestPromptsList(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	prompts := listPromptsOf(t, srv.URL)
	for _, want := range []string{
		"milon-onboarding", "milon-transfer", "milon-sft-lifecycle",
		"milon-vc-issuance", "milon-error-triage", "milon-guide",
	} {
		p, ok := prompts[want]
		if !ok {
			t.Errorf("prompts/list 缺 %s，实际=%v", want, prompts)
			continue
		}
		if title, _ := p["title"].(string); strings.TrimSpace(title) == "" {
			t.Errorf("%s: 缺 title", want)
		}
	}

	// milon-transfer 的参数声明：to / amount 必填
	tr := prompts["milon-transfer"]
	args, _ := tr["arguments"].([]any)
	argMap := map[string]map[string]any{}
	for _, a := range args {
		if e, ok := a.(map[string]any); ok {
			argMap[e["name"].(string)] = e
		}
	}
	for _, req := range []string{"to", "amount"} {
		a, ok := argMap[req]
		if !ok {
			t.Errorf("milon-transfer 缺参数声明 %s: %v", req, argMap)
			continue
		}
		if a["required"] != true {
			t.Errorf("milon-transfer 参数 %s 应 required=true", req)
		}
	}
}

func getPrompt(t *testing.T, url, name string, args map[string]any) (string, map[string]any) {
	t.Helper()
	out := rpcCall(t, url, "prompts/get", map[string]any{"name": name, "arguments": args})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("prompts/get %s 无 result: %v", name, out)
	}
	msgs, _ := res["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatalf("prompts/get %s 无 messages: %v", name, res)
	}
	m0, _ := msgs[0].(map[string]any)
	content, _ := m0["content"].(map[string]any)
	text, _ := content["text"].(string)
	return text, res
}

func TestPromptGetRender(t *testing.T) {
	srv := mcpHTTPServer(t, "")

	// transfer：参数渲染进文本
	text, res := getPrompt(t, srv.URL, "milon-transfer", map[string]any{
		"from": "AddrFROM", "to": "AddrTO", "amount": "9800",
	})
	if role, _ := res["messages"].([]any)[0].(map[string]any)["role"].(string); role != "user" {
		t.Errorf("首条消息 role 应为 user，got %v", role)
	}
	for _, want := range []string{"AddrTO", "9800", "transfer_mil", "tx_track"} {
		if !strings.Contains(text, want) {
			t.Errorf("milon-transfer 文本缺 %q:\n%s", want, text)
		}
	}

	// guide：渲染手册页
	gtext, _ := getPrompt(t, srv.URL, "milon-guide", map[string]any{"topic": "keys"})
	if !strings.Contains(gtext, "fndsa") && len(gtext) < 200 {
		t.Errorf("milon-guide(keys) 应渲染 keys 手册页:\n%.200s", gtext)
	}

	// error-triage：query 渲染
	etext, _ := getPrompt(t, srv.URL, "milon-error-triage", map[string]any{"query": "521"})
	for _, want := range []string{"521", "error_lookup"} {
		if !strings.Contains(etext, want) {
			t.Errorf("milon-error-triage 文本缺 %q", want)
		}
	}
}

func TestPromptGetUnknown(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	out := rpcCall(t, srv.URL, "prompts/get", map[string]any{"name": "nope", "arguments": map[string]any{}})
	if out["error"] == nil {
		t.Errorf("prompts/get 未知名字应返回 error: %v", out)
	}
}
