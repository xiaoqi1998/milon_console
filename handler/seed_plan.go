package handler

import (
	"fmt"
)

// ==================== 播种器:确定性计划 ====================
// 与 tools/seeder/seedlib/plan.py 逐项对齐(数据源同构):
// 同一 seed 短语 + 同一规模,每次重建的地址/代币/SFT/证书矩阵完全一致。
// 图片一律 picsum.photos 固定 seed——真实公网可见,重建后 URL 不变。

var SeedModuleOrder = []string{"accounts", "tokens", "sft", "vc", "compliance", "txs"}

const SeedDefaultPhrase = "milon-devnet-seed-v1"

const seedPicsumBase = "https://picsum.photos/seed"

func seedPicsum(seed string, w, h int) string {
	// 带 .jpg 后缀:部分环境/预览组件无法打开无扩展名 URL
	return fmt.Sprintf("%s/%s/%d/%d.jpg", seedPicsumBase, seed, w, h)
}

func seedAvatarURL(name string) string {
	return seedPicsum("milon-ava-"+name, 200, 200)
}

func seedTokenIconURL(symbol string) string {
	return seedPicsum("milon-icon-"+strings2Lower(symbol), 128, 128)
}

func seedTokenURI(symbol string) string {
	return seedPicsum("milon-meta-"+strings2Lower(symbol), 400, 400)
}

func seedSftCoverURL(key string) string {
	return seedPicsum("milon-sft-"+key, 600, 600)
}

func seedSlotCoverURL(sftKey, slotKey string) string {
	return seedPicsum("milon-slot-"+sftKey+"-"+slotKey, 600, 600)
}

func seedCardCoverURL(sftKey, slotKey string, idx int) string {
	return seedPicsum(fmt.Sprintf("milon-card-%s-%s-%d", sftKey, slotKey, idx), 600, 600)
}

func strings2Lower(s string) string {
	out := []byte(s)
	for i := range out {
		if out[i] >= 'A' && out[i] <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}

func seedFormatTestName(i int) string {
	return fmt.Sprintf("test%02d", i)
}

// ---------- 数据结构 ----------

type SeedAccountSpec struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Curve       string `json:"curve"`
	SubjectType string `json:"subjectType"`
	Alias       string `json:"alias"`
	AliasSuffix uint32 `json:"aliasSuffix"`
	AvatarURL   string `json:"avatarUrl"`
}

type SeedPostOp struct {
	Op     string `json:"op"` // burn | transfer | reject_transfer
	By     string `json:"by"`
	To     string `json:"to,omitempty"`
	Amount uint64 `json:"amount,omitempty"`
	Index  int    `json:"index,omitempty"` // burn: tokenIds 下标(负数从尾数)
}

type SeedDist struct {
	To       string         `json:"to"` // 用户名或 "owner"(=admin)
	Amount   uint64         `json:"amount"`
	Override map[string]any `json:"override,omitempty"`
}

type SeedScenario struct {
	Key          string       `json:"key"`
	Name         string       `json:"name"`
	Transferable bool         `json:"transferable"`
	SlotMeta     map[string]any `json:"slotMeta"`
	Dists        []SeedDist   `json:"dists"`
	PostOps      []SeedPostOp `json:"postOps,omitempty"`
}

type SeedSftSpec struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Symbol      string         `json:"symbol"`
	RoyaltyBps  uint16         `json:"royaltyBps"`
	CoverURL    string         `json:"coverUrl"`
	MetadataURL string         `json:"metadataUrl"`
	Scenarios   []SeedScenario `json:"scenarios"`
}

type SeedTokenSpec struct {
	Symbol         string            `json:"symbol"`
	Name           string            `json:"name"`
	Decimals       uint8             `json:"decimals"`
	IconURL        string            `json:"iconUrl"`
	URI            string            `json:"uri"`
	MintAmounts    map[string]uint64 `json:"mintAmounts"`
	FreezeDemoUser string            `json:"freezeDemoUser,omitempty"`
}

