"""私钥类型（对应 Go crypto/secretkey.go）。

- ClassicalSecretKey：32 字节种子，可派生 Secp256k1 / Ed25519 / BLS12381 公钥并签名。
- FnDsa512SecretKey：1281 字节签名密钥（后量子，见 fn_dsa512.py 的门控说明）。
"""
from __future__ import annotations

import secrets
from enum import IntEnum
from typing import TYPE_CHECKING, Optional, Protocol

import base58  # type: ignore

from ..postcard import Deserializer, Serializer
from .errors import InvalidSecretKeyError
from .fn_dsa512 import FNDSA512_SIGN_KEY_LEN, _validate_sign_key
from .keys import PublicKey, PublicKeyType
from .signature import Signature, SignatureType

if TYPE_CHECKING:  # pragma: no cover
    from . import _BLS_BACKEND  # noqa: F401

CLASSICAL_KEY_SIZE = 32


class SecretKeyType(IntEnum):
    CLASSICAL = 0
    FNDSA512 = 1


class SecretKeyer(Protocol):
    """统一私钥接口（对应 Go SecretKeyer）。"""

    def type(self) -> SecretKeyType: ...
    def as_bytes(self) -> bytes: ...
    def to_hex(self) -> str: ...
    def to_base58(self) -> str: ...
    def zeroize(self) -> None: ...
    def sign_for(self, public_key: PublicKey, msg: bytes) -> Signature: ...


# ================================================================
# Classical（32 字节种子）
# ================================================================

# BLS 后端惰性加载（优先 blst，回退 py_ecc）——见 crypto/_bls.py
def _load_bls():
    from . import _bls  # 惰性导入避免循环依赖

    return _bls


