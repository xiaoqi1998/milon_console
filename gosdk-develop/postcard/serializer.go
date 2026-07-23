package postcard

import (
	"fmt"
	"math"
	"math/big"
	"unicode/utf8"
)

var maxU128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

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
	return s.serializeVarUint64(uint64(value)) // 使用可变长度编码！
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
	return s.SerializeU8(uint8(encodeZigZag64(int64(value))))
}

func (s *Serializer) SerializeI16(value int16) error {
	return s.SerializeU16(uint16(encodeZigZag64(int64(value))))
}

func (s *Serializer) SerializeI32(value int32) error {
	return s.SerializeU32(uint32(encodeZigZag64(int64(value))))
}

func (s *Serializer) SerializeI64(value int64) error {
	return s.SerializeU64(encodeZigZag64(value))
}

func (s *Serializer) SerializeEnumVariant(index uint32) error {
	return s.SerializeU32(index)
}

// serializeVarUint64 将 uint64 值序列化为 varint（可变长度整数）编码
//
// Varint 编码规则：
//   - 每个字节的最高位（bit 7）是继续标志：1=还有后续字节，0=最后一个字节
//   - 低 7 位（bit 0-6）是数据位，采用小端序排列
//   - 数值越小，使用的字节数越少（优化空间）
//
// 编码示例：
//
//	0         → [0x00]              (1字节)
//	127       → [0x7F]              (1字节)
//	128       → [0x80, 0x01]        (2字节)
//	2581      → [0x95, 0x14]        (2字节)
//	16384     → [0x80, 0x80, 0x01]  (3字节)
//	4294967295 → [0xFF, 0xFF, 0xFF, 0xFF, 0x0F] (5字节，uint32最大值)
//
// 字节数范围：
//   - uint8:  1-2 字节
//   - uint16: 1-3 字节
//   - uint32: 1-5 字节
//   - uint64: 1-10 字节
func (s *Serializer) serializeVarUint64(value uint64) error {
	for value >= 0x80 {
		s.bytes = append(s.bytes, byte(value&0x7f)|0x80) //高位设为 1，表示还有后续字节
		value >>= 7
	}
	s.bytes = append(s.bytes, byte(value)) //最后一个字节，高位为 0
	return nil
}

func (s *Serializer) serializeVarUintBig(value *big.Int) error {
	remaining := new(big.Int).Set(value)
	mask := big.NewInt(0x7f)
	for remaining.Cmp(big.NewInt(0x80)) >= 0 {
		byteValue := new(big.Int).And(remaining, mask).Uint64()
		s.bytes = append(s.bytes, byte(byteValue)|0x80)
		remaining.Rsh(remaining, 7)
	}
	if !remaining.IsUint64() || remaining.Uint64() > math.MaxUint8 {
		return fmt.Errorf("u128 out of range")
	}
	s.bytes = append(s.bytes, byte(remaining.Uint64()))
	return nil
}

func encodeZigZag64(value int64) uint64 {
	return uint64((value << 1) ^ (value >> 63))
}
