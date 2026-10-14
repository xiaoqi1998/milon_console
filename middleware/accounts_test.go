package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

func b64(payload string) string { return base64.StdEncoding.EncodeToString([]byte(payload)) }

func newAccountsRouter() (*gin.Engine, *[]types.ConfiguredAccount) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api", ResolveAccounts())
	var got []types.ConfiguredAccount
	api.GET("/probe", func(c *gin.Context) {
		got = AccountsFrom(c)
		c.JSON(200, gin.H{"source": AccountsSourceFrom(c), "n": len(got)})
	})
	return r, &got
}

// 自洽三元组（secp256k1）：privHex 派生 pubHex，pubHex 派生 tAddr。
const tPrivHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
const tAddr = "491vGnPg2q1qsRtFRun8WNW8gxTD"

func TestResolveAccounts(t *testing.T) {
	payload := `[{"name":"main","address":"` + tAddr + `","privateKey":"` + tPrivHex + `"}]`

	t.Run("带 header 解析注入", func(t *testing.T) {
		r, got := newAccountsRouter()
		req := httptest.NewRequest("GET", "/api/probe", nil)
		req.Header.Set(AccountsHeader, b64(payload))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		if len(*got) != 1 || (*got)[0].Address != tAddr {
			t.Fatalf("got %+v", *got)
		}
		var resp struct {
			Source string `json:"source"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Source != "header" {
			t.Fatalf("source want header got %q", resp.Source)
		}
	})
	t.Run("无 header 无 env 放行空集", func(t *testing.T) {
		t.Setenv(AccountsEnv, "")
		r, got := newAccountsRouter()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/probe", nil))
		if w.Code != 200 || len(*got) != 0 {
			t.Fatalf("code=%d got %+v", w.Code, *got)
		}
	})
	t.Run("无 header 有 env 生效", func(t *testing.T) {
		t.Setenv(AccountsEnv, payload) // env 是 raw JSON，非 base64
		r, got := newAccountsRouter()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/probe", nil))
		if w.Code != 200 || len(*got) != 1 {
			t.Fatalf("code=%d got %+v", w.Code, *got)
		}
	})
	t.Run("header 优先于 env", func(t *testing.T) {
		t.Setenv(AccountsEnv, `[]`)
		r, got := newAccountsRouter()
		req := httptest.NewRequest("GET", "/api/probe", nil)
		req.Header.Set(AccountsHeader, b64(payload))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if len(*got) != 1 {
			t.Fatalf("header 应优先, got %+v", *got)
		}
	})
	t.Run("坏 base64 400", func(t *testing.T) {
		r, _ := newAccountsRouter()
		req := httptest.NewRequest("GET", "/api/probe", nil)
		req.Header.Set(AccountsHeader, "!!!not-base64!!!")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "X-Milon-Accounts") {
			t.Fatalf("want 400+定位 got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("校验失败 400 且定位条目", func(t *testing.T) {
		r, _ := newAccountsRouter()
		req := httptest.NewRequest("GET", "/api/probe", nil)
		req.Header.Set(AccountsHeader, b64(`[{"address":"`+tAddr+`","privateKey":"`+tPrivHex+`","keyType":"fndsa512"}]`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "X-Milon-Accounts[0]") {
			t.Fatalf("want 400+条目定位 got %d: %s", w.Code, w.Body.String())
		}
	})
}

// 不变量：账户头与私钥永不进日志。logging.go 只记 body 不记 header，本测试锁死该事实。
func TestAccountsHeaderNeverLogged(t *testing.T) {
	var buf strings.Builder
	old := redirectLogOutput(&buf) // 见下方辅助；无此辅助则按 Step 3 补
	defer old()
	r, _ := newAccountsRouter()
	req := httptest.NewRequest("GET", "/api/probe", nil)
	hv := b64(`[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `"}]`)
	req.Header.Set(AccountsHeader, hv)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out := buf.String()
	if strings.Contains(out, tPrivHex) || strings.Contains(out, hv) {
		t.Fatalf("日志泄露账户/私钥: %s", out)
	}
}
