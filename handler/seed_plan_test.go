package handler

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// 与 Python 版 tools/seeder/seedlib/plan.py 逐项对拍。

func TestSeedModuleOrder(t *testing.T) {
	want := []string{"accounts", "tokens", "sft", "vc", "compliance", "txs"}
	if !reflect.DeepEqual(SeedModuleOrder, want) {
		t.Fatalf("SeedModuleOrder = %v, want %v", SeedModuleOrder, want)
	}
}

func TestSeedFullPlanUsers(t *testing.T) {
	p := BuildSeedPlan("full", "s1")
	if len(p.Users) != 12 {
		t.Fatalf("users = %d, want 12", len(p.Users))
	}
	for i, u := range p.Users {
		want := [2]string{seedTestName(i + 1), "seed-" + seedTestName(i + 1)}
		if u.Name != want[0] || u.Alias != want[1] {
			t.Fatalf("user[%d] = %s/%s, want %s/%s", i, u.Name, u.Alias, want[0], want[1])
		}
		if u.AliasSuffix != uint32(1001+i) {
			t.Fatalf("user[%d] suffix = %d, want %d", i, u.AliasSuffix, 1001+i)
		}
		if u.Role != "user" || u.Curve != "secp256k1" {
			t.Fatalf("user[%d] role/curve = %s/%s", i, u.Role, u.Curve)
		}
	}
}

func TestSeedFullPlanIssuers(t *testing.T) {
	p := BuildSeedPlan("full", "s1")
	if len(p.Issuers) != 2 {
		t.Fatalf("issuers = %d, want 2", len(p.Issuers))
	}
	kyc, game := p.Issuers[0], p.Issuers[1]
	if kyc.Name != "kycorg" || game.Name != "gameorg" {
		t.Fatalf("issuer names = %s,%s", kyc.Name, game.Name)
	}
	if kyc.Curve != "ed25519" || game.Curve != "ed25519" {
		t.Fatal("issuers must be ed25519 (VC 签名限制)")
	}
	if kyc.SubjectType != "Organization" || game.SubjectType != "Organization" {
		t.Fatal("issuers must be Organization")
	}
	if kyc.AliasSuffix != 2011 || game.AliasSuffix != 2012 {
		t.Fatalf("suffixes = %d,%d, want 2011,2012", kyc.AliasSuffix, game.AliasSuffix)
	}
	if len(p.VcIssuers) != 2 {
		t.Fatalf("vc issuers = %d, want 2", len(p.VcIssuers))
	}
	vk, vg := p.VcIssuers[0], p.VcIssuers[1]
	if vk.SchemaPrefix != "SeedKyc" || vk.SchemaCount != 2 || vk.Schema != "SeedKyc1" {
		t.Fatalf("kyc vc spec = %+v", vk)
	}
	wantCerts := append([]string{"admin"}, seedTestRange(1, 8)...)
	if !reflect.DeepEqual(vk.Certifies, wantCerts) {
		t.Fatalf("kyc certifies = %v, want %v", vk.Certifies, wantCerts)
	}
	if vg.Schema != "SeedPlayer1" || vg.SchemaCount != 1 {
		t.Fatalf("game vc spec = %+v", vg)
	}
	if !reflect.DeepEqual(vg.Certifies, seedTestRange(3, 10)) {
		t.Fatalf("game certifies = %v", vg.Certifies)
	}
}

func TestSeedFullPlanTokens(t *testing.T) {
	p := BuildSeedPlan("full", "s1")
	if len(p.Tokens) != 10 {
		t.Fatalf("tokens = %d, want 10", len(p.Tokens))
	}
	seen := map[uint8]bool{}
	for _, tok := range p.Tokens {
		if tok.Decimals > 9 {
			t.Fatalf("%s decimals %d > 9 (链端上限)", tok.Symbol, tok.Decimals)
		}
		seen[tok.Decimals] = true
		if _, ok := tok.MintAmounts["test01"]; !ok {
			t.Fatalf("%s 缺 test01 铸量", tok.Symbol)
		}
		var total uint64
		for _, v := range tok.MintAmounts {
			total += v
		}
		if total >= math.MaxUint64 {
			t.Fatalf("%s 总量溢出 u64", tok.Symbol)
		}
	}
	for d := uint8(0); d <= 9; d++ {
		if !seen[d] {
			t.Fatalf("缺 %d 精度代币", d)
		}
	}
	demos := 0
	for _, tok := range p.Tokens {
		if tok.FreezeDemoUser != "" {
			demos++
			if tok.FreezeDemoUser != "test05" {
				t.Fatalf("freeze demo user = %s, want test05", tok.FreezeDemoUser)
			}
		}
	}
	if demos != 1 {
		t.Fatalf("freeze demo = %d, want 1", demos)
	}
}

