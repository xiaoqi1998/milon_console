package test

import (
	"encoding/json"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicKeyVariant(t *testing.T) {
	pk1, err := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).Secp256k1Public()
	assert.NoError(t, err)
	assert.Equal(t, crypto.PublicKeyTypeSecp256k1, pk1.Variant)
	assert.True(t, pk1.IsSecp256k1())
	assert.False(t, pk1.IsEd25519())
	assert.False(t, pk1.IsBLS12381())
	assert.False(t, pk1.IsFnDsa512())

	pk2 := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Ed25519Public()
	assert.Equal(t, crypto.PublicKeyTypeEd25519, pk2.Variant)
	assert.True(t, pk2.IsEd25519())
	assert.False(t, pk2.IsSecp256k1())
	assert.False(t, pk2.IsBLS12381())
	assert.False(t, pk2.IsFnDsa512())

	pk3 := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).BLS12381Public()
	assert.Equal(t, crypto.PublicKeyTypeBLS12381, pk3.Variant)
	assert.True(t, pk3.IsBLS12381())
	assert.False(t, pk3.IsSecp256k1())
	assert.False(t, pk3.IsEd25519())
	assert.False(t, pk3.IsFnDsa512())

	_, pk4, err := crypto.NewFnDsa512SecretKey()
	assert.NoError(t, err)
	assert.Equal(t, crypto.PublicKeyTypeFnDsa512, pk4.Variant)
	assert.False(t, pk4.IsBLS12381())
	assert.False(t, pk4.IsSecp256k1())
	assert.False(t, pk4.IsEd25519())
	assert.True(t, pk4.IsFnDsa512())
}

func TestPublicKeyRoundTrip(t *testing.T) {
	testCases := []struct {
		name     string
		createPk func() *crypto.PublicKey
	}{
		{
			name: "Secp256k1",
			createPk: func() *crypto.PublicKey {
				pk, err := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).Secp256k1Public()
				assert.NoError(t, err)
				return pk
			},
		},
		{
			name: "Ed25519",
			createPk: func() *crypto.PublicKey {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Ed25519Public()
			},
		},
		{
			name: "BLS12381",
			createPk: func() *crypto.PublicKey {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).BLS12381Public()
			},
		},
		{
			name: "FnDsa512",
			createPk: func() *crypto.PublicKey {
				_, pk, err := crypto.NewFnDsa512SecretKey()
				assert.NoError(t, err)
				return pk
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pk1 := tc.createPk()

			pk2, err := crypto.NewPublicKeyFromBytes(pk1.AsBytes())
			assert.NoError(t, err)
			assert.Equal(t, pk1.Variant, pk2.Variant)
			assert.Equal(t, pk1.Bytes, pk2.Bytes)

			pk3, err := crypto.NewPublicKeyFromStringRelaxed(pk1.ToHex())
			assert.NoError(t, err)
			assert.Equal(t, pk1.Variant, pk3.Variant)
			assert.Equal(t, pk1.Bytes, pk3.Bytes)

			pk4, err := crypto.NewPublicKeyFromStringRelaxed("0x" + pk1.ToHex())
			assert.NoError(t, err)
			assert.Equal(t, pk1.Variant, pk4.Variant)
			assert.Equal(t, pk1.Bytes, pk4.Bytes)

			pk5, err := crypto.NewPublicKeyFromStringRelaxed(pk1.ToBase58())
			assert.NoError(t, err)
			assert.Equal(t, pk1.Variant, pk5.Variant)
			assert.Equal(t, pk1.Bytes, pk5.Bytes)
		})
	}
}

func TestPublicKeyFromBytesWrongLen(t *testing.T) {
	_, err := crypto.NewPublicKeyFromBytes([]byte{})
	assert.Error(t, err)

	_, err = crypto.NewPublicKeyFromBytes(make([]byte, 31))
	assert.Error(t, err)

	_, err = crypto.NewPublicKeyFromBytes(make([]byte, 47))
	assert.Error(t, err)
}

func TestPublicKeyFromStringRelaxedInvalidFormat(t *testing.T) {
	_, err := crypto.NewPublicKeyFromStringRelaxed("not a valid hex string")
	assert.Error(t, err)

	_, err = crypto.NewPublicKeyFromStringRelaxed("!!!invalid base58!!!")
	assert.Error(t, err)

	_, err = crypto.NewPublicKeyFromStringRelaxed("[1,2,3]")
	assert.Error(t, err)
}