class ClassicalSecretKey:
    __slots__ = ("_bytes",)

    def __init__(self, raw: bytes):
        if len(raw) < CLASSICAL_KEY_SIZE:
            raise InvalidSecretKeyError(
                f"classical secret key requires at least {CLASSICAL_KEY_SIZE} bytes, got {len(raw)}"
            )
        self._bytes = bytes(raw[:CLASSICAL_KEY_SIZE])

    # ---------------------------------------------------------- 构造
    @classmethod
    def generate(cls, *, validate_secp256k1: bool = True) -> "ClassicalSecretKey":
        """随机生成 32 字节密钥。

        validate_secp256k1=True（默认，对应 Go NewClassicalSecretKey）时保证种子是
        合法的 secp256k1 标量（0 < d < n）；False 对应 NewPureClassicalSecretKey。
        """
        while True:
            raw = secrets.token_bytes(CLASSICAL_KEY_SIZE)
            if not validate_secp256k1:
                return cls(raw)
            try:
                _require_coincurve().PrivateKey(raw)
                return cls(raw)
            except Exception:
                continue

    @classmethod
    def from_bytes(cls, raw: bytes) -> "ClassicalSecretKey":
        return cls(raw)

    @classmethod
    def from_string_relaxed(cls, s: str) -> "ClassicalSecretKey":
        s = s.strip()
        # 数组格式 [1,2,3,...]
        if s.startswith("[") and s.endswith("]"):
            parts = [p.strip() for p in s[1:-1].split(",")]
            if len(parts) != CLASSICAL_KEY_SIZE:
                raise InvalidSecretKeyError("invalid array length")
            return cls(bytes(int(p) for p in parts))
        # hex（可带 0x）
        body = s[2:] if len(s) >= 2 and s[:2] in ("0x", "0X") else s
        try:
            return cls(bytes.fromhex(body))
        except ValueError:
            pass
        # base58
        buf = base58.b58decode(s)
        if len(buf) != CLASSICAL_KEY_SIZE:
            raise InvalidSecretKeyError(
                f"invalid base58 decoded length: expected {CLASSICAL_KEY_SIZE}, got {len(buf)}"
            )
        return cls(buf)

    @classmethod
    def from_ed25519_private(cls, priv: bytes) -> "ClassicalSecretKey":
        """从 64 字节 ed25519 私钥提取前 32 字节种子（对应 FromEd25519Native）。"""
        if len(priv) != 64:
            raise InvalidSecretKeyError(
                f"ed25519 private key must be 64 bytes, got {len(priv)}"
            )
        return cls(priv[:32])

    @classmethod
    def from_secp256k1_scalar(cls, scalar: bytes) -> "ClassicalSecretKey":
        """从 secp256k1 私钥标量右对齐到 32 字节（对应 FromSecp256k1Native）。"""
        if len(scalar) > CLASSICAL_KEY_SIZE:
            raise InvalidSecretKeyError("private key too long")
        return cls(b"\x00" * (CLASSICAL_KEY_SIZE - len(scalar)) + scalar)

    # ---------------------------------------------------------- 接口
    def type(self) -> SecretKeyType:
        return SecretKeyType.CLASSICAL

    def as_bytes(self) -> bytes:
        return self._bytes

    def to_hex(self) -> str:
        return self._bytes.hex()

    def to_base58(self) -> str:
        return base58.b58encode(self._bytes).decode("ascii")

    def __str__(self) -> str:
        return self.to_base58()

    def __repr__(self) -> str:
        return f"ClassicalSecretKey({self.to_base58()})"

    def zeroize(self) -> None:
        self._bytes = b"\x00" * CLASSICAL_KEY_SIZE

    # ---------------------------------------------------------- 派生
    def secp256k1_public(self) -> PublicKey:
        cc = _require_coincurve()
        priv = cc.PrivateKey(self._bytes)
        return PublicKey(PublicKeyType.SECP256K1, priv.public_key.format(compressed=True))

    def ed25519_public(self) -> PublicKey:
        from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

        priv = Ed25519PrivateKey.from_private_bytes(self._bytes)
        pub = priv.public_key().public_bytes_raw()
        return PublicKey(PublicKeyType.ED25519, pub)

    def bls12381_public(self) -> PublicKey:
        raw = _load_bls().bls_public_from_seed(self._bytes)
        return PublicKey(PublicKeyType.BLS12381, raw)

    # ---------------------------------------------------------- 签名
    def sign_for(self, public_key: PublicKey, msg: bytes) -> Signature:
        if public_key.variant == PublicKeyType.SECP256K1:
            return self.sign_secp256k1(msg)
        if public_key.variant == PublicKeyType.ED25519:
            return self.sign_ed25519(msg)
        if public_key.variant == PublicKeyType.BLS12381:
            return self.sign_bls12381(msg)
        raise InvalidSecretKeyError(
            f"unsupported public key type for classical secret key: {int(public_key.variant)}"
        )

    def sign_secp256k1(self, msg: bytes) -> Signature:
        msg_hash = msg if len(msg) == 32 else blake3_sum(msg)
        cc = _require_coincurve()
        priv = cc.PrivateKey(self._bytes)
        sig = priv.sign_recoverable(msg_hash, hasher=None)
        # sig 65 字节：R(32) + S(32) + recovery_id(0..3)。
        # 以太坊风格可恢复签名：取 y 奇偶位（bit0）→ 27/28，与 Go 一致。
        v = (sig[64] & 1) + 27
        return Signature(SignatureType.SECP256K1, sig[:64] + bytes([v]))

    def sign_ed25519(self, msg: bytes) -> Signature:
        from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

        priv = Ed25519PrivateKey.from_private_bytes(self._bytes)
        return Signature(SignatureType.ED25519, priv.sign(msg))

    def sign_bls12381(self, msg: bytes) -> Signature:
        return Signature(SignatureType.BLS12381, _load_bls().bls_sign(self._bytes, msg))


def _require_coincurve():
    try:
        import coincurve  # type: ignore

        return coincurve
    except ImportError as exc:  # pragma: no cover
        raise ImportError(
            "Secp256k1 签名需要 'coincurve'（libsecp256k1 绑定）。"
            f"原始错误: {exc}"
        ) from exc


def blake3_sum(data: bytes) -> bytes:
    import blake3  # type: ignore

    return blake3.blake3(data).digest()  # type: ignore[attr-defined]


# ================================================================
# FnDsa512（1281 字节）
# ================================================================


