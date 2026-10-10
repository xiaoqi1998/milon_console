package types

import (
	"encoding/json"
	"fmt"

	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
)

// ParseSignatureMode parses a signature mode JSON object.
// Supported formats:
//   - {"type":"pubkey","publicKey":"base58_pk"}
//   - {"type":"multisig","index":2,"publicKey":"base58_pk"}
//
// 宽容化（2026-10 AI 易用性修复）：type 缺省/空串视为 "pubkey"（pubkey 语义
// 不言自明，AI 高频省略）；解析失败统一附正确形态示例，调用方（多为 AI）
// 可据此自纠。
func ParseSignatureMode(modeMap map[string]interface{}) (lib.AccountSignatureMode, error) {
	typeVal, ok := modeMap["type"]
	if !ok {
		typeVal = "pubkey"
	}

	typeStr, ok := typeVal.(string)
	if !ok {
		return nil, fmt.Errorf("signatureMode 'type' must be a string")
	}
	if typeStr == "" {
		typeStr = "pubkey"
	}

	pkStr, ok := modeMap["publicKey"].(string)
	if !ok {
		return nil, fmt.Errorf("signatureMode missing or invalid 'publicKey' field")
	}

	pk, err := crypto.NewPublicKeyFromStringRelaxed(pkStr)
	if err != nil {
		return nil, fmt.Errorf("invalid publicKey in signatureMode: %w", err)
	}

	switch typeStr {
	case "pubkey":
		return lib.PubKeySignatureMode{PublicKey: *pk}, nil
	case "multisig":
		indexVal, ok := modeMap["index"]
		if !ok {
			return nil, fmt.Errorf("multisig signatureMode missing 'index' field")
		}
		// JSON numbers parse to float64 by default
		indexFloat, ok := indexVal.(float64)
		if !ok {
			return nil, fmt.Errorf("multisig signatureMode 'index' must be a number")
		}
		index := uint8(indexFloat)
		return lib.MultisigKeySignatureMode{Index: index, PublicKey: *pk}, nil
	default:
		return nil, fmt.Errorf("unsupported signatureMode type: %s", typeStr)
	}
}

// ParseSignatureModeFromJSON parses a signature mode from raw JSON bytes.
// 宽容化：对象被二次序列化成字符串再传（"{\"type\":...}"）时剥壳解析——
// 这是 AI 真实使用中的高频误用；所有失败统一附正确形态示例。
func ParseSignatureModeFromJSON(raw json.RawMessage) (lib.AccountSignatureMode, error) {
	mode, err := parseSignatureModeFromJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid signatureMode %s: %w；正确形态：{\"type\":\"pubkey\",\"publicKey\":\"<base58公钥>\"} 或 {\"type\":\"multisig\",\"index\":N,\"publicKey\":\"<base58公钥>\"}", string(raw), err)
	}
	return mode, nil
}

func parseSignatureModeFromJSON(raw json.RawMessage) (lib.AccountSignatureMode, error) {
	var modeMap map[string]interface{}
	if err := json.Unmarshal(raw, &modeMap); err != nil {
		var str string
		if json.Unmarshal(raw, &str) == nil && json.Unmarshal([]byte(str), &modeMap) == nil {
			return ParseSignatureMode(modeMap)
		}
		return nil, err
	}
	return ParseSignatureMode(modeMap)
}

// ParseSignatureModeParts 把 signatureMode 拆成组成字段，publicKey 允许缺省
// （handler 在持有对应私钥时自动派生——2026-10 AI 易用性）。
// 返回：typeStr（缺省 "pubkey"，空串表示完全未传）、pkBase58（可为空）、
// index（multisig 必填）。raw 为空时返回全空且无错误。
// 兼容对象被二次序列化成字符串的误用（与 ParseSignatureModeFromJSON 同规则）。
func ParseSignatureModeParts(raw json.RawMessage) (typeStr, pkBase58 string, index *int, err error) {
	if len(raw) == 0 {
		return "", "", nil, nil
	}
	var modeMap map[string]interface{}
	if uerr := json.Unmarshal(raw, &modeMap); uerr != nil {
		var str string
		if json.Unmarshal(raw, &str) == nil && json.Unmarshal([]byte(str), &modeMap) == nil {
			uerr = nil
		}
		if uerr != nil {
			return "", "", nil, fmt.Errorf("invalid signatureMode %s: %w；正确形态：{\"type\":\"pubkey\",\"publicKey\":\"<base58公钥>\"} 或 {\"type\":\"multisig\",\"index\":N,\"publicKey\":\"<base58公钥>\"}", string(raw), uerr)
		}
	}

	typeStr, _ = modeMap["type"].(string)
	if typeStr == "" {
		typeStr = "pubkey"
	}
	switch typeStr {
	case "pubkey":
	case "multisig":
		idx, ok := modeMap["index"]
		if !ok {
			return "", "", nil, fmt.Errorf("invalid signatureMode %s: multisig 缺 index；正确形态：{\"type\":\"multisig\",\"index\":N,\"publicKey\":\"<base58公钥>\"}", string(raw))
		}
		idxFloat, ok := idx.(float64)
		if !ok {
			return "", "", nil, fmt.Errorf("invalid signatureMode %s: index 必须是数字", string(raw))
		}
		i := int(idxFloat)
		index = &i
	default:
		return "", "", nil, fmt.Errorf("invalid signatureMode %s: 不支持的 type %q（可选 pubkey/multisig）", string(raw), typeStr)
	}

	if pkAny, ok := modeMap["publicKey"]; ok {
		pkStr, ok := pkAny.(string)
		if !ok || pkStr == "" {
			return "", "", nil, fmt.Errorf("invalid signatureMode %s: publicKey 必须是非空字符串", string(raw))
		}
		pkBase58 = pkStr
	}
	return typeStr, pkBase58, index, nil
}

