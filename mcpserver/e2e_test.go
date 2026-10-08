//go:build e2e

package mcpserver

// E2E 冒烟：直连本机已运行的 milon-api-server（SERVER_PORT=18080，devNet 可达）。
// 起法见 README；日常 go test 不带 e2e 标签不会编译本文件。
// 复用 tools_test.go 的 rpcCall——build tag 是叠加的，-tags e2e 时两文件同包编译。
//
// 运行：go test ./mcpserver/ -tags e2e -run TestE2ESmoke -v -timeout 300s

import (
	"encoding/json"
	"strings"
	"testing"
)

// e2eURL 是被测真服务的 MCP 端点（由外部启动，测试不自起服务）。
const e2eURL = "http://127.0.0.1:18080/mcp"

// e2eCallTools 便捷封装：tools/call 并返回 result 段。
func e2eCallTools(t *testing.T, tool, args string) map[string]any {
	t.Helper()
	out := rpcCall(t, e2eURL, "tools/call", map[string]any{"name": tool, "arguments": json.RawMessage(args)})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("%s: 无 result（工具未注册或请求被拒）: %v", tool, out)
	}
	return res
}

// e2eContentText 取 result 首个 text 内容（REST 响应/错误 JSON 原文）。
func e2eContentText(t *testing.T, res map[string]any) string {
	t.Helper()
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("content 为空: %v", res)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	return text
}

func TestE2ESmoke(t *testing.T) {
	// ---- 1. tools/list：55 个工具齐备 ----
	out := rpcCall(t, e2eURL, "tools/list", map[string]any{})
	tools := out["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 55 {
		t.Fatalf("tools=%d want 55", len(tools))
	}

	// ---- 2. 真链冒烟：无参工具直调 ----
	// 注意：account_generate 此处显式传 keyType——运行中的服务二进制不含
	// Task 8 的 schema 修复（keyType 已改可选），显式传值对新旧 schema 均可过；
	// 修复本身由日常单测 TestBasicToolCallMapping 的空参数用例锁定。
	for _, c := range []struct{ tool, args string }{
		{"network_list", `{}`},
		{"idl_metadata", `{}`},
		{"account_generate", `{"keyType":"secp256k1"}`},
	} {
		res := e2eCallTools(t, c.tool, c.args)
		if v, _ := res["isError"].(bool); v {
			t.Errorf("%s isError: %v", c.tool, e2eContentText(t, res))
		}
	}

	// ---- 3. 多工具串联：account_generate 拿新地址 → faucet_balance 查余额 ----
	// 优先生成新地址而非工具描述里的零地址：顺带验证工具间数据传递。
	// 链端语义（实测 devNet）：从未领水的账户（含零地址）查 faucet_balance
	// 返回链端错误"账户不存在 (code 512)"而非 0 余额——这是 BalanceOf 视图
	// 的契约而非链路故障。故本步断言"响应结构化可读"（能解析出 message 字段
	// 的 JSON），isError 两可；链接通了、参数传对了、错误可读透传即达标。
	gen := e2eCallTools(t, "account_generate", `{"keyType":"secp256k1"}`)
	genText := e2eContentText(t, gen)
	var genBody struct {
		Data struct {
			Address string `json:"address"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(genText), &genBody); err != nil || genBody.Data.Address == "" {
		t.Fatalf("account_generate 响应解析失败（err=%v）: %s", err, genText)
	}
	bal := e2eCallTools(t, "faucet_balance", `{"address":"`+genBody.Data.Address+`"}`)
	balText := e2eContentText(t, bal)
	var balBody struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(balText), &balBody); err != nil || balBody.Message == "" {
		t.Fatalf("faucet_balance(%s) 响应应含 message 字段的可读 JSON（err=%v）: %s",
			genBody.Data.Address, err, balText)
	}
	if balBody.Success && strings.Contains(balText, `"balance"`) {
		t.Logf("faucet_balance 成功返回余额（新账户 %s）: %s", genBody.Data.Address, balText)
	} else {
		t.Logf("faucet_balance 返回链端业务错误（新账户在链上无记录属预期）: %s", balText)
	}

	// ---- 4. 错误路径：坏 hash 透传 REST 错误（可读、含 message）----
	// 断言的是"REST 错误透传"而非 MCP schema 校验错：schema 校验错的正文
	// 形如 `validating "arguments": ...`，不含 REST 错误结构的 message 字段。
	bad := e2eCallTools(t, "tx_get", `{"hash":"not-a-hash"}`)
	if v, _ := bad["isError"].(bool); !v {
		t.Fatalf("tx_get 坏 hash 应 isError=true: %v", bad)
	}
	badText := e2eContentText(t, bad)
	if strings.Contains(badText, `validating "arguments"`) {
		t.Fatalf("错误应来自 REST 透传而非 schema 校验: %s", badText)
	}
	var errBody struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(badText), &errBody); err != nil || strings.TrimSpace(errBody.Message) == "" {
		t.Fatalf("错误正文应含可读 message 字段（got err=%v）: %s", err, badText)
	}
	t.Logf("错误路径正文（人工确认可读性）: %s", badText)
}
