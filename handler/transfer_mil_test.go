package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/api"
	lib "github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

// newTransferMilRouter 组装 POST /api/tool/transfer-mil 路由。
// fake RPC 后端返回协议错误（status:1）——足够验证「参数解析→密钥派生→
// 构造写请求→进入提交层」的链路；真实上链在 devNet 冒烟覆盖。
func newTransferMilRouter(t *testing.T) *gin.Engine {
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
	api.POST("/tool/transfer-mil", NewContractHandler(nm).TransferMil)
	return r
}

// postTransferMil 发请求并返回 (status, body)。
func postTransferMil(r *gin.Engine, body string) (int, string) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tool/transfer-mil", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// TestTransferMilValidation：缺必填/非法 keyType/非法私钥 → 400 且信息可读。
func TestTransferMilValidation(t *testing.T) {
	r := newTransferMilRouter(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"缺 privateKey", `{"to":"2DSTx","amount":100}`, "privateKey"},
		{"非法私钥", `{"to":"2DSTx","amount":100,"privateKey":"not-a-key"}`, ""},
		{"非法 keyType", `{"to":"2DSTx","amount":100,"privateKey":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","keyType":"rsa"}`, "keyType"},
	}
	for _, tc := range cases {
		code, body := postTransferMil(r, tc.body)
		if code != http.StatusBadRequest {
			t.Errorf("%s: want 400 got %d: %s", tc.name, code, body)
		}
	}
}

// TestTransferMilReachesSubmit：合法输入应完整走完 派生→构造→提交 链路，
// 在提交层因 fake 协议错误而失败（500），证明前置构造全部成功。
func TestTransferMilReachesSubmit(t *testing.T) {
	r := newTransferMilRouter(t)
	code, body := postTransferMil(r, `{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000000,"privateKey":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"}`)
	if code != http.StatusInternalServerError {
		t.Fatalf("want 500（fake 后端拒绝提交）got %d: %s", code, body)
	}
	if !strings.Contains(body, "failed to transfer") && !strings.Contains(body, "error") {
		t.Fatalf("应包含错误上下文: %s", body)
	}
}

// TestTransferMilFnDsa512NeedsPublicKey：Go SDK 无法从 fndsa512 私钥派生
// 公钥（FnDsa512Public 为 TODO not implemented）——keyType=fndsa512 时
// publicKey 必填，缺失应 400 且提示明确；带公钥则正常进入提交层。
func TestTransferMilFnDsa512NeedsPublicKey(t *testing.T) {
	r := newTransferMilRouter(t)
	fndsaAcc, err := readTestFile("fndsa_acc.json")
	if err != nil {
		t.Skip("fndsa 测试账户未生成，跳过")
	}
	var acc struct {
		PrivateKey string `json:"privateKey"`
		PublicKey  string `json:"publicKey"`
	}
	_ = json.Unmarshal([]byte(fndsaAcc), &acc)

	// 缺 publicKey → 400 且信息指路
	body := `{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000,"privateKey":"` + acc.PrivateKey + `","keyType":"fndsa512"}`
	code, respBody := postTransferMil(r, body)
	if code != http.StatusBadRequest || !strings.Contains(respBody, "publicKey") {
		t.Fatalf("fndsa512 缺 publicKey 应 400 并提示, got %d: %s", code, respBody)
	}

	// 带 publicKey → 走到提交层（fake 拒绝 → 500）
	body = `{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000,"privateKey":"` + acc.PrivateKey + `","keyType":"fndsa512","publicKey":"` + acc.PublicKey + `"}`
	code, respBody = postTransferMil(r, body)
	if code != http.StatusInternalServerError {
		t.Fatalf("fndsa512 带 publicKey 应进入提交层(500), got %d: %s", code, respBody)
	}
}

// readTestFile 读项目根下的测试数据文件。
func readTestFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join("..", name))
}

// ==================== wait 选项（2026-11 六刀刀4a） ====================

// newTransferMilWaitRouter：fake RPC 双传输分流（同 write_wait_test 模式）——
// postcard（SubmitTx）回 status ok；JSON（轮询）回 status:0 + 空 TxHistory
// （不确认 → wait 按超时路径返回 confirmed=false）。
func newTransferMilWaitRouter(t *testing.T) *gin.Engine {
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
	apiGroup.POST("/tool/transfer-mil", NewContractHandler(nm).TransferMil)
	return r
}

// TestTransferMilWaitTrue：wait=true 时响应带 confirmed（fake 不确认 → false
// 与 waitError）；缺省不带 wait 字段行为不变（脚本/前端零影响）。
func TestTransferMilWaitTrue(t *testing.T) {
	r := newTransferMilWaitRouter(t)
	priv := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

	// wait=true → confirmed 字段在
	w := httptest.NewRecorder()
	body := `{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000,"privateKey":"` + priv + `","wait":true,"waitTimeoutSecs":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/tool/transfer-mil", strings.NewReader(body))
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
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success || resp.Data.TxHash == "" {
		t.Fatalf("应提交成功: %s", w.Body.String())
	}
	if resp.Data.Confirmed == nil || *resp.Data.Confirmed != false || resp.Data.WaitError == "" {
		t.Fatalf("wait=true 应带 confirmed=false 与 waitError: %s", w.Body.String())
	}

	// 缺省 → 无 confirmed 字段（现状不变）
	w2 := httptest.NewRecorder()
	body2 := `{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1000,"privateKey":"` + priv + `"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/tool/transfer-mil", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK || strings.Contains(w2.Body.String(), `"confirmed"`) {
		t.Fatalf("缺省不带 confirmed: %d %s", w2.Code, w2.Body.String())
	}
}
