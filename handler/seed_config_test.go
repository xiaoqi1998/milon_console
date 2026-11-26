// handler/seed_config_test.go
package handler

import "testing"

func validCfg() SeedConfig {
	return SeedConfigDefaults()
}

func TestValidateSeedConfig_AcceptsDefaults(t *testing.T) {
	if errs := ValidateSeedConfig(validCfg()); len(errs) != 0 {
		t.Fatalf("默认配置应合法, got %v", errs)
	}
}

// 默认曲线(抗量子):播种器只产出 fndsa512;2026-11-26 起经典曲线在校验层直接拒绝。
func TestSeedConfigDefaults_Fndsa512Curves(t *testing.T) {
	c := SeedConfigDefaults()
	if len(c.Accounts.Curves) != 1 || c.Accounts.Curves[0] != "fndsa512" {
		t.Fatalf("默认曲线应为 ['fndsa512'], got %v", c.Accounts.Curves)
	}
	if errs := ValidateSeedConfig(c); len(errs) != 0 {
		t.Fatalf("fndsa512 默认配置应合法, got %v", errs)
	}
	// 经典曲线一律拒绝:链端 owner/signer 已 PQ 化,生成出来即废数据
	for _, classic := range []string{"secp256k1", "ed25519"} {
		bad := SeedConfigDefaults()
		bad.Accounts.Curves = []string{classic}
		if errs := ValidateSeedConfig(bad); len(errs) == 0 {
			t.Fatalf("%s 应被拒绝", classic)
		}
	}
	mix := SeedConfigDefaults()
	mix.Accounts.Curves = []string{"fndsa512", "secp256k1"}
	if errs := ValidateSeedConfig(mix); len(errs) == 0 {
		t.Fatal("混入经典曲线应被拒绝")
	}
}

func TestValidateSeedConfig_FieldPathErrors(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*SeedConfig)
		want string // 错误信息须含的字段路径
	}{
		{"账号数越界", func(c *SeedConfig) { c.Accounts.Count = 0 }, "accounts.count"},
		{"曲线为空", func(c *SeedConfig) { c.Accounts.Curves = nil }, "accounts.curves"},
		{"非法曲线", func(c *SeedConfig) { c.Accounts.Curves = []string{"rsa"} }, "accounts.curves[rsa]"},
		{"经典曲线被拒", func(c *SeedConfig) { c.Accounts.Curves = []string{"secp256k1"} }, "accounts.curves[secp256k1]"},
		{"精度越界", func(c *SeedConfig) { c.Tokens.Decimals = []uint8{10} }, "tokens.decimals[10]"},
		{"合规数超代币数", func(c *SeedConfig) { c.Tokens.ComplianceCount = 9 }, "tokens.complianceCount"},
		{"合规但无证书", func(c *SeedConfig) { c.Tokens.ComplianceCount = 1; c.Accounts.VcCert = false }, "tokens.complianceCount"},
		{"合规但账号不足", func(c *SeedConfig) { c.Tokens.ComplianceCount = 1; c.Accounts.Count = 2; c.Tokens.Count = 1; c.Tokens.Decimals = []uint8{6} }, "tokens.complianceCount"},
		{"多签超账号", func(c *SeedConfig) { c.Multisig.Count = 1; c.Multisig.Signers = 99 }, "multisig.signers"},
		{"阈值超签者", func(c *SeedConfig) { c.Multisig.Count = 1; c.Multisig.Signers = 3; c.Multisig.Threshold = 5 }, "multisig.threshold"},
		{"DID 无 gas", func(c *SeedConfig) { c.Accounts.Faucet = false; c.Accounts.BalanceTiers = nil; c.Accounts.DID = true; c.Accounts.VcCert = false }, "accounts.did"},
		{"非法余额档", func(c *SeedConfig) { c.Accounts.BalanceTiers = []string{"million"} }, "accounts.balanceTiers[million]"},
		{"非法场景", func(c *SeedConfig) { c.SFT.Scenarios = []string{"stake"} }, "sft.scenarios[stake]"},
		{"sft 有 slot 无场景", func(c *SeedConfig) { c.SFT.Scenarios = nil }, "sft.scenarios"},
	}
	for _, tc := range cases {
		c := validCfg()
		tc.mut(&c)
		errs := ValidateSeedConfig(c)
		if len(errs) == 0 {
			t.Errorf("%s: 应报错", tc.name)
			continue
		}
		found := false
		for _, e := range errs {
			if contains2(e, tc.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: 错误 %v 未含字段路径 %q", tc.name, errs, tc.want)
		}
	}
}

// one 档(1 MIL):2026-11-25 水龙头降为单次 1 MIL 后唯一零充值可达成档。
func TestValidateSeedConfig_OneTier(t *testing.T) {
	c := validCfg()
	c.Accounts.BalanceTiers = []string{"one", "hundred"}
	if errs := ValidateSeedConfig(c); len(errs) != 0 {
		t.Fatalf("one 档应合法, got %v", errs)
	}
}

func TestNormalizeSeedConfig_VcCertImpliesDid(t *testing.T) {
	c := validCfg()
	c.Accounts.DID = false
	c.Accounts.VcCert = true
	out := NormalizeSeedConfig(c)
	if !out.Accounts.DID {
		t.Fatal("vcCert=true 应自动纠正 did=true")
	}
}

func TestSeedConfigHash_DeterministicAndSensitive(t *testing.T) {
	a := validCfg()
	b := validCfg()
	if SeedConfigHash(a) != SeedConfigHash(b) {
		t.Fatal("同配置哈希须一致")
	}
	if len(SeedConfigHash(a)) != 16 {
		t.Fatalf("哈希须 16 hex, got %d", len(SeedConfigHash(a)))
	}
	a.Tokens.Count++
	if SeedConfigHash(a) == SeedConfigHash(b) {
		t.Fatal("改任一旋钮哈希须变")
	}
}

func contains2(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
