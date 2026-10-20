package handler

import (
	"fmt"
	"strings"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func parseSeedAddr(addrStr string) crypto.Address {
	addr, err := parseSeedAddress(addrStr)
	if err != nil {
		return crypto.Address{}
	}
	return *addr
}

// ==================== 播种器:sft / vc 模块 ====================
// sft 复用 sft_flow_handler.go 的自由函数内核;vc 按 vc_flow_handler.go
// 的七步链路拼装(领水/DID/组织/签发/披露/验证全为现成幂等函数)。

func (r *seedRunner) runSft() error {
	admin := r.acct("admin")
	for _, sftSpec := range r.plan.Sfts {
		res := r.acct(sftSpec.Key)

		// createSft 用确定性 ed25519 资源账户创建 SFT;
		// 链端实际已存在(SftAlreadyExists=516)视为复用成功——重建分支的
		// SftOwnerOf 核实与创建之间存在窗口,或 state 脱节但视图瞬断时兜底。
		createSft := func() error {
			attr := fmt.Sprintf("collection=%s", sftSpec.Key)
			meta := &sftFlowMetadata{
				Name: sftSpec.Name, Symbol: sftSpec.Symbol,
				CoverUrl: sftSpec.CoverURL, Metadata: sftSpec.MetadataURL,
				Attribute: &attr,
			}
			if _, err := sftFlowCreateSft(r.mc, *res.ADR, res.SK, res.PUB,
				meta, sftSpec.RoyaltyBps, *admin.ADR, admin.SK, admin.PUB); err != nil {
				if isTolerableChainError(err.Error(), "SftAlreadyExists", "516") {
					r.logf("  SFT %s 链端已存在(516),视为复用 → %s", sftSpec.Key, shortAddr(res.Address))
					return nil
				}
				return fmt.Errorf("create sft %s: %w", sftSpec.Key, err)
			}
			return nil
		}

		stage := "create:" + sftSpec.Key
		if !r.state.IsDone("sft", stage) {
			// 已存在(SftOwnerOf 可读且 Ok)则跳过创建;否则创建。
			// 注意链端 Err 载荷经 flowCallView 解码为 *api.ApiFailure 而非 error,
			// 存在性判定必须加 flowViewOK,否则不存在的 SFT 会被误判「已存在」。
			existing := false
			if value, err := flowCallView(r.mc, sftFlowIdentityApp, "SftOwnerOf",
				provider.Args{"sft": res.Address}); err == nil && flowViewOK(value) {
				existing = true
			}
			if existing {
				r.logf("  SFT %s(%s) 已存在,复用 → %s", sftSpec.Key, sftSpec.Symbol, shortAddr(res.Address))
			} else {
				if err := createSft(); err != nil {
					return err
				}
				r.logf("  SFT %s(%s) 创建 ✓ → %s", sftSpec.Key, sftSpec.Symbol, shortAddr(res.Address))
			}
			r.state.Mark("sft", stage, map[string]any{"address": res.Address})
			r.state.Save()
		} else if value, err := flowCallView(r.mc, sftFlowIdentityApp, "SftOwnerOf",
			provider.Args{"sft": res.Address}); err != nil || !flowViewOK(value) {
			// 状态标记 create 已完成仍须链端核实:seed_state 与链可能脱节
			// (如重建创世后 state 未清),此时标记不可信——重建 SFT 走创建分支;
			// 链端实际仍存在时创建报 516,由 createSft 已有容忍兜底。
			r.logf("  SFT %s 状态标记与链端不一致,重建 SFT", sftSpec.Key)
			if err := createSft(); err != nil {
				return err
			}
			r.logf("  SFT %s(%s) 重建 ✓ → %s", sftSpec.Key, sftSpec.Symbol, shortAddr(res.Address))
			r.state.Mark("sft", stage, map[string]any{"address": res.Address, "recreated": true})
			r.state.Save()
		}

		sftAddr := res.Address
		for _, sc := range sftSpec.Scenarios {
			stage = fmt.Sprintf("slot:%s:%s", sftSpec.Key, sc.Key)
			if !r.state.IsDone("sft", stage) {
				slotId, tokenIds, err := r.seedSftSlot(sftAddr, admin, sc)
				if err != nil {
					return fmt.Errorf("slot %s/%s: %w", sftSpec.Key, sc.Key, err)
				}
				r.logf("  [%s] slot %d [%s] transferable=%v tokens=%v",
					sftSpec.Key, slotId, sc.Key, sc.Transferable, tokenIds)
				r.state.Mark("sft", stage, map[string]any{
					"slotId": slotId, "tokenIds": tokenIds, "transferable": sc.Transferable})
				r.state.Save()
			}
			rec, _ := r.state.Get("sft", stage).(map[string]any)
			if err := r.seedSftPostOps(sftSpec, sc, rec); err != nil {
				return err
			}
		}
	}
	total := 0
	for _, s := range r.plan.Sfts {
		total += len(s.Scenarios)
	}
	r.logf("sft 模块完成: %d 个集合 / %d 个场景 slot", len(r.plan.Sfts), total)
	return nil
}

// seedSftSlot 建槽位 + 逐笔分发,返回 slotId 与 tokenIds。
func (r *seedRunner) seedSftSlot(sftAddr string, admin *SeedPlanAccount, sc SeedScenario) (uint64, []uint64, error) {
	isTransferable := sc.Transferable
	opts := &sftFlowSlotOptions{
		SlotId:         0,
		Metadata:       seedOverrideFromMap(sc.SlotMeta),
		IsTransferable: &isTransferable,
	}
	slotRes, err := sftFlowEnsureSlot(r.mc, opts, parseSeedAddr(sftAddr), *admin.ADR, admin.SK, admin.PUB)
	if err != nil {
		return 0, nil, err
	}
	slotId := uint64(0)
	if slotRes != nil {
		slotId = slotRes.SlotId
	}
	tokenIds := []uint64{}
	for _, d := range sc.Dists {
		to := r.acct(d.To)
		dist := sftFlowDistribution{
			To:       to.Address,
			Amount:   d.Amount,
			Metadata: seedOverrideFromMap(d.Override),
		}
		mint, err := sftFlowMintOne(r.mc, parseSeedAddr(sftAddr), slotId, dist,
			*admin.ADR, admin.SK, admin.PUB)
		if err != nil {
			return slotId, tokenIds, err
		}
		tokenIds = append(tokenIds, mint.TokenId)
	}
	return slotId, tokenIds, nil
}

// seedSftPostOps 场景后置:烧卡 / 多持有人转移 / 不可转移拒绝实测。
func (r *seedRunner) seedSftPostOps(sftSpec SeedSftSpec, sc SeedScenario, rec map[string]any) error {
	if len(sc.PostOps) == 0 || rec == nil {
		return nil
	}
	rawIds, _ := rec["tokenIds"].([]any)
	tokenIds := make([]uint64, 0, len(rawIds))
	for _, id := range rawIds {
		switch v := id.(type) {
		case float64:
			tokenIds = append(tokenIds, uint64(v))
		case uint64:
			tokenIds = append(tokenIds, v)
		}
	}
	if len(tokenIds) == 0 {
		return nil
	}
	admin := r.acct("admin")
	sftAddr := r.acct(sftSpec.Key).Address
	for i, op := range sc.PostOps {
		stage := fmt.Sprintf("post:%s:%s:%d", sftSpec.Key, sc.Key, i)
		if r.state.IsDone("sft", stage) {
			continue
		}
		switch op.Op {
		case "burn":
			holder := r.acct(op.By)
			tid := seedPickTokenId(tokenIds, op.Index)
			if _, err := seedSendSingle(r.mc, sftFlowIdentityApp, "Burn", provider.Args{
				"owner": holder.Address, "sft": sftAddr, "token_id": tid, "amount": uint64(1),
			}, holder); err != nil {
				if !seedHasCode(err.Error(), 1282, 1283) {
					return fmt.Errorf("burn: %w", err)
				}
				r.logf("  [%s/%s] burn token %d 已完成,跳过", sftSpec.Key, sc.Key, tid)
			} else {
				r.logf("  [%s/%s] burn token %d by %s ✓", sftSpec.Key, sc.Key, tid, op.By)
			}
			r.state.Mark("sft", stage, map[string]any{"op": "burn", "tokenId": tid})
		case "transfer":
			tid := tokenIds[0]
			to := r.acct(op.To)
			if _, err := seedSendSingle(r.mc, sftFlowIdentityApp, "Transfer", provider.Args{
				"owner": admin.Address, "sft": sftAddr, "token_id": tid,
				"to": to.Address, "amount": op.Amount,
			}, admin); err != nil {
				if !seedHasCode(err.Error(), 1282, 1283) {
					return fmt.Errorf("transfer: %w", err)
				}
				r.logf("  [%s/%s] transfer token %d 已完成,跳过", sftSpec.Key, sc.Key, tid)
			} else {
				r.logf("  [%s/%s] transfer token %d → %s ×%d ✓", sftSpec.Key, sc.Key, tid, op.To, op.Amount)
			}
			r.state.Mark("sft", stage, map[string]any{"op": "transfer", "tokenId": tid, "to": op.To})
		case "reject_transfer":
			by := r.acct(op.By)
			to := r.acct(op.To)
			tid := tokenIds[0]
			_, err := seedSendSingle(r.mc, sftFlowIdentityApp, "Transfer", provider.Args{
				"owner": by.Address, "sft": sftAddr, "token_id": tid,
				"to": to.Address, "amount": uint64(1),
			}, by)
			if err == nil {
				return fmt.Errorf("[%s/%s] 不可转移槽位竟然转移成功——环境异常!", sftSpec.Key, sc.Key)
			}
			r.logf("  [%s/%s] 拒绝转移实测 ✓ 被链端拦截", sftSpec.Key, sc.Key)
			r.state.Mark("sft", stage, map[string]any{"op": "reject_transfer"})
		case "freeze":
			holder := r.acct(op.By)
			tid := tokenIds[0]
			_, err := seedSendSingle(r.mc, sftFlowIdentityApp, "Freeze", provider.Args{
				"freezer": admin.Address, "sft": sftAddr, "token_id": tid, "owner": holder.Address,
			}, admin)
			// 1290=TokenFrozen(已冻结再冻结,幂等);1283=TokenNotFound(已完成销账)
			if err != nil && !seedHasCode(err.Error(), 1290, 1283) {
				return fmt.Errorf("freeze token: %w", err)
			}
			if err != nil {
				r.logf("  [%s/%s] freeze token %d 已完成,跳过", sftSpec.Key, sc.Key, tid)
			} else {
				r.logf("  [%s/%s] freeze token %d ✓ (owner=%s)", sftSpec.Key, sc.Key, tid, op.By)
			}
			r.state.Mark("sft", stage, map[string]any{"op": "freeze", "tokenId": tid})
		case "merge":
			if len(tokenIds) < 2 {
				r.logf("  [%s/%s] merge 需 ≥2 token,该 slot 仅 %d,跳过", sftSpec.Key, sc.Key, len(tokenIds))
				r.state.Mark("sft", stage, map[string]any{"op": "merge", "skipped": "insufficient-tokens"})
				break
			}
			holder := r.acct(op.By)
			from, to := tokenIds[0], tokenIds[1]
			_, err := seedSendSingle(r.mc, sftFlowIdentityApp, "Merge", provider.Args{
				"owner": holder.Address, "sft": sftAddr,
				"from_token_id": from, "to_token_id": to,
			}, holder)
			if err != nil && !seedHasCode(err.Error(), 1282, 1283) {
				return fmt.Errorf("merge token: %w", err)
			}
			if err != nil {
				r.logf("  [%s/%s] merge %d→%d 已完成,跳过", sftSpec.Key, sc.Key, from, to)
			} else {
				r.logf("  [%s/%s] merge token %d → %d ✓", sftSpec.Key, sc.Key, from, to)
			}
			r.state.Mark("sft", stage, map[string]any{"op": "merge", "from": from, "to": to})
		default:
			return fmt.Errorf("未知后置操作 %q", op.Op)
		}
		r.state.Save()
	}
	return nil
}

func seedPickTokenId(ids []uint64, index int) uint64 {
	if index < 0 {
		index = len(ids) + index
	}
	if index < 0 || index >= len(ids) {
		if len(ids) == 0 {
			return 0
		}
		return ids[0]
	}
	return ids[index]
}

// seedOverrideFromMap 把计划里的覆盖项 map 转 sftFlowMetadataOverride。
func seedOverrideFromMap(m map[string]any) *sftFlowMetadataOverride {
	if m == nil {
		return nil
	}
	out := &sftFlowMetadataOverride{}
	strPtr := func(key string) *string {
		if v, ok := m[key].(string); ok && v != "" {
			return &v
		}
		return nil
	}
	out.Name = strPtr("name")
	out.Symbol = strPtr("symbol")
	out.CoverUrl = strPtr("coverUrl")
	out.Metadata = strPtr("metadata")
	out.Attribute = strPtr("attribute")
	return out
}

// ---------- vc ----------

func (r *seedRunner) runVc() error {
	for _, vi := range r.plan.VcIssuers {
		issuer := r.acct(vi.IssuerName)
		for _, uname := range vi.Certifies {
			stage := fmt.Sprintf("flow:%s:%s", vi.IssuerName, uname)
			if r.state.IsDone("vc", stage) {
				continue
			}
			user := r.acct(uname)
			if err := r.seedVcFlow(issuer, user, vi); err != nil {
				return fmt.Errorf("vc %s→%s: %w", vi.IssuerName, uname, err)
			}
			r.state.Mark("vc", stage, map[string]any{"issuer": vi.IssuerName, "user": uname})
			r.state.Save()
		}
	}
	for _, op := range r.plan.VcLifecycle {
		stage := fmt.Sprintf("lifecycle:%s:%s:%s", op.Kind, op.IssuerName, op.User)
		if r.state.IsDone("vc", stage) {
			continue
		}
		if err := r.seedVcLifecycle(op); err != nil {
			return fmt.Errorf("vc lifecycle %s %s→%s: %w", op.Kind, op.IssuerName, op.User, err)
		}
		r.state.Mark("vc", stage, map[string]any{"kind": op.Kind, "user": op.User})
		r.state.Save()
	}
	total := 0
	for _, vi := range r.plan.VcIssuers {
		total += len(vi.Certifies)
	}
	r.logf("vc 模块完成: %d 人持证 / %d 条生命周期记录", total, len(r.plan.VcLifecycle))
	return nil
}

// seedVcFlow 单个 (issuer, user) 的完整证书链路(与 vc_flow_handler 七步一致)。
func (r *seedRunner) seedVcFlow(issuer, user *SeedPlanAccount, vi SeedVcIssuerSpec) error {
	mc := r.mc
	// [1] 双方领水(幂等)
	if _, err := flowEnsureFaucet(mc, *issuer.ADR, issuer.SK, issuer.PUB); err != nil {
		return fmt.Errorf("issuer faucet: %w", err)
	}
	if _, err := flowEnsureFaucet(mc, *user.ADR, user.SK, user.PUB); err != nil {
		return fmt.Errorf("user faucet: %w", err)
	}
	// [2][3] issuer 建 Organization DID + 注册组织(幂等)
	issuerOpts := &vcFlowDidOptions{Alias: issuer.Alias, Suffix: &issuer.AliasSuffix,
		AvatarURI: issuer.AvatarURL, AliasOptional: true}
	issuerDidStep, err := vcFlowEnsureDID(mc, *issuer.ADR, issuer.SK, issuer.PUB, "Organization", issuerOpts)
	if err != nil {
		return fmt.Errorf("issuer did: %w", err)
	}
	if strings.Contains(issuerDidStep.Detail, "alias occupied") {
		r.logf("    ⚠ 别名 %s 已被占用,跳过绑定(DID 本体不受影响)", issuer.Alias)
	}
	schemas := make([]string, 0, vi.SchemaCount)
	for i := 1; i <= vi.SchemaCount; i++ {
		schemas = append(schemas, fmt.Sprintf("%s%d", vi.SchemaPrefix, i))
	}
	if _, err := vcFlowEnsureOrganization(mc, *issuer.ADR, issuer.SK, issuer.PUB, schemas); err != nil {
		return fmt.Errorf("register organization: %w", err)
	}
	// [4] user 建 Personal DID(幂等)
	userOpts := &vcFlowDidOptions{Alias: user.Alias, Suffix: &user.AliasSuffix,
		AvatarURI: user.AvatarURL, AliasOptional: true}
	userDidStep, err := vcFlowEnsureDID(mc, *user.ADR, user.SK, user.PUB, "Personal", userOpts)
	if err != nil {
		return fmt.Errorf("user did: %w", err)
	}
	if strings.Contains(userDidStep.Detail, "alias occupied") {
		r.logf("    ⚠ 别名 %s 已被占用,跳过绑定(DID 本体不受影响)", user.Alias)
	}
	// [5] 链下签发 N 张
	chainID, err := flowChainID(mc)
	if err != nil {
		return fmt.Errorf("chain id: %w", err)
	}
	creds, err := buildVcFlowCredentials(chainID, *issuer.ADR, *user.ADR,
		issuer.SK, issuer.PUB, vi.SchemaPrefix, vi.SchemaCount, nil)
	if err != nil {
		return fmt.Errorf("build credentials: %w", err)
	}
	// [6] 逐张披露(幂等)
	for _, cred := range creds {
		if _, _, _, err := vcFlowDisclose(mc, *user.ADR, user.SK, user.PUB, *issuer.ADR, cred); err != nil {
			return fmt.Errorf("disclose %s: %w", cred.Schema, err)
		}
	}
	// [7] 回读验证:披露上链(OnChain)即链路完成;Valid=false 是冻结/撤销
	// 等生命周期状态的正常反映,记录不失败(对齐 Python 版语义)。
	items := vcFlowVerify(mc, *user.ADR, *issuer.ADR, creds)
	allOnChain := len(items) > 0
	invalid := []string{}
	for _, it := range items {
		if !it.OnChain {
			allOnChain = false
		}
		if !it.Valid {
			invalid = append(invalid, it.Schema)
		}
	}
	if !allOnChain {
		return fmt.Errorf("凭证披露丢失: %v", items)
	}
	if len(invalid) > 0 {
		r.logf("  %s → %s: %d 张上链 ✓,%v 状态非有效(冻结/撤销,正常生命周期数据)",
			vi.IssuerName, user.Name, len(creds), invalid)
	} else {
		r.logf("  %s → %s: %d 张凭证 全部有效 ✓", vi.IssuerName, user.Name, len(creds))
	}
	return nil
}

// seedVcLifecycle 冻结(FreezeVcAttestation 按 credential_id)/撤销(RevokeVcAttestation 按 schema)。
func (r *seedRunner) seedVcLifecycle(op SeedLifecycleOp) error {
	issuer := r.acct(op.IssuerName)
	subject := r.acct(op.User)
	var vi *SeedVcIssuerSpec
	for i := range r.plan.VcIssuers {
		if r.plan.VcIssuers[i].IssuerName == op.IssuerName {
			vi = &r.plan.VcIssuers[i]
		}
	}
	if vi == nil {
		return fmt.Errorf("issuer %s 不在 VC 计划中", op.IssuerName)
	}
	switch op.Kind {
	case "freeze":
		v, err := flowCallView(r.mc, vcFlowIdentityApp, "CredentialId", provider.Args{
			"issuer": issuer.Address, "credential_schema": vi.Schema,
		})
		if err != nil {
			return err
		}
		if f, ok := v.(*api.ApiFailure); ok {
			return fmt.Errorf("credential id: %s", f.Message)
		}
		credID, _ := v.(string)
		_, err = seedSendSingle(r.mc, vcFlowIdentityApp, "FreezeVcAttestation", provider.Args{
			"issuer": issuer.Address, "subject": subject.Address, "credential_id": credID,
		}, issuer)
		// 新节点码表与老 IDL 文档不一致(实测重复冻结报 1073),按消息语义容忍
		if err != nil && !containsFold(err.Error(), "frozen") && !seedHasCode(err.Error(), 1073, 1074, 1033) {
			return err
		}
	case "revoke":
		_, err := seedSendSingle(r.mc, vcFlowIdentityApp, "RevokeVcAttestation", provider.Args{
			"issuer": issuer.Address, "subject": subject.Address, "credential_schema": vi.Schema,
		}, issuer)
		if err != nil && !containsFold(err.Error(), "revoked") && !seedHasCode(err.Error(), 1033) {
			return err
		}
	default:
		return fmt.Errorf("未知生命周期操作 %q", op.Kind)
	}
	// 链上状态回读
	v, err := flowCallView(r.mc, vcFlowIdentityApp, "VcAttestationLifecycle", provider.Args{
		"subject": subject.Address, "credential_schema": vi.Schema, "issuer": issuer.Address,
	})
	if err == nil {
		r.logf("  %s %s→%s ✓ 链上状态=%v", op.Kind, op.IssuerName, op.User, v)
	} else {
		r.logf("  %s %s→%s ✓(状态回读失败: %v)", op.Kind, op.IssuerName, op.User, err)
	}
	return nil
}

func containsFold(s, sub string) bool {
	a, b := []byte(s), []byte(sub)
	for i := 0; i+len(b) <= len(a); i++ {
		match := true
		for j := range b {
			ca, cb := a[i+j], b[j]
			if ca >= 'A' && ca <= 'Z' {
				ca += 'a' - 'A'
			}
			if cb >= 'A' && cb <= 'Z' {
				cb += 'a' - 'A'
			}
			if ca != cb {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
