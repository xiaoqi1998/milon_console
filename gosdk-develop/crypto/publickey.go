package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/btcutil/base58"
	"github.com/cloudflare/circl/sign/bls"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/milon-labs/milon-go-sdk/postcard"
	blst "github.com/supranational/blst/bindings/go"
	"strings"
)

const (
	PublicKeySecp256k1Size = 33
	PublicKeyEd25519Size   = 32
	PublicKeyBLS12381Size  = 48
	PublicKeyFnDsa512Size  = 897
)

type PublicKeyType uint8

const (
	PublicKeyTypeSecp256k1 PublicKeyType = iota
	PublicKeyTypeEd25519
	PublicKeyTypeBLS12381
	PublicKeyTypeFnDsa512
)

// NewPublicKeyFromBytes parses raw bytes into a PublicKey based on length and curve rules
func NewPublicKeyFromBytes(raw []byte) (*PublicKey, error) {
	switch len(raw) {
	case PublicKeySecp256k1Size:
		if _, err := secp256k1.ParsePubKey(raw); err != nil {
			return nil, fmt.Errorf("invalid secp256k1 public key: %w", err)
		}
		return &PublicKey{
			Variant: PublicKeyTypeSecp256k1,
			Bytes:   raw,
		}, nil
	case PublicKeyEd25519Size:
		if len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid ed25519 public key length: %d", len(raw))
		}
		return &PublicKey{
			Variant: PublicKeyTypeEd25519,
			Bytes:   raw,
		}, nil
	case PublicKeyBLS12381Size:
		var pub bls.PublicKey[bls.G1]
		if err := pub.UnmarshalBinary(raw); err != nil {
			return nil, fmt.Errorf("invalid BLS public key: %w", err)
		}
		return &PublicKey{
			Variant: PublicKeyTypeBLS12381,
			Bytes:   raw,
		}, nil
	case PublicKeyFnDsa512Size:
		return &PublicKey{
			Variant: PublicKeyTypeFnDsa512,
			Bytes:   raw,
		}, nil
	default:
		return nil, ErrInvalidPublicKey
	}
}

func NewPublicKeyFromStringRelaxed(s string) (*PublicKey, error) {
	s = strings.TrimSpace(s)

	if pk, err := newPublicKeyFromHex(s); err == nil {
		return pk, nil
	}

	return newPublicKeyFromBase58(s)
}
func newPublicKeyFromHex(s string) (*PublicKey, error) {
	s = strings.TrimSpace(s)

	hexStr := s
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		hexStr = s[2:]
	}

	buf, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}

	return NewPublicKeyFromBytes(buf)
}

func newPublicKeyFromBase58(b58Str string) (*PublicKey, error) {
	buf := base58.Decode(b58Str)
	if len(buf) == 0 {
		return nil, errors.New("invalid base58 string")
	}
	return NewPublicKeyFromBytes(buf)
}

type PublicKey struct {
	Variant PublicKeyType
	Bytes   []byte
}

func (pk *PublicKey) AsBytes() []byte {
	return pk.Bytes
}

func (pk *PublicKey) ToHex() string {
	return hex.EncodeToString(pk.Bytes)
}

func (pk *PublicKey) ToBase58() string {
	return base58.Encode(pk.Bytes)
}

// String implements the Stringer interface, returning Base58 format
func (pk *PublicKey) String() string {
	return pk.ToBase58()
}

func (pk *PublicKey) ToSecp256k1() (*secp256k1.PublicKey, error) {
	if pk.Variant != PublicKeyTypeSecp256k1 {
		return nil, ErrInvalidPublicKey
	}
	return secp256k1.ParsePubKey(pk.Bytes)
}

func (pk *PublicKey) ToEd25519() (ed25519.PublicKey, error) {
	if pk.Variant != PublicKeyTypeEd25519 {
		return nil, fmt.Errorf("not an ed25519 public key, actual type: %d", pk.Variant)
	}

	if len(pk.Bytes) != PublicKeyEd25519Size {
		return nil, fmt.Errorf("invalid ed25519 public key length: expected %d, got %d", PublicKeyEd25519Size, len(pk.Bytes))
	}

	return pk.Bytes, nil
}

