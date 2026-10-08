"""SecretKey. Faithful port of gosdk-develop/crypto/secretkey.go."""

from __future__ import annotations

import binascii
import os
from enum import IntEnum
from typing import Optional, Protocol, runtime_checkable

import base58
import blake3
from coincurve import PrivateKey as _SecpPrivateKey
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from . import _bls, _fndsa
from .error import InvalidSecretKeyError
from .publickey import PublicKey, PublicKeyType
from .signature import Signature, SignatureType

# ClassicalKeySize 32-byte seed length shared by classical curves
CLASSICAL_KEY_SIZE = 32

# FnDsa512KeySize FN-DSA-512 signing key length (1281 bytes)
FN_DSA512_KEY_SIZE = 1281

SECRET_KEY_TYPE_CLASSICAL = 0
SECRET_KEY_TYPE_FN_DSA512 = 1


class SecretKeyType(IntEnum):
    CLASSICAL = 0
    FN_DSA512 = 1


@runtime_checkable
class SecretKeyer(Protocol):
    """Unified secret key interface（Go: SecretKeyer）."""

    def type(self) -> SecretKeyType: ...

    def as_bytes(self) -> bytes: ...

    def to_hex(self) -> str: ...

    def to_base58(self) -> str: ...

    def zeroize(self) -> None: ...

    def sign_for(self, public_key: PublicKey, msg: bytes) -> Signature: ...


class ClassicalSecretKey:
    """Classical key implementation (32-byte seed)."""

    def __init__(self, data: bytes = b"\x00" * CLASSICAL_KEY_SIZE) -> None:
        self.Bytes = bytes(data)

    @property
    def bytes_(self) -> bytes:
        return self.Bytes

    def from_bytes(self, raw: bytes) -> None:
        if len(raw) < CLASSICAL_KEY_SIZE:
            raise InvalidSecretKeyError()
        self.zeroize()
        self.Bytes = bytes(raw[:CLASSICAL_KEY_SIZE])

    def from_string_relaxed(self, s: str) -> None:
        """Parses from hex, Base58, or array format "[1,2,3,...]"."""
        s = s.strip()

        # Try array format [1,2,3,...]
        if s.startswith("[") and s.endswith("]"):
            trimmed = s.strip("[]")
            parts = trimmed.split(",")
            if len(parts) != CLASSICAL_KEY_SIZE:
                raise InvalidSecretKeyError()
            try:
                arr = bytes(int(part.strip()) for part in parts)
            except ValueError:
                raise InvalidSecretKeyError() from None
            return self.from_bytes(arr)

        # Try hex parsing
        try:
            self._from_hex(s)
            return
        except InvalidSecretKeyError:
            pass

        # Try Base58
        return self._from_base58(s)

    def _from_hex(self, s: str) -> None:
        s = s.strip()
        hex_str = s
        if len(s) >= 2 and s[0] == "0" and s[1] in ("x", "X"):
            hex_str = s[2:]
        try:
            buf = binascii.unhexlify(hex_str)
        except (binascii.Error, ValueError):
            raise InvalidSecretKeyError() from None
        self.from_bytes(buf)

    def _from_base58(self, b58_str: str) -> None:
        try:
            data = base58.b58decode(b58_str)
        except ValueError:
            raise InvalidSecretKeyError() from None
        if len(data) != CLASSICAL_KEY_SIZE:
            raise InvalidSecretKeyError()
        self.from_bytes(data)

    # ---- SecretKeyer interface ----

    def type(self) -> SecretKeyType:
        return SecretKeyType.CLASSICAL

    def as_bytes(self) -> bytes:
        return self.Bytes

    def to_hex(self) -> str:
        return self.Bytes.hex()

    def to_base58(self) -> str:
        return base58.b58encode(self.Bytes).decode()

    def __str__(self) -> str:
        return self.to_base58()

    def __repr__(self) -> str:
        return "ClassicalSecretKey(<redacted>)"

    def zeroize(self) -> None:
        self.Bytes = b"\x00" * CLASSICAL_KEY_SIZE

    def sign_for(self, public_key: PublicKey, msg: bytes) -> Signature:
        if public_key.Variant == PublicKeyType.SECP256K1:
            return self.sign_secp256k1(msg)
        if public_key.Variant == PublicKeyType.ED25519:
            return self.sign_ed25519(msg)
        if public_key.Variant == PublicKeyType.BLS12381:
            return self.sign_bls12381(msg)
        raise ValueError(
            "unsupported public key type for classical secret key: "
            f"{int(public_key.Variant)}"
        )

    # ---- Classical key specific methods ----

    def to_secp256k1(self) -> _SecpPrivateKey:
        """Go: crypto.ToECDSA（要求 0 < d < n）。"""
        try:
            return _SecpPrivateKey(self.Bytes)
        except ValueError as exc:
            raise ValueError(
                "invalid secret key: secp256k1 scalar out of range"
            ) from exc

    def to_ed25519(self) -> Ed25519PrivateKey:
        return Ed25519PrivateKey.from_private_bytes(self.Bytes)

    def to_bls12381(self) -> int:
        """Go: blst.KeyGen(seed)。Returns the scalar (int)."""
        return _bls.derive_master_sk(self.Bytes)

    def secp256k1_public(self) -> PublicKey:
        priv = self.to_secp256k1()
        return PublicKey(
            PublicKeyType.SECP256K1, priv.public_key.format(compressed=True)
        )

    def ed25519_public(self) -> PublicKey:
        pub = self.to_ed25519().public_key()
        from cryptography.hazmat.primitives.serialization import (
            Encoding,
            PublicFormat,
        )

        return PublicKey(
            PublicKeyType.ED25519,
            pub.public_bytes(Encoding.Raw, PublicFormat.Raw),
        )

    def bls12381_public(self) -> PublicKey:
        return PublicKey(PublicKeyType.BLS12381, _bls.sk_to_pk(self.to_bls12381()))

    def sign_secp256k1(self, msg: bytes) -> Signature:
        if len(msg) == CLASSICAL_KEY_SIZE:
            msg_hash = msg
        else:
            msg_hash = blake3.blake3(msg).digest()

        priv = self.to_secp256k1()

        signature = bytearray(priv.sign_recoverable(msg_hash, hasher=None))

        if signature[64] in (0, 1):
            signature[64] += 27

        return Signature(SignatureType.SECP256K1, bytes(signature))

    def sign_ed25519(self, msg: bytes) -> Signature:
        return Signature(
            SignatureType.ED25519, self.to_ed25519().sign(msg)
        )

    def sign_bls12381(self, msg: bytes) -> Signature:
        return Signature(
            SignatureType.BLS12381, _bls.sign(self.to_bls12381(), msg)
        )

    def from_ed25519_native(self, priv: Ed25519PrivateKey) -> None:
        """Converts from an ed25519 native private key back to ClassicalSecretKey."""
        if priv is None:
            raise ValueError("ed25519 private key is nil")
        from cryptography.hazmat.primitives.serialization import (
            Encoding,
            PrivateFormat,
            NoEncryption,
        )

        # cryptography 库的 Ed25519PrivateKey 序列化为 32 字节 seed
        # （Go 的 ed25519.PrivateKey 为 64 字节 seed||pub，语义等价取 seed）。
        seed = priv.private_bytes(
            Encoding.Raw, PrivateFormat.Raw, NoEncryption()
        )
        if len(seed) != 32:
            raise ValueError(
                "invalid ed25519 private key length: expected 32-byte seed, "
                f"got {len(seed)}"
            )
        self.zeroize()
        self.Bytes = bytes(seed)

    def from_secp256k1_native(self, priv: _SecpPrivateKey) -> None:
        """Converts from a secp256k1 native private key back to ClassicalSecretKey."""
        if priv is None:
            raise ValueError("private key is nil")
        priv_bytes = priv.secret  # BE, 无前导零保留
        if len(priv_bytes) > CLASSICAL_KEY_SIZE:
            raise ValueError(
                f"private key too long: expected {CLASSICAL_KEY_SIZE}, got "
                f"{len(priv_bytes)}"
            )
        self.zeroize()
        # Copy bytes (right-aligned)
        offset = CLASSICAL_KEY_SIZE - len(priv_bytes)
        self.Bytes = b"\x00" * offset + bytes(priv_bytes)


