package handler

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== 播种器:六模块执行器 ====================
// 语义与 tools/seeder/seedlib 对齐:确定性派生、断点续跑、链端幂等
// (516/1291 skip)、领水冷却守卫、余额对账补铸、预期失败断言。

const (
	seedMinBalance   = uint64(100 * 1_000_000) // 100 MIL:低于才需要领水
	seedTokenApp     = "token"
	seedMintRetryMax = 3
)

var seedCodeRe = regexp.MustCompile(`\b(5\d\d|10\d\d|12\d\d)\b`)

// seedCodesIn 从错误文本提取链端错误码。
func seedCodesIn(msg string) []int {
	out := []int{}
	for _, m := range seedCodeRe.FindAllString(msg, -1) {
		if n, err := strconv.Atoi(m); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func seedHasCode(msg string, codes ...int) bool {
	got := seedCodesIn(msg)
	for _, want := range codes {
		for _, c := range got {
			if c == want {
				return true
			}
		}
	}
	return false
}

// seedVcRejected 判定无证收款被合规拦截(521 VcRequired / 522 VcPolicyRequired;
// 包装层码语义可能漂移,两者皆视为拦截)。
func seedVcRejected(msg string) bool {
	return seedHasCode(msg, 521, 522)
}

// seedTokenAlreadyExists 516。
func seedTokenAlreadyExists(msg string) bool {
	return seedHasCode(msg, 516) || strings.Contains(msg, "TokenAlreadyExists")
}

// seedMintDiffs 对账:计划铸造量 vs 链上余额,返回需补铸的地址与差额(顺序稳定)。
func seedMintDiffs(want map[string]uint64, have map[string]uint64) ([]string, []uint64) {
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	addrs, amounts := []string{}, []uint64{}
	for _, k := range keys {
		if have[k] < want[k] {
			addrs = append(addrs, k)
			amounts = append(amounts, want[k]-have[k])
		}
	}
	return addrs, amounts
}

// seedFreezeAmount 冻结演示量 = 持有量的 1/3;持有量过小(不足 3)返回 0 = 跳过。
func seedFreezeAmount(balance uint64) uint64 {
	return balance / 3
}

// seedViewU64 数值视图;链端 512(账户无记录)视为 0,其他链端错误返回 error。
func seedViewU64(mc *milon.Client, app, method string, args provider.Args) (uint64, error) {
	value, err := flowCallView(mc, app, method, args)
	if err != nil {
		return 0, err
	}
	if f, ok := value.(*api.ApiFailure); ok {
		if f.Code != nil && *f.Code == 512 {
			return 0, nil
		}
		return 0, fmt.Errorf("view %s.%s: %s", app, method, f.Message)
	}
	switch v := value.(type) {
	case uint64:
		return v, nil
	case int:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case float64:
		return uint64(v), nil
	default:
		return 0, fmt.Errorf("view %s.%s: unexpected type %T", app, method, value)
	}
}

// seedSendSingle 单签写:signer 同时是指令签名者与 gas payer。
func seedSendSingle(mc *milon.Client, app, method string, args provider.Args, signer *SeedPlanAccount) (string, error) {
	pd, ok := mc.GetAllPd()[app]
	if !ok {
		return "", fmt.Errorf("IDL app %q not found", app)
	}
	wire, err := encodeWithCoercion(pd, method, args)
	if err != nil {
		return "", fmt.Errorf("encode %s.%s: %w", app, method, err)
	}
	mode, err := normalizeSignatureModeForAccount(mc, *signer.ADR,
		lib.PubKeySignatureMode{PublicKey: *signer.PUB})
	if err != nil {
		return "", fmt.Errorf("signature mode: %w", err)
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(signer.ADR).
		AddIxAndPayerSig(*signer.ADR, signer.SK, 0, mode).
		Build()
	if err != nil {
		return "", fmt.Errorf("build %s.%s tx: %w", app, method, err)
	}
	return flowSendTx(mc, tx)
}

// seedSendDual 双签写:资源账户签指令,owner 付 gas(token.Create 同款)。
func seedSendDual(mc *milon.Client, app, method string, args provider.Args,
	ix *SeedPlanAccount, payer *SeedPlanAccount) (string, error) {
	pd, ok := mc.GetAllPd()[app]
	if !ok {
		return "", fmt.Errorf("IDL app %q not found", app)
	}
	wire, err := encodeWithCoercion(pd, method, args)
	if err != nil {
		return "", fmt.Errorf("encode %s.%s: %w", app, method, err)
	}
	ixMode, err := normalizeSignatureModeForAccount(mc, *ix.ADR,
		lib.PubKeySignatureMode{PublicKey: *ix.PUB})
	if err != nil {
		return "", fmt.Errorf("ix signature mode: %w", err)
	}
	payerMode, err := normalizeSignatureModeForAccount(mc, *payer.ADR,
		lib.PubKeySignatureMode{PublicKey: *payer.PUB})
	if err != nil {
		return "", fmt.Errorf("payer signature mode: %w", err)
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(payer.ADR).
		AddIxesSig(*ix.ADR, ix.SK, []uint8{0}, false, ixMode).
		AddPayerSig(*payer.ADR, payer.SK, payerMode).
		Build()
	if err != nil {
		return "", fmt.Errorf("build %s.%s tx: %w", app, method, err)
	}
	return flowSendTx(mc, tx)
}

// seedRunner 执行器上下文。
type seedRunner struct {
	mc    *milon.Client
	plan  SeedPlan
	state *SeedState
	accts map[string]*SeedPlanAccount
	logf  func(format string, a ...any)
}

func (r *seedRunner) acct(name string) *SeedPlanAccount {
	if name == "owner" {
		name = "admin"
	}
	return r.accts[name]
}

// run 按顺序执行选中模块;每模块完成后落盘。
func (r *seedRunner) run(modules []string) error {
	if r.accts == nil {
		accts, err := SeedBuildAccounts(r.plan)
		if err != nil {
			return fmt.Errorf("derive accounts: %w", err)
		}
		r.accts = accts
		r.logf("派生确定性账户 %d 个(seed=%q)", len(accts), r.plan.SeedPhrase)
	}
	for _, m := range modules {
		r.logf("===== 模块 %s =====", m)
		var err error
		switch m {
		case "accounts":
			err = r.runAccounts()
		case "multisig":
			err = r.runMultisig()
		case "tokens":
			err = r.runTokens()
		case "sft":
			err = r.runSft()
		case "vc":
			err = r.runVc()
		case "compliance":
			err = r.runCompliance()
		case "txs":
			err = r.runTxs()
		default:
			err = fmt.Errorf("未知模块 %q", m)
		}
		if err != nil {
			return fmt.Errorf("模块 %s: %w", m, err)
		}
		r.state.Save()
	}
	return nil
}

// ---------- accounts ----------

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

// ---------- tokens ----------

func (r *seedRunner) runTokens() error {
	admin := r.acct("admin")
	for _, spec := range r.plan.Tokens {
		tok := r.acct(spec.Symbol)

		stage := "create:" + spec.Symbol
		if !r.state.IsDone("tokens", stage) {
			metadata := map[string]any{
				"name": spec.Name, "symbol": spec.Symbol,
				"decimals": uint32(spec.Decimals), "icon": spec.IconURL, "uri": spec.URI,
			}
			_, err := seedSendDual(r.mc, seedTokenApp, "Create", provider.Args{
				"token": tok.Address, "owner": admin.Address, "metadata": metadata,
			}, tok, admin)
			if err != nil {
				if !seedTokenAlreadyExists(err.Error()) {
					return fmt.Errorf("create %s: %w", spec.Symbol, err)
				}
				r.logf("  %s 链上已存在(516),跳过创建", spec.Symbol)
			} else {
				r.logf("  %s(%ddp) 创建 ✓", spec.Symbol, spec.Decimals)
			}
			r.state.Mark("tokens", stage, map[string]any{"address": tok.Address})
			r.state.Save()
		}

		mintStage := "mint:" + spec.Symbol
		wantByAddr := map[string]uint64{}
		nameByAddr := map[string]string{}
		for name, amount := range spec.MintAmounts {
			wantByAddr[r.acct(name).Address] = amount
			nameByAddr[r.acct(name).Address] = name
		}
		haveByAddr := map[string]uint64{}
		for addr := range wantByAddr {
			have, err := seedViewU64(r.mc, seedTokenApp, "BalanceOf",
				provider.Args{"token": tok.Address, "account": addr})
			if err != nil {
				return fmt.Errorf("balance %s: %w", spec.Symbol, err)
			}
			haveByAddr[addr] = have
		}
		addrs, amounts := seedMintDiffs(wantByAddr, haveByAddr)
		if !r.state.IsDone("tokens", mintStage) || len(addrs) > 0 {
			if len(addrs) == 0 {
				r.logf("  %s 余额均已到位,跳过铸造", spec.Symbol)
			} else {
				var lastErr error
				done := false
				for attempt := 0; attempt < seedMintRetryMax && !done; attempt++ {
					args := []any{}
					for _, a := range addrs {
						args = append(args, a)
					}
					_ = args
					_, lastErr = seedSendSingle(r.mc, seedTokenApp, "MintBatch", provider.Args{
						"token": tok.Address, "to": addrs, "amount": amounts,
					}, admin)
					if lastErr == nil {
						done = true
						break
					}
					r.logf("    重试 %d/%d: %v", attempt+1, seedMintRetryMax, truncateErr(lastErr))
					time.Sleep(5 * time.Second)
				}
				if !done {
					return fmt.Errorf("mint %s: %w", spec.Symbol, lastErr)
				}
				r.logf("  %s 补铸 %d 人(差额合计 %d)✓", spec.Symbol, len(addrs), sumU64(amounts))
			}
			r.state.Mark("tokens", mintStage, map[string]any{"holders": len(wantByAddr)})
			r.state.Save()
		}

		if spec.FreezeDemoUser != "" {
			fstage := "freeze:" + spec.Symbol
			if !r.state.IsDone("tokens", fstage) {
				holder := r.acct(spec.FreezeDemoUser)
				bal, err := seedViewU64(r.mc, seedTokenApp, "BalanceOf",
					provider.Args{"token": tok.Address, "account": holder.Address})
				if err != nil {
					return fmt.Errorf("balance %s: %w", spec.Symbol, err)
				}
				frozen, err := seedViewU64(r.mc, seedTokenApp, "FrozenOf",
					provider.Args{"token": tok.Address, "account": holder.Address})
				if err != nil {
					return fmt.Errorf("frozen %s: %w", spec.Symbol, err)
				}
				amt := seedFreezeAmount(bal)
				switch {
				case frozen > 0:
					r.logf("  %s 冻结演示已完成(frozen=%d),跳过", spec.Symbol, frozen)
				case amt == 0:
					r.logf("  %s %s 链上持有 0,跳过冻结演示", spec.Symbol, spec.FreezeDemoUser)
				default:
					if _, err := seedSendSingle(r.mc, seedTokenApp, "Freeze", provider.Args{
						"token": tok.Address, "holder": holder.Address, "amount": amt,
					}, admin); err != nil {
						return fmt.Errorf("freeze %s: %w", spec.Symbol, err)
					}
					r.logf("  %s 冻结 %s %d ✓", spec.Symbol, spec.FreezeDemoUser, amt)
				}
				r.state.Mark("tokens", fstage, map[string]any{"holder": spec.FreezeDemoUser, "amount": frozen})
				r.state.Save()
			}
		}
	}
	r.logf("tokens 模块完成: %d 个代币(精度 0~9)", len(r.plan.Tokens))
	return nil
}

// ---------- compliance ----------

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

// runComplianceOne 单个合规代币的七阶段链路(阶段键带代币符号,多代币互不串位)。
func (r *seedRunner) runComplianceOne(c SeedComplianceSpec) error {
	token := r.acct(c.Symbol)
	admin := r.acct("admin")
	issuer := r.acct(c.IssuerName)
	minter := r.acct(c.MinterUser)
	receiver := r.acct(c.CertifiedReceiver)
	victim := r.acct(c.UncertifiedVictim)
	mintAmt := uint64(1000) * pow10u64(uint64(c.Decimals))

	mark := func(stage string, data map[string]any) {
		r.state.Mark("compliance", stage, data)
		r.state.Save()
	}

	stage := "credential"
	credID := ""
	if v := r.state.Get("compliance", seedComplianceStageKey(stage, c.Symbol)); v != nil {
		if m, ok := v.(map[string]any); ok {
			credID, _ = m["credentialId"].(string)
		}
	}
	if credID == "" {
		v, err := flowCallView(r.mc, vcFlowIdentityApp, "CredentialId", provider.Args{
			"issuer": issuer.Address, "credential_schema": c.Schema,
		})
		if err != nil {
			return fmt.Errorf("credential id: %w", err)
		}
		if f, ok := v.(*api.ApiFailure); ok {
			return fmt.Errorf("credential id: %s", f.Message)
		}
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("credential id: 意外返回类型 %T", v)
		}
		credID = s
		r.logf("  凭证 ID(%s/%s) = %s", c.IssuerName, c.Schema, credID)
		mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"credentialId": credID})
	}

	stage = "create"
	if !r.state.IsDone("compliance", seedComplianceStageKey(stage, c.Symbol)) {
		metadata := map[string]any{
			"name": c.Name, "symbol": c.Symbol,
			"decimals": uint32(c.Decimals), "icon": c.IconURL, "uri": c.URI,
		}
		_, err := seedSendDual(r.mc, seedTokenApp, "CreateWithCompliance", provider.Args{
			"token": token.Address, "owner": admin.Address,
			"metadata": metadata, "credential_id": credID,
		}, token, admin)
		if err != nil {
			if !seedTokenAlreadyExists(err.Error()) {
				return fmt.Errorf("create %s: %w", c.Symbol, err)
			}
			r.logf("  %s 链上已存在(516),跳过创建", c.Symbol)
		} else {
			r.logf("  合规代币 %s 创建 ✓(挂证 %s)", c.Symbol, credID)
		}
		mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"address": token.Address})
	}

	stage = "mode"
	if !r.state.IsDone("compliance", seedComplianceStageKey(stage, c.Symbol)) {
		if _, err := seedSendSingle(r.mc, seedTokenApp, "SetComplianceMode", provider.Args{
			"token": token.Address, "mode": c.Mode,
		}, admin); err != nil {
			return fmt.Errorf("set mode: %w", err)
		}
		mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"mode": c.Mode})
	}

	stage = "requirements"
	if !r.state.IsDone("compliance", seedComplianceStageKey(stage, c.Symbol)) {
		if _, err := seedSendSingle(r.mc, seedTokenApp, "AddComplianceRequirement", provider.Args{
			"token": token.Address, "credential_ids": []string{credID},
		}, admin); err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "uplicate") && !seedHasCode(msg, 1059, 1064, 520) {
				return fmt.Errorf("add requirement: %w", err)
			}
			r.logf("  凭证要求已存在,跳过")
		}
		mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"credentialIds": []string{credID}})
	}

	stage = "mint"
	if !r.state.IsDone("compliance", seedComplianceStageKey(stage, c.Symbol)) {
		bal, err := seedViewU64(r.mc, seedTokenApp, "BalanceOf",
			provider.Args{"token": token.Address, "account": minter.Address})
		if err != nil {
			return err
		}
		if bal < mintAmt {
			if _, err := seedSendSingle(r.mc, seedTokenApp, "MintBatch", provider.Args{
				"token": token.Address, "to": []string{minter.Address}, "amount": []uint64{mintAmt},
			}, admin); err != nil {
				return fmt.Errorf("compliance mint: %w", err)
			}
			r.logf("  %s 铸给 %s %d ✓", c.Symbol, c.MinterUser, mintAmt)
		} else {
			r.logf("  %s minter 余额充足(%d),跳过铸造", c.Symbol, bal)
		}
		mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"amount": mintAmt})
	}

	stage = "accept"
	if !r.state.IsDone("compliance", seedComplianceStageKey(stage, c.Symbol)) {
		bal, err := seedViewU64(r.mc, seedTokenApp, "BalanceOf",
			provider.Args{"token": token.Address, "account": receiver.Address})
		if err != nil {
			return err
		}
		if bal == 0 {
			amt := uint64(100) * pow10u64(uint64(c.Decimals))
			txHash, err := seedSendSingle(r.mc, seedTokenApp, "Transfer", provider.Args{
				"from": minter.Address, "token": token.Address,
				"to": receiver.Address, "amount": amt,
			}, minter)
			if err != nil {
				return fmt.Errorf("持证用户转账失败(应当成功!): %w", err)
			}
			r.logf("  持证用户 %s 收款 ✓ tx=%s", c.CertifiedReceiver, shortHash(txHash))
		} else {
			r.logf("  持证接收方已有余额(%d),跳过成功转账", bal)
		}
		mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"to": c.CertifiedReceiver})
	}

	stage = "reject"
	if !r.state.IsDone("compliance", seedComplianceStageKey(stage, c.Symbol)) {
		victimAcct := victim
		// 自适应:同链可能先跑过更大规模的证书矩阵,victim 已持证。
		// 此时换一个真正无证的用户;全员持证才跳过拦截实测(记录而非失败)。
		hasVc, herr := seedHasValidVc(r.mc, victim.Address, issuer.Address, c.Schema)
		if herr == nil && hasVc {
			alt := ""
			for _, u := range r.plan.Users {
				uh, e := seedHasValidVc(r.mc, r.acct(u.Name).Address, issuer.Address, c.Schema)
				if e == nil && !uh {
					alt = u.Name
					break
				}
			}
			if alt == "" {
				r.logf("  全员持证,无法实测无证拦截——跳过(已记录)")
				mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"skipped": "all certified"})
			} else {
				r.logf("  victim %s 已持证,改用无证用户 %s 实测拦截", c.UncertifiedVictim, alt)
				victimAcct = r.acct(alt)
			}
		}
		if victimAcct != victim {
			_, xerr := seedSendSingle(r.mc, seedTokenApp, "Transfer", provider.Args{
				"from": minter.Address, "token": token.Address,
				"to": victimAcct.Address, "amount": uint64(10),
			}, minter)
			if xerr == nil {
				return fmt.Errorf("无证用户 %s 竟然收到合规代币——环境异常!", victimAcct.Name)
			}
			if !seedVcRejected(xerr.Error()) {
				return fmt.Errorf("无证拦截错误码异常(期望 521/522): %v", truncateErr(xerr))
			}
			r.logf("  无证用户 %s 被拦截 ✓", victimAcct.Name)
			mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"victim": victimAcct.Name})
		}
	}

	stage = "verify"
	if !r.state.IsDone("compliance", seedComplianceStageKey(stage, c.Symbol)) {
		v, err := flowCallView(r.mc, seedTokenApp, "Compliance", provider.Args{"token": token.Address})
		if err == nil {
			r.logf("  链上合规配置: %v", v)
			mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"compliance": fmt.Sprint(v)})
		} else {
			mark(seedComplianceStageKey(stage, c.Symbol), map[string]any{"error": err.Error()})
		}
	}
	return nil
}

