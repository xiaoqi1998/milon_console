"""crypto 包共享异常. Faithful port of gosdk-develop/crypto/error.go + fn_dsa512.go 错误变量."""

from __future__ import annotations


class MilonCryptoError(ValueError):
    """milon crypto 错误基类（对应 Go errors.New 的导出错误变量）。"""


class InvalidSecretKeyError(MilonCryptoError):
    """Go: ErrInvalidSecretKey = "invalid secret key"."""

    def __str__(self) -> str:
        return "invalid secret key"


class InvalidPublicKeyError(MilonCryptoError):
    """Go: ErrInvalidPublicKey = "invalid public key"."""

    def __str__(self) -> str:
        return "invalid public key"


class InvalidSignatureError(MilonCryptoError):
    """Go: ErrInvalidSignature = "invalid signature"."""

    def __str__(self) -> str:
        return "invalid signature"


class SignFailedError(MilonCryptoError):
    """Go: ErrSignFailed = "signing failed"."""

    def __str__(self) -> str:
        return "signing failed"


class KeyGenFailedError(MilonCryptoError):
    """Go: ErrKeyGenFailed = "key generation failed"."""

    def __str__(self) -> str:
        return "key generation failed"