class FnDsa512SecretKey:
    """FN-DSA-512 key implementation (1281 bytes)."""

    def __init__(self, data: bytes = b"\x00" * FN_DSA512_KEY_SIZE) -> None:
        self.bytes = bytes(data)

    def from_bytes(self, raw: bytes) -> None:
        if len(raw) != FN_DSA512_KEY_SIZE:
            raise InvalidSecretKeyError()
        self.zeroize()
        self.bytes = bytes(raw)

    def from_string_relaxed(self, s: str) -> None:
        """Parses from hex, Base58, or array format "[1,2,3,...]"."""
        s = s.strip()

        if s.startswith("[") and s.endswith("]"):
            trimmed = s.strip("[]")
            parts = trimmed.split(",")
            if len(parts) != FN_DSA512_KEY_SIZE:
                raise InvalidSecretKeyError()
            try:
                arr = bytes(int(part.strip()) for part in parts)
            except ValueError:
                raise InvalidSecretKeyError() from None
            return self.from_bytes(arr)

        try:
            self._from_hex(s)
            return
        except InvalidSecretKeyError:
            pass

        return self._from_base58(s)

    def _from_hex(self, s: str) -> None:
        s = s.strip()
        hex_str = s
        if len(s) >= 2 and s[0] == "0" and s[1] in ("x", "X"):
            hex_str = s[2:]
        try:
            buf = binascii.unhexlify(hex_str)
        except (binascii.Error, ValueError):
            raise InvalidSecretKeyError() from None
        self.from_bytes(buf)

    def _from_base58(self, b58_str: str) -> None:
        try:
            data = base58.b58decode(b58_str)
        except ValueError:
            raise InvalidSecretKeyError() from None
        if len(data) != FN_DSA512_KEY_SIZE:
            raise InvalidSecretKeyError()
        self.from_bytes(data)

    # ---- SecretKeyer interface ----

    def type(self) -> SecretKeyType:
        return SecretKeyType.FN_DSA512

    def as_bytes(self) -> bytes:
        return self.bytes

    def to_hex(self) -> str:
        return self.bytes.hex()

    def to_base58(self) -> str:
        return base58.b58encode(self.bytes).decode()

    def __str__(self) -> str:
        return self.to_base58()

    def __repr__(self) -> str:
        return "FnDsa512SecretKey(<redacted>)"

    def zeroize(self) -> None:
        self.bytes = b"\x00" * FN_DSA512_KEY_SIZE

    def sign_for(self, public_key: PublicKey, msg: bytes) -> Signature:
        if public_key.Variant != PublicKeyType.FN_DSA512:
            raise ValueError(
                "fndsa512 secret key can only sign for fndsa512 public key, "
                f"got: {int(public_key.Variant)}"
            )
        return self.sign_fn_dsa512(msg)

    # ---- FN-DSA-512 specific methods ----

    def sign_fn_dsa512(self, msg: bytes) -> Signature:
        signature = _fndsa.sign(self.bytes, b"", 0, msg)
        return Signature(SignatureType.FN_DSA512, signature)

    def fn_dsa512_public(self) -> PublicKey:
        """TODO: Go version cannot derive the verification key from the signing
        key; a pre-generated public key is required."""
        raise ValueError("not implemented")


