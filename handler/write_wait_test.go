package handler

// 2026-10 AI 提速（Step 2）：写交易 wait 选项。
// 此前 contract_write 返回 txHash 即结束，AI 必须再调 tx_track 确认（忘了就是
// 静默失败）。修复后：wait=true 时同请求内等待确认（waitTimeoutSecs 缺省 60s），
// 响应带 confirmed 布尔与 waitError——成功路径 2 次调用并 1 次。

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/api"
	lib "github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

// newWriteWaitRouter：fake RPC 双传输分流——postcard 请求（SubmitTx）回 postcard
// 编码的 RpcResponse（status ok）；JSON 请求（View/GetTx 轮询）回 JSON status:0 +
// 空 TxHistory postcard（不确认 → 按 waitTimeoutSecs 超时）。
func newWriteWaitRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var requestID uint64
		if r.Header.Get("Content-Type") == "application/x-milon+postcard" {
			var preq lib.RpcRequest
			if _, err := postcard.DeserializePostcard(raw, func(d *postcard.Deserializer) (lib.RpcRequest, error) {
				return preq, preq.UnmarshalPostcard(d)
			}, false); err == nil {
				requestID = uint64(preq.RequestId)
			}
			rsp := lib.RpcResponse{RequestId: requestID, Status: 0}
			ser := postcard.NewSerializerWithCap(64)
			if err := rsp.MarshalPostcard(ser); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/x-milon+postcard")
			_, _ = w.Write(ser.Bytes())
			return
		}
		var jreq struct {
			RequestID uint64 `json:"request_id"`
		}
		_ = json.Unmarshal(raw, &jreq)
		th := api.TxHistory{}
		ser := postcard.NewSerializerWithCap(64)
		_ = th.MarshalPostcard(ser)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			RequestID uint64 `json:"request_id"`
			Status    uint8  `json:"status"`
			Body      []byte `json:"body"`
		}{RequestID: jreq.RequestID, Status: 0, Body: ser.Bytes()})
	}))
	t.Cleanup(fake.Close)

	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	apiGroup := r.Group("/api", middleware.ResolveNetwork(nm))
	ch := NewContractHandler(nm)
	apiGroup.POST("/write", ch.WriteContract)
	return r
}

// TestWriteWaitOption：wait=true + 短超时 → 响应直接带 confirmed=false 与
// waitError（超时），不再需要 AI 二次调 tx_track 才知道没确认。
func TestWriteWaitOption(t *testing.T) {
	r := newWriteWaitRouter(t)
	w := httptest.NewRecorder()
	body := `{"appName":"token","methodName":"Transfer","args":{"from":"2DSTxAj1h6JtrhyayNFo4heZBa29","to":"2DSTxAj1h6JtrhyayNFo4heZBa29","token":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000},` +
		`"paymentMode":"unified_payer_all","payerPrivateKey":"` + testSeedHex + `","wait":true,"waitTimeoutSecs":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/write", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("http %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			TxHash    string `json:"txHash"`
			Confirmed *bool  `json:"confirmed"`
			WaitError string `json:"waitError"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.Data.TxHash == "" {
		t.Fatalf("应提交成功并返回 txHash: %s", w.Body.String())
	}
	if resp.Data.Confirmed == nil {
		t.Fatalf("wait=true 应在响应中携带 confirmed 字段（省一次 tx_track）: %s", w.Body.String())
	}
	if *resp.Data.Confirmed {
		t.Fatalf("fake 链上无确认，confirmed 应为 false: %s", w.Body.String())
	}
	if resp.Data.WaitError == "" {
		t.Fatalf("等待失败应带 waitError: %s", w.Body.String())
	}
}

// TestWriteNoWaitByDefault：缺省不等待——响应不含 confirmed 字段（行为不变）。
func TestWriteNoWaitByDefault(t *testing.T) {
	r := newWriteWaitRouter(t)
	w := httptest.NewRecorder()
	body := `{"appName":"token","methodName":"Transfer","args":{"from":"2DSTxAj1h6JtrhyayNFo4heZBa29","to":"2DSTxAj1h6JtrhyayNFo4heZBa29","token":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000},` +
		`"paymentMode":"unified_payer_all","payerPrivateKey":"` + testSeedHex + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/write", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("http %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "\"confirmed\"") {
		t.Fatalf("缺省不等待，不应有 confirmed 字段: %s", w.Body.String())
	}
}
