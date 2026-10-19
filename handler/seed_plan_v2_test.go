// handler/seed_plan_v2_test.go
package handler

import (
	"encoding/json"
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
	// 曲线轮转:8 人三曲线 → secp/ed/fndsa/secp/ed/fndsa/secp/ed
	wantCurves := []string{"secp256k1", "ed25519", "fndsa512", "secp256k1", "ed25519", "fndsa512", "secp256k1", "ed25519"}
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
