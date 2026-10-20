package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestSeedDeriveKeyContract(t *testing.T) {
	sum := sha256.Sum256([]byte("sp:user:test01"))
	want := hex.EncodeToString(sum[:])
	if got := SeedDeriveKey("sp", "user", "test01"); got != want {
		t.Fatalf("SeedDeriveKey = %s, want %s", got, want)
	}
}

func TestSeedDeriveKeyDistinct(t *testing.T) {
	a := SeedDeriveKey("seed", "user", "test01")
	b := SeedDeriveKey("seed", "user", "test02")
	c := SeedDeriveKey("seed", "issuer", "test01")
	d := SeedDeriveKey("seed2", "user", "test01")
	set := map[string]bool{a: true, b: true, c: true, d: true}
	if len(set) != 4 {
		t.Fatal("名字/角色/短语任一不同必须得到不同密钥")
	}
	if len(a) != 64 {
		t.Fatalf("key len = %d, want 64 hex chars", len(a))
	}
}

func TestSeedEnsureAccountFillsPubAndAddr(t *testing.T) {
	acct := SeedPlanAccount{Name: "test01", Role: "user", Curve: "secp256k1",
		PrivateKey: SeedDeriveKey(SeedDefaultPhrase, "user", "test01")}
	if err := SeedEnsureAccount(&acct); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if acct.PublicKey == "" || acct.Address == "" {
		t.Fatal("公钥/地址必须被填充")
	}
	if acct.Address == acct.PublicKey {
		t.Fatal("地址与公钥不应相同")
	}
	// 幂等:已填不重算
	pk, ad := acct.PublicKey, acct.Address
	if err := SeedEnsureAccount(&acct); err != nil {
		t.Fatalf("ensure again: %v", err)
	}
	if acct.PublicKey != pk || acct.Address != ad {
		t.Fatal("二次 ensure 不得改变已派生值")
	}
}

func TestSeedEnsureAccountEd25519(t *testing.T) {
	kyc := SeedPlanAccount{Name: "kycorg", Role: "issuer", Curve: "ed25519",
		PrivateKey: SeedDeriveKey(SeedDefaultPhrase, "issuer", "kycorg")}
	if err := SeedEnsureAccount(&kyc); err != nil {
		t.Fatalf("ensure ed25519: %v", err)
	}
	if kyc.Address == "" || kyc.PublicKey == "" {
		t.Fatal("ed25519 派生失败")
	}
	// 同一 32 字节不同曲线必须得到不同地址(链规则)
	sec := SeedPlanAccount{Name: "kycorg", Role: "issuer", Curve: "secp256k1",
		PrivateKey: kyc.PrivateKey}
	if err := SeedEnsureAccount(&sec); err != nil {
		t.Fatalf("ensure secp: %v", err)
	}
	if sec.Address == kyc.Address {
		t.Fatal("不同曲线不得派生相同地址")
	}
}

func TestSeedStateRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed_state.json")
	st := LoadSeedState(path)
	if st.IsDone("tokens", "create:SEED6") {
		t.Fatal("空 state 不应有完成标记")
	}
	st.Mark("sft", "slot:genesis:empty", map[string]any{"slotId": 4})
	st.Mark("vc", "flow:kycorg:admin", map[string]any{"allValid": true})
	if err := st.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	st2 := LoadSeedState(path)
	if !st2.IsDone("sft", "slot:genesis:empty") || !st2.IsDone("vc", "flow:kycorg:admin") {
		t.Fatal("roundtrip 后标记丢失")
	}
	if st2.IsDone("sft", "slot:genesis:burn_shell") || st2.IsDone("txs", "batch") {
		t.Fatal("未标记阶段误报完成")
	}
	leftovers, _ := filepath.Glob(filepath.Join(t.TempDir(), "*.tmp"))
	if len(leftovers) != 0 {
		t.Fatal("save 不得留下 .tmp 残留")
	}
}

func TestSeedStateMissingFileIsEmpty(t *testing.T) {
	st := LoadSeedState(filepath.Join(t.TempDir(), "nope.json"))
	if st.IsDone("accounts", "faucet") {
		t.Fatal("缺失文件应视为空")
	}
}

func TestSeedStateOverwriteStage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	st := LoadSeedState(path)
	st.Mark("m", "g", map[string]any{"v": 1})
	st.Mark("m", "g", map[string]any{"v": 2})
	if err := st.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := LoadSeedState(path).Get("m", "g")
	m, ok := got.(map[string]any)
	if !ok || m["v"] != float64(2) {
		t.Fatalf("覆盖失败: %v", got)
	}
	_ = os.Remove(path)
}

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
	// 历史:此处曾显式覆盖 Curves=[secp256k1,ed25519] 规避 fndsa512 派生缺口,
	// 9d1fc13 后 defaults 已等价,覆盖删除(背景见 task-4-5-report)。
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
