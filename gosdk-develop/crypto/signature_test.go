package crypto

import (
	"encoding/json"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecp256k1SigAndVerifyAndRoundTrip(t *testing.T) {
	sk := AsClassicalSecretKey(NewClassicalSecretKey())

	pk, err := sk.Secp256k1Public()
	assert.NoError(t, err)

	msg := []byte("hello secp")
	sig1, err := sk.SignSecp256k1(msg)
	assert.NoError(t, err)
	assert.Equal(t, SignatureTypeSecp256k1, sig1.Variant)
	err = sig1.Verify(msg, pk)
	assert.NoError(t, err)

	sig2, err := NewSignatureFromBytes(sig1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig2.Bytes)
	assert.Equal(t, sig1.Variant, sig2.Variant)

	sig3, err := NewSignatureFromStringRelaxed(sig1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig3.Bytes)
	assert.Equal(t, sig1.Variant, sig3.Variant)

	sig4, err := NewSignatureFromStringRelaxed("0x" + sig1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig4.Bytes)
	assert.Equal(t, sig1.Variant, sig4.Variant)

	sig5, err := NewSignatureFromStringRelaxed(sig1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig5.Bytes)
	assert.Equal(t, sig1.Variant, sig5.Variant)
}

func TestEd25519SigAndVerifyAndRoundTrip(t *testing.T) {
	sk := AsClassicalSecretKey(NewPureClassicalSecretKey())

	pk := sk.Ed25519Public()

	msg := []byte("hello ed25519")
	sig1 := sk.SignEd25519(msg)
	assert.Equal(t, SignatureTypeEd25519, sig1.Variant)
	err := sig1.Verify(msg, pk)
	assert.NoError(t, err)

	sig2, err := NewSignatureFromBytes(sig1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig2.Bytes)
	assert.Equal(t, sig1.Variant, sig2.Variant)

	sig3, err := NewSignatureFromStringRelaxed(sig1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig3.Bytes)
	assert.Equal(t, sig1.Variant, sig3.Variant)

	sig4, err := NewSignatureFromStringRelaxed("0x" + sig1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig4.Bytes)
	assert.Equal(t, sig1.Variant, sig4.Variant)

	sig5, err := NewSignatureFromStringRelaxed(sig1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig5.Bytes)
	assert.Equal(t, sig1.Variant, sig5.Variant)
}

func TestBLS12381SigAndVerifyAndRoundTrip(t *testing.T) {
	sk := AsClassicalSecretKey(NewPureClassicalSecretKey())

	pk := sk.BLS12381Public()

	msg := []byte("hello bls")
	sig1 := sk.SignBLS12381(msg)
	assert.Equal(t, SignatureTypeBLS12381, sig1.Variant)
	err := sig1.Verify(msg, pk)
	assert.NoError(t, err)

	sig2, err := NewSignatureFromBytes(sig1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig2.Bytes)
	assert.Equal(t, sig1.Variant, sig2.Variant)

	sig3, err := NewSignatureFromStringRelaxed(sig1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig3.Bytes)
	assert.Equal(t, sig1.Variant, sig3.Variant)

	sig4, err := NewSignatureFromStringRelaxed("0x" + sig1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig4.Bytes)
	assert.Equal(t, sig1.Variant, sig4.Variant)

	sig5, err := NewSignatureFromStringRelaxed(sig1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig5.Bytes)
	assert.Equal(t, sig1.Variant, sig5.Variant)
}

func TestFnDsa512SigAndVerifyAndRoundTrip(t *testing.T) {
	sker, pk, err := NewFnDsa512SecretKey()
	assert.NoError(t, err)

	sk := AsFnDsa512SecretKey(sker)

	msg := []byte("hello fn-dsa-512")
	sig1, err := sk.SignFnDsa512(msg)
	assert.NoError(t, err)
	assert.Equal(t, SignatureTypeFnDsa512, sig1.Variant)
	err = sig1.Verify(msg, pk)
	assert.NoError(t, err)

	sig2, err := NewSignatureFromBytes(sig1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig2.Bytes)
	assert.Equal(t, sig1.Variant, sig2.Variant)

	sig3, err := NewSignatureFromStringRelaxed(sig1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig3.Bytes)
	assert.Equal(t, sig1.Variant, sig3.Variant)

	sig4, err := NewSignatureFromStringRelaxed("0x" + sig1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig4.Bytes)
	assert.Equal(t, sig1.Variant, sig4.Variant)

	sig5, err := NewSignatureFromStringRelaxed(sig1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, sig1.Bytes, sig5.Bytes)
	assert.Equal(t, sig1.Variant, sig5.Variant)
}

func TestInvalidSignatureVariantMethods(t *testing.T) {
	sk := AsClassicalSecretKey(NewClassicalSecretKey())
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

func TestVerifyBatchEmpty(t *testing.T) {
	err := VerifyBatch([]*Signature{}, [][]byte{}, []*PublicKey{})
	assert.NoError(t, err)
}

func TestVerifyBatchLengthMismatch(t *testing.T) {
	sk := AsClassicalSecretKey(NewPureClassicalSecretKey())
	pk := sk.Ed25519Public()
	sig := sk.SignEd25519([]byte("a"))

	err := VerifyBatch([]*Signature{sig}, [][]byte{[]byte("a"), []byte("b")}, []*PublicKey{pk})
	assert.Error(t, err)
}

func TestVerifyBatchSecp256k1Three(t *testing.T) {
	sigs := make([]*Signature, 3)
	msgs := make([][]byte, 3)
	pks := make([]*PublicKey, 3)
	var err error

	for i := 0; i < 3; i++ {
		sk := AsClassicalSecretKey(NewPureClassicalSecretKey())
		pks[i], err = sk.Secp256k1Public()
		assert.NoError(t, err)
		msgs[i] = []byte{byte(i), byte(i), byte(i)}
		sigs[i], err = sk.SignSecp256k1(msgs[i])
		assert.NoError(t, err)
	}

	err = VerifyBatch(sigs, msgs, pks)
	assert.NoError(t, err)
}

func TestVerifyBatchEd25519Three(t *testing.T) {
	sigs := make([]*Signature, 3)
	msgs := make([][]byte, 3)
	pks := make([]*PublicKey, 3)

	for i := 0; i < 3; i++ {
		sk := AsClassicalSecretKey(NewPureClassicalSecretKey())
		pks[i] = sk.Ed25519Public()
		msgs[i] = []byte{byte(i), byte(i), byte(i)}
		sigs[i] = sk.SignEd25519(msgs[i])
	}

	err := VerifyBatch(sigs, msgs, pks)
	assert.NoError(t, err)
}

func TestVerifyBatchBLS12381Three(t *testing.T) {
	sigs := make([]*Signature, 3)
	msgs := make([][]byte, 3)
	pks := make([]*PublicKey, 3)

	for i := 0; i < 3; i++ {
		sk := AsClassicalSecretKey(NewPureClassicalSecretKey())
		pks[i] = sk.BLS12381Public()
		msgs[i] = []byte{byte(i), byte(i), byte(i)}
		sigs[i] = sk.SignBLS12381(msgs[i])
	}

	err := VerifyBatch(sigs, msgs, pks)
	assert.NoError(t, err)
}

func TestVerifyBatchFnDsa512Three(t *testing.T) {
	sigs := make([]*Signature, 3)
	msgs := make([][]byte, 3)
	pks := make([]*PublicKey, 3)

	for i := 0; i < 3; i++ {
		sker, pk, err := NewFnDsa512SecretKey()
		assert.NoError(t, err)
		sk := AsFnDsa512SecretKey(sker)
		pks[i] = pk
		msgs[i] = []byte{byte(i), byte(i), byte(i)}
		sig, err := sk.SignFnDsa512(msgs[i])
		sigs[i] = sig
	}

	err := VerifyBatch(sigs, msgs, pks)
	assert.NoError(t, err)
}

func TestVerifyBatchAll(t *testing.T) {
	sk1 := AsClassicalSecretKey(NewPureClassicalSecretKey())
	pk1, err := sk1.Secp256k1Public()
	assert.NoError(t, err)
	m1 := []byte("1")
	sig1, err := sk1.SignSecp256k1(m1)
	assert.NoError(t, err)

	sk2 := AsClassicalSecretKey(NewPureClassicalSecretKey())
	pk2 := sk2.Ed25519Public()
	m2 := []byte("2")
	sig2 := sk2.SignEd25519(m2)

	sk3 := AsClassicalSecretKey(NewPureClassicalSecretKey())
	pk3 := sk3.BLS12381Public()
	m3 := []byte("3")
	sig3 := sk3.SignBLS12381(m3)

	sker4, pk4, err := NewFnDsa512SecretKey()
	assert.NoError(t, err)
	sk4 := AsFnDsa512SecretKey(sker4)
	m4 := []byte("4")
	sig4, err := sk4.SignFnDsa512(m4)
	assert.NoError(t, err)

	sigs := []*Signature{sig1, sig2, sig3, sig4}
	msgs := [][]byte{m1, m2, m3, m4}
	pks := []*PublicKey{pk1, pk2, pk3, pk4}
	err = VerifyBatch(sigs, msgs, pks)
	assert.NoError(t, err)
}

func TestSignatureJSONRoundTrip(t *testing.T) {
	testCases := []struct {
		name      string
		createSig func() *Signature
	}{
		{
			name: "Secp256k1",
			createSig: func() *Signature {
				sig, err := AsClassicalSecretKey(NewClassicalSecretKey()).SignSecp256k1([]byte("test"))
				assert.NoError(t, err)
				return sig
			},
		},
		{
			name: "Ed25519",
			createSig: func() *Signature {
				sig := AsClassicalSecretKey(NewClassicalSecretKey()).SignEd25519([]byte("test"))
				return sig
			},
		},
		{
			name: "BLS12381",
			createSig: func() *Signature {
				sig := AsClassicalSecretKey(NewClassicalSecretKey()).SignBLS12381([]byte("test"))
				return sig
			},
		},
		{
			name: "FnDsa512",
			createSig: func() *Signature {
				sker, _, err := NewFnDsa512SecretKey()
				assert.NoError(t, err)
				sig, err := AsFnDsa512SecretKey(sker).SignFnDsa512([]byte("test"))
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

			var decoded Signature
			err = json.Unmarshal(jsonData, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, sig.Bytes, decoded.Bytes)
			assert.Equal(t, sig.Variant, decoded.Variant)
		})
	}
}

func TestSignaturePostcardRoundTrip(t *testing.T) {
	// 综合测试所有类型的签名
	testCases := []struct {
		name        string
		createSig   func() *Signature
		expectedLen int
	}{
		{
			name: "Secp256k1-65bytes",
			createSig: func() *Signature {
				sig, err := AsClassicalSecretKey(NewClassicalSecretKey()).SignSecp256k1([]byte("test"))
				assert.NoError(t, err)
				return sig
			},
			expectedLen: SignatureSecp256k1Size,
		},
		{
			name: "Ed25519-64bytes",
			createSig: func() *Signature {
				return AsClassicalSecretKey(NewPureClassicalSecretKey()).SignEd25519([]byte("test"))
			},
			expectedLen: SignatureEd25519Size,
		},
		{
			name: "BLS12381-96bytes",
			createSig: func() *Signature {
				return AsClassicalSecretKey(NewPureClassicalSecretKey()).SignBLS12381([]byte("test"))
			},
			expectedLen: SignatureBLS12381Size,
		},
		{
			name: "FnDsa512-666bytes",
			createSig: func() *Signature {
				sk, _, err := NewFnDsa512SecretKey()
				assert.NoError(t, err)
				sig, err := AsFnDsa512SecretKey(sk).SignFnDsa512([]byte("test"))
				assert.NoError(t, err)
				return sig
			},
			expectedLen: SignatureFnDsa512Size,
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
			decoded := &Signature{}
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

func TestSignatureSerializePostcardInterface(t *testing.T) {
	sig := AsClassicalSecretKey(NewClassicalSecretKey()).SignEd25519([]byte("test"))

	buf, err := postcard.SerializePostcard(sig)
	assert.NoError(t, err)

	deserializer := postcard.NewDeserializer(buf)
	decoded := &Signature{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	assert.Equal(t, sig.Bytes, decoded.Bytes)

	err = deserializer.AssertEnd()
	assert.NoError(t, err)
}

func TestSignatureSerializerWithMultipleFields(t *testing.T) {
	sig := AsClassicalSecretKey(NewClassicalSecretKey()).SignEd25519([]byte("test"))

	serializer := postcard.NewSerializer()

	err := serializer.SerializeU32(1)
	assert.NoError(t, err)
	err = sig.MarshalPostcard(serializer)
	assert.NoError(t, err)
	err = serializer.SerializeStr("test-data")
	assert.NoError(t, err)
	data := serializer.Bytes()
	assert.NotEmpty(t, data)

	deserializer := postcard.NewDeserializer(data)

	id, err := deserializer.DeserializeU32()
	assert.NoError(t, err)
	assert.Equal(t, uint32(1), id)

	decoded := &Signature{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	assert.Equal(t, sig.Bytes, decoded.Bytes)

	str, err := deserializer.DeserializeStr()
	assert.NoError(t, err)
	assert.Equal(t, "test-data", str)

	err = deserializer.AssertEnd()
	assert.NoError(t, err)
}

func TestSignatureDeserializePostcardInvalidData(t *testing.T) {
	sig := &Signature{}
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

func TestSignatureDeserializePostcardTrailingBytes(t *testing.T) {
	sig := AsClassicalSecretKey(NewClassicalSecretKey()).SignEd25519([]byte("test"))

	serializer := postcard.NewSerializer()
	err := sig.MarshalPostcard(serializer)
	assert.NoError(t, err)

	data := serializer.Bytes()
	data = append(data, []byte{0xFF, 0xFE, 0xFD}...)

	deserializer := postcard.NewDeserializer(data)
	decoded := &Signature{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	err = deserializer.AssertEnd()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trailing bytes")

	deserializer = postcard.NewDeserializer(data)
	decoded = &Signature{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	assert.Equal(t, sig.Bytes, decoded.Bytes)
	assert.Equal(t, 3, deserializer.Remaining())
}
