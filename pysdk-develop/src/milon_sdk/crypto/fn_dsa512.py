"""FN-DSA-512 定长字节类型与高层 API. Faithful port of gosdk-develop/crypto/fn_dsa512.go."""

from __future__ import annotations

from . import _fndsa
from .error import (
    InvalidPublicKeyError,
    InvalidSecretKeyError,
    InvalidSignatureError,
    KeyGenFailedError,
    SignFailedError,
)

# LogN value, 9 means degree 2^9 = 512
LOG_N = 9

FN_DSA512_SIGN_KEY_LEN = 1281
FN_DSA512_VRFY_KEY_LEN = 897
FN_DSA512_SIG_LEN = 666

# 定长 bytes 类型（Go: [N]byte 数组类型）。Python 侧以 bytes + 构造校验表达。


def keygen_512() -> tuple[bytes, bytes]:
    """Generates an FN-DSA-512 key pair -> (sign_key, vrfy_key)."""
    try:
        sign_key, verify_key = _fndsa.keygen(LOG_N)
    except NotImplementedError:
        raise
    except Exception as exc:
        raise KeyGenFailedError() from exc

    if len(sign_key) != FN_DSA512_SIGN_KEY_LEN:
        raise KeyGenFailedError()
    if len(verify_key) != FN_DSA512_VRFY_KEY_LEN:
        raise KeyGenFailedError()
    return sign_key, verify_key


def new_sign_key_512_from_bytes(raw: bytes) -> bytes:
    if len(raw) != FN_DSA512_SIGN_KEY_LEN:
        raise InvalidSecretKeyError()
    return bytes(raw)


def new_vrfy_key_512_from_bytes(raw: bytes) -> bytes:
    if len(raw) != FN_DSA512_VRFY_KEY_LEN:
        raise InvalidPublicKeyError()
    return bytes(raw)


def sign_512(signKey: bytes, msg: bytes) -> bytes:
    """Signs the message msg.

    Parameter notes:
      - rng: nil uses system RNG (recommended)
      - ctx: domain separation context (empty string corresponds to DOMAIN_NONE)
      - id: pre-hash function identifier (0 for raw message)
    """
    try:
        signature = _fndsa.sign(signKey, b"", 0, msg)
    except NotImplementedError:
        raise
    except Exception as exc:
        raise SignFailedError() from exc

    if len(signature) != FN_DSA512_SIG_LEN:
        raise SignFailedError()
    return signature


def verify_512(vrfy_key: bytes, sig: bytes, msg: bytes) -> None:
    """Verifies a signature; raises InvalidSignatureError on failure."""
    is_valid = _fndsa.verify(vrfy_key, b"", 0, msg, sig)
    if not is_valid:
        raise InvalidSignatureError()