func TestPublicKeyWrongVariantConversions(t *testing.T) {
	pk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Ed25519Public()

	_, err := pk.ToEd25519()
	assert.NoError(t, err)

	_, err = pk.ToSecp256k1()
	assert.Error(t, err)

	_, err = pk.ToBLS12381()
	assert.Error(t, err)
}

func TestPublicKeyToNative(t *testing.T) {
	// Secp256k1
	pk1, err := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).Secp256k1Public()
	assert.NoError(t, err)

	native1, err := pk1.ToSecp256k1()
	assert.NoError(t, err)

	pk1Decoded := crypto.PublicKey{}
	err = pk1Decoded.FromSecp256k1Native(native1)
	assert.NoError(t, err)
	assert.Equal(t, pk1.Bytes, pk1Decoded.AsBytes())

	// Ed25519
	pk2 := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).Ed25519Public()
	native2, err := pk2.ToEd25519()
	assert.NoError(t, err)

	pk2Decoded := crypto.PublicKey{}
	err = pk2Decoded.FromEd25519Native(native2)
	assert.NoError(t, err)
	assert.Equal(t, pk2.Bytes, pk2Decoded.AsBytes())

	// BLS
	pk3 := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).BLS12381Public()
	native3, err := pk3.ToBLS12381()
	assert.NoError(t, err)

	pk3Decoded := crypto.PublicKey{}
	err = pk3Decoded.FromBLS12381Native(native3)
	assert.NoError(t, err)
	assert.Equal(t, pk3.Bytes, pk3Decoded.AsBytes())
}

func TestPublicKeyJSONRoundTrip(t *testing.T) {
	testCases := []struct {
		name     string
		createPk func() *crypto.PublicKey
	}{
		{
			name: "Secp256k1",
			createPk: func() *crypto.PublicKey {
				pk, err := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Secp256k1Public()
				assert.NoError(t, err)
				return pk
			},
		},
		{
			name: "Ed25519",
			createPk: func() *crypto.PublicKey {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Ed25519Public()
			},
		},
		{
			name: "BLS12381",
			createPk: func() *crypto.PublicKey {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).BLS12381Public()
			},
		},
		{
			name: "FnDsa512",
			createPk: func() *crypto.PublicKey {
				_, pk, err := crypto.NewFnDsa512SecretKey()
				assert.NoError(t, err)
				return pk
			},
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
			assert.Equal(t, pk.ToBase58(), b58Str)

			decoded := &crypto.PublicKey{}
			err = json.Unmarshal(jsonData, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, pk.Bytes, decoded.Bytes)
			assert.Equal(t, pk.Variant, decoded.Variant)
		})
	}
}

func TestPublicKeyPostcardRoundTrip(t *testing.T) {
	testCases := []struct {
		name        string
		createPk    func() *crypto.PublicKey
		expectedLen int
	}{
		{
			name: "Secp256k1-33bytes",
			createPk: func() *crypto.PublicKey {
				pk, err := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Secp256k1Public()
				assert.NoError(t, err)
				return pk
			},
			expectedLen: crypto.PublicKeySecp256k1Size,
		},
		{
			name: "Ed25519-32bytes",
			createPk: func() *crypto.PublicKey {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Ed25519Public()
			},
			expectedLen: crypto.PublicKeyEd25519Size,
		},
		{
			name: "BLS12381-48bytes",
			createPk: func() *crypto.PublicKey {
				return crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).BLS12381Public()
			},
			expectedLen: crypto.PublicKeyBLS12381Size,
		},
		{
			name: "FnDsa512-897bytes",
			createPk: func() *crypto.PublicKey {
				_, pk, err := crypto.NewFnDsa512SecretKey()
				assert.NoError(t, err)
				return pk
			},
			expectedLen: crypto.PublicKeyFnDsa512Size,
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
			decoded := &crypto.PublicKey{}
			err = decoded.UnmarshalPostcard(deserializer)
			assert.NoError(t, err)
			assert.Equal(t, pk.Bytes, decoded.Bytes)
			assert.Equal(t, pk.Variant, decoded.Variant)
			assert.Equal(t, tc.expectedLen, len(decoded.Bytes))

			err = deserializer.AssertEnd()
			assert.NoError(t, err)
		})
	}
}

func TestPublicKeyDeserializePostcardInvalidData(t *testing.T) {
	pk := &crypto.PublicKey{}
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
