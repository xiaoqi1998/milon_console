// handler/seed_plan_vesting_test.go
package handler

import (
	"strconv"
	"testing"
)

// vestPlanCfg 返回 plans=2 的展开测试基线:tokens 3 个、前 1 个合规 → 非合规池 2 个。
func vestPlanCfg() SeedConfig {
	c := validCfg()
	c.Vesting.Plans = 2
	c.Vesting.Recipients = 2
	return c
}

func planVestings(t *testing.T, cfg SeedConfig) []SeedVestingSpec {
	t.Helper()
	p, err := BuildSeedPlanV2(cfg)
	if err != nil {
		t.Fatalf("BuildSeedPlanV2: %v", err)
	}
	return p.Vestings
}

// 537 合法定义:initial≤total、每期释放 floor((total−initial)/periods) ≥1。
func vestDefOK(r SeedVestingRecipientSpec) bool {
	return r.Initial <= r.Total && r.Total >= r.Initial &&
		(r.Total-r.Initial) >= r.Periods && r.Periods >= 1 && r.PeriodSeconds >= 1
}

// 非合规代币池够用:计划依次复用 ST02/ST03(complianceCount=1 跳过 ST01)。
func TestBuildSeedPlanV2_Vesting_ReuseNonComplianceTokens(t *testing.T) {
	vs := planVestings(t, vestPlanCfg())
	if len(vs) != 2 {
		t.Fatalf("应展开 2 个计划, got %d", len(vs))
	}
	if vs[0].ReuseToken != "ST02" || vs[1].ReuseToken != "ST03" {
		t.Fatalf("应依次复用 ST02/ST03, got %s / %s", vs[0].ReuseToken, vs[1].ReuseToken)
	}
	if vs[0].TokenSpec != nil || vs[1].TokenSpec != nil {
		t.Fatal("复用模式不应携带补建代币规格")
	}
}

// 非合规池不足:自动补建 VT01..,补建代币 mint 总额=Σ 每人 total(admin 托管出资)。
func TestBuildSeedPlanV2_Vesting_SupplementTokens(t *testing.T) {
	c := vestPlanCfg()
	c.Tokens.Count = 1
	c.Tokens.ComplianceCount = 1 // 非合规池为空
	vs := planVestings(t, c)
	if len(vs) != 2 {
		t.Fatalf("应展开 2 个计划, got %d", len(vs))
	}
	for i, v := range vs {
		want := "VT0" + string(rune('1'+i))
		if v.ReuseToken != "" || v.TokenSpec == nil {
			t.Fatalf("计划 %d 应补建代币 %s, got reuse=%q spec=%v", i, want, v.ReuseToken, v.TokenSpec)
		}
		if v.TokenSpec.Symbol != want {
			t.Fatalf("补建代币符号应为 %s, got %s", want, v.TokenSpec.Symbol)
		}
		var sum uint64
		for _, r := range v.Recipients {
			sum += r.Total
		}
		if v.TokenSpec.MintAmounts["admin"] != sum {
			t.Fatalf("补建代币 admin 铸额应=Σtotal(%d), got %d", sum, v.TokenSpec.MintAmounts["admin"])
		}
	}
}

// 混合:池 1 个 + 计划 2 个 → 第一个复用、第二个补建。
func TestBuildSeedPlanV2_Vesting_MixedPool(t *testing.T) {
	c := vestPlanCfg()
	c.Tokens.Count = 2
	c.Tokens.ComplianceCount = 1
	vs := planVestings(t, c)
	if vs[0].ReuseToken != "ST02" {
		t.Fatalf("计划 0 应复用 ST02, got %q", vs[0].ReuseToken)
	}
	if vs[1].TokenSpec == nil || vs[1].TokenSpec.Symbol != "VT01" {
		t.Fatalf("计划 1 应补建 VT01, got reuse=%q", vs[1].ReuseToken)
	}
}

// 名单覆盖:names 非空时按名单取人(recipients 旋钮失效);计划间循环复用名单。
func TestBuildSeedPlanV2_Vesting_NamesOverride(t *testing.T) {
	c := vestPlanCfg()
	c.Vesting.Recipients = 5 // 名单模式下忽略
	c.Vesting.Names = []string{"test03", "test01"}
	vs := planVestings(t, c)
	if len(vs[0].Recipients) != 2 || vs[0].Recipients[0].Name != "test03" || vs[0].Recipients[1].Name != "test01" {
		t.Fatalf("名单应按序取人, got %+v", vs[0].Recipients)
	}
	if len(vs[1].Recipients) != 2 || vs[1].Recipients[0].Name != "test03" {
		t.Fatalf("第二计划应循环复用名单, got %+v", vs[1].Recipients)
	}
}

// 无名单:按人数从 users 接续轮转(计划 i 第 j 人 = users[(i*M+j)%n])。
func TestBuildSeedPlanV2_Vesting_UsersRotation(t *testing.T) {
	c := vestPlanCfg()
	c.Vesting.Recipients = 2
	vs := planVestings(t, c)
	if len(vs) != 2 {
		t.Fatalf("应展开 2 个计划, got %d (%+v)", len(vs), vs)
	}
	for i, v := range vs {
		if len(v.Recipients) != 2 {
			t.Fatalf("计划 %d 应有 2 个领款人, got %d (%+v)", i, len(v.Recipients), v.Recipients)
		}
	}
	if vs[0].Recipients[0].Name != "test01" || vs[0].Recipients[1].Name != "test02" {
		t.Fatalf("计划 0 应从 test01 起, got %+v", vs[0].Recipients)
	}
	if vs[1].Recipients[0].Name != "test03" || vs[1].Recipients[1].Name != "test04" {
		t.Fatalf("计划 1 应接续轮转 test03/test04, got %+v", vs[1].Recipients)
	}
}

