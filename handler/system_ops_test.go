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

// TestOpsEndpointsBypassAccountsMiddleware：运维三端点（/api/health、
// /api/chain-head、/api/version）注册在只挂 ResolveNetwork 的运维子组，
// MILON_ACCOUNTS env 配错（坏 JSON）时仍须 200——K8s liveness 探针不能因
// 账户配置错误被 400 劫持而误重启容器。同路由树里的业务组端点仍被
// ResolveAccounts 拦为 400，反向证明中间件未失效、只是作用域收窄。
// 路由构造与 main.go 同构（ops 组与 api 组同前缀 /api，不同中间件链）。
func TestOpsEndpointsBypassAccountsMiddleware(t *testing.T) {
	// fake RPC 后端充当 devNet 端点：health/chain-head 拉链头要打真协议
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	systemHandler := NewSystemHandler(nm)

	// 与 main.go 同构：运维子组只挂 ResolveNetwork（health/chain-head 要
	// 网络 client，version 不读 client 但统一挂载与原行为一致且无害）
	ops := r.Group("/api", middleware.ResolveNetwork(nm))
	{
		ops.GET("/health", systemHandler.Health)
		ops.GET("/chain-head", systemHandler.GetChainHead)
		ops.GET("/version", systemHandler.Version)
	}
	// 业务组保持 ResolveNetwork + ResolveAccounts 不变
	apiGroup := r.Group("/api", middleware.ResolveNetwork(nm), middleware.ResolveAccounts())
	{
		apiGroup.GET("/accounts-config", NewAccountsConfigHandler().List)
	}

	// env 配错：坏 JSON——生产事故形态（MILON_ACCOUNTS 手滑配坏）
	t.Setenv(middleware.AccountsEnv, `{not-json`)

	for _, path := range []string{"/api/health", "/api/chain-head", "/api/version"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("%s 应 200（探活不受 MILON_ACCOUNTS 配错波及）, got %d: %s", path, w.Code, w.Body.String())
		}
	}

	// 反向证明：业务组端点在同 env 下仍被账户中间件拦为 400 且定位到 env 来源
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/accounts-config", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("业务组端点应仍被 ResolveAccounts 拦为 400, got %d: %s", w.Code, w.Body.String())
	}
}
