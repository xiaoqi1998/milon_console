// handler/seed_plan_v2_test.go
package handler

import (
	"encoding/json"
	"regexp"
	"testing"
)

func TestBuildSeedPlanV2_Deterministic(t *testing.T) {
	cfg := SeedConfigDefaults()
	p1, err := BuildSeedPlanV2(cfg)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := BuildSeedPlanV2(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(p1)
	b, _ := json.Marshal(p2)
	if string(a) != string(b) {
		t.Fatal("同输入两次构建须逐字节相同")
	}
}

func TestBuildSeedPlanV2_InvalidConfig(t *testing.T) {
	cfg := SeedConfigDefaults()
	cfg.Accounts.Count = 0
	if _, err := BuildSeedPlanV2(cfg); err == nil {
		t.Fatal("非法配置应报错")
	}
}

func TestBuildSeedPlanV2_Matrix(t *testing.T) {
	cfg := SeedConfigDefaults() // 8 账号/3 代币(1 合规)/1 多签 3-of-5/2 SFT×2 slot×3 token
	p, err := BuildSeedPlanV2(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Users) != 8 {
		t.Fatalf("用户数 %d", len(p.Users))
	}
	// 曲线轮转:8 人双曲线(secp/ed 交替)——fndsa512 播种器确定性派生不支持
	wantCurves := []string{"secp256k1", "ed25519", "secp256k1", "ed25519", "secp256k1", "ed25519", "secp256k1", "ed25519"}
	for i, w := range wantCurves {
		if p.Users[i].Curve != w {
			t.Errorf("users[%d].Curve=%s want %s", i, p.Users[i].Curve, w)
		}
	}
	if len(p.Tokens) != 2 { // 3 代币 − 1 合规 = 2 普通代币
		t.Fatalf("普通代币数 %d", len(p.Tokens))
	}
	if len(p.Compliance) != 1 {
		t.Fatalf("合规代币数 %d", len(p.Compliance))
	}
	if p.Compliance[0].IssuerName != "kycorg" || p.Compliance[0].Schema != "SeedKyc1" {
		t.Error("合规代币须挂 kycorg/SeedKyc1")
	}
	if len(p.Multisig) != 1 || p.Multisig[0].Name != "ms01" || p.Multisig[0].Threshold != 3 || len(p.Multisig[0].SignerNames) != 5 {
		t.Fatalf("多签规格不符: %+v", p.Multisig)
	}
	if p.Multisig[0].SignerNames[0] != "test01" || p.Multisig[0].SignerNames[4] != "test05" {
		t.Errorf("签名者应轮取 test01..test05, got %v", p.Multisig[0].SignerNames)
	}
	slots, tokens := 0, 0
	for _, s := range p.Sfts {
		slots += len(s.Scenarios)
		for _, sc := range s.Scenarios {
			tokens += len(sc.Dists)
		}
	}
	if slots != 4 || tokens != 12 { // 2 SFT × 2 slot;每 slot 3 token
		t.Fatalf("slot=%d token=%d", slots, tokens)
	}
	// 合规配置开启 ⇒ VC 证书链存在
	if len(p.VcIssuers) == 0 || p.VcIssuers[0].Schema != "SeedKyc1" {
		t.Error("vcCert 应产出 kycorg/SeedKyc1")
	}
	if p.Config == nil || p.ConfigHash == "" {
		t.Fatal("计划须携带 Config 与 ConfigHash")
	}
}

func TestBuildSeedPlanV2_KnobIsolation(t *testing.T) {
	base, _ := BuildSeedPlanV2(SeedConfigDefaults())
	changed := SeedConfigDefaults()
	changed.SFT.Count = 1
	alt, _ := BuildSeedPlanV2(changed)
	if len(alt.Sfts) != 1 || len(base.Sfts) != 2 {
		t.Fatal("改 sft.count 只应影响 SFT 段")
	}
	if len(alt.Users) != len(base.Users) {
		t.Fatal("账号段不应受 SFT 旋钮影响")
	}
}

func TestBuildSeedPlanV2_MinimalNoIssuer(t *testing.T) {
	cfg, _ := SeedConfigForPreset("minimal")
	p, err := BuildSeedPlanV2(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Issuers) != 0 || len(p.VcIssuers) != 0 || len(p.Compliance) != 0 {
		t.Error("minimal 无 vcCert/compliance ⇒ 不应产出 issuer/VC/合规")
	}
}

func TestBuildSeedPlanLegacyWrapper(t *testing.T) {
	p, err := BuildSeedPlan("full", SeedDefaultPhrase)
	if err != nil {
		t.Fatal(err)
	}
	if p.Scale != "full" || p.Config == nil {
		t.Fatalf("旧包装应产出 full 预设计划: %+v", p.Scale)
	}
	full, _ := SeedConfigForPreset("full")
	p2, _ := BuildSeedPlanV2(full)
	if p.ConfigHash != p2.ConfigHash {
		t.Fatal("旧 scale=full 与新 preset=full 须同一配置同一哈希")
	}
	if _, err := BuildSeedPlan("small", SeedDefaultPhrase); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildSeedPlan("medium", SeedDefaultPhrase); err == nil {
		t.Fatal("未知 scale 应报错")
	}
}

func TestSeedEstimateTxs_Positive(t *testing.T) {
	p, _ := BuildSeedPlanV2(SeedConfigDefaults())
	if SeedEstimateTxs(p) <= 0 {
		t.Fatal("预估交易数须为正")
	}
}

// TestSeedImageURLs 锁定真实图源(20261020 裁定:弃随机占位,全部真实内容):
// 头像=randomuser 真人照片;代币图标=真实货币 logo;URI/封面=picsum 真实摄影图。
func TestSeedImageURLs(t *testing.T) {
	ava := seedAvatarURL("test01")
	if !regexp.MustCompile(`^https://randomuser\.me/api/portraits/(men|women)/\d+\.jpg$`).MatchString(ava) {
		t.Fatalf("头像须为 randomuser 真人照片, got %s", ava)
	}
	if seedAvatarURL("test01") != ava {
		t.Fatal("头像 URL 须确定性")
	}
	icon := seedTokenIconURL("ST01")
	if !regexp.MustCompile(`^https://raw\.githubusercontent\.com/spothq/cryptocurrency-icons/master/128/color/[a-z0-9]+\.png$`).MatchString(icon) {
		t.Fatalf("代币图标须为真实货币 logo, got %s", icon)
	}
	if got := seedTokenIconURL("SCUSD"); !regexp.MustCompile(`/usdt\.png$`).MatchString(got) {
		t.Fatalf("合规稳定币 SCUSD 须钉 usdt logo, got %s", got)
	}
	if seedTokenIconURL("ST01") != seedTokenIconURL("ST01") {
		t.Fatal("图标 URL 须确定性")
	}
	if !regexp.MustCompile(`^https://picsum\.photos/`).MatchString(seedTokenURI("ST01")) {
		t.Fatalf("URI 大图维持 picsum 真实摄影图, got %s", seedTokenURI("ST01"))
	}
}
