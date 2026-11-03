package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// POST /api/tool/vc-issue:给派生账户发证书(复用播种器 vc 模块幂等函数链)。
// 网络 Executes 留给 devNet 集成实测;此处覆盖:派生解析(纯函数)+ 校验(400)+ 不变量(响应无私钥)。

func vcIssueDo(t *testing.T, h *SeedJobHandler, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/tool/vc-issue", h.VcIssue)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/tool/vc-issue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var raw map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	return w, raw
}

func validVcIssueBody() string {
	return `{"config":{"seedPhrase":"milon-devnet-seed-v1","accounts":{"count":3,"curves":["secp256k1"],"faucet":true,"balanceTiers":["hundred"],"did":true,"vcCert":true},"multisig":{"count":0,"signers":5,"threshold":3,"faucet":false,"did":false},"tokens":{"count":1,"decimals":[6],"complianceCount":0,"freezeDemo":false},"sft":{"count":0,"slotsPerSft":1,"tokensPerSlot":3,"scenarios":["transferable"],"postOps":false}},"issuerName":"kycorg","subjectName":"test01","schemaPrefix":"SeedKyc","count":1}`
}

// resolveVcIssueAccounts(纯函数,离线):config+名字 → 派生账户对。
func TestVcIssueResolve(t *testing.T) {
	var cfgSeed struct {
		Config json.RawMessage `json:"config"`
	}
	_ = json.Unmarshal([]byte(validVcIssueBody()), &cfgSeed)
	var cfg SeedConfig
	if err := json.Unmarshal(cfgSeed.Config, &cfg); err != nil {
		t.Fatal(err)
	}
	issuer, subject, err := resolveVcIssueAccounts(cfg, "kycorg", "test01")
	if err != nil {
		t.Fatalf("合法输入应解析成功: %v", err)
	}
	if issuer.Role != "issuer" || issuer.Name != "kycorg" || issuer.Address == "" {
		t.Fatalf("issuer 解析异常: %+v", issuer)
	}
	if subject.Role != "user" || subject.Name != "test01" || subject.Address == "" {
		t.Fatalf("subject 解析异常: %+v", subject)
	}
	// 与 SeedDeriveKey 同口径:私钥 = sha256(短语:角色:名)
	if issuer.PrivateKey != SeedDeriveKey(cfg.SeedPhrase, "issuer", "kycorg") {
		t.Fatal("issuer 私钥派生口径漂移")
	}
	if subject.PrivateKey != SeedDeriveKey(cfg.SeedPhrase, "user", "test01") {
		t.Fatal("subject 私钥派生口径漂移")
	}
	// 未知账户
	if _, _, err := resolveVcIssueAccounts(cfg, "kycorg", "nope"); err == nil {
		t.Fatal("未知 subject 应报错")
	}
	if _, _, err := resolveVcIssueAccounts(cfg, "admin", "test01"); err == nil {
		t.Fatal("非 issuer 角色不能签发(admin 不是签发方)")
	}
	// vcCert=false 的配置没有签发方
	cfg.Accounts.VcCert = false
	if _, _, err := resolveVcIssueAccounts(cfg, "kycorg", "test01"); err == nil {
		t.Fatal("无签发方配置应报错")
	}
}

func TestVcIssueEndpoint_Validation(t *testing.T) {
	h := NewSeedJobHandler(nil)
	cases := []struct {
		name, body string
	}{
		{"坏 JSON", `{not-json`},
		{"缺 schemaPrefix", `{"config":{"seedPhrase":"p","accounts":{"count":3,"curves":["secp256k1"],"faucet":true,"balanceTiers":["hundred"]}},"issuerName":"kycorg","subjectName":"test01"}`},
		{"count 越界", `{"config":{"seedPhrase":"p","accounts":{"count":3,"curves":["secp256k1"],"faucet":true,"balanceTiers":["hundred"]}},"issuerName":"kycorg","subjectName":"test01","schemaPrefix":"SeedKyc","count":99}`},
		{"未知 subject", `{"config":{"seedPhrase":"p","accounts":{"count":3,"curves":["secp256k1"],"faucet":true,"balanceTiers":["hundred"]}},"issuerName":"kycorg","subjectName":"nope","schemaPrefix":"SeedKyc","count":1}`},
	}
	for _, c := range cases {
		w, raw := vcIssueDo(t, h, c.body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400 (body=%s)", c.name, w.Code, w.Body.String())
		}
		if ok, _ := raw["success"].(bool); ok {
			t.Fatalf("%s: success 应为 false", c.name)
		}
	}
}

// 合法请求:校验通过后进入网络段(测试环境无客户端,预期 500 no current network)——
// 关键不变量:任何分支响应都不得含私钥。
func TestVcIssueEndpoint_NoPrivateKeyInResponse(t *testing.T) {
	h := NewSeedJobHandler(nil)
	w, _ := vcIssueDo(t, h, validVcIssueBody())
	if strings.Contains(w.Body.String(), "privateKey") || strings.Contains(w.Body.String(), "\"sk\"") {
		t.Fatalf("响应不得包含私钥: %s", w.Body.String())
	}
}
