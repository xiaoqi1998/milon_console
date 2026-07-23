package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pornin/go-fn-dsa/fndsa"
	blst "github.com/supranational/blst/bindings/go"
	"golang.org/x/crypto/ed25519"
	"lukechampine.com/blake3"
)

const (
	// ClassicalKeySize 32-byte seed length shared by classical curves
	ClassicalKeySize = 32

	// FnDsa512KeySize FN-DSA-512 signing key length (1281 bytes)
	FnDsa512KeySize = 1281
)

// SecretKeyer unified secret key interface
type SecretKeyer interface {
	Type() SecretKeyType
	AsBytes() []byte
	ToHex() string
	ToBase58() string
	String() string
	Zeroize()
	SignFor(publicKey PublicKey, msg []byte) (*Signature, error)
}

type SecretKeyType uint8

const (
	SecretKeyTypeClassical SecretKeyType = iota
	SecretKeyTypeFnDsa512
)

// ============================================
// Classical key implementation (32-byte seed)
// ============================================

type ClassicalSecretKey struct {
	Bytes [ClassicalKeySize]byte
}

// NewClassicalSecretKey generates a random classical secret key (ensures valid secp256k1 scalar)
func NewClassicalSecretKey() SecretKeyer {
	for {
		var ret [ClassicalKeySize]byte
		if _, err := rand.Read(ret[:]); err != nil {
			// 系统级 RNG 故障属致命错误，无法恢复
			panic(fmt.Sprintf("crypto/rand failed: %v", err))
		}

		// 检查是否为有效的 secp256k1 私钥
		if _, err := crypto.ToECDSA(ret[:]); err == nil {
			sk := &ClassicalSecretKey{Bytes: ret}
			return sk
		}
	}
}

// NewPureClassicalSecretKey creates a random 32-byte classical seed without secp256k1 scalar validation
func NewPureClassicalSecretKey() SecretKeyer {
	var ret [ClassicalKeySize]byte
	if _, err := rand.Read(ret[:]); err != nil {
		// 系统级 RNG 故障属致命错误，无法恢复
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	sk := &ClassicalSecretKey{Bytes: ret}
	return sk
}

func (sk *ClassicalSecretKey) FromBytes(raw []byte) error {
	if len(raw) < ClassicalKeySize {
		return ErrInvalidSecretKey
	}

	var ret [ClassicalKeySize]byte
	copy(ret[:], raw[:ClassicalKeySize])

	sk.Zeroize()
	sk.Bytes = ret
	return nil
}

// FromStringRelaxed parses from hex, Base58, or array format "[1,2,3,...]"
func (sk *ClassicalSecretKey) FromStringRelaxed(s string) error {
	s = strings.TrimSpace(s)

	// Try array format [1,2,3,...]
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		trimmed := strings.Trim(s, "[]")
		parts := strings.Split(trimmed, ",")
		if len(parts) != ClassicalKeySize {
			return ErrInvalidSecretKey
		}

		var bytes [ClassicalKeySize]byte
		for i, part := range parts {
			val, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8)
			if err != nil {
				return ErrInvalidSecretKey
			}
			bytes[i] = byte(val)
		}

		return sk.FromBytes(bytes[:])
	}

	// Try hex parsing
	if err := sk.fromHex(s); err == nil {
		return nil
	}

	// Try Base58
	return sk.fromBase58(s)
}
func (sk *ClassicalSecretKey) fromHex(s string) error {
	s = strings.TrimSpace(s)

	hexStr := s
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		hexStr = s[2:]
	}

	buf, err := hex.DecodeString(hexStr)
	if err != nil {
		return ErrInvalidSecretKey
	}

	return sk.FromBytes(buf)
}

func (sk *ClassicalSecretKey) fromBase58(b58Str string) error {
	bytes := base58.Decode(b58Str)
	if len(bytes) != ClassicalKeySize {
		return ErrInvalidSecretKey
	}
	return sk.FromBytes(bytes)
}

// SecretKeyer interface implementation

func (sk *ClassicalSecretKey) Type() SecretKeyType {
	return SecretKeyTypeClassical
}

func (sk *ClassicalSecretKey) AsBytes() []byte {
	return sk.Bytes[:]
}

