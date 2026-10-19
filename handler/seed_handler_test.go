package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
