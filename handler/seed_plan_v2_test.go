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
	// 曲线:默认全 fndsa512(抗量子,2026-11-24 起)——单曲线即全轮转同值
	wantCurves := []string{"fndsa512", "fndsa512", "fndsa512", "fndsa512", "fndsa512", "fndsa512", "fndsa512", "fndsa512"}
	for i, w := range wantCurves {
		if p.Users[i].Curve != w {
			t.Errorf("users[%d].Curve=%s want %s", i, p.Users[i].Curve, w)
		}
	}
	if p.Admin.Curve != "fndsa512" {
		t.Errorf("admin.Curve=%s want fndsa512", p.Admin.Curve)
	}
	if len(p.Issuers) > 0 && p.Issuers[0].Curve != "fndsa512" {
		t.Errorf("issuer.Curve=%s want fndsa512", p.Issuers[0].Curve)
	}
	if len(p.Multisig) > 0 && p.Multisig[0].Curve != "fndsa512" {
		t.Errorf("multisig.Curve=%s want fndsa512", p.Multisig[0].Curve)
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

// TestBuildSeedPlanV2_TransferOpsPerToken 锁定多持有人场景的两笔转移:
// 持有人 = 分发对象(op.By,非 admin),各笔动不同 token(Index 0/1)。
// 回归:曾用 admin 作 owner + 恒取 tokenIds[0],链端报
// insufficient token amount(token: 1, available: 0, required: 1)。
func TestBuildSeedPlanV2_TransferOpsPerToken(t *testing.T) {
	p, err := BuildSeedPlanV2(SeedConfigDefaults())
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, s := range p.Sfts {
		for _, sc := range s.Scenarios {
			if sc.Key != "transferable" {
				continue
			}
			seen++
			if len(sc.Dists) == 0 {
				t.Fatalf("%s/%s 无分发", s.Key, sc.Key)
			}
			hold := sc.Dists[0].To
			if len(sc.PostOps) != 2 {
				t.Fatalf("%s/%s 转移笔数 %d, want 2", s.Key, sc.Key, len(sc.PostOps))
			}
			for i, op := range sc.PostOps {
				if op.Op != "transfer" {
					t.Fatalf("%s/%s post[%d].Op=%s", s.Key, sc.Key, i, op.Op)
				}
				if op.By != hold {
					t.Fatalf("%s/%s post[%d].By=%s, want 持有人 %s(admin 只是铸造签名方)",
						s.Key, sc.Key, i, op.By, hold)
				}
				if op.Index != i {
					t.Fatalf("%s/%s post[%d].Index=%d, want %d(每笔转不同 token)",
						s.Key, sc.Key, i, op.Index, i)
				}
				if op.Index >= len(sc.Dists) {
					t.Fatalf("%s/%s post[%d] 下标越界: %d ≥ %d", s.Key, sc.Key, i, op.Index, len(sc.Dists))
				}
			}
		}
	}
	if seen == 0 {
		t.Fatal("默认配置应产出 transferable 场景")
	}
}

// TestBuildSeedPlanV2_TransferTargetsAvoidSelf 账号不足 3 个时转移/拦截后置
// 不得把目标设成持有人本人(链端 SelfTransfer 1287):按可用人数减笔或跳过。
func TestBuildSeedPlanV2_TransferTargetsAvoidSelf(t *testing.T) {
	cases := []struct{ count, wantTransfers int }{{1, 0}, {2, 1}, {3, 2}, {8, 2}}
	for _, tc := range cases {
		cfg := SeedConfigDefaults()
		cfg.Tokens.Count = 1
		cfg.Accounts.Count = tc.count
		cfg.Multisig.Count = 0
		cfg.Tokens.ComplianceCount = 0
		cfg.Accounts.VcCert = false
		cfg.SFT.Scenarios = []string{"transferable", "reject_transfer"}
		p, err := BuildSeedPlanV2(cfg)
		if err != nil {
			t.Fatalf("count=%d: %v", tc.count, err)
		}
		for _, s := range p.Sfts {
			for _, sc := range s.Scenarios {
				if len(sc.Dists) == 0 {
					t.Fatalf("count=%d %s/%s 无分发", tc.count, s.Key, sc.Key)
				}
				hold := sc.Dists[0].To
				switch sc.Key {
				case "transferable":
					if len(sc.PostOps) != tc.wantTransfers {
						t.Fatalf("count=%d 转移笔数 %d, want %d", tc.count, len(sc.PostOps), tc.wantTransfers)
					}
					for i, op := range sc.PostOps {
						if op.To == hold {
							t.Fatalf("count=%d post[%d] 目标=持有人 %s(SelfTransfer 1287)", tc.count, i, hold)
						}
						if op.Index != i || op.Index >= len(sc.Dists) {
							t.Fatalf("count=%d post[%d].Index=%d 越界或重号", tc.count, i, op.Index)
						}
					}
				case "reject_transfer":
					if tc.count == 1 {
						if len(sc.PostOps) != 0 {
							t.Fatalf("单账号不应产出拦截实测(目标只能是本人), got %+v", sc.PostOps)
						}
						continue
					}
					if len(sc.PostOps) != 1 || sc.PostOps[0].To == hold {
						t.Fatalf("count=%d 拦截目标须为他人, got %+v", tc.count, sc.PostOps)
					}
				}
			}
		}
	}
}

// TestBuildSeedPlanV2_AllAccountsPostQuantum 三个模板的整条计划链(admin/users/
// issuers/multisig + 运行期派生 token/compliance/sft 资源账户)全部 fndsa512,
// 不出现任何经典曲线;派生账户须带公钥与地址(fndsa512 公钥不可反推)。
func TestBuildSeedPlanV2_AllAccountsPostQuantum(t *testing.T) {
	for _, preset := range []string{"minimal", "standard", "full"} {
		cfg, err := SeedConfigForPreset(preset)
		if err != nil {
			t.Fatalf("%s: %v", preset, err)
		}
		p, err := BuildSeedPlanV2(cfg)
		if err != nil {
			t.Fatalf("%s: %v", preset, err)
		}
		check := func(where, curve string) {
			if curve != "fndsa512" {
				t.Fatalf("%s/%s 曲线=%q, want fndsa512", preset, where, curve)
			}
		}
		check("admin", p.Admin.Curve)
		for _, u := range p.Users {
			check(u.Name, u.Curve)
		}
		for _, i := range p.Issuers {
			check(i.Name, i.Curve)
		}
		for _, ms := range p.Multisig {
			check(ms.Name, ms.Curve)
		}
		accts, err := SeedBuildAccounts(p)
		if err != nil {
			t.Fatalf("%s: 派生账户失败: %v", preset, err)
		}
		if len(accts) == 0 {
			t.Fatalf("%s: 无派生账户", preset)
		}
		for name, a := range accts {
			check(name, a.Curve)
			if a.PublicKey == "" || a.Address == "" {
				t.Fatalf("%s/%s: fndsa512 须带公钥与地址", preset, name)
			}
		}
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
