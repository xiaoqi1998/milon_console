"""Signature. Faithful port of gosdk-develop/crypto/signature.go."""

from __future__ import annotations

import binascii
from enum import IntEnum
from typing import Any

import base58
import blake3
from coincurve import PublicKey as _SecpPublicKey
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
from cryptography.exceptions import InvalidSignature

from ..postcard.deserializer import Deserializer
from ..postcard.serializer import Serializer
from . import _bls, _fndsa
from .error import InvalidSignatureError
from .publickey import PublicKey, PublicKeyType

SIGNATURE_SECP256K1_SIZE = 65
SIGNATURE_ED25519_SIZE = 64
SIGNATURE_BLS12381_SIZE = 96
SIGNATURE_FN_DSA512_SIZE = 666

SIGNATURE_TYPE_SECP256K1 = 0
SIGNATURE_TYPE_ED25519 = 1
SIGNATURE_TYPE_BLS12381 = 2
SIGNATURE_TYPE_FN_DSA512 = 3


class SignatureType(IntEnum):
    SECP256K1 = 0
    ED25519 = 1
    BLS12381 = 2
    FN_DSA512 = 3


def new_signature_from_bytes(raw: bytes) -> "Signature":
    """Creates a Signature from raw bytes, auto-detecting the type by length."""
    if len(raw) == SIGNATURE_ED25519_SIZE:
        return Signature(SignatureType.ED25519, bytes(raw))
    if len(raw) == SIGNATURE_BLS12381_SIZE:
        return Signature(SignatureType.BLS12381, bytes(raw))
    if len(raw) == SIGNATURE_SECP256K1_SIZE:
        return Signature(SignatureType.SECP256K1, bytes(raw))
    if len(raw) == SIGNATURE_FN_DSA512_SIZE:
        return Signature(SignatureType.FN_DSA512, bytes(raw))
    raise InvalidSignatureError()


def new_signature_from_string_relaxed(s: str) -> "Signature":
    """Parses a hex (with optional 0x prefix) or Base58 string."""
    s = s.strip()
    try:
        return _new_signature_from_hex(s)
    except ValueError:
        pass
    return _new_signature_from_base58(s)


def _new_signature_from_hex(s: str) -> "Signature":
    s = s.strip()
    hex_str = s
    if len(s) >= 2 and s[0] == "0" and s[1] in ("x", "X"):
        hex_str = s[2:]
    try:
        buf = binascii.unhexlify(hex_str)
    except (binascii.Error, ValueError) as exc:
        raise ValueError(f"invalid hex string: {exc}") from exc
    return new_signature_from_bytes(buf)


def _new_signature_from_base58(b58_str: str) -> "Signature":
    buf = base58.b58decode(b58_str)
    if len(buf) == 0:
        raise ValueError("invalid base58 string")
    return new_signature_from_bytes(buf)


