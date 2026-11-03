package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// POST /api/tool/seed/accounts:计划派生账户总表(名称/角色/曲线/地址/公钥),
// 服务端永不返回私钥——总览页私钥列由前端本地 sha256(短语:角色:名字)得出。
func seedAccountsRequest(t *testing.T, h *SeedJobHandler, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/tool/seed/accounts", h.SeedAccounts)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/tool/seed/accounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var raw map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	return w, raw
}

func TestSeedAccountsEndpoint_Derived(t *testing.T) {
	h := NewSeedJobHandler(nil)
	w, raw := seedAccountsRequest(t, h, `{"scale":"full"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	// 不变量:响应任何位置不得出现 privateKey
	if strings.Contains(w.Body.String(), "privateKey") || strings.Contains(w.Body.String(), "PrivateKey") {
		t.Fatal("响应不得包含私钥字段")
	}

	data, _ := raw["data"].(map[string]any)
	if data == nil {
		t.Fatalf("data 缺失: %s", w.Body.String())
	}
	accts, _ := data["accounts"].([]any)
	if len(accts) == 0 {
		t.Fatal("accounts 为空")
	}

	// full 模板:admin + 12 用户(双曲线轮转) + kycorg + 2 多签 + 10 代币(8 普通 ST03.. + 2 合规) + 3 SFT
	type row struct {
		name, role, curve, addr string
	}
	var got []row
	seen := map[string]bool{}
	for _, a := range accts {
		m, _ := a.(map[string]any)
		name, _ := m["name"].(string)
		role, _ := m["role"].(string)
		curve, _ := m["curve"].(string)
		addr, _ := m["address"].(string)
		if name == "" || role == "" || addr == "" {
			t.Fatalf("账户字段缺失: %v", m)
		}
		if seen[name] {
			t.Fatalf("账户重名: %s", name)
		}
		seen[name] = true
		got = append(got, row{name, role, curve, addr})
	}
	for _, want := range []struct{ name, role, curve string }{
		{"admin", "admin", "secp256k1"},
		{"test01", "user", "secp256k1"},
		{"test02", "user", "ed25519"},
		{"kycorg", "issuer", "ed25519"},
		{"ms01", "multisig", "secp256k1"},
	} {
		found := false
		for _, g := range got {
			if g.name == want.name {
				found = true
				if g.role != want.role || g.curve != want.curve {
					t.Fatalf("%s role/curve = %s/%s, want %s/%s", want.name, g.role, g.curve, want.role, want.curve)
				}
			}
		}
		if !found {
			t.Fatalf("缺账户 %s, got %v", want.name, got)
		}
	}
	// 分角色计数(full: 10 代币账户(含 2 合规) + 3 SFT + 2 多签)
	var tokRole, sftRole, msRole int
	for _, g := range got {
		switch g.role {
		case "token":
			tokRole++
		case "sft":
			sftRole++
		case "multisig":
			msRole++
		}
	}
	if tokRole != 10 || sftRole != 3 || msRole != 2 {
		t.Fatalf("token/sft/multisig 账户数 = %d/%d/%d, want 10/3/2", tokRole, sftRole, msRole)
	}
}

// 地址必须与 SeedBuildAccounts(计划执行期用的同一派生)逐字段一致。
func TestSeedAccountsEndpoint_MatchesRunner(t *testing.T) {
	h := NewSeedJobHandler(nil)
	w, raw := seedAccountsRequest(t, h, `{"scale":"full"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	data, _ := raw["data"].(map[string]any)
	accts, _ := data["accounts"].([]any)

	cfg, err := seedResolveConfig(seedStartRequest{Scale: "full"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildSeedPlanV2(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want, err := SeedBuildAccounts(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range accts {
		m, _ := a.(map[string]any)
		name, _ := m["name"].(string)
		addr, _ := m["address"].(string)
		if wantAcct, ok := want[name]; !ok {
			t.Fatalf("端点多出账户 %s", name)
		} else if wantAcct.Address != addr {
			t.Fatalf("%s 地址不一致: endpoint=%s runner=%s", name, addr, wantAcct.Address)
		}
	}
}

// 同请求两次 → 地址一致(确定性);短语不同 → 地址不同。
func TestSeedAccountsEndpoint_Deterministic(t *testing.T) {
	h := NewSeedJobHandler(nil)
	_, raw1 := seedAccountsRequest(t, h, `{"preset":"minimal"}`)
	_, raw2 := seedAccountsRequest(t, h, `{"preset":"minimal"}`)
	addrOf := func(raw map[string]any) map[string]string {
		out := map[string]string{}
		data, _ := raw["data"].(map[string]any)
		accts, _ := data["accounts"].([]any)
		for _, a := range accts {
			m, _ := a.(map[string]any)
			name, _ := m["name"].(string)
			addr, _ := m["address"].(string)
			out[name] = addr
		}
		return out
	}
	a1, a2 := addrOf(raw1), addrOf(raw2)
	if len(a1) == 0 || len(a1) != len(a2) {
		t.Fatalf("账户数不一致: %d vs %d", len(a1), len(a2))
	}
	for k, v := range a1 {
		if a2[k] != v {
			t.Fatalf("同配置地址漂移: %s %s vs %s", k, v, a2[k])
		}
	}

	_, raw3 := seedAccountsRequest(t, h, `{"preset":"minimal","seedPhrase":"another-phrase"}`)
	a3 := addrOf(raw3)
	if a3["admin"] == a1["admin"] {
		t.Fatal("不同短语应派生不同地址")
	}
}

// 非法配置 → 400 + 错误列表。
func TestSeedAccountsEndpoint_InvalidConfig(t *testing.T) {
	h := NewSeedJobHandler(nil)
	w, raw := seedAccountsRequest(t, h, `{"config":{"seedPhrase":"p","accounts":{"count":0}}}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if success, _ := raw["success"].(bool); success {
		t.Fatal("success 应为 false")
	}
}

// 坏 JSON → 400。
func TestSeedAccountsEndpoint_BadBody(t *testing.T) {
	h := NewSeedJobHandler(nil)
	w, _ := seedAccountsRequest(t, h, `{not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
}
