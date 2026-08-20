"""签名类型与验签（对应 Go crypto/signature.go）。

线上签名长度：Ed25519=64 / Secp256k1=65 / BLS12381=96 / FnDsa512=666。
Postcard 形态：Variant(u32 变长) + 定长字节。
"""
from __future__ import annotations

from enum import IntEnum

import base58  # type: ignore

from ..postcard import Deserializer, Serializer
from . import _bls
from .errors import (
    InvalidPublicKeyError,
    InvalidSignatureError,
    SignatureVerificationError,
)
from .keys import PublicKey, PublicKeyType

SIGNATURE_SECP256K1_SIZE = 65
SIGNATURE_ED25519_SIZE = 64
SIGNATURE_BLS12381_SIZE = 96
SIGNATURE_FNDSA512_SIZE = 666


class SignatureType(IntEnum):
    SECP256K1 = 0
    ED25519 = 1
    BLS12381 = 2
    FNDSA512 = 3


def _signature_size_for(variant: SignatureType) -> int:
    return {
        SignatureType.SECP256K1: SIGNATURE_SECP256K1_SIZE,
        SignatureType.ED25519: SIGNATURE_ED25519_SIZE,
        SignatureType.BLS12381: SIGNATURE_BLS12381_SIZE,
        SignatureType.FNDSA512: SIGNATURE_FNDSA512_SIZE,
    }[SignatureType(variant)]


class Signature:
    __slots__ = ("variant", "bytes")

    def __init__(self, variant: SignatureType, raw: bytes):
        self.variant = SignatureType(variant)
        self.bytes = bytes(raw)

    # ---------------------------------------------------------- 构造
    @classmethod
    def from_bytes(cls, raw: bytes) -> "Signature":
        """按长度自动识别类型。"""
        n = len(raw)
        for st, size in (
            (SignatureType.ED25519, SIGNATURE_ED25519_SIZE),
            (SignatureType.BLS12381, SIGNATURE_BLS12381_SIZE),
            (SignatureType.SECP256K1, SIGNATURE_SECP256K1_SIZE),
            (SignatureType.FNDSA512, SIGNATURE_FNDSA512_SIZE),
        ):
            if n == size:
                return cls(st, raw)
        raise InvalidSignatureError(f"invalid signature length: {n}")

    @classmethod
    def from_string_relaxed(cls, s: str) -> "Signature":
        s = s.strip()
        body = s[2:] if len(s) >= 2 and s[:2] in ("0x", "0X") else s
        try:
            raw = bytes.fromhex(body)
        except ValueError:
            raw = base58.b58decode(s)
            if not raw:
                raise InvalidSignatureError("invalid base58 string")
        return cls.from_bytes(raw)

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
        return f"Signature({self.variant.name}, {self.to_base58()})"

    def __bytes__(self) -> bytes:
        return self.bytes

    def __eq__(self, other: object) -> bool:
        if isinstance(other, Signature):
            return self.variant == other.variant and self.bytes == other.bytes
        return NotImplemented

    def __hash__(self) -> int:
        return hash((self.variant, self.bytes))

    # ---------------------------------------------------------- 验签
    def verify(self, msg: bytes, pub_key: PublicKey) -> bool:
        if self.variant != pub_key.variant:
            raise InvalidSignatureError(
                f"signature type mismatch: {int(self.variant)} vs {int(pub_key.variant)}"
            )
        if self.variant == SignatureType.SECP256K1:
            return _verify_secp256k1(msg, self.bytes, pub_key)
        if self.variant == SignatureType.ED25519:
            return _verify_ed25519(msg, self.bytes, pub_key)
        if self.variant == SignatureType.BLS12381:
            return _verify_bls12381(msg, self.bytes, pub_key)
        if self.variant == SignatureType.FNDSA512:
            from .fn_dsa512 import verify512

            return verify512(pub_key.as_bytes(), self.bytes, msg)
        return False

    # ---------------------------------------------------------- 编解码
    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_u32(int(self.variant))
        serializer.write(self.bytes)

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "Signature":
        variant = d.deserialize_u32()
        try:
            st = SignatureType(variant)
        except ValueError:
            raise InvalidSignatureError(f"unknown signature variant: {variant}")
        raw = d.deserialize_fixed_bytes(_signature_size_for(st))
        return cls(st, raw)


