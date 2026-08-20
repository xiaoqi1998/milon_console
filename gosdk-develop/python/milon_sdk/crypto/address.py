"""地址类型与派生（对应 Go crypto/address.go）。

- Address = BLAKE3(MILON_ROOT || "milon.address.pk.v1" || pk.Bytes)[:20]
- 序列化形态：Hex（40 字符）或 Base58（Go String() 默认 Base58）。
"""
from __future__ import annotations

import base58  # type: ignore

from ..postcard import Deserializer, Serializer
from .errors import InvalidPublicKeyError
from .hashes import PK_ADDRESS_DOMAIN, hash32

ADDRESS_RAW_LEN = 20
ADDRESS_HEX_LEN = ADDRESS_RAW_LEN * 2


def _decode_hex(s: str) -> bytes:
    body = s[2:] if len(s) >= 2 and s[:2] in ("0x", "0X") else s
    if len(body) != ADDRESS_HEX_LEN:
        raise ValueError(f"invalid hex length: expected {ADDRESS_HEX_LEN}, got {len(body)}")
    return bytes.fromhex(body)


class Address:
    """20 字节链上地址。以不可变 bytes 承载，可哈希（Go 中用作 map key）。"""

    __slots__ = ("_bytes",)

    def __init__(self, raw: bytes):
        if len(raw) != ADDRESS_RAW_LEN:
            raise ValueError(f"invalid address length: expected {ADDRESS_RAW_LEN}, got {len(raw)}")
        self._bytes = bytes(raw)

    # ---------------------------------------------------------- 构造
    @classmethod
    def from_public_key(cls, pk: "PublicKey") -> "Address":  # noqa: F821
        digest = hash32(PK_ADDRESS_DOMAIN, pk.as_bytes())
        return cls(digest[:ADDRESS_RAW_LEN])

    @classmethod
    def from_bytes(cls, raw: bytes) -> "Address":
        return cls(raw)

    @classmethod
    def from_hex(cls, s: str) -> "Address":
        return cls(_decode_hex(s.strip()))

    @classmethod
    def from_base58(cls, s: str) -> "Address":
        buf = base58.b58decode(s.strip())
        if len(buf) != ADDRESS_RAW_LEN:
            raise ValueError(
                f"invalid base58 decoded length: expected {ADDRESS_RAW_LEN}, got {len(buf)}"
            )
        return cls(buf)

    @classmethod
    def from_relaxed(cls, value: object) -> "Address":
        """接受 Address、bytes、hex 字符串（可带 0x）、base58 字符串。"""
        if isinstance(value, Address):
            return value
        if isinstance(value, bytes):
            return cls(value)
        if isinstance(value, str):
            s = value.strip()
            body = s[2:] if len(s) >= 2 and s[:2] in ("0x", "0X") else s
            if len(body) == ADDRESS_HEX_LEN:
                return cls.from_hex(body)
            return cls.from_base58(s)
        raise TypeError(f"unsupported address input type {type(value).__name__}")

    # ---------------------------------------------------------- 输出
    def as_bytes(self) -> bytes:
        return self._bytes

    def to_hex(self) -> str:
        return self._bytes.hex()

    def to_base58(self) -> str:
        return base58.b58encode(self._bytes).decode("ascii")

    def __str__(self) -> str:
        return self.to_base58()

    def __repr__(self) -> str:
        return f"Address({self.to_base58()})"

    def __bytes__(self) -> bytes:
        return self._bytes

    def __eq__(self, other: object) -> bool:
        if isinstance(other, Address):
            return self._bytes == other._bytes
        if isinstance(other, bytes):
            return self._bytes == other
        return NotImplemented

    def __hash__(self) -> int:
        return hash(self._bytes)

    # ---------------------------------------------------------- 编解码
    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.write(self._bytes)

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "Address":
        buf = d.deserialize_fixed_bytes(ADDRESS_RAW_LEN)
        return cls(buf)
