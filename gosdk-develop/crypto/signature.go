package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/btcutil/base58"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hdevalence/ed25519consensus"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/pornin/go-fn-dsa/fndsa"
	blst "github.com/supranational/blst/bindings/go"
	"lukechampine.com/blake3"
	"strings"
)

const (
	SignatureSecp256k1Size = 65
	SignatureEd25519Size   = 64
	SignatureBLS12381Size  = 96
	SignatureFnDsa512Size  = 666
)

type SignatureType uint8

const (
	SignatureTypeSecp256k1 SignatureType = iota
	SignatureTypeEd25519
	SignatureTypeBLS12381
	SignatureTypeFnDsa512
)

// NewSignatureFromBytes creates a Signature from raw bytes, auto-detecting the type by length.
func NewSignatureFromBytes(raw []byte) (*Signature, error) {
	switch len(raw) {
	case SignatureEd25519Size:
		return &Signature{
			Variant: SignatureTypeEd25519,
			Bytes:   raw,
		}, nil
	case SignatureBLS12381Size:
		return &Signature{
			Variant: SignatureTypeBLS12381,
			Bytes:   raw,
		}, nil
	case SignatureSecp256k1Size:
		return &Signature{
			Variant: SignatureTypeSecp256k1,
			Bytes:   raw,
		}, nil
	case SignatureFnDsa512Size:
		return &Signature{
			Variant: SignatureTypeFnDsa512,
			Bytes:   raw,
		}, nil
	default:
		return nil, ErrInvalidSignature
	}
}

// NewSignatureFromStringRelaxed parses a hex (with optional 0x prefix) or Base58 string.
func NewSignatureFromStringRelaxed(s string) (*Signature, error) {
	s = strings.TrimSpace(s)

	if sg, err := newSignatureFromHex(s); err == nil {
		return sg, nil
	}

	return newSignatureFromBase58(s)
}
func newSignatureFromHex(s string) (*Signature, error) {
	s = strings.TrimSpace(s)

	hexStr := s
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		hexStr = s[2:]
	}

	buf, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}

	return NewSignatureFromBytes(buf)
}
func newSignatureFromBase58(b58Str string) (*Signature, error) {
	buf := base58.Decode(b58Str)
	if len(buf) == 0 {
		return nil, errors.New("invalid base58 string")
	}
	return NewSignatureFromBytes(buf)
}

type Signature struct {
	Variant SignatureType
	Bytes   []byte
}

func (sig *Signature) AsBytes() []byte {
	return sig.Bytes
}

func (sig *Signature) ToHex() string {
	return hex.EncodeToString(sig.Bytes)
}

func (sig *Signature) ToBase58() string {
	return base58.Encode(sig.Bytes)
}

// String implements the Stringer interface.
func (sig *Signature) String() string {
	return sig.ToBase58()
}

// ToSecp256k1 converts to a secp256k1 recoverable signature.
func (sig *Signature) ToSecp256k1() ([]byte, error) {
	if sig.Variant != SignatureTypeSecp256k1 {
		return nil, fmt.Errorf("not a secp256k1 signature, actual type: %d", sig.Variant)
	}

	if len(sig.Bytes) != SignatureSecp256k1Size {
		return nil, fmt.Errorf("invalid secp256k1 signature length: %d", len(sig.Bytes))
	}

	return sig.Bytes, nil
}

// ToEd25519 converts to an Ed25519 signature.
func (sig *Signature) ToEd25519() ([]byte, error) {
	if sig.Variant != SignatureTypeEd25519 {
		return nil, fmt.Errorf("not an ed25519 signature, actual type: %d", sig.Variant)
	}

	if len(sig.Bytes) != SignatureEd25519Size {
		return nil, fmt.Errorf("invalid ed25519 signature length: %d", len(sig.Bytes))
	}

	return sig.Bytes, nil
}

