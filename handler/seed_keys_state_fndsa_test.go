package handler

import (
	"strings"
	"testing"
)

// ==================== A1:fndsa512 确定性派生 ====================
// fndsa.KeyGen 从 rng 恰读 32 字节种子且 keygen_inner 对种子确定性——
// sha256("{seed}:{role}:{name}") 直接作种子即可复现同一密钥对。

// TestSeedDeriveFndsaKeyPairDeterministic 同种子两次派生结果完全一致。
func TestSeedDeriveFndsaKeyPairDeterministic(t *testing.T) {
	p1, err := SeedDeriveFndsaKeyPair("phrase", "admin", "admin")
	if err != nil {
		t.Fatalf("derive#1: %v", err)
	}
	p2, err := SeedDeriveFndsaKeyPair("phrase", "admin", "admin")
	if err != nil {
		t.Fatalf("derive#2: %v", err)
	}
	if p1.PrivateKey != p2.PrivateKey {
		t.Fatalf("私钥不一致:\n%s\n%s", p1.PrivateKey, p2.PrivateKey)
	}
	if p1.PublicKey != p2.PublicKey {
		t.Fatalf("公钥不一致:\n%s\n%s", p1.PublicKey, p2.PublicKey)
	}
	if p1.Address != p2.Address {
		t.Fatalf("地址不一致: %s vs %s", p1.Address, p2.Address)
	}
}

// TestSeedDeriveFndsaKeyPairShapes 形状:私钥 1281 字节 hex(2562 字符)、公钥可解析、地址可解析。
func TestSeedDeriveFndsaKeyPairShapes(t *testing.T) {
	p, err := SeedDeriveFndsaKeyPair("phrase", "admin", "admin")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(p.PrivateKey) != 2*1281 {
		t.Fatalf("私钥应为 1281 字节 hex(2562 字符), got %d", len(p.PrivateKey))
	}
	if p.PublicKey == "" || p.Address == "" {
		t.Fatalf("公钥/地址为空: pub=%q addr=%q", p.PublicKey, p.Address)
	}
	if _, err := parseSeedAddress(p.Address); err != nil {
		t.Fatalf("地址不可解析 %s: %v", p.Address, err)
	}
}

// TestSeedDeriveFndsaKeyPairVarying 不同 seed/role/name 派生不同密钥。
func TestSeedDeriveFndsaKeyPairVarying(t *testing.T) {
	a, err := SeedDeriveFndsaKeyPair("phrase", "admin", "admin")
	if err != nil {
		t.Fatalf("derive a: %v", err)
	}
	b, err := SeedDeriveFndsaKeyPair("phrase", "user", "u1")
	if err != nil {
		t.Fatalf("derive b: %v", err)
	}
	if a.PrivateKey == b.PrivateKey || a.Address == b.Address {
		t.Fatalf("不同输入不应派生出相同密钥")
	}
	c, err := SeedDeriveFndsaKeyPair("phrase2", "admin", "admin")
	if err != nil {
		t.Fatalf("derive c: %v", err)
	}
	if a.PrivateKey == c.PrivateKey {
		t.Fatalf("不同 seedPhrase 不应派生出相同私钥")
	}
}

// TestSeedEnsureAccountFndsaWithPubKey 自带 publicKey/address 的 fndsa512 条目
// 只装 SK 不派生公钥(Go SDK 无 fndsa512 公钥派生)——账号本既有承诺。
func TestSeedEnsureAccountFndsaWithPubKey(t *testing.T) {
	p, err := SeedDeriveFndsaKeyPair("phrase", "admin", "admin")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	acct := &SeedPlanAccount{Name: "admin", Curve: "fndsa512",
		PrivateKey: p.PrivateKey, PublicKey: p.PublicKey, Address: p.Address}
	if err := SeedEnsureAccount(acct); err != nil {
		t.Fatalf("自带公钥不应触发派生: %v", err)
	}
	if acct.SK == nil {
		t.Fatalf("SK 未装载")
	}
}

// TestSeedEnsureAccountFndsaWithoutPubKey 只有私钥的 fndsa512 条目给明确报错
// (不透传裸 not implemented),提示需带公钥/地址。
func TestSeedEnsureAccountFndsaWithoutPubKey(t *testing.T) {
	p, err := SeedDeriveFndsaKeyPair("phrase", "admin", "admin")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	acct := &SeedPlanAccount{Name: "admin", Curve: "fndsa512", PrivateKey: p.PrivateKey}
	err = SeedEnsureAccount(acct)
	if err == nil {
		t.Fatalf("缺公钥应报错")
	}
	if !strings.Contains(err.Error(), "公钥") {
		t.Fatalf("报错应提示公钥, got: %v", err)
	}
}

// TestSeedBuildAccountsFndsa 全 fndsa512 计划:reg 路径自动补公钥/地址且确定。
func TestSeedBuildAccountsFndsa(t *testing.T) {
	plan := SeedPlan{SeedPhrase: "phrase",
		Admin: SeedAccountSpec{Name: "admin", Role: "admin", Curve: "fndsa512"},
		Users: []SeedAccountSpec{{Name: "u1", Role: "user", Curve: "fndsa512"}}}
	accts, err := SeedBuildAccounts(plan)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	admin := accts["admin"]
	if admin == nil || admin.Address == "" || admin.PublicKey == "" || admin.SK == nil {
		t.Fatalf("admin 账户未补齐: %+v", admin)
	}
	p, _ := SeedDeriveFndsaKeyPair("phrase", "admin", "admin")
	if admin.PrivateKey != p.PrivateKey || admin.Address != p.Address {
		t.Fatalf("BuildAccounts 与直接派生不一致")
	}
}

// TestSeedBuildAccountsTokenSftFndsa token/sft 角色硬编码曲线也走 fndsa512。
func TestSeedBuildAccountsTokenSftFndsa(t *testing.T) {
	plan := SeedPlan{SeedPhrase: "phrase",
		Tokens: []SeedTokenSpec{{Symbol: "TK1"}},
		Sfts:   []SeedSftSpec{{Key: "sft1"}},
		Admin:  SeedAccountSpec{Name: "admin", Role: "admin", Curve: "fndsa512"}}
	accts, err := SeedBuildAccounts(plan)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if tk := accts["TK1"]; tk == nil || tk.Curve != "fndsa512" || tk.Address == "" {
		t.Fatalf("token 账户应为 fndsa512: %+v", tk)
	}
	if sf := accts["sft1"]; sf == nil || sf.Curve != "fndsa512" || sf.Address == "" {
		t.Fatalf("sft 账户应为 fndsa512: %+v", sf)
	}
}
