// handler/seed_runner_multisig.go
package handler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== 播种器:multisig 模块 ====================
// 语义对齐 gosdk example/create_multisig_demo:创建者 = 多签账户自身;
// 先领水,再建 DID,最后 CreateMultisig(owner=自身, signers=普通账号公钥,
// 等权 1, threshold=m)。幂等:ListSigners 视图签名者数一致 → 跳过注册。

// seedMultisigStageOrder 多签模块阶段执行顺序:faucet → did → create。
// DID 必须在 CreateMultisig 注册**之前**:注册后账户转为多签签名模式,
// 链端拒绝单公钥签名(error 285 PubkeyModeForbidden),DID 步骤会失败。
// 阶段状态键固定为 faucet:/did:/create: 前缀,调整顺序不改键名。
var seedMultisigStageOrder = []string{"faucet", "did", "create"}

// seedMultisigDidSkippable 判断多签 DID 失败是否属于「多签已激活,单公钥签名被拒」:
// 链端 285 PubkeyModeForbidden,或消息含 pubkey mode / signers list(大小写不敏感)。
// 语义取 "signers list" 而非裸 "signer":避免 ListSigners 视图的网络错误被误判可跳过。
// 该场景仅出现在续跑(旧状态 create 已注册而 did 未完成),降级为跳过。
func seedMultisigDidSkippable(errMsg string) bool {
	if seedHasCode(errMsg, 285) {
		return true
	}
	return containsFold(errMsg, "pubkey mode") || containsFold(errMsg, "signers list")
}

// seedMultisigSignersReady 幂等判定:视图返回的签名者条数与计划一致。
func seedMultisigSignersReady(want int, view []any) bool {
	return view != nil && len(view) == want && want > 0
}

// seedMultisigViewSigners 从 ListSigners 视图结果中取签名者 vec。
// 链端返回 tuple<Account, vec<tuple<PublicKey,u8,u8>>>——postcard 解码后
// 外层 tuple 是 []any(下标 1)或 map(键 "1"),签名者 vec 自身也有这两种形态。
// 解析失败返回 nil(预检退化,走注册路径)。
func seedMultisigViewSigners(view any) []any {
	if view == nil {
		return nil
	}
	outer := view
	switch v := view.(type) {
	case []any:
		if len(v) < 2 {
			return nil
		}
		outer = v[1]
	case map[string]any:
		outer = v["1"]
	}
	switch v := outer.(type) {
	case []any:
		return v
	case map[string]any:
		return seedTupleMapToSlice(v)
	}
	return nil
}

// seedTupleMapToSlice 把 postcard 序列/元组解码出的 map(键 "0","1",...)还原为有序切片。
func seedTupleMapToSlice(m map[string]any) []any {
	idx := make([]int, 0, len(m))
	for k := range m {
		n, err := strconv.Atoi(k)
		if err != nil {
			return nil
		}
		idx = append(idx, n)
	}
	sort.Ints(idx)
	out := make([]any, len(idx))
	for i, n := range idx {
		out[i] = m[strconv.Itoa(n)]
	}
	return out
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

		// 按阶段顺序执行(faucet → did → create,见 seedMultisigStageOrder):
		// DID 必须先于注册,注册后链端拒绝单公钥签名(285)
		for _, phase := range seedMultisigStageOrder {
			switch phase {
			case "faucet":
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

			case "did":
				if ms.DID {
					stage := "did:" + ms.Name
					if !r.state.IsDone("multisig", stage) {
						suffix := ms.AliasSuffix
						opts := &vcFlowDidOptions{Alias: ms.Alias, Suffix: &suffix, AliasOptional: true}
						step, err := vcFlowEnsureDID(r.mc, *a.ADR, a.SK, a.PUB, "Personal", opts)
						if err != nil {
							// 续跑兜底:create 阶段已注册多签(旧状态/旧版顺序残留)时,
							// 链端拒绝单公钥签名(285 PubkeyModeForbidden)——降级为
							// 跳过 DID 不阻断任务;其余错误照常返回
							if r.state.IsDone("multisig", "create:"+ms.Name) && seedMultisigDidSkippable(err.Error()) {
								r.logf("  ⚠ 多签已激活,无法单签建 DID,跳过(标记 skipped)")
								r.state.Mark("multisig", stage, map[string]any{"skipped": "multisig-active"})
								r.state.Save()
								continue
							}
							return fmt.Errorf("did %s: %w", ms.Name, err)
						}
						if strings.Contains(step.Detail, "alias occupied") {
							// 别名是装饰性数据:被其他地址占用时跳过绑定,DID 本体已建成
							r.logf("  ⚠ 别名 %s 已被占用,跳过绑定(DID 本体不受影响)", ms.Alias)
						}
						r.logf("  %s DID ✓", ms.Name)
						r.state.Mark("multisig", stage, nil)
						r.state.Save()
					}
				}

			case "create":
				stage := "create:" + ms.Name
				if !r.state.IsDone("multisig", stage) {
					registered := false
					if v, err := flowCallView(r.mc, "account", "ListSigners",
						provider.Args{"owner": a.Address}); err == nil {
						list := seedMultisigViewSigners(v)
						if seedMultisigSignersReady(len(ms.SignerNames), list) {
							registered = true
							r.logf("  %s 已注册多签(%d 签者),跳过", ms.Name, len(list))
						}
					} else {
						r.logf("  %s ListSigners 视图失败(%v),转注册路径", ms.Name, err)
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
						// 1059/1064 为 identity 域幂等码(保留无害);account 域真实码待 devNet
						// 冒烟确认,先以消息语义 AlreadyExist 兜底(大小写不敏感)。
						if err != nil && !seedHasCode(err.Error(), 1059, 1064) &&
							!containsFold(err.Error(), "alreadyexist") {
							return fmt.Errorf("create multisig %s: %w", ms.Name, err)
						}
						if err != nil {
							r.logf("  %s 链上已注册(幂等码/AlreadyExist),跳过", ms.Name)
						} else {
							r.logf("  %s 多签注册 ✓ %d-of-%d 签者 %v",
								ms.Name, ms.Threshold, len(ms.SignerNames), ms.SignerNames)
						}
					}
					r.state.Mark("multisig", stage, map[string]any{
						"address": a.Address, "threshold": ms.Threshold, "signers": ms.SignerNames})
					r.state.Save()
				}
			}
		}
	}
	r.logf("multisig 模块完成: %d 个多签账户", len(r.plan.Multisig))
	return nil
}
