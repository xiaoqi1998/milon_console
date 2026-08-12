package test

import (
	"encoding/json"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddressFromPublicKeyRoundTrip(t *testing.T) {
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
				return crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).Ed25519Public()
			},
		},
		{
			name: "BLS12381",
			createPk: func() *crypto.PublicKey {
				return crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()).BLS12381Public()
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
			addr1, err := crypto.NewAddressFromPublicKey(tc.createPk())
			assert.NoError(t, err)
			assert.Equal(t, crypto.AddressRawLen, len(addr1.Bytes))

			addr2, err := crypto.NewAddressFromBytes(addr1.Bytes[:])
			assert.NoError(t, err)
			assert.Equal(t, addr1.ToBase58(), addr2.ToBase58())

			addr3, err := crypto.NewAddressFromRelaxed(addr1.ToHex())
			assert.NoError(t, err)
			assert.Equal(t, addr1.ToBase58(), addr3.ToBase58())

			addr4, err := crypto.NewAddressFromRelaxed("0x" + addr1.ToHex())
			assert.NoError(t, err)
			assert.Equal(t, addr1.ToBase58(), addr4.ToBase58())

			addr5, err := crypto.NewAddressFromRelaxed(addr1.ToBase58())
			assert.NoError(t, err)
			assert.Equal(t, addr1.ToBase58(), addr5.ToBase58())
		})
	}
}

func TestAddressJSONRoundTrip(t *testing.T) {
	pk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Ed25519Public()
	addr, err := crypto.NewAddressFromPublicKey(pk)
	assert.NoError(t, err)

	jsonData, err := addr.MarshalJSON()
	assert.NoError(t, err)

	decoded := &crypto.Address{}
	err = decoded.UnmarshalJSON(jsonData)
	assert.NoError(t, err)
	assert.Equal(t, *addr, *decoded)

	decoded = &crypto.Address{}
	err = json.Unmarshal(jsonData, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, *addr, *decoded)
}

func TestAddressPostcardRoundTrip(t *testing.T) {
	pk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey()).Ed25519Public()
	addr, err := crypto.NewAddressFromPublicKey(pk)
	assert.NoError(t, err)

	serializer := postcard.NewSerializer()
	err = addr.MarshalPostcard(serializer)
	assert.NoError(t, err)

	data := serializer.Bytes()
	assert.Equal(t, crypto.AddressRawLen, len(data))

	deserializer := postcard.NewDeserializer(data)
	decoded := &crypto.Address{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	assert.Equal(t, *addr, *decoded)

	err = deserializer.AssertEnd()
	assert.NoError(t, err)
}

func TestAddressFromBytesError(t *testing.T) {
	_, err := crypto.NewAddressFromBytes(make([]byte, crypto.AddressRawLen-1))
	assert.Error(t, err)

	_, err = crypto.NewAddressFromBytes(make([]byte, crypto.AddressRawLen+1))
	assert.Error(t, err)
}

func TestAddressFromRelaxedError(t *testing.T) {
	var nilAddr *crypto.Address
	_, err := crypto.NewAddressFromRelaxed(nilAddr)
	assert.Error(t, err)

	_, err = crypto.NewAddressFromRelaxed(12345)
	assert.Error(t, err)

	_, err = crypto.NewAddressFromRelaxed("0xzz")
	assert.Error(t, err)

	_, err = crypto.NewAddressFromRelaxed("!!invalid base58!!")
	assert.Error(t, err)
}
