"""PublicKey. Faithful port of gosdk-develop/crypto/publickey.go."""

from __future__ import annotations

import binascii
from enum import IntEnum
from typing import Any

import base58
from coincurve import PublicKey as _SecpPublicKey

from ..postcard.deserializer import Deserializer
from ..postcard.serializer import Serializer
from . import _bls
from .error import InvalidPublicKeyError

PUBLIC_KEY_SECP256K1_SIZE = 33
PUBLIC_KEY_ED25519_SIZE = 32
PUBLIC_KEY_BLS12381_SIZE = 48
PUBLIC_KEY_FN_DSA512_SIZE = 897

# 模块级别名（CONVENTIONS：枚举成员 UPPER_SNAKE + 模块级别名）
PUBLIC_KEY_TYPE_SECP256K1 = 0
PUBLIC_KEY_TYPE_ED25519 = 1
PUBLIC_KEY_TYPE_BLS12381 = 2
PUBLIC_KEY_TYPE_FN_DSA512 = 3


class PublicKeyType(IntEnum):
    SECP256K1 = 0
    ED25519 = 1
    BLS12381 = 2
    FN_DSA512 = 3


def new_public_key_from_bytes(raw: bytes) -> "PublicKey":
    """Parses raw bytes into a PublicKey based on length and curve rules."""
    if len(raw) == PUBLIC_KEY_SECP256K1_SIZE:
        try:
            _SecpPublicKey(bytes(raw))
        except Exception as exc:
            raise ValueError(f"invalid secp256k1 public key: {exc}") from exc
        return PublicKey(PublicKeyType.SECP256K1, bytes(raw))
    if len(raw) == PUBLIC_KEY_ED25519_SIZE:
        return PublicKey(PublicKeyType.ED25519, bytes(raw))
    if len(raw) == PUBLIC_KEY_BLS12381_SIZE:
        try:
            _bls.pubkey_to_G1(raw)
        except Exception as exc:
            raise ValueError(f"invalid BLS public key: {exc}") from exc
        return PublicKey(PublicKeyType.BLS12381, bytes(raw))
    if len(raw) == PUBLIC_KEY_FN_DSA512_SIZE:
        return PublicKey(PublicKeyType.FN_DSA512, bytes(raw))
    raise InvalidPublicKeyError()


def new_public_key_from_string_relaxed(s: str) -> "PublicKey":
    s = s.strip()
    try:
        return _new_public_key_from_hex(s)
    except ValueError:
        pass
    return _new_public_key_from_base58(s)


def _new_public_key_from_hex(s: str) -> "PublicKey":
    s = s.strip()
    hex_str = s
    if len(s) >= 2 and s[0] == "0" and s[1] in ("x", "X"):
        hex_str = s[2:]
    try:
        buf = binascii.unhexlify(hex_str)
    except (binascii.Error, ValueError) as exc:
        raise ValueError(f"invalid hex string: {exc}") from exc
    return new_public_key_from_bytes(buf)


def _new_public_key_from_base58(b58_str: str) -> "PublicKey":
    buf = base58.b58decode(b58_str)
    if len(buf) == 0:
        raise ValueError("invalid base58 string")
    return new_public_key_from_bytes(buf)