def _hash_msg_for_secp(msg: bytes) -> bytes:
    """32 字节直通，否则 BLAKE3（与 Go verifySecp256k1 一致）。"""
    if len(msg) == 32:
        return msg
    import blake3  # type: ignore

    return blake3.blake3(msg).digest()  # type: ignore[attr-defined]


def _compact_to_der(sig: bytes) -> bytes:
    """把 64 字节紧凑 R‖S 签名转为 DER（coincurve 的 verify 只接受 DER）。

    对应 Go ethcrypto.VerifySignature 直接吃 R‖S（无 DER 包装）。
    """
    r = int.from_bytes(sig[:32], "big")
    s = int.from_bytes(sig[32:], "big")

    def _int_der(n: int) -> bytes:
        b = n.to_bytes((n.bit_length() + 7) // 8 or 1, "big")
        if b[0] & 0x80:
            b = b"\x00" + b
        return b"\x02" + bytes([len(b)]) + b

    body = _int_der(r) + _int_der(s)
    return b"\x30" + bytes([len(body)]) + body


def _verify_secp256k1(msg: bytes, sig_bytes: bytes, pub_key: PublicKey) -> bool:
    if len(sig_bytes) != SIGNATURE_SECP256K1_SIZE:
        raise InvalidSignatureError(
            f"invalid secp256k1 signature length: expected {SIGNATURE_SECP256K1_SIZE}, "
            f"got {len(sig_bytes)}"
        )
    if not pub_key.is_secp256k1():
        raise InvalidPublicKeyError("not a secp256k1 public key")
    try:
        import coincurve  # type: ignore

        pk = coincurve.PublicKey(pub_key.as_bytes())
        return pk.verify(
            _compact_to_der(sig_bytes[:64]), _hash_msg_for_secp(msg), hasher=None
        )
    except ImportError as exc:  # pragma: no cover
        raise ImportError(f"Secp256k1 验签需要 'coincurve': {exc}") from exc


def _verify_ed25519(msg: bytes, sig_bytes: bytes, pub_key: PublicKey) -> bool:
    if len(sig_bytes) != SIGNATURE_ED25519_SIZE:
        raise InvalidSignatureError(
            f"invalid ed25519 signature length: expected {SIGNATURE_ED25519_SIZE}, "
            f"got {len(sig_bytes)}"
        )
    if not pub_key.is_ed25519():
        raise InvalidPublicKeyError("not an ed25519 public key")
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

    try:
        pk = Ed25519PublicKey.from_public_bytes(pub_key.as_bytes())
        pk.verify(sig_bytes, msg)
        return True
    except Exception:
        return False


def _verify_bls12381(msg: bytes, sig_bytes: bytes, pub_key: PublicKey) -> bool:
    if len(sig_bytes) != SIGNATURE_BLS12381_SIZE:
        raise InvalidSignatureError(
            f"invalid BLS signature length: expected {SIGNATURE_BLS12381_SIZE}, "
            f"got {len(sig_bytes)}"
        )
    if not pub_key.is_bls12381():
        raise InvalidPublicKeyError("not a BLS12-381 public key")
    return _bls.bls_verify(pub_key.as_bytes(), sig_bytes, msg)


def verify_batch(
    sigs: list[Signature],
    msgs: list[bytes],
    pub_keys: list[PublicKey],
) -> bool:
    """批量验签：全部为 Ed25519 时逐条验证（Python 无批量加速，语义等价）。"""
    if not (len(sigs) == len(msgs) == len(pub_keys)):
        raise ValueError(
            f"length mismatch: sigs={len(sigs)}, msgs={len(msgs)}, pubkeys={len(pub_keys)}"
        )
    if not sigs:
        return True
    for sig, msg, pk in zip(sigs, msgs, pub_keys):
        if not sig.verify(msg, pk):
            return False
    return True
