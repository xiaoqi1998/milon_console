package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"milon-api-server/types"

	"github.com/milon-labs/milon-go-sdk/crypto"
)

// ==================== 播种器:确定性派生 + 断点状态 ====================

// SeedPlanAccount 是运行期账户(计划 spec + 派生结果)。
type SeedPlanAccount struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Curve       string `json:"curve"`
	SubjectType string `json:"subjectType"`
	Alias       string `json:"alias"`
	AliasSuffix uint32 `json:"aliasSuffix"`
	AvatarURL   string `json:"avatarUrl"`

	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey,omitempty"`
	Address    string `json:"address,omitempty"`

	SK  crypto.SecretKeyer   `json:"-"`
	PUB *crypto.PublicKey    `json:"-"`
	ADR *crypto.Address      `json:"-"`
}

// SeedDeriveKey 确定性派生:privkey = sha256("{seed}:{role}:{name}")。
// 与 Python 版 accounts.derive_key 一致——两边可以互相接续同一批地址。
func SeedDeriveKey(seedPhrase, role, name string) string {
	sum := sha256.Sum256([]byte(seedPhrase + ":" + role + ":" + name))
	return hex.EncodeToString(sum[:])
}

// SeedParseSecretKey 把 hex 私钥解析为 SDK SecretKeyer(曲线无关;
// 公钥派生时用 derivePublicKeyByType 指定曲线)。
func SeedParseSecretKey(privHex, curve string) (crypto.SecretKeyer, error) {
	return types.ParseSecretKey(privHex)
}

// SeedEnsureAccount 补齐 publicKey/address(幂等)。地址派生必须走 SDK 口径:
// 同一 32 字节在不同曲线下地址不同,不本地臆造。
func SeedEnsureAccount(acct *SeedPlanAccount) error {
	if acct.SK == nil || acct.PublicKey == "" || acct.Address == "" {
		sk, err := SeedParseSecretKey(acct.PrivateKey, acct.Curve)
		if err != nil {
			return fmt.Errorf("parse %s key: %w", acct.Name, err)
		}
		pub, err := derivePublicKeyByType(sk, acct.Curve)
		if err != nil {
			return fmt.Errorf("derive %s public key: %w", acct.Name, err)
		}
		addr, err := crypto.NewAddressFromPublicKey(pub)
		if err != nil {
			return fmt.Errorf("derive %s address: %w", acct.Name, err)
		}
		acct.SK = sk
		acct.PUB = pub
		acct.ADR = addr
		acct.PublicKey = pub.ToBase58()
		acct.Address = addr.ToBase58()
	}
	return nil
}

// parseSeedAddress 解析 base58 地址(types.ParseAddress 口径)。
func parseSeedAddress(addrStr string) (*crypto.Address, error) {
	addr, err := types.ParseAddress(addrStr)
	if err != nil {
		return nil, err
	}
	return &addr, nil
}

// SeedBuildAccounts 从计划派生全部运行期账户。
func SeedBuildAccounts(plan SeedPlan) (map[string]*SeedPlanAccount, error) {
	accts := map[string]*SeedPlanAccount{}
	reg := func(spec SeedAccountSpec, keyName string) error {
		name := keyName
		if name == "" {
			name = spec.Name
		}
		acct := &SeedPlanAccount{
			Name: name, Role: spec.Role, Curve: spec.Curve,
			SubjectType: spec.SubjectType, Alias: spec.Alias,
			AliasSuffix: spec.AliasSuffix, AvatarURL: spec.AvatarURL,
			PrivateKey: SeedDeriveKey(plan.SeedPhrase, spec.Role, name),
		}
		if err := SeedEnsureAccount(acct); err != nil {
			return err
		}
		accts[name] = acct
		return nil
	}
	if err := reg(plan.Admin, ""); err != nil {
		return nil, err
	}
	for _, u := range plan.Users {
		if err := reg(u, ""); err != nil {
			return nil, err
		}
	}
	for _, i := range plan.Issuers {
		if err := reg(i, ""); err != nil {
			return nil, err
		}
	}
	for _, tk := range plan.Tokens {
		spec := SeedAccountSpec{Name: tk.Symbol, Role: "token", Curve: "secp256k1"}
		if err := reg(spec, tk.Symbol); err != nil {
			return nil, err
		}
	}
	for _, cm := range plan.Compliance {
		spec := SeedAccountSpec{Name: cm.Symbol, Role: "token", Curve: "secp256k1"}
		if err := reg(spec, cm.Symbol); err != nil {
			return nil, err
		}
	}
	for _, s := range plan.Sfts {
		spec := SeedAccountSpec{Name: s.Key, Role: "sft", Curve: "ed25519"}
		if err := reg(spec, s.Key); err != nil {
			return nil, err
		}
	}
	return accts, nil
}

// ==================== 断点状态 ====================

// SeedState 断点续跑状态:{module: {stage: data}},schema 与 Python 版一致,
// 共用 data/seed_state.json——两个实现可互相接续。
type SeedState struct {
	data map[string]map[string]any
	path string
}

// LoadSeedState 读取状态文件;缺失/损坏视为空。
func LoadSeedState(path string) *SeedState {
	st := &SeedState{data: map[string]map[string]any{}, path: path}
	raw, err := os.ReadFile(path)
	if err == nil && len(raw) > 0 {
		var parsed map[string]map[string]any
		if json.Unmarshal(raw, &parsed) == nil && parsed != nil {
			st.data = parsed
		}
	}
	return st
}

// IsDone 判断某模块某阶段是否已完成。
func (s *SeedState) IsDone(module, stage string) bool {
	_, ok := s.data[module][stage]
	return ok
}

// Get 取某阶段产物数据。
func (s *SeedState) Get(module, stage string) any {
	return s.data[module][stage]
}

// Mark 标记阶段完成并记录产物。
func (s *SeedState) Mark(module, stage string, data any) {
	if s.data[module] == nil {
		s.data[module] = map[string]any{}
	}
	s.data[module][stage] = data
}

// Reset 清空全部标记(等价 Python --reset)。
func (s *SeedState) Reset() {
	s.data = map[string]map[string]any{}
}

// Data 只读视图(快照用)。
func (s *SeedState) Data() map[string]map[string]any {
	return s.data
}

// Save 原子落盘(tmp + rename),不留半截文件。
func (s *SeedState) Save() error {
	raw, err := json.MarshalIndent(s.data, "", " ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