type SeedVcIssuerSpec struct {
	IssuerName   string   `json:"issuerName"`
	SchemaPrefix string   `json:"schemaPrefix"`
	SchemaCount  int      `json:"schemaCount"`
	Schema       string   `json:"schema"`
	Certifies    []string `json:"certifies"`
}

type SeedLifecycleOp struct {
	IssuerName string `json:"issuerName"`
	Kind       string `json:"kind"` // freeze | revoke
	User       string `json:"user"`
}

type SeedComplianceSpec struct {
	Symbol             string `json:"symbol"`
	Name               string `json:"name"`
	Decimals           uint8  `json:"decimals"`
	IconURL            string `json:"iconUrl"`
	URI                string `json:"uri"`
	IssuerName         string `json:"issuerName"`
	Schema             string `json:"schema"`
	Mode               string `json:"mode"`
	MinterUser         string `json:"minterUser"`
	CertifiedReceiver  string `json:"certifiedReceiver"`
	UncertifiedVictim  string `json:"uncertifiedVictim"`
}

type SeedFailureSpec struct {
	Kind       string `json:"kind"`
	ExpectCode int    `json:"expectCode"`
	By         string `json:"by"`
	Amount     uint64 `json:"amount"`
	Detail     string `json:"detail"`
}

type SeedMilTransfer struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
}

type SeedTxsSpec struct {
	MilTransfers []SeedMilTransfer `json:"milTransfers"`
	BatchFrom    string            `json:"batchFrom"`
	BatchTo      []string          `json:"batchTo"`
	BatchAmounts []uint64          `json:"batchAmounts"`
	Failures     []SeedFailureSpec `json:"failures"`
}

type SeedPlan struct {
	Scale       string              `json:"scale"`
	SeedPhrase  string              `json:"seedPhrase"`
	Admin       SeedAccountSpec     `json:"admin"`
	Users       []SeedAccountSpec   `json:"users"`
	Issuers     []SeedAccountSpec   `json:"issuers"`
	Tokens      []SeedTokenSpec     `json:"tokens"`
	Sfts        []SeedSftSpec       `json:"sfts"`
	VcIssuers   []SeedVcIssuerSpec  `json:"vcIssuers"`
	VcLifecycle []SeedLifecycleOp   `json:"vcLifecycle"`
	Compliance  SeedComplianceSpec  `json:"compliance"`
	Txs         SeedTxsSpec         `json:"txs"`
}

// ---------- 计划构建 ----------

const seedMil = uint64(1_000_000)

// 每精度基准铸造量(人类可读):raw = base*(i+1)*10^d,总量远小于 u64 上限。
// 与 Python _HUMAN_BASE 一致;链端 decimals 上限 9(519)。
var seedHumanBase = map[uint8]uint64{
	0: 1000, 1: 500, 2: 100, 3: 50, 4: 25, 5: 20, 6: 50, 7: 20, 8: 20, 9: 10,
}

