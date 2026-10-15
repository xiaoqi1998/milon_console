package test

import (
	"encoding/binary"
	"encoding/json"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"math"
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

func TestAddressReservedSpaceWithoutValidSeedChecksum(t *testing.T) {
	address := crypto.NewAddressFromAppSeed(crypto.ContractAppID, 1)
	address.Bytes[19] ^= 1

	_, _, ok := address.AppSeed()
	assert.False(t, ok)
	assert.True(t, address.IsAppReserved())

	restored, err := crypto.NewAddressFromBytes(address.Bytes[:])
	assert.NoError(t, err)
	assert.Equal(t, *address, *restored)

	fromRelaxed, err := crypto.NewAddressFromRelaxed(address.ToBase58())
	assert.NoError(t, err)
	assert.Equal(t, *address, *fromRelaxed)
}

func TestAddressSeedReservationChecksEntireChecksum(t *testing.T) {
	for appID := 0; appID <= 255; appID++ {
		for _, seed := range []uint64{0, 1, math.MaxUint64} {
			address := crypto.NewAddressFromAppSeed(uint8(appID), seed)

			gotAppID, gotSeed, ok := address.AppSeed()
			assert.True(t, ok)
			assert.Equal(t, uint8(appID), gotAppID)
			assert.Equal(t, seed, gotSeed)
			assert.True(t, address.IsAppReserved())

			for index := 9; index < crypto.AddressRawLen; index++ {
				corrupted := *address
				corrupted.Bytes[index] ^= 1

				_, _, ok := corrupted.AppSeed()
				assert.False(t, ok)
				assert.Equal(t, uint8(appID) == crypto.ContractAppID, corrupted.IsAppReserved())
			}
		}
	}

	var zero crypto.Address
	assert.False(t, zero.IsAppReserved())
}

func TestAddressPublicKeyRetry(t *testing.T) {
	testCases := []struct {
		name   string
		domain []byte
		build  func(seed []byte) *crypto.PublicKey
	}{
		{
			name:   "Ed25519",
			domain: crypto.PkAddressDomainBytes,
			build: func(seed []byte) *crypto.PublicKey {
				return &crypto.PublicKey{Variant: crypto.PublicKeyTypeEd25519, Bytes: seed}
			},
		},
		{
			name:   "KeylessDerived",
			domain: crypto.KeylessAddressDomainBytes,
			build: func(seed []byte) *crypto.PublicKey {
				var arr [32]byte
				copy(arr[:], seed)
				return crypto.NewKeylessDerivedPublicKey(arr)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seed := findReservedFirstCandidate(t, tc.domain)
			pk := tc.build(seed)

			address, err := crypto.NewAddressFromPublicKey(pk)
			assert.NoError(t, err)
			assert.False(t, address.IsAppReserved())

			again, err := crypto.NewAddressFromPublicKey(pk)
			assert.NoError(t, err)
			assert.Equal(t, *address, *again)

			var counterBytes [8]byte
			binary.BigEndian.PutUint64(counterBytes[:], 1)
			hash := crypto.Hash32(crypto.PkAddressRetryDomainBytes, tc.domain, seed, counterBytes[:])

			var expected [crypto.AddressRawLen]byte
			copy(expected[:], hash[:crypto.AddressRawLen])
			assert.Equal(t, crypto.Address{Bytes: expected}, *address)
		})
	}
}

func TestAddressFromPublicKeyMatchesSingleHashWhenUnreserved(t *testing.T) {
	var seed [32]byte
	for i := range seed {
		seed[i] = 7
	}
	pk := &crypto.PublicKey{Variant: crypto.PublicKeyTypeEd25519, Bytes: seed[:]}

	hash := crypto.Hash32(crypto.PkAddressDomainBytes, seed[:])
	var direct crypto.Address
	copy(direct.Bytes[:], hash[:crypto.AddressRawLen])

	if direct.IsAppReserved() {
		t.Skip("fixture hits the reserved space")
	}

	address, err := crypto.NewAddressFromPublicKey(pk)
	assert.NoError(t, err)
	assert.Equal(t, direct, *address)
}

func TestAddressAllPublicKeyTypesDeriveOnlyOrdinary(t *testing.T) {
	classical := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	secpPk, err := classical.Secp256k1Public()
	assert.NoError(t, err)

	_, fnDsaPk, err := crypto.NewFnDsa512SecretKey()
	assert.NoError(t, err)

	var keylessSeed [32]byte
	for i := range keylessSeed {
		keylessSeed[i] = 7
	}

	publicKeys := []*crypto.PublicKey{
		secpPk,
		classical.Ed25519Public(),
		classical.BLS12381Public(),
		fnDsaPk,
		crypto.NewKeylessDerivedPublicKey(keylessSeed),
	}

	for _, pk := range publicKeys {
		address, err := crypto.NewAddressFromPublicKey(pk)
		assert.NoError(t, err)
		assert.False(t, address.IsAppReserved())

		again, err := crypto.NewAddressFromPublicKey(pk)
		assert.NoError(t, err)
		assert.Equal(t, *address, *again)
	}
}

func TestAddressKeylessDerivedDisjointFromEd25519(t *testing.T) {
	var seedBytes [32]byte
	for i := range seedBytes {
		seedBytes[i] = 7
	}

	keyless, err := crypto.NewAddressFromPublicKey(crypto.NewKeylessDerivedPublicKey(seedBytes))
	assert.NoError(t, err)

	ed25519Addr, err := crypto.NewAddressFromPublicKey(&crypto.PublicKey{Variant: crypto.PublicKeyTypeEd25519, Bytes: seedBytes[:]})
	assert.NoError(t, err)

	assert.NotEqual(t, *ed25519Addr, *keyless)

	keylessAgain, err := crypto.NewAddressFromPublicKey(crypto.NewKeylessDerivedPublicKey(seedBytes))
	assert.NoError(t, err)
	assert.Equal(t, *keyless, *keylessAgain)
}

func TestAddressFromAppSeedDeterministicAndScoped(t *testing.T) {
	a1 := crypto.NewAddressFromAppSeed(255, 1)
	a2 := crypto.NewAddressFromAppSeed(255, 1)
	a3 := crypto.NewAddressFromAppSeed(255, 2)
	a4 := crypto.NewAddressFromAppSeed(2, 1)

	assert.Equal(t, *a1, *a2)
	assert.NotEqual(t, *a1, *a3)
	assert.NotEqual(t, *a1, *a4)

	appID, seed, ok := a1.AppSeed()
	assert.True(t, ok)
	assert.Equal(t, uint8(255), appID)
	assert.Equal(t, uint64(1), seed)

	appID, seed, ok = a4.AppSeed()
	assert.True(t, ok)
	assert.Equal(t, uint8(2), appID)
	assert.Equal(t, uint64(1), seed)

	edAddress, err := crypto.NewAddressFromPublicKey(&crypto.PublicKey{Variant: crypto.PublicKeyTypeEd25519, Bytes: make([]byte, crypto.PublicKeyEd25519Size)})
	assert.NoError(t, err)
	_, _, ok = edAddress.AppSeed()
	assert.False(t, ok)
}

// findReservedFirstCandidate returns a 32-byte fixture whose first address
// candidate (single domain-separated hash) hits the reserved space.
func findReservedFirstCandidate(t *testing.T, domain []byte) []byte {
	t.Helper()

	for n := uint64(0); n < 100_000; n++ {
		var seed [32]byte
		binary.BigEndian.PutUint64(seed[:8], n)
		if crypto.Hash32(domain, seed[:])[0] == crypto.ContractAppID {
			return seed[:]
		}
	}

	t.Fatal("fixture contains a reserved first candidate")
	return nil
}
