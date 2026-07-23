package crypto

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestKeygen512(t *testing.T) {
	signKey, vrfyKey, err := KeyGen512()
	assert.NoError(t, err)

	assert.Equal(t, FnDsa512SignKeyLen, len(signKey))
	assert.Equal(t, FnDsa512VrfyKeyLen, len(vrfyKey))
}
func TestNewSignKey512FromBytes(t *testing.T) {
	signKey, _, _ := KeyGen512()

	decoded, err := NewSignKey512FromBytes(signKey[:])
	assert.NoError(t, err)
	assert.Equal(t, signKey, decoded)
}

func TestNewVrfyKey512FromBytes(t *testing.T) {
	_, vrfyKey, _ := KeyGen512()

	decoded, err := NewVrfyKey512FromBytes(vrfyKey[:])
	assert.NoError(t, err)
	assert.Equal(t, vrfyKey, decoded)
}

func TestSignAndVerify512(t *testing.T) {
	signKey, vrfyKey, err := KeyGen512()
	assert.NoError(t, err)

	msg := []byte("Hello, FN-DSA-512!")

	sig, err := Sign512(signKey, msg)
	assert.NoError(t, err)
	assert.Equal(t, len(sig), FnDsa512SigLen)
	assert.NoError(t, Verify512(vrfyKey, sig, msg))

	assert.Error(t, Verify512(vrfyKey, sig, []byte("Wrong message")))
}

// ============================================
// Benchmark
// ============================================

func BenchmarkKeygen512(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = KeyGen512()
	}
}

func BenchmarkSign512(b *testing.B) {
	signKey, _, _ := KeyGen512()
	msg := []byte("Benchmark message for FN-DSA-512 signing")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Sign512(signKey, msg)
		assert.NoError(b, err)
	}
}

func BenchmarkVerify512(b *testing.B) {
	signKey, vrfyKey, _ := KeyGen512()
	msg := []byte("Benchmark message for FN-DSA-512 verification")
	sig, err := Sign512(signKey, msg)
	assert.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = Verify512(vrfyKey, sig, msg)
		assert.NoError(b, err)
	}
}
