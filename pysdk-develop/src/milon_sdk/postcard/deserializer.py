"""Postcard deserializer. Faithful port of gosdk-develop/postcard/deserializer.go."""

from __future__ import annotations

from typing import Protocol

MAX_U128 = (1 << 128) - 1


class TypeResolver(Protocol):
    """Resolves a type_tag into the byte range of its value.

    Implemented by provider.IDLTypeResolver; injected per-deserializer so
    multiple clients can decode with their own loaded IDLs concurrently.
    """

    def decode_resource(self, type_tag: int, data: bytes) -> tuple[bytes, bytes]:
        """Returns (value_bytes, remaining)."""
        ...

    def decode_event(self, type_tag: int, data: bytes) -> tuple[bytes, bytes]:
        """Returns (event_bytes, remaining)."""
        ...


class Deserializer:
    def __init__(self, data: bytes) -> None:
        self._buffer = data
        self._offset = 0
        self._type_resolver: TypeResolver | None = None

    def set_type_resolver(self, resolver: TypeResolver) -> None:
        """Sets the type_tag resolver used by api.ReadAnySerializeValueWithTypeTag
        and api.DeserializeEventEntry during this deserialization."""
        self._type_resolver = resolver

    def type_resolver(self) -> TypeResolver | None:
        """Returns the resolver set via set_type_resolver, or None."""
        return self._type_resolver

    def remaining(self) -> int:
        return len(self._buffer) - self._offset

    def deserialize_str(self) -> str:
        data = self.deserialize_bytes()
        try:
            return data.decode("utf-8")
        except UnicodeDecodeError as exc:
            raise ValueError("invalid UTF-8 string") from exc

    def deserialize_bytes(self) -> bytes:
        length = self.deserialize_u32()
        if length > self.remaining():
            raise ValueError(
                f"bytes length {length} exceeds remaining buffer {self.remaining()}"
            )
        return self.deserialize_fixed_bytes(length)

    def deserialize_fixed_bytes(self, length: int) -> bytes:
        return self._read(length)

    def deserialize_bool(self) -> bool:
        value = self.deserialize_u8()
        if value not in (0, 1):
            raise ValueError("invalid postcard boolean")
        return value == 1

    def deserialize_u8(self) -> int:
        if self._offset >= len(self._buffer):
            raise ValueError("reached end of postcard buffer")
        b = self._buffer[self._offset]
        self._offset += 1
        return b

    def deserialize_u16(self) -> int:
        return self._deserialize_var_uint64(0xFFFF, "u16")

    def deserialize_u32(self) -> int:
        return self._deserialize_var_uint64(0xFFFFFFFF, "u32")

    def deserialize_u64(self) -> int:
        return self._deserialize_var_uint64(0xFFFFFFFFFFFFFFFF, "u64")

    def deserialize_u128(self) -> int:
        return self._deserialize_var_uint_big(MAX_U128, "u128")

    def deserialize_i8(self) -> int:
        value = self.deserialize_u8()
        return value - 0x100 if value >= 0x80 else value

    def deserialize_i16(self) -> int:
        value = self.deserialize_u16()
        return value - 0x10000 if value >= 0x8000 else value

    def deserialize_i32(self) -> int:
        value = self.deserialize_u32()
        return value - 0x100000000 if value >= 0x80000000 else value

    def deserialize_i64(self) -> int:
        value = self.deserialize_u64()
        return value - 0x10000000000000000 if value >= 0x8000000000000000 else value

    def deserialize_enum_variant(self) -> int:
        return self.deserialize_u32()

    def assert_end(self) -> None:
        if self.remaining() != 0:
            raise ValueError(f"{self.remaining()} trailing bytes")

    def _deserialize_var_uint64(self, max_value: int, name: str) -> int:
        value = 0
        shift = 0
        for _ in range(19):
            if self._offset >= len(self._buffer):
                raise ValueError("reached end of postcard buffer")
            b = self._buffer[self._offset]
            self._offset += 1
            value |= (b & 0x7F) << shift
            if (b & 0x80) == 0:
                if value > max_value:
                    raise ValueError(f"{name} overflow")
                return value
            shift += 7
        raise ValueError(f"{name} varint is too long")

    def _deserialize_var_uint_big(self, max_value: int, name: str) -> int:
        value = 0
        for i in range(19):
            if self._offset >= len(self._buffer):
                raise ValueError("reached end of postcard buffer")
            b = self._buffer[self._offset]
            self._offset += 1
            value |= (b & 0x7F) << (i * 7)
            if (b & 0x80) == 0:
                if value > max_value:
                    raise ValueError(f"{name} overflow")
                return value
        raise ValueError(f"{name} varint is too long")

    def _read(self, length: int) -> bytes:
        if length < 0:
            raise ValueError("invalid read length")
        if self._offset + length > len(self._buffer):
            raise ValueError("reached end of postcard buffer")
        result = self._buffer[self._offset : self._offset + length]
        self._offset += length
        return result

    def peek(self, n: int) -> bytes:
        if self._offset + n > len(self._buffer):
            raise ValueError("not enough bytes to peek")
        return self._buffer[self._offset : self._offset + n]

    def offset(self) -> int:
        return self._offset

    def buffer(self) -> bytes:
        return self._buffer

    def advance(self, n: int) -> None:
        if n < 0:
            raise ValueError(f"Advance with negative offset {n}")
        if self._offset + n > len(self._buffer):
            raise ValueError(
                f"Advance({n}) would exceed buffer length {len(self._buffer)} "
                f"(current offset {self._offset})"
            )
        self._offset += n
