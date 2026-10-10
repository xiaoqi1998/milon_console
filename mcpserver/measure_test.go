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
