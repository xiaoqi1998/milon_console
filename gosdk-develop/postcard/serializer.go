package postcard

import (
	"fmt"
	"math"
	"math/big"
	"unicode/utf8"
)

var maxU128, _ = new(big.Int).SetString("340282366920938463463374607431768211455", 10)

var varintMask = big.NewInt(0x7f)
var varintThreshold = big.NewInt(0x80)

type Serializer struct {
	bytes []byte
}

func NewSerializer() *Serializer {
	return &Serializer{bytes: make([]byte, 0, 64)}
}

func (s *Serializer) Bytes() []byte {
	return append([]byte(nil), s.bytes...)
}

func (s *Serializer) Serialize(value Marshaler) error {
	return value.MarshalPostcard(s)
}

func (s *Serializer) SerializeStr(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("expected valid UTF-8 string")
	}
	return s.SerializeBytes([]byte(value))
}

func (s *Serializer) SerializeBytes(value []byte) error {
	if err := s.SerializeU32(uint32(len(value))); err != nil {
		return err
	}
	s.SerializeFixedBytes(value)
	return nil
}

func (s *Serializer) SerializeFixedBytes(value []byte) {
	s.bytes = append(s.bytes, value...)
}

func (s *Serializer) SerializeBool(value bool) error {
	if value {
		return s.SerializeU8(1)
	}
	return s.SerializeU8(0)
}

func (s *Serializer) SerializeU8(value uint8) error {
	s.bytes = append(s.bytes, value)
	return nil
}

func (s *Serializer) SerializeU16(value uint16) error {
	return s.serializeVarUint64(uint64(value))
}

func (s *Serializer) SerializeU32(value uint32) error {
	return s.serializeVarUint64(uint64(value)) // uses variable-length encoding
}

func (s *Serializer) SerializeU64(value uint64) error {
	return s.serializeVarUint64(value)
}

func (s *Serializer) SerializeU128(value *big.Int) error {
	if value == nil {
		return fmt.Errorf("u128 out of range")
	}
	if value.Sign() < 0 || value.Cmp(maxU128) > 0 {
		return fmt.Errorf("u128 out of range")
	}
	return s.serializeVarUintBig(value)
}

func (s *Serializer) SerializeI8(value int8) error {
	return s.SerializeU8(uint8(value))
}

func (s *Serializer) SerializeI16(value int16) error {
	return s.SerializeU16(uint16(value))
}

func (s *Serializer) SerializeI32(value int32) error {
	return s.SerializeU32(uint32(value))
}

func (s *Serializer) SerializeI64(value int64) error {
	return s.SerializeU64(uint64(value))
}

func (s *Serializer) SerializeEnumVariant(index uint32) error {
	return s.SerializeU32(index)
}

// serializeVarUint64 serializes a uint64 value using varint (variable-length integer) encoding
//
// Varint encoding rules:
//   - The highest bit (bit 7) of each byte is the continuation flag: 1 = more bytes follow, 0 = last byte
//   - The low 7 bits (bit 0-6) are data bits, arranged in little-endian order
//   - Smaller values use fewer bytes (space optimized)
//
// Encoding examples:
//
//	0         → [0x00]              (1 byte)
//	127       → [0x7F]              (1 byte)
//	128       → [0x80, 0x01]        (2 bytes)
//	2581      → [0x95, 0x14]        (2 bytes)
//	16384     → [0x80, 0x80, 0x01]  (3 bytes)
//	4294967295 → [0xFF, 0xFF, 0xFF, 0xFF, 0x0F] (5 bytes, uint32 max)
//
// Byte count ranges:
//   - uint8:  1-2 bytes
//   - uint16: 1-3 bytes
//   - uint32: 1-5 bytes
//   - uint64: 1-10 bytes
func (s *Serializer) serializeVarUint64(value uint64) error {
	for value >= 0x80 {
		s.bytes = append(s.bytes, byte(value&0x7f)|0x80) // set high bit to 1, indicating more bytes follow
		value >>= 7
	}
	s.bytes = append(s.bytes, byte(value)) // last byte, high bit is 0
	return nil
}

func (s *Serializer) serializeVarUintBig(value *big.Int) error {
	remaining := new(big.Int).Set(value)
	part := new(big.Int)
	for remaining.Cmp(varintThreshold) >= 0 {
		part.And(remaining, varintMask)
		s.bytes = append(s.bytes, byte(part.Uint64())|0x80)
		remaining.Rsh(remaining, 7)
	}
	if !remaining.IsUint64() || remaining.Uint64() > math.MaxUint8 {
		return fmt.Errorf("u128 out of range")
	}
	s.bytes = append(s.bytes, byte(remaining.Uint64()))
	return nil
}