// ---------- txs ----------

func (r *seedRunner) runTxs() error {
	mil := api.MILToken.ToBase58()
	for _, tr := range r.plan.Txs.MilTransfers {
		stage := fmt.Sprintf("mil:%s->%s", tr.From, tr.To)
		if r.state.IsDone("txs", stage) {
			continue
		}
		src := r.acct(tr.From)
		dst := r.acct(tr.To)
		_, err := seedSendSingle(r.mc, seedTokenApp, "Transfer", provider.Args{
			"from": src.Address, "token": mil, "to": dst.Address, "amount": tr.Amount,
		}, src)
		if err != nil {
			if seedHasCode(err.Error(), 513) {
				r.logf("  MIL %s→%s 余额不足,跳过", tr.From, tr.To)
			} else {
				return fmt.Errorf("mil %s→%s: %w", tr.From, tr.To, err)
			}
		} else {
			r.logf("  MIL %s→%s %d ✓", tr.From, tr.To, tr.Amount/1_000_000)
		}
		r.state.Mark("txs", stage, map[string]any{"amount": tr.Amount})
		r.state.Save()
	}

	if !r.state.IsDone("txs", "batch") {
		src := r.acct(r.plan.Txs.BatchFrom)
		tos := []string{}
		for _, n := range r.plan.Txs.BatchTo {
			tos = append(tos, r.acct(n).Address)
		}
		if _, err := seedSendSingle(r.mc, seedTokenApp, "TransferBatch", provider.Args{
			"from": src.Address, "token": mil, "to": tos, "amount": r.plan.Txs.BatchAmounts,
		}, src); err != nil {
			if !seedHasCode(err.Error(), 513) {
				return fmt.Errorf("batch: %w", err)
			}
			r.logf("  MIL 批量余额不足,跳过")
		} else {
			r.logf("  MIL 批量 %s → %d 人 ✓", r.plan.Txs.BatchFrom, len(tos))
		}
		r.state.Mark("txs", "batch", map[string]any{"from": r.plan.Txs.BatchFrom})
		r.state.Save()
	}

	for i, f := range r.plan.Txs.Failures {
		stage := fmt.Sprintf("failure:%s:%d", f.Kind, i)
		if r.state.IsDone("txs", stage) {
			continue
		}
		src := r.acct(f.By)
		_, err := seedSendSingle(r.mc, seedTokenApp, "Transfer", provider.Args{
			"from": src.Address, "token": mil, "to": r.acct("admin").Address, "amount": f.Amount,
		}, src)
		if err == nil {
			return fmt.Errorf("预期失败交易(%s)竟然成功——环境异常!", f.Detail)
		}
		if !seedHasCode(err.Error(), f.ExpectCode) {
			return fmt.Errorf("预期错误码 %d 不符: %v", f.ExpectCode, truncateErr(err))
		}
		r.logf("  预期失败实测 ✓ %s", f.Detail)
		r.state.Mark("txs", stage, map[string]any{"expect": f.ExpectCode})
		r.state.Save()
	}
	r.logf("txs 模块完成: %d 笔互转 + 批量 + %d 笔预期失败",
		len(r.plan.Txs.MilTransfers), len(r.plan.Txs.Failures))
	return nil
}

