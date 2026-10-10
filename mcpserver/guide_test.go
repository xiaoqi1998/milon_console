package mcpserver

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// guide_test.go 锁定「文档即工具」的 guide：一次调用返回指定主题的整页手册。
// 三层断言：
//  1. 单元层：六个主题页非空、带 doc_version、含关键锚点；未知主题报可用清单；
//     q 过滤只留命中行；
//  2. RPC 层：guide 是本地工具——后端不可达时仍正常返回（零 REST 依赖），
//     非法 topic 走 isError 并报可用主题；
//  3. 注册层：tools/list 与 inventory 双端都有 guide（由 TestToolInventory 的
//     数量/对账断言兜底，此处只锁描述关键词与分组）。

// TestGuidePageTopics 每个主题页必须非空、带版本号与该页的关键锚点——
// 锚点是「AI 靠本页能答出的问题」的最小集，删锚点等于删文档。
func TestGuidePageTopics(t *testing.T) {
	anchors := map[string][]string{
		"overview":    {"idl_apps", "transfer_mil", "tx_track", "error_lookup"},
		"chain":       {"900000001", "unified_payer_all", "unified_dual_sign", "multi_signer", "fndsa512", "postcard"},
		"workflows":   {"tx_track", "contract_write_safe", "sft_flow", "vc_flow", "bulk_transfer", "saved_instruction"},
		"limits":      {"1042", "1026", "200KB", "20", "5000", "ENABLE_UTIL_SIGN"},
		"performance": {"contract_read_multi", "wait", "account_summary", "idl_apps"},
		"keys":        {"secp256k1", "ed25519", "bls12381", "fndsa512", "897"},
	}
	for topic, wants := range anchors {
		page, ok := guidePage(topic, "")
		if !ok {
			t.Fatalf("主题 %s 不存在", topic)
		}
		if !strings.Contains(page, guideDocVersion) {
			t.Errorf("%s 页缺 doc_version %s", topic, guideDocVersion)
		}
		if len(page) < 400 {
			t.Errorf("%s 页过短（%d 字符），疑似残页", topic, len(page))
		}
		for _, w := range wants {
			if !strings.Contains(page, w) {
				t.Errorf("%s 页缺锚点 %q", topic, w)
			}
		}
	}
}

// TestGuidePageUnknownTopic 未知主题必须报可用清单（与 idl_metadata 传未知名
// 400 报可用 app 清单同构——错误路径即发现路径）。
func TestGuidePageUnknownTopic(t *testing.T) {
	page, ok := guidePage("nope", "")
	if ok {
		t.Fatalf("未知主题不应命中, got %q", page)
	}
	for _, topic := range []string{"overview", "chain", "workflows", "limits", "performance", "keys"} {
		if !strings.Contains(page, topic) {
			t.Errorf("错误提示缺可用主题 %q: %q", topic, page)
		}
	}
}

// TestGuidePageQFilter q 过滤：只保留包含关键词的行（大小写不敏感），
// 结果必须是原页真子集。
func TestGuidePageQFilter(t *testing.T) {
	full, _ := guidePage("limits", "")
	filtered, ok := guidePage("limits", "faucet")
	if !ok {
		t.Fatal("limits 页应存在")
	}
	lines := 0
	for _, ln := range strings.Split(filtered, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		lines++
		if !strings.Contains(strings.ToLower(ln), "faucet") {
			t.Errorf("过滤结果含未命中行: %q", ln)
		}
		if !strings.Contains(full, ln) {
			t.Errorf("过滤结果行不在原页中: %q", ln)
		}
	}
	if lines == 0 {
		t.Fatal("faucet 关键词在 limits 页应有命中")
	}
	if len(filtered) >= len(full) {
		t.Errorf("过滤结果(%d)应短于原页(%d)", len(filtered), len(full))
	}
}

// TestGuideToolCallLocal guide 不触后端：不注入 MILON_REST_BASE_URL（回环
// 指向不可达的 127.0.0.1:8080），overview 仍应正常返回且非 isError——
// 这是「零 IO 秒回」的行为级证明。
func TestGuideToolCallLocal(t *testing.T) {
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)

	out := rpcCall(t, front.URL, "tools/call", map[string]any{
		"name":      "guide",
		"arguments": map[string]any{"topic": "overview"},
	})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("无 result: %v", out)
	}
	if v, _ := res["isError"].(bool); v {
		t.Fatalf("guide 不应报错（本地工具与后端无关）: %v", out)
	}
	cs, _ := res["content"].([]any)
	if len(cs) == 0 {
		t.Fatal("guide 应返回内容")
	}
	text, _ := cs[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "idl_apps") || !strings.Contains(text, guideDocVersion) {
		t.Errorf("overview 页内容异常: %.200s", text)
	}

	// 非法主题：isError=true 且正文报可用主题
	out2 := rpcCall(t, front.URL, "tools/call", map[string]any{
		"name":      "guide",
		"arguments": map[string]any{"topic": "bogus"},
	})
	res2, _ := out2["result"].(map[string]any)
	if v, _ := res2["isError"].(bool); !v {
		t.Fatalf("非法主题应 isError=true: %v", out2)
	}
	cs2, _ := res2["content"].([]any)
	text2, _ := cs2[0].(map[string]any)["text"].(string)
	if !strings.Contains(text2, "overview") {
		t.Errorf("错误正文应列出可用主题: %.200s", text2)
	}
}

// TestGuideRegistered 锁注册面：tools/list 有 guide、描述含主题清单关键词
// （AI 只看描述就该知道六个 topic 与 error_lookup 的分工）、分组为快速手册。
func TestGuideRegistered(t *testing.T) {
	srv := mcpHTTPServer(t, "")
	out := rpcCall(t, srv.URL, "tools/list", map[string]any{})
	var desc string
	found := false
	for _, tl := range out["result"].(map[string]any)["tools"].([]any) {
		m := tl.(map[string]any)
		if m["name"] == "guide" {
			found = true
			desc, _ = m["description"].(string)
		}
	}
	if !found {
		t.Fatal("tools/list 缺 guide")
	}
	for _, kw := range []string{"overview", "chain", "workflows", "limits", "performance", "keys", "error_lookup"} {
		if !strings.Contains(desc, kw) {
			t.Errorf("guide 描述缺关键词 %q: %q", kw, desc)
		}
	}
	if g := toolGroupOf("guide"); g != "快速手册" {
		t.Errorf("guide 分组=%q, want 快速手册", g)
	}
}
