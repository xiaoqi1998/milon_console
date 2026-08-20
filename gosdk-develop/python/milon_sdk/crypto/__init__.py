"""Milon 密码学层（对应 Go crypto/ 包）。"""

from .errors import (
    MilonCryptoError,
    InvalidSecretKeyError,
    InvalidPublicKeyError,
    InvalidSignatureError,
    SignatureVerificationError,
)
from .hashes import (
    MILON_ROOT_DOMAIN_CONTEXT,
    MILON_IX_HASH_DOMAIN_CONTEXT,
    MILON_TX_HASH_DOMAIN_CONTEXT,
    MILON_TX_AUTH_DOMAIN_CONTEXT,
    MILON_PK_ADDRESS_DOMAIN_CONTEXT,
    hasher,
    hash32,
)
from .address import Address, ADDRESS_RAW_LEN
from .keys import (
    PublicKey,
    PublicKeyType,
    PUBLIC_KEY_SECP256K1_SIZE,
    PUBLIC_KEY_ED25519_SIZE,
    PUBLIC_KEY_BLS12381_SIZE,
    PUBLIC_KEY_FNDSA512_SIZE,
)
from .secretkey import (
    SecretKeyer,
    SecretKeyType,
    ClassicalSecretKey,
    FnDsa512SecretKey,
    secret_keyer_from_bytes,
    secret_keyer_from_string_relaxed,
)
from .signature import (
    Signature,
    SignatureType,
    SIGNATURE_SECP256K1_SIZE,
    SIGNATURE_ED25519_SIZE,
    SIGNATURE_BLS12381_SIZE,
    SIGNATURE_FNDSA512_SIZE,
    verify_batch,
)
from . import fn_dsa512, _bls

__all__ = [
    "MilonCryptoError",
    "InvalidSecretKeyError",
    "InvalidPublicKeyError",
    "InvalidSignatureError",
    "SignatureVerificationError",
    "Address",
    "ADDRESS_RAW_LEN",
    "PublicKey",
    "PublicKeyType",
    "PUBLIC_KEY_SECP256K1_SIZE",
    "PUBLIC_KEY_ED25519_SIZE",
    "PUBLIC_KEY_BLS12381_SIZE",
    "PUBLIC_KEY_FNDSA512_SIZE",
    "SecretKeyer",
    "SecretKeyType",
    "ClassicalSecretKey",
    "FnDsa512SecretKey",
    "secret_keyer_from_bytes",
    "secret_keyer_from_string_relaxed",
    "Signature",
    "SignatureType",
    "SIGNATURE_SECP256K1_SIZE",
    "SIGNATURE_ED25519_SIZE",
    "SIGNATURE_BLS12381_SIZE",
    "SIGNATURE_FNDSA512_SIZE",
    "verify_batch",
    "hasher",
    "hash32",
    "MILON_ROOT_DOMAIN_CONTEXT",
    "MILON_IX_HASH_DOMAIN_CONTEXT",
    "MILON_TX_HASH_DOMAIN_CONTEXT",
    "MILON_TX_AUTH_DOMAIN_CONTEXT",
    "MILON_PK_ADDRESS_DOMAIN_CONTEXT",
    "fn_dsa512",
    "_bls",
]