func (sk *ClassicalSecretKey) ToHex() string {
	return hex.EncodeToString(sk.Bytes[:])
}

func (sk *ClassicalSecretKey) ToBase58() string {
	return base58.Encode(sk.Bytes[:])
}

func (sk *ClassicalSecretKey) String() string {
	return sk.ToBase58()
}

func (sk *ClassicalSecretKey) Zeroize() {
	for i := range sk.Bytes {
		sk.Bytes[i] = 0
	}
}

func (sk *ClassicalSecretKey) SignFor(publicKey PublicKey, msg []byte) (*Signature, error) {
	switch publicKey.Variant {
	case PublicKeyTypeSecp256k1:
		return sk.SignSecp256k1(msg)
	case PublicKeyTypeEd25519:
		return sk.SignEd25519(msg), nil
	case PublicKeyTypeBLS12381:
		return sk.SignBLS12381(msg), nil
	default:
		return nil, fmt.Errorf("unsupported public key type for classical secret key: %d", publicKey.Variant)
	}
}

// Classical key specific methods
func (sk *ClassicalSecretKey) ToSecp256k1() (*ecdsa.PrivateKey, error) {
	return crypto.ToECDSA(sk.Bytes[:])
}

func (sk *ClassicalSecretKey) ToEd25519() ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(sk.Bytes[:])
}

func (sk *ClassicalSecretKey) ToBLS12381() *blst.SecretKey {
	return blst.KeyGen(sk.Bytes[:])
}

func (sk *ClassicalSecretKey) Secp256k1Public() (*PublicKey, error) {
	priv, err := sk.ToSecp256k1()
	if err != nil {
		return nil, err
	}
	return &PublicKey{
		Variant: PublicKeyTypeSecp256k1,
		Bytes:   crypto.CompressPubkey(&priv.PublicKey),
	}, nil
}

func (sk *ClassicalSecretKey) Ed25519Public() *PublicKey {
	return &PublicKey{
		Variant: PublicKeyTypeEd25519,
		Bytes:   sk.ToEd25519().Public().(ed25519.PublicKey),
	}
}

func (sk *ClassicalSecretKey) BLS12381Public() *PublicKey {
	return &PublicKey{
		Variant: PublicKeyTypeBLS12381,
		Bytes:   new(blst.P1Affine).From(sk.ToBLS12381()).Compress(),
	}
}

func (sk *ClassicalSecretKey) SignSecp256k1(msg []byte) (*Signature, error) {
	var msgHash [32]byte
	if len(msg) == ClassicalKeySize {
		copy(msgHash[:], msg)
	} else {
		msgHash = blake3.Sum256(msg)
	}

	priv, err := sk.ToSecp256k1()
	if err != nil {
		return nil, err
	}

	signature, err := crypto.Sign(msgHash[:], priv)
	if err != nil {
		return nil, err
	}

	if signature[64] == 0 || signature[64] == 1 {
		signature[64] += 27
	}

	return &Signature{
		Variant: SignatureTypeSecp256k1,
		Bytes:   signature,
	}, nil
}

func (sk *ClassicalSecretKey) SignEd25519(msg []byte) *Signature {
	return &Signature{
		Variant: SignatureTypeEd25519,
		Bytes:   ed25519.Sign(sk.ToEd25519(), msg),
	}
}

func (sk *ClassicalSecretKey) SignBLS12381(msg []byte) *Signature {
	return &Signature{
		Variant: SignatureTypeBLS12381,
		Bytes:   new(blst.P2Affine).Sign(sk.ToBLS12381(), msg, nil).Compress(),
	}
}

// FromEd25519Native converts from an ed25519 native private key back to ClassicalSecretKey
func (sk *ClassicalSecretKey) FromEd25519Native(priv ed25519.PrivateKey) error {
	if priv == nil {
		return errors.New("ed25519 private key is nil")
	}

	// ed25519 private key is 64 bytes, the first 32 bytes are the seed
	if len(priv) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid ed25519 private key length: expected %d, got %d", ed25519.PrivateKeySize, len(priv))
	}

	// Extract seed (first 32 bytes)
	seed := priv.Seed()

	// Zeroize the current secret key (security measure)
	sk.Zeroize()

	// Copy seed into ClassicalSecretKey
	copy(sk.Bytes[:], seed)

	return nil
}

