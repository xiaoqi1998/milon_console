package crypto

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"math"
	"strings"

	"github.com/btcsuite/btcutil/base58"
)

// AddressRawLen raw byte length of a address
const AddressRawLen = 20

// AddressHexLen hexadecimal literal length of a address
const AddressHexLen = AddressRawLen * 2

// ContractAppID is the App ID reserved for future third-party contracts; the
// whole address space whose first byte is 0xFF is reserved.
const ContractAppID uint8 = 0xFF

// NewAddressFromPublicKey derives an ordinary account address from a public key
// and guarantees IsAppReserved() is false.
//
// KeylessDerived keys use the dedicated KeylessAddressDomainBytes domain. The
// first candidate is the domain-separated Blake3 hash truncated to AddressRawLen
// bytes; when a candidate hits the reserved space it is re-hashed with
// PkAddressRetryDomainBytes, the original domain, the public key bytes and a
// big-endian u64 counter starting from 1. Retrying never changes the private
// key, mnemonic or public key.
func NewAddressFromPublicKey(pk *PublicKey) (*Address, error) {
	domain := PkAddressDomainBytes
	if pk.Variant == PublicKeyTypeKeylessDerived {
		domain = KeylessAddressDomainBytes
	}
	keyBytes := pk.Bytes

	address := firstAccountCandidate(func(counter uint64) *Address {
		var hash [32]byte
		if counter == 0 {
			hash = Hash32(domain, keyBytes)
		} else {
			var counterBytes [8]byte
			binary.BigEndian.PutUint64(counterBytes[:], counter)
			hash = Hash32(PkAddressRetryDomainBytes, domain, keyBytes, counterBytes[:])
		}

		var digest [AddressRawLen]byte
		copy(digest[:], hash[:AddressRawLen])
		return &Address{Bytes: digest}
	})

	return address, nil
}

// firstAccountCandidate returns the first candidate usable for an ordinary
// account and never returns a reserved address; it panics when the counter is
// exhausted.
func firstAccountCandidate(candidate func(counter uint64) *Address) *Address {
	for counter := uint64(0); ; counter++ {
		address := candidate(counter)
		if !address.IsAppReserved() {
			return address
		}
		if counter == math.MaxUint64 {
			panic("public key address retry counter exhausted")
		}
	}
}

// NewAddressFromBytes parses an address from a 20-byte slice
func NewAddressFromBytes(bytes []byte) (*Address, error) {
	if len(bytes) != AddressRawLen {
		return nil, fmt.Errorf("invalid address length: expected %d, got %d", AddressRawLen, len(bytes))
	}

	var digest [AddressRawLen]byte
	copy(digest[:], bytes)
	return &Address{Bytes: digest}, nil
}

// NewAddressFromAppSeed derives an App address from (appID, seed). This is a
// runtime-internal derivation without a corresponding private key, not a client
// API. Layout: [appID | seed u64 BE | Blake3(SeedAddressDomainBytes || appID || seed)[0..11]].
func NewAddressFromAppSeed(appID uint8, seed uint64) *Address {
	appIDBytes := [1]byte{appID}
	var seedBytes [8]byte
	binary.BigEndian.PutUint64(seedBytes[:], seed)

	hash := Hash32(SeedAddressDomainBytes, appIDBytes[:], seedBytes[:])

	var digest [AddressRawLen]byte
	digest[0] = appID
	copy(digest[1:9], seedBytes[:])
	copy(digest[9:], hash[:11])
	return &Address{Bytes: digest}
}

// NewAddressFromAppSeed32 derives a truncated-hash address from (appID, seed)
// without embedding a recoverable seed; this derivation is not guaranteed to
// be app reserved.
func NewAddressFromAppSeed32(appID uint8, seed [32]byte) *Address {
	appIDBytes := [1]byte{appID}
	hash := Hash32(SeedAddress32DomainBytes, appIDBytes[:], seed[:])

	var digest [AddressRawLen]byte
	copy(digest[:], hash[:AddressRawLen])
	return &Address{Bytes: digest}
}

