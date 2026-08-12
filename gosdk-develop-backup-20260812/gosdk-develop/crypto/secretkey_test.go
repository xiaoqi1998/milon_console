package crypto

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassicalSecretKeyRoundTrip(t *testing.T) {
	sk1 := NewClassicalSecretKey()

	sk2 := &ClassicalSecretKey{}
	err := sk2.FromBytes(sk1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk2.AsBytes())

	sk3 := &ClassicalSecretKey{}
	err = sk3.FromStringRelaxed(sk1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sk3.AsBytes(), sk3.AsBytes())

	sk4 := &ClassicalSecretKey{}
	err = sk4.FromStringRelaxed("0x" + sk1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk4.AsBytes())

	sk5 := &ClassicalSecretKey{}
	err = sk5.FromStringRelaxed(sk1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk5.AsBytes())

	var parts []string
	for _, b := range sk1.AsBytes() {
		parts = append(parts, fmt.Sprintf("%d", b))
	}
	bracketStr := fmt.Sprintf("[%s]", strings.Join(parts, ","))
	sk6 := &ClassicalSecretKey{}
	err = sk6.FromStringRelaxed(bracketStr)
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk6.AsBytes())
}

func TestPureClassicalSecretKeyRoundTrip(t *testing.T) {
	sk1 := NewPureClassicalSecretKey()

	sk2 := &ClassicalSecretKey{}
	err := sk2.FromBytes(sk1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk2.AsBytes())

	sk3 := &ClassicalSecretKey{}
	err = sk3.FromStringRelaxed(sk1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk3.AsBytes())

	sk4 := &ClassicalSecretKey{}
	err = sk4.FromStringRelaxed("0x" + sk1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk4.AsBytes())

	sk5 := &ClassicalSecretKey{}
	err = sk5.FromStringRelaxed(sk1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk5.AsBytes())

	var parts []string
	for _, b := range sk1.AsBytes() {
		parts = append(parts, fmt.Sprintf("%d", b))
	}
	bracketStr := fmt.Sprintf("[%s]", strings.Join(parts, ","))
	sk6 := &ClassicalSecretKey{}
	err = sk6.FromStringRelaxed(bracketStr)
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk6.AsBytes())
}

func TestFnDsa512SecretKeyRoundTrip(t *testing.T) {
	sk, _, err := NewFnDsa512SecretKey()
	assert.NoError(t, err)
	sk1 := AsFnDsa512SecretKey(sk)
	assert.NotNil(t, sk1)

	sk2 := &FnDsa512SecretKey{}
	err = sk2.FromBytes(sk1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk2.AsBytes())

	sk3 := &FnDsa512SecretKey{}
	err = sk3.FromStringRelaxed(sk1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk3.AsBytes())

	sk4 := &FnDsa512SecretKey{}
	err = sk4.FromStringRelaxed("0x" + sk1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk4.AsBytes())

	sk5 := &FnDsa512SecretKey{}
	err = sk5.FromStringRelaxed(sk1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk5.AsBytes())

	var parts []string
	for _, b := range sk1.AsBytes() {
		parts = append(parts, fmt.Sprintf("%d", b))
	}
	bracketStr := fmt.Sprintf("[%s]", strings.Join(parts, ","))
	sk6 := &FnDsa512SecretKey{}
	err = sk6.FromStringRelaxed(bracketStr)
	assert.NoError(t, err)
	assert.Equal(t, sk1.AsBytes(), sk6.AsBytes())
}

func TestSecretKeyerSecp256k1SignVerify(t *testing.T) {
	sk := AsClassicalSecretKey(NewClassicalSecretKey())
	assert.NotNil(t, sk)

	pk, err := sk.Secp256k1Public()
	assert.NoError(t, err)
	msg := []byte("hello secp")

	sig, err := sk.SignSecp256k1(msg)
	assert.NoError(t, err)

	err = sig.Verify(msg, pk)
	assert.NoError(t, err)

	err = sig.Verify([]byte("other"), pk)
	assert.Error(t, err)

	pkOther, err := AsClassicalSecretKey(NewPureClassicalSecretKey()).Secp256k1Public()
	err = sig.Verify(msg, pkOther)
	assert.Error(t, err)
}

func TestSecretKeyerEd25519SignVerify(t *testing.T) {
	sk := AsClassicalSecretKey(NewPureClassicalSecretKey())
	assert.NotNil(t, sk)

	pk := sk.Ed25519Public()
	msg := []byte("hello ed25519")
	sig := sk.SignEd25519(msg)

	err := sig.Verify(msg, pk)
	assert.NoError(t, err)

	err = sig.Verify([]byte("other"), pk)
	assert.Error(t, err)

	err = sig.Verify(msg, AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public())
	assert.Error(t, err)
}

func TestSecretKeyerBLS12381SignVerify(t *testing.T) {
	sk := AsClassicalSecretKey(NewClassicalSecretKey())
	assert.NotNil(t, sk)

	pk := sk.BLS12381Public()
	msg := []byte("hello bls")
	sig := sk.SignBLS12381(msg)

	err := sig.Verify(msg, pk)
	assert.NoError(t, err)

	err = sig.Verify([]byte("other"), pk)
	assert.Error(t, err)

	err = sig.Verify(msg, AsClassicalSecretKey(NewClassicalSecretKey()).BLS12381Public())
	assert.Error(t, err)
}

