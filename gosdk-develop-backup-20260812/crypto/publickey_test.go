package crypto

import (
	"bytes"
	"encoding/json"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicKeyVariant(t *testing.T) {
	pk1, err := AsClassicalSecretKey(NewClassicalSecretKey()).Secp256k1Public()
	assert.NoError(t, err)
	assert.Equal(t, PublicKeyTypeSecp256k1, pk1.Variant)
	assert.True(t, pk1.IsSecp256k1())
	assert.False(t, pk1.IsEd25519())
	assert.False(t, pk1.IsBLS12381())
	assert.False(t, pk1.IsFnDsa512())

	pk2 := AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public()
	assert.Equal(t, PublicKeyTypeEd25519, pk2.Variant)
	assert.True(t, pk2.IsEd25519())
	assert.False(t, pk2.IsSecp256k1())
	assert.False(t, pk2.IsBLS12381())
	assert.False(t, pk2.IsFnDsa512())

	pk3 := AsClassicalSecretKey(NewPureClassicalSecretKey()).BLS12381Public()
	assert.Equal(t, PublicKeyTypeBLS12381, pk3.Variant)
	assert.True(t, pk3.IsBLS12381())
	assert.False(t, pk3.IsSecp256k1())
	assert.False(t, pk3.IsEd25519())
	assert.False(t, pk3.IsFnDsa512())

	_, pk4, err := NewFnDsa512SecretKey()
	assert.NoError(t, err)
	assert.Equal(t, PublicKeyTypeFnDsa512, pk4.Variant)
	assert.False(t, pk4.IsBLS12381())
	assert.False(t, pk4.IsSecp256k1())
	assert.False(t, pk4.IsEd25519())
	assert.True(t, pk4.IsFnDsa512())
}

func TestPublicKeySecp256k1RoundTrip(t *testing.T) {
	pk1, err := AsClassicalSecretKey(NewClassicalSecretKey()).Secp256k1Public()
	assert.NoError(t, err)

	pk2, err := NewPublicKeyFromBytes(pk1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk2.Variant)
	assert.Equal(t, pk1.Bytes, pk2.Bytes)

	pk3, err := NewPublicKeyFromStringRelaxed(pk1.ToHex())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk3.Variant, pk3.Variant)
	assert.Equal(t, pk3.Bytes, pk3.Bytes)

	pk4, err := NewPublicKeyFromStringRelaxed("0x" + pk1.ToHex())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk4.Variant, pk4.Variant)
	assert.Equal(t, pk4.Bytes, pk4.Bytes)

	pk5, err := NewPublicKeyFromStringRelaxed(pk1.ToBase58())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk5.Variant, pk5.Variant)
	assert.Equal(t, pk5.Bytes, pk5.Bytes)
}

func TestPublicKeyEd25519RoundTrip(t *testing.T) {
	pk1 := AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public()

	pk2, err := NewPublicKeyFromBytes(pk1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk2.Variant)
	assert.Equal(t, pk1.Bytes, pk2.Bytes)

	pk3, err := NewPublicKeyFromStringRelaxed(pk1.ToHex())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk3.Variant)
	assert.Equal(t, pk1.Bytes, pk3.Bytes)

	pk4, err := NewPublicKeyFromStringRelaxed("0x" + pk1.ToHex())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk4.Variant)
	assert.Equal(t, pk1.Bytes, pk4.Bytes)

	pk5, err := NewPublicKeyFromStringRelaxed(pk1.ToBase58())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk5.Variant)
	assert.Equal(t, pk1.Bytes, pk5.Bytes)
}

func TestPublicKeyBLS12381RoundTrip(t *testing.T) {
	pk1 := AsClassicalSecretKey(NewPureClassicalSecretKey()).BLS12381Public()

	pk2, err := NewPublicKeyFromBytes(pk1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk2.Variant)
	assert.Equal(t, pk1.Bytes, pk2.Bytes)

	pk3, err := NewPublicKeyFromStringRelaxed(pk1.ToHex())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk3.Variant)
	assert.Equal(t, pk1.Bytes, pk3.Bytes)

	p4, err := NewPublicKeyFromStringRelaxed("0x" + pk1.ToHex())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, p4.Variant)
	assert.Equal(t, pk1.Bytes, p4.Bytes)

	pk5, err := NewPublicKeyFromStringRelaxed(pk1.ToBase58())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk5.Variant)
	assert.Equal(t, pk1.Bytes, pk5.Bytes)
}