// NewAddressFromRelaxed parses an address from an *Address, an Address,
// a hex string (with or without 0x prefix), or a base58 string. Hex-shaped
// inputs try hex first and fall back to base58; other inputs try base58 first
// and fall back to hex.
func NewAddressFromRelaxed(addressRelaxed any) (*Address, error) {
	switch v := addressRelaxed.(type) {
	case *Address:
		if v == nil {
			return nil, fmt.Errorf("nil address")
		}
		return v, nil
	case Address:
		return &v, nil
	case string:
		str := strings.TrimSpace(v)

		hexBody := str
		if len(str) >= 2 && str[0] == '0' && (str[1] == 'x' || str[1] == 'X') {
			hexBody = str[2:]
		}

		tryHexFirst := len(hexBody) == AddressHexLen && isHexString(hexBody)
		if tryHexFirst {
			if addr, err := newAddressFromHex(str); err == nil {
				return addr, nil
			}
			return newAddressFromBase58(str)
		}

		if addr, err := newAddressFromBase58(str); err == nil {
			return addr, nil
		}
		return newAddressFromHex(str)
	default:
		return nil, fmt.Errorf("unsupported address input type %T", addressRelaxed)
	}
}

// isHexString reports whether s consists only of hexadecimal digits
func isHexString(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func newAddressFromHex(s string) (*Address, error) {
	s = strings.TrimSpace(s)

	hexStr := s
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		hexStr = s[2:]
	}

	if len(hexStr) != AddressHexLen {
		return nil, fmt.Errorf("invalid hex length: expected %d, got %d", AddressHexLen, len(hexStr))
	}

	buf, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}

	return NewAddressFromBytes(buf)
}

func newAddressFromBase58(s string) (*Address, error) {
	buf := base58.Decode(s)

	if len(buf) != AddressRawLen {
		return nil, fmt.Errorf("invalid base58 decoded length: expected %d, got %d", AddressRawLen, len(buf))
	}

	return NewAddressFromBytes(buf)
}

type Address struct {
	Bytes [AddressRawLen]byte
}

func (a *Address) AsBytes() [AddressRawLen]byte {
	return a.Bytes
}

func (a *Address) ToHex() string {
	return hex.EncodeToString(a.Bytes[:])
}

func (a *Address) ToBase58() string {
	return base58.Encode(a.Bytes[:])
}

// IsAppReserved reports whether the address is forbidden for ordinary account
// registration and external authentication: every address whose first byte is
// 0xFF, plus any valid u64 seed address of any App, belongs to the reserved
// space. This check does not grant App authorization and does not restrict
// address parsing, receiving or balance queries.
func (a Address) IsAppReserved() bool {
	if a.Bytes[0] == ContractAppID {
		return true
	}
	_, _, ok := a.AppSeed()
	return ok
}

// AppSeed re-computes the u64 seed layout and returns (appID, seed, true) only
// when the whole checksum matches. Public-key derived addresses always return
// false; the 32-byte seed derivation does not encode this layout.
func (a Address) AppSeed() (uint8, uint64, bool) {
	appID := a.Bytes[0]
	seed := binary.BigEndian.Uint64(a.Bytes[1:9])

	if a == *NewAddressFromAppSeed(appID, seed) {
		return appID, seed, true
	}
	return 0, 0, false
}

func (a *Address) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.ToBase58())
}

func (a *Address) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	addr, err := NewAddressFromRelaxed(s)
	if err != nil {
		return fmt.Errorf("failed to parse address: %w", err)
	}

	*a = *addr
	return nil
}

// String implements the Stringer interface, returns base58 format
func (a Address) String() string {
	return a.ToBase58()
}

func (a *Address) MarshalPostcard(serializer *postcard.Serializer) error {
	serializer.SerializeFixedBytes(a.Bytes[:])
	return nil
}

func (a *Address) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	buf, err := deserializer.DeserializeFixedBytes(AddressRawLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize address bytes: %w", err)
	}

	addr, err := NewAddressFromBytes(buf)
	if err != nil {
		return fmt.Errorf("failed to create address from bytes: %w", err)
	}

	*a = *addr
	return nil
}
