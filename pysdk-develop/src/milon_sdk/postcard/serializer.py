"""Postcard serializer. Faithful port of gosdk-develop/postcard/serializer.go."""

from __future__ import annotations

from typing import Protocol

MAX_U128 = (1 << 128) - 1


class Marshaler(Protocol):
    def marshal_postcard(self, serializer: Serializer) -> None:
        ...


class Serializer:
    """Accumulates Postcard-encoded bytes."""

    def __init__(self) -> None:
        self._bytes = bytearray()

    def bytes(self) -> bytes:
        return bytes(self._bytes)

    def serialize(self, value: Marshaler) -> None:
        value.marshal_postcard(self)

    def serialize_str(self, value: str) -> None:
        try:
            encoded = value.encode("utf-8")
        except UnicodeEncodeError as exc:
            raise ValueError("expected valid UTF-8 string") from exc
        self.serialize_bytes(encoded)

    def serialize_bytes(self, value: bytes) -> None:
        self.serialize_u32(len(value))
        self.serialize_fixed_bytes(value)

    def serialize_fixed_bytes(self, value: bytes) -> None:
        self._bytes += value

    def serialize_bool(self, value: bool) -> None:
        if value:
            self.serialize_u8(1)
        else:
            self.serialize_u8(0)

    def serialize_u8(self, value: int) -> None:
        if not 0 <= value <= 0xFF:
            raise ValueError("u8 out of range")
        self._bytes.append(value)

    def serialize_u16(self, value: int) -> None:
        if not 0 <= value <= 0xFFFF:
            raise ValueError("u16 out of range")
        self._serialize_var_uint64(value)

    def serialize_u32(self, value: int) -> None:
        if not 0 <= value <= 0xFFFFFFFF:
            raise ValueError("u32 out of range")
        self._serialize_var_uint64(value)  # uses variable-length encoding

    def serialize_u64(self, value: int) -> None:
        if not 0 <= value <= 0xFFFFFFFFFFFFFFFF:
            raise ValueError("u64 out of range")
        self._serialize_var_uint64(value)

    def serialize_u128(self, value: int) -> None:
        if value is None or value < 0 or value > MAX_U128:
            raise ValueError("u128 out of range")
        self._serialize_var_uint_big(value)

    def serialize_i8(self, value: int) -> None:
        if not -0x80 <= value <= 0x7F:
            raise ValueError("i8 out of range")
        self.serialize_u8(value & 0xFF)

    def serialize_i16(self, value: int) -> None:
        if not -0x8000 <= value <= 0x7FFF:
            raise ValueError("i16 out of range")
        self.serialize_u16(value & 0xFFFF)

    def serialize_i32(self, value: int) -> None:
        if not -0x80000000 <= value <= 0x7FFFFFFF:
            raise ValueError("i32 out of range")
        self.serialize_u32(value & 0xFFFFFFFF)

    def serialize_i64(self, value: int) -> None:
        if not -0x8000000000000000 <= value <= 0x7FFFFFFFFFFFFFFF:
            raise ValueError("i64 out of range")
        self.serialize_u64(value & 0xFFFFFFFFFFFFFFFF)

    def serialize_enum_variant(self, index: int) -> None:
        self.serialize_u32(index)

    # serializeVarUint64 serializes a uint64 value using varint
    # (variable-length integer) encoding.
    #
    # Varint encoding rules:
    #   - The highest bit (bit 7) of each byte is the continuation flag:
    #     1 = more bytes follow, 0 = last byte
    #   - The low 7 bits (bit 0-6) are data bits, arranged in little-endian order
    #   - Smaller values use fewer bytes (space optimized)
    #
    # Encoding examples:
    #
    #     0          -> [0x00]                          (1 byte)
    #     127        -> [0x7F]                          (1 byte)
    #     128        -> [0x80, 0x01]                    (2 bytes)
    #     2581       -> [0x95, 0x14]                    (2 bytes)
    #     16384      -> [0x80, 0x80, 0x01]              (3 bytes)
    #     4294967295 -> [0xFF, 0xFF, 0xFF, 0xFF, 0x0F]  (5 bytes, uint32 max)
    #
    # Byte count ranges:
    #   - uint8:  1-2 bytes
    #   - uint16: 1-3 bytes
    #   - uint32: 1-5 bytes
    #   - uint64: 1-10 bytes
    def _serialize_var_uint64(self, value: int) -> None:
        while value >= 0x80:
            self._bytes.append((value & 0x7F) | 0x80)  # set high bit to 1, indicating more bytes follow
            value >>= 7
        self._bytes.append(value)  # last byte, high bit is 0

    def _serialize_var_uint_big(self, value: int) -> None:
        remaining = value
        while remaining >= 0x80:
            self._bytes.append((remaining & 0x7F) | 0x80)
            remaining >>= 7
        if remaining > 0xFF:
            raise ValueError("u128 out of range")
        self._bytes.append(remaining)
