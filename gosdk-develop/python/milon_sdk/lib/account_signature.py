"""账户签名与鉴权（对应 Go lib/accountSignature.go）。

AuthBit 布局（64 位）：bit0-61 = 指令授权，bit62 = 保留，bit63 = gas(payer)。
AuthMessage = BLAKE3(ROOT || "milon.tx.auth.v1" || chain_id(BE u64) || owner(20B)
                     || auth_bit(LE u64) || tx_hash || 按序的已授权 ix 哈希)
"""
from __future__ import annotations

import struct
from dataclasses import dataclass, field
from typing import Optional, Protocol

from ..api.base import TxHash
from ..crypto.address import Address
from ..crypto.errors import MilonCryptoError
from ..crypto.hashes import TX_AUTH_DOMAIN, hasher
from ..crypto.keys import PublicKey, PublicKeyType
from ..crypto.secretkey import SecretKeyer
from ..crypto.signature import (
    SIGNATURE_BLS12381_SIZE,
    SIGNATURE_ED25519_SIZE,
    SIGNATURE_FNDSA512_SIZE,
    SIGNATURE_SECP256K1_SIZE,
    Signature,
    SignatureType,
)
from ..postcard import Deserializer, Serializer
from ..types import Bitmap64
from .chain import get_chain_id

AUTH_PAYER_BIT = 63
AUTH_RESERVED_BIT = 62


# ================================================================
# 签名模式
# ================================================================


class AccountSignatureMode(Protocol):
    def is_account_signature_mode(self) -> None: ...


@dataclass
class PubKeySignatureMode:
    """单公钥签名。SkipPubKey=True 时线格式省略公钥，SigBit 需从链上 signers 解析。"""

    public_key: PublicKey
    skip_pub_key: bool = False
    sig_bit: Bitmap64 = field(default_factory=lambda: Bitmap64(0))

    def is_account_signature_mode(self) -> None:
        pass


@dataclass
class MultisigKeySignatureMode:
    """多签参与者：公钥由 SigBit = 1<<Index 在链上定位。"""

    index: int
    public_key: PublicKey

    def is_account_signature_mode(self) -> None:
        pass


# ================================================================
# 账户签名
# ================================================================


class AccountSignature:
    __slots__ = ("auth_bit", "sig_bit", "signatures", "pub_key")

    def __init__(
        self,
        auth_bit: Bitmap64,
        sig_bit: Bitmap64 = None,
        signatures: list[Signature] | None = None,
        pub_key: PublicKey | None = None,
    ):
        self.auth_bit = auth_bit
        self.sig_bit = sig_bit if sig_bit is not None else Bitmap64(0)
        self.signatures = list(signatures or [])
        self.pub_key = pub_key

    # ---------------------------------------------------------- 查询
    def authorizes_ix(self, ix: int) -> bool:
        return self.auth_bit.test(ix)

    def authorizes_payer(self) -> bool:
        return self.auth_bit.test(AUTH_PAYER_BIT)

    def add_multisig_key(self, key_index: int, signature: Signature) -> None:
        if key_index >= 64:
            raise MilonCryptoError(f"key index {key_index} out of range (max 63)")
        if self.pub_key is not None:
            raise MilonCryptoError("pubkey mode cannot add multisig keys")
        self.sig_bit = self.sig_bit.set(key_index)
        self.signatures.append(signature)

    def is_vote_gate_only(self) -> bool:
        return (
            self.pub_key is None
            and len(self.signatures) == 0
            and self.sig_bit.raw() == 0
            and (self.auth_bit.raw() & ((1 << AUTH_RESERVED_BIT) - 1)) != 0
        )

    # ---------------------------------------------------------- 鉴权消息
    def auth_message(self, account: Address, tx_hash: TxHash, ix_hashes: list["IxHashItem"]) -> TxHash:
        h = hasher(TX_AUTH_DOMAIN)
        h.update(struct.pack(">Q", get_chain_id()))  # BigEndian
        h.update(account.as_bytes())
        h.update(struct.pack("<Q", self.auth_bit.raw()))  # LittleEndian（与 TxHash/IxHash 不同！）
        h.update(tx_hash.as_bytes())
        for item in ix_hashes:
            if not self.auth_bit.test(item.index):
                raise MilonCryptoError(f"ix index {item.index} is not authorized in auth_bit")
            h.update(item.hash.as_bytes())
        return TxHash(h.digest())

    # ---------------------------------------------------------- 线编解码
    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_u64(self.auth_bit.raw())
        serializer.serialize_u64(self.sig_bit.raw())
        serializer.serialize_seq(self.signatures, lambda s, sig: sig.marshal_postcard(s))
        serializer.serialize_option(
            self.pub_key is not None,
            lambda s: self.pub_key.marshal_postcard(s),
        )

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "AccountSignature":
        auth_bit = Bitmap64(d.deserialize_u64())
        sig_bit = Bitmap64(d.deserialize_u64())
        signatures = d.deserialize_seq(lambda dd: Signature.unmarshal_postcard(dd))
        pub_key = d.deserialize_option(lambda dd: PublicKey.unmarshal_postcard(dd))
        return cls(auth_bit, sig_bit, signatures, pub_key)


