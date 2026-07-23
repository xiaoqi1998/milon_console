package postcard

import (
	"encoding/hex"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/big"
	"reflect"
	"testing"
)

type pair struct {
	Left  uint8
	Right string
}

func (p pair) MarshalPostcard(serializer *Serializer) error {
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

func (p person) MarshalPostcard(serializer *Serializer) error {
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

func Test1(t *testing.T) {
	var typeTag uint64 = 4454442085531989710

	s1 := NewSerializer()
	err := s1.SerializeU64(typeTag)
	assert.NoError(t, err)
	fmt.Printf("s1.Bytes(): %v \n", s1.Bytes())

	s2 := NewSerializer()
	err = s2.serializeVarUint64(typeTag)
	assert.NoError(t, err)
	fmt.Printf("s2.Bytes(): %v \n", s2.Bytes())
	assert.Equal(t, s1.Bytes(), s2.Bytes())

	deserializer := NewDeserializer(s2.bytes)
	back, err := deserializer.DeserializeU64()
	assert.NoError(t, err)
	fmt.Printf("back: %v \n", back)

	assert.Equal(t, typeTag, back)
}

func TestSerializerDeserializerRoundTrip(t *testing.T) {
	s := NewSerializer()
	mustNoErr(t, s.SerializeBool(true))
	mustNoErr(t, s.SerializeBool(false))
	mustNoErr(t, s.SerializeU8(255))
	mustNoErr(t, s.SerializeU16(300))
	mustNoErr(t, s.SerializeU32(300))
	mustNoErr(t, s.SerializeU64(300))
	mustNoErr(t, s.SerializeU128(big.NewInt(300)))
	mustNoErr(t, s.SerializeI8(-2))
	mustNoErr(t, s.SerializeI16(-3))
	mustNoErr(t, s.SerializeI32(-4))
	mustNoErr(t, s.SerializeI64(-5))
	mustNoErr(t, s.SerializeEnumVariant(11))
	mustNoErr(t, s.SerializeStr("hi"))
	mustNoErr(t, s.SerializeBytes([]byte{1, 2}))
	s.SerializeFixedBytes([]byte{3, 4})
	mustNoErr(t, s.Serialize(pair{Left: 5, Right: "x"}))
	mustNoErr(t, SerializeSeq(s, []pair{{Left: 6, Right: "y"}}, func(serializer *Serializer, value pair) error {
		return value.MarshalPostcard(serializer)
	}))
	optionPair := pair{Left: 7, Right: "z"}
	mustNoErr(t, SerializeOption(s, &optionPair, func(serializer *Serializer, value pair) error {
		return value.MarshalPostcard(serializer)
	}))
	mustNoErr(t, SerializeOption[pair](s, nil, func(serializer *Serializer, value pair) error {
		return value.MarshalPostcard(serializer)
	}))

	d := NewDeserializer(s.Bytes())
	assertEqual(t, true, mustCall(t, d.DeserializeBool))
	assertEqual(t, false, mustCall(t, d.DeserializeBool))
	assertEqual(t, uint8(255), mustCall(t, d.DeserializeU8))
	assertEqual(t, uint16(300), mustCall(t, d.DeserializeU16))
	assertEqual(t, uint32(300), mustCall(t, d.DeserializeU32))
	assertEqual(t, uint64(300), mustCall(t, d.DeserializeU64))
	assertEqual(t, big.NewInt(300).String(), mustCall(t, d.DeserializeU128).String())
	assertEqual(t, int8(-2), mustCall(t, d.DeserializeI8))
	assertEqual(t, int16(-3), mustCall(t, d.DeserializeI16))
	assertEqual(t, int32(-4), mustCall(t, d.DeserializeI32))
	assertEqual(t, int64(-5), mustCall(t, d.DeserializeI64))
	assertEqual(t, uint32(11), mustCall(t, d.DeserializeEnumVariant))
	assertEqual(t, "hi", mustCall(t, d.DeserializeStr))
	assertEqual(t, []byte{1, 2}, mustCall(t, d.DeserializeBytes))
	assertEqual(t, []byte{3, 4}, mustCall(t, func() ([]byte, error) { return d.DeserializeFixedBytes(2) }))
	assertEqual(t, pair{Left: 5, Right: "x"}, mustCall(t, func() (pair, error) { return DeserializeValue(d, deserializePair) }))
	assertEqual(t, []pair{{Left: 6, Right: "y"}}, mustCall(t, func() ([]pair, error) { return DeserializeSeq(d, deserializePair) }))
	optionValue := mustCall(t, func() (*pair, error) { return DeserializeOption(d, deserializePair) })
	if optionValue == nil || *optionValue != (pair{Left: 7, Right: "z"}) {
		t.Fatalf("unexpected option value: %#v", optionValue)
	}
	assertEqual[*pair](t, nil, mustCall(t, func() (*pair, error) { return DeserializeOption(d, deserializePair) }))
	mustNoErr(t, d.AssertEnd())
}

func TestPersonMatchesTypeScriptFixture(t *testing.T) {
	bytes, err := SerializePostcard(person{Name: "Alice", Age: 30})
	mustNoErr(t, err)
	assertEqual(t, "05416c6963651e", hex.EncodeToString(bytes))

	value, err := DeserializePostcard(bytes, deserializePerson, false)
	mustNoErr(t, err)
	assertEqual(t, person{Name: "Alice", Age: 30}, value)
}

func TestErrorsAndTrailingBytes(t *testing.T) {
	_, err := NewDeserializer([]byte{2}).DeserializeBool()
	assertError(t, err, "invalid postcard boolean")

	_, err = NewDeserializer([]byte{0x80}).DeserializeU32()
	assertError(t, err, "reached end of postcard buffer")

	_, err = NewDeserializer([]byte{1}).DeserializeFixedBytes(2)
	assertError(t, err, "reached end of postcard buffer")

	_, err = DeserializePostcard([]byte{1, 0, 9}, deserializePair, false)
	assertError(t, err, "1 trailing bytes")

	value, err := DeserializePostcard([]byte{1, 0, 9}, deserializePair, true)
	mustNoErr(t, err)
	assertEqual(t, pair{Left: 1, Right: ""}, value)
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustValue[T any](t *testing.T, value T, err error) T {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustCall[T any](t *testing.T, fn func() (T, error)) T {
	t.Helper()
	value, err := fn()
	return mustValue(t, value, err)
}

func assertEqual[T any](t *testing.T, want, got T) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func assertError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("want error %q, got %v", want, err)
	}
}
