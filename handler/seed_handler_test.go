package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSeedNormalizeModules(t *testing.T) {
	got, err := seedNormalizeModules(nil)
	if err != nil {
		t.Fatalf("nil modules: %v", err)
	}
	if len(got) != len(SeedModuleOrder) {
		t.Fatalf("nil → 全模块, got %v", got)
	}
	got, _ = seedNormalizeModules([]string{"compliance"})
	if len(got) != 2 || got[0] != "vc" || got[1] != "compliance" {
		t.Fatalf("compliance 应自动补 vc 且有序: %v", got)
	}
	got, _ = seedNormalizeModules([]string{"txs", "tokens"})
	if got[0] != "tokens" || got[1] != "txs" {
		t.Fatalf("应按依赖顺序输出: %v", got)
	}
	if _, err := seedNormalizeModules([]string{"nope"}); err == nil {
		t.Fatal("未知模块应报错")
	}
}

func TestSeedPlanPreviewEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSeedJobHandler(nil)
	r := gin.New()
	r.GET("/api/tool/seed/plan", h.SeedPlanPreview)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/tool/seed/plan?scale=small", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Scale string `json:"scale"`
			Users []struct {
				Name string `json:"name"`
			} `json:"users"`
			Tokens []struct {
				Decimals uint8 `json:"decimals"`
			} `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// v2 兼容映射:small → minimal 模板(3 账号/1 代币,规格 §3)
	if !body.Success || body.Data.Scale != "small" || len(body.Data.Users) != 3 || len(body.Data.Tokens) != 1 {
		t.Fatalf("plan 响应异常: success=%v scale=%s users=%d tokens=%d",
			body.Success, body.Data.Scale, len(body.Data.Users), len(body.Data.Tokens))
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/api/tool/seed/plan?scale=huge", nil))
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("非法 scale 应 400, got %d", w2.Code)
	}
}

func TestSeedResolveConfig_Priority(t *testing.T) {
	// config > preset+overrides > scale 映射 > 默认
	req := seedStartRequest{Scale: "small"}
	cfg, err := seedResolveConfig(req)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts.Count != 3 {
		t.Fatalf("scale=small 应映射 minimal(3 账号), got %d", cfg.Accounts.Count)
	}
	req = seedStartRequest{Preset: "full"}
	cfg, err = seedResolveConfig(req)
	if err != nil || cfg.Accounts.Count != 12 {
		t.Fatalf("preset=full 应 12 账号: %v %d", err, cfg.Accounts.Count)
	}
	req = seedStartRequest{Preset: "minimal",
		Overrides: json.RawMessage(`{"accounts":{"count":7},"seedPhrase":"custom-phrase"}`)}
	cfg, err = seedResolveConfig(req)
	if err != nil || cfg.Accounts.Count != 7 || cfg.SeedPhrase != "custom-phrase" {
		t.Fatalf("overrides 应生效: %v %+v", err, cfg.Accounts)
	}
	if cfg.Tokens.Count != 1 {
		t.Fatalf("未覆盖字段应保留 minimal 值, got %d", cfg.Tokens.Count)
	}
	full, _ := SeedConfigForPreset("full")
	req = seedStartRequest{Config: &full}
	cfg, err = seedResolveConfig(req)
	if err != nil || cfg.Accounts.Count != 12 {
		t.Fatalf("config 直传应优先: %v", err)
	}
	req = seedStartRequest{}
	cfg, err = seedResolveConfig(req)
	if err != nil || cfg.Accounts.Count != 8 {
		t.Fatalf("空请求应取默认 standard: %v %d", err, cfg.Accounts.Count)
	}
	req = seedStartRequest{Preset: "nope"}
	if _, err = seedResolveConfig(req); err == nil {
		t.Fatal("未知 preset 应报错")
	}
	req = seedStartRequest{Scale: "typo"}
	if _, err = seedResolveConfig(req); err == nil {
		t.Fatal("非法 scale 应报错,不得静默落默认(对齐 v1 scale 校验)")
	}
}

func TestSeedPlanEndpoint_ValidationErrors(t *testing.T) {
	// 直接构造 handler 调用(gin test recorder),非法 config → 400 且错误含字段路径
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/tool/seed/plan",
		strings.NewReader(`{"config":{"accounts":{"count":0}}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h := NewSeedJobHandler(nil)
	h.SeedPlanV2(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("应 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "accounts.count") {
		t.Fatalf("错误须含字段路径: %s", w.Body.String())
	}
}

func TestSeedJobLogRing(t *testing.T) {
	j := &seedJob{ID: "t"}
	for i := 0; i < seedLogRingSize+50; i++ {
		j.addLog("line %d", i)
	}
	tail := j.logTail()
	if len(tail) != seedLogTailSize {
		t.Fatalf("tail = %d, want %d", len(tail), seedLogTailSize)
	}
	if tail[len(tail)-1] == "" {
		t.Fatal("最新日志不应为空")
	}
}