func seedScenarioPack(sftKey string, names []string) []SeedScenario {
	u := func(i int) string { return names[i%len(names)] }
	meta := func(key, name, attr string) map[string]any {
		return map[string]any{
			"name":      name,
			"coverUrl":  seedSlotCoverURL(sftKey, key),
			"attribute": attr,
		}
	}
	return []SeedScenario{
		{
			Key: "no_transfer", Name: "Slot-NoTransfer", Transferable: false,
			SlotMeta: meta("no_transfer", "Slot-NoTransfer", "scenario=no_transfer;transferable=false"),
			Dists: []SeedDist{
				{To: u(0), Amount: 1}, {To: u(1), Amount: 1}, {To: u(2), Amount: 1},
			},
			PostOps: []SeedPostOp{{Op: "reject_transfer", By: u(0), To: u(3)}},
		},
		{
			Key: "metadata_full", Name: "Slot-MetaFull", Transferable: true,
			SlotMeta: map[string]any{
				"name": "Slot-MetaFull", "symbol": "SMF",
				"coverUrl": seedSlotCoverURL(sftKey, "metadata_full"),
				"metadata": "scenario=metadata_full;all-fields-overridden",
				"attribute": "scenario=metadata_full",
			},
			Dists: []SeedDist{{To: u(4), Amount: 1}, {To: u(4), Amount: 1}},
		},
		{
			Key: "token_override", Name: "Slot-TokenOverride", Transferable: true,
			SlotMeta: meta("token_override", "Slot-TokenOverride", "scenario=token_override"),
			Dists: []SeedDist{
				{To: u(0), Amount: 1, Override: map[string]any{
					"name": "OV-01", "symbol": "OV",
					"coverUrl": seedCardCoverURL(sftKey, "token_override", 1),
					"metadata": "scenario=token_override;full-override",
					"attribute": "grade=S",
				}},
				{To: u(0), Amount: 1, Override: map[string]any{"name": "OV-02", "attribute": "grade=A"}},
			},
		},
		{
			Key: "burn_shell", Name: "Slot-BurnShell", Transferable: true,
			SlotMeta: meta("burn_shell", "Slot-BurnShell", "scenario=burn_shell;3-minted-1-burned"),
			Dists: []SeedDist{
				{To: u(4), Amount: 1, Override: map[string]any{"name": "BS-01"}},
				{To: u(4), Amount: 1, Override: map[string]any{"name": "BS-02"}},
				{To: u(4), Amount: 1, Override: map[string]any{"name": "BS-03"}},
			},
			PostOps: []SeedPostOp{{Op: "burn", By: u(4), Index: -1}},
		},
		{
			Key: "multi_holder", Name: "Slot-MultiHolder", Transferable: true,
			SlotMeta: meta("multi_holder", "Slot-MultiHolder", "scenario=multi_holder;1-token-3-holders"),
			Dists:    []SeedDist{{To: "owner", Amount: 5, Override: map[string]any{"name": "MH-01"}}},
			PostOps: []SeedPostOp{
				{Op: "transfer", By: "owner", To: u(1), Amount: 2},
				{Op: "transfer", By: "owner", To: u(5), Amount: 2},
			},
		},
		{
			Key: "empty", Name: "Slot-Empty", Transferable: true,
			SlotMeta: meta("empty", "Slot-Empty", "scenario=empty;zero-tokens"),
		},
	}
}

func seedScenarioFilter(all []SeedScenario, keep map[string]bool) []SeedScenario {
	out := []SeedScenario{}
	for _, sc := range all {
		if keep[sc.Key] {
			out = append(out, sc)
		}
	}
	return out
}

func seedTokens(names []string, decimals []uint8, freezeDecimals uint8) []SeedTokenSpec {
	out := []SeedTokenSpec{}
	for _, d := range decimals {
		base := seedHumanBase[d]
		mint := map[string]uint64{}
		for i, name := range names {
			mult := uint64(i + 1)
			if d >= 12 { // 不会出现(上限 9),保留对齐 Python 的语义
				mult = 1
			}
			mint[name] = base * mult * pow10u64(uint64(d))
		}
		freeze := ""
		if d == freezeDecimals {
			freeze = names[4%len(names)]
		}
		symbol := fmt.Sprintf("SEED%d", d)
		out = append(out, SeedTokenSpec{
			Symbol: symbol, Name: fmt.Sprintf("Seed Token %ddp", d), Decimals: d,
			IconURL: seedTokenIconURL(symbol), URI: seedTokenURI(symbol),
			MintAmounts: mint, FreezeDemoUser: freeze,
		})
	}
	return out
}

func pow10u64(n uint64) uint64 {
	v := uint64(1)
	for i := uint64(0); i < n; i++ {
		v *= 10
	}
	return v
}

