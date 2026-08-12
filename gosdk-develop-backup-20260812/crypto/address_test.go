package crypto

import (
	"encoding/json"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddressFromSecp256k1PublicKeyRoundTrip(t *testing.T) {
	sk := AsClassicalSecretKey(NewClassicalSecretKey())

	pk, err := sk.Secp256k1Public()
	assert.NoError(t, err)

	addr1, err := NewAddressFromPublicKey(pk)
	assert.NoError(t, err)
	assert.Equal(t, AddressRawLen, len(addr1.Bytes))

	addr2, err := NewAddressFromBytes(addr1.Bytes[:])
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr2.ToBase58())

	addr3, err := NewAddressFromStringRelaxed(addr1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr3.ToBase58())

	addr4, err := NewAddressFromStringRelaxed("0x" + addr1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr4.ToBase58())

	addr5, err := NewAddressFromStringRelaxed(addr1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr5.ToBase58())
}

func TestAddressFromEd25519PublicKeyRoundTrip(t *testing.T) {
	sk := AsClassicalSecretKey(NewClassicalSecretKey())

	addr1, err := NewAddressFromPublicKey(sk.Ed25519Public())
	assert.NoError(t, err)
	assert.Equal(t, AddressRawLen, len(addr1.Bytes))

	addr2, err := NewAddressFromBytes(addr1.Bytes[:])
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr2.ToBase58())

	addr3, err := NewAddressFromStringRelaxed(addr1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr3.ToBase58())

	addr4, err := NewAddressFromStringRelaxed("0x" + addr1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr4.ToBase58())

	addr5, err := NewAddressFromStringRelaxed(addr1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr5.ToBase58())
}

func TestAddressFromBLS12381PublicKeyRoundTrip(t *testing.T) {
	sk := AsClassicalSecretKey(NewClassicalSecretKey())

	addr1, err := NewAddressFromPublicKey(sk.BLS12381Public())
	assert.NoError(t, err)
	assert.Equal(t, AddressRawLen, len(addr1.Bytes))

	addr2, err := NewAddressFromBytes(addr1.Bytes[:])
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr2.ToBase58())

	addr3, err := NewAddressFromStringRelaxed(addr1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr3.ToBase58())

	addr4, err := NewAddressFromStringRelaxed("0x" + addr1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr4.ToBase58())

	addr5, err := NewAddressFromStringRelaxed(addr1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr5.ToBase58())
}

func TestAddressFromFnDsa512PublicKeyRoundTrip(t *testing.T) {
	_, pk, err := NewFnDsa512SecretKey()
	assert.NoError(t, err)

	addr1, err := NewAddressFromPublicKey(pk)
	assert.NoError(t, err)
	assert.Equal(t, AddressRawLen, len(addr1.Bytes))

	addr2, err := NewAddressFromBytes(addr1.Bytes[:])
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr2.ToBase58())

	addr3, err := NewAddressFromStringRelaxed(addr1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr3.ToBase58())

	addr4, err := NewAddressFromStringRelaxed("0x" + addr1.ToHex())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr4.ToBase58())

	addr5, err := NewAddressFromStringRelaxed(addr1.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, addr1.ToBase58(), addr5.ToBase58())
}

func TestAddressJSONRoundTrip(t *testing.T) {
	pk := AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public()
	addr, err := NewAddressFromPublicKey(pk)
	assert.NoError(t, err)

	jsonData, err := addr.MarshalJSON()
	assert.NoError(t, err)

	decoded := &Address{}
	err = decoded.UnmarshalJSON(jsonData)
	assert.NoError(t, err)
	assert.Equal(t, *addr, *decoded)

	decoded = &Address{}
	err = json.Unmarshal(jsonData, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, *addr, *decoded)
}

func TestAddressPostcardRoundTrip(t *testing.T) {
	pk := AsClassicalSecretKey(NewPureClassicalSecretKey()).Ed25519Public()
	addr, err := NewAddressFromPublicKey(pk)
	assert.NoError(t, err)

	serializer := postcard.NewSerializer()
	err = addr.MarshalPostcard(serializer)
	assert.NoError(t, err)

	data := serializer.Bytes()
	assert.Equal(t, AddressRawLen, len(data))

	deserializer := postcard.NewDeserializer(data)
	decoded := &Address{}
	err = decoded.UnmarshalPostcard(deserializer)
	assert.NoError(t, err)
	assert.Equal(t, *addr, *decoded)

	err = deserializer.AssertEnd()
	assert.NoError(t, err)
}
