// handler/seed_config.go
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ==================== 播种器 v2:配置模型 ====================
// SeedConfig 是唯一真相源:校验、模板展开、计划生成、执行全部围绕它。
// 规格见 docs/superpowers/specs/2026-10-19-seed-builder-v2-design.md §2。

// seedBalanceTiers 余额档位 → MIL 数(展示口径;执行期乘 seedMil 转最小单位)。
// one=1 MIL:2026-11-25 水龙头降为单次 1 MIL 后唯一零充值可达成档。
var seedBalanceTiers = map[string]uint64{
	"one": 1, "hundred": 100, "thousand": 1000, "ten_k": 10000,
}

// seedValidCurves 合法密钥曲线(链端三选一)。
// 2026-11-24 起播种器支持 fndsa512 确定性派生(sha256 种子直接作 fndsa KeyGen 种子)。
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
	Vesting    SeedVestingCfg  `json:"vesting"`
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

// SeedVestingCfg 代币分配计划(Vesting)旋钮。plans=0 整块关闭。
// 链端约束(2026-11-20 实测):vesting_id 只由 token 派生——同 token 第二笔
// CreateVesting 恒 539,故每计划绑定独立代币(复用 tokens 模块非合规代币,
// 不足由计划展开补建 VT01..);start 不能在过去(537),startDelaySeconds ≥1。
type SeedVestingCfg struct {
	Plans             int      `json:"plans"`
	Recipients        int      `json:"recipients"`
	Names             []string `json:"names"`   // 可选名单(覆盖 users 轮转;random 模式清空)
	Random            bool     `json:"random"`  // 随机构造:挑人+金额/周期参数走安全范围
	Total             uint64   `json:"total"`   // 每人总额(random 忽略)
	Initial           uint64   `json:"initial"` // 首释量,0..total(random 忽略)
	CliffSeconds      uint64   `json:"cliffSeconds"`
	PeriodSeconds     uint64   `json:"periodSeconds"`
	Periods           uint64   `json:"periods"`
	StartDelaySeconds uint64   `json:"startDelaySeconds"` // start = 执行时 now + 延迟
	Claim             bool     `json:"claim"`             // postOps:计划建好后领一次款
}

// SeedConfigDefaults 缺省配置(= standard 模板内容)。
func SeedConfigDefaults() SeedConfig {
	return SeedConfig{
		SeedPhrase: SeedDefaultPhrase,
		Accounts: SeedAccountsCfg{
			// 默认抗量子曲线(2026-11-24):fndsa512 确定性派生已支持。
			Count: 8, Curves: []string{"fndsa512"},
			Faucet: true, BalanceTiers: []string{"one"}, DID: true, VcCert: true,
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
		// Vesting 默认关闭(plans=0):模板快照与既有行为不变,开启属显式选择。
		Vesting: SeedVestingCfg{
			Plans: 0, Recipients: 2, Random: false,
			Total: 1_000_000, Initial: 100_000,
			CliffSeconds: 60, PeriodSeconds: 60, Periods: 10,
			StartDelaySeconds: 120, Claim: false,
		},
	}
}

// NormalizeSeedConfig 应用隐含规则:vcCert ⇒ did(证书挂 DID 主体);
// vesting.random ⇒ 清空 names(随机模式挑人,与名单互斥)。
func NormalizeSeedConfig(c SeedConfig) SeedConfig {
	if c.Accounts.VcCert {
		c.Accounts.DID = true
	}
	if c.Vesting.Random {
		c.Vesting.Names = nil
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
		addf("accounts.balanceTiers: 须至少选一档(可选 one/hundred/thousand/ten_k)")
	}
	for _, tier := range c.Accounts.BalanceTiers {
		if _, ok := seedBalanceTiers[tier]; !ok {
			addf("accounts.balanceTiers[%s]: 非法档位(可选 one/hundred/thousand/ten_k)", tier)
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

	if c.Vesting.Plans < 0 || c.Vesting.Plans > 20 {
		addf("vesting.plans: 须在 0..20, got %d", c.Vesting.Plans)
	}
	if c.Vesting.Plans > 0 {
		if c.Vesting.Recipients < 1 || c.Vesting.Recipients > 20 {
			addf("vesting.recipients: 须在 1..20, got %d", c.Vesting.Recipients)
		}
		for _, n := range c.Vesting.Names {
			if strings.TrimSpace(n) == "" {
				addf("vesting.names: 名单含空名(逗号分隔的账号名不能为空)")
				break
			}
		}
		// random=true:金额/周期参数走服务端安全范围随机,跳过范围校验;
		// plans/recipients/names 仍校验(规模与名单不随机)。
		if !c.Vesting.Random {
			if c.Vesting.Total < 1 {
				addf("vesting.total: 须 ≥1, got %d", c.Vesting.Total)
			}
			if c.Vesting.Initial > c.Vesting.Total {
				addf("vesting.initial: 首释 %d 超总额 %d(链端 537)", c.Vesting.Initial, c.Vesting.Total)
			}
			if c.Vesting.Periods < 1 {
				addf("vesting.periods: 须 ≥1, got %d", c.Vesting.Periods)
			}
			if c.Vesting.PeriodSeconds < 1 {
				addf("vesting.periodSeconds: 须 ≥1, got %d", c.Vesting.PeriodSeconds)
			}
			// 每期释放量 floor((total−initial)/periods) 须 ≥1(链端 537)。
			if c.Vesting.Total >= c.Vesting.Initial && c.Vesting.Total-c.Vesting.Initial < c.Vesting.Periods {
				addf("vesting.release: 每期释放量须 ≥1——(total−initial)=%d 须 ≥ periods=%d", c.Vesting.Total-c.Vesting.Initial, c.Vesting.Periods)
			}
			if c.Vesting.StartDelaySeconds < 1 {
				addf("vesting.startDelaySeconds: 须 ≥1, got %d(start 不能在过去,链端 537)", c.Vesting.StartDelaySeconds)
			}
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
