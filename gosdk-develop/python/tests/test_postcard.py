"""postcard 协议单元测试：round-trip + Go 注释中的黄金向量。"""
from __future__ import annotations

import os
import sys

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from milon_sdk.postcard import Deserializer, Serializer, deserialize_postcard  # noqa: E402

# Go postcard/serializer.go 注释中的编码示例
VARINT_GOLDEN = {
    0: "00",
    127: "7f",
    128: "8001",
    2581: "9514",
    16384: "808001",
    4294967295: "ffffffff0f",
}


@pytest.mark.parametrize("value,expected", VARINT_GOLDEN.items())
def test_varint_encode(value: int, expected: str) -> None:
    s = Serializer()
    s.serialize_u64(value)
    assert s.bytes().hex() == expected


@pytest.mark.parametrize("value,expected", VARINT_GOLDEN.items())
def test_varint_roundtrip(value: int, expected: str) -> None:
    d = Deserializer(bytes.fromhex(expected))
    assert d.deserialize_u64() == value
    d.assert_end()


@pytest.mark.parametrize("width", [8, 16, 32, 64])
def test_uint_roundtrip(width: int) -> None:
    from milon_sdk.postcard.serializer import U128_MAX

    maxv = (1 << width) - 1
    # 0x3FFF/0x4000 仅在该宽度可容纳时才纳入（u8 上溢为测试逻辑错误）
    for value in (0, 1, 0x7F, 0x80, min(0x3FFF, maxv), min(0x4000, maxv), maxv):
        s = Serializer()
        fn = getattr(s, f"serialize_u{width}")
        fn(value)
        d = Deserializer(s.bytes())
        dfn = getattr(d, f"deserialize_u{width}")
        assert dfn() == value


def test_u128_roundtrip() -> None:
    for value in (0, 1, 127, 128, (1 << 64), (1 << 128) - 1):
        s = Serializer()
        s.serialize_u128(value)
        d = Deserializer(s.bytes())
        assert d.deserialize_u128() == value


def test_u128_overflow() -> None:
    s = Serializer()
    with pytest.raises(ValueError):
        s.serialize_u128(1 << 128)


def test_str_roundtrip() -> None:
    s = Serializer()
    s.serialize_str("中文 Milon 🚀")
    d = Deserializer(s.bytes())
    assert d.deserialize_str() == "中文 Milon 🚀"


def test_bytes_roundtrip() -> None:
    payload = bytes(range(256)) * 4
    s = Serializer()
    s.serialize_bytes(payload)
    d = Deserializer(s.bytes())
    assert d.deserialize_bytes() == payload


def test_bool_roundtrip() -> None:
    for value in (True, False):
        s = Serializer()
        s.serialize_bool(value)
        d = Deserializer(s.bytes())
        assert d.deserialize_bool() is value


def test_seq_roundtrip() -> None:
    s = Serializer()
    s.serialize_seq([1, 2, 3, 1000], lambda ss, v: ss.serialize_u32(v))
    d = Deserializer(s.bytes())
    out = d.deserialize_seq(lambda dd: dd.deserialize_u32())
    assert out == [1, 2, 3, 1000]


def test_option_roundtrip() -> None:
    s = Serializer()
    s.serialize_option(False, lambda ss: ss.serialize_u16(0))
    d = Deserializer(s.bytes())
    assert d.deserialize_option(lambda dd: dd.deserialize_u16()) is None

    s = Serializer()
    s.serialize_option(True, lambda ss: ss.serialize_u16(2581))
    d = Deserializer(s.bytes())
    assert d.deserialize_option(lambda dd: dd.deserialize_u16()) == 2581


def test_fixed_bytes() -> None:
    s = Serializer()
    s.write(b"\x01\x02\x03")
    d = Deserializer(s.bytes())
    assert d.deserialize_fixed_bytes(3) == b"\x01\x02\x03"


def test_assert_end_trailing() -> None:
    s = Serializer()
    s.serialize_u64(1)
    d = Deserializer(s.bytes() + b"\x00")
    d.deserialize_u64()
    with pytest.raises(ValueError):
        d.assert_end()


def test_truncated_buffer() -> None:
    s = Serializer()
    s.serialize_u64(1 << 30)  # 需要多字节
    d = Deserializer(s.bytes()[:-1])
    with pytest.raises(ValueError):
        d.deserialize_u64()


def test_deserialize_postcard_strict() -> None:
    s = Serializer()
    s.serialize_u32(42)
    value = deserialize_postcard(s.bytes(), lambda d: d.deserialize_u32())
    assert value == 42
    with pytest.raises(ValueError):
        deserialize_postcard(s.bytes() + b"\x00", lambda d: d.deserialize_u32())