// FromSecp256k1Native converts from a secp256k1 native private key back to ClassicalSecretKey
func (sk *ClassicalSecretKey) FromSecp256k1Native(priv *ecdsa.PrivateKey) error {
	if priv == nil {
		return errors.New("private key is nil")
	}

	privBytes := priv.D.Bytes()
	if len(privBytes) > ClassicalKeySize {
		return fmt.Errorf("private key too long: expected %d, got %d", ClassicalKeySize, len(privBytes))
	}

	// Zeroize the current secret key (security measure)
	sk.Zeroize()

	// Copy bytes (right-aligned)
	offset := ClassicalKeySize - len(privBytes)
	for i, b := range privBytes {
		sk.Bytes[offset+i] = b
	}

	return nil
}

// ============================================
// FN-DSA-512 key implementation (1281 bytes)
// ============================================

type FnDsa512SecretKey struct {
	bytes [FnDsa512KeySize]byte
}

func NewFnDsa512SecretKey() (SecretKeyer, *PublicKey, error) {
	signKey, verifyKey, err := fndsa.KeyGen(9, rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	if len(signKey) != FnDsa512KeySize {
		return nil, nil, fmt.Errorf("unexpected signing key size: %d", len(signKey))
	}

	var bytes [FnDsa512KeySize]byte
	copy(bytes[:], signKey)

	pubKey := &PublicKey{
		Variant: PublicKeyTypeFnDsa512,
		Bytes:   verifyKey,
	}

	return &FnDsa512SecretKey{bytes: bytes}, pubKey, nil
}

func (sk *FnDsa512SecretKey) FromBytes(raw []byte) error {
	if len(raw) != FnDsa512KeySize {
		return ErrInvalidSecretKey
	}

	sk.Zeroize()
	copy(sk.bytes[:], raw)

	return nil
}

// FromStringRelaxed parses from hex, Base58, or array format "[1,2,3,...]"
func (sk *FnDsa512SecretKey) FromStringRelaxed(s string) error {
	s = strings.TrimSpace(s)

	// Try array format [1,2,3,...]
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		trimmed := strings.Trim(s, "[]")
		parts := strings.Split(trimmed, ",")
		if len(parts) != FnDsa512KeySize {
			return ErrInvalidSecretKey
		}

		var bytes [FnDsa512KeySize]byte
		for i, part := range parts {
			val, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8)
			if err != nil {
				return ErrInvalidSecretKey
			}
			bytes[i] = byte(val)
		}

		return sk.FromBytes(bytes[:])
	}

	// Try hex parsing
	if err := sk.fromHex(s); err == nil {
		return nil
	}

	// Try Base58
	return sk.fromBase58(s)
}
func (sk *FnDsa512SecretKey) fromHex(s string) error {
	s = strings.TrimSpace(s)

	hexStr := s
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		hexStr = s[2:]
	}

	buf, err := hex.DecodeString(hexStr)
	if err != nil {
		return ErrInvalidSecretKey
	}

	return sk.FromBytes(buf)
}
func (sk *FnDsa512SecretKey) fromBase58(b58Str string) error {
	bytes := base58.Decode(b58Str)
	if len(bytes) != FnDsa512KeySize {
		return ErrInvalidSecretKey
	}
	return sk.FromBytes(bytes)
}

// SecretKeyer interface implementation
func (sk *FnDsa512SecretKey) Type() SecretKeyType {
	return SecretKeyTypeFnDsa512
}

func (sk *FnDsa512SecretKey) AsBytes() []byte {
	// 返回副本，避免外部修改污染私钥底层数组，确保 Zeroize 能完全清零。
	return append([]byte(nil), sk.bytes[:]...)
}

func (sk *FnDsa512SecretKey) ToHex() string {
	return hex.EncodeToString(sk.bytes[:])
}

func (sk *FnDsa512SecretKey) ToBase58() string {
	return base58.Encode(sk.bytes[:])
}

func (sk *FnDsa512SecretKey) String() string {
	return sk.ToBase58()
}

func (sk *FnDsa512SecretKey) Zeroize() {
	for i := range sk.bytes {
		sk.bytes[i] = 0
	}
}

