package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ==================== 账号本:自定义测试账号 CRUD ====================
// 用户自有的密钥库(如 sft_1000_testdata.json 那批),与播种派生账号并列展示。
// 私钥为用户主动录入(同 X-Milon-Accounts/MILON_ACCOUNTS 性质),回显含私钥供控制台复制;
// 播种器/secrets 相关的"服务端永不返回私钥"不变量不受影响(seed/status、accounts-config 仍脱敏)。

func newBookHandler(t *testing.T) *AccountBookHandler {
	t.Helper()
	h := NewAccountBookHandler()
	h.path = filepath.Join(t.TempDir(), "custom_accounts.json")
	return h
}

func bookDo(t *testing.T, h *AccountBookHandler, method, target, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/tool/account-book", h.List)
	r.POST("/api/tool/account-book", h.Upsert)
	r.DELETE("/api/tool/account-book/:name", h.Remove)
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var raw map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	return w, raw
}

func bookNames(raw map[string]any) []string {
	data, _ := raw["data"].([]any)
	var out []string
	for _, d := range data {
		m, _ := d.(map[string]any)
		name, _ := m["name"].(string)
		out = append(out, name)
	}
	return out
}

func TestAccountBook_Empty(t *testing.T) {
	h := newBookHandler(t)
	w, raw := bookDo(t, h, "GET", "/api/tool/account-book", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if rows := bookNames(raw); len(rows) != 0 {
		t.Fatalf("空库应返回 [], got %v", rows)
	}
}

func TestAccountBook_UpsertFlatAndDedup(t *testing.T) {
	h := newBookHandler(t)
	body := `[{"name":"u1","privateKey":"8b1c8ff3252ed1fc3db67b1b0abdf59e61f9528cdf42063be152c0d3224a35cc"},
	          {"name":"u2","address":"3R4xzbWqd5N9VKUUeCvxFHKj5mTv","keyType":"ed25519","note":"只录地址"}]`
	w, raw := bookDo(t, h, "POST", "/api/tool/account-book", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if names := bookNames(raw); len(names) != 2 {
		t.Fatalf("应 2 条, got %v", names)
	}
	// 同名再投 = 覆盖,不新增
	w, raw = bookDo(t, h, "POST", "/api/tool/account-book", `[{"name":"u1","address":"eK8r3bnEdiJ7DtUEbA9HESgkfEm"}]`)
	if w.Code != http.StatusOK {
		t.Fatalf("upsert status = %d", w.Code)
	}
	names := bookNames(raw)
	if len(names) != 2 {
		t.Fatalf("同名 upsert 后仍应 2 条, got %v", names)
	}
	// 持久化:重新 GET 仍在
	_, raw = bookDo(t, h, "GET", "/api/tool/account-book", "")
	if names := bookNames(raw); len(names) != 2 {
		t.Fatalf("落盘后应 2 条, got %v", names)
	}
}

// sft_1000_testdata.json 三段式:{owner:{...}, users:[...], newUsers:[...]}
// 实档 owner 无 name 字段:缺名默认 "owner"。
func TestAccountBook_ImportDatasetShape(t *testing.T) {
	h := newBookHandler(t)
	body := `{"owner":{"address":"eK8r3bnEdiJ7DtUEbA9HESgkfEm","privateKey":"8b1c8ff3252ed1fc3db67b1b0abdf59e61f9528cdf42063be152c0d3224a35cc","role":"SFT owner / mint authority / gas payer"},
	          "users":[{"name":"user1","address":"3R4xzbWqd5N9VKUUeCvxFHKj5mTv","privateKey":"34a90389bb3751a2747176b47fff3ba72258787126ce14c361caddca8e52ece6"}],
	          "newUsers":[{"name":"new1","privateKey":"6e2094268729669be15479216741c4f943203582810ecef2ae0a85b4fd6abef2"}]}`
	w, raw := bookDo(t, h, "POST", "/api/tool/account-book", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	names := bookNames(raw)
	if len(names) != 3 || names[0] != "owner" {
		t.Fatalf("应 3 条且首条名 owner, got %v", names)
	}
	// 只有私钥的 new1:服务端补派生地址(与 SeedEnsureAccount 同一 SDK 口径)
	data, _ := raw["data"].([]any)
	for _, d := range data {
		m, _ := d.(map[string]any)
		if m["name"] == "new1" {
			addr, _ := m["address"].(string)
			if addr == "" {
				t.Fatal("new1 仅私钥应自动补地址")
			}
			want := &SeedPlanAccount{PrivateKey: "6e2094268729669be15479216741c4f943203582810ecef2ae0a85b4fd6abef2", Curve: "secp256k1"}
			if err := SeedEnsureAccount(want); err != nil {
				t.Fatal(err)
			}
			if addr != want.Address {
				t.Fatalf("new1 地址 = %s, want %s", addr, want.Address)
			}
		}
	}
}

func TestAccountBook_Invalid(t *testing.T) {
	h := newBookHandler(t)
	cases := []struct {
		name, body string
	}{
		{"缺名字", `[{"address":"eK8r3bnEdiJ7DtUEbA9HESgkfEm"}]`},
		{"无私钥也无地址", `[{"name":"x"}]`},
		{"坏私钥十六进制", `[{"name":"x","privateKey":"zzz"}]`},
		{"非数组非对象", `"hello"`},
	}
	for _, c := range cases {
		w, _ := bookDo(t, h, "POST", "/api/tool/account-book", c.body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400 (%s)", c.name, w.Code, c.body)
		}
	}
}

func TestAccountBook_Remove(t *testing.T) {
	h := newBookHandler(t)
	bookDo(t, h, "POST", "/api/tool/account-book", `[{"name":"u1","address":"a"},{"name":"u2","address":"b"}]`)
	w, raw := bookDo(t, h, "DELETE", "/api/tool/account-book/u1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if names := bookNames(raw); len(names) != 1 || names[0] != "u2" {
		t.Fatalf("删 u1 后应剩 u2, got %v", names)
	}
	w, _ = bookDo(t, h, "DELETE", "/api/tool/account-book/nope", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("删不存在应 404, got %d", w.Code)
	}
}

// 文件损坏视为空库(与 LoadSeedState 容错口径一致)。
func TestAccountBook_CorruptFile(t *testing.T) {
	h := newBookHandler(t)
	if err := os.WriteFile(h.path, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, raw := bookDo(t, h, "GET", "/api/tool/account-book", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if names := bookNames(raw); len(names) != 0 {
		t.Fatalf("损坏文件应视为空库, got %v", names)
	}
}
