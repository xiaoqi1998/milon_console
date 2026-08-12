package test

import (
	"encoding/json"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignatureSignVerifyAndRoundTrip(t *testing.T) {
	msg := []byte("hello crypto")

	testCases := []struct {
		name       string
		sigType    crypto.SignatureType
		createPair func() (*crypto.Signature, *crypto.PublicKey, error)
	}{
		{
			name:    "Secp256k1",
			sigType: crypto.SignatureTypeSecp256k1,
			createPair: func() (*crypto.Signature, *crypto.PublicKey, error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
				pk, err := sk.Secp256k1Public()
				if err != nil {
					return nil, nil, err
				}
				sig, err := sk.SignSecp256k1(msg)
				return sig, pk, err
			},
		},
		{
			name:    "Ed25519",
			sigType: crypto.SignatureTypeEd25519,
			createPair: func() (*crypto.Signature, *crypto.PublicKey, error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
				return sk.SignEd25519(msg), sk.Ed25519Public(), nil
			},
		},
		{
			name:    "BLS12381",
			sigType: crypto.SignatureTypeBLS12381,
			createPair: func() (*crypto.Signature, *crypto.PublicKey, error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
				return sk.SignBLS12381(msg), sk.BLS12381Public(), nil
			},
		},
		{
			name:    "FnDsa512",
			sigType: crypto.SignatureTypeFnDsa512,
			createPair: func() (*crypto.Signature, *crypto.PublicKey, error) {
				sker, pk, err := crypto.NewFnDsa512SecretKey()
				if err != nil {
					return nil, nil, err
				}
				sig, err := crypto.AsFnDsa512SecretKey(sker).SignFnDsa512(msg)
				return sig, pk, err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sig1, pk, err := tc.createPair()
			assert.NoError(t, err)
			assert.Equal(t, tc.sigType, sig1.Variant)
			assert.NoError(t, sig1.Verify(msg, pk))

			sig2, err := crypto.NewSignatureFromBytes(sig1.AsBytes())
			assert.NoError(t, err)
			assert.Equal(t, sig1.Bytes, sig2.Bytes)
			assert.Equal(t, sig1.Variant, sig2.Variant)

			sig3, err := crypto.NewSignatureFromStringRelaxed(sig1.ToHex())
			assert.NoError(t, err)
			assert.Equal(t, sig1.Bytes, sig3.Bytes)
			assert.Equal(t, sig1.Variant, sig3.Variant)

			sig4, err := crypto.NewSignatureFromStringRelaxed("0x" + sig1.ToHex())
			assert.NoError(t, err)
			assert.Equal(t, sig1.Bytes, sig4.Bytes)
			assert.Equal(t, sig1.Variant, sig4.Variant)

			sig5, err := crypto.NewSignatureFromStringRelaxed(sig1.ToBase58())
			assert.NoError(t, err)
			assert.Equal(t, sig1.Bytes, sig5.Bytes)
			assert.Equal(t, sig1.Variant, sig5.Variant)
		})
	}
}

func TestSignatureWrongVariantConversions(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	msg := []byte("z")
	sig, err := sk.SignSecp256k1(msg)
	assert.NoError(t, err)

	_, err = sig.ToSecp256k1()
	assert.NoError(t, err)

	_, err = sig.ToEd25519()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an ed25519 signature")

	_, err = sig.ToBLS12381()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a BLS signature")

	_, err = sig.ToFnDsa512()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a FN-DSA-512 signature")
}

func TestSignatureFromBytesWrongLen(t *testing.T) {
	_, err := crypto.NewSignatureFromBytes([]byte{})
	assert.Error(t, err)

	_, err = crypto.NewSignatureFromBytes(make([]byte, 63))
	assert.Error(t, err)

	_, err = crypto.NewSignatureFromBytes(make([]byte, 95))
	assert.Error(t, err)
}

func TestSignatureFromStringRelaxedInvalidFormat(t *testing.T) {
	_, err := crypto.NewSignatureFromStringRelaxed("not a valid hex string")
	assert.Error(t, err)

	_, err = crypto.NewSignatureFromStringRelaxed("!!!invalid base58!!!")
	assert.Error(t, err)
}

func TestVerifyBatchEmpty(t *testing.T) {
	err := crypto.VerifyBatch([]*crypto.Signature{}, [][]byte{}, []*crypto.PublicKey{})
	assert.NoError(t, err)
}

func TestVerifyBatchLengthMismatch(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk := sk.Ed25519Public()
	sig := sk.SignEd25519([]byte("a"))

	err := crypto.VerifyBatch([]*crypto.Signature{sig}, [][]byte{[]byte("a"), []byte("b")}, []*crypto.PublicKey{pk})
	assert.Error(t, err)
}

func TestVerifyBatchThree(t *testing.T) {
	testCases := []struct {
		name       string
		createPair func() (*crypto.Signature, []byte, *crypto.PublicKey, error)
	}{
		{
			name: "Secp256k1",
			createPair: func() (*crypto.Signature, []byte, *crypto.PublicKey, error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
				pk, err := sk.Secp256k1Public()
				if err != nil {
					return nil, nil, nil, err
				}
				msg := []byte("a")
				sig, err := sk.SignSecp256k1(msg)
				return sig, msg, pk, err
			},
		},
		{
			name: "Ed25519",
			createPair: func() (*crypto.Signature, []byte, *crypto.PublicKey, error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
				msg := []byte("b")
				return sk.SignEd25519(msg), msg, sk.Ed25519Public(), nil
			},
		},
		{
			name: "BLS12381",
			createPair: func() (*crypto.Signature, []byte, *crypto.PublicKey, error) {
				sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
				msg := []byte("c")
				return sk.SignBLS12381(msg), msg, sk.BLS12381Public(), nil
			},
		},
		{
			name: "FnDsa512",
			createPair: func() (*crypto.Signature, []byte, *crypto.PublicKey, error) {
				sker, pk, err := crypto.NewFnDsa512SecretKey()
				if err != nil {
					return nil, nil, nil, err
				}
				msg := []byte("d")
				sig, err := crypto.AsFnDsa512SecretKey(sker).SignFnDsa512(msg)
				return sig, msg, pk, err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sigs := make([]*crypto.Signature, 3)
			msgs := make([][]byte, 3)
			pks := make([]*crypto.PublicKey, 3)
			for i := 0; i < 3; i++ {
				sig, msg, pk, err := tc.createPair()
				assert.NoError(t, err)
				sigs[i], msgs[i], pks[i] = sig, msg, pk
			}
			assert.NoError(t, crypto.VerifyBatch(sigs, msgs, pks))
		})
	}
}

func TestVerifyBatchAll(t *testing.T) {
	sk1 := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk1, err := sk1.Secp256k1Public()
	assert.NoError(t, err)
	m1 := []byte("1")
	sig1, err := sk1.SignSecp256k1(m1)
	assert.NoError(t, err)

	sk2 := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk2 := sk2.Ed25519Public()
	m2 := []byte("2")
	sig2 := sk2.SignEd25519(m2)

	sk3 := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk3 := sk3.BLS12381Public()
	m3 := []byte("3")
	sig3 := sk3.SignBLS12381(m3)

	sker4, pk4, err := crypto.NewFnDsa512SecretKey()
	assert.NoError(t, err)
	sk4 := crypto.AsFnDsa512SecretKey(sker4)
	m4 := []byte("4")
	sig4, err := sk4.SignFnDsa512(m4)
	assert.NoError(t, err)

	sigs := []*crypto.Signature{sig1, sig2, sig3, sig4}
	msgs := [][]byte{m1, m2, m3, m4}
	pks := []*crypto.PublicKey{pk1, pk2, pk3, pk4}
	err = crypto.VerifyBatch(sigs, msgs, pks)
	assert.NoError(t, err)
}

func TestSignatureVerifyTypeMismatch(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	sig := sk.SignEd25519([]byte("msg"))

	pk, err := sk.Secp256k1Public()
	assert.NoError(t, err)
	err = sig.Verify([]byte("msg"), pk)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "type mismatch")
}

