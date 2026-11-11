package mcpserver

// 一次性测量工具（不入正式套件）：MILON_MEASURE=1 时输出 tools/list 总字节与
// 前 5 大工具，用于瘦身前后对比。跑法：
//   MILON_MEASURE=1 go test ./mcpserver/ -run TestMeasureToolsListSize -v
import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
)

func TestMeasureToolsListSize(t *testing.T) {
	if os.Getenv("MILON_MEASURE") == "" {
		t.Skip("set MILON_MEASURE=1 to run")
	}
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
	n, _ := buf.ReadFrom(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(buf.Bytes(), &out)
	tools := out["result"].(map[string]any)["tools"].([]any)
	total := 0
	sizes := map[string]int{}
	for _, tl := range tools {
		m := tl.(map[string]any)
		b, _ := json.Marshal(m)
		total += len(b)
		sizes[m["name"].(string)] = len(b)
	}
	var names []string
	for k := range sizes {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return sizes[names[i]] > sizes[names[j]] })
	fmt.Printf("TOOLS=%d TOOLS_LIST_BYTES=%d\n", len(tools), total)
	for i := 0; i < 5 && i < len(names); i++ {
		fmt.Printf("  top%d %-28s %d bytes\n", i+1, names[i], sizes[names[i]])
	}
	_ = n
}

// TestToolsListBudget：tools/list 红线（2026-11 六刀防回弹护栏）——工具数 ≤52、
// 总字节 ≤92000。超线即失败：新增工具必须先合并瘦身或压描述。
// 红线依据：六刀合并后实测 50 个 / 89,547B（+3% 余量）；spec 原估 80KB 过于
// 激进——再压需牺牲 schema 自足性（AI 不调 guide 也能懂参数），不值。
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
	tools, _ := out["result"].(map[string]any)["tools"].([]any)
	if len(tools) > 52 {
		t.Errorf("工具数 %d 超红线 52——新工具需先合并瘦身（参照 2026-11 六刀）", len(tools))
	}
	total := 0
	for _, tl := range tools {
		b, _ := json.Marshal(tl)
		total += len(b)
	}
	if total > 92000 {
		t.Errorf("tools/list %d 字节超红线 92000——schema 描述需瘦身", total)
	}
}
