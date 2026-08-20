"""Postcard 反序列化器（字节级复刻 Go 版 postcard.Deserializer）。

- 变长整数读取：LE 7-bit 组，与序列化严格互逆。
- u8 为单字节原样读取；u128 为最多 19 字节的大整数变长读取。
- TypeResolver 注入：反序列化带 type_tag 的资源/事件时，按 type_tag 动态
  分发到已加载 IDL 的 DecodeResource/DecodeEvent，返回消耗的字节区间。
"""
from __future__ import annotations

from typing import Callable, Optional, Protocol, TypeVar

U128_MAX = (1 << 128) - 1

T = TypeVar("T")


class TypeResolver(Protocol):
    """把 type_tag 解析为值字节区间（对应 Go postcard.TypeResolver）。"""

    def decode_resource(self, type_tag: int, data: bytes) -> tuple[bytes, bytes]:
        """返回 (值字节, 剩余字节)。"""
        ...

    def decode_event(self, type_tag: int, data: bytes) -> tuple[bytes, bytes]:
        """返回 (事件字节, 剩余字节)。"""
        ...


class Deserializer:
    __slots__ = ("_buf", "_offset", "type_resolver")

    def __init__(self, data: bytes | bytearray) -> None:
        self._buf = bytes(data)
        self._offset = 0
        self.type_resolver: Optional[TypeResolver] = None

    # ---------------------------------------------------------- 基础
    @property
    def buffer(self) -> bytes:
        return self._buf

    def offset(self) -> int:
        return self._offset

    def remaining(self) -> int:
        return len(self._buf) - self._offset

    def peek(self, n: int) -> bytes:
        if self._offset + n > len(self._buf):
            raise ValueError(f"not enough bytes to peek (need {n}, have {self.remaining()})")
        return self._buf[self._offset:self._offset + n]

    def advance(self, n: int) -> None:
        if n < 0:
            raise ValueError(f"Advance with negative offset {n}")
        if self._offset + n > len(self._buf):
            raise ValueError(
                f"Advance({n}) would exceed buffer length {len(self._buf)} "
                f"(current offset {self._offset})"
            )
        self._offset += n

    def read(self, length: int) -> bytes:
        if length < 0:
            raise ValueError("invalid read length")
        if self._offset + length > len(self._buf):
            raise ValueError("reached end of postcard buffer")
        result = self._buf[self._offset:self._offset + length]
        self._offset += length
        return result

    def assert_end(self) -> None:
        if self.remaining() != 0:
            raise ValueError(f"{self.remaining()} trailing bytes")

    # ---------------------------------------------------------- 变长整数
    def _deserialize_varuint64(self, max_value: int, name: str) -> int:
        value = 0
        shift = 0
        for _ in range(19):
            if self._offset >= len(self._buf):
                raise ValueError("reached end of postcard buffer")
            b = self._buf[self._offset]
            self._offset += 1
            value |= (b & 0x7F) << shift
            if (b & 0x80) == 0:
                if value > max_value:
                    raise ValueError(f"{name} overflow")
                return value
            shift += 7
        raise ValueError(f"{name} varint is too long")

    def _deserialize_varuint_big(self, max_value: int, name: str) -> int:
        value = 0
        for i in range(19):
            if self._offset >= len(self._buf):
                raise ValueError("reached end of postcard buffer")
            b = self._buf[self._offset]
            self._offset += 1
            value |= (b & 0x7F) << (i * 7)
            if (b & 0x80) == 0:
                if value > max_value:
                    raise ValueError(f"{name} overflow")
                return value
        raise ValueError(f"{name} varint is too long")

    # ---------------------------------------------------------- 具体类型
    def deserialize_u8(self) -> int:
        return self.read(1)[0]

    def deserialize_u16(self) -> int:
        return self._deserialize_varuint64(0xFFFF, "u16")

    def deserialize_u32(self) -> int:
        return self._deserialize_varuint64(0xFFFFFFFF, "u32")

    def deserialize_u64(self) -> int:
        return self._deserialize_varuint64(0xFFFFFFFFFFFFFFFF, "u64")

    def deserialize_u128(self) -> int:
        return self._deserialize_varuint_big(U128_MAX, "u128")

    def deserialize_i8(self) -> int:
        v = self.deserialize_u8()
        return v - 256 if v >= 128 else v

    def deserialize_i16(self) -> int:
        v = self._deserialize_varuint64(0xFFFF, "i16")  # Go 按 u16 读后强转 int16
        return v - 0x10000 if v >= 0x8000 else v

    def deserialize_i32(self) -> int:
        v = self._deserialize_varuint64(0xFFFFFFFF, "i32")
        return v - 0x100000000 if v >= 0x80000000 else v

    def deserialize_i64(self) -> int:
        v = self._deserialize_varuint64(0xFFFFFFFFFFFFFFFF, "i64")
        return v - 0x10000000000000000 if v >= 0x8000000000000000 else v

    def deserialize_bool(self) -> bool:
        value = self.deserialize_u8()
        if value not in (0, 1):
            raise ValueError("invalid postcard boolean")
        return value == 1

    def deserialize_fixed_bytes(self, length: int) -> bytes:
        return self.read(length)

    def deserialize_bytes(self) -> bytes:
        length = self.deserialize_u32()
        if length > self.remaining():
            raise ValueError(f"bytes length {length} exceeds remaining buffer {self.remaining()}")
        return self.deserialize_fixed_bytes(length)

    def deserialize_str(self) -> str:
        raw = self.deserialize_bytes()
        try:
            return raw.decode("utf-8")
        except UnicodeDecodeError as exc:  # Go 侧显式校验 UTF-8
            raise ValueError(f"invalid UTF-8 string: {exc}")

    def deserialize_enum_variant(self) -> int:
        return self.deserialize_u32()

    # ---------------------------------------------------------- 组合类型
    def deserialize_option(self, fn: Callable[["Deserializer"], T]) -> Optional[T]:
        has_value = self.deserialize_bool()
        if not has_value:
            return None
        return fn(self)

    def deserialize_seq(self, fn: Callable[["Deserializer"], T]) -> list[T]:
        length = self.deserialize_u32()
        if length > self.remaining():
            raise ValueError(f"seq length {length} exceeds remaining buffer {self.remaining()}")
        return [fn(self) for _ in range(length)]


def deserialize_postcard(
    data: bytes,
    fn: Callable[[Deserializer], T],
    allow_trailing: bool = False,
    resolver: Optional[TypeResolver] = None,
) -> T:
    """入口：反序列化并（默认）校验无尾随字节。resolver 注入 type_tag 解析器。"""
    d = Deserializer(data)
    d.type_resolver = resolver
    value = fn(d)
    if not allow_trailing:
        d.assert_end()
    return value
