// handler/seed_runner_multisig.go
package handler

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== 播种器:multisig 模块 ====================
// 语义对齐 gosdk example/create_multisig_demo:创建者 = 多签账户自身;
// 先领水,再 CreateMultisig(owner=自身, signers=普通账号公钥, 等权 1, threshold=m),
// 可选建 DID。幂等:ListSigners 视图签名者数一致 → 跳过注册。

// seedMultisigSignersReady 幂等判定:视图返回的签名者条数与计划一致。
func seedMultisigSignersReady(want int, view []any) bool {
	return view != nil && len(view) == want && want > 0
}

// seedMultisigWeights 等权 bytes:每个签名者权重 1。
func seedMultisigWeights(n int) []uint8 {
	w := make([]uint8, n)
	for i := range w {
		w[i] = 1
	}
	return w
}

func (r *seedRunner) runMultisig() error {
	for _, ms := range r.plan.Multisig {
		a := r.acct(ms.Name)

		if ms.Faucet {
			stage := "faucet:" + ms.Name
			if !r.state.IsDone("multisig", stage) {
				if _, err := flowEnsureFaucet(r.mc, *a.ADR, a.SK, a.PUB); err != nil {
					return fmt.Errorf("faucet %s: %w", ms.Name, err)
				}
				r.logf("  %s 领水 ✓ (%s)", ms.Name, shortAddr(a.Address))
				r.state.Mark("multisig", stage, nil)
				r.state.Save()
			}
		}

		stage := "create:" + ms.Name
		if !r.state.IsDone("multisig", stage) {
			registered := false
			if v, err := flowCallView(r.mc, "account", "ListSigners",
				provider.Args{"owner": a.Address}); err == nil {
				if list, ok := v.([]any); ok && seedMultisigSignersReady(len(ms.SignerNames), list) {
					registered = true
					r.logf("  %s 已注册多签(%d 签者),跳过", ms.Name, len(list))
				}
			}
			if !registered {
				signers := make([]any, len(ms.SignerNames))
				for j, sn := range ms.SignerNames {
					signers[j] = r.acct(sn).PUB
				}
				_, err := seedSendSingle(r.mc, "account", "CreateMultisig", provider.Args{
					"owner": a.Address, "signers": signers,
					"weights": seedMultisigWeights(len(ms.SignerNames)), "threshold": ms.Threshold,
				}, a)
				if err != nil && !seedHasCode(err.Error(), 1059, 1064) {
					return fmt.Errorf("create multisig %s: %w", ms.Name, err)
				}
				if err != nil {
					r.logf("  %s 链上已注册(幂等码),跳过", ms.Name)
				} else {
					r.logf("  %s 多签注册 ✓ %d-of-%d 签者 %v",
						ms.Name, ms.Threshold, len(ms.SignerNames), ms.SignerNames)
				}
			}
			r.state.Mark("multisig", stage, map[string]any{
				"address": a.Address, "threshold": ms.Threshold, "signers": ms.SignerNames})
			r.state.Save()
		}

		if ms.DID {
			stage = "did:" + ms.Name
			if !r.state.IsDone("multisig", stage) {
				suffix := ms.AliasSuffix
				opts := &vcFlowDidOptions{Alias: ms.Alias, Suffix: &suffix}
				if _, err := vcFlowEnsureDID(r.mc, *a.ADR, a.SK, a.PUB, "Personal", opts); err != nil {
					return fmt.Errorf("did %s: %w", ms.Name, err)
				}
				r.logf("  %s DID ✓", ms.Name)
				r.state.Mark("multisig", stage, nil)
				r.state.Save()
			}
		}
	}
	r.logf("multisig 模块完成: %d 个多签账户", len(r.plan.Multisig))
	return nil
}
