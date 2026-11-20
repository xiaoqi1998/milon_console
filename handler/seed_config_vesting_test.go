// handler/seed_config_vesting_test.go
package handler

import "testing"

// vestCfg 返回带合法 vesting 计划的基线配置(plans=2)。
func vestCfg() SeedConfig {
	c := validCfg()
	c.Vesting.Plans = 2
	return c
}

func TestValidateSeedConfig_Vesting_AcceptsPlans(t *testing.T) {
	if errs := ValidateSeedConfig(vestCfg()); len(errs) != 0 {
		t.Fatalf("plans=2 默认参数应合法, got %v", errs)
	}
}

func TestValidateSeedConfig_Vesting_FieldPathErrors(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*SeedConfig)
		want string // 错误信息须含的字段路径
	}{
		{"计划数负数", func(c *SeedConfig) { c.Vesting.Plans = -1 }, "vesting.plans"},
		{"计划数超上限", func(c *SeedConfig) { c.Vesting.Plans = 21 }, "vesting.plans"},
		{"人数为零", func(c *SeedConfig) { c.Vesting.Recipients = 0 }, "vesting.recipients"},
		{"人数超上限", func(c *SeedConfig) { c.Vesting.Recipients = 21 }, "vesting.recipients"},
		{"总额为零", func(c *SeedConfig) { c.Vesting.Total = 0 }, "vesting.total"},
		{"首释超总额", func(c *SeedConfig) { c.Vesting.Initial = c.Vesting.Total + 1 }, "vesting.initial"},
		{"期数为零", func(c *SeedConfig) { c.Vesting.Periods = 0 }, "vesting.periods"},
		{"每期秒为零", func(c *SeedConfig) { c.Vesting.PeriodSeconds = 0 }, "vesting.periodSeconds"},
		{"每期释放不足一", func(c *SeedConfig) {
			c.Vesting.Total = 10
			c.Vesting.Initial = 9
			c.Vesting.Periods = 10
		}, "vesting.release"},
		{"起始延迟为零", func(c *SeedConfig) { c.Vesting.StartDelaySeconds = 0 }, "vesting.startDelaySeconds"},
		{"名单含空名", func(c *SeedConfig) { c.Vesting.Names = []string{"test01", "  "} }, "vesting.names"},
	}
	for _, tc := range cases {
		c := vestCfg()
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

// random=true 时金额/周期参数走服务端随机范围,用户填的参数不再校验。
func TestValidateSeedConfig_Vesting_RandomSkipsParamErrors(t *testing.T) {
	c := vestCfg()
	c.Vesting.Random = true
	c.Vesting.Total = 0
	c.Vesting.Initial = 999999 // > total,普通模式必报
	c.Vesting.Periods = 0
	c.Vesting.PeriodSeconds = 0
	c.Vesting.StartDelaySeconds = 0
	if errs := ValidateSeedConfig(c); len(errs) != 0 {
		t.Fatalf("random=true 应跳过参数范围校验, got %v", errs)
	}
	// plans/recipients 范围仍校验
	c.Vesting.Recipients = 0
	if errs := ValidateSeedConfig(c); len(errs) == 0 {
		t.Fatal("random=true 仍应校验 recipients 范围")
	}
}

func TestNormalizeSeedConfig_VestingRandomClearsNames(t *testing.T) {
	c := vestCfg()
	c.Vesting.Random = true
	c.Vesting.Names = []string{"test01", "test02"}
	out := NormalizeSeedConfig(c)
	if len(out.Vesting.Names) != 0 {
		t.Fatalf("random=true 应清空 names(随机挑人), got %v", out.Vesting.Names)
	}
}

func TestSeedConfigHash_VestingFieldSensitive(t *testing.T) {
	a := vestCfg()
	b := vestCfg()
	b.Vesting.Plans = 3
	if SeedConfigHash(a) == SeedConfigHash(b) {
		t.Fatal("改 vesting.plans 应产生不同配置命名空间")
	}
}
