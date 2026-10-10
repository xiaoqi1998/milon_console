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

		mode, err := ParseSignatureModeFromJSON(s.SignatureMode)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("signer[%d] invalid signatureMode: %w", i, err)
		}

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

		addresses = append(addresses, addr)
		sks = append(sks, sk)
		modes = append(modes, mode)
	}

	return addresses, sks, modes, nil
}
