package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/middleware"
)

// transfer_mil：无私钥+fromAddress 命中 → 越过私钥校验（fake RPC 报链错而非参数错）
func TestTransferMilFromAddressConfigured(t *testing.T) {
	r := newAccountsContractRouter(t) // 复用 Task 4 脚手架（fake RPC + accounts 中间件）
	body := `{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1,"fromAddress":"` + cAddr + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/tool/transfer-mil", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.AccountsHeader, accountsHeader(`[{"address":"`+cAddr+`","privateKey":"`+cPrivHex+`"}]`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "privateKey") {
		t.Fatalf("仍被 privateKey 校验拦截: %s", w.Body.String())
	}
	// 未命中：400 且文案含提示
	req2 := httptest.NewRequest(http.MethodPost, "/api/tool/transfer-mil", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	t.Setenv(middleware.AccountsEnv, "")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest || !strings.Contains(w2.Body.String(), "account_config_list") {
		t.Fatalf("want 400+hint got %d: %s", w2.Code, w2.Body.String())
	}
}

// faucet_claim：无私钥+address 命中 → 越过私钥解析
func TestFaucetClaimConfigured(t *testing.T) {
	r := newAccountsContractRouter(t)
	body := `{"address":"` + cAddr + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/faucet/claim", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.AccountsHeader, accountsHeader(`[{"address":"`+cAddr+`","privateKey":"`+cPrivHex+`"}]`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "privateKey") {
		t.Fatalf("仍被 privateKey 校验拦截: %s", w.Body.String())
	}
	// 未命中：400 且文案含提示（binding 放开后由显式校验拦截）
	req2 := httptest.NewRequest(http.MethodPost, "/api/faucet/claim", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	t.Setenv(middleware.AccountsEnv, "")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest || !strings.Contains(w2.Body.String(), "account_config_list") {
		t.Fatalf("want 400+hint got %d: %s", w2.Code, w2.Body.String())
	}
}
