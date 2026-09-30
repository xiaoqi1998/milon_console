package handler

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"milon-api-server/types"

	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== 流程编排端点共用基础能力 ====================
//
// /api/tool/vc-flow、/api/tool/sft-flow 等流程编排端点共用的链上基础步骤:
// 身份解析、gas 保障、view 调用、交易构建/提交、幂等容错判断。
// 各端点只负责自己的业务编排,不直接触碰底层 SDK 细节。

const flowGasThreshold = 100 * 1_000_000 // 余额 ≥ 100 MIL 视为无需领水

// flowStep 一步链上写操作的幂等执行结果。
type flowStep struct {
	Skipped bool   `json:"skipped"`
	TxHash  string `json:"txHash,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// flowFaucetStep 领水步骤结果。
type flowFaucetStep struct {
	Skipped       bool   `json:"skipped"`
	Claimed       bool   `json:"claimed"`
	TxHash        string `json:"txHash,omitempty"`
	BalanceBefore string `json:"balanceBefore"`
	BalanceAfter  string `json:"balanceAfter"`
	Detail        string `json:"detail,omitempty"`
}

// flowChainID 从链头取 chain ID(链下签名摘要的组成部分)。
func flowChainID(mc *milon.Client) (int, error) {
	result, err := mc.GetChainHead(milon.WithRequestID(lib.RequestID(time.Now().UnixMilli())))
	if err != nil {
		return 0, fmt.Errorf("failed to get chain head: %w", err)
	}
	return int(result.BodyChainHead.ChainId), nil
}

// flowCallView 执行一次 view 调用并解码。返回值可能是链端 Err 载荷
// (*api.TxFailurePayload),用 flowViewOK 区分。
func flowCallView(mc *milon.Client, appName, method string, args provider.Args) (any, error) {
	pd, ok := mc.GetAllPd()[appName]
	if !ok {
		return nil, fmt.Errorf("IDL app %q not found", appName)
	}
	wire, err := encodeWithCoercion(pd, method, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode view %s.%s: %w", appName, method, err)
	}
	result, err := mc.View([]api.PackedInstruction{wire}, milon.WithRequestID(lib.RequestID(time.Now().UnixMilli())))
	if err != nil {
		return nil, fmt.Errorf("view %s.%s failed: %w", appName, method, err)
	}
	value, err := pd.DecodeViewData(method, result.HTTPResponseBody)
	if err != nil {
		return nil, fmt.Errorf("failed to decode view %s.%s: %w", appName, method, err)
	}
	return value, nil
}

// flowViewOK 判断 view 解码值是否为 Ok 载荷(而非链端 Err 载荷)。
func flowViewOK(value any) bool {
	_, isFailure := value.(*api.TxFailurePayload)
	return !isFailure
}

// flowBuildTx 构建一笔「自付 gas + 签 ix0」的交易(等价 /api/write 的 unified_payer_all)。
// 签名模式按账户链上状态自动修正(pubkey → 签名者列表)。
func flowBuildTx(mc *milon.Client, appName, method string, args provider.Args, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey) (*lib.Transaction, error) {
	pd, ok := mc.GetAllPd()[appName]
	if !ok {
		return nil, fmt.Errorf("IDL app %q not found", appName)
	}
	wire, err := encodeWithCoercion(pd, method, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s.%s: %w", appName, method, err)
	}
	mode, err := normalizeSignatureModeForAccount(mc, addr, lib.PubKeySignatureMode{PublicKey: *pub})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve signature mode for %s: %w", addr.ToBase58(), err)
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(&addr).
		AddIxAndPayerSig(addr, sk, 0, mode).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %s.%s: %w", appName, method, err)
	}
	return tx, nil
}

// flowSendTx 提交交易并等待确认,返回 txHash;链端执行错误以 error 返回
// (txHash 一并返回,便于追踪已提交但执行失败的交易)。
func flowSendTx(mc *milon.Client, tx *lib.Transaction) (string, error) {
	if err := mc.SubmitTx(tx, milon.WithRequestID(lib.RequestID(time.Now().UnixMilli()))); err != nil {
		return "", err
	}
	txHash := txHashHex(tx)
	if _, err := mc.WaitForTransaction(txHash, milon.WithWaitRequestID(lib.RequestID(1))); err != nil {
		return txHash, err
	}
	return txHash, nil
}

// flowEnsureFaucet 确保地址有足够 gas:
//   - 余额 ≥ 阈值:跳过领水
//   - 领水失败(典型:24h 冷却)但余额 ≥ 阈值:放行并在 Detail 说明
//   - 领水成功:确认到账后返回
func flowEnsureFaucet(mc *milon.Client, addr crypto.Address, sk crypto.SecretKeyer, pub *crypto.PublicKey) (flowFaucetStep, error) {
	step := flowFaucetStep{}
	before, err := mc.BalanceOf(&addr)
	if err != nil {
		// 全新账户首次上链前链上无 Account 资源,查余额报「账户不存在」→ 视为余额 0
		if !isAccountNotFoundErr(err.Error()) {
			return step, fmt.Errorf("failed to query balance of %s: %w", addr.ToBase58(), err)
		}
		before = 0
	}
	step.BalanceBefore = fmt.Sprint(before)
	step.BalanceAfter = step.BalanceBefore

	if before >= flowGasThreshold {
		step.Skipped = true
		step.Detail = "balance is sufficient, skip faucet"
		return step, nil
	}

	mode, err := normalizeSignatureModeForAccount(mc, addr, lib.PubKeySignatureMode{PublicKey: *pub})
	if err != nil {
		return step, fmt.Errorf("faucet signature mode: %w", err)
	}
	tx, err := buildClaimFaucetTx(mc, addr, sk, mode)
	if err != nil {
		return step, fmt.Errorf("build faucet tx: %w", err)
	}
	if err := submitClaimFaucetTx(mc, tx); err != nil {
		// 常见幂等场景:24h 内重复领水被拒,但余额足够 → 放行
		if after, berr := mc.BalanceOf(&addr); berr == nil && after >= flowGasThreshold {
			step.Skipped = true
			step.BalanceAfter = fmt.Sprint(after)
			step.Detail = "faucet rejected but balance is sufficient: " + err.Error()
			return step, nil
		}
		return step, fmt.Errorf("claim faucet: %w", err)
	}

	txHash := txHashHex(tx)
	if _, err := mc.WaitForTransaction(txHash, milon.WithWaitRequestID(lib.RequestID(1))); err != nil {
		if after, berr := mc.BalanceOf(&addr); berr == nil && after >= flowGasThreshold {
			step.Claimed = true
			step.TxHash = txHash
			step.BalanceAfter = fmt.Sprint(after)
			step.Detail = "faucet submitted, balance confirms funding (wait error ignored): " + err.Error()
			return step, nil
		}
		return step, fmt.Errorf("wait faucet tx %s: %w", txHash, err)
	}

	after, err := mc.BalanceOf(&addr)
	if err != nil {
		return step, fmt.Errorf("re-query balance: %w", err)
	}
	step.Claimed = true
	step.TxHash = txHash
	step.BalanceAfter = fmt.Sprint(after)
	return step, nil
}

// isTolerableChainError 判断链端错误是否属于幂等容忍范围(错误名或错误码子串匹配)。
func isTolerableChainError(errMsg string, tolerable ...string) bool {
	if errMsg == "" {
		return false
	}
	for _, sub := range tolerable {
		if strings.Contains(errMsg, sub) {
			return true
		}
	}
	return false
}

// isAccountNotFoundErr 判断是否为「账户不存在」错误(code=512):
// 全新账户首次上链前查余额的预期错误,应视为余额 0 而非失败。
func isAccountNotFoundErr(msg string) bool {
	return strings.Contains(msg, "code=512") ||
		strings.Contains(msg, "账户不存在") ||
		strings.Contains(msg, "account not exist")
}

// resolveFlowParty 解析流程参与方身份:私钥必填。
// role 用于错误信息区分角色("issuer"/"user"/"owner"...)——同一错误在不同角色
// 身上措辞不同,避免出现「user 传 FN-DSA-512 私钥缺公钥,报错却让补 issuerPublicKey」
// 的误导。32 字节经典私钥默认按 Ed25519 解释(与 /api/util/vc-attestation 一致);
// 显式传入的地址与 Ed25519 派生地址不一致时,自动尝试 secp256k1 / bls12381
// 曲线解释(同一 32 字节私钥在不同曲线下派生不同地址,以显式地址为准)。
// FN-DSA-512(1281 字节)私钥须额外传公钥(SDK 无法从签名密钥反推)。
func resolveFlowParty(role, privKey, pubKey, addrStr string) (crypto.SecretKeyer, *crypto.PublicKey, crypto.Address, error) {
	// FN-DSA-512 缺公钥的前置检查:resolveIssuerIdentity 的错误措辞固定为 issuer,
	// 这里按角色先行给出准确报错
	if fk, err := crypto.SecretKeyerFromStringRelaxed(privKey); err == nil && crypto.AsFnDsa512SecretKey(fk) != nil && strings.TrimSpace(pubKey) == "" {
		return nil, nil, crypto.Address{}, fmt.Errorf(
			"%sPublicKey is required when %s private key is FN-DSA-512(1281字节后量子私钥无法反推公钥,请传账户生成时返回的 %sPublicKey)",
			role, role, role)
	}

	sk, pub, addr, err := resolveIssuerIdentity(privKey, pubKey)
	if err != nil {
		// 其余错误措辞同样按角色改写(resolveIssuerIdentity 的文案均以 issuer 主语)
		rewritten := strings.ReplaceAll(err.Error(), "issuer", role)
		return nil, nil, crypto.Address{}, errors.New(rewritten)
	}
	explicit := strings.TrimSpace(addrStr)
	if explicit == "" {
		return sk, pub, *addr, nil
	}
	want, perr := types.ParseAddress(explicit)
	if perr != nil {
		return nil, nil, crypto.Address{}, fmt.Errorf("invalid address %q: %w", explicit, perr)
	}
	if want.ToBase58() == addr.ToBase58() {
		return sk, pub, *addr, nil
	}

	// 曲线回退:经典私钥换 secp256k1 / bls12381 解释再比对
	if ck := crypto.AsClassicalSecretKey(sk); ck != nil {
		blsDerive := func() (*crypto.PublicKey, error) { return ck.BLS12381Public(), nil }
		for _, derive := range []func() (*crypto.PublicKey, error){ck.Secp256k1Public, blsDerive} {
			altPub, derr := derive()
			if derr != nil {
				continue
			}
			altAddr, aerr := crypto.NewAddressFromPublicKey(altPub)
			if aerr != nil {
				continue
			}
			if altAddr.ToBase58() == want.ToBase58() {
				return sk, altPub, *altAddr, nil
			}
		}
	}
	return nil, nil, crypto.Address{}, fmt.Errorf(
		"address %s does not match any address derived from the private key (ed25519: %s, plus secp256k1/bls12381 variants)",
		explicit, addr.ToBase58())
}
