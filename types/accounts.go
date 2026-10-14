package types

import (
	"encoding/json"
	"fmt"

	"github.com/milon-labs/milon-go-sdk/crypto"
)

// ConfiguredAccount 是客户端配置的操作账户（X-Milon-Accounts / MILON_ACCOUNTS
// 的数组元素）。解析归一后 PublicKey 恒为 hex 全文（显式或从私钥派生），
// KeyType 恒为四曲线之一（缺省 secp256k1）——下游指纹与注入统一取用。
type ConfiguredAccount struct {
	Name       string `json:"name,omitempty"`
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey,omitempty"`
	KeyType    string `json:"keyType,omitempty"`
}

// MaxConfiguredAccounts 是单次请求允许携带的配置账户数上限：
// 防客户端误贴超大数组把解析/校验拖成 CPU 热点（在逐条校验之前拦截）。
const MaxConfiguredAccounts = 64

// ConfiguredAccountHint 是「私钥必填但地址未配置」错误的追加提示（types 侧
// 独立定义以避免 import handler 成环；文案与 handler.configuredAccountHint
// 逐字一致，改文案须两侧同步）。
const ConfiguredAccountHint = "；该地址未配置为操作账户——可在 MCP 客户端 mcp.json 的 milon.headers 配 X-Milon-Accounts（base64(JSON数组)，fndsa512 须带 publicKey）后免传私钥，或先调 account_config_list 核对已配置账户"

// ParseConfiguredAccounts 解析并校验一段账户 JSON 数组（raw 为 base64 解码后
// 的 JSON 原文，或 MILON_ACCOUNTS 环境变量原文）。sourceName 是来源名
// （"X-Milon-Accounts" / "MILON_ACCOUNTS"），用于错误文案定位，避免 env
// 配错时误导排障方向。
// 校验（任一失败即整包拒绝，错误定位到条目下标）：
//  0. 条目数不超过 MaxConfiguredAccounts（JSON 解析后先查，逐条校验前拦）；
//  1. address 非空且为合法地址；
//  2. keyType 归一到四曲线之一（secp256k1/ed25519/bls12381/fndsa512）；
//  3. privateKey 非空且可解析；
//  4. 公钥确定：显式 publicKey 优先；fndsa512 缺 publicKey 直接拒绝
//     （Go 无法从其私钥派生）；其余曲线从私钥按曲线派生；
//  5. 公钥派生地址须与 address 一致（防配错 key）；
//  6. 数组内地址唯一。
func ParseConfiguredAccounts(sourceName string, raw []byte) ([]ConfiguredAccount, error) {
	var accounts []ConfiguredAccount
	if err := json.Unmarshal(raw, &accounts); err != nil {
		return nil, fmt.Errorf("%s 不是合法 JSON 数组: %w", sourceName, err)
	}
	if len(accounts) > MaxConfiguredAccounts {
		return nil, fmt.Errorf("%s: 账户数 %d 超过上限 %d", sourceName, len(accounts), MaxConfiguredAccounts)
	}
	seen := make(map[string]bool, len(accounts))
	for i := range accounts {
		a := &accounts[i]
		where := fmt.Sprintf("%s[%d]", sourceName, i)
		if a.Address == "" {
			return nil, fmt.Errorf("%s: address is required", where)
		}
		addr, err := ParseAddress(a.Address)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid address: %w", where, err)
		}
		// 去重按归一键（地址串行化），防同一地址 hex/base58 双形态绕过唯一性
		addrKey := addr.String()
		if a.KeyType == "" {
			a.KeyType = "secp256k1"
		}
		switch a.KeyType {
		case "secp256k1", "ed25519", "bls12381", "fndsa512":
		default:
			return nil, fmt.Errorf("%s: unsupported keyType %q (supported: secp256k1/ed25519/bls12381/fndsa512)", where, a.KeyType)
		}
		if a.PrivateKey == "" {
			return nil, fmt.Errorf("%s: privateKey is required", where)
		}
		sk, err := ParseSecretKey(a.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid privateKey: %w", where, err)
		}
		var pk *crypto.PublicKey
		if a.PublicKey != "" {
			pk, err = ParsePublicKey(a.PublicKey)
			if err != nil {
				return nil, fmt.Errorf("%s: invalid publicKey: %w", where, err)
			}
			// 私钥配对校验：经典曲线用同曲线派生结果比对显式公钥，
			// 防止「私钥 K2 + 公钥 P1（P1 与 address 配对）」配错组合
			// 整体通过；fndsa512 无法从私钥派生，豁免（缺 publicKey 已被
			// 上游路径拒绝，走到这里必显式携带）。
			if a.KeyType != "fndsa512" {
				derivedPk, derr := derivePubKeyByType(sk, a.KeyType)
				if derr != nil {
					return nil, fmt.Errorf("%s: 无法从私钥派生公钥以校验配对: %w", where, derr)
				}
				if derivedPk.ToHex() != pk.ToHex() {
					return nil, fmt.Errorf("%s: privateKey 与 publicKey 不配对（同曲线派生结果不一致）", where)
				}
			}
		} else if a.KeyType == "fndsa512" {
			return nil, fmt.Errorf("%s: keyType fndsa512 必须显式携带 publicKey（Go 无法从私钥派生；account_generate 的返回里有）", where)
		} else {
			pk, err = derivePubKeyByType(sk, a.KeyType)
			if err != nil {
				return nil, fmt.Errorf("%s: 无法从私钥派生公钥: %w", where, err)
			}
		}
		derived, err := crypto.NewAddressFromPublicKey(pk)
		if err != nil {
			return nil, fmt.Errorf("%s: 公钥无法派生地址: %w", where, err)
		}
		if derived.String() != addr.String() {
			return nil, fmt.Errorf("%s: 公钥派生地址 %s 与配置 address %s 不一致（key 配错）", where, derived.String(), a.Address)
		}
		a.PublicKey = pk.ToHex() // 归一为 hex 全文，供指纹与注入
		if seen[addrKey] {
			return nil, fmt.Errorf("%s: 重复地址 %s", where, a.Address)
		}
		seen[addrKey] = true
	}
	return accounts, nil
}