func TestSecretKeyerFnDsa512Verify(t *testing.T) {
	sker, pk, err := NewFnDsa512SecretKey()
	assert.NoError(t, err)
	assert.NotNil(t, sker)
	assert.NotNil(t, pk)

	sk := AsFnDsa512SecretKey(sker)
	assert.NotNil(t, sk)

	msg := []byte("fn-dsa-512 test message")
	sig, err := sk.SignFnDsa512(msg)
	assert.NoError(t, err)

	err = sig.Verify(msg, pk)
	assert.NoError(t, err)

	err = sig.Verify([]byte("other"), pk)
	assert.Error(t, err)

	_, pkOther, _ := NewFnDsa512SecretKey()
	err = sig.Verify(msg, pkOther)
	assert.Error(t, err)
}

func TestSecretKeyerInvalidSecretTooShort(t *testing.T) {
	shortBytes := make([]byte, ClassicalKeySize-1)
	sk1 := &ClassicalSecretKey{}
	err := sk1.FromBytes(shortBytes)
	assert.Error(t, err)

	shortBytes = make([]byte, FnDsa512KeySize-1)
	sk2 := &FnDsa512SecretKey{}
	err = sk2.FromBytes(shortBytes)
	assert.Error(t, err)
}

func TestSecretKeyerFromStringRelaxedInvalidFormat(t *testing.T) {
	_, err := SecretKeyerFromStringRelaxed("not a valid hex string")
	assert.Error(t, err)

	_, err = SecretKeyerFromStringRelaxed("!!!invalid base58!!!")
	assert.Error(t, err)

	_, err = SecretKeyerFromStringRelaxed("[1,2,3]")
	assert.Error(t, err)
}

func TestSecretKeyerNativeSecp256k1(t *testing.T) {
	sk := AsClassicalSecretKey(NewClassicalSecretKey())
	assert.NotNil(t, sk)

	native, err := sk.ToSecp256k1()
	assert.NoError(t, err)

	decoded := &ClassicalSecretKey{}
	err = decoded.FromSecp256k1Native(native)
	assert.NoError(t, err)
	assert.Equal(t, sk.AsBytes(), decoded.AsBytes())
}

func TestSecretKeyerNativeEd25519(t *testing.T) {
	sk := AsClassicalSecretKey(NewPureClassicalSecretKey())
	assert.NotNil(t, sk)

	native := sk.ToEd25519()

	decoded := &ClassicalSecretKey{}
	err := decoded.FromEd25519Native(native)
	assert.NoError(t, err)
	assert.Equal(t, sk.AsBytes(), decoded.AsBytes())
}

func TestSecretKeyerClassicalAndFnDsa512Mixed(t *testing.T) {
	classical := NewClassicalSecretKey()
	fnDsa, _, err := NewFnDsa512SecretKey()
	assert.NoError(t, err)

	assert.Equal(t, SecretKeyTypeClassical, classical.Type())
	assert.Equal(t, SecretKeyTypeFnDsa512, fnDsa.Type())

	assert.NotNil(t, AsClassicalSecretKey(classical))
	assert.Nil(t, AsFnDsa512SecretKey(classical))

	assert.Nil(t, AsClassicalSecretKey(fnDsa))
	assert.NotNil(t, AsFnDsa512SecretKey(fnDsa))
}

// ============================================
// Benchmark
// ============================================

func BenchmarkSecretKeyerNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewClassicalSecretKey()
	}
}

func BenchmarkSecretKeyerNewPure(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewPureClassicalSecretKey()
	}
}

func BenchmarkSecretKeyerSecp256k1SignVerify(b *testing.B) {
	sk := NewClassicalSecretKey()
	ck := AsClassicalSecretKey(sk)
	pk, _ := ck.Secp256k1Public()
	msg := []byte("benchmark message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig, err := ck.SignSecp256k1(msg)
		assert.NoError(b, err)
		err = sig.Verify(msg, pk)
		assert.NoError(b, err)
	}
}

func BenchmarkSecretKeyerEd25519SignVerify(b *testing.B) {
	sk := NewPureClassicalSecretKey()
	ck := AsClassicalSecretKey(sk)
	pk := ck.Ed25519Public()
	msg := []byte("benchmark message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig := ck.SignEd25519(msg)
		err := sig.Verify(msg, pk)
		assert.NoError(b, err)
	}
}

func BenchmarkSecretKeyerBLS12381SignVerify(b *testing.B) {
	sk := NewClassicalSecretKey()
	ck := AsClassicalSecretKey(sk)
	pk := ck.BLS12381Public()
	msg := []byte("benchmark message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig := ck.SignBLS12381(msg)
		err := sig.Verify(msg, pk)
		assert.NoError(b, err)
	}
}

func BenchmarkSecretKeyerFnDsa512SignVerify(b *testing.B) {
	sk, pk, err := NewFnDsa512SecretKey()
	if err != nil {
		b.Fatal(err)
	}
	fk := AsFnDsa512SecretKey(sk)
	msg := []byte("benchmark message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig, err := fk.SignFnDsa512(msg)
		assert.NoError(b, err)
		err = sig.Verify(msg, pk)
		assert.NoError(b, err)
	}
}
