package test

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretKeyerClassicalAndFnDsa512Mixed(t *testing.T) {
	classical := crypto.NewClassicalSecretKey()
	fnDsa, _, err := crypto.NewFnDsa512SecretKey()
	assert.NoError(t, err)

	assert.Equal(t, crypto.SecretKeyTypeClassical, classical.Type())
	assert.Equal(t, crypto.SecretKeyTypeFnDsa512, fnDsa.Type())

	assert.NotNil(t, crypto.AsClassicalSecretKey(classical))
	assert.Nil(t, crypto.AsFnDsa512SecretKey(classical))

	assert.Nil(t, crypto.AsClassicalSecretKey(fnDsa))
	assert.NotNil(t, crypto.AsFnDsa512SecretKey(fnDsa))
}

func TestSecretKeyRoundTrip(t *testing.T) {
	testCases := []struct {
		name      string
		createKey func() (crypto.SecretKeyer, error)
	}{
		{
			name: "Classical",
			createKey: func() (crypto.SecretKeyer, error) {
				return crypto.NewClassicalSecretKey(), nil
			},
		},
		{
			name: "PureClassical",
			createKey: func() (crypto.SecretKeyer, error) {
				return crypto.NewPureClassicalSecretKey(), nil
			},
		},
		{
			name: "FnDsa512",
			createKey: func() (crypto.SecretKeyer, error) {
				sk, _, err := crypto.NewFnDsa512SecretKey()
				return sk, err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sk1, err := tc.createKey()
			assert.NoError(t, err)

			sk2, err := crypto.SecretKeyerFromBytes(sk1.AsBytes())
			assert.NoError(t, err)
			assert.Equal(t, sk1.AsBytes(), sk2.AsBytes())

			sk3, err := crypto.SecretKeyerFromStringRelaxed(sk1.ToHex())
			assert.NoError(t, err)
			assert.Equal(t, sk1.AsBytes(), sk3.AsBytes())

			sk4, err := crypto.SecretKeyerFromStringRelaxed("0x" + sk1.ToHex())
			assert.NoError(t, err)
			assert.Equal(t, sk1.AsBytes(), sk4.AsBytes())

			sk5, err := crypto.SecretKeyerFromStringRelaxed(sk1.ToBase58())
			assert.NoError(t, err)
			assert.Equal(t, sk1.AsBytes(), sk5.AsBytes())

			var parts []string
			for _, b := range sk1.AsBytes() {
				parts = append(parts, fmt.Sprintf("%d", b))
			}
			bracketStr := fmt.Sprintf("[%s]", strings.Join(parts, ","))
			sk6, err := crypto.SecretKeyerFromStringRelaxed(bracketStr)
			assert.NoError(t, err)
			assert.Equal(t, sk1.AsBytes(), sk6.AsBytes())
		})
	}
}

func TestSecretKeyerSignVerify(t *testing.T) {
	msg := []byte("hello crypto")

	testCases := []struct {
		name       string
		createPair func() (*crypto.Signature, *crypto.PublicKey, error)
		otherPk    func() *crypto.PublicKey
	}{
		{
			name: "Secp256k1",
			createPair: func() (*crypto.Signature, *crypto.PublicKey, error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
				pk, err := sk.Secp256k1Public()
				if err != nil {
					return nil, nil, err
				}
				sig, err := sk.SignSecp256k1(msg)
				return sig, pk, err
			},
			otherPk: func() *crypto.PublicKey {
				pk, err := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Secp256k1Public()
				assert.NoError(t, err)
				return pk
			},
		},
		{
			name: "Ed25519",
			createPair: func() (*crypto.Signature, *crypto.PublicKey, error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
				return sk.SignEd25519(msg), sk.Ed25519Public(), nil
			},
			otherPk: func() *crypto.PublicKey {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Ed25519Public()
			},
		},
		{
			name: "BLS12381",
			createPair: func() (*crypto.Signature, *crypto.PublicKey, error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
				return sk.SignBLS12381(msg), sk.BLS12381Public(), nil
			},
			otherPk: func() *crypto.PublicKey {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).BLS12381Public()
			},
		},
		{
			name: "FnDsa512",
			createPair: func() (*crypto.Signature, *crypto.PublicKey, error) {
				sker, pk, err := crypto.NewFnDsa512SecretKey()
				if err != nil {
					return nil, nil, err
				}
				sig, err := crypto.AsFnDsa512SecretKey(sker).SignFnDsa512(msg)
				return sig, pk, err
			},
			otherPk: func() *crypto.PublicKey {
				_, pk, err := crypto.NewFnDsa512SecretKey()
				assert.NoError(t, err)
				return pk
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sig, pk, err := tc.createPair()
			assert.NoError(t, err)

			assert.NoError(t, sig.Verify(msg, pk))
			assert.Error(t, sig.Verify([]byte("other"), pk))
			assert.Error(t, sig.Verify(msg, tc.otherPk()))
		})
	}
}

func TestSecretKeyFromBytesTooShort(t *testing.T) {
	shortBytes := make([]byte, crypto.ClassicalKeySize-1)
	sk1 := &crypto.ClassicalSecretKey{}
	err := sk1.FromBytes(shortBytes)
	assert.Error(t, err)

	shortBytes = make([]byte, crypto.FnDsa512KeySize-1)
	sk2 := &crypto.FnDsa512SecretKey{}
	err = sk2.FromBytes(shortBytes)
	assert.Error(t, err)
}

