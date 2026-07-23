package crypto

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"strings"

	"github.com/btcsuite/btcutil/base58"
)

// AddressRawLen raw byte length of a address
const AddressRawLen = 20

// AddressHexLen hexadecimal literal length of a address
const AddressHexLen = AddressRawLen * 2

// NewAddressFromPublicKey derives a address from a public key
func NewAddressFromPublicKey(pk *PublicKey) (*Address, error) {
	// 计算公钥的 BLAKE3 哈希（32字节）
	hash := Hash32([]byte(PkAddressDomainContext), pk.Bytes)

	// 取前20字节作为地址
	var digest [AddressRawLen]byte
	copy(digest[:], hash[:AddressRawLen])

	return &Address{Bytes: digest}, nil
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

// NewAddressFromStringRelaxed parses an address from a hex or base58 string
func NewAddressFromStringRelaxed(s string) (*Address, error) {
	s = strings.TrimSpace(s)

	hexBody := s
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		hexBody = s[2:]
	}
	if len(hexBody) == AddressHexLen {
		return newAddressFromHex(s)
	}

	return newAddressFromBase58(s)
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

func (a *Address) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.ToBase58())
}

func (a *Address) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	addr, err := NewAddressFromStringRelaxed(s)
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