func TestSignatureJSONRoundTrip(t *testing.T) {
	testCases := []struct {
		name      string
		createSig func() *crypto.Signature
	}{
		{
			name: "Secp256k1",
			createSig: func() *crypto.Signature {
				sig, err := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).SignSecp256k1([]byte("test"))
				assert.NoError(t, err)
				return sig
			},
		},
		{
			name: "Ed25519",
			createSig: func() *crypto.Signature {
				sig := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).SignEd25519([]byte("test"))
				return sig
			},
		},
		{
			name: "BLS12381",
			createSig: func() *crypto.Signature {
				sig := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).SignBLS12381([]byte("test"))
				return sig
			},
		},
		{
			name: "FnDsa512",
			createSig: func() *crypto.Signature {
				sker, _, err := crypto.NewFnDsa512SecretKey()
				assert.NoError(t, err)
				sig, err := crypto.AsFnDsa512SecretKey(sker).SignFnDsa512([]byte("test"))
				assert.NoError(t, err)
				return sig
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sig := tc.createSig()

			jsonData, err := json.Marshal(sig)
			assert.NoError(t, err)

			var str string
			err = json.Unmarshal(jsonData, &str)
			assert.NoError(t, err)
			assert.Equal(t, sig.ToBase58(), str)

			var decoded crypto.Signature
			err = json.Unmarshal(jsonData, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, sig.Bytes, decoded.Bytes)
			assert.Equal(t, sig.Variant, decoded.Variant)
		})
	}
}

