package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

// newTxTrackRouter 组装挂了 ResolveNetwork 的 tx_track 路由，fake RPC 后端
// 按 mode 决定响应：err → status:1（协议错误，GetTxByHash 必失败）；ok →
// status:0 + 空 TxHistory 的 postcard（结构合法，GetTxByHash 可解析）。
func newTxTrackRouter(t *testing.T, mode string) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq struct {
			RequestID uint64 `json:"request_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpcReq)
		resp := struct {
			RequestID uint64 `json:"request_id"`
			Status    uint8  `json:"status"`
			Body      []byte `json:"body"`
		}{RequestID: rpcReq.RequestID, Status: 1}
		if mode == "ok" {
			th := api.TxHistory{}
			ser := postcard.NewSerializerWithCap(64)
			if err := th.MarshalPostcard(ser); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			resp.Status = 0
			resp.Body = ser.Bytes()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(fake.Close)

	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	api.GET("/transactions/:hash/track", NewTransactionHandler(nm).TrackTransaction)
	return r
}

// TestTxTrackNotFound：链上查无此交易（协议错误同样视为不可得）→
// 200 包裹 + data.status="not_found"，不 500——AI 侧拿到可判读的终态。
func TestTxTrackNotFound(t *testing.T) {
	r := newTxTrackRouter(t, "err")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/transactions/deadbeef/track", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("http %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.Data.Status != "not_found" {
		t.Fatalf("want status=not_found, got %q (body=%s)", resp.Data.Status, w.Body.String())
	}
}

// TestTxTrackTimeoutWithSummary：交易可得但 1s 内未确认 → status="timeout"
// 且携带已取得的交易摘要（聚合不因 wait 超时而丢掉已有信息）。
func TestTxTrackTimeoutWithSummary(t *testing.T) {
	r := newTxTrackRouter(t, "ok")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/transactions/Gktrqa8t4D37vwiNRTcfF99tKwgz94DXxMnX7pfrvfMu/track?timeoutSecs=1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("http %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Status      string                 `json:"status"`
			Transaction map[string]interface{} `json:"transaction"`
			Events      []interface{}          `json:"events"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.Data.Status != "timeout" {
		t.Fatalf("want status=timeout, got %q (body=%s)", resp.Data.Status, w.Body.String())
	}
	if resp.Data.Transaction == nil {
		t.Fatalf("timeout 应携带交易摘要, body=%s", w.Body.String())
	}
}
