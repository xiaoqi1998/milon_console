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
	_ = warnMerge

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