func TestSignaturePostcardRoundTrip(t *testing.T) {
	testCases := []struct {
		name        string
		createSig   func() *crypto.Signature
		expectedLen int
	}{
		{
			name: "Secp256k1-65bytes",
			createSig: func() *crypto.Signature {
				sig, err := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).SignSecp256k1([]byte("test"))
				assert.NoError(t, err)
				return sig
			},
			expectedLen: crypto.SignatureSecp256k1Size,
		},
		{
			name: "Ed25519-64bytes",
			createSig: func() *crypto.Signature {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).SignEd25519([]byte("test"))
			},
			expectedLen: crypto.SignatureEd25519Size,
		},
		{
			name: "BLS12381-96bytes",
			createSig: func() *crypto.Signature {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).SignBLS12381([]byte("test"))
			},
			expectedLen: crypto.SignatureBLS12381Size,
		},
		{
			name: "FnDsa512-666bytes",
			createSig: func() *crypto.Signature {
				sk, _, err := crypto.NewFnDsa512SecretKey()
				assert.NoError(t, err)
				sig, err := crypto.AsFnDsa512SecretKey(sk).SignFnDsa512([]byte("test"))
				assert.NoError(t, err)
				return sig
			},
			expectedLen: crypto.SignatureFnDsa512Size,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sig := tc.createSig()
			assert.Equal(t, tc.expectedLen, len(sig.Bytes))

			serializer := postcard.NewSerializer()
			err := sig.MarshalPostcard(serializer)
			assert.NoError(t, err)

			data := serializer.Bytes()
			deserializer := postcard.NewDeserializer(data)
			decoded := &crypto.Signature{}
			err = decoded.UnmarshalPostcard(deserializer)
			assert.NoError(t, err)
			assert.Equal(t, sig.Bytes, decoded.Bytes)
			assert.Equal(t, sig.Variant, decoded.Variant)
			assert.Equal(t, tc.expectedLen, len(decoded.Bytes))

			err = deserializer.AssertEnd()
			assert.NoError(t, err)
		})
	}
}

func TestSignatureDeserializePostcardInvalidData(t *testing.T) {
	sig := &crypto.Signature{}
	deserializer := postcard.NewDeserializer([]byte{})
	err := sig.UnmarshalPostcard(deserializer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize signature variant")

	deserializer = postcard.NewDeserializer([]byte{0x05})
	err = sig.UnmarshalPostcard(deserializer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown signature variant")

	deserializer = postcard.NewDeserializer([]byte{0x01, 0x02, 0x03, 0x04, 0x05})
	err = sig.UnmarshalPostcard(deserializer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize signature Bytes")
}