@dataclass
class IxHashItem:
    index: int
    hash: TxHash


# ================================================================
# AuthBit 构造辅助
# ================================================================


def auth_ix(ix: int) -> Bitmap64:
    if ix >= AUTH_RESERVED_BIT:
        raise MilonCryptoError(f"ix index {ix} out of range (max {AUTH_RESERVED_BIT - 1})")
    return Bitmap64(1 << ix)


def auth_ixes(indices: list[int]) -> Bitmap64:
    raw = 0
    for ix in indices:
        if ix >= AUTH_RESERVED_BIT:
            raise MilonCryptoError(f"ix index {ix} out of range (max {AUTH_RESERVED_BIT - 1})")
        raw |= 1 << ix
    return Bitmap64(raw)


def auth_payer() -> Bitmap64:
    return Bitmap64(1 << AUTH_PAYER_BIT)


def auth_ix_and_payer(ix: int) -> Bitmap64:
    if ix >= AUTH_RESERVED_BIT:
        raise MilonCryptoError(f"ix index {ix} out of range (max {AUTH_RESERVED_BIT - 1})")
    return Bitmap64((1 << ix) | (1 << AUTH_PAYER_BIT))


def authorizes_ix(sig: "AccountSignature", ix: int) -> bool:
    """谓词：签名是否授权了指令 ix（供 validate_wire 使用）。"""
    return sig.authorizes_ix(ix)


def authorizes_payer(sig: "AccountSignature") -> bool:
    """谓词：签名是否授权了 gas(payer) 位（供 validate_wire 使用）。"""
    return sig.authorizes_payer()


def unsigned(auth_bit: Bitmap64) -> AccountSignature:
    return AccountSignature(auth_bit=auth_bit, sig_bit=Bitmap64(0), signatures=[], pub_key=None)


# ================================================================
# 签名流程
# ================================================================


def _resolve_mode(account: Address, mode: AccountSignatureMode) -> tuple[PublicKey, Bitmap64, Optional[PublicKey]]:
    """校验签名模式与 owner 匹配，推导 (签名公钥, sigBit, PubKey 字段)。"""
    if isinstance(mode, PubKeySignatureMode):
        pk_addr = Address.from_public_key(mode.public_key)
        if pk_addr != account:
            raise MilonCryptoError("public key does not match owner address")
        if mode.skip_pub_key:
            if mode.sig_bit.raw() == 0:
                raise MilonCryptoError("SkipPubKey requires SigBit resolved from the on-chain signers list")
            return mode.public_key, mode.sig_bit, None
        return mode.public_key, Bitmap64(0), mode.public_key
    if isinstance(mode, MultisigKeySignatureMode):
        if mode.index >= 64:
            raise MilonCryptoError(f"multisig key index {mode.index} out of range (max 63)")
        return mode.public_key, Bitmap64(1 << mode.index), None
    raise MilonCryptoError("invalid signature mode")


def sign(
    account: Address,
    sk: SecretKeyer,
    auth_bit: Bitmap64,
    tx_hash: TxHash,
    ix_hashes: list[IxHashItem],
    mode: AccountSignatureMode,
) -> AccountSignature:
    public_key, sig_bit, pub_key_field = _resolve_mode(account, mode)
    acc_sig = AccountSignature(auth_bit=auth_bit, sig_bit=sig_bit, pub_key=pub_key_field)
    auth_hash = acc_sig.auth_message(account, tx_hash, ix_hashes)
    signature = sk.sign_for(public_key, auth_hash.as_bytes())
    acc_sig.signatures = [signature]
    return acc_sig


def simulate_sign(account: Address, auth_bit: Bitmap64, mode: AccountSignatureMode) -> AccountSignature:
    """无真实签名的占位签名：签名长度与真实一致，便于 gas 模拟。"""
    public_key, sig_bit, pub_key_field = _resolve_mode(account, mode)
    return AccountSignature(
        auth_bit=auth_bit,
        sig_bit=sig_bit,
        signatures=[_placeholder_signature(public_key)],
        pub_key=pub_key_field,
    )


def _placeholder_signature(pk: PublicKey) -> Signature:
    size = {
        PublicKeyType.SECP256K1: SIGNATURE_SECP256K1_SIZE,
        PublicKeyType.ED25519: SIGNATURE_ED25519_SIZE,
        PublicKeyType.BLS12381: SIGNATURE_BLS12381_SIZE,
        PublicKeyType.FNDSA512: SIGNATURE_FNDSA512_SIZE,
    }[pk.variant]
    return Signature(SignatureType(pk.variant), b"\x00" * size)


def collect_ix_hashes(auth_bit: Bitmap64, ix_hashes: list[TxHash]) -> list[IxHashItem]:
    out: list[IxHashItem] = []
    for i in range(AUTH_RESERVED_BIT):
        if not auth_bit.test(i):
            continue
        if i < len(ix_hashes):
            out.append(IxHashItem(index=i, hash=ix_hashes[i]))
    return out
