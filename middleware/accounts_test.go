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
	t.Run("65 条超上限 400", func(t *testing.T) {
		// 上限在逐条校验之前拦截，65 个任意结构条目即可触发
		entries := make([]string, 65)
		for i := range entries {
			entries[i] = `{"address":"x"}`
		}
		r, _ := newAccountsRouter()
		req := httptest.NewRequest("GET", "/api/probe", nil)
		req.Header.Set(AccountsHeader, b64("["+strings.Join(entries, ",")+"]"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "X-Milon-Accounts: 账户数 65 超过上限 64") {
			t.Fatalf("want 400+超上限文案 got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("env 校验失败定位 MILON_ACCOUNTS", func(t *testing.T) {
		t.Setenv(AccountsEnv, `[{"address":"`+tAddr+`","privateKey":"`+tPrivHex+`","keyType":"fndsa512"}]`)
		r, _ := newAccountsRouter()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/probe", nil))
		if w.Code != 400 || !strings.Contains(w.Body.String(), "MILON_ACCOUNTS[0]") {
			t.Fatalf("want 400+env 前缀定位 got %d: %s", w.Code, w.Body.String())
		}
	})
}

// 不变量：账户头与私钥永不进日志。挂上生产 LoggerMiddleware(enableBodyLog=true)
// 走 POST + 坏 header + 带 privateKey 的 body——reqBody 脱敏路径、respBody、
// 400 warn 路径一并覆盖；断言捕获的 slog 输出里既无私钥也无 header 原文。
func TestAccountsHeaderNeverLogged(t *testing.T) {
	var buf strings.Builder
	old := redirectLogOutput(&buf) // 须在构造路由前生效，防构造期日志漏网
	defer old()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api", LoggerMiddleware(true), ResolveAccounts())
	api.POST("/probe", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	req := httptest.NewRequest("POST", "/api/probe", strings.NewReader(`{"privateKey":"`+tPrivHex+`"}`))
	hv := "!!!" + b64(`[{"address":"`+tAddr+`","privateKey":"`+tPrivHex+`"}]`) + "!!!" // 坏 base64 且含私钥载荷
	req.Header.Set(AccountsHeader, hv)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("坏 base64 应 400, got %d: %s", w.Code, w.Body.String())
	}
	out := buf.String()
	if !strings.Contains(out, "reqBody") || !strings.Contains(out, "respBody") {
		t.Fatalf("日志中间件未进入 reqBody/respBody 记录路径, 覆盖无效: %s", out)
	}
	if strings.Contains(out, tPrivHex) || strings.Contains(out, hv) {
		t.Fatalf("日志泄露账户/私钥: %s", out)
	}
}
