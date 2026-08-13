package postcard

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

type pair struct {
	Left  uint8
	Right string
}

func (p *pair) MarshalPostcard(serializer *Serializer) error {
	if err := serializer.SerializeU8(p.Left); err != nil {
		return err
	}
	return serializer.SerializeStr(p.Right)
}

func deserializePair(deserializer *Deserializer) (pair, error) {
	left, err := deserializer.DeserializeU8()
	if err != nil {
		return pair{}, err
	}
	right, err := deserializer.DeserializeStr()
	if err != nil {
		return pair{}, err
	}
	return pair{Left: left, Right: right}, nil
}

type person struct {
	Name string
	Age  uint8
}

func (p *person) MarshalPostcard(serializer *Serializer) error {
	if err := serializer.SerializeStr(p.Name); err != nil {
		return err
	}
	return serializer.SerializeU8(p.Age)
}

func deserializePerson(deserializer *Deserializer) (person, error) {
	name, err := deserializer.DeserializeStr()
	if err != nil {
		return person{}, err
	}
	age, err := deserializer.DeserializeU8()
	if err != nil {
		return person{}, err
	}
	return person{Name: name, Age: age}, nil
}

func TestSerializerDeserializerRoundTrip(t *testing.T) {
	s := NewSerializer()
	assert.NoError(t, s.SerializeBool(true))
	assert.NoError(t, s.SerializeBool(false))
	assert.NoError(t, s.SerializeU8(255))
	assert.NoError(t, s.SerializeU16(300))
	assert.NoError(t, s.SerializeU32(300))
	assert.NoError(t, s.SerializeU64(300))
	assert.NoError(t, s.SerializeU128(big.NewInt(300)))
	assert.NoError(t, s.SerializeI8(-2))
	assert.NoError(t, s.SerializeI16(-3))
	assert.NoError(t, s.SerializeI32(-4))
	assert.NoError(t, s.SerializeI64(-5))
	assert.NoError(t, s.SerializeEnumVariant(11))
	assert.NoError(t, s.SerializeStr("hi"))
	assert.NoError(t, s.SerializeBytes([]byte{1, 2}))
	s.SerializeFixedBytes([]byte{3, 4})
	assert.NoError(t, s.Serialize(&pair{Left: 5, Right: "x"}))
	assert.NoError(t, SerializeSeq(s, []pair{{Left: 6, Right: "y"}}, func(serializer *Serializer, value pair) error {
		return value.MarshalPostcard(serializer)
	}))
	optionPair := pair{Left: 7, Right: "z"}
	assert.NoError(t, SerializeOption(s, &optionPair, func(serializer *Serializer, value pair) error {
		return value.MarshalPostcard(serializer)
	}))
	assert.NoError(t, SerializeOption[pair](s, nil, func(serializer *Serializer, value pair) error {
		return value.MarshalPostcard(serializer)
	}))

	d := NewDeserializer(s.Bytes())
	if value, err := d.DeserializeBool(); assert.NoError(t, err) {
		assert.True(t, value)
	}
	if value, err := d.DeserializeBool(); assert.NoError(t, err) {
		assert.False(t, value)
	}
	if value, err := d.DeserializeU8(); assert.NoError(t, err) {
		assert.Equal(t, uint8(255), value)
	}
	if value, err := d.DeserializeU16(); assert.NoError(t, err) {
		assert.Equal(t, uint16(300), value)
	}
	if value, err := d.DeserializeU32(); assert.NoError(t, err) {
		assert.Equal(t, uint32(300), value)
	}
	if value, err := d.DeserializeU64(); assert.NoError(t, err) {
		assert.Equal(t, uint64(300), value)
	}
	if value, err := d.DeserializeU128(); assert.NoError(t, err) {
		assert.Equal(t, big.NewInt(300).String(), value.String())
	}
	if value, err := d.DeserializeI8(); assert.NoError(t, err) {
		assert.Equal(t, int8(-2), value)
	}
	if value, err := d.DeserializeI16(); assert.NoError(t, err) {
		assert.Equal(t, int16(-3), value)
	}
	if value, err := d.DeserializeI32(); assert.NoError(t, err) {
		assert.Equal(t, int32(-4), value)
	}
	if value, err := d.DeserializeI64(); assert.NoError(t, err) {
		assert.Equal(t, int64(-5), value)
	}
	if value, err := d.DeserializeEnumVariant(); assert.NoError(t, err) {
		assert.Equal(t, uint32(11), value)
	}
	if value, err := d.DeserializeStr(); assert.NoError(t, err) {
		assert.Equal(t, "hi", value)
	}
	if value, err := d.DeserializeBytes(); assert.NoError(t, err) {
		assert.Equal(t, []byte{1, 2}, value)
	}
	if value, err := d.DeserializeFixedBytes(2); assert.NoError(t, err) {
		assert.Equal(t, []byte{3, 4}, value)
	}
	if value, err := DeserializeValue(d, deserializePair); assert.NoError(t, err) {
		assert.Equal(t, pair{Left: 5, Right: "x"}, value)
	}
	if values, err := DeserializeSeq(d, deserializePair); assert.NoError(t, err) {
		assert.Equal(t, []pair{{Left: 6, Right: "y"}}, values)
	}
	optionValue, err := DeserializeOption(d, deserializePair)
	assert.NoError(t, err)
	if assert.NotNil(t, optionValue) {
		assert.Equal(t, pair{Left: 7, Right: "z"}, *optionValue)
	}
	optionNil, err := DeserializeOption(d, deserializePair)
	assert.NoError(t, err)
	assert.Nil(t, optionNil)
	assert.NoError(t, d.AssertEnd())
}

