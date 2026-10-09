package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

// TestNetworkHeaderRouting 是方案 A 的核心证明：fake RPC 后端充当 devNet
// 端点，同一 REST 端点带不同 X-Milon-Network 头，请求必须落到对应网络。
// localNet(127.0.0.1:6280) 在测试环境无服务，用「fake 计数不增加」做反向证明。
func TestNetworkHeaderRouting(t *testing.T) {
	var devNetHits int64
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&devNetHits, 1)
		// SDK 线协议（application/x-milon+json）：请求 {"method":N,"request_id":X,"body":[...]}；
		// 响应须回显 request_id、status=0，body 为 postcard 编码的 ChainHead（[]byte 走 base64）。
		var rpcReq struct {
			RequestID uint64 `json:"request_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpcReq)

		ch := api.ChainHead{ChainId: 900000001, BlockHeight: 42, TimestampMsecs: 1}
		ser := postcard.NewSerializerWithCap(64)
		if err := ch.MarshalPostcard(ser); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(struct {
			RequestID uint64 `json:"request_id"`
			Status    uint8  `json:"status"`
			Body      []byte `json:"body"`
		}{RequestID: rpcReq.RequestID, Status: 0, Body: ser.Bytes()})
	}))
	t.Cleanup(fake.Close)

	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	api.GET("/chain-head", NewSystemHandler(nm).GetChainHead)

	// 不带头（默认=devNet）→ 落 fake
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/chain-head", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("默认网络请求失败: http %d: %s", w.Code, w.Body.String())
	}
	if atomic.LoadInt64(&devNetHits) == 0 {
		t.Fatal("默认网络(缺省头)应路由到 devNet fake 后端")
	}

	// 带 devNet 头 → 落 fake
	before := atomic.LoadInt64(&devNetHits)
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/chain-head", nil)
	req.Header.Set(middleware.NetworkHeader, "devNet")
	r.ServeHTTP(w, req)
	if atomic.LoadInt64(&devNetHits) <= before {
		t.Fatal("X-Milon-Network: devNet 应路由到 devNet fake 后端")
	}

	// 带 localNet 头 → 不落 fake（去了 localNet 端点），响应非 200（127.0.0.1:6280 无服务）
	before = atomic.LoadInt64(&devNetHits)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/chain-head", nil)
	req.Header.Set(middleware.NetworkHeader, "localNet")
	r.ServeHTTP(w, req)
	if atomic.LoadInt64(&devNetHits) != before {
		t.Fatal("X-Milon-Network: localNet 不应路由到 devNet fake 后端")
	}
	if w.Code == http.StatusOK {
		t.Fatalf("localNet 无服务应失败, got %d: %s", w.Code, w.Body.String())
	}

	// 未知网络 → 400
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/chain-head", nil)
	req.Header.Set(middleware.NetworkHeader, "nope")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未知网络应 400, got %d", w.Code)
	}
}