class Signature:
    def __init__(self, variant: SignatureType = None, data: bytes = None) -> None:
        self.Variant = variant
        self.Bytes = data

    @property
    def variant(self) -> SignatureType:
        return self.Variant

    @property
    def bytes_(self) -> bytes:
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
        return f"Signature(variant={self.Variant}, bytes={self.Bytes!r})"

    def __eq__(self, other: object) -> bool:
        if isinstance(other, Signature):
            return self.Variant == other.Variant and self.Bytes == other.Bytes
        return NotImplemented

    def __hash__(self) -> int:
        return hash((self.Variant, self.Bytes))

    def to_secp256k1(self) -> bytes:
        """Converts to a secp256k1 recoverable signature (R||S||V, V>=27)."""
        if self.Variant != SignatureType.SECP256K1:
            raise ValueError(
                f"not a secp256k1 signature, actual type: {int(self.Variant)}"
            )
        if len(self.Bytes) != SIGNATURE_SECP256K1_SIZE:
            raise ValueError(
                f"invalid secp256k1 signature length: {len(self.Bytes)}"
            )
        return self.Bytes

    def to_ed25519(self) -> bytes:
        if self.Variant != SignatureType.ED25519:
            raise ValueError(
                f"not an ed25519 signature, actual type: {int(self.Variant)}"
            )
        if len(self.Bytes) != SIGNATURE_ED25519_SIZE:
            raise ValueError(
                f"invalid ed25519 signature length: {len(self.Bytes)}"
            )
        return self.Bytes

    def to_bls12381(self) -> bytes:
        if self.Variant != SignatureType.BLS12381:
            raise ValueError(
                f"not a BLS signature, actual type: {int(self.Variant)}"
            )
        if len(self.Bytes) != SIGNATURE_BLS12381_SIZE:
            raise ValueError(
                f"invalid BLS signature length: {len(self.Bytes)}"
            )
        return self.Bytes

    def to_fn_dsa512(self) -> bytes:
        if self.Variant != SignatureType.FN_DSA512:
            raise ValueError(
                f"not a FN-DSA-512 signature, actual type: {int(self.Variant)}"
            )
        if len(self.Bytes) != SIGNATURE_FN_DSA512_SIZE:
            raise ValueError(
                f"invalid FN-DSA-512 signature length: {len(self.Bytes)}"
            )
        return self.Bytes

    def verify(self, msg: bytes, pub_key: PublicKey) -> None:
        """Verifies the signature; raises ValueError on failure."""
        if int(self.Variant) != int(pub_key.Variant):
            raise ValueError(
                f"signature type mismatch: {int(self.Variant)} vs "
                f"{int(pub_key.Variant)}"
            )

        if self.Variant == SignatureType.SECP256K1:
            return _verify_secp256k1(msg, self.Bytes, pub_key)
        if self.Variant == SignatureType.ED25519:
            return _verify_ed25519(msg, self.Bytes, pub_key)
        if self.Variant == SignatureType.BLS12381:
            return _verify_bls12381(msg, self.Bytes, pub_key)
        if self.Variant == SignatureType.FN_DSA512:
            return _verify_fn_dsa512(msg, self.Bytes, pub_key)
        raise InvalidSignatureError()

    # ---- JSON ----
    def to_json_value(self) -> str:
        return self.to_base58()

    def from_json_value(self, value: Any) -> None:
        if not isinstance(value, str):
            raise ValueError("signature JSON must be a string")
        try:
            new_sig = new_signature_from_string_relaxed(value)
        except ValueError as exc:
            raise ValueError(f"invalid signature: {exc}") from exc
        self.Variant = new_sig.Variant
        self.Bytes = new_sig.Bytes

    # ---- Postcard ----
    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u32(int(self.Variant))
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize signature variant: {exc}"
            ) from exc
        serializer.serialize_fixed_bytes(self.Bytes)

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            variant = deserializer.deserialize_u32()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize signature variant: {exc}"
            ) from exc

        try:
            expected_len = {
                SignatureType.SECP256K1: SIGNATURE_SECP256K1_SIZE,
                SignatureType.ED25519: SIGNATURE_ED25519_SIZE,
                SignatureType.BLS12381: SIGNATURE_BLS12381_SIZE,
                SignatureType.FN_DSA512: SIGNATURE_FN_DSA512_SIZE,
            }[SignatureType(variant)]
        except ValueError:
            raise ValueError(f"unknown signature variant: {variant}") from None

        try:
            buf = deserializer.deserialize_fixed_bytes(expected_len)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize signature Bytes: {exc}"
            ) from exc

        try:
            new_sig = new_signature_from_bytes(buf)
        except ValueError as exc:
            raise ValueError(
                f"failed to create signature from Bytes: {exc}"
            ) from exc

        self.Variant = SignatureType(variant)
        self.Bytes = new_sig.Bytes