func TestPersonMatchesTypeScriptFixture(t *testing.T) {
	bytes, err := SerializePostcard(&person{Name: "Alice", Age: 30})
	assert.NoError(t, err)
	assert.Equal(t, "05416c6963651e", hex.EncodeToString(bytes))

	value, err := DeserializePostcard(bytes, deserializePerson, false)
	assert.NoError(t, err)
	assert.Equal(t, person{Name: "Alice", Age: 30}, value)
}

func TestVarUint64RoundTrip(t *testing.T) {
	var typeTag uint64 = 4454442085531989710

	s1 := NewSerializer()
	assert.NoError(t, s1.SerializeU64(typeTag))

	s2 := NewSerializer()
	assert.NoError(t, s2.serializeVarUint64(typeTag))
	assert.Equal(t, s1.Bytes(), s2.Bytes())

	back, err := NewDeserializer(s2.bytes).DeserializeU64()
	assert.NoError(t, err)
	assert.Equal(t, typeTag, back)
}

func TestErrorPaths(t *testing.T) {
	t.Run("truncated varint", func(t *testing.T) {
		_, err := NewDeserializer([]byte{0x80}).DeserializeU32()
		assert.EqualError(t, err, "reached end of postcard buffer")
	})
	t.Run("read beyond buffer", func(t *testing.T) {
		_, err := NewDeserializer([]byte{1}).DeserializeFixedBytes(2)
		assert.EqualError(t, err, "reached end of postcard buffer")
	})
	t.Run("trailing bytes rejected", func(t *testing.T) {
		_, err := DeserializePostcard([]byte{1, 0, 9}, deserializePair, false)
		assert.EqualError(t, err, "1 trailing bytes")
	})
	t.Run("trailing bytes allowed", func(t *testing.T) {
		value, err := DeserializePostcard([]byte{1, 0, 9}, deserializePair, true)
		assert.NoError(t, err)
		assert.Equal(t, pair{Left: 1, Right: ""}, value)
	})
	t.Run("bool invalid value", func(t *testing.T) {
		_, err := NewDeserializer([]byte{2}).DeserializeBool()
		assert.EqualError(t, err, "invalid postcard boolean")
	})
	t.Run("u16 overflow", func(t *testing.T) {
		_, err := NewDeserializer([]byte{0xFF, 0xFF, 0x04}).DeserializeU16()
		assert.EqualError(t, err, "u16 overflow")
	})
	t.Run("u32 overflow", func(t *testing.T) {
		_, err := NewDeserializer([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x20}).DeserializeU32()
		assert.EqualError(t, err, "u32 overflow")
	})
	t.Run("varint too long", func(t *testing.T) {
		data := make([]byte, 19)
		for i := range data {
			data[i] = 0x80
		}
		_, err := NewDeserializer(data).DeserializeU32()
		assert.EqualError(t, err, "u32 varint is too long")
	})
	t.Run("u128 overflow", func(t *testing.T) {
		data := make([]byte, 18)
		for i := range data {
			data[i] = 0xFF
		}
		_, err := NewDeserializer(append(data, 0x7F)).DeserializeU128()
		assert.EqualError(t, err, "u128 overflow")
	})
	t.Run("negative read length", func(t *testing.T) {
		_, err := NewDeserializer([]byte{}).DeserializeFixedBytes(-1)
		assert.EqualError(t, err, "invalid read length")
	})
	t.Run("deserialize invalid utf8", func(t *testing.T) {
		_, err := NewDeserializer([]byte{0x02, 0xFF, 0xFE}).DeserializeStr()
		assert.EqualError(t, err, "invalid UTF-8 string")
	})
	t.Run("peek out of range", func(t *testing.T) {
		_, err := NewDeserializer([]byte{1, 2}).Peek(3)
		assert.EqualError(t, err, "not enough bytes to peek")
	})
	t.Run("advance negative panics", func(t *testing.T) {
		assert.Panics(t, func() { NewDeserializer([]byte{1}).Advance(-1) })
	})

	t.Run("serialize u128 nil", func(t *testing.T) {
		err := NewSerializer().SerializeU128(nil)
		assert.EqualError(t, err, "u128 out of range")
	})
	t.Run("serialize u128 negative", func(t *testing.T) {
		err := NewSerializer().SerializeU128(big.NewInt(-1))
		assert.EqualError(t, err, "u128 out of range")
	})
	t.Run("serialize u128 too large", func(t *testing.T) {
		err := NewSerializer().SerializeU128(new(big.Int).Lsh(big.NewInt(1), 128))
		assert.EqualError(t, err, "u128 out of range")
	})

	t.Run("serialize invalid utf8", func(t *testing.T) {
		err := NewSerializer().SerializeStr(string([]byte{0xFF, 0xFE}))
		assert.EqualError(t, err, "expected valid UTF-8 string")
	})
}

func TestPeekRemainingOffset(t *testing.T) {
	d := NewDeserializer([]byte{1, 2, 3})
	assert.Equal(t, 3, d.Remaining())

	peeked, err := d.Peek(2)
	assert.NoError(t, err)
	assert.Equal(t, []byte{1, 2}, peeked)
	assert.Equal(t, 3, d.Remaining())
	assert.Equal(t, 0, d.Offset())
	assert.Equal(t, []byte{1, 2, 3}, d.Buffer())

	d.Advance(2)
	assert.Equal(t, 1, d.Remaining())
	assert.Equal(t, 2, d.Offset())
	assert.Equal(t, []byte{1, 2, 3}, d.Buffer())
}
