package postcard

import (
	"fmt"
	"math"
	"math/big"
	"unicode/utf8"
)

// TypeResolver resolves a type_tag into the byte range of its value.
// Implemented by provider.IDLTypeResolver; injected per-deserializer so
// multiple clients can decode with their own loaded IDLs concurrently.
type TypeResolver interface {
	DecodeResource(typeTag uint64, bytes []byte) (valueBytes []byte, remaining []byte, err error)
	DecodeEvent(typeTag uint64, bytes []byte) (eventBytes []byte, remaining []byte, err error)
}

type Deserializer struct {
	buffer       []byte
	offset       int
	typeResolver TypeResolver
}

func NewDeserializer(data []byte) *Deserializer {
	return &Deserializer{buffer: data}
}

// SetTypeResolver sets the type_tag resolver used by api.ReadAnySerializeValueWithTypeTag
// and api.DeserializeEventEntry during this deserialization.
func (d *Deserializer) SetTypeResolver(r TypeResolver) { d.typeResolver = r }

// TypeResolver returns the resolver set via SetTypeResolver, or nil.
func (d *Deserializer) TypeResolver() TypeResolver { return d.typeResolver }

func (d *Deserializer) Remaining() int {
	return len(d.buffer) - d.offset
}

func (d *Deserializer) DeserializeStr() (string, error) {
	bytes, err := d.DeserializeBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(bytes) {
		return "", fmt.Errorf("invalid UTF-8 string")
	}
	return string(bytes), nil
}

func (d *Deserializer) DeserializeBytes() ([]byte, error) {
	length, err := d.DeserializeU32()
	if err != nil {
		return nil, err
	}
	if int(length) > d.Remaining() {
		return nil, fmt.Errorf("bytes length %d exceeds remaining buffer %d", length, d.Remaining())
	}
	return d.DeserializeFixedBytes(int(length))
}

func (d *Deserializer) DeserializeFixedBytes(length int) ([]byte, error) {
	return d.read(length)
}

func (d *Deserializer) DeserializeBool() (bool, error) {
	value, err := d.DeserializeU8()
	if err != nil {
		return false, err
	}
	if value != 0 && value != 1 {
		return false, fmt.Errorf("invalid postcard boolean")
	}
	return value == 1, nil
}

func (d *Deserializer) DeserializeU8() (uint8, error) {
	if d.offset >= len(d.buffer) {
		return 0, fmt.Errorf("reached end of postcard buffer")
	}
	b := d.buffer[d.offset]
	d.offset++
	return b, nil
}

func (d *Deserializer) DeserializeU16() (uint16, error) {
	value, err := d.deserializeVarUint64(math.MaxUint16, "u16")
	return uint16(value), err
}

func (d *Deserializer) DeserializeU32() (uint32, error) {
	value, err := d.deserializeVarUint64(math.MaxUint32, "u32")
	return uint32(value), err
}

func (d *Deserializer) DeserializeU64() (uint64, error) {
	return d.deserializeVarUint64(math.MaxUint64, "u64")
}

func (d *Deserializer) DeserializeU128() (*big.Int, error) {
	return d.deserializeVarUintBig(maxU128, "u128")
}

func (d *Deserializer) DeserializeI8() (int8, error) {
	value, err := d.DeserializeU8()
	return int8(value), err
}

func (d *Deserializer) DeserializeI16() (int16, error) {
	value, err := d.DeserializeU16()
	return int16(value), err
}

func (d *Deserializer) DeserializeI32() (int32, error) {
	value, err := d.DeserializeU32()
	return int32(value), err
}

func (d *Deserializer) DeserializeI64() (int64, error) {
	value, err := d.DeserializeU64()
	return int64(value), err
}

func (d *Deserializer) DeserializeEnumVariant() (uint32, error) {
	return d.DeserializeU32()
}

func (d *Deserializer) AssertEnd() error {
	if d.Remaining() != 0 {
		return fmt.Errorf("%d trailing bytes", d.Remaining())
	}
	return nil
}

func (d *Deserializer) deserializeVarUint64(max uint64, name string) (uint64, error) {
	var value uint64
	var shift uint
	for i := 0; i < 19; i++ {
		if d.offset >= len(d.buffer) {
			return 0, fmt.Errorf("reached end of postcard buffer")
		}
		b := d.buffer[d.offset]
		d.offset++
		value |= uint64(b&0x7f) << shift
		if (b & 0x80) == 0 {
			if value > max {
				return 0, fmt.Errorf("%s overflow", name)
			}
			return value, nil
		}
		shift += 7
	}
	return 0, fmt.Errorf("%s varint is too long", name)
}

func (d *Deserializer) deserializeVarUintBig(max *big.Int, name string) (*big.Int, error) {
	value := big.NewInt(0)
	part := new(big.Int)
	for i := 0; i < 19; i++ {
		if d.offset >= len(d.buffer) {
			return nil, fmt.Errorf("reached end of postcard buffer")
		}
		b := d.buffer[d.offset]
		d.offset++
		part.SetInt64(int64(b & 0x7f))
		part.Lsh(part, uint(i*7))
		value.Or(value, part)
		if (b & 0x80) == 0 {
			if value.Cmp(max) > 0 {
				return nil, fmt.Errorf("%s overflow", name)
			}
			return value, nil
		}
	}
	return nil, fmt.Errorf("%s varint is too long", name)
}

func (d *Deserializer) read(length int) ([]byte, error) {
	if length < 0 {
		return nil, fmt.Errorf("invalid read length")
	}
	if d.offset+length > len(d.buffer) {
		return nil, fmt.Errorf("reached end of postcard buffer")
	}
	result := make([]byte, length)
	copy(result, d.buffer[d.offset:d.offset+length])
	d.offset += length
	return result, nil
}

func (d *Deserializer) Peek(n int) ([]byte, error) {
	if d.offset+n > len(d.buffer) {
		return nil, fmt.Errorf("not enough bytes to peek")
	}
	return d.buffer[d.offset : d.offset+n], nil
}

func (d *Deserializer) Offset() int {
	return d.offset
}

func (d *Deserializer) Buffer() []byte {
	return d.buffer
}

func (d *Deserializer) Advance(n int) error {
	if n < 0 {
		return fmt.Errorf("Advance with negative offset %d", n)
	}
	if d.offset+n > len(d.buffer) {
		return fmt.Errorf("Advance(%d) would exceed buffer length %d (current offset %d)", n, len(d.buffer), d.offset)
	}
	d.offset += n
	return nil
}