func TestPublicKeyFnDsa512RoundTrip(t *testing.T) {
	_, pk1, err := NewFnDsa512SecretKey()
	assert.NoError(t, err)

	pk2, err := NewPublicKeyFromBytes(pk1.AsBytes())
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk2.Variant)
	assert.Equal(t, pk1.Bytes, pk2.Bytes)

	pk3, err := NewPublicKeyFromStringRelaxed(pk1.ToHex())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk3.Variant)
	assert.Equal(t, pk1.Bytes, pk3.Bytes)

	pk4, err := NewPublicKeyFromStringRelaxed("0x" + pk1.ToHex())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk4.Variant)
	assert.Equal(t, pk1.Bytes, pk4.Bytes)

	pk5, err := NewPublicKeyFromStringRelaxed(pk1.ToBase58())
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Variant, pk5.Variant)
	assert.Equal(t, pk1.Bytes, pk5.Bytes)
}

func TestPublicKeyFromBytesWrongLen(t *testing.T) {
	_, err := NewPublicKeyFromBytes([]byte{})
	assert.Error(t, err)

	_, err = NewPublicKeyFromBytes(make([]byte, 31))
	assert.Error(t, err)

	_, err = NewPublicKeyFromBytes(make([]byte, 47))
	assert.Error(t, err)
}

func TestPublicKeyWrongVariantConversions(t *testing.T) {
	pk := AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public()

	_, err := pk.ToEd25519()
	assert.NoError(t, err)

	_, err = pk.ToSecp256k1()
	assert.Error(t, err)

	_, err = pk.ToBLS12381()
	assert.Error(t, err)
}

func TestPublicKeyToNative(t *testing.T) {
	// Secp256k1
	pk1, err := AsClassicalSecretKey(NewClassicalSecretKey()).Secp256k1Public()
	assert.NoError(t, err)

	native1, err := pk1.ToSecp256k1()
	assert.NoError(t, err)

	pk1Decoded := PublicKey{}
	err = pk1Decoded.FromSecp256k1Native(native1)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Bytes, pk1Decoded.AsBytes())

	// Ed25519
	pk2 := AsClassicalSecretKey(NewClassicalSecretKey()).Ed25519Public()
	native2, err := pk2.ToEd25519()
	assert.NoError(t, err)

	pk2Decoded := PublicKey{}
	err = pk2Decoded.FromEd25519Native(native2)
	assert.NoError(t, err)
	assert.Equal(t, pk2.Bytes, pk2Decoded.AsBytes())

	// BLS
	pk3 := AsClassicalSecretKey(NewClassicalSecretKey()).BLS12381Public()
	native3, err := pk3.ToBLS12381()
	assert.NoError(t, err)

	pk3Decoded := PublicKey{}
	err = pk3Decoded.FromBLS12381Native(native3)
	assert.NoError(t, err)
	assert.Equal(t, pk3.Bytes, pk3Decoded.AsBytes())
}

func TestPublicKeyJSONRoundTrip(t *testing.T) {
	testCases := []struct {
		name     string
		createPk func() *PublicKey
	}{
		{
			name: "Secp256k1",
			createPk: func() *PublicKey {
				pk, err := AsClassicalSecretKey(NewPureClassicalSecretKey()).Secp256k1Public()
				assert.NoError(t, err)
				return pk
			},
		},
		{
			name:     "Ed25519",
			createPk: func() *PublicKey { return AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public() },
		},
		{
			name:     "BLS12381",
			createPk: func() *PublicKey { return AsClassicalSecretKey(NewPureClassicalSecretKey()).BLS12381Public() },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pk := tc.createPk()

			jsonData, err := json.Marshal(pk)
			assert.NoError(t, err)

			var b58Str string
			err = json.Unmarshal(jsonData, &b58Str)
			assert.NoError(t, err)
			assert.Equal(t, b58Str, pk.ToBase58())

			decoded := &PublicKey{}
			err = json.Unmarshal(jsonData, &decoded)
			assert.NoError(t, err)
			assert.True(t, bytes.Equal(pk.Bytes, decoded.Bytes))
			assert.Equal(t, pk.Bytes, decoded.Bytes)
			assert.Equal(t, pk.Variant, decoded.Variant)
		})
	}
}