func (sk *FnDsa512SecretKey) SignFor(publicKey PublicKey, msg []byte) (*Signature, error) {
	if publicKey.Variant != PublicKeyTypeFnDsa512 {
		return nil, fmt.Errorf("fndsa512 secret key can only sign for fndsa512 public key, got: %d", publicKey.Variant)
	}
	return sk.SignFnDsa512(msg)
}

// FN-DSA-512 specific methods
func (sk *FnDsa512SecretKey) SignFnDsa512(msg []byte) (*Signature, error) {
	signature, err := fndsa.Sign(rand.Reader, sk.bytes[:], []byte{}, 0, msg)
	if err != nil {
		return nil, err
	}

	return &Signature{
		Variant: SignatureTypeFnDsa512,
		Bytes:   signature,
	}, nil
}

// TODO: Go version cannot derive the verification key from the signing key; a pre-generated public key is required
func (sk *FnDsa512SecretKey) FnDsa512Public() (*PublicKey, error) {
	return nil, fmt.Errorf("not implemented")
}

// ============================================
// Helper functions: key parsing and conversion
// ============================================

// SecretKeyerFromBytes parses by raw byte length: 32 → Classical, 1281 → FnDsa512
func SecretKeyerFromBytes(raw []byte) (SecretKeyer, error) {
	switch len(raw) {
	case ClassicalKeySize:
		sk := &ClassicalSecretKey{}
		err := sk.FromBytes(raw)
		if err != nil {
			return nil, err
		}
		return sk, err
	case FnDsa512KeySize:
		sk := &FnDsa512SecretKey{}
		err := sk.FromBytes(raw)
		if err != nil {
			return nil, err
		}
		return sk, err
	default:
		return nil, fmt.Errorf("%w: unsupported length %d", ErrInvalidSecretKey, len(raw))
	}
}

// SecretKeyerFromStringRelaxed 从多种格式解析：十六进制、Base58 或数组格式 "[1,2,3,...]"
func SecretKeyerFromStringRelaxed(s string) (SecretKeyer, error) {
	s = strings.TrimSpace(s)

	// Try array format [1,2,3,...]
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		trimmed := strings.Trim(s, "[]")
		parts := strings.Split(trimmed, ",")

		// Determine length: 32 or 1281
		if len(parts) == ClassicalKeySize {
			var bytes [ClassicalKeySize]byte
			for i, part := range parts {
				val, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8)
				if err != nil {
					return nil, ErrInvalidSecretKey
				}
				bytes[i] = byte(val)
			}

			sk := &ClassicalSecretKey{}
			err := sk.FromBytes(bytes[:])
			if err != nil {
				return nil, err
			}
			return sk, err
		}

		if len(parts) == FnDsa512KeySize {
			var bytes [FnDsa512KeySize]byte
			for i, part := range parts {
				val, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8)
				if err != nil {
					return nil, ErrInvalidSecretKey
				}
				bytes[i] = byte(val)
			}

			sk := &FnDsa512SecretKey{}
			err := sk.FromBytes(bytes[:])
			if err != nil {
				return nil, err
			}
			return sk, err
		}

		return nil, ErrInvalidSecretKey
	}

	// Try hex parsing
	secretKeyer, err := newSecretKeyerFromHex(s)
	if err == nil {
		return secretKeyer, err
	}

	// Try Base58
	return newSecretKeyerFromBase58(s)
}
func newSecretKeyerFromHex(s string) (SecretKeyer, error) {
	s = strings.TrimSpace(s)

	hexStr := s
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		hexStr = s[2:]
	}

	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}

	return SecretKeyerFromBytes(raw)
}

func newSecretKeyerFromBase58(b58Str string) (SecretKeyer, error) {
	return SecretKeyerFromBytes(base58.Decode(b58Str))
}

func AsClassicalSecretKey(sk SecretKeyer) *ClassicalSecretKey {
	if ck, ok := sk.(*ClassicalSecretKey); ok {
		return ck
	}
	return nil
}

func AsFnDsa512SecretKey(sk SecretKeyer) *FnDsa512SecretKey {
	if fk, ok := sk.(*FnDsa512SecretKey); ok {
		return fk
	}
	return nil
}
