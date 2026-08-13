package postcard

import (
	"fmt"
	"math"
	"math/big"
	"unicode/utf8"
)

type Deserializer struct {
	buffer []byte
	offset int
}

func NewDeserializer(data []byte) *Deserializer {
	return &Deserializer{buffer: append([]byte(nil), data...)}
}

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
	bytes, err := d.read(1)
	if err != nil {
		return 0, err
	}
	return bytes[0], nil
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
		byteValue, err := d.DeserializeU8()
		if err != nil {
			return 0, err
		}
		value |= uint64(byteValue&0x7f) << shift
		if (byteValue & 0x80) == 0 {
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
		byteValue, err := d.DeserializeU8()
		if err != nil {
			return nil, err
		}
		part.SetInt64(int64(byteValue & 0x7f))
		part.Lsh(part, uint(i*7))
		value.Or(value, part)
		if (byteValue & 0x80) == 0 {
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
	bytes := append([]byte(nil), d.buffer[d.offset:d.offset+length]...)
	d.offset += length
	return bytes, nil
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

func (d *Deserializer) Advance(n int) {
	if n < 0 {
		panic("Advance with negative offset")
	}
	d.offset += n
}
