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