func TestSeedFullPlanSft(t *testing.T) {
	p := BuildSeedPlan("full", "s1")
	if len(p.Sfts) != 2 {
		t.Fatalf("sfts = %d, want 2", len(p.Sfts))
	}
	byKey := map[string]SeedSftSpec{}
	for _, s := range p.Sfts {
		byKey[s.Key] = s
	}
	genKeys := map[string]bool{}
	for _, sc := range byKey["genesis"].Scenarios {
		genKeys[sc.Key] = true
	}
	for _, k := range []string{"no_transfer", "metadata_full", "token_override", "burn_shell", "multi_holder", "empty"} {
		if !genKeys[k] {
			t.Fatalf("genesis 缺场景 %s", k)
		}
	}
	gameKeys := map[string]bool{}
	for _, sc := range byKey["gameitems"].Scenarios {
		gameKeys[sc.Key] = true
	}
	if len(gameKeys) != 4 {
		t.Fatalf("gameitems 场景数 = %d, want 4", len(gameKeys))
	}
	var noXfer *SeedScenario
	for i := range byKey["genesis"].Scenarios {
		if byKey["genesis"].Scenarios[i].Key == "no_transfer" {
			noXfer = &byKey["genesis"].Scenarios[i]
		}
	}
	if noXfer == nil || noXfer.Transferable {
		t.Fatal("no_transfer 必须 transferable=false")
	}
	if !strings.HasPrefix(byKey["genesis"].CoverURL, "https://picsum.photos/seed/") {
		t.Fatal("封面必须是 picsum 真图")
	}
}

func TestSeedFullPlanCompliance(t *testing.T) {
	p := BuildSeedPlan("full", "s1")
	c := p.Compliance
	if c.Symbol != "SCUSD" || c.Decimals != 6 || c.Mode != "Any" {
		t.Fatalf("compliance = %+v", c)
	}
	if c.IssuerName != "kycorg" || c.Schema != "SeedKyc1" {
		t.Fatalf("compliance 挂证 = %s/%s", c.IssuerName, c.Schema)
	}
	if c.CertifiedReceiver != "test01" || c.UncertifiedVictim != "test09" || c.MinterUser != "test02" {
		t.Fatalf("compliance 角色 = %s/%s/%s", c.MinterUser, c.CertifiedReceiver, c.UncertifiedVictim)
	}
}

func TestSeedFullPlanTxs(t *testing.T) {
	p := BuildSeedPlan("full", "s1")
	if len(p.Txs.MilTransfers) < 5 {
		t.Fatalf("mil transfers = %d, want >=5", len(p.Txs.MilTransfers))
	}
	for _, tr := range p.Txs.MilTransfers {
		if tr.From == tr.To || tr.Amount == 0 {
			t.Fatalf("非法互转 %+v", tr)
		}
	}
	if len(p.Txs.Failures) == 0 || p.Txs.Failures[0].ExpectCode != 513 {
		t.Fatal("缺 513 预期失败")
	}
}

func TestSeedFullPlanLifecycle(t *testing.T) {
	p := BuildSeedPlan("full", "s1")
	kinds := map[string]bool{}
	for _, op := range p.VcLifecycle {
		kinds[op.IssuerName+"/"+op.Kind+"/"+op.User] = true
	}
	if !kinds["gameorg/freeze/test05"] || !kinds["gameorg/revoke/test08"] {
		t.Fatalf("lifecycle = %v", kinds)
	}
}

func TestSeedPlanDeterministic(t *testing.T) {
	a := BuildSeedPlan("full", "s1")
	b := BuildSeedPlan("full", "s1")
	if !reflect.DeepEqual(a, b) {
		t.Fatal("同 seed 短语两次构建必须完全相等")
	}
	c := BuildSeedPlan("full", "s2")
	if reflect.DeepEqual(a, c) {
		t.Fatal("不同 seed 短语必须不同")
	}
}

func TestSeedSmallPreset(t *testing.T) {
	p := BuildSeedPlan("small", "s1")
	if len(p.Users) != 4 || len(p.Tokens) != 3 || len(p.Sfts) != 1 {
		t.Fatalf("small 规模 users/tokens/sfts = %d/%d/%d", len(p.Users), len(p.Tokens), len(p.Sfts))
	}
	if len(p.Sfts[0].Scenarios) != 3 {
		t.Fatalf("small 场景 = %d, want 3", len(p.Sfts[0].Scenarios))
	}
	if len(p.VcIssuers) != 1 || len(p.VcLifecycle) != 0 {
		t.Fatalf("small vc = %d issuers, %d lifecycle", len(p.VcIssuers), len(p.VcLifecycle))
	}
	victim := p.Compliance.UncertifiedVictim
	for _, vi := range p.VcIssuers {
		for _, u := range vi.Certifies {
			if u == victim {
				t.Fatalf("victim %s 不应持证", victim)
			}
		}
	}
}

func TestSeedBuildPlanUnknownScale(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("未知规模应 panic")
		}
	}()
	BuildSeedPlan("huge", "s1")
}

func seedTestName(i int) string {
	return seedFormatTestName(i)
}

func seedTestRange(from, to int) []string {
	out := []string{}
	for i := from; i <= to; i++ {
		out = append(out, seedTestName(i))
	}
	return out
}