// ToBLS12381 converts to a BLS12-381 signature.
func (sig *Signature) ToBLS12381() ([]byte, error) {
	if sig.Variant != SignatureTypeBLS12381 {
		return nil, fmt.Errorf("not a BLS signature, actual type: %d", sig.Variant)
	}

	if len(sig.Bytes) != SignatureBLS12381Size {
		return nil, fmt.Errorf("invalid BLS signature length: %d", len(sig.Bytes))
	}

	return sig.Bytes, nil
}

// ToFnDsa512 converts to an FN-DSA-512 signature.
func (sig *Signature) ToFnDsa512() ([]byte, error) {
	if sig.Variant != SignatureTypeFnDsa512 {
		return nil, fmt.Errorf("not a FN-DSA-512 signature, actual type: %d", sig.Variant)
	}

	if len(sig.Bytes) != SignatureFnDsa512Size {
		return nil, fmt.Errorf("invalid FN-DSA-512 signature length: %d", len(sig.Bytes))
	}

	return sig.Bytes, nil
}

func (sig *Signature) Verify(msg []byte, pubKey *PublicKey) error {
	if uint8(sig.Variant) != uint8(pubKey.Variant) {
		return fmt.Errorf("signature type mismatch: %d vs %d", sig.Variant, pubKey.Variant)
	}

	switch sig.Variant {
	case SignatureTypeSecp256k1:
		return verifySecp256k1(msg, sig.Bytes, pubKey)
	case SignatureTypeEd25519:
		return verifyEd25519(msg, sig.Bytes, pubKey)
	case SignatureTypeBLS12381:
		return verifyBLS12381(msg, sig.Bytes, pubKey)
	case SignatureTypeFnDsa512:
		return verifyFnDsa512(msg, sig.Bytes, pubKey)
	default:
		return ErrInvalidSignature
	}
}

