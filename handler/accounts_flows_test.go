package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/middleware"
)

// TestFlowEntriesConfiguredAccount 验证 did×10 / sft-flow / vc-flow 三个流程入口
// 接入「地址即引用」回填：请求只给地址不给私钥时，命中配置账户即可越过
// 私钥必填校验（错误应来自链端/后续步骤，而非参数校验）；未命中则保持
// 400 且文案附带配置账户 hint。
func TestFlowEntriesConfiguredAccount(t *testing.T) {
	r := newAccountsContractRouter(t)
	hdr := accountsHeader(`[{"address":"` + cAddr + `","privateKey":"` + cPrivHex + `"}]`)
	// 脚手架补挂（路径与 main.go 一致）：
	// api.POST("/tool/did/create", didH.Create)（did 十端点路由前缀 /api/tool/did/*）
	// api.POST("/tool/sft-flow", sftH.SftFlow)
	// api.POST("/tool/vc-flow", vcH.VcFlow)

	// post 统一发请求；withHeader 控制是否携带配置账户头。
	post := func(t *testing.T, path, body string, withHeader bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if withHeader {
			req.Header.Set(middleware.AccountsHeader, hdr)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("did 缺私钥+地址命中→越过 privateKey is required", func(t *testing.T) {
		body := `{"address":"` + cAddr + `","alias":"tester"}` // privateKey 缺省
		w := post(t, "/api/tool/did/create", body, true)
		if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "privateKey is required") {
			t.Fatalf("仍被私钥校验拦截: %s", w.Body.String())
		}
	})
	t.Run("did 未命中→400 文案含 hint", func(t *testing.T) {
		t.Setenv(middleware.AccountsEnv, "")
		body := `{"address":"` + cAddr + `"}`
		w := post(t, "/api/tool/did/create", body, false)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "account_config_list") {
			t.Fatalf("want 400+hint got %d: %s", w.Code, w.Body.String())
		}
	})

	// did 其余九个细粒度端点共用 bindMutate 绑定路径，取 set-alias 作代表
	// 验证该路径的回填接线（命中→越过 bindMutate 的私钥必填）。
	t.Run("did set-alias 缺私钥+地址命中→越过 privateKey is required", func(t *testing.T) {
		body := `{"address":"` + cAddr + `","alias":"tester"}` // privateKey 缺省
		w := post(t, "/api/tool/did/set-alias", body, true)
		if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "privateKey is required") {
			t.Fatalf("仍被私钥校验拦截: %s", w.Body.String())
		}
	})
	t.Run("did set-alias 未命中→400 文案含 hint", func(t *testing.T) {
		t.Setenv(middleware.AccountsEnv, "")
		body := `{"address":"` + cAddr + `","alias":"tester"}`
		w := post(t, "/api/tool/did/set-alias", body, false)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "account_config_list") {
			t.Fatalf("want 400+hint got %d: %s", w.Code, w.Body.String())
		}
	})

	// sft-flow：owner 三锚点回填；body 给 ownerAddress+sft.address（复用已有 SFT）
	// 以越过元数据必填，让命中场景的错误来自链端而非参数校验。
	t.Run("sft-flow 缺 owner 私钥+地址命中→越过 ownerPrivateKey is required", func(t *testing.T) {
		body := `{"ownerAddress":"` + cAddr + `","sft":{"address":"` + cAddr + `"}}` // ownerPrivateKey 缺省
		w := post(t, "/api/tool/sft-flow", body, true)
		if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "ownerPrivateKey is required") {
			t.Fatalf("仍被私钥校验拦截: %s", w.Body.String())
		}
	})
	t.Run("sft-flow 未命中→400 文案含 hint", func(t *testing.T) {
		t.Setenv(middleware.AccountsEnv, "")
		body := `{"ownerAddress":"` + cAddr + `"}`
		w := post(t, "/api/tool/sft-flow", body, false)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "account_config_list") {
			t.Fatalf("want 400+hint got %d: %s", w.Code, w.Body.String())
		}
	})

	// vc-flow：issuer+user 双角色回填；双方地址各自命中配置账户（此处同一账户）
	// 时均应回填，越过 issuerPrivateKey/userPrivateKey 必填。注意 issuer 只给
	// userAddress 时不回填——「地址即引用」以地址为锚，无地址锚点的角色无从命中。
	t.Run("vc-flow 缺双方私钥+双地址命中→越过 *PrivateKey is required", func(t *testing.T) {
		body := `{"issuerAddress":"` + cAddr + `","userAddress":"` + cAddr + `"}` // 双方 privateKey 均缺省
		w := post(t, "/api/tool/vc-flow", body, true)
		if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "PrivateKey is required") {
			t.Fatalf("仍被私钥校验拦截: %s", w.Body.String())
		}
	})
	t.Run("vc-flow 未命中→400 文案含 hint", func(t *testing.T) {
		t.Setenv(middleware.AccountsEnv, "")
		body := `{"userAddress":"` + cAddr + `"}`
		w := post(t, "/api/tool/vc-flow", body, false)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "account_config_list") {
			t.Fatalf("want 400+hint got %d: %s", w.Code, w.Body.String())
		}
	})
}
