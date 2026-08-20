"""密码学公共错误（对应 Go crypto/error.go）。"""


class MilonCryptoError(Exception):
    """SDK 密码学错误基类。"""


class InvalidSecretKeyError(MilonCryptoError):
    """无效私钥。"""


class InvalidPublicKeyError(MilonCryptoError):
    """无效公钥。"""


class InvalidSignatureError(MilonCryptoError):
    """无效签名。"""


class SignatureVerificationError(InvalidSignatureError):
    """签名验签失败。"""
