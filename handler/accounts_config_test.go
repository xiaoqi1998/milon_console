package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/middleware"
)

// TestAccountsConfigList 锁定 GET /api/accounts-config 脱敏回显不变量：
// 响应永不出现 privateKey 字段名与私钥/公钥全文，公钥只回 sha256 指纹前
// 8 hex；空集时 message 附 X-Milon-Accounts 配置指引。
func TestAccountsConfigList(t *testing.T) {
	r := newAccountsContractRouter(t) // 脚手架补挂：api.GET("/accounts-config", NewAccountsConfigHandler().List)
	req := httptest.NewRequest("GET", "/api/accounts-config", nil)
	req.Header.Set(middleware.AccountsHeader, base64.StdEncoding.EncodeToString(
		[]byte(`[{"name":"main","address":"`+cAddr+`","privateKey":"`+cPrivHex+`"}]`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, banned := range []string{cPrivHex, "privateKey"} {
		if strings.Contains(body, banned) {
			t.Fatalf("响应泄露敏感字段 %q: %s", banned, body)
		}
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || len(resp.Data) != 1 {
		t.Fatalf("data=%v err=%v", resp.Data, err)
	}
	row := resp.Data[0]
	if row["address"] != cAddr || row["keyType"] != "secp256k1" {
		t.Fatalf("row=%v", row)
	}
	fp, _ := row["publicKeyFingerprint"].(string)
	if len(fp) != 8 {
		t.Fatalf("指纹应 8 hex: %q", fp)
	}

	// 空集：提示文案
	req2 := httptest.NewRequest("GET", "/api/accounts-config", nil)
	t.Setenv(middleware.AccountsEnv, "")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if !strings.Contains(w2.Body.String(), "X-Milon-Accounts") {
		t.Fatalf("空集应带配置指引: %s", w2.Body.String())
	}
}