def new_classical_secret_key() -> SecretKeyer:
    """Generates a random classical secret key (ensures valid secp256k1 scalar)."""
    while True:
        ret = os.urandom(CLASSICAL_KEY_SIZE)
        # Check whether it is a valid secp256k1 private key.
        try:
            _SecpPrivateKey(ret)
            return ClassicalSecretKey(ret)
        except ValueError:
            continue


def new_pure_classical_secret_key() -> SecretKeyer:
    """Creates a random 32-byte classical seed without secp256k1 scalar validation."""
    ret = os.urandom(CLASSICAL_KEY_SIZE)
    return ClassicalSecretKey(ret)


def new_fn_dsa512_secret_key() -> tuple[SecretKeyer, PublicKey]:
    """Go: NewFnDsa512SecretKey() (SecretKeyer, *PublicKey, error)。"""
    sign_key, verify_key = _fndsa.keygen(LOG_N := 9)
    if len(sign_key) != FN_DSA512_KEY_SIZE:
        raise ValueError(f"unexpected signing key size: {len(sign_key)}")
    return FnDsa512SecretKey(sign_key), PublicKey(
        PublicKeyType.FN_DSA512, verify_key
    )


def secret_keyer_from_bytes(raw: bytes) -> SecretKeyer:
    """Parses by raw byte length: 32 → Classical, 1281 → FnDsa512."""
    if len(raw) == CLASSICAL_KEY_SIZE:
        sk = ClassicalSecretKey()
        sk.from_bytes(raw)
        return sk
    if len(raw) == FN_DSA512_KEY_SIZE:
        sk = FnDsa512SecretKey()
        sk.from_bytes(raw)
        return sk
    raise InvalidSecretKeyError()


def secret_keyer_from_string_relaxed(s: str) -> SecretKeyer:
    """Parses from multiple formats: hex, Base58, or array "[1,2,3,...]"."""
    s = s.strip()

    # Try array format [1,2,3,...]
    if s.startswith("[") and s.endswith("]"):
        trimmed = s.strip("[]")
        parts = trimmed.split(",")

        # Determine length: 32 or 1281
        if len(parts) in (CLASSICAL_KEY_SIZE, FN_DSA512_KEY_SIZE):
            try:
                arr = bytes(int(part.strip()) for part in parts)
            except ValueError:
                raise InvalidSecretKeyError() from None
            return secret_keyer_from_bytes(arr)

        raise InvalidSecretKeyError()

    # Try hex parsing
    try:
        return _new_secret_keyer_from_hex(s)
    except ValueError:
        pass

    # Try Base58
    return _new_secret_keyer_from_base58(s)


def _new_secret_keyer_from_hex(s: str) -> SecretKeyer:
    s = s.strip()
    hex_str = s
    if len(s) >= 2 and s[0] == "0" and s[1] in ("x", "X"):
        hex_str = s[2:]
    try:
        raw = binascii.unhexlify(hex_str)
    except (binascii.Error, ValueError) as exc:
        raise ValueError(exc) from None
    return secret_keyer_from_bytes(raw)


def _new_secret_keyer_from_base58(b58_str: str) -> SecretKeyer:
    try:
        data = base58.b58decode(b58_str)
    except ValueError as exc:
        raise ValueError(exc) from None
    return secret_keyer_from_bytes(data)


def as_classical_secret_key(sk: SecretKeyer) -> Optional[ClassicalSecretKey]:
    if isinstance(sk, ClassicalSecretKey):
        return sk
    return None


def as_fn_dsa512_secret_key(sk: SecretKeyer) -> Optional[FnDsa512SecretKey]:
    if isinstance(sk, FnDsa512SecretKey):
        return sk
    return None
