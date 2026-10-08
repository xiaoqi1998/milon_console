"""Port of gosdk-develop/postcard/postcard_test.go."""

from dataclasses import dataclass

import pytest

from milon_sdk.postcard import (
    Deserializer,
    Serializer,
    deserialize_option,
    deserialize_postcard,
    deserialize_seq,
    deserialize_value,
    serialize_option,
    serialize_postcard,
    serialize_seq,
)


@dataclass
class Pair:
    Left: int
    Right: str

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_u8(self.Left)
        serializer.serialize_str(self.Right)


def deserialize_pair(deserializer: Deserializer) -> Pair:
    left = deserializer.deserialize_u8()
    right = deserializer.deserialize_str()
    return Pair(Left=left, Right=right)


@dataclass
class Person:
    Name: str
    Age: int

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_str(self.Name)
        serializer.serialize_u8(self.Age)


def deserialize_person(deserializer: Deserializer) -> Person:
    name = deserializer.deserialize_str()
    age = deserializer.deserialize_u8()
    return Person(Name=name, Age=age)


def _marshal_pair(serializer: Serializer, value: Pair) -> None:
    value.marshal_postcard(serializer)


def test_serializer_deserializer_round_trip():
    s = Serializer()
    s.serialize_bool(True)
    s.serialize_bool(False)
    s.serialize_u8(255)
    s.serialize_u16(300)
    s.serialize_u32(300)
    s.serialize_u64(300)
    s.serialize_u128(300)
    s.serialize_i8(-2)
    s.serialize_i16(-3)
    s.serialize_i32(-4)
    s.serialize_i64(-5)
    s.serialize_enum_variant(11)
    s.serialize_str("hi")
    s.serialize_bytes(b"\x01\x02")
    s.serialize_fixed_bytes(b"\x03\x04")
    s.serialize(Pair(Left=5, Right="x"))
    serialize_seq(s, [Pair(Left=6, Right="y")], _marshal_pair)
    serialize_option(s, Pair(Left=7, Right="z"), _marshal_pair)
    serialize_option(s, None, _marshal_pair)

    d = Deserializer(s.bytes())
    assert d.deserialize_bool() is True
    assert d.deserialize_bool() is False
    assert d.deserialize_u8() == 255
    assert d.deserialize_u16() == 300
    assert d.deserialize_u32() == 300
    assert d.deserialize_u64() == 300
    assert d.deserialize_u128() == 300
    assert d.deserialize_i8() == -2
    assert d.deserialize_i16() == -3
    assert d.deserialize_i32() == -4
    assert d.deserialize_i64() == -5
    assert d.deserialize_enum_variant() == 11
    assert d.deserialize_str() == "hi"
    assert d.deserialize_bytes() == b"\x01\x02"
    assert d.deserialize_fixed_bytes(2) == b"\x03\x04"
    assert deserialize_value(d, deserialize_pair) == Pair(Left=5, Right="x")
    assert deserialize_seq(d, deserialize_pair) == [Pair(Left=6, Right="y")]
    assert deserialize_option(d, deserialize_pair) == Pair(Left=7, Right="z")
    assert deserialize_option(d, deserialize_pair) is None
    d.assert_end()


def test_person_matches_type_script_fixture():
    data = serialize_postcard(Person(Name="Alice", Age=30))
    assert data.hex() == "05416c6963651e"

    value = deserialize_postcard(data, deserialize_person, allow_trailing=False)
    assert value == Person(Name="Alice", Age=30)


def test_var_uint64_round_trip():
    type_tag = 4454442085531989710

    s1 = Serializer()
    s1.serialize_u64(type_tag)

    s2 = Serializer()
    s2._serialize_var_uint64(type_tag)
    assert s2.bytes() == s1.bytes()

    d = Deserializer(s2.bytes())
    assert d.deserialize_u64() == type_tag


def test_error_paths():
    # truncated varint
    with pytest.raises(ValueError, match="reached end of postcard buffer"):
        Deserializer(b"\x80").deserialize_u32()
    # read beyond buffer
    with pytest.raises(ValueError, match="reached end of postcard buffer"):
        Deserializer(b"\x01").deserialize_fixed_bytes(2)
    # trailing bytes rejected
    with pytest.raises(ValueError, match="1 trailing bytes"):
        deserialize_postcard(b"\x01\x00\x09", deserialize_pair, allow_trailing=False)
    # trailing bytes allowed
    value = deserialize_postcard(b"\x01\x00\x09", deserialize_pair, allow_trailing=True)
    assert value == Pair(Left=1, Right="")
    # bool invalid value
    with pytest.raises(ValueError, match="invalid postcard boolean"):
        Deserializer(b"\x02").deserialize_bool()
    # u16 overflow
    with pytest.raises(ValueError, match="u16 overflow"):
        Deserializer(b"\xff\xff\x04").deserialize_u16()
    # u32 overflow
    with pytest.raises(ValueError, match="u32 overflow"):
        Deserializer(b"\xff\xff\xff\xff\x20").deserialize_u32()
    # varint too long
    with pytest.raises(ValueError, match="u32 varint is too long"):
        Deserializer(b"\x80" * 19).deserialize_u32()
    # u128 overflow
    with pytest.raises(ValueError, match="u128 overflow"):
        Deserializer(b"\xff" * 18 + b"\x7f").deserialize_u128()
    # negative read length
    with pytest.raises(ValueError, match="invalid read length"):
        Deserializer(b"").deserialize_fixed_bytes(-1)
    # deserialize invalid utf8
    with pytest.raises(ValueError, match="invalid UTF-8 string"):
        Deserializer(b"\x02\xff\xfe").deserialize_str()
    # peek out of range
    with pytest.raises(ValueError, match="not enough bytes to peek"):
        Deserializer(b"\x01\x02").peek(3)
    # advance negative errors
    with pytest.raises(ValueError):
        Deserializer(b"\x01").advance(-1)

    # serialize u128 nil（Go 传 nil；Python 传 None 同路径）
    with pytest.raises(ValueError, match="u128 out of range"):
        Serializer().serialize_u128(None)
    # serialize u128 negative
    with pytest.raises(ValueError, match="u128 out of range"):
        Serializer().serialize_u128(-1)
    # serialize u128 too large
    with pytest.raises(ValueError, match="u128 out of range"):
        Serializer().serialize_u128(1 << 128)

    # serialize invalid utf8（Go string([]byte{0xFF,0xFE})：Python 用孤立代理对等价表达非法 UTF-8）
    with pytest.raises(ValueError, match="expected valid UTF-8 string"):
        Serializer().serialize_str("\udcff\udcfe")


def test_peek_remaining_offset():
    d = Deserializer(b"\x01\x02\x03")
    assert d.remaining() == 3

    peeked = d.peek(2)
    assert peeked == b"\x01\x02"
    assert d.remaining() == 3
    assert d.offset() == 0
    assert d.buffer() == b"\x01\x02\x03"

    d.advance(2)
    assert d.remaining() == 1
    assert d.offset() == 2
    assert d.buffer() == b"\x01\x02\x03"
