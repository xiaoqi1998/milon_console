package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

// newUtilRouter 组装 util 三个端点路由（派生/签名/验签），fake RPC 兜底
// （这三个端点不触网，NewClient 只加载 IDL）。
func newUtilRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm))
	uh := NewUtilHandler(true)
	api.POST("/util/key/derive-public", uh.DerivePublicKey)
	api.POST("/util/sign", uh.SignMessage)
	api.POST("/util/verify", uh.VerifySignature)
	return r
}

// fndsaAcc 读项目根的 fndsa_acc.json（devNet 实测时生成的真实密钥对）。
func fndsaAcc(t *testing.T) (priv, pub string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "fndsa_acc.json"))
	if err != nil {
		t.Skip("fndsa_acc.json 不存在，跳过 fndsa 用例")
	}
	var acc struct {
		PrivateKey string `json:"privateKey"`
		PublicKey  string `json:"publicKey"`
	}
	if err := json.Unmarshal(b, &acc); err != nil {
		t.Fatal(err)
	}
	return acc.PrivateKey, acc.PublicKey
}

// TestDerivePublicKeyFndsa512ClearError：Go 无法从 fndsa512 私钥派生公钥，
// 错误必须给出明确指引（而不是裸的 not implemented）。
func TestDerivePublicKeyFndsa512ClearError(t *testing.T) {
	r := newUtilRouter(t)
	priv, _ := fndsaAcc(t)
	body := `{"privateKey":"` + priv + `","keyType":"fndsa512"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/util/key/derive-public", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "无法从私钥派生") {
		t.Fatalf("错误应含明确指引: %s", w.Body.String())
	}
}

// TestSignVerifyFndsa512RoundTrip：util_sign 带 publicKey（fndsa512 必填）→
// 签名成功，再用 util_verify 验签闭环。
func TestSignVerifyFndsa512RoundTrip(t *testing.T) {
	r := newUtilRouter(t)
	priv, pub := fndsaAcc(t)

	// 签名（fndsa512 + 显式公钥）
	w := httptest.NewRecorder()
	body := `{"privateKey":"` + priv + `","message":"48656c6c6f","keyType":"fndsa512","publicKey":"` + pub + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/util/sign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sign want 200 got %d: %s", w.Code, w.Body.String())
	}
	var signResp struct {
		Data struct {
			Signature string `json:"signature"`
			PublicKey string `json:"publicKey"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &signResp); err != nil {
		t.Fatal(err)
	}
	if signResp.Data.Signature == "" {
		t.Fatalf("签名为空: %s", w.Body.String())
	}

	// 验签闭环
	w2 := httptest.NewRecorder()
	body2 := `{"publicKey":"` + pub + `","message":"48656c6c6f","signature":"` + signResp.Data.Signature + `","keyType":"fndsa512"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/util/verify", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK || !strings.Contains(w2.Body.String(), "\"valid\":true") {
		t.Fatalf("verify 应通过: %d %s", w2.Code, w2.Body.String())
	}
}