def _verify_secp256k1(msg: bytes, sig_bytes: bytes, pub_key: PublicKey) -> None:
    # 1. Compute message hash
    if len(msg) == 32:
        msg_hash = msg
    else:
        msg_hash = blake3.blake3(msg).digest()

    # 2. Get public key
    pk = pub_key.to_secp256k1()

    # 3. Verify signature length
    if len(sig_bytes) != SIGNATURE_SECP256K1_SIZE:
        raise ValueError(
            "invalid secp256k1 signature length: expected "
            f"{SIGNATURE_SECP256K1_SIZE}, got {len(sig_bytes)}"
        )

    # 4/5. Verify：coincurve 21 无 raw-64B 验证入口，等价改用
    # 「恢复公钥并比对」——libsecp256k1 recovery 仅接受规范 low-S 签名，
    # 恢复成功且公钥匹配 ⟺ go-ethereum 的 VerifySignature 通过。
    signature_without_v = bytearray(sig_bytes[: len(sig_bytes) - 1])
    recovery_id = sig_bytes[64] - 27 if sig_bytes[64] >= 27 else sig_bytes[64]
    recoverable = bytes(signature_without_v) + bytes([recovery_id])
    try:
        recovered = _SecpPublicKey.from_signature_and_message(
            recoverable, msg_hash, hasher=None
        )
    except Exception as exc:
        raise ValueError("signature verification failed") from exc
    if recovered.format() != pk.format():
        raise ValueError("signature verification failed")


def _verify_ed25519(msg: bytes, sig_bytes: bytes, pub_key: PublicKey) -> None:
    pk_bytes = pub_key.to_ed25519()
    if len(sig_bytes) != SIGNATURE_ED25519_SIZE:
        raise ValueError(
            "invalid ed25519 signature length: expected "
            f"{SIGNATURE_ED25519_SIZE}, got {len(sig_bytes)}"
        )
    try:
        Ed25519PublicKey.from_public_bytes(pk_bytes).verify(sig_bytes, msg)
    except InvalidSignature as exc:
        raise ValueError("signature verification failed") from exc


def _verify_bls12381(msg: bytes, sig_bytes: bytes, pub_key: PublicKey) -> None:
    # 1. Get public key
    pub_key.to_bls12381()

    # 2. Verify signature length
    if len(sig_bytes) != SIGNATURE_BLS12381_SIZE:
        raise ValueError(
            "invalid BLS signature length: expected "
            f"{SIGNATURE_BLS12381_SIZE}, got {len(sig_bytes)}"
        )

    # 3/4. Recover signature and verify
    if not _bls.verify(pub_key.Bytes, msg, sig_bytes):
        raise ValueError("signature verification failed")


def _verify_fn_dsa512(msg: bytes, sig_bytes: bytes, key: PublicKey) -> None:
    if not _fndsa.verify(key.Bytes, b"", 0, msg, sig_bytes):
        raise ValueError("signature verification failed")


def verify_batch(
    sigs: list, msgs: list, pub_keys: list
) -> None:
    """Batch-verifies multiple signatures.

    Go 版用 ed25519consensus 批量验证 Ed25519；Python 等价为逐条验证
    （接受/拒绝语义一致）。混合或非 Ed25519 类型同样逐条验证。
    """
    if len(sigs) != len(msgs) or len(msgs) != len(pub_keys):
        raise ValueError(
            f"length mismatch: sigs={len(sigs)}, msgs={len(msgs)}, "
            f"pubkeys={len(pub_keys)}"
        )

    if len(sigs) == 0:
        return

    # Check whether all signatures are Ed25519 type
    all_ed25519 = all(sig.Variant == SignatureType.ED25519 for sig in sigs)

    if all_ed25519:
        for i, sig in enumerate(sigs):
            try:
                ed_sig = sig.to_ed25519()
                ed_pub = pub_keys[i].to_ed25519()
            except ValueError as exc:
                raise ValueError(
                    f"failed to convert signature at index {i}: {exc}"
                ) from exc
            try:
                Ed25519PublicKey.from_public_bytes(ed_pub).verify(
                    ed_sig, msgs[i]
                )
            except InvalidSignature as exc:
                raise ValueError(
                    "batch verification failed: one or more Ed25519 "
                    "signatures are invalid"
                ) from exc
        return

    # Mixed or non-Ed25519 types: verify individually
    for i, sig in enumerate(sigs):
        sig.verify(msgs[i], pub_keys[i])
