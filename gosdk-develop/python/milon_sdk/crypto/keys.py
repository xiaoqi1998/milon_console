"""公钥类型（对应 Go crypto/publickey.go）。

公钥形态：Variant(u32 变长) + 定长字节。按长度自动识别曲线：
    Secp256k1=33 / Ed25519=32 / BLS12381=48 / FnDsa512=897
"""
from __future__ import annotations

from enum import IntEnum

import base58  # type: ignore

from ..postcard import Deserializer, Serializer
from .errors import InvalidPublicKeyError

PUBLIC_KEY_SECP256K1_SIZE = 33
PUBLIC_KEY_ED25519_SIZE = 32
PUBLIC_KEY_BLS12381_SIZE = 48
PUBLIC_KEY_FNDSA512_SIZE = 897

PUBLIC_KEY_SIZES = {
    PUBLIC_KEY_SECP256K1_SIZE,
    PUBLIC_KEY_ED25519_SIZE,
    PUBLIC_KEY_BLS12381_SIZE,
    PUBLIC_KEY_FNDSA512_SIZE,
}


class PublicKeyType(IntEnum):
    SECP256K1 = 0
    ED25519 = 1
    BLS12381 = 2
    FNDSA512 = 3


def _decode_hex(s: str) -> bytes:
    body = s[2:] if len(s) >= 2 and s[:2] in ("0x", "0X") else s
    return bytes.fromhex(body)


class PublicKey:
    __slots__ = ("variant", "bytes")

    def __init__(self, variant: PublicKeyType, raw: bytes):
        self.variant = PublicKeyType(variant)
        self.bytes = bytes(raw)

    # ---------------------------------------------------------- 构造
    @classmethod
    def from_bytes(cls, raw: bytes) -> "PublicKey":
        """按长度自动识别曲线类型。"""
        n = len(raw)
        if n == PUBLIC_KEY_SECP256K1_SIZE:
            return cls(PublicKeyType.SECP256K1, raw)
        if n == PUBLIC_KEY_ED25519_SIZE:
            return cls(PublicKeyType.ED25519, raw)
        if n == PUBLIC_KEY_BLS12381_SIZE:
            return cls(PublicKeyType.BLS12381, raw)
        if n == PUBLIC_KEY_FNDSA512_SIZE:
            return cls(PublicKeyType.FNDSA512, raw)
        raise InvalidPublicKeyError(f"invalid public key length: {n}")

    @classmethod
    def from_string_relaxed(cls, s: str) -> "PublicKey":
        s = s.strip()
        try:
            return cls.from_bytes(_decode_hex(s))
        except ValueError:
            buf = base58.b58decode(s)
            if not buf:
                raise InvalidPublicKeyError("invalid base58 string")
            return cls.from_bytes(buf)

    @classmethod
    def from_relaxed(cls, value: object) -> "PublicKey":
        if isinstance(value, PublicKey):
            return value
        if isinstance(value, bytes):
            return cls.from_bytes(value)
        if isinstance(value, str):
            return cls.from_string_relaxed(value)
        raise TypeError(f"invalid type for PublicKey: {type(value).__name__}")

    # ---------------------------------------------------------- 输出
    def as_bytes(self) -> bytes:
        return self.bytes

    def to_hex(self) -> str:
        return self.bytes.hex()

    def to_base58(self) -> str:
        return base58.b58encode(self.bytes).decode("ascii")

    def __str__(self) -> str:
        return self.to_base58()

    def __repr__(self) -> str:
        return f"PublicKey({self.variant.name}, {self.to_base58()})"

    def __bytes__(self) -> bytes:
        return self.bytes

    def __eq__(self, other: object) -> bool:
        if isinstance(other, PublicKey):
            return self.variant == other.variant and self.bytes == other.bytes
        return NotImplemented

    def __hash__(self) -> int:
        return hash((self.variant, self.bytes))

    # ---------------------------------------------------------- 判别
    def is_secp256k1(self) -> bool:
        return self.variant == PublicKeyType.SECP256K1

    def is_ed25519(self) -> bool:
        return self.variant == PublicKeyType.ED25519

    def is_bls12381(self) -> bool:
        return self.variant == PublicKeyType.BLS12381

    def is_fndsa512(self) -> bool:
        return self.variant == PublicKeyType.FNDSA512

    # ---------------------------------------------------------- 编解码
    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_u32(int(self.variant))
        serializer.write(self.bytes)

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "PublicKey":
        variant = d.deserialize_u32()
        try:
            pk_type = PublicKeyType(variant)
        except ValueError:
            raise InvalidPublicKeyError(f"unknown public key variant: {variant}")
        expected_len = {
            PublicKeyType.SECP256K1: PUBLIC_KEY_SECP256K1_SIZE,
            PublicKeyType.ED25519: PUBLIC_KEY_ED25519_SIZE,
            PublicKeyType.BLS12381: PUBLIC_KEY_BLS12381_SIZE,
            PublicKeyType.FNDSA512: PUBLIC_KEY_FNDSA512_SIZE,
        }[pk_type]
        buf = d.deserialize_fixed_bytes(expected_len)
        return cls(pk_type, buf)
