package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/pornin/go-fn-dsa/fndsa"
)

const (
	// LogN value, 9 means degree 2^9 = 512
	LogN = 9
)

const (
	FnDsa512SignKeyLen = 1281
	FnDsa512VrfyKeyLen = 897
	FnDsa512SigLen     = 666
)

type SecretKeyBytesFnDsa512 [FnDsa512SignKeyLen]byte
type PublicKeyBytesFnDsa512 [FnDsa512VrfyKeyLen]byte
type SignatureBytesFnDsa512 [FnDsa512SigLen]byte

var (
	ErrSignFailed   = errors.New("signing failed")
	ErrKeyGenFailed = errors.New("key generation failed")
)

// KeyGen512 generates an FN-DSA-512 key pair
func KeyGen512() (*SecretKeyBytesFnDsa512, *PublicKeyBytesFnDsa512, error) {
	signKey, verifyKey, err := fndsa.KeyGen(LogN, rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrKeyGenFailed, err)
	}

	var signKeyArr SecretKeyBytesFnDsa512
	var verifyKeyArr PublicKeyBytesFnDsa512

	if len(signKey) != FnDsa512SignKeyLen {
		return nil, nil, fmt.Errorf("%w: unexpected signing key length %d", ErrKeyGenFailed, len(signKey))
	}
	if len(verifyKey) != FnDsa512VrfyKeyLen {
		return nil, nil, fmt.Errorf("%w: unexpected verifying key length %d", ErrKeyGenFailed, len(verifyKey))
	}

	copy(signKeyArr[:], signKey)
	copy(verifyKeyArr[:], verifyKey)

	return &signKeyArr, &verifyKeyArr, nil
}

func NewSignKey512FromBytes(raw []byte) (*SecretKeyBytesFnDsa512, error) {
	if len(raw) != FnDsa512SignKeyLen {
		return nil, fmt.Errorf("%w: expected %d Bytes, got %d", ErrInvalidSecretKey, FnDsa512SignKeyLen, len(raw))
	}

	var key SecretKeyBytesFnDsa512
	copy(key[:], raw)
	return &key, nil
}

func NewVrfyKey512FromBytes(raw []byte) (*PublicKeyBytesFnDsa512, error) {
	if len(raw) != FnDsa512VrfyKeyLen {
		return nil, fmt.Errorf("%w: expected %d Bytes, got %d", ErrInvalidPublicKey, FnDsa512VrfyKeyLen, len(raw))
	}

	var key PublicKeyBytesFnDsa512
	copy(key[:], raw)
	return &key, nil
}

// Sign512 signs the message msg
func Sign512(signKey *SecretKeyBytesFnDsa512, msg []byte) (*SignatureBytesFnDsa512, error) {
	// Parameter notes:
	//   - rng: crypto/rand.Reader (consistent with secretkey.go)
	//   - ctx: domain separation context (empty string corresponds to DOMAIN_NONE)
	//   - id: pre-hash function identifier (0 for raw message, corresponds to HASH_ID_RAW)
	signature, err := fndsa.Sign(rand.Reader, signKey[:], []byte{}, 0, msg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSignFailed, err)
	}

	if len(signature) != FnDsa512SigLen {
		return nil, fmt.Errorf("%w: unexpected signature length %d", ErrSignFailed, len(signature))
	}

	var sigArr SignatureBytesFnDsa512
	copy(sigArr[:], signature)
	return &sigArr, nil
}

// Verify512 verifies a signature
func Verify512(vrfyKey *PublicKeyBytesFnDsa512, sig *SignatureBytesFnDsa512, msg []byte) error {
	// Parameter notes:
	//   - ctx: domain separation context (must match the one used in signing, empty string corresponds to DOMAIN_NONE)
	//   - id: pre-hash function identifier (must match the one used in signing, 0 for raw message, corresponds to HASH_ID_RAW)
	isValid := fndsa.Verify(vrfyKey[:], []byte{}, 0, msg, sig[:])
	if isValid {
		return nil
	}
	return ErrInvalidSignature
}