func TestSecretKeyerFromStringRelaxedInvalidFormat(t *testing.T) {
	_, err := crypto.SecretKeyerFromStringRelaxed("not a valid hex string")
	assert.Error(t, err)

	_, err = crypto.SecretKeyerFromStringRelaxed("!!!invalid base58!!!")
	assert.Error(t, err)

	_, err = crypto.SecretKeyerFromStringRelaxed("[1,2,3]")
	assert.Error(t, err)
}

func TestClassicalSecretKeyNativeSecp256k1(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	assert.NotNil(t, sk)

	native, err := sk.ToSecp256k1()
	assert.NoError(t, err)

	decoded := &crypto.ClassicalSecretKey{}
	err = decoded.FromSecp256k1Native(native)
	assert.NoError(t, err)
	assert.Equal(t, sk.AsBytes(), decoded.AsBytes())
}

func TestClassicalSecretKeyNativeEd25519(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	assert.NotNil(t, sk)

	native := sk.ToEd25519()

	decoded := &crypto.ClassicalSecretKey{}
	err := decoded.FromEd25519Native(native)
	assert.NoError(t, err)
	assert.Equal(t, sk.AsBytes(), decoded.AsBytes())
}

func TestSecretKeyZeroize(t *testing.T) {
	testCases := []struct {
		name      string
		createKey func() (crypto.SecretKeyer, error)
		size      int
	}{
		{
			name:      "Classical",
			createKey: func() (crypto.SecretKeyer, error) { return crypto.NewClassicalSecretKey(), nil },
			size:      crypto.ClassicalKeySize,
		},
		{
			name: "FnDsa512",
			createKey: func() (crypto.SecretKeyer, error) {
				sk, _, err := crypto.NewFnDsa512SecretKey()
				return sk, err
			},
			size: crypto.FnDsa512KeySize,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sk, err := tc.createKey()
			assert.NoError(t, err)
			assert.NotEqual(t, make([]byte, tc.size), sk.AsBytes())

			sk.Zeroize()
			assert.Equal(t, make([]byte, tc.size), sk.AsBytes())
		})
	}
}

func TestSecretKeyerSignFor(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	msg := []byte("hello")

	pk, err := sk.Secp256k1Public()
	assert.NoError(t, err)
	sig, err := sk.SignFor(*pk, msg)
	assert.NoError(t, err)
	assert.NoError(t, sig.Verify(msg, pk))

	pk2 := sk.Ed25519Public()
	sig2, err := sk.SignFor(*pk2, msg)
	assert.NoError(t, err)
	assert.NoError(t, sig2.Verify(msg, pk2))

	pk3 := sk.BLS12381Public()
	sig3, err := sk.SignFor(*pk3, msg)
	assert.NoError(t, err)
	assert.NoError(t, sig3.Verify(msg, pk3))

	_, err = sk.SignFor(crypto.PublicKey{Variant: crypto.PublicKeyTypeFnDsa512}, msg)
	assert.Error(t, err)

	fnSk, fnPk, err := crypto.NewFnDsa512SecretKey()
	assert.NoError(t, err)
	sig4, err := fnSk.SignFor(*fnPk, msg)
	assert.NoError(t, err)
	assert.NoError(t, sig4.Verify(msg, fnPk))

	_, err = fnSk.SignFor(*pk, msg)
	assert.Error(t, err)
}

// ============================================
// Benchmark
// ============================================

func BenchmarkSecretKeyerNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = crypto.NewClassicalSecretKey()
	}
}

func BenchmarkSecretKeyerNewPure(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = crypto.NewPureClassicalSecretKey()
	}
}

func BenchmarkSecretKeyerSignVerify(b *testing.B) {
	benchCases := []struct {
		name  string
		setup func() (func([]byte) (*crypto.Signature, error), func([]byte, *crypto.Signature) error)
	}{
		{
			name: "Secp256k1",
			setup: func() (func([]byte) (*crypto.Signature, error), func([]byte, *crypto.Signature) error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
				pk, err := sk.Secp256k1Public()
				if err != nil {
					b.Fatal(err)
				}
				return sk.SignSecp256k1, func(msg []byte, sig *crypto.Signature) error { return sig.Verify(msg, pk) }
			},
		},
		{
			name: "Ed25519",
			setup: func() (func([]byte) (*crypto.Signature, error), func([]byte, *crypto.Signature) error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
				pk := sk.Ed25519Public()
				return func(msg []byte) (*crypto.Signature, error) { return sk.SignEd25519(msg), nil },
					func(msg []byte, sig *crypto.Signature) error { return sig.Verify(msg, pk) }
			},
		},
		{
			name: "BLS12381",
			setup: func() (func([]byte) (*crypto.Signature, error), func([]byte, *crypto.Signature) error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
				pk := sk.BLS12381Public()
				return func(msg []byte) (*crypto.Signature, error) { return sk.SignBLS12381(msg), nil },
					func(msg []byte, sig *crypto.Signature) error { return sig.Verify(msg, pk) }
			},
		},
		{
			name: "FnDsa512",
			setup: func() (func([]byte) (*crypto.Signature, error), func([]byte, *crypto.Signature) error) {
				sker, pk, err := crypto.NewFnDsa512SecretKey()
				if err != nil {
					b.Fatal(err)
				}
				sk := crypto.AsFnDsa512SecretKey(sker)
				return sk.SignFnDsa512, func(msg []byte, sig *crypto.Signature) error { return sig.Verify(msg, pk) }
			},
		},
	}

	msg := []byte("benchmark message")
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			sign, verify := bc.setup()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sig, err := sign(msg)
				if err != nil {
					b.Fatal(err)
				}
				if err := verify(msg, sig); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