// BuildSeedPlan 构建确定性播种计划(纯数据,零 IO)。
func BuildSeedPlan(scale, seedPhrase string) SeedPlan {
	var p SeedPlan
	switch scale {
	case "full":
		names := seedTestNames(12)
		p = SeedPlan{Scale: scale, SeedPhrase: seedPhrase}
		p.Admin = SeedAccountSpec{Name: "admin", Role: "admin", Curve: "secp256k1",
			SubjectType: "Personal", Alias: "seed-admin", AliasSuffix: 3001,
			AvatarURL: seedAvatarURL("admin")}
		p.Users = seedUserSpecs(names, 1001)
		p.Issuers = []SeedAccountSpec{
			{Name: "kycorg", Role: "issuer", Curve: "ed25519", SubjectType: "Organization",
				Alias: "seed-kycorg", AliasSuffix: 2011, AvatarURL: seedAvatarURL("kycorg")},
			{Name: "gameorg", Role: "issuer", Curve: "ed25519", SubjectType: "Organization",
				Alias: "seed-gameorg", AliasSuffix: 2012, AvatarURL: seedAvatarURL("gameorg")},
		}
		decimals := []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
		p.Tokens = seedTokens(names, decimals, 8)
		pack := seedScenarioPack("genesis", names)
		keep := func(keys ...string) map[string]bool {
			m := map[string]bool{}
			for _, k := range keys {
				m[k] = true
			}
			return m
		}
		p.Sfts = []SeedSftSpec{
			{Key: "genesis", Name: "Seed Genesis Cards", Symbol: "SGC", RoyaltyBps: 250,
				CoverURL: seedSftCoverURL("genesis"), MetadataURL: seedPicsum("milon-meta-sft-genesis", 400, 400),
				Scenarios: seedScenarioFilter(pack, keep("no_transfer", "metadata_full", "token_override", "burn_shell", "multi_holder", "empty"))},
			{Key: "gameitems", Name: "Seed Game Items", Symbol: "SGI", RoyaltyBps: 0,
				CoverURL: seedSftCoverURL("gameitems"), MetadataURL: seedPicsum("milon-meta-sft-gameitems", 400, 400),
				Scenarios: seedScenarioFilter(seedScenarioPack("gameitems", names), keep("multi_holder", "burn_shell", "token_override", "empty"))},
		}
		p.VcIssuers = []SeedVcIssuerSpec{
			{IssuerName: "kycorg", SchemaPrefix: "SeedKyc", SchemaCount: 2, Schema: "SeedKyc1",
				Certifies: append([]string{"admin"}, seedRangeNames(1, 8)...)},
			{IssuerName: "gameorg", SchemaPrefix: "SeedPlayer", SchemaCount: 1, Schema: "SeedPlayer1",
				Certifies: seedRangeNames(3, 10)},
		}
		p.VcLifecycle = []SeedLifecycleOp{
			{IssuerName: "gameorg", Kind: "freeze", User: "test05"},
			{IssuerName: "gameorg", Kind: "revoke", User: "test08"},
		}
		p.Compliance = SeedComplianceSpec{
			Symbol: "SCUSD", Name: "Seed Compliant USD", Decimals: 6,
			IconURL: seedTokenIconURL("SCUSD"), URI: seedTokenURI("SCUSD"),
			IssuerName: "kycorg", Schema: "SeedKyc1", Mode: "Any",
			MinterUser: "test02", CertifiedReceiver: "test01", UncertifiedVictim: "test09",
		}
		p.Txs = SeedTxsSpec{
			MilTransfers: []SeedMilTransfer{
				{From: "test01", To: "test03", Amount: 137 * seedMil},
				{From: "test03", To: "test05", Amount: 89 * seedMil},
				{From: "test05", To: "test07", Amount: 45 * seedMil},
				{From: "test02", To: "test04", Amount: 200 * seedMil},
				{From: "test04", To: "test06", Amount: 64 * seedMil},
				{From: "test06", To: "test08", Amount: 100 * seedMil},
				{From: "test08", To: "test10", Amount: 33 * seedMil},
				{From: "test10", To: "test01", Amount: 21 * seedMil},
			},
			BatchFrom: "test01",
			BatchTo:   []string{"test02", "test03", "test04", "test05", "test06"},
			BatchAmounts: []uint64{10 * seedMil, 10 * seedMil, 10 * seedMil, 10 * seedMil, 10 * seedMil},
			Failures: []SeedFailureSpec{
				{Kind: "insufficient", ExpectCode: 513, By: "test04", Amount: math2Pow62(),
					Detail: "超额转账触发 InsufficientBalance(513)"},
			},
		}
	case "small":
		names := seedTestNames(4)
		p = SeedPlan{Scale: scale, SeedPhrase: seedPhrase}
		p.Admin = SeedAccountSpec{Name: "admin", Role: "admin", Curve: "secp256k1",
			SubjectType: "Personal", Alias: "seed-admin", AliasSuffix: 3001,
			AvatarURL: seedAvatarURL("admin")}
		p.Users = seedUserSpecs(names, 1001)
		p.Issuers = []SeedAccountSpec{
			{Name: "kycorg", Role: "issuer", Curve: "ed25519", SubjectType: "Organization",
				Alias: "seed-kycorg", AliasSuffix: 2011, AvatarURL: seedAvatarURL("kycorg")},
		}
		p.Tokens = seedTokens(names, []uint8{0, 6, 9}, 6)
		p.Sfts = []SeedSftSpec{
			{Key: "genesis", Name: "Seed Genesis Cards", Symbol: "SGC", RoyaltyBps: 250,
				CoverURL: seedSftCoverURL("genesis"), MetadataURL: seedPicsum("milon-meta-sft-genesis", 400, 400),
				Scenarios: seedScenarioFilter(seedScenarioPack("genesis", names),
					map[string]bool{"no_transfer": true, "burn_shell": true, "multi_holder": true})},
		}
		p.VcIssuers = []SeedVcIssuerSpec{
			{IssuerName: "kycorg", SchemaPrefix: "SeedKyc", SchemaCount: 2, Schema: "SeedKyc1",
				Certifies: []string{"admin", "test01", "test02", "test03"}},
		}
		p.Compliance = SeedComplianceSpec{
			Symbol: "SCUSD", Name: "Seed Compliant USD", Decimals: 6,
			IconURL: seedTokenIconURL("SCUSD"), URI: seedTokenURI("SCUSD"),
			IssuerName: "kycorg", Schema: "SeedKyc1", Mode: "Any",
			MinterUser: "test02", CertifiedReceiver: "test01", UncertifiedVictim: "test04",
		}
		p.Txs = SeedTxsSpec{
			MilTransfers: []SeedMilTransfer{
				{From: "test01", To: "test03", Amount: 137 * seedMil},
				{From: "test02", To: "test04", Amount: 89 * seedMil},
				{From: "test03", To: "test01", Amount: 45 * seedMil},
			},
			BatchFrom: "test01",
			BatchTo:   []string{"test02", "test03"},
			BatchAmounts: []uint64{10 * seedMil, 10 * seedMil},
			Failures: []SeedFailureSpec{
				{Kind: "insufficient", ExpectCode: 513, By: "test04", Amount: math2Pow62(),
					Detail: "超额转账触发 InsufficientBalance(513)"},
			},
		}
	default:
		panic(fmt.Sprintf("seed: 未知规模 %q(可选 full/small)", scale))
	}
	return p
}

func seedTestNames(n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = seedFormatTestName(i + 1)
	}
	return out
}

func seedRangeNames(from, to int) []string {
	out := []string{}
	for i := from; i <= to; i++ {
		out = append(out, seedFormatTestName(i))
	}
	return out
}

func seedUserSpecs(names []string, suffixBase int) []SeedAccountSpec {
	out := make([]SeedAccountSpec, len(names))
	for i, name := range names {
		out[i] = SeedAccountSpec{Name: name, Role: "user", Curve: "secp256k1",
			SubjectType: "Personal", Alias: "seed-" + name,
			AliasSuffix: uint32(suffixBase + i), AvatarURL: seedAvatarURL(name)}
	}
	return out
}

func math2Pow62() uint64 {
	return uint64(1) << 62
}
