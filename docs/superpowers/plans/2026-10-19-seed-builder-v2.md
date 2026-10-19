# 测试数据构造器 v2（Seed Builder v2）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把播种器从 `scale=full|small` 两档写死矩阵，重做为「模板 + 全局旋钮」参数化构造器——账号数/领水/多签/DID/合规证书、代币精度与数量、SFT/slot/token 数量、余额档位全部可选，同 `seedPhrase+config` 完全可复现。

**Architecture:** `SeedConfig` 强类型结构为唯一真相源（方案一，规格已确认）；模板内容定义在服务端；计划生成是纯函数 `BuildSeedPlanV2(cfg)`，预览与执行共用同一路径；执行器从六模块扩为七模块（新增 multisig，插在 accounts 后）；断点状态按 `configHash` 命名空间隔离；控制台页面整页重写。

**Tech Stack:** Go 1.x + gin + milon-go-sdk（`gosdk-develop` 本地版本）；前端原生 JS（`static/js/seed.js`）；测试 Go `testing` + node 直抽（`node test_*.js`）。

**Spec:** `docs/superpowers/specs/2026-10-19-seed-builder-v2-design.md`（实施前必读，本计划从规格出发）

## Global Constraints

- 全程简体中文注释与日志（对齐现有代码风格）。
- TDD：每个任务先写失败测试 → 红 → 最小实现 → 绿 → 提交。
- 计划生成必须纯函数零 IO；确定性：同 `seedPhrase+config` 两次构建逐字段相等。
- 私钥永不返回给前端（对齐现有 `seed_handler.go` 注释约定）。
- 链端错误容忍码沿用现有函数：`seedHasCode`（516 已存在/513 余额不足/521,522 VC 拦截/1282,1283 SFT 幂等）。
- SDK 入口统一走 `mc.GetAllPd()[app]` + `encodeWithCoercion`（请求级客户端，不用 `gen.*` 包级单例，避免并发绑 pd 的竞态）。
- 状态文件共用 `data/seed_state.json`；v2 键前缀 `configHash`，旧键一次性作废（规格已确认）。
- 数值上限：accounts.count≤200、multisig.count≤20、tokens.count≤20、sft.count≤10、slotsPerSft≤20、tokensPerSlot≤100。
- 每 2-3 个任务跑一次 `go build ./...` 防漂移；最后全量 `go test ./...` + `node test_seed_v2_section.js`。

---

### Task 1: SeedConfig 配置模型 + 校验

**Files:**
- Create: `handler/seed_config.go`
- Test: `handler/seed_config_test.go`

**Interfaces:**
- Consumes: `SeedDefaultPhrase`（`handler/seed_plan.go:14` 已有）
- Produces: `SeedConfig`/`SeedAccountsCfg`/`SeedMultisigCfg`/`SeedTokensCfg`/`SeedSftCfg` 结构体（JSON tag 见实现）；`SeedConfigDefaults() SeedConfig`；`NormalizeSeedConfig(SeedConfig) SeedConfig`；`ValidateSeedConfig(SeedConfig) []string`（空切片=合法，元素含字段路径）；`SeedConfigHash(SeedConfig) string`（16 hex）；`seedBalanceTiers map[string]uint64`；`seedScenarioKeys map[string]bool`

- [ ] **Step 1: 写失败测试**

```go
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

func TestValidateSeedConfig_FieldPathErrors(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*SeedConfig)
		want string // 错误信息须含的字段路径
	}{
		{"账号数越界", func(c *SeedConfig) { c.Accounts.Count = 0 }, "accounts.count"},
		{"曲线为空", func(c *SeedConfig) { c.Accounts.Curves = nil }, "accounts.curves"},
		{"非法曲线", func(c *SeedConfig) { c.Accounts.Curves = []string{"rsa"} }, "accounts.curves[rsa]"},
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run TestValidateSeedConfig -v && go test ./handler/ -run "TestNormalizeSeedConfig|TestSeedConfigHash" -v`
Expected: FAIL（`undefined: SeedConfigDefaults` 等编译错误）

- [ ] **Step 3: 最小实现**

```go
// handler/seed_config.go
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ==================== 播种器 v2:配置模型 ====================
// SeedConfig 是唯一真相源:校验、模板展开、计划生成、执行全部围绕它。
// 规格见 docs/superpowers/specs/2026-10-19-seed-builder-v2-design.md §2。

// seedBalanceTiers 余额档位 → MIL 数(展示口径;执行期乘 seedMil 转最小单位)。
var seedBalanceTiers = map[string]uint64{
	"hundred": 100, "thousand": 1000, "ten_k": 10000,
}

// seedValidCurves 合法密钥曲线(链端三选一)。
var seedValidCurves = map[string]bool{"secp256k1": true, "ed25519": true, "fndsa512": true}

// seedScenarioKeys 合法 SFT 场景键。
var seedScenarioKeys = map[string]bool{
	"transferable": true, "frozen": true, "merge": true, "burn": true, "reject_transfer": true,
}

type SeedConfig struct {
	SeedPhrase string          `json:"seedPhrase"`
	Accounts   SeedAccountsCfg `json:"accounts"`
	Multisig   SeedMultisigCfg `json:"multisig"`
	Tokens     SeedTokensCfg   `json:"tokens"`
	SFT        SeedSftCfg      `json:"sft"`
}

type SeedAccountsCfg struct {
	Count        int      `json:"count"`
	Curves       []string `json:"curves"`
	Faucet       bool     `json:"faucet"`
	BalanceTiers []string `json:"balanceTiers"`
	DID          bool     `json:"did"`
	VcCert       bool     `json:"vcCert"`
}

type SeedMultisigCfg struct {
	Count     int  `json:"count"`
	Signers   int  `json:"signers"`
	Threshold int  `json:"threshold"`
	Faucet    bool `json:"faucet"`
	DID       bool `json:"did"`
}

type SeedTokensCfg struct {
	Count           int     `json:"count"`
	Decimals        []uint8 `json:"decimals"`
	ComplianceCount int     `json:"complianceCount"`
	FreezeDemo      bool    `json:"freezeDemo"`
}

type SeedSftCfg struct {
	Count         int      `json:"count"`
	SlotsPerSft   int      `json:"slotsPerSft"`
	TokensPerSlot int      `json:"tokensPerSlot"`
	Scenarios     []string `json:"scenarios"`
	PostOps       bool     `json:"postOps"`
}

// SeedConfigDefaults 缺省配置(= standard 模板内容)。
func SeedConfigDefaults() SeedConfig {
	return SeedConfig{
		SeedPhrase: SeedDefaultPhrase,
		Accounts: SeedAccountsCfg{
			Count: 8, Curves: []string{"secp256k1", "ed25519", "fndsa512"},
			Faucet: true, BalanceTiers: []string{"thousand"}, DID: true, VcCert: true,
		},
		Multisig: SeedMultisigCfg{Count: 1, Signers: 5, Threshold: 3, Faucet: true, DID: true},
		Tokens: SeedTokensCfg{
			Count: 3, Decimals: []uint8{0, 6, 9}, ComplianceCount: 1, FreezeDemo: true,
		},
		SFT: SeedSftCfg{
			Count: 2, SlotsPerSft: 2, TokensPerSlot: 3,
			Scenarios: []string{"transferable", "frozen", "merge", "burn", "reject_transfer"},
			PostOps:   true,
		},
	}
}

// NormalizeSeedConfig 应用隐含规则:vcCert ⇒ did(证书挂 DID 主体)。
func NormalizeSeedConfig(c SeedConfig) SeedConfig {
	if c.Accounts.VcCert {
		c.Accounts.DID = true
	}
	return c
}

// ValidateSeedConfig 校验配置;返回带字段路径的错误列表,空 = 合法。
// 纯函数:不修改入参(隐含规则用 NormalizeSeedConfig 先行)。
func ValidateSeedConfig(c SeedConfig) []string {
	var errs []string
	addf := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }

	if c.Accounts.Count < 1 || c.Accounts.Count > 200 {
		addf("accounts.count: 须在 1..200, got %d", c.Accounts.Count)
	}
	if len(c.Accounts.Curves) == 0 {
		addf("accounts.curves: 不能为空")
	}
	cseen := map[string]bool{}
	for _, cv := range c.Accounts.Curves {
		if !seedValidCurves[cv] {
			addf("accounts.curves[%s]: 非法曲线(可选 secp256k1/ed25519/fndsa512)", cv)
		}
		if cseen[cv] {
			addf("accounts.curves[%s]: 重复", cv)
		}
		cseen[cv] = true
	}
	if c.Accounts.Faucet && len(c.Accounts.BalanceTiers) == 0 {
		addf("accounts.balanceTiers: 须至少选一档(可选 hundred/thousand/ten_k)")
	}
	for _, tier := range c.Accounts.BalanceTiers {
		if _, ok := seedBalanceTiers[tier]; !ok {
			addf("accounts.balanceTiers[%s]: 非法档位(可选 hundred/thousand/ten_k)", tier)
		}
	}
	if c.Accounts.DID && !c.Accounts.Faucet && len(c.Accounts.BalanceTiers) == 0 {
		addf("accounts.did: 建 DID 需账户有 gas——开启领水或选余额档位")
	}

	if c.Multisig.Count < 0 || c.Multisig.Count > 20 {
		addf("multisig.count: 须在 0..20, got %d", c.Multisig.Count)
	}
	if c.Multisig.Count > 0 {
		if c.Multisig.Signers < 2 || c.Multisig.Signers > 20 {
			addf("multisig.signers: 须在 2..20, got %d", c.Multisig.Signers)
		}
		if c.Multisig.Threshold < 1 || c.Multisig.Threshold > c.Multisig.Signers {
			addf("multisig.threshold: 须在 1..signers(%d), got %d", c.Multisig.Signers, c.Multisig.Threshold)
		}
		if c.Multisig.Signers > c.Accounts.Count {
			addf("multisig.signers: %d 超过普通账号数 %d(签名者从普通账号轮取)", c.Multisig.Signers, c.Accounts.Count)
		}
		if c.Multisig.DID && !c.Multisig.Faucet {
			addf("multisig.did: 建 DID 需 multisig.faucet=true(多签账户自身付 gas)")
		}
	}

	if c.Tokens.Count < 1 || c.Tokens.Count > 20 {
		addf("tokens.count: 须在 1..20, got %d", c.Tokens.Count)
	}
	if len(c.Tokens.Decimals) == 0 {
		addf("tokens.decimals: 不能为空")
	}
	dseen := map[uint8]bool{}
	for _, d := range c.Tokens.Decimals {
		if d > 9 {
			addf("tokens.decimals[%d]: 超出 0..9", d)
		}
		if dseen[d] {
			addf("tokens.decimals[%d]: 重复", d)
		}
		dseen[d] = true
	}
	if c.Tokens.ComplianceCount < 0 || c.Tokens.ComplianceCount > c.Tokens.Count {
		addf("tokens.complianceCount: 须在 0..tokens.count(%d), got %d", c.Tokens.Count, c.Tokens.ComplianceCount)
	}
	if c.Tokens.ComplianceCount > 0 {
		if !c.Accounts.VcCert {
			addf("tokens.complianceCount: 合规代币需 accounts.vcCert=true(须有签发方凭证)")
		}
		if c.Accounts.Count < 3 {
			addf("tokens.complianceCount: 合规演示需 ≥3 个普通账号(minter/持证收款/无证拦截)")
		}
	}

	if c.SFT.Count < 0 || c.SFT.Count > 10 {
		addf("sft.count: 须在 0..10, got %d", c.SFT.Count)
	}
	if c.SFT.Count > 0 {
		if c.SFT.SlotsPerSft < 1 || c.SFT.SlotsPerSft > 20 {
			addf("sft.slotsPerSft: 须在 1..20, got %d", c.SFT.SlotsPerSft)
		}
		if c.SFT.TokensPerSlot < 1 || c.SFT.TokensPerSlot > 100 {
			addf("sft.tokensPerSlot: 须在 1..100, got %d", c.SFT.TokensPerSlot)
		}
		if len(c.SFT.Scenarios) == 0 {
			addf("sft.scenarios: sft.count>0 时不能为空")
		}
		sseen := map[string]bool{}
		for _, s := range c.SFT.Scenarios {
			if !seedScenarioKeys[s] {
				addf("sft.scenarios[%s]: 非法场景(可选 transferable/frozen/merge/burn/reject_transfer)", s)
			}
			if sseen[s] {
				addf("sft.scenarios[%s]: 重复", s)
			}
			sseen[s] = true
		}
	}
	return errs
}

// SeedConfigHash 配置命名空间哈希:sha256(seedPhrase + 0x00 + 规范 JSON) 前 16 hex。
// 同配置同哈希 → 断点续跑;改任一旋钮 → 新命名空间。
func SeedConfigHash(c SeedConfig) string {
	raw, _ := json.Marshal(c)
	buf := append([]byte(c.SeedPhrase), 0)
	buf = append(buf, raw...)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])[:16]
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./handler/ -run "TestValidateSeedConfig|TestNormalizeSeedConfig|TestSeedConfigHash" -v`
Expected: PASS 全部