// 非 random:参数按 index 轮转互异,且全部满足链端 537 合法定义。
func TestBuildSeedPlanV2_Vesting_ParamVariants(t *testing.T) {
	c := vestPlanCfg()
	c.Vesting.Recipients = 3
	c.Vesting.Total = 100
	c.Vesting.Initial = 50
	c.Vesting.Periods = 10 // 极端小参数也要被 sanitize 成合法
	vs := planVestings(t, c)
	for _, v := range vs {
		seen := map[[4]uint64]bool{}
		for _, r := range v.Recipients {
			if !vestDefOK(r) {
				t.Fatalf("recipient %+v 不满足 537 合法定义", r)
			}
			key := [4]uint64{r.Total, r.Initial, r.PeriodSeconds, r.Periods}
			if seen[key] {
				t.Fatalf("同计划内参数应轮转去重, 重复 %+v", key)
			}
			seen[key] = true
		}
	}
}

// 同输入同计划(含 random):确定性纯函数。
func TestBuildSeedPlanV2_Vesting_Deterministic(t *testing.T) {
	for _, random := range []bool{false, true} {
		c := vestPlanCfg()
		c.Vesting.Random = random
		a := planVestings(t, c)
		b := planVestings(t, c)
		if len(a) != len(b) {
			t.Fatalf("random=%v: 计划数不一致", random)
		}
		for i := range a {
			if vestSpecStr(a[i]) != vestSpecStr(b[i]) {
				t.Fatalf("random=%v 计划 %d 不确定:\n%v\n%v", random, i, vestSpecStr(a[i]), vestSpecStr(b[i]))
			}
		}
	}
}

// random=true:参数在安全范围内(总额 1万..50万、periods 3..12、periodSeconds ≥30)。
func TestBuildSeedPlanV2_Vesting_RandomRanges(t *testing.T) {
	c := vestPlanCfg()
	c.Vesting.Random = true
	c.Vesting.Recipients = 4
	vs := planVestings(t, c)
	for _, v := range vs {
		if v.StartDelaySeconds < 60 || v.StartDelaySeconds > 600 {
			t.Fatalf("随机 startDelay 应在 60..600, got %d", v.StartDelaySeconds)
		}
		for _, r := range v.Recipients {
			if !vestDefOK(r) {
				t.Fatalf("random recipient %+v 不满足 537", r)
			}
			if r.Total < 10_000 || r.Total > 500_000 {
				t.Fatalf("random total 应在 1万..50万, got %d", r.Total)
			}
			if r.Periods < 3 || r.Periods > 12 {
				t.Fatalf("random periods 应在 3..12, got %d", r.Periods)
			}
			if r.PeriodSeconds < 30 || r.PeriodSeconds > 120 {
				t.Fatalf("random periodSeconds 应在 30..120, got %d", r.PeriodSeconds)
			}
		}
	}
}

// Summary 计数与交易预估计入 vesting(补建代币 create+mint、每计划 CreateVesting、claim 按 recipients)。
func TestSeedPlanSummary_VestingCounts(t *testing.T) {
	c := vestPlanCfg()
	c.Vesting.Claim = true
	p, err := BuildSeedPlanV2(c)
	if err != nil {
		t.Fatalf("BuildSeedPlanV2: %v", err)
	}
	s := SeedPlanSummary(p)
	if s.Vestings != 2 {
		t.Fatalf("summary.vestings 应=2, got %d", s.Vestings)
	}
	base := vestPlanCfg()
	p0, _ := BuildSeedPlanV2(base)
	s0 := SeedPlanSummary(p0)
	if s.TxEstimate <= s0.TxEstimate {
		t.Fatalf("claim=true 应比不领多交易(%d vs %d)", s.TxEstimate, s0.TxEstimate)
	}
	if s.TxEstimate-s0.TxEstimate != 4 { // 2 计划 × 2 recipients
		t.Fatalf("claim 增量应=4(2 计划×2 人), got %d", s.TxEstimate-s0.TxEstimate)
	}
}

// vestSpecStr 计划的稳定序列化(测试比对用)。
func vestSpecStr(v SeedVestingSpec) string {
	s := v.ReuseToken + "|" + v.Symbol + "|" + v.Key + "|" + strconv.FormatUint(v.StartDelaySeconds, 10) + "\n"
	for _, r := range v.Recipients {
		s += r.Name + "|" + strconv.FormatUint(r.Total, 10) + "|" + strconv.FormatUint(r.Initial, 10) + "|" +
			strconv.FormatUint(r.InitialPeriodSeconds, 10) + "|" + strconv.FormatUint(r.PeriodSeconds, 10) + "|" +
			strconv.FormatUint(r.Periods, 10) + "\n"
	}
	return s
}