func TestPublicKeyPostcardRoundTrip(t *testing.T) {
	// 综合测试所有类型的公钥
	testCases := []struct {
		name        string
		createPk    func() *PublicKey
		expectedLen int
	}{
		{
			name: "Secp256k1-33bytes",
			createPk: func() *PublicKey {
				pk, err := AsClassicalSecretKey(NewPureClassicalSecretKey()).Secp256k1Public()
				assert.NoError(t, err)
				return pk
			},
			expectedLen: PublicKeySecp256k1Size,
		},
		{
			name:        "Ed25519-32bytes",
			createPk:    func() *PublicKey { return AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public() },
			expectedLen: PublicKeyEd25519Size,
		},
		{
			name:        "BLS12381-48bytes",
			createPk:    func() *PublicKey { return AsClassicalSecretKey(NewPureClassicalSecretKey()).BLS12381Public() },
			expectedLen: PublicKeyBLS12381Size,
		},
		{
			name: "FnDsa512-897bytes",
			createPk: func() *PublicKey {
				_, pk, err := NewFnDsa512SecretKey()
				assert.NoError(t, err)
				return pk
			},
			expectedLen: PublicKeyFnDsa512Size,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pk := tc.createPk()
			assert.Equal(t, tc.expectedLen, len(pk.Bytes))

			serializer := postcard.NewSerializer()
			err := pk.MarshalPostcard(serializer)
			assert.NoError(t, err)

			data := serializer.Bytes()
			deserializer := postcard.NewDeserializer(data)
			decoded := &PublicKey{}
			err = decoded.UnmarshalPostcard(deserializer)
			assert.NoError(t, err)
			assert.True(t, bytes.Equal(pk.Bytes, decoded.Bytes))
			assert.Equal(t, pk.Variant, decoded.Variant)
			assert.Equal(t, tc.expectedLen, len(decoded.Bytes))

			err = deserializer.AssertEnd()
			assert.NoError(t, err)
		})
	}
}

func TestPublicKeySerializePostcardInterface(t *testing.T) {
	pk := AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public()

	buf, err := postcard.SerializePostcard(pk)
	assert.NoError(t, err)

	deserializer := postcard.NewDeserializer(buf)
	decoded := &PublicKey{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(pk.Bytes, decoded.Bytes))

	err = deserializer.AssertEnd()
	assert.NoError(t, err)
}

func TestPublicKeySerializerPostcardWithMultipleFields(t *testing.T) {
	pk := AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public()

	serializer := postcard.NewSerializer()

	err := serializer.SerializeU32(1)
	assert.NoError(t, err)
	err = pk.MarshalPostcard(serializer)
	assert.NoError(t, err)
	err = serializer.SerializeStr("test-data")
	assert.NoError(t, err)
	data := serializer.Bytes()
	assert.NotEmpty(t, data)

	deserializer := postcard.NewDeserializer(data)

	id, err := deserializer.DeserializeU32()
	assert.NoError(t, err)
	assert.Equal(t, uint32(1), id)

	decoded := &PublicKey{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(pk.Bytes, decoded.Bytes))

	str, err := deserializer.DeserializeStr()
	assert.NoError(t, err)
	assert.Equal(t, "test-data", str)

	err = deserializer.AssertEnd()
	assert.NoError(t, err)
}

func TestPublicKeyDeserializePostcardInvalidData(t *testing.T) {
	pk := &PublicKey{}
	deserializer := postcard.NewDeserializer([]byte{})
	err := pk.UnmarshalPostcard(deserializer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize public key variant")

	deserializer = postcard.NewDeserializer([]byte{0x06})
	err = pk.UnmarshalPostcard(deserializer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown public key variant")

	deserializer = postcard.NewDeserializer([]byte{0x01, 0x02, 0x03, 0x04, 0x05})
	err = pk.UnmarshalPostcard(deserializer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize public key Bytes")
}

func TestPublicKeyDeserializePostcardTrailingBytes(t *testing.T) {
	pk := AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public()

	serializer := postcard.NewSerializer()
	err := pk.MarshalPostcard(serializer)
	assert.NoError(t, err)

	data := serializer.Bytes()
	data = append(data, []byte{0xFF, 0xFE, 0xFD}...)

	deserializer := postcard.NewDeserializer(data)
	decoded := &PublicKey{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	err = deserializer.AssertEnd()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trailing bytes")

	deserializer = postcard.NewDeserializer(data)
	decoded = &PublicKey{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(pk.Bytes, decoded.Bytes))
	assert.Equal(t, 3, deserializer.Remaining())
}