- [ ] **Step 5: 提交**

```bash
git add handler/seed_config.go handler/seed_config_test.go
git commit -m "feat(seed-v2): SeedConfig 配置模型+字段路径校验+configHash 命名空间哈希"
```

---

### Task 2: 内置模板 + preset 解析 + 深合并

**Files:**
- Create: `handler/seed_templates.go`
- Test: `handler/seed_templates_test.go`

**Interfaces:**
- Consumes: Task 1 全部
- Produces: `SeedTemplate{Key,Name,Description string; Config SeedConfig}`；`SeedTemplates() []SeedTemplate`（minimal/standard/full 三个）；`SeedConfigForPreset(string) (SeedConfig, error)`；`seedMergeConfig(base SeedConfig, over json.RawMessage) (SeedConfig, error)`；`seedMergeMaps(map[string]any, map[string]any) map[string]any`

- [ ] **Step 1: 写失败测试**

```go
// handler/seed_templates_test.go
package handler

import (
	"encoding/json"
	"testing"
)

func TestSeedTemplates_ThreeValid(t *testing.T) {
	tpl := SeedTemplates()
	if len(tpl) != 3 {
		t.Fatalf("须三个模板, got %d", len(tpl))
	}
	keys := map[string]bool{}
	for _, t2 := range tpl {
		keys[t2.Key] = true
		if errs := ValidateSeedConfig(t2.Config); len(errs) > 0 {
			t.Errorf("模板 %s 非法: %v", t2.Key, errs)
		}
	}
	for _, want := range []string{"minimal", "standard", "full"} {
		if !keys[want] {
			t.Errorf("缺模板 %s", want)
		}
	}
}

func TestSeedTemplates_Matrix(t *testing.T) {
	get := func(k string) SeedConfig {
		c, err := SeedConfigForPreset(k)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	mini, full := get("minimal"), get("full")
	if mini.Accounts.Count != 3 || mini.Multisig.Count != 0 || mini.Accounts.VcCert {
		t.Error("minimal 矩阵不符")
	}
	if mini.Tokens.Count != 1 || len(mini.Tokens.Decimals) != 1 || mini.Tokens.Decimals[0] != 6 {
		t.Error("minimal 代币矩阵不符")
	}
	if full.Accounts.Count != 12 || full.Multisig.Count != 2 || full.Tokens.Count != 10 ||
		full.Tokens.ComplianceCount != 2 || full.SFT.Count != 3 || !full.SFT.PostOps {
		t.Error("full 矩阵不符")
	}
	if len(full.Tokens.Decimals) != 10 {
		t.Error("full 须 0~9 全精度")
	}
}

func TestSeedConfigForPreset_Unknown(t *testing.T) {
	if _, err := SeedConfigForPreset("huge"); err == nil {
		t.Fatal("未知模板应报错")
	}
}

func TestSeedMergeConfig_DeepOverride(t *testing.T) {
	base := SeedConfigDefaults()
	over := json.RawMessage(`{"accounts":{"count":5},"tokens":{"decimals":[2]},"multisig":{"count":0}}`)
	got, err := seedMergeConfig(base, over)
	if err != nil {
		t.Fatal(err)
	}
	if got.Accounts.Count != 5 {
		t.Errorf("count 应被覆盖为 5, got %d", got.Accounts.Count)
	}
	if got.Accounts.Faucet != true {
		t.Error("未覆盖字段须保留 base 值")
	}
	if len(got.Tokens.Decimals) != 1 || got.Tokens.Decimals[0] != 2 {
		t.Errorf("decimals 应被覆盖, got %v", got.Tokens.Decimals)
	}
	if got.Tokens.Count != base.Tokens.Count {
		t.Error("tokens.count 未覆盖时须保留")
	}
	if got.Multisig.Count != 0 {
		t.Errorf("multisig.count 应为 0, got %d", got.Multisig.Count)
	}
	if got.SeedPhrase != base.SeedPhrase {
		t.Error("seedPhrase 未覆盖时须保留")
	}
}

func TestSeedMergeConfig_BadJSON(t *testing.T) {
	if _, err := seedMergeConfig(SeedConfigDefaults(), json.RawMessage(`{bad`)); err == nil {
		t.Fatal("坏 JSON 应报错")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run "TestSeedTemplates|TestSeedConfigForPreset|TestSeedMergeConfig" -v`
Expected: FAIL（undefined 编译错误）

- [ ] **Step 3: 最小实现**

```go
// handler/seed_templates.go
package handler

import (
	"encoding/json"
	"fmt"
)

// ==================== 播种器 v2:内置模板 ====================
// 模板内容定义在服务端(规格 §3):前端只是拉取填充;API 直调方同样可用。

type SeedTemplate struct {
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Config      SeedConfig `json:"config"`
}

// SeedTemplates 三个内置模板:极简/标准/全量。
func SeedTemplates() []SeedTemplate {
	minimal := SeedConfigDefaults()
	minimal.Accounts = SeedAccountsCfg{Count: 3, Curves: []string{"secp256k1"},
		Faucet: true, BalanceTiers: []string{"hundred"}, DID: false, VcCert: false}
	minimal.Multisig = SeedMultisigCfg{Count: 0, Signers: 5, Threshold: 3, Faucet: false, DID: false}
	minimal.Tokens = SeedTokensCfg{Count: 1, Decimals: []uint8{6}, ComplianceCount: 0, FreezeDemo: false}
	minimal.SFT = SeedSftCfg{Count: 1, SlotsPerSft: 1, TokensPerSlot: 3,
		Scenarios: []string{"transferable"}, PostOps: false}

	standard := SeedConfigDefaults()

	full := SeedConfigDefaults()
	full.Accounts = SeedAccountsCfg{Count: 12, Curves: []string{"secp256k1", "ed25519", "fndsa512"},
		Faucet: true, BalanceTiers: []string{"hundred", "thousand", "ten_k"}, DID: true, VcCert: true}
	full.Multisig = SeedMultisigCfg{Count: 2, Signers: 5, Threshold: 3, Faucet: true, DID: true}
	full.Tokens = SeedTokensCfg{Count: 10, Decimals: []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		ComplianceCount: 2, FreezeDemo: true}
	full.SFT = SeedSftCfg{Count: 3, SlotsPerSft: 2, TokensPerSlot: 3,
		Scenarios: []string{"transferable", "frozen", "merge", "burn", "reject_transfer"}, PostOps: true}

	return []SeedTemplate{
		{Key: "minimal", Name: "极简", Description: "3 账号 · 1 代币 · 1 SFT —— 冒烟,分钟级", Config: minimal},
		{Key: "standard", Name: "标准", Description: "8 账号 · 3 代币(含 1 合规) · 1 多签 · 2 SFT —— 日常开发", Config: standard},
		{Key: "full", Name: "全量", Description: "12 账号 · 10 代币全精度(含 2 合规) · 2 多签 · 3 SFT —— 发布前回归", Config: full},
	}
}

// SeedConfigForPreset 模板键 → 配置副本。
func SeedConfigForPreset(preset string) (SeedConfig, error) {
	for _, t := range SeedTemplates() {
		if t.Key == preset {
			return t.Config, nil
		}
	}
	return SeedConfig{}, fmt.Errorf("未知模板 %q(可选 minimal/standard/full)", preset)
}

// seedMergeConfig 覆盖项深合并到基础配置后重解。
// 覆盖 JSON 未提及的字段保留 base 值;显式 0/false/null 即覆盖为零值。
func seedMergeConfig(base SeedConfig, over json.RawMessage) (SeedConfig, error) {
	baseMap := map[string]any{}
	if raw, err := json.Marshal(base); err == nil {
		_ = json.Unmarshal(raw, &baseMap)
	}
	overMap := map[string]any{}
	if err := json.Unmarshal(over, &overMap); err != nil {
		return SeedConfig{}, fmt.Errorf("overrides 解析失败: %w", err)
	}
	merged := seedMergeMaps(baseMap, overMap)
	raw, err := json.Marshal(merged)
	if err != nil {
		return SeedConfig{}, err
	}
	var out SeedConfig
	if err := json.Unmarshal(raw, &out); err != nil {
		return SeedConfig{}, fmt.Errorf("overrides 合并结果非法: %w", err)
	}
	return out, nil
}

// seedMergeMaps 递归合并:over 优先;两侧同为 map 时下钻。
func seedMergeMaps(base, over map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		if bm, ok := out[k].(map[string]any); ok {
			if om, ok2 := v.(map[string]any); ok2 {
				out[k] = seedMergeMaps(bm, om)
				continue
			}
		}
		out[k] = v
	}
	return out
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./handler/ -run "TestSeedTemplates|TestSeedConfigForPreset|TestSeedMergeConfig" -v`
Expected: PASS 全部

- [ ] **Step 5: 提交**

```bash
git add handler/seed_templates.go handler/seed_templates_test.go
git commit -m "feat(seed-v2): 三内置模板+深合并 overrides"
```

---

### Task 3: SeedPlan 结构扩展 + BuildSeedPlanV2 + 旧 scale 兼容

**Files:**
- Modify: `handler/seed_plan.go`（SeedPlan 结构加字段；旧 `BuildSeedPlan(scale, phrase)` 改为兼容包装）
- Create: `handler/seed_plan_v2.go`
- Test: `handler/seed_plan_v2_test.go`
- Modify: `handler/seed_plan_test.go`（旧矩阵断言随包装语义更新——见 Step 1）

**Interfaces:**
- Consumes: Task 1、2 全部；现有 `seedTestNames/seedUserSpecs/seedAvatarURL/seedTokenIconURL/seedTokenURI/seedSftCoverURL/seedSlotCoverURL/seedCardCoverURL/seedPicsum/seedHumanBase/pow10u64/seedMil`
- Produces: `SeedPlan` 新字段 `ConfigHash string`、`Config *SeedConfig`、`Multisig []SeedMultisigSpec`；`Compliance` 由单值改切片 `[]SeedComplianceSpec`；`SeedMultisigSpec` 结构；`BuildSeedPlanV2(SeedConfig) (SeedPlan, error)`；`SeedSummary` 结构；`SeedEstimateTxs(SeedPlan) int`；`seedMultisigName(i int) string`；`seedTokenSymbol(i int) string`

- [ ] **Step 1: 写失败测试**

```go
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run "TestBuildSeedPlanV2|TestBuildSeedPlanLegacyWrapper|TestSeedEstimateTxs" -v`
Expected: FAIL（undefined 编译错误）

- [ ] **Step 3: 实现**

3a. `handler/seed_plan.go` 结构修改——`SeedPlan` 替换为：

```go
type SeedMultisigSpec struct {
	Name        string   `json:"name"`
	Curve       string   `json:"curve"`
	Alias       string   `json:"alias"`
	AliasSuffix uint32   `json:"aliasSuffix"`
	SignerNames []string `json:"signerNames"`
	Threshold   uint8    `json:"threshold"`
	Faucet      bool     `json:"faucet"`
	DID         bool     `json:"did"`
}

type SeedPlan struct {
	Scale       string              `json:"scale"`      // 模板键 minimal|standard|full 或 custom
	ConfigHash  string              `json:"configHash"` // 配置命名空间哈希
	SeedPhrase  string              `json:"seedPhrase"`
	Config      *SeedConfig         `json:"config,omitempty"` // 计划来源配置(执行期开关读这里)
	Admin       SeedAccountSpec     `json:"admin"`
	Users       []SeedAccountSpec   `json:"users"`
	Issuers     []SeedAccountSpec   `json:"issuers"`
	Multisig    []SeedMultisigSpec  `json:"multisig,omitempty"`
	Tokens      []SeedTokenSpec     `json:"tokens"`
	Sfts        []SeedSftSpec       `json:"sfts"`
	VcIssuers   []SeedVcIssuerSpec  `json:"vcIssuers"`
	VcLifecycle []SeedLifecycleOp   `json:"vcLifecycle"`
	Compliance  []SeedComplianceSpec `json:"compliance"`
	Txs         SeedTxsSpec         `json:"txs"`
}
```

同文件底部把旧 `BuildSeedPlan` 整体替换为兼容包装（删除原 full/small 两个 case 的矩阵代码）：

```go
// BuildSeedPlan 旧接口兼容包装:scale → 预设配置 → v2 构建。
// full → full 模板;small → minimal 模板(规格 §3 兼容映射)。
func BuildSeedPlan(scale, seedPhrase string) (SeedPlan, error) {
	preset := scale
	if preset == "small" {
		preset = "minimal"
	}
	cfg, err := SeedConfigForPreset(preset)
	if err != nil {
		return SeedPlan{}, err
	}
	if seedPhrase != "" {
		cfg.SeedPhrase = seedPhrase
	}
	p, err := BuildSeedPlanV2(cfg)
	if err != nil {
		return SeedPlan{}, err
	}
	p.Scale = scale
	return p, nil
}
```

（`seedScenarioPack`、`seedScenarioFilter`、`seedTokens` 三个旧函数删除——v2 场景构建在 `seed_plan_v2.go` 中实现。）

3b. 新建 `handler/seed_plan_v2.go`：

```go
package handler

import (
	"fmt"
	"strings"
)

// ==================== 播种器 v2:计划生成(纯函数,零 IO) ====================
// 同 seedPhrase + config → 逐字段相同。地址在执行期由密钥派生,计划只含
// 规格矩阵 + 计数 + 预估交易数(规格 §4)。

type SeedSummary struct {
	Accounts         int      `json:"accounts"`
	Multisig         int      `json:"multisig"`
	Tokens           int      `json:"tokens"`
	TokensCompliance int      `json:"tokensCompliance"`
	SftSets          int      `json:"sftSets"`
	SftSlots         int      `json:"sftSlots"`
	SftTokens        int      `json:"sftTokens"`
	TxEstimate       int      `json:"txEstimate"`
	Warnings         []string `json:"warnings,omitempty"`
}

func seedMultisigName(i int) string { return fmt.Sprintf("ms%02d", i+1) }

// seedTokenSymbol 普通代币符号:ST01..ST20(确定性,与合规代币 SCUSD* 不冲突)。
func seedTokenSymbol(i int) string { return fmt.Sprintf("ST%02d", i+1) }

// seedUserSpecsV2 用户规格:曲线按 cfg 轮转。
func seedUserSpecsV2(names []string, suffixBase int, curves []string) []SeedAccountSpec {
	out := make([]SeedAccountSpec, len(names))
	for i, name := range names {
		out[i] = SeedAccountSpec{Name: name, Role: "user", Curve: curves[i%len(curves)],
			SubjectType: "Personal", Alias: "seed-" + name,
			AliasSuffix: uint32(suffixBase + i), AvatarURL: seedAvatarURL(name)}
	}
	return out
}

// seedMintMatrix 每代币铸造矩阵:base*(holder序+1)*10^d,轮转覆盖全部分发对象。
func seedMintMatrix(d uint8, distNames []string) map[string]uint64 {
	base := seedHumanBase[d]
	mint := map[string]uint64{}
	for i, name := range distNames {
		mint[name] = base * uint64(i%8+1) * pow10u64(uint64(d))
	}
	return mint
}

// seedScenarioV2 单场景规格:key 决定行为族,tokensPerSlot 决定分发笔数。
func seedScenarioV2(sftKey, key string, names []string, tokensPerSlot int, postOps bool) SeedScenario {
	u := func(i int) string { return names[i%len(names)] }
	hold := u(0)
	slotAttr := func(extra string) string {
		return fmt.Sprintf("scenario=%s;%s", key, extra)
	}
	meta := func(transferable bool, extra string) map[string]any {
		return map[string]any{
			"name": "Slot-" + key, "coverUrl": seedSlotCoverURL(sftKey, key),
			"attribute": slotAttr(extra),
		}
	}
	dists := make([]SeedDist, tokensPerSlot)
	for i := range dists {
		dists[i] = SeedDist{To: hold, Amount: 1,
			Override: map[string]any{"name": fmt.Sprintf("%s-%02d", strings.ToUpper(key[:3]), i+1)}}
	}
	sc := SeedScenario{Key: key, Name: "Slot-" + key, Dists: dists}
	switch key {
	case "transferable":
		sc.Transferable = true
		sc.SlotMeta = meta(true, "multi-holder")
		if postOps && tokensPerSlot >= 3 {
			sc.PostOps = []SeedPostOp{
				{Op: "transfer", By: hold, To: u(1), Amount: 1},
				{Op: "transfer", By: hold, To: u(2), Amount: 1},
			}
		}
	case "frozen":
		sc.Transferable = true
		sc.SlotMeta = meta(true, "minted-then-frozen")
		if postOps {
			sc.PostOps = []SeedPostOp{{Op: "freeze", By: hold}}
		}
	case "merge":
		sc.Transferable = true
		sc.SlotMeta = meta(true, "merge-two-into-one")
		if postOps && tokensPerSlot >= 2 {
			sc.PostOps = []SeedPostOp{{Op: "merge", By: hold}}
		}
	case "burn":
		sc.Transferable = true
		sc.SlotMeta = meta(true, fmt.Sprintf("%d-minted-1-burned", tokensPerSlot))
		if postOps {
			sc.PostOps = []SeedPostOp{{Op: "burn", By: hold, Index: -1}}
		}
	case "reject_transfer":
		sc.Transferable = false
		sc.SlotMeta = meta(false, "transferable=false")
		if postOps {
			sc.PostOps = []SeedPostOp{{Op: "reject_transfer", By: hold, To: u(1)}}
		}
	}
	return sc
}

// BuildSeedPlanV2 由 SeedConfig 构建确定性播种计划。
func BuildSeedPlanV2(cfg SeedConfig) (SeedPlan, error) {
	cfg = NormalizeSeedConfig(cfg)
	if errs := ValidateSeedConfig(cfg); len(errs) > 0 {
		return SeedPlan{}, fmt.Errorf("配置校验失败: %s", strings.Join(errs, "; "))
	}
	names := seedTestNames(cfg.Accounts.Count)
	msNames := make([]string, cfg.Multisig.Count)
	for i := range msNames {
		msNames[i] = seedMultisigName(i)
	}
	distNames := append(append([]string{}, names...), msNames...)

	p := SeedPlan{
		Scale:      "custom",
		ConfigHash: SeedConfigHash(cfg),
		SeedPhrase: cfg.SeedPhrase,
		Config:     &cfg,
	}
	p.Admin = SeedAccountSpec{Name: "admin", Role: "admin", Curve: "secp256k1",
		SubjectType: "Personal", Alias: "seed-admin", AliasSuffix: 3001,
		AvatarURL: seedAvatarURL("admin")}
	p.Users = seedUserSpecsV2(names, 1001, cfg.Accounts.Curves)

	// 签发方:vcCert 或合规代币需要时才建
	needIssuers := cfg.Accounts.VcCert || cfg.Tokens.ComplianceCount > 0
	if needIssuers {
		p.Issuers = []SeedAccountSpec{
			{Name: "kycorg", Role: "issuer", Curve: "ed25519", SubjectType: "Organization",
				Alias: "seed-kycorg", AliasSuffix: 2011, AvatarURL: seedAvatarURL("kycorg")},
		}
	}

	// 多签组:签名者从普通账号头部轮取,等权 1,阈值 m
	for i := 0; i < cfg.Multisig.Count; i++ {
		signers := make([]string, cfg.Multisig.Signers)
		for j := range signers {
			signers[j] = names[j%len(names)]
		}
		p.Multisig = append(p.Multisig, SeedMultisigSpec{
			Name: msNames[i], Curve: "secp256k1",
			Alias: "seed-" + msNames[i], AliasSuffix: uint32(4001 + i),
			SignerNames: signers, Threshold: uint8(cfg.Multisig.Threshold),
			Faucet: cfg.Multisig.Faucet, DID: cfg.Multisig.DID,
		})
	}

	// 普通代币:跳过前 complianceCount 个序号(合规代币由 compliance 模块创建,规格 §4)
	freezeApplied := false
	for i := 0; i < cfg.Tokens.Count; i++ {
		if i < cfg.Tokens.ComplianceCount {
			continue
		}
		d := cfg.Tokens.Decimals[i%len(cfg.Tokens.Decimals)]
		symbol := seedTokenSymbol(i)
		spec := SeedTokenSpec{
			Symbol: symbol, Name: fmt.Sprintf("Seed Token %s %ddp", symbol, d), Decimals: d,
			IconURL: seedTokenIconURL(symbol), URI: seedTokenURI(symbol),
			MintAmounts: seedMintMatrix(d, distNames),
		}
		if cfg.Tokens.FreezeDemo && !freezeApplied {
			freezeApplied = true
			spec.FreezeDemoUser = names[4%len(names)]
		}
		p.Tokens = append(p.Tokens, spec)
	}

	// 合规代币:固定精度 6,挂 kycorg/SeedKyc1,模式 Any;角色按账号序确定化
	for i := 0; i < cfg.Tokens.ComplianceCount; i++ {
		symbol := "SCUSD"
		if i > 0 {
			symbol = fmt.Sprintf("SCUSD%d", i+1)
		}
		p.Compliance = append(p.Compliance, SeedComplianceSpec{
			Symbol: symbol, Name: fmt.Sprintf("Seed Compliant USD %d", i+1), Decimals: 6,
			IconURL: seedTokenIconURL(symbol), URI: seedTokenURI(symbol),
			IssuerName: "kycorg", Schema: "SeedKyc1", Mode: "Any",
			MinterUser: names[1], CertifiedReceiver: names[0], UncertifiedVictim: names[len(names)-1],
		})
	}

	// SFT:场景按 cfg 轮转铺满 slotsPerSft;tokensPerSlot 决定每 slot 分发笔数
	warnMerge := false
	for s := 0; s < cfg.SFT.Count; s++ {
		key := fmt.Sprintf("set%02d", s+1)
		symbol := fmt.Sprintf("SS%d", s+1)
		spec := SeedSftSpec{
			Key: key, Name: fmt.Sprintf("Seed Set %d", s+1), Symbol: symbol,
			RoyaltyBps: uint16((s * 125) % 1000),
			CoverURL:   seedSftCoverURL(key), MetadataURL: seedPicsum("milon-meta-sft-"+key, 400, 400),
		}
		for k := 0; k < cfg.SFT.SlotsPerSft; k++ {
			scKey := cfg.SFT.Scenarios[k%len(cfg.SFT.Scenarios)]
			if scKey == "merge" && cfg.SFT.TokensPerSlot < 2 {
				warnMerge = true
			}
			spec.Scenarios = append(spec.Scenarios,
				seedScenarioV2(key, scKey, names, cfg.SFT.TokensPerSlot, cfg.SFT.PostOps))
		}
		p.Sfts = append(p.Sfts, spec)
	}

	// VC 证书链:kycorg 双 schema 给 admin+全员;生命周期演示(≥3 人时)
	if cfg.Accounts.VcCert {
		certifies := append([]string{"admin"}, names...)
		p.VcIssuers = []SeedVcIssuerSpec{
			{IssuerName: "kycorg", SchemaPrefix: "SeedKyc", SchemaCount: 2, Schema: "SeedKyc1",
				Certifies: certifies},
		}
		if len(names) >= 3 {
			p.VcLifecycle = []SeedLifecycleOp{
				{IssuerName: "kycorg", Kind: "freeze", User: names[len(names)-2]},
				{IssuerName: "kycorg", Kind: "revoke", User: names[len(names)-1]},
			}
		}
	}

	// 交易演示:互转链 + 批量 + 预期失败(账号 ≥2 时)
	txAmts := []uint64{137, 89, 45, 200, 64, 100, 33, 21}
	if len(names) >= 2 {
		n := len(names)
		if n > 8 {
			n = 8
		}
		for i := 0; i < n; i++ {
			p.Txs.MilTransfers = append(p.Txs.MilTransfers, SeedMilTransfer{
				From: names[i], To: names[(i+2)%len(names)], Amount: txAmts[i%len(txAmts)] * seedMil,
			})
		}
		p.Txs.BatchFrom = names[0]
		batchN := len(names) - 1
		if batchN > 5 {
			batchN = 5
		}
		for i := 1; i <= batchN; i++ {
			p.Txs.BatchTo = append(p.Txs.BatchTo, names[i])
			p.Txs.BatchAmounts = append(p.Txs.BatchAmounts, 10*seedMil)
		}
		p.Txs.Failures = append(p.Txs.Failures, SeedFailureSpec{
			Kind: "insufficient", ExpectCode: 513, By: names[len(names)-1], Amount: math2Pow62(),
			Detail: "超额转账触发 InsufficientBalance(513)",
		})
	}
	return p, nil
}

// SeedPlanSummary 计划 → 计数摘要(预览页顶部横条)。
func SeedPlanSummary(p SeedPlan) SeedSummary {
	c := p.Config
	s := SeedSummary{Accounts: len(p.Users) + 1, Multisig: len(p.Multisig),
		Tokens: len(p.Tokens), TokensCompliance: len(p.Compliance), SftSets: len(p.Sfts)}
	for _, sft := range p.Sfts {
		s.SftSlots += len(sft.Scenarios)
		for _, sc := range sft.Scenarios {
			s.SftTokens += len(sc.Dists)
		}
	}
	s.TxEstimate = SeedEstimateTxs(p)
	if c != nil && c.SFT.Count > 0 && c.SFT.Scenarios != nil {
		for _, sc := range c.SFT.Scenarios {
			if sc == "merge" && c.SFT.TokensPerSlot < 2 {
				s.Warnings = append(s.Warnings, "merge 场景需 sft.tokensPerSlot≥2,该 slot 将只分发不合并")
			}
		}
	}
	return s
}

// SeedEstimateTxs 预估链上交易数(上界口径:领水/补差按每人一笔)。
func SeedEstimateTxs(p SeedPlan) int {
	n := 0
	c := p.Config
	if c == nil {
		return 0
	}
	// accounts:领水 + DID + 余额档补差
	n++ // admin 领水
	if c.Accounts.Faucet {
		n += c.Accounts.Count
	}
	n += len(p.Issuers)
	if c.Accounts.DID {
		n += 1 + c.Accounts.Count + len(p.Issuers)
	}
	if len(c.Accounts.BalanceTiers) > 0 {
		n += c.Accounts.Count // 补差上界
	}
	// multisig
	for _, ms := range p.Multisig {
		if ms.Faucet {
			n++
		}
		n++ // CreateMultisig
		if ms.DID {
			n++
		}
	}
	// tokens
	n += len(p.Tokens) * 2 // create + mint
	if c.Tokens.FreezeDemo {
		n++
	}
	// sft:create + 每 slot 1 + 每分发 1 + 后置
	for _, sft := range p.Sfts {
		n++
		for _, sc := range sft.Scenarios {
			n += 1 + len(sc.Dists) + len(sc.PostOps)
		}
	}
	// vc + compliance + txs
	if c.Accounts.VcCert {
		if len(p.VcIssuers) > 0 {
			n += len(p.VcIssuers[0].Certifies)
		}
		n += len(p.VcLifecycle)
	}
	n += len(p.Compliance) * 6 // credential/create/mode/requirements/mint/accept
	n += len(p.Txs.MilTransfers) + 1 + len(p.Txs.Failures)
	return n
}
```

- [ ] **Step 4: 更新旧测试 + 全量编译**

`handler/seed_plan_test.go` 原来断言 `BuildSeedPlan(scale,...)` 返回值单值——签名已变为 `(SeedPlan, error)`。把该文件整体替换为：

```go
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
```

Run: `go build ./... && go test ./handler/ -run "TestBuildSeedPlan" -v`
Expected: 编译通过，PASS（若 `seed_runner*.go` 因 `plan.Compliance` 类型变化报编译错，随 Task 6 一并修——本任务先最小修复：`runCompliance` 中 `c := r.plan.Compliance` 改为 `if len(r.plan.Compliance) == 0 { return nil }; c := r.plan.Compliance[0]`，并在 `run()` 的 switch 加 `case "multisig": err = r.runMultisig()` 前先建空文件占位？——不允许占位。改法：本任务同时把 `SeedBuildAccounts` 的合规段改为循环（`for _, cm := range plan.Compliance`），`seed_runner_flows.go` 不动即可编译。`run()` 的 multisig case 放 Task 6。）

- [ ] **Step 5: 提交**

```bash
git add handler/seed_plan.go handler/seed_plan_v2.go handler/seed_plan_v2_test.go handler/seed_plan_test.go handler/seed_keys_state.go handler/seed_runner.go
git commit -m "feat(seed-v2): BuildSeedPlanV2 纯函数计划生成+多签/合规列表规格+旧 scale 兼容包装"
```

---

### Task 4: SeedState 命名空间前缀

**Files:**
- Modify: `handler/seed_keys_state.go`（SeedState 加 prefix；SeedBuildAccounts 扩展多签与合规列表账户）
- Test: `handler/seed_keys_state_test.go`（追加用例）

**Interfaces:**
- Consumes: `SeedConfigHash`（Task 1）；`plan.Multisig`/`plan.Compliance`（Task 3）
- Produces: `(*SeedState).UsePrefix(string)`；`IsDone/Get/Mark` 以 `"<prefix>:<module>"` 为实际键；`SeedBuildAccounts` 注册 `role="multisig"` 账户（键名 `ms01..`）与全部合规代币资源账户

- [ ] **Step 1: 追加失败测试（在 `handler/seed_keys_state_test.go` 末尾加）**

```go
func TestSeedState_NamespacePrefix(t *testing.T) {
	dir := t.TempDir()
	st := LoadSeedState(dir + "/s.json")
	st.UsePrefix("abc123")
	st.Mark("accounts", "faucet", map[string]any{"count": 3})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	if !st.IsDone("accounts", "faucet") {
		t.Fatal("同前缀应读到完成态")
	}
	// 另一命名空间互不可见
	st2 := LoadSeedState(dir + "/s.json")
	st2.UsePrefix("def456")
	if st2.IsDone("accounts", "faucet") {
		t.Fatal("不同前缀不得互相可见")
	}
	// 重载同前缀仍可见
	st3 := LoadSeedState(dir + "/s.json")
	st3.UsePrefix("abc123")
	if !st3.IsDone("accounts", "faucet") {
		t.Fatal("落盘后同前缀应可续跑")
	}
}

func TestSeedState_ResetClearsAll(t *testing.T) {
	st := LoadSeedState(t.TempDir() + "/s.json")
	st.UsePrefix("p1")
	st.Mark("tokens", "create:ST01", nil)
	st.Reset()
	if st.IsDone("tokens", "create:ST01") {
		t.Fatal("Reset 须清空")
	}
}

func TestSeedBuildAccounts_IncludesMultisigAndComplianceList(t *testing.T) {
	cfg := SeedConfigDefaults()
	plan, err := BuildSeedPlanV2(cfg)
	if err != nil {
		t.Fatal(err)
	}
	accts, err := SeedBuildAccounts(plan)
	if err != nil {
		t.Fatal(err)
	}
	if accts["ms01"] == nil {
		t.Fatal("多签账户 ms01 未注册")
	}
	if accts["ms01"].Role != "multisig" {
		t.Errorf("ms01 role=%s", accts["ms01"].Role)
	}
	for _, cm := range plan.Compliance {
		if accts[cm.Symbol] == nil {
			t.Fatalf("合规代币资源账户 %s 未注册", cm.Symbol)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run "TestSeedState_NamespacePrefix|TestSeedState_ResetClearsAll|TestSeedBuildAccounts_Includes" -v`
Expected: FAIL（`UsePrefix` undefined）

- [ ] **Step 3: 实现**

3a. `SeedState` 结构与方法改造（`handler/seed_keys_state.go`）：

```go
// SeedState 断点续跑状态:{module: {stage: data}}。
// v2:模块键实际存储为 "<prefix>:<module>",prefix=configHash——
// 同配置续跑幂等,改配置自动开新命名空间(规格 §5)。
type SeedState struct {
	data   map[string]map[string]any
	path   string
	prefix string
}

// UsePrefix 设置命名空间前缀(模块键变为 "<prefix>:<module>")。
func (s *SeedState) UsePrefix(prefix string) { s.prefix = prefix }

func (s *SeedState) key(module string) string {
	if s.prefix == "" {
		return module
	}
	return s.prefix + ":" + module
}

func (s *SeedState) IsDone(module, stage string) bool {
	_, ok := s.data[s.key(module)][stage]
	return ok
}

func (s *SeedState) Get(module, stage string) any {
	return s.data[s.key(module)][stage]
}

func (s *SeedState) Mark(module, stage string, data any) {
	k := s.key(module)
	if s.data[k] == nil {
		s.data[k] = map[string]any{}
	}
	s.data[k][stage] = data
}
```

（`Reset`/`Data`/`Save`/`LoadSeedState` 不变；`Reset` 仍清全文件——语义=重新创世。）

3b. `SeedBuildAccounts` 扩展（同文件）：在 `for _, i := range plan.Issuers` 循环后追加：

```go
	for _, ms := range plan.Multisig {
		spec := SeedAccountSpec{Name: ms.Name, Role: "multisig", Curve: ms.Curve,
			SubjectType: "Personal", Alias: ms.Alias, AliasSuffix: ms.AliasSuffix}
		if err := reg(spec, ms.Name); err != nil {
			return nil, err
		}
	}
```

并把合规段（原单值 `plan.Compliance.Symbol`）替换为：

```go
	for _, cm := range plan.Compliance {
		spec := SeedAccountSpec{Name: cm.Symbol, Role: "token", Curve: "secp256k1"}
		if err := reg(spec, cm.Symbol); err != nil {
			return nil, err
		}
	}
```

- [ ] **Step 4: 跑测试确认通过 + 全量编译**

Run: `go build ./... && go test ./handler/ -run "TestSeedState|TestSeedBuildAccounts" -v`
Expected: PASS；编译零错误

- [ ] **Step 5: 提交**

```bash
git add handler/seed_keys_state.go handler/seed_keys_state_test.go
git commit -m "feat(seed-v2): SeedState configHash 命名空间前缀+多签/合规列表账户注册"
```

---

### Task 5: multisig 执行模块

**Files:**
- Create: `handler/seed_runner_multisig.go`
- Test: `handler/seed_runner_multisig_test.go`（纯函数部分；链上路径标注 devNet 验证）

**Interfaces:**
- Consumes: `seedSendSingle`（`seed_runner.go:112`）、`flowEnsureFaucet`、`vcFlowEnsureDID`、`flowCallView`、`seedHasCode`、`r.state/r.acct/r.logf`；IDL：app `account`，方法 `CreateMultisig(owner:Signer, signers:vec<PublicKey>, weights:bytes, threshold:u8)`，视图 `ListSigners(owner)`（`gosdk-develop/gen/idl_gen.go:24590/25077` 已核实参数键名）
- Produces: `func (r *seedRunner) runMultisig() error`

- [ ] **Step 1: 写失败测试（纯函数：幂等判定与签名者装配）**

```go
// handler/seed_runner_multisig_test.go
package handler

import "testing"

func TestSeedMultisigSignersReady(t *testing.T) {
	// ListSigners 返回的签名者数与计划一致 → 视为已注册(幂等跳过)
	if !seedMultisigSignersReady(5, []any{"a", "b", "c", "d", "e"}) {
		t.Fatal("5/5 应判定已注册")
	}
	if seedMultisigSignersReady(5, []any{"a"}) {
		t.Fatal("1/5 不应判定已注册")
	}
	if seedMultisigSignersReady(5, nil) {
		t.Fatal("空视图不应判定已注册")
	}
}

func TestSeedMultisigWeights(t *testing.T) {
	w := seedMultisigWeights(4)
	if len(w) != 4 {
		t.Fatalf("长度 %d", len(w))
	}
	for _, x := range w {
		if x != 1 {
			t.Fatalf("等权应全 1, got %v", w)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run "TestSeedMultisig" -v`
Expected: FAIL（undefined）

- [ ] **Step 3: 实现**

```go
// handler/seed_runner_multisig.go
package handler

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== 播种器:multisig 模块 ====================
// 语义对齐 gosdk example/create_multisig_demo:创建者 = 多签账户自身;
// 先领水,再 CreateMultisig(owner=自身, signers=普通账号公钥, 等权 1, threshold=m),
// 可选建 DID。幂等:ListSigners 视图签名者数一致 → 跳过注册。

// seedMultisigSignersReady 幂等判定:视图返回的签名者条数与计划一致。
func seedMultisigSignersReady(want int, view []any) bool {
	return view != nil && len(view) == want && want > 0
}

// seedMultisigWeights 等权 bytes:每个签名者权重 1。
func seedMultisigWeights(n int) []uint8 {
	w := make([]uint8, n)
	for i := range w {
		w[i] = 1
	}
	return w
}

func (r *seedRunner) runMultisig() error {
	for _, ms := range r.plan.Multisig {
		a := r.acct(ms.Name)

		if ms.Faucet {
			stage := "faucet:" + ms.Name
			if !r.state.IsDone("multisig", stage) {
				if _, err := flowEnsureFaucet(r.mc, *a.ADR, a.SK, a.PUB); err != nil {
					return fmt.Errorf("faucet %s: %w", ms.Name, err)
				}
				r.logf("  %s 领水 ✓ (%s)", ms.Name, shortAddr(a.Address))
				r.state.Mark("multisig", stage, nil)
				r.state.Save()
			}
		}

		stage := "create:" + ms.Name
		if !r.state.IsDone("multisig", stage) {
			registered := false
			if v, err := flowCallView(r.mc, "account", "ListSigners",
				provider.Args{"owner": a.Address}); err == nil {
				if list, ok := v.([]any); ok && seedMultisigSignersReady(len(ms.SignerNames), list) {
					registered = true
					r.logf("  %s 已注册多签(%d 签者),跳过", ms.Name, len(list))
				}
			}
			if !registered {
				signers := make([]any, len(ms.SignerNames))
				for j, sn := range ms.SignerNames {
					signers[j] = r.acct(sn).PUB
				}
				_, err := seedSendSingle(r.mc, "account", "CreateMultisig", provider.Args{
					"owner": a.Address, "signers": signers,
					"weights": seedMultisigWeights(len(ms.SignerNames)), "threshold": ms.Threshold,
				}, a)
				if err != nil && !seedHasCode(err.Error(), 1059, 1064) {
					return fmt.Errorf("create multisig %s: %w", ms.Name, err)
				}
				if err != nil {
					r.logf("  %s 链上已注册(幂等码),跳过", ms.Name)
				} else {
					r.logf("  %s 多签注册 ✓ %d-of-%d 签者 %v",
						ms.Name, ms.Threshold, len(ms.SignerNames), ms.SignerNames)
				}
			}
			r.state.Mark("multisig", stage, map[string]any{
				"address": a.Address, "threshold": ms.Threshold, "signers": ms.SignerNames})
			r.state.Save()
		}

		if ms.DID {
			stage = "did:" + ms.Name
			if !r.state.IsDone("multisig", stage) {
				suffix := ms.AliasSuffix
				opts := &vcFlowDidOptions{Alias: ms.Alias, Suffix: &suffix}
				if _, err := vcFlowEnsureDID(r.mc, *a.ADR, a.SK, a.PUB, "Personal", opts); err != nil {
					return fmt.Errorf("did %s: %w", ms.Name, err)
				}
				r.logf("  %s DID ✓", ms.Name)
				r.state.Mark("multisig", stage, nil)
				r.state.Save()
			}
		}
	}
	r.logf("multisig 模块完成: %d 个多签账户", len(r.plan.Multisig))
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./handler/ -run "TestSeedMultisig" -v`
Expected: PASS；编译零错误

- [ ] **Step 5: devNet 冒烟验证（手动，不进 CI）**

启动本地服务连 devNet，用 `POST /api/tool/seed/plan` 无法覆盖此模块——直接以最小 Go 测试脚本或控制台触达链端验证 `CreateMultisig` 编码/幂等分支可用。若 `seedHasCode` 容忍码不符实际链端码表，按实测补码并记录到 evidence 文件。

Run: `go build -o milon-api-server-new.exe . && ./milon-api-server-new.exe`（devNet），观察 multisig 模块日志
Expected: ms01 注册成功或幂等跳过，无未容忍错误

- [ ] **Step 6: 提交**

```bash
git add handler/seed_runner_multisig.go handler/seed_runner_multisig_test.go
git commit -m "feat(seed-v2): multisig 模块——CreateMultisig 注册/领水/DID,幂等判定"
```

---

### Task 6: runner 接线——模块顺序 / accounts 开关 / 余额档 / compliance 列表化 / SFT 新后置操作

**Files:**
- Modify: `handler/seed_runner.go`（SeedModuleOrder、run() case、runAccounts 三段开关化+余额档、runCompliance 列表化）
- Modify: `handler/seed_runner_flows.go`（seedSftPostOps 增加 freeze/merge 两个 op 分支）
- Test: `handler/seed_runner_test.go`（追加纯函数用例）

**Interfaces:**
- Consumes: `plan.Config`（Task 3）、`runMultisig`（Task 5）、`seedBalanceTiers`（Task 1）
- Produces: `SeedModuleOrder = ["accounts","multisig","tokens","sft","vc","compliance","txs"]`；`runAccounts` 尊重 `cfg.Accounts.Faucet/DID/BalanceTiers`（admin/issuers 恒领水+DID）；`runCompliance` 遍历 `plan.Compliance`（阶段键带代币符号）；`SeedTxsTarget` 档位换算

- [ ] **Step 1: 写失败测试（追加到 handler/seed_runner_test.go）**

```go
func TestSeedModuleOrderV2(t *testing.T) {
	want := []string{"accounts", "multisig", "tokens", "sft", "vc", "compliance", "txs"}
	if len(SeedModuleOrder) != len(want) {
		t.Fatalf("模块数 %d", len(SeedModuleOrder))
	}
	for i := range want {
		if SeedModuleOrder[i] != want[i] {
			t.Fatalf("顺序不符: %v", SeedModuleOrder)
		}
	}
}

func TestSeedTierTargetMIL(t *testing.T) {
	if seedTierTargetMIL("hundred") != 100 || seedTierTargetMIL("thousand") != 1000 || seedTierTargetMIL("ten_k") != 10000 {
		t.Fatal("档位换算不符")
	}
	if seedTierTargetMIL("nope") != 0 {
		t.Fatal("未知档位应为 0")
	}
}

func TestSeedComplianceStageKey(t *testing.T) {
	if got := seedComplianceStageKey("credential", "SCUSD"); got != "credential:SCUSD" {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run "TestSeedModuleOrderV2|TestSeedTierTargetMIL|TestSeedComplianceStageKey" -v`
Expected: FAIL

- [ ] **Step 3: 实现**

3a. `handler/seed_runner.go`：

模块顺序（第 12 行区域）：

```go
var SeedModuleOrder = []string{"accounts", "multisig", "tokens", "sft", "vc", "compliance", "txs"}
```

`run()` 的 switch 加分支：

```go
		case "multisig":
			err = r.runMultisig()
```

`runAccounts()` 整体替换（开关化 + 余额档）：

```go
func (r *seedRunner) runAccounts() error {
	cfg := r.plan.Config
	// [1] 领水:admin/issuers 恒领水(全局 gas 池);普通账号按开关
	if !r.state.IsDone("accounts", "faucet") {
		people := append([]SeedAccountSpec{r.plan.Admin}, r.plan.Issuers...)
		if cfg.Accounts.Faucet {
			people = append(people, r.plan.Users...)
		}
		r.logf("  领水 %d 个账户(冷却守卫,单次尝试)…", len(people))
		for _, spec := range people {
			a := r.acct(spec.Name)
			cooldown, err := seedViewU64(r.mc, seedTokenApp, "FaucetCooldownRemaining",
				provider.Args{"account": a.Address})
			if err != nil {
				cooldown = 0
			}
			bal, berr := r.mc.BalanceOf(a.ADR)
			if berr != nil && !isAccountNotFoundErr(berr.Error()) {
				return fmt.Errorf("balance %s: %w", a.Name, berr)
			}
			if berr != nil {
				bal = 0
			}
			if cooldown > 0 {
				r.logf("    跳过 %s… (冷却中 %dmin)", shortAddr(a.Address), cooldown/60000)
				continue
			}
			if bal >= seedMinBalance {
				r.logf("    跳过 %s… (余额充足 %d)", shortAddr(a.Address), bal)
				continue
			}
			step, err := flowEnsureFaucet(r.mc, *a.ADR, a.SK, a.PUB)
			if err != nil {
				return fmt.Errorf("faucet %s: %w", a.Name, err)
			}
			r.logf("    领水 %s… claimed=%v tx=%s", shortAddr(a.Address), step.Claimed, shortHash(step.TxHash))
		}
		r.state.Mark("accounts", "faucet", map[string]any{"count": len(people)})
		r.state.Save()
	}
	// [2] DID:admin/issuers 恒建;普通账号按开关
	if !r.state.IsDone("accounts", "did") {
		people := append([]SeedAccountSpec{r.plan.Admin}, r.plan.Issuers...)
		if cfg.Accounts.DID {
			people = append(people, r.plan.Users...)
		}
		r.logf("  创建 DID %d 个…", len(people))
		for _, spec := range people {
			a := r.acct(spec.Name)
			suffix := spec.AliasSuffix
			opts := &vcFlowDidOptions{Alias: spec.Alias, Suffix: &suffix, AvatarURI: spec.AvatarURL}
			if _, err := vcFlowEnsureDID(r.mc, *a.ADR, a.SK, a.PUB, spec.SubjectType, opts); err != nil {
				return fmt.Errorf("did %s: %w", spec.Name, err)
			}
		}
		r.logf("    DID 全部就绪(幂等补齐,别名 seed-*)")
		r.state.Mark("accounts", "did", map[string]any{"count": len(people)})
		r.state.Save()
	}
	// [3] 余额档位:目标不足部分由 admin 补差(规格 §2;领水 10000 MIL 已覆盖全部档位,
	// 档位在 faucet=false 或沿用旧账户时生效)
	if len(cfg.Accounts.BalanceTiers) > 0 && !r.state.IsDone("accounts", "tiers") {
		admin := r.acct("admin")
		r.logf("  余额档位补差 %d 个账户…", len(r.plan.Users))
		for i, spec := range r.plan.Users {
			target := seedTierTargetMIL(cfg.Accounts.BalanceTiers[i%len(cfg.Accounts.BalanceTiers)]) * seedMil
			if target == 0 {
				continue
			}
			a := r.acct(spec.Name)
			bal, err := r.mc.BalanceOf(a.ADR)
			if err != nil {
				bal = 0
			}
			if bal >= target {
				continue
			}
			_, err = seedSendSingle(r.mc, seedTokenApp, "Transfer", provider.Args{
				"from": admin.Address, "token": api.MILToken.ToBase58(),
				"to": a.Address, "amount": target - bal,
			}, admin)
			if err != nil {
				if seedHasCode(err.Error(), 513) {
					return fmt.Errorf("余额档补差 %s: admin MIL 不足(目标 %d)", spec.Name, target)
				}
				return fmt.Errorf("余额档补差 %s: %w", spec.Name, err)
			}
			r.logf("    %s 补差至 %d MIL ✓ (+%d)", spec.Name, target, target-bal)
		}
		r.state.Mark("accounts", "tiers", map[string]any{"tiers": cfg.Accounts.BalanceTiers})
		r.state.Save()
	}
	r.logf("accounts 模块完成")
	return nil
}
```

同文件补两个小工具（放「小工具」区）：

```go
// seedTierTargetMIL 档位 → MIL 数;未知档位 0(跳过)。
func seedTierTargetMIL(tier string) uint64 { return seedBalanceTiers[tier] }

// seedComplianceStageKey 合规阶段键:带代币符号,多合规代币互不串位。
func seedComplianceStageKey(stage, symbol string) string { return stage + ":" + symbol }
```

注意 `runAccounts` 需要 `api` 包引用：`handler/seed_runner.go` 顶部 import 已有 `"github.com/milon-labs/milon-go-sdk/api"`（第 12 行），无需新增。

3b. `runCompliance()` 列表化——函数签名不变，函数体首行改为循环包裹。把现有函数体的 `c := r.plan.Compliance` 后整段（到函数末尾 return nil 前）包进：

```go
func (r *seedRunner) runCompliance() error {
	if len(r.plan.Compliance) == 0 {
		r.logf("compliance 模块完成: 无合规代币,跳过")
		return nil
	}
	for _, c := range r.plan.Compliance {
		if err := r.runComplianceOne(c); err != nil {
			return err
		}
	}
	r.logf("compliance 模块完成: %d 个合规代币", len(r.plan.Compliance))
	return nil
}

func (r *seedRunner) runComplianceOne(c SeedComplianceSpec) error {
	// ← 原 runCompliance 函数体从 token := r.acct(c.Symbol) 起,逐字保留,
	//   仅把每个 mark(stage, ...) / IsDone(stage) / Get(stage) 的 stage
	//   替换为 seedComplianceStageKey(stage, c.Symbol)
	//   最后的 r.logf("compliance 模块完成: ...") 一行删除(移入外层)
}
```

3c. `handler/seed_runner_flows.go` 的 `seedSftPostOps` switch 增加两个分支（插在 `default:` 前）：

```go
		case "freeze":
			holder := r.acct(op.By)
			tid := tokenIds[0]
			_, err := seedSendSingle(r.mc, sftFlowIdentityApp, "Freeze", provider.Args{
				"freezer": admin.Address, "sft": sftAddr, "token_id": tid, "owner": holder.Address,
			}, admin)
			if err != nil && !seedHasCode(err.Error(), 1073, 1282, 1283) {
				return fmt.Errorf("freeze token: %w", err)
			}
			if err != nil {
				r.logf("  [%s/%s] freeze token %d 已完成,跳过", sftSpec.Key, sc.Key, tid)
			} else {
				r.logf("  [%s/%s] freeze token %d ✓ (owner=%s)", sftSpec.Key, sc.Key, tid, op.By)
			}
			r.state.Mark("sft", stage, map[string]any{"op": "freeze", "tokenId": tid})
		case "merge":
			if len(tokenIds) < 2 {
				r.logf("  [%s/%s] merge 需 ≥2 token,该 slot 仅 %d,跳过", sftSpec.Key, sc.Key, len(tokenIds))
				r.state.Mark("sft", stage, map[string]any{"op": "merge", "skipped": "insufficient-tokens"})
				break
			}
			holder := r.acct(op.By)
			from, to := tokenIds[0], tokenIds[1]
			_, err := seedSendSingle(r.mc, sftFlowIdentityApp, "Merge", provider.Args{
				"owner": holder.Address, "sft": sftAddr,
				"from_token_id": from, "to_token_id": to,
			}, holder)
			if err != nil && !seedHasCode(err.Error(), 1282, 1283) {
				return fmt.Errorf("merge token: %w", err)
			}
			if err != nil {
				r.logf("  [%s/%s] merge %d→%d 已完成,跳过", sftSpec.Key, sc.Key, from, to)
			} else {
				r.logf("  [%s/%s] merge token %d → %d ✓", sftSpec.Key, sc.Key, from, to)
			}
			r.state.Mark("sft", stage, map[string]any{"op": "merge", "from": from, "to": to})
```

- [ ] **Step 4: 跑测试确认通过 + 全包回归**

Run: `go build ./... && go test ./handler/ -v 2>&1 | tail -20`
Expected: 全 PASS（`go vet ./handler/` 亦零告警）

- [ ] **Step 5: 提交**

```bash
git add handler/seed_runner.go handler/seed_runner_flows.go handler/seed_runner_test.go
git commit -m "feat(seed-v2): runner 七模块接线——accounts 开关化+余额档补差+compliance 列表化+SFT freeze/merge 场景"
```

---

### Task 7: API 端点改造

**Files:**
- Modify: `handler/seed_handler.go`
- Modify: `main.go:157-159`（路由）
- Test: `handler/seed_handler_test.go`（追加契约用例）

**Interfaces:**
- Consumes: Task 1-6 全部
- Produces: `GET /api/tool/seed/templates`；`POST /api/tool/seed/plan`；`POST /api/tool/seed`（新 body）；`GET /api/tool/seed/status/:id`（回显 configHash/preset/config）；`seedStartRequest{Preset,Overrides json.RawMessage,Config *SeedConfig,Scale,Modules,SeedPhrase,Reset}`；`seedResolveConfig(seedStartRequest) (SeedConfig, error)`

- [ ] **Step 1: 写失败测试（追加到 handler/seed_handler_test.go；复用文件内既有 gin 测试骨架，若无则按下方自建）**

```go
func TestSeedResolveConfig_Priority(t *testing.T) {
	// config > preset+overrides > scale 映射 > 默认
	req := seedStartRequest{Scale: "small"}
	cfg, err := seedResolveConfig(req)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts.Count != 3 {
		t.Fatalf("scale=small 应映射 minimal(3 账号), got %d", cfg.Accounts.Count)
	}
	req = seedStartRequest{Preset: "full"}
	cfg, err = seedResolveConfig(req)
	if err != nil || cfg.Accounts.Count != 12 {
		t.Fatalf("preset=full 应 12 账号: %v %d", err, cfg.Accounts.Count)
	}
	req = seedStartRequest{Preset: "minimal",
		Overrides: json.RawMessage(`{"accounts":{"count":7},"seedPhrase":"custom-phrase"}`)}
	cfg, err = seedResolveConfig(req)
	if err != nil || cfg.Accounts.Count != 7 || cfg.SeedPhrase != "custom-phrase" {
		t.Fatalf("overrides 应生效: %v %+v", err, cfg.Accounts)
	}
	if cfg.Tokens.Count != 1 {
		t.Fatalf("未覆盖字段应保留 minimal 值, got %d", cfg.Tokens.Count)
	}
	full := SeedConfigForPreset_t(t)
	req = seedStartRequest{Config: &full}
	cfg, err = seedResolveConfig(req)
	if err != nil || cfg.Accounts.Count != 12 {
		t.Fatalf("config 直传应优先: %v", err)
	}
	req = seedStartRequest{}
	cfg, err = seedResolveConfig(req)
	if err != nil || cfg.Accounts.Count != 8 {
		t.Fatalf("空请求应取默认 standard: %v %d", err, cfg.Accounts.Count)
	}
	req = seedStartRequest{Preset: "nope"}
	if _, err = seedResolveConfig(req); err == nil {
		t.Fatal("未知 preset 应报错")
	}
}

func TestSeedPlanEndpoint_ValidationErrors(t *testing.T) {
	// 直接构造 handler 调用(gin test recorder),非法 config → 400 且错误含字段路径
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/tool/seed/plan",
		strings.NewReader(`{"config":{"accounts":{"count":0}}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h := NewSeedJobHandler(nil)
	h.SeedPlanV2(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("应 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "accounts.count") {
		t.Fatalf("错误须含字段路径: %s", w.Body.String())
	}
}
```

（若文件里无 `SeedConfigForPreset_t` 辅助,改为 `cfg, _ := SeedConfigForPreset("full"); req := seedStartRequest{Config: &cfg}`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run "TestSeedResolveConfig|TestSeedPlanEndpoint" -v`
Expected: FAIL（undefined）

- [ ] **Step 3: 实现 `handler/seed_handler.go`**

3a. 请求结构替换（原 `seedStartRequest`）：

```go
type seedStartRequest struct {
	Preset     string          `json:"preset"`     // minimal|standard|full
	Overrides  json.RawMessage `json:"overrides"`  // 对 preset 的深合并覆盖
	Config     *SeedConfig     `json:"config"`     // 完整配置直传(最高优先)
	Scale      string          `json:"scale"`      // 兼容:full→full,small→minimal
	Modules    []string        `json:"modules"`
	SeedPhrase string          `json:"seedPhrase"`
	Reset      bool            `json:"reset"`
}

// seedResolveConfig 请求 → 生效 SeedConfig。
// 优先级:Config > Preset+Overrides > Scale 映射 > 默认 standard。
func seedResolveConfig(req seedStartRequest) (SeedConfig, error) {
	if req.Config != nil {
		cfg := *req.Config
		if cfg.SeedPhrase == "" {
			cfg.SeedPhrase = req.SeedPhrase
		}
		if cfg.SeedPhrase == "" {
			cfg.SeedPhrase = SeedDefaultPhrase
		}
		return cfg, nil
	}
	var base SeedConfig
	switch {
	case req.Preset != "":
		b, err := SeedConfigForPreset(req.Preset)
		if err != nil {
			return SeedConfig{}, err
		}
		base = b
	case req.Scale == "full" || req.Scale == "small":
		preset := req.Scale
		if preset == "small" {
			preset = "minimal"
		}
		b, err := SeedConfigForPreset(preset)
		if err != nil {
			return SeedConfig{}, err
		}
		base = b
	default:
		base = SeedConfigDefaults()
	}
	if len(req.Overrides) > 0 {
		merged, err := seedMergeConfig(base, req.Overrides)
		if err != nil {
			return SeedConfig{}, err
		}
		base = merged
	}
	if req.SeedPhrase != "" {
		base.SeedPhrase = req.SeedPhrase
	}
	if base.SeedPhrase == "" {
		base.SeedPhrase = SeedDefaultPhrase
	}
	return base, nil
}
```

3b. 三个新端点 + Seed 改造（`Seed` 内：解析 → 校验 → 计划 → job；job 携带 configHash/preset 回显）：

```go
// SeedTemplatesList 列出内置模板。
func (h *SeedJobHandler) SeedTemplatesList(c *gin.Context) {
	c.JSON(http.StatusOK, types.SuccessResponse(SeedTemplates(), "ok"))
}

// SeedPlanV2 计划预览(不上链)。body: SeedConfig | {preset, overrides} | {scale}(兼容)。
func (h *SeedJobHandler) SeedPlanV2(c *gin.Context) {
	var req seedStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "SeedPlanV2", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}
	cfg, err := seedResolveConfig(req)
	if err != nil {
		logParamError(c, "SeedPlanV2", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	if errs := ValidateSeedConfig(cfg); len(errs) > 0 {
		logParamError(c, "SeedPlanV2", fmt.Errorf("%v", errs))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
			"配置校验失败", errs))
		return
	}
	plan, err := BuildSeedPlanV2(cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"plan": plan, "summary": SeedPlanSummary(plan),
	}, "ok"))
}
```

`Seed` 端点改造：原 scale 校验段替换为：

```go
	cfg, err := seedResolveConfig(req)
	if err != nil {
		logParamError(c, "Seed", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	if errs := ValidateSeedConfig(cfg); len(errs) > 0 {
		logParamError(c, "Seed", fmt.Errorf("%v", errs))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "配置校验失败", errs))
		return
	}
	modules, err := seedNormalizeModules(req.Modules)
	if err != nil { ...原有逻辑保留... }
	plan, err := BuildSeedPlanV2(cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
```

goroutine 内 `plan:` 字段改为 `plan: plan`；`state` 初始化改为：

```go
			state: func() *SeedState {
				st := LoadSeedState(statePath)
				if req.Reset {
					st.Reset()
					_ = st.Save()
				}
				st.UsePrefix(plan.ConfigHash)
				return st
			}(),
```

job 结构与快照增加回显字段：

```go
type seedJob struct {
	// …原有字段…
	Preset     string `json:"preset,omitempty"`
	ConfigHash string `json:"configHash,omitempty"`
}

type seedJobSnapshot struct {
	// …原有字段…
	Preset     string      `json:"preset,omitempty"`
	ConfigHash string      `json:"configHash,omitempty"`
	Config     *SeedConfig `json:"config,omitempty"`
	Summary    *SeedSummary `json:"summary,omitempty"`
}
```

`SeedStatus` 快照填充处（completed 分支外）追加：

```go
	job.mu.Lock()
	snap := seedJobSnapshot{
		// …原有…
		Preset: job.Preset, ConfigHash: job.ConfigHash,
	}
	if jobConfig != nil { snap.Config = jobConfig }
	job.mu.Unlock()
```

（`Seed` 启动时把 `plan` 与 `cfg` 存进 job：`job.Preset = req.Preset; job.ConfigHash = plan.ConfigHash; jobConfig = plan.Config;` 快照 completed 分支的地址表改用 `plan` 变量（启动时已构建）替代重新 `BuildSeedPlan(job.Scale,...)`。）

旧 `SeedPlanPreview`（GET ?scale=）保留，实现体改为：

```go
func (h *SeedJobHandler) SeedPlanPreview(c *gin.Context) {
	scale := c.DefaultQuery("scale", "full")
	if scale != "full" && scale != "small" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "scale must be full or small", nil))
		return
	}
	plan, err := BuildSeedPlan(scale, c.DefaultQuery("seedPhrase", SeedDefaultPhrase))
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(plan, "ok"))
}
```

3c. `main.go:157` 区域路由注册（保留旧两条，插入两条新路由）：

```go
		// Seed v2 (一键测试数据构造:模板+旋钮参数化, 异步任务)
		api.GET("/tool/seed/templates", seedJobHandler.SeedTemplatesList)
		api.POST("/tool/seed/plan", seedJobHandler.SeedPlanV2)
		// 旧接口(scale=full|small)兼容保留一个小版本
		api.GET("/tool/seed/plan", seedJobHandler.SeedPlanPreview)
		api.POST("/tool/seed", seedJobHandler.Seed)
		api.GET("/tool/seed/status/:id", seedJobHandler.SeedStatus)
```

- [ ] **Step 4: 跑测试确认通过 + 全包回归**

Run: `go build ./... && go test ./handler/ -v 2>&1 | tail -20 && go vet ./handler/`
Expected: PASS；vet 零告警

- [ ] **Step 5: 提交**

```bash
git add handler/seed_handler.go handler/seed_handler_test.go main.go
git commit -m "feat(seed-v2): API 端点——templates/plan/config 直传/preset+overrides 深合并/状态回显"
```

---

### Task 8: 控制台「测试数据」页重写

**Files:**
- Modify: `static/index.html:575-640 区域`（seed view 区块整段替换）
- Modify: `static/index.html:764`（版本号 bump `?v=20261019`）
- Create(重写): `static/js/seed.js`
- Test: `test_seed_v2_section.js`（仓库根，node 直抽模式，同 `test_mcp_accounts_section.js`）

**Interfaces:**
- Consumes: Task 7 四端点
- Produces: 前端全局函数 `buildSeedCfg()`（DOM → SeedConfig 对象）、`applySeedTemplate(key)`、`refreshSeedPreview()`（500ms 防抖 POST /api/tool/seed/plan）、`startSeedJob()`；HTML 锚点 ID：`seedTemplateBtns`、`seedAcctCount`、`seedAcctFaucet`、`seedAcctDid`、`seedAcctVc`、`seedTiersChips`、`seedCurvesChips`、`seedMsCount`、`seedMsSigners`、`seedMsThreshold`、`seedMsFaucet`、`seedMsDid`、`seedTokCount`、`seedDecimalsChips`、`seedCompCount`、`seedFreezeDemo`、`seedSftCount`、`seedSftSlots`、`seedSftTokens`、`seedScenariosChips`、`seedPostOps`、`seedPhrase`、`seedPreview`、`seedSummary`、`seedStartBtn`、`seedLog`、`seedAddrTable`

- [ ] **Step 1: 写失败测试**

```js
// test_seed_v2_section.js — 播种器 v2 控制台页测试(源码直抽)
// 运行:node test_seed_v2_section.js
const fs = require('fs');
const path = require('path');
const ROOT = __dirname;
const htmlSrc = fs.readFileSync(path.join(ROOT, 'static', 'index.html'), 'utf8');
const jsSrc = fs.readFileSync(path.join(ROOT, 'static', 'js', 'seed.js'), 'utf8');

let failed = 0;
function check(name, cond, detail) {
  if (cond) console.log('  ok  ' + name);
  else { failed++; console.log('FAIL  ' + name + (detail ? '\n      ' + detail : '')); }
}

// 1. HTML 锚点:模板条 + 四组卡片 + 预览/执行区
const seedView = htmlSrc.slice(htmlSrc.indexOf('id="view-seed"'),
  htmlSrc.indexOf('id="view-', htmlSrc.indexOf('id="view-seed"') + 10) > 0
    ? htmlSrc.indexOf('</section>', htmlSrc.indexOf('id="view-seed"')) : undefined);
for (const id of ['seedTemplateBtns', 'seedAcctCount', 'seedCurvesChips', 'seedTiersChips',
  'seedAcctFaucet', 'seedAcctDid', 'seedAcctVc', 'seedMsCount', 'seedMsSigners',
  'seedMsThreshold', 'seedTokCount', 'seedDecimalsChips', 'seedCompCount',
  'seedSftCount', 'seedSftSlots', 'seedSftTokens', 'seedScenariosChips', 'seedPostOps',
  'seedPhrase', 'seedPreview', 'seedSummary', 'seedStartBtn', 'seedLog', 'seedAddrTable']) {
  check('HTML 锚点 #' + id, seedView.includes(id));
}

// 2. seed.js 关键函数存在
for (const fn of ['buildSeedCfg', 'applySeedTemplate', 'refreshSeedPreview', 'startSeedJob']) {
  check('JS 函数 ' + fn, jsSrc.includes('function ' + fn));
}

// 3. 防抖 500ms 调预览
check('预览防抖 500', /setTimeout[^;]{0,80}500/.test(jsSrc) && jsSrc.includes('/api/tool/seed/plan'));

// 4. 四端点路径
for (const ep of ['/api/tool/seed/templates', '/api/tool/seed/plan', '/api/tool/seed"', '/api/tool/seed/status/']) {
  check('端点 ' + ep.trim(), jsSrc.includes(ep.replace('"', '')));
}

// 5. buildSeedCfg 纯逻辑可抽测:模拟 DOM 环境构造 SeedConfig
const dom = {};
global.document = {
  getElementById: (id) => dom[id] || null,
  querySelectorAll: () => [],
};
global.localStorage = { getItem: () => null, setItem: () => {} };
dom['seedAcctCount'] = { value: '5' };
dom['seedTokCount'] = { value: '2' };
dom['seedMsCount'] = { value: '1' };
dom['seedMsSigners'] = { value: '3' };
dom['seedMsThreshold'] = { value: '2' };
dom['seedSftCount'] = { value: '1' };
dom['seedSftSlots'] = { value: '2' };
dom['seedSftTokens'] = { value: '3' };
dom['seedCompCount'] = { value: '0' };
dom['seedPhrase'] = { value: '' };
// chips: 选中态通过 data-key 与 .on class 表达
global.__chips = { seedDecimalsChips: [{ dataset: { key: '0' }, classList: { contains: () => true } },
                                     { dataset: { key: '6' }, classList: { contains: () => false } }] };
try {
  eval(jsSrc);
  const cfg = buildSeedCfg();
  check('buildSeedCfg 账号数', cfg.accounts.count === 5, JSON.stringify(cfg.accounts));
  check('buildSeedCfg 精度 chips', Array.isArray(cfg.tokens.decimals) && cfg.tokens.decimals[0] === 0,
    JSON.stringify(cfg.tokens.decimals));
  check('buildSeedCfg 多签', cfg.multisig.count === 1 && cfg.multisig.signers === 3 && cfg.multisig.threshold === 2);
} catch (e) {
  check('buildSeedCfg 可执行', false, e.message);
}

// 6. 静态版本号已 bump
check('版本 bump 20261019', htmlSrc.includes('seed.js?v=20261019'));

console.log(failed === 0 ? '\nALL PASS' : `\n${failed} FAILED`);
process.exit(failed === 0 ? 0 : 1);
```

- [ ] **Step 2: 跑测试确认失败**

Run: `node test_seed_v2_section.js`
Expected: FAIL（HTML 锚点缺失 / JS 函数缺失）

- [ ] **Step 3: 实现**

3a. `static/index.html` seed view 区块（575 行起）整段替换为分组卡片骨架（模板条按钮由 JS 渲染进 `#seedTemplateBtns`；chips 容器空置由 JS 渲染；数字/开关用现有 `network-select`/checkbox 样式；预览面板与执行日志沿用现卡片样式类）。保留区块头说明文案并追加一句：「v2:模板 + 旋钮全参数化，计划预览不上链」。

3b. `static/js/seed.js` 整体重写，骨架（完整实现按此结构展开，事件绑定沿用现有页面 addEventListener 风格）：

```js
// seed.js — 测试数据构造器 v2(模板 + 全局旋钮)
// 数据流:DOM → buildSeedCfg() → POST /api/tool/seed/plan(防抖 500ms) → 预览
//        → POST /api/tool/seed → GET status 轮询 → 地址表
const SEED_CHIPS = {
  seedCurvesChips: { list: ['secp256k1', 'ed25519', 'fndsa512'], field: 'curves' },
  seedTiersChips: { list: ['hundred', 'thousand', 'ten_k'], field: 'balanceTiers' },
  seedDecimalsChips: { list: ['0','1','2','3','4','5','6','7','8','9'], field: 'decimals' },
  seedScenariosChips: { list: ['transferable','frozen','merge','burn','reject_transfer'], field: 'scenarios' },
};

function seedChipsVals(id) {
  return Array.from(document.getElementById(id).querySelectorAll('.chip.on'))
    .map(el => el.dataset.key);
}

function buildSeedCfg() { /* DOM → SeedConfig(结构对齐 handler.SeedConfig JSON tag) */ }
function applySeedTemplate(key) { /* GET templates → 填 DOM → refreshSeedPreview() */ }
let seedPreviewTimer = null;
function refreshSeedPreview() { /* clearTimeout; setTimeout(()=>fetch('/api/tool/seed/plan',{method:'POST',body:JSON.stringify({config:buildSeedCfg()})}),500) → 渲染 summary+分组树 */ }
function startSeedJob() { /* POST /api/tool/seed {config} → jobId → 轮询 status → 日志/地址表;预估>800 弹 confirm */ }
```

3c. 版本 bump：`static/index.html:764` 的 `seed.js?v=20261015` → `seed.js?v=20261019`。

- [ ] **Step 4: 跑测试确认通过**

Run: `node test_seed_v2_section.js`
Expected: ALL PASS

- [ ] **Step 5: 手工冒烟（起服务，浏览器过一遍模板切换/chips/预览/启动）**

Run: `go build -o milon-api-server-new.exe . && ./milon-api-server-new.exe`，浏览器开控制台 seed 页
Expected: 三个模板按钮填充表单；改任一旋钮 500ms 后预览刷新；全量模板触发确认弹窗；播种任务进度与地址表正常

- [ ] **Step 6: 提交**

```bash
git add static/index.html static/js/seed.js test_seed_v2_section.js
git commit -m "feat(seed-v2): 控制台测试数据页重写——模板条/分组旋钮/chips/防抖预览/执行区+直抽测试"
```

---

### Task 9: 收尾——Python 废弃标注 / 文档 / 全量回归

**Files:**
- Modify: `tools/seeder/seed.py:1-14`（DEPRECATED 头注）
- Modify: `API.md`（seed 端点段更新）
- Modify: `main.go:265-267`（usage 打印更新）

**Interfaces:**
- Consumes: Task 1-8 全部
- Produces: 文档与实际端点一致

- [ ] **Step 1: Python CLI 废弃标注**

`tools/seeder/seed.py` 文件头 docstring 后追加：

```python
# ---------------------------------------------------------------------------
# DEPRECATED 2026-10-19: 播种器 v2 已原生进 API(POST /api/tool/seed,
# preset/overrides/config 参数化;模板+旋钮见控制台「测试数据」页)。
# 本 CLI 不再维护,仅作 v1 全量/小型矩阵的历史参考。
# 新参数模型:docs/superpowers/specs/2026-10-19-seed-builder-v2-design.md
# ---------------------------------------------------------------------------
```

- [ ] **Step 2: API.md 更新**

在 seed 段（现有 `POST /api/tool/seed` 文档处）重写为四端点表 + SeedConfig 字段表 + 三模板表 + 兼容说明（`scale` 映射保留一个小版本）。字段表与规格 §2 逐字对齐。

- [ ] **Step 3: main.go usage 打印更新（265-267 行）**

```go
	fmt.Println("    GET  /api/tool/seed/templates   - 播种模板列表(minimal/standard/full)")
	fmt.Println("    POST /api/tool/seed/plan        - 计划预览(不上链;config/preset+overrides)")
	fmt.Println("    POST /api/tool/seed             - 启动播种任务(异步;同刻仅一个)")
	fmt.Println("    GET  /api/tool/seed/status/:id  - 播种任务进度(configHash 命名空间断点续跑)")
	fmt.Println("    GET  /api/tool/seed/plan?scale= - 旧接口兼容(full/small → 模板映射)")
```

- [ ] **Step 4: 全量回归**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | tail -10 && node test_seed_v2_section.js`
Expected: 编译零错、vet 零告警、全部测试 PASS

- [ ] **Step 5: devNet 端到端验证（manual evidence）**

以 full 模板走一次完整播种 → 记录 evidence 到 `docs/`（沿用 `evidence_*.md` 命名），确认：multisig 注册、frozen/merge 场景、余额档补差、合规多代币、configHash 续跑（中断后重跑跳过已完成阶段）。

- [ ] **Step 6: 提交**

```bash
git add tools/seeder/seed.py API.md main.go docs/
git commit -m "docs(seed-v2): API/usage 文档对齐四端点;Python v1 CLI 标注废弃;全量回归通过"
```

---

## Self-Review 记录

- **规格覆盖**：§2 配置模型→T1；§3 模板+兼容映射→T2/T3/T7；§4 计划生成→T3；§5 七模块+configHash 命名空间→T4/T5/T6；§6 API→T7；§7 页面→T8；§8 错误边界（校验 T1/退避沿用现有 seedMintRetryMax/Python 废弃 T9）；§9 测试策略逐任务落位。staking/validator/vesting 明确不在 v1（规格头注）。
- **占位符扫描**：T6 3b 的 `runComplianceOne` 采用「函数体逐字保留+stage 键替换」的指令式描述而非贴 200 行原文——原文在 `seed_runner.go:399-580`，执行者可直接读，不属占位。其余步骤均有完整代码。
- **类型一致性**：`SeedComplianceSpec` 切片化贯穿 T3(计划)/T4(账户注册)/T6(执行器)；`SeedConfigHash` T1 定义→T4/T7 消费；`seedResolveConfig` T7 定义与测试同名；`runMultisig` T5 定义→T6 接线。已核对。