// verifySecp256k1 verifies a secp256k1 signature.
func verifySecp256k1(msg []byte, sigBytes []byte, pubKey *PublicKey) error {
	// 1. Compute message hash
	var msgHash [32]byte
	if len(msg) == 32 {
		copy(msgHash[:], msg)
	} else {
		msgHash = blake3.Sum256(msg)
	}

	// 2. Get public key
	pk, err := pubKey.ToSecp256k1()
	if err != nil {
		return err
	}

	// 3. Verify signature length
	if len(sigBytes) != SignatureSecp256k1Size {
		return fmt.Errorf("invalid secp256k1 signature length: expected %d, got %d", SignatureSecp256k1Size, len(sigBytes))
	}

	// 4. Validate recovery id (V) value range (27/28 for legacy, 0/1 for EIP-155)
	v := sigBytes[len(sigBytes)-1]
	if v != 27 && v != 28 && v != 0 && v != 1 {
		return fmt.Errorf("invalid signature recovery id (V): %d", v)
	}

	// 5. Convert secp256k1 public key to compressed format (33 bytes)
	compressedPubKey := pk.SerializeCompressed()

	// 6. Verify using go-ethereum's verification method
	signatureWithoutV := sigBytes[:len(sigBytes)-1] // take the first 64 bytes (R, S)
	if crypto.VerifySignature(compressedPubKey, msgHash[:], signatureWithoutV) == false {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// verifyEd25519 verifies an Ed25519 signature.
func verifyEd25519(msg []byte, sigBytes []byte, pubKey *PublicKey) error {
	pk, err := pubKey.ToEd25519()
	if err != nil {
		return err
	}

	if len(sigBytes) != SignatureEd25519Size {
		return fmt.Errorf("invalid ed25519 signature length: expected %d, got %d", SignatureEd25519Size, len(sigBytes))
	}

	if !ed25519.Verify(pk, msg, sigBytes) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// verifyBLS12381 验证 BLS12-381 签名
func verifyBLS12381(msg []byte, sigBytes []byte, pubKey *PublicKey) error {
	// 1. Get public key
	pk, err := pubKey.ToBLS12381()
	if err != nil {
		return err
	}

	// 2. Verify signature length
	if len(sigBytes) != SignatureBLS12381Size {
		return fmt.Errorf("invalid BLS signature length: expected %d, got %d", SignatureBLS12381Size, len(sigBytes))
	}

	// 3. Recover signature
	sig := new(blst.P2Affine).Uncompress(sigBytes)
	if sig == nil {
		return fmt.Errorf("failed to recover signature")
	}

	// 4. Verify signature - correct parameter order: (checkPub, pk, checkSig, msg, dst)
	if !sig.Verify(true, pk, true, msg, nil) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// verifyFnDsa512 verifies an FN-DSA-512 signature
func verifyFnDsa512(msg []byte, sigBytes []byte, key *PublicKey) error {
	if !fndsa.Verify(key.Bytes, []byte{}, 0, msg, sigBytes) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// VerifyBatch batch-verifies multiple signatures.
// Batch verification is used only when all signatures are Ed25519;
// otherwise each signature is verified individually.
func VerifyBatch(sigs []*Signature, msgs [][]byte, pubKeys []*PublicKey) error {
	if len(sigs) != len(msgs) || len(msgs) != len(pubKeys) {
		return fmt.Errorf("length mismatch: sigs=%d, msgs=%d, pubkeys=%d", len(sigs), len(msgs), len(pubKeys))
	}

	if len(sigs) == 0 {
		return nil
	}

	// Check whether all signatures are Ed25519 type (batch verification requires all Ed25519)
	allEd25519 := true
	for _, sig := range sigs {
		if sig.Variant != SignatureTypeEd25519 {
			allEd25519 = false
			break
		}
	}

	if allEd25519 {
		// Ed25519 supports batch verification
		verifier := ed25519consensus.NewBatchVerifier()

		for i := range sigs {
			edSig, err := sigs[i].ToEd25519()
			if err != nil {
				return fmt.Errorf("failed to convert signature at index %d: %w", i, err)
			}
			edPub, err := pubKeys[i].ToEd25519()
			if err != nil {
				return fmt.Errorf("failed to convert public key at index %d: %w", i, err)
			}

			verifier.Add(edPub, msgs[i], edSig)
		}

		if !verifier.Verify() {
			return fmt.Errorf("batch verification failed: one or more Ed25519 signatures are invalid")
		}

		return nil
	}

	// Mixed or non-Ed25519 types: verify individually
	for i := range sigs {
		if err := sigs[i].Verify(msgs[i], pubKeys[i]); err != nil {
			return err
		}
	}
	return nil
}

func (sig *Signature) MarshalJSON() ([]byte, error) {
	return json.Marshal(sig.ToBase58())
}

func (sig *Signature) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	newSig, err := NewSignatureFromStringRelaxed(s)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	*sig = *newSig
	return nil
}

func (sig *Signature) MarshalPostcard(serializer *postcard.Serializer) error {
	err := serializer.SerializeU32(uint32(sig.Variant))
	if err != nil {
		return fmt.Errorf("failed to serialize signature variant: %w", err)
	}

	serializer.SerializeFixedBytes(sig.Bytes)
	return nil
}

func (sig *Signature) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	variant, err := deserializer.DeserializeU32()
	if err != nil {
		return fmt.Errorf("failed to deserialize signature variant: %w", err)
	}

	var expectedLen int
	switch SignatureType(variant) {
	case SignatureTypeSecp256k1:
		expectedLen = SignatureSecp256k1Size
	case SignatureTypeEd25519:
		expectedLen = SignatureEd25519Size
	case SignatureTypeBLS12381:
		expectedLen = SignatureBLS12381Size
	case SignatureTypeFnDsa512:
		expectedLen = SignatureFnDsa512Size
	default:
		return fmt.Errorf("unknown signature variant: %d", variant)
	}

	buf, err := deserializer.DeserializeFixedBytes(expectedLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize signature Bytes: %w", err)
	}

	newSig, err := NewSignatureFromBytes(buf)
	if err != nil {
		return fmt.Errorf("failed to create signature from Bytes: %w", err)
	}

	sig.Variant = SignatureType(variant)
	sig.Bytes = newSig.Bytes
	return nil
}
