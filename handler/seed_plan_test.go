// handler/seed_plan_test.go
package handler

import "testing"

// 旧矩阵断言已迁至 seed_plan_v2_test.go(计划生成 v2)。
// 本文件保留旧 scale 包装的最小冒烟断言。

func TestBuildSeedPlanLegacySmoke(t *testing.T) {
	for _, scale := range []string{"full", "small"} {
		p, err := BuildSeedPlan(scale, SeedDefaultPhrase)
		if err != nil {
			t.Fatalf("scale=%s: %v", scale, err)
		}
		if len(p.Users) == 0 || p.Admin.Name != "admin" {
			t.Fatalf("scale=%s 计划不完整", scale)
		}
	}
}
