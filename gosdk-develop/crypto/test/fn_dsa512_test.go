package test

import (
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestKeygen512(t *testing.T) {
	signKey, vrfyKey, err := crypto.KeyGen512()
	assert.NoError(t, err)

	assert.Equal(t, crypto.FnDsa512SignKeyLen, len(signKey))
	assert.Equal(t, crypto.FnDsa512VrfyKeyLen, len(vrfyKey))
}
func TestNewSignKey512FromBytes(t *testing.T) {
	signKey, _, _ := crypto.KeyGen512()

	decoded, err := crypto.NewSignKey512FromBytes(signKey[:])
	assert.NoError(t, err)
	assert.Equal(t, signKey, decoded)
}

func TestNewVrfyKey512FromBytes(t *testing.T) {
	_, vrfyKey, _ := crypto.KeyGen512()

	decoded, err := crypto.NewVrfyKey512FromBytes(vrfyKey[:])
	assert.NoError(t, err)
	assert.Equal(t, vrfyKey, decoded)
}

func TestSignAndVerify512(t *testing.T) {
	signKey, vrfyKey, err := crypto.KeyGen512()
	assert.NoError(t, err)

	msg := []byte("Hello, FN-DSA-512!")

	sig, err := crypto.Sign512(signKey, msg)
	assert.NoError(t, err)
	assert.Equal(t, len(sig), crypto.FnDsa512SigLen)
	assert.NoError(t, crypto.Verify512(vrfyKey, sig, msg))

	assert.Error(t, crypto.Verify512(vrfyKey, sig, []byte("Wrong message")))
}

func TestNewSignKey512FromBytesWrongLen(t *testing.T) {
	_, err := crypto.NewSignKey512FromBytes(make([]byte, crypto.FnDsa512SignKeyLen-1))
	assert.Error(t, err)
}

func TestNewVrfyKey512FromBytesWrongLen(t *testing.T) {
	_, err := crypto.NewVrfyKey512FromBytes(make([]byte, crypto.FnDsa512VrfyKeyLen-1))
	assert.Error(t, err)
}

// ============================================
// Benchmark
// ============================================

func BenchmarkKeygen512(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = crypto.KeyGen512()
	}
}

func BenchmarkSign512(b *testing.B) {
	signKey, _, _ := crypto.KeyGen512()
	msg := []byte("Benchmark message for FN-DSA-512 signing")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := crypto.Sign512(signKey, msg)
		assert.NoError(b, err)
	}
}

func BenchmarkVerify512(b *testing.B) {
	signKey, vrfyKey, _ := crypto.KeyGen512()
	msg := []byte("Benchmark message for FN-DSA-512 verification")
	sig, err := crypto.Sign512(signKey, msg)
	assert.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = crypto.Verify512(vrfyKey, sig, msg)
		assert.NoError(b, err)
	}
}