// ---------- 小工具 ----------

// seedTierTargetMIL 档位 → MIL 数;未知档位 0(跳过)。
func seedTierTargetMIL(tier string) uint64 { return seedBalanceTiers[tier] }

// seedComplianceStageKey 合规阶段键:带代币符号,多合规代币互不串位。
func seedComplianceStageKey(stage, symbol string) string { return stage + ":" + symbol }

func shortAddr(addr string) string {
	if len(addr) > 10 {
		return addr[:10] + "…"
	}
	return addr
}

func shortHash(h string) string {
	if len(h) > 14 {
		return h[:14] + "…"
	}
	return h
}

func truncateErr(err error) string {
	s := err.Error()
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

func sumU64(vs []uint64) uint64 {
	var total uint64
	for _, v := range vs {
		total += v
	}
	return total
}

// seedHasValidVc 查 subject 是否持有指定 issuer+schema 的有效凭证。
func seedHasValidVc(mc *milon.Client, subject, issuer, schema string) (bool, error) {
	v, err := flowCallView(mc, vcFlowIdentityApp, "HasValidVcFromIssuer", provider.Args{
		"subject": subject, "issuer": issuer, "credential_schema": schema,
	})
	if err != nil {
		return false, err
	}
	if f, ok := v.(*api.ApiFailure); ok {
		return false, fmt.Errorf("%s", f.Message)
	}
	b, _ := v.(bool)
	return b, nil
}