func (pk *PublicKey) ToBLS12381() (*blst.P1Affine, error) {
	if pk.Variant != PublicKeyTypeBLS12381 {
		return nil, fmt.Errorf("not an BLS12-381 public key, actual type: %d", pk.Variant)
	}

	if len(pk.Bytes) != PublicKeyBLS12381Size {
		return nil, fmt.Errorf("invalid BLS12-381 public key length: expected %d, got %d", PublicKeyBLS12381Size, len(pk.Bytes))
	}

	p1Affine := new(blst.P1Affine).Uncompress(pk.Bytes)
	if p1Affine == nil {
		return nil, fmt.Errorf("failed to parse BLS12-381 public key from Bytes")
	}

	return p1Affine, nil
}

func (pk *PublicKey) IsSecp256k1() bool {
	return pk.Variant == PublicKeyTypeSecp256k1
}

func (pk *PublicKey) IsEd25519() bool {
	return pk.Variant == PublicKeyTypeEd25519
}

func (pk *PublicKey) IsBLS12381() bool {
	return pk.Variant == PublicKeyTypeBLS12381
}

func (pk *PublicKey) IsFnDsa512() bool {
	return pk.Variant == PublicKeyTypeFnDsa512
}

// FromSecp256k1Native converts from a secp256k1 public key back to PublicKey
func (pk *PublicKey) FromSecp256k1Native(p *secp256k1.PublicKey) error {
	if p == nil {
		return fmt.Errorf("secp256k1 public key is nil")
	}

	publicKeyCompressedBytes := p.SerializeCompressed()

	if len(publicKeyCompressedBytes) != PublicKeySecp256k1Size {
		return fmt.Errorf("invalid secp256k1 public key length: expected %d, got %d", PublicKeySecp256k1Size, len(publicKeyCompressedBytes))
	}

	pk.Variant = PublicKeyTypeSecp256k1
	pk.Bytes = make([]byte, len(publicKeyCompressedBytes))
	copy(pk.Bytes, publicKeyCompressedBytes)

	return nil
}

// FromEd25519Native converts from an ed25519 public key back to PublicKey
func (pk *PublicKey) FromEd25519Native(vk ed25519.PublicKey) error {
	if len(vk) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid ed25519 public key length: expected %d, got %d", ed25519.PublicKeySize, len(vk))
	}

	pk.Variant = PublicKeyTypeEd25519
	pk.Bytes = make([]byte, len(vk))
	copy(pk.Bytes, vk)

	return nil
}

// FromBLS12381Native converts from a BLS12-381 public key back to PublicKey
func (pk *PublicKey) FromBLS12381Native(p1Affine *blst.P1Affine) error {
	p1AffineBytes := p1Affine.Compress()

	if len(p1AffineBytes) != PublicKeyBLS12381Size {
		return fmt.Errorf("invalid BLS12-381 public key length: expected %d, got %d", PublicKeyBLS12381Size, len(p1AffineBytes))
	}

	pk.Variant = PublicKeyTypeBLS12381
	pk.Bytes = make([]byte, len(p1AffineBytes))
	copy(pk.Bytes, p1AffineBytes)

	return nil
}

func (pk *PublicKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(pk.ToBase58())
}

func (pk *PublicKey) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	newPk, err := NewPublicKeyFromStringRelaxed(s)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	*pk = *newPk
	return nil
}

func (pk *PublicKey) MarshalPostcard(serializer *postcard.Serializer) error {
	err := serializer.SerializeU32(uint32(pk.Variant))
	if err != nil {
		return fmt.Errorf("failed to serialize public key variant: %w", err)
	}

	serializer.SerializeFixedBytes(pk.Bytes)
	return nil
}

func (pk *PublicKey) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	variant, err := deserializer.DeserializeU32()
	if err != nil {
		return fmt.Errorf("failed to deserialize public key variant: %w", err)
	}

	var expectedLen int
	switch PublicKeyType(variant) {
	case PublicKeyTypeSecp256k1:
		expectedLen = PublicKeySecp256k1Size
	case PublicKeyTypeEd25519:
		expectedLen = PublicKeyEd25519Size
	case PublicKeyTypeBLS12381:
		expectedLen = PublicKeyBLS12381Size
	case PublicKeyTypeFnDsa512:
		expectedLen = PublicKeyFnDsa512Size
	default:
		return fmt.Errorf("unknown public key variant: %d", variant)
	}

	buf, err := deserializer.DeserializeFixedBytes(expectedLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize public key Bytes: %w", err)
	}

	newPk, err := NewPublicKeyFromBytes(buf)
	if err != nil {
		return fmt.Errorf("failed to create public key from Bytes: %w", err)
	}

	pk.Variant = PublicKeyType(variant)
	pk.Bytes = newPk.Bytes
	return nil
}
