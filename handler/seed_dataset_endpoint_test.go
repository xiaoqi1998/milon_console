package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// GET /api/tool/seed/dataset:读断点状态文件,按 configHash 命名空间汇总
// (模块 → 阶段 → 原始产物),供总览页展示"当前生效数据集"。
func seedDatasetRequest(t *testing.T, h *SeedJobHandler) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/tool/seed/dataset", h.SeedDataset)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/tool/seed/dataset", nil))
	var raw map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	return w, raw
}

func newDatasetHandler(t *testing.T) *SeedJobHandler {
	t.Helper()
	h := NewSeedJobHandler(nil)
	h.statePath = filepath.Join(t.TempDir(), "seed_state.json")
	return h
}

func TestSeedDataset_MissingFile(t *testing.T) {
	h := newDatasetHandler(t)
	w, raw := seedDatasetRequest(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	data, _ := raw["data"].(map[string]any)
	ds, _ := data["datasets"].([]any)
	if len(ds) != 0 {
		t.Fatalf("缺文件应返回空数据集, got %v", ds)
	}
}

func TestSeedDataset_Namespaces(t *testing.T) {
	h := newDatasetHandler(t)
	content := `{
 "2b6cca834a4e1d06:accounts": {"faucet": {"count": 4}, "did": {"count": 1}},
 "2b6cca834a4e1d06:tokens": {"create:ST01": {"address": "AddrTok"}, "mint:ST01": {"holders": 3}},
 "9999aaaa0000bbbb:sft": {"create:set01": {"address": "AddrSft"}},
 "accounts": {"faucet": {"count": 2}}
}`
	if err := os.WriteFile(h.statePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	w, raw := seedDatasetRequest(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	data, _ := raw["data"].(map[string]any)
	ds, _ := data["datasets"].([]any)
	if len(ds) != 3 {
		t.Fatalf("应 3 个命名空间(2 个 v2 + 1 个 v1 遗留), got %d: %s", len(ds), w.Body.String())
	}

	byHash := map[string]map[string]any{}
	for _, d := range ds {
		m, _ := d.(map[string]any)
		hash, _ := m["configHash"].(string)
		byHash[hash] = m
	}
	// v2 命名空间:模块/阶段/产物原样透出
	ns := byHash["2b6cca834a4e1d06"]
	if ns == nil {
		t.Fatalf("缺 2b6cca 命名空间: %s", w.Body.String())
	}
	mods, _ := ns["modules"].(map[string]any)
	tok, _ := mods["tokens"].(map[string]any)
	create, _ := tok["create:ST01"].(map[string]any)
	if create == nil || create["address"] != "AddrTok" {
		t.Fatalf("tokens.create:ST01 产物缺失: %v", ns)
	}
	faucet, _ := mods["accounts"].(map[string]any)
	f, _ := faucet["faucet"].(map[string]any)
	if f == nil || f["count"] != float64(4) {
		t.Fatalf("accounts.faucet 数据缺失: %v", mods)
	}
	// 阶段计数
	if sc, _ := ns["stageCount"].(float64); sc != 4 {
		t.Fatalf("stageCount = %v, want 4", ns["stageCount"])
	}
	// v1 遗留(无前缀)归 configHash ""
	if byHash[""] == nil {
		t.Fatalf("缺 v1 遗留命名空间: %s", w.Body.String())
	}
}

// 输出确定性:命名空间按 configHash 排序,便于前端与测试。
func TestSeedDataset_Sorted(t *testing.T) {
	h := newDatasetHandler(t)
	content := `{"zzzz:tokens": {"a": {}}, "aaaa:tokens": {"b": {}}, "mmmm:accounts": {"c": {}}}`
	if err := os.WriteFile(h.statePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	w, raw := seedDatasetRequest(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	data, _ := raw["data"].(map[string]any)
	ds, _ := data["datasets"].([]any)
	var order []string
	for _, d := range ds {
		m, _ := d.(map[string]any)
		hash, _ := m["configHash"].(string)
		order = append(order, hash)
	}
	if len(order) != 3 || order[0] != "aaaa" || order[1] != "mmmm" || order[2] != "zzzz" {
		t.Fatalf("命名空间应按 configHash 排序: %v", order)
	}
}

// 状态文件损坏 → 视为空,不 5xx(与 LoadSeedState 容错口径一致)。
func TestSeedDataset_CorruptFile(t *testing.T) {
	h := newDatasetHandler(t)
	if err := os.WriteFile(h.statePath, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, raw := seedDatasetRequest(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	data, _ := raw["data"].(map[string]any)
	ds, _ := data["datasets"].([]any)
	if len(ds) != 0 {
		t.Fatalf("损坏文件应返回空数据集, got %v", ds)
	}
}
