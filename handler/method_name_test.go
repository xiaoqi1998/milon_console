package handler

// 2026-10 AI 易用性修复（Step G）：方法名大小写宽容解析。
// IDL 指令名是 PascalCase（Transfer），但 AI 高频按 snake_case 传 "transfer"，
// 此前直接报 "IDL method not found: transfer"——按文档描述调用必失败。
// 修复后：精确命中优先，否则不区分大小写唯一匹配即用；无命中保持原错误。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

func newMethodRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1,"message":"fake"}`))
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	ch := NewContractHandler(nm)
	api.POST("/simulate", ch.SimulateContract)
	return r
}

// TestMethodNameCaseInsensitive：snake_case 方法名应能命中 PascalCase 指令。
func TestMethodNameCaseInsensitive(t *testing.T) {
	r := newMethodRouter(t)
	w := httptest.NewRecorder()
	body := `{"appName":"token","methodName":"transfer","args":{"from":"2DSTxAj1h6JtrhyayNFo4heZBa29","to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000},"paymentMode":"unified_payer_all","payerAddress":"2DSTxAj1h6JtrhyayNFo4heZBa29","signatureMode":{"type":"pubkey","publicKey":"0x02f175c4673255dc8e674d70c3d6d45550ccebafb745fbd386fa52ce6f74bb2ef6"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/simulate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "IDL method not found") {
		t.Fatalf("小写方法名应宽容解析为 PascalCase: %s", w.Body.String())
	}
}