class FnDsa512SecretKey:
    __slots__ = ("_bytes",)

    def __init__(self, raw: bytes):
        self._bytes = _validate_sign_key(raw)

    @classmethod
    def from_bytes(cls, raw: bytes) -> "FnDsa512SecretKey":
        return cls(raw)

    @classmethod
    def from_string_relaxed(cls, s: str) -> "FnDsa512SecretKey":
        s = s.strip()
        if s.startswith("[") and s.endswith("]"):
            parts = [p.strip() for p in s[1:-1].split(",")]
            if len(parts) != FNDSA512_SIGN_KEY_LEN:
                raise InvalidSecretKeyError("invalid array length")
            return cls(bytes(int(p) for p in parts))
        body = s[2:] if len(s) >= 2 and s[:2] in ("0x", "0X") else s
        try:
            return cls(bytes.fromhex(body))
        except ValueError:
            pass
        buf = base58.b58decode(s)
        if len(buf) != FNDSA512_SIGN_KEY_LEN:
            raise InvalidSecretKeyError(
                f"invalid base58 decoded length: expected {FNDSA512_SIGN_KEY_LEN}, got {len(buf)}"
            )
        return cls(buf)

    def type(self) -> SecretKeyType:
        return SecretKeyType.FNDSA512

    def as_bytes(self) -> bytes:
        return self._bytes

    def to_hex(self) -> str:
        return self._bytes.hex()

    def to_base58(self) -> str:
        return base58.b58encode(self._bytes).decode("ascii")

    def __str__(self) -> str:
        return self.to_base58()

    def zeroize(self) -> None:
        self._bytes = b"\x00" * FNDSA512_SIGN_KEY_LEN

    def sign_for(self, public_key: PublicKey, msg: bytes) -> Signature:
        if public_key.variant != PublicKeyType.FNDSA512:
            raise InvalidSecretKeyError(
                f"fndsa512 secret key can only sign for fndsa512 public key, got: {int(public_key.variant)}"
            )
        from .fn_dsa512 import sign512

        return Signature(SignatureType.FNDSA512, sign512(self._bytes, msg))

    # Go 侧亦未实现：无法从签名密钥推导验证密钥，公钥需预生成传入
    def fndsa512_public(self) -> PublicKey:
        raise NotImplementedError(
            "Go 版本同样未实现 FnDsa512Public()：验证密钥无法从签名密钥推导，"
            "请使用预生成的公钥（keygen512 产出的 verify key）。"
        )


# ================================================================
# 统一解析入口
# ================================================================


def secret_keyer_from_bytes(raw: bytes) -> SecretKeyer:
    """按长度解析：32 → Classical，1281 → FnDsa512。"""
    if len(raw) == CLASSICAL_KEY_SIZE:
        return ClassicalSecretKey(raw)
    if len(raw) == FNDSA512_SIGN_KEY_LEN:
        return FnDsa512SecretKey(raw)
    raise InvalidSecretKeyError(f"unsupported secret key length {len(raw)}")


def secret_keyer_from_string_relaxed(s: str) -> SecretKeyer:
    """按长度自动识别：hex / base58 / 数组格式均可。"""
    s = s.strip()
    if s.startswith("[") and s.endswith("]"):
        parts = [p.strip() for p in s[1:-1].split(",")]
        if len(parts) == CLASSICAL_KEY_SIZE:
            return ClassicalSecretKey(bytes(int(p) for p in parts))
        if len(parts) == FNDSA512_SIGN_KEY_LEN:
            return FnDsa512SecretKey(bytes(int(p) for p in parts))
        raise InvalidSecretKeyError("invalid array length")
    body = s[2:] if len(s) >= 2 and s[:2] in ("0x", "0X") else s
    try:
        raw = bytes.fromhex(body)
    except ValueError:
        raw = base58.b58decode(s)
    return secret_keyer_from_bytes(raw)


def as_classical_secret_key(sk: SecretKeyer) -> Optional[ClassicalSecretKey]:
    return sk if isinstance(sk, ClassicalSecretKey) else None


def as_fndsa512_secret_key(sk: SecretKeyer) -> Optional[FnDsa512SecretKey]:
    return sk if isinstance(sk, FnDsa512SecretKey) else None
