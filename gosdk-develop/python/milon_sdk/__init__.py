"""Milon Python SDK（对应 Go milon 包公共接口）。

用法示例：

    from milon_sdk import NewClient, DevNet, LocalNet
    from milon_sdk.crypto import ClassicalSecretKey
    from milon_sdk.lib import PubKeySignatureMode

    client = NewClient(DevNet)
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    client.create_account(sk, pk)
    client.claim_faucet(sk, Address.from_public_key(pk), PubKeySignatureMode(public_key=pk))
"""
from __future__ import annotations

from . import api, crypto, gen, lib, postcard, provider, types
from .client import (
    Client,
    NewClient,
    WithClientPollPeriod,
    WithClientPollTimeout,
    WithContext,
    WithRequestID,
    WithWaitContext,
    WithWaitPollPeriod,
    WithWaitPollTimeout,
    WithWaitRequestID,
)
from .crypto import Address, PublicKey, SecretKeyer
from .crypto.keys import PublicKeyType
from .crypto.secretkey import (
    ClassicalSecretKey,
    FnDsa512SecretKey,
    SecretKeyType,
    secret_keyer_from_bytes,
    secret_keyer_from_string_relaxed,
)
from .crypto.signature import (
    Signature,
    SignatureType,
    verify_batch,
)
from .lib import AccountSignatureMode, PubKeySignatureMode, MultisigKeySignatureMode
from .network import DevNet, LocalNet, Network

# 常用常量与类型
from .api.base import MIL, MIL_TOKEN
from .types import Bitmap64

__version__ = "0.1.0"

__all__ = [
    "NewClient",
    "Client",
    "Network",
    "LocalNet",
    "DevNet",
    "WithClientPollPeriod",
    "WithClientPollTimeout",
    "WithContext",
    "WithRequestID",
    "WithWaitContext",
    "WithWaitPollPeriod",
    "WithWaitPollTimeout",
    "WithWaitRequestID",
    "Address",
    "PublicKey",
    "PublicKeyType",
    "SecretKeyer",
    "ClassicalSecretKey",
    "FnDsa512SecretKey",
    "SecretKeyType",
    "secret_keyer_from_bytes",
    "secret_keyer_from_string_relaxed",
    "Signature",
    "SignatureType",
    "verify_batch",
    "AccountSignatureMode",
    "PubKeySignatureMode",
    "MultisigKeySignatureMode",
    "Bitmap64",
    "MIL",
    "MIL_TOKEN",
    "api",
    "crypto",
    "gen",
    "provider",
    "lib",
    "postcard",
    "types",
]
