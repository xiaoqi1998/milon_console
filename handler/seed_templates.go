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
	// 同默认配置:播种器确定性派生仅支持双曲线(Go SDK 无法从种子派生 fndsa512 公钥)。
	full.Accounts = SeedAccountsCfg{Count: 12, Curves: []string{"secp256k1", "ed25519"},
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