class PublicKey:
    def __init__(self, variant: PublicKeyType = None, data: bytes = None) -> None:
        self.Variant = variant
        self.Bytes = data

    # ---- 字段访问（CONVENTIONS：字段 snake_case 别名） ----
    @property
    def variant(self) -> PublicKeyType:
        return self.Variant

    @property
    def bytes_(self) -> bytes:
        """Go 字段名 Bytes 的 snake 访问器（`bytes` 是内建名，避开冲突）。"""
        return self.Bytes

    def as_bytes(self) -> bytes:
        return self.Bytes

    def to_hex(self) -> str:
        return self.Bytes.hex()

    def to_base58(self) -> str:
        return base58.b58encode(self.Bytes).decode()

    def __str__(self) -> str:
        return self.to_base58()

    def __repr__(self) -> str:
        return f"PublicKey(variant={self.Variant}, bytes={self.Bytes!r})"

    def __eq__(self, other: object) -> bool:
        if isinstance(other, PublicKey):
            return self.Variant == other.Variant and self.Bytes == other.Bytes
        return NotImplemented

    def __hash__(self) -> int:
        return hash((self.Variant, self.Bytes))

    def to_secp256k1(self) -> _SecpPublicKey:
        if self.Variant != PublicKeyType.SECP256K1:
            raise InvalidPublicKeyError()
        return _SecpPublicKey(self.Bytes)

    def to_ed25519(self) -> bytes:
        if self.Variant != PublicKeyType.ED25519:
            raise ValueError(
                f"not an ed25519 public key, actual type: {int(self.Variant)}"
            )
        if len(self.Bytes) != PUBLIC_KEY_ED25519_SIZE:
            raise ValueError(
                "invalid ed25519 public key length: expected "
                f"{PUBLIC_KEY_ED25519_SIZE}, got {len(self.Bytes)}"
            )
        return self.Bytes

    def to_bls12381(self):
        """Returns a py_ecc G1 仿射点（Go: blst.P1Affine）。"""
        if self.Variant != PublicKeyType.BLS12381:
            raise ValueError(
                f"not an BLS12-381 public key, actual type: {int(self.Variant)}"
            )
        if len(self.Bytes) != PUBLIC_KEY_BLS12381_SIZE:
            raise ValueError(
                "invalid BLS12-381 public key length: expected "
                f"{PUBLIC_KEY_BLS12381_SIZE}, got {len(self.Bytes)}"
            )
        try:
            return _bls.uncompress_g1(self.Bytes)
        except Exception as exc:
            raise ValueError(
                "failed to parse BLS12-381 public key from Bytes"
            ) from exc

    def is_secp256k1(self) -> bool:
        return self.Variant == PublicKeyType.SECP256K1

    def is_ed25519(self) -> bool:
        return self.Variant == PublicKeyType.ED25519

    def is_bls12381(self) -> bool:
        return self.Variant == PublicKeyType.BLS12381

    def is_fn_dsa512(self) -> bool:
        return self.Variant == PublicKeyType.FN_DSA512

    def from_secp256k1_native(self, p: _SecpPublicKey) -> None:
        """Converts from a secp256k1 public key back to PublicKey."""
        if p is None:
            raise ValueError("secp256k1 public key is nil")
        compressed = p.format(compressed=True)
        if len(compressed) != PUBLIC_KEY_SECP256K1_SIZE:
            raise ValueError(
                "invalid secp256k1 public key length: expected "
                f"{PUBLIC_KEY_SECP256K1_SIZE}, got {len(compressed)}"
            )
        self.Variant = PublicKeyType.SECP256K1
        self.Bytes = compressed

    def from_ed25519_native(self, vk: bytes) -> None:
        """Converts from an ed25519 public key back to PublicKey."""
        if len(vk) != PUBLIC_KEY_ED25519_SIZE:
            raise ValueError(
                "invalid ed25519 public key length: expected "
                f"{PUBLIC_KEY_ED25519_SIZE}, got {len(vk)}"
            )
        self.Variant = PublicKeyType.ED25519
        self.Bytes = bytes(vk)

    def from_bls12381_native(self, p1_affine: Any) -> None:
        """Converts from a BLS12-381 public key (py_ecc G1 点) back to PublicKey."""
        compressed = _bls.compress_g1(p1_affine)
        if len(compressed) != PUBLIC_KEY_BLS12381_SIZE:
            raise ValueError(
                "invalid BLS12-381 public key length: expected "
                f"{PUBLIC_KEY_BLS12381_SIZE}, got {len(compressed)}"
            )
        self.Variant = PublicKeyType.BLS12381
        self.Bytes = compressed

    # ---- JSON ----
    def to_json_value(self) -> str:
        return self.to_base58()

    def from_json_value(self, value: Any) -> None:
        if not isinstance(value, str):
            raise ValueError("public key JSON must be a string")
        try:
            new_pk = new_public_key_from_string_relaxed(value)
        except ValueError as exc:
            raise ValueError(f"failed to parse public key: {exc}") from exc
        self.Variant = new_pk.Variant
        self.Bytes = new_pk.Bytes

    # ---- Postcard ----
    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u32(int(self.Variant))
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize public key variant: {exc}"
            ) from exc
        serializer.serialize_fixed_bytes(self.Bytes)

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            variant = deserializer.deserialize_u32()
        except ValueError as exc:
            raise ValueError(
                "failed to deserialize public key variant: "
                f"{exc}"
            ) from exc

        try:
            expected_len = {
                PublicKeyType.SECP256K1: PUBLIC_KEY_SECP256K1_SIZE,
                PublicKeyType.ED25519: PUBLIC_KEY_ED25519_SIZE,
                PublicKeyType.BLS12381: PUBLIC_KEY_BLS12381_SIZE,
                PublicKeyType.FN_DSA512: PUBLIC_KEY_FN_DSA512_SIZE,
            }[PublicKeyType(variant)]
        except ValueError:
            raise ValueError(f"unknown public key variant: {variant}") from None

        try:
            buf = deserializer.deserialize_fixed_bytes(expected_len)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize public key Bytes: {exc}"
            ) from exc

        try:
            new_pk = new_public_key_from_bytes(buf)
        except ValueError as exc:
            raise ValueError(
                f"failed to create public key from Bytes: {exc}"
            ) from exc

        self.Variant = PublicKeyType(variant)
        self.Bytes = new_pk.Bytes