// ParseSecretKey parses a secret key from hex or base58 string.
func ParseSecretKey(skStr string) (crypto.SecretKeyer, error) {
	return crypto.SecretKeyerFromStringRelaxed(skStr)
}

// ParsePublicKey parses a public key from hex or base58 string.
func ParsePublicKey(pkStr string) (*crypto.PublicKey, error) {
	return crypto.NewPublicKeyFromStringRelaxed(pkStr)
}

// ParseAddress parses an address from hex or base58 string.
func ParseAddress(addrStr string) (crypto.Address, error) {
	addr, err := crypto.NewAddressFromRelaxed(addrStr)
	if err != nil {
		return crypto.Address{}, err
	}
	return *addr, nil
}

// ParseSignerList parses a list of SignerEntry into parallel slices of addresses, secret keys, and signature modes.
// Validates that addresses are non-empty and unique. privateKey is parsed only if non-empty (required for write, optional for simulate).
// 宽容化（2026-10 AI 易用性）：signatureMode 缺省时若有 privateKey 则自动派生
// 公钥模式（按 entry.KeyType，缺省 secp256k1）；两者皆无则明确报错。
func ParseSignerList(signers []SignerEntry, requirePrivateKey bool) ([]crypto.Address, []crypto.SecretKeyer, []lib.AccountSignatureMode, error) {
	if len(signers) == 0 {
		return nil, nil, nil, fmt.Errorf("signers cannot be empty")
	}

	addresses := make([]crypto.Address, 0, len(signers))
	sks := make([]crypto.SecretKeyer, 0, len(signers))
	modes := make([]lib.AccountSignatureMode, 0, len(signers))

	seen := make(map[string]bool)
	for i, s := range signers {
		addr, err := ParseAddress(s.Address)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("signer[%d] invalid address: %w", i, err)
		}

		addrKey := string(addr.Bytes[:])
		if seen[addrKey] {
			return nil, nil, nil, fmt.Errorf("duplicate signer address: %s", s.Address)
		}
		seen[addrKey] = true

		var sk crypto.SecretKeyer
		if requirePrivateKey {
			if s.PrivateKey == "" {
				return nil, nil, nil, fmt.Errorf("signer[%d] privateKey is required for write mode", i)
			}
			sk, err = ParseSecretKey(s.PrivateKey)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("signer[%d] invalid privateKey: %w", i, err)
			}
		} else if s.PrivateKey != "" {
			sk, err = ParseSecretKey(s.PrivateKey)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("signer[%d] invalid privateKey: %w", i, err)
			}
		}

		var mode lib.AccountSignatureMode
		if len(s.SignatureMode) == 0 {
			// 缺省：从私钥派生公钥模式
			if sk == nil {
				return nil, nil, nil, fmt.Errorf(
					"signer[%d] signatureMode is required: 无私钥可自动派生，请传 {\"type\":\"pubkey\",\"publicKey\":\"<base58公钥>\"} 或提供 privateKey", i)
			}
			pk, err := derivePubKeyByType(sk, s.KeyType)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("signer[%d] 自动派生 signatureMode 失败: %w", i, err)
			}
			mode = lib.PubKeySignatureMode{PublicKey: *pk}
		} else {
			mode, err = ParseSignatureModeFromJSON(s.SignatureMode)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("signer[%d] invalid signatureMode: %w", i, err)
			}
		}

		addresses = append(addresses, addr)
		sks = append(sks, sk)
		modes = append(modes, mode)
	}

	return addresses, sks, modes, nil
}

// derivePubKeyByType 按曲线名从私钥派生公钥；keyType 空串缺省 secp256k1
// （与全服务端约定一致：32 字节经典私钥的曲线由调用方声明）。
// fndsa512 无法从私钥派生（Go SDK 未实现），返回明确指引。
func derivePubKeyByType(sk crypto.SecretKeyer, keyType string) (*crypto.PublicKey, error) {
	if keyType == "" {
		keyType = "secp256k1"
	}
	classical := crypto.AsClassicalSecretKey(sk)
	if classical == nil {
		return nil, fmt.Errorf("keyType %s 无法从该私钥派生公钥：请显式传 signatureMode.publicKey（account_generate 的返回里有）", keyType)
	}
	switch keyType {
	case "secp256k1":
		return classical.Secp256k1Public()
	case "ed25519":
		return classical.Ed25519Public(), nil
	case "bls12381":
		return classical.BLS12381Public(), nil
	default:
		return nil, fmt.Errorf("unsupported keyType: %s (supported: secp256k1, ed25519, bls12381, fndsa512)", keyType)
	}
}
