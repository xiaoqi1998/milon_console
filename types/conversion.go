package types

import (
	"encoding/json"
	"fmt"

	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

// ParseSignatureMode parses a signature mode JSON object.
// Supported formats:
//   - {"type":"pubkey","publicKey":"base58_pk"}
//   - {"type":"multisig","index":2,"publicKey":"base58_pk"}
func ParseSignatureMode(modeMap map[string]interface{}) (milon.AccountSignatureMode, error) {
	typeVal, ok := modeMap["type"]
	if !ok {
		return nil, fmt.Errorf("signatureMode missing 'type' field")
	}

	typeStr, ok := typeVal.(string)
	if !ok {
		return nil, fmt.Errorf("signatureMode 'type' must be a string")
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
		return milon.PubKeySignatureMode{PublicKey: *pk}, nil
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
		return milon.MultisigKeySignatureMode{Index: index, PublicKey: *pk}, nil
	default:
		return nil, fmt.Errorf("unsupported signatureMode type: %s", typeStr)
	}
}

// ParseSignatureModeFromJSON parses a signature mode from raw JSON bytes.
func ParseSignatureModeFromJSON(raw json.RawMessage) (milon.AccountSignatureMode, error) {
	var modeMap map[string]interface{}
	if err := json.Unmarshal(raw, &modeMap); err != nil {
		return nil, fmt.Errorf("invalid signatureMode JSON: %w", err)
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
	addr, err := crypto.NewAddressFromStringRelaxed(addrStr)
	if err != nil {
		return crypto.Address{}, err
	}
	return *addr, nil
}
