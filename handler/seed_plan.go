package handler

import (
	"crypto/sha256"
	"fmt"
)

// ==================== 播种器:确定性计划 ====================
// 与 tools/seeder/seedlib/plan.py 逐项对齐(数据源同构):
// 同一 seed 短语 + 同一规模,每次重建的地址/代币/SFT/证书矩阵完全一致。

var SeedModuleOrder = []string{"accounts", "multisig", "tokens", "sft", "vc", "compliance", "txs"}

const SeedDefaultPhrase = "milon-devnet-seed-v1"

const seedPicsumBase = "https://picsum.photos/seed"

func seedPicsum(seed string, w, h int) string {
	// 带 .jpg 后缀:部分环境/预览组件无法打开无扩展名 URL
	return fmt.Sprintf("%s/%s/%d/%d.jpg", seedPicsumBase, seed, w, h)
}

// ---------- 真实图源(20261020 裁定:全部用真实内容,弃随机占位图) ----------
// 头像     = randomuser.me 真人照片(男女 × 0..99,按名字确定性选取)
// 代币图标 = spothq/cryptocurrency-icons 真实货币 logo(真实币种池确定性轮转;
//            合规稳定币 SCUSD* 钉 usdt)
// URI/封面/卡片 = picsum.photos 真实摄影图(语义中性大图,维持)
// 三个域均经本机 curl 实测 200 可达(20261020)。

// seedIconCurrencies 真实货币 logo 池(cryptocurrency-icons 库确有 128/color 资源)。
var seedIconCurrencies = []string{
	"btc", "eth", "bnb", "sol", "ada", "xrp", "doge", "dot",
	"link", "matic", "ltc", "avax", "atom", "near", "trx", "eos",
}

// seedHashPick 名字确定性选桶:sha256 前 2 字节取模,同名恒同图。
func seedHashPick(name string, mod int) int {
	if mod <= 0 {
		return 0
	}
	sum := sha256.Sum256([]byte(name))
	return int(uint16(sum[0])<<8|uint16(sum[1])) % mod
}

func seedAvatarURL(name string) string {
	gender := "men"
	if seedHashPick("gender:"+name, 2) == 1 {
		gender = "women"
	}
	return fmt.Sprintf("https://randomuser.me/api/portraits/%s/%d.jpg", gender, seedHashPick("ava:"+name, 100))
}

func seedTokenIconURL(symbol string) string {
	// 合规稳定币语义钉 usdt(SCUSD/SCUSD2..);其余按池轮转
	if strings2HasPrefix(symbol, "SCUSD") {
		return "https://raw.githubusercontent.com/spothq/cryptocurrency-icons/master/128/color/usdt.png"
	}
	cur := seedIconCurrencies[seedHashPick("icon:"+symbol, len(seedIconCurrencies))]
	return fmt.Sprintf("https://raw.githubusercontent.com/spothq/cryptocurrency-icons/master/128/color/%s.png", cur)
}

func strings2HasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
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
	Op     string `json:"op"` // burn | transfer | reject_transfer | freeze | merge
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
	Scale       string               `json:"scale"`      // 模板键 minimal|standard|full 或 custom
	ConfigHash  string               `json:"configHash"` // 配置命名空间哈希
	SeedPhrase  string               `json:"seedPhrase"`
	Config      *SeedConfig          `json:"config,omitempty"` // 计划来源配置(执行期开关读这里)
	Admin       SeedAccountSpec      `json:"admin"`
	Users       []SeedAccountSpec    `json:"users"`
	Issuers     []SeedAccountSpec    `json:"issuers"`
	Multisig    []SeedMultisigSpec   `json:"multisig,omitempty"`
	Tokens      []SeedTokenSpec      `json:"tokens"`
	Sfts        []SeedSftSpec        `json:"sfts"`
	VcIssuers   []SeedVcIssuerSpec   `json:"vcIssuers"`
	VcLifecycle []SeedLifecycleOp    `json:"vcLifecycle"`
	Compliance  []SeedComplianceSpec `json:"compliance"`
	Txs         SeedTxsSpec          `json:"txs"`
}

// ---------- 计划构建 ----------

const seedMil = uint64(1_000_000)

// 每精度基准铸造量(人类可读):raw = base*(i+1)*10^d,总量远小于 u64 上限。
// 与 Python _HUMAN_BASE 一致;链端 decimals 上限 9(519)。
var seedHumanBase = map[uint8]uint64{
	0: 1000, 1: 500, 2: 100, 3: 50, 4: 25, 5: 20, 6: 50, 7: 20, 8: 20, 9: 10,
}

func pow10u64(n uint64) uint64 {
	v := uint64(1)
	for i := uint64(0); i < n; i++ {
		v *= 10
	}
	return v
}

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
