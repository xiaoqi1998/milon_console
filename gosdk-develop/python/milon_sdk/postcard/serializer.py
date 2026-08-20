"""Postcard 二进制序列化器（字节级复刻 Go 版 postcard.Serializer）。

编码规则（与 Go 完全一致，勿改动）：
- 变长整数（u16/u32/u64 共用）：LE 7-bit 组，每字节最高位(bit7)为续接标志：
  1=还有后续字节，0=末字节。低 7 位为数据位，按小端顺序排列。
  例：0→[0x00]  127→[0x7F]  128→[0x80,0x01]  2581→[0x95,0x14]
  16384→[0x80,0x80,0x01]  4294967295→[0xFF,0xFF,0xFF,0xFF,0x0F]
- u8：单字节原样写入（不走变长编码）。
- u128：大整数变长编码（最多 19 字节，7 位一组 LE）。
- bytes/str：u32 长度前缀(varint) + 原始字节。
- bool：单字节 0/1。
"""
from __future__ import annotations

from typing import TYPE_CHECKING, Protocol

if TYPE_CHECKING:  # pragma: no cover
    from .deserializer import Deserializer  # noqa: F401

U128_MAX = (1 << 128) - 1


class Marshaler(Protocol):
    """实现该协议的对象可被序列化（对应 Go postcard.Marshaler）。"""

    def marshal_postcard(self, serializer: "Serializer") -> None:
        ...


class Serializer:
    """按顺序向内部字节缓冲写入 Postcard 编码。"""

    __slots__ = ("_buf",)

    def __init__(self) -> None:
        self._buf = bytearray()

    # ---------------------------------------------------------- 基础输出
    def bytes(self) -> bytes:
        return bytes(self._buf)

    def write(self, raw: bytes) -> None:
        """追加原始字节（对应 Go SerializeFixedBytes）。"""
        self._buf += raw

    # ---------------------------------------------------------- 变长整数
    def serialize_varuint64(self, value: int) -> None:
        """u64 变长编码（LE 7-bit 组，continuation bit 置 1）。"""
        if value < 0:
            raise ValueError("postcard varint: negative value")
        while value >= 0x80:
            self._buf.append((value & 0x7F) | 0x80)
            value >>= 7
        self._buf.append(value)

    def serialize_varuint_big(self, value: int) -> None:
        """u128 大整数变长编码。"""
        if value < 0 or value > U128_MAX:
            raise ValueError("u128 out of range")
        remaining = value
        while remaining >= 0x80:
            self._buf.append((remaining & 0x7F) | 0x80)
            remaining >>= 7
        self._buf.append(remaining)

    # ---------------------------------------------------------- 具体类型
    def serialize_u8(self, value: int) -> None:
        self._buf.append(value & 0xFF)

    def serialize_u16(self, value: int) -> None:
        self.serialize_varuint64(value)

    def serialize_u32(self, value: int) -> None:
        self.serialize_varuint64(value)

    def serialize_u64(self, value: int) -> None:
        self.serialize_varuint64(value)

    def serialize_u128(self, value: int) -> None:
        self.serialize_varuint_big(value)

    def serialize_i8(self, value: int) -> None:
        self.serialize_u8(value & 0xFF)

    def serialize_i16(self, value: int) -> None:
        self.serialize_u16(value & 0xFFFF)

    def serialize_i32(self, value: int) -> None:
        self.serialize_u32(value & 0xFFFFFFFF)

    def serialize_i64(self, value: int) -> None:
        self.serialize_u64(value & 0xFFFFFFFFFFFFFFFF)

    def serialize_bool(self, value: bool) -> None:
        self.serialize_u8(1 if value else 0)

    def serialize_bytes(self, value: bytes | bytearray) -> None:
        """字节串：[u32 长度(varint)] + 原始字节。"""
        self.serialize_u32(len(value))
        self._buf += value

    def serialize_str(self, value: str) -> None:
        """UTF-8 字符串：同 bytes。"""
        self.serialize_bytes(value.encode("utf-8"))

    def serialize_enum_variant(self, index: int) -> None:
        self.serialize_u32(index)

    # ---------------------------------------------------------- 组合类型
    def serialize_option(self, has_value: bool, marshal_value) -> None:
        """option<T>：[bool] + [值]（has_value=True 时）。"""
        self.serialize_bool(has_value)
        if has_value:
            marshal_value(self)

    def serialize_seq(self, values, marshal_value) -> None:
        """seq<T>：[u32 长度] + 各元素。"""
        self.serialize_u32(len(values))
        for v in values:
            marshal_value(self, v)

    def serialize(self, value: Marshaler) -> None:
        value.marshal_postcard(self)


def serialize_postcard(value: Marshaler) -> bytes:
    s = Serializer()
    value.marshal_postcard(s)
    return s.bytes()
