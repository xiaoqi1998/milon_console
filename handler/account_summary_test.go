package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

// newAccountSummaryRouter 组装 GET /api/accounts/:address/summary 路由。
// fake RPC 返回协议错误——聚合视图必须把子项失败折叠为 null 而非 500。
func newAccountSummaryRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq struct {
			RequestID uint64 `json:"request_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpcReq)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			RequestID uint64 `json:"request_id"`
			Status    uint8  `json:"status"`
		}{RequestID: rpcReq.RequestID, Status: 1})
	}))
	t.Cleanup(fake.Close)

	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	api.GET("/accounts/:address/summary", NewAccountHandler(nm).Summary)
	return r
}

// TestAccountSummaryDegraded：全部子查询失败（fake 协议错误）时仍返回 200，
// 各子项为 null + errors 说明——AI 侧拿到可判读的部分视图而非整体失败。
func TestAccountSummaryDegraded(t *testing.T) {
	r := newAccountSummaryRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/accounts/2DSTxAj1h6JtrhyayNFo4heZBa29/summary", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Address string          `json:"address"`
			Balance *uint64         `json:"balance"`
			DID     json.RawMessage `json:"did"`
			Errors  map[string]any  `json:"errors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.Data.Address != "2DSTxAj1h6JtrhyayNFo4heZBa29" {
		t.Fatalf("address 回显缺失: %s", w.Body.String())
	}
	if resp.Data.Balance != nil {
		t.Fatalf("fake 失败时 balance 应为 null: %s", w.Body.String())
	}
	if len(resp.Data.Errors) == 0 {
		t.Fatalf("子项失败应记录 errors: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "balance") {
		t.Fatal("errors 应含 balance 键")
	}
}
