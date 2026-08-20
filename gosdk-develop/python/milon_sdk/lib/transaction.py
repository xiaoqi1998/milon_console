"""交易结构与哈希（对应 Go lib/transaction.go）。

哈希定义（端序纪律：哈希输入整数一律 BigEndian）：
    IxHash  = BLAKE3(ROOT || "milon.ix.v1"   || chain_id(BE u64) || wire)
    TxHash  = BLAKE3(ROOT || "milon.tx.v1"   || chain_id(BE u64) || stamp(BE u64)
                     || [payer(20B)] || ix_hashes...)
    AuthMsg = BLAKE3(ROOT || "milon.tx.auth.v1" || chain_id(BE u64) || owner(20B)
                     || auth_bit(LE u64) || tx_hash || 被授权的 ix 哈希序列)
  （注意 auth_bit 是 LittleEndian——与上面两个不同！）

Postcard 线：
    stamp(u64) + payer(option Address) + instructions(seq bytes) + tx_sigs(seq)
"""
from __future__ import annotations

import struct

from ..api.base import TxHash, PackedInstruction
from ..crypto.address import Address
from ..crypto.hashes import IX_HASH_DOMAIN, TX_HASH_DOMAIN, hasher
from ..postcard import Deserializer, Serializer
from .chain import get_chain_id
from .account_signature import AccountSignature


def _be_u64(value: int) -> bytes:
    return struct.pack(">Q", value)


class TransactionSignatures:
    __slots__ = ("address", "account_signature")

    def __init__(self, address: Address, account_signature: AccountSignature):
        self.address = address
        self.account_signature = account_signature


class Transaction:
    __slots__ = ("stamp", "payer", "instructions", "tx_sigs")

    def __init__(self, stamp: int, payer: Address | None, instructions: list[bytes], tx_sigs: list[TransactionSignatures]):
        self.stamp = stamp
        self.payer = payer
        self.instructions = list(instructions)
        self.tx_sigs = list(tx_sigs)

    # ---------------------------------------------------------- 哈希
    def ix_hash_from_wire(self, wire: bytes) -> TxHash:
        h = hasher(IX_HASH_DOMAIN)
        h.update(_be_u64(get_chain_id()))
        h.update(wire)
        return TxHash(h.digest())

    def ix_hashes(self) -> list[TxHash]:
        return [self.ix_hash_from_wire(w) for w in self.instructions]

    def tx_hash(self) -> TxHash:
        h = hasher(TX_HASH_DOMAIN)
        h.update(_be_u64(get_chain_id()))
        h.update(_be_u64(self.stamp))
        if self.payer is not None:
            h.update(self.payer.as_bytes())
        for instruction in self.instructions:
            h.update(self.ix_hash_from_wire(instruction).as_bytes())
        return TxHash(h.digest())

    # ---------------------------------------------------------- 签名管理
    def add_signature(self, address: Address, account_sig: AccountSignature) -> None:
        self.tx_sigs.append(TransactionSignatures(address, account_sig))

    # ---------------------------------------------------------- 线编解码
    def to_bytes(self) -> bytes:
        serializer = Serializer()
        self.marshal_postcard(serializer)
        return serializer.bytes()

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_u64(self.stamp)
        serializer.serialize_option(self.payer is not None, lambda s: self.payer.marshal_postcard(s))
        serializer.serialize_seq(self.instructions, lambda s, w: s.serialize_bytes(w))
        serializer.serialize_seq(
            self.tx_sigs,
            lambda s, sig: _marshal_tx_signatures(s, sig),
        )

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "Transaction":
        stamp = d.deserialize_u64()
        payer = d.deserialize_option(lambda dd: Address.unmarshal_postcard(dd))
        instructions = d.deserialize_seq(lambda dd: dd.deserialize_bytes())
        tx_sigs = d.deserialize_seq(_unmarshal_tx_signatures)
        return cls(stamp, payer, instructions, tx_sigs)


def _marshal_tx_signatures(s: Serializer, sig: TransactionSignatures) -> None:
    sig.address.marshal_postcard(s)
    sig.account_signature.marshal_postcard(s)


def _unmarshal_tx_signatures(d: Deserializer) -> TransactionSignatures:
    address = Address.unmarshal_postcard(d)
    account_sig = AccountSignature.unmarshal_postcard(d)
    return TransactionSignatures(address, account_sig)


def _validate_wire(tx: Transaction, sponsor_ix: list[int] | None = None) -> None:
    """对应 Go ValidateWireWith：校验指令数/去重、签名 owner、auth 位图、gas 签名。"""
    from .account_signature import (
        AUTH_PAYER_BIT,
        AUTH_RESERVED_BIT,
        authorizes_ix,
        authorizes_payer,
    )

    sponsor_ix = sponsor_ix or []
    if not tx.instructions:
        raise ValueError("empty instructions")
    if len(tx.instructions) > AUTH_RESERVED_BIT:
        raise ValueError(f"too many instructions: {len(tx.instructions)} (max {AUTH_RESERVED_BIT})")

    seen_ix: set[bytes] = set()
    for wire in tx.instructions:
        h = tx.ix_hash_from_wire(wire).as_bytes()
        if h in seen_ix:
            raise ValueError("duplicate ix hash")
        seen_ix.add(h)

    owners: set[bytes] = set()
    for sig in tx.tx_sigs:
        if sig.address.as_bytes() in owners:
            raise ValueError("duplicate signature owner")
        owners.add(sig.address.as_bytes())
        if sig.account_signature.auth_bit.raw() == 0:
            raise ValueError("empty auth bit")
        for i in range(64):
            if sig.account_signature.auth_bit.test(i):
                if i != AUTH_PAYER_BIT and i >= len(tx.instructions):
                    raise ValueError(f"auth ix index {i} out of range")

    sponsor_set = set(sponsor_ix)

    if tx.payer is not None:  # UnifiedPayer 模式：payer 必须签名并授权 bit63
        has_payer_sig = any(
            s.address == tx.payer and authorizes_payer(s.account_signature)
            for s in tx.tx_sigs
        )
        if not has_payer_sig:
            raise ValueError("payer signature required")
    else:  # SplitPayerSelfPay：每个未赞助的 ix 需有人同时授权 bit63 与该 ix
        for i in range(len(tx.instructions)):
            if i in sponsor_set:
                continue
            has_gas = any(
                authorizes_payer(s.account_signature) and authorizes_ix(s.account_signature, i)
                for s in tx.tx_sigs
            )
            if not has_gas:
                raise ValueError(f"gas signer required for ix {i}")
        for sig in tx.tx_sigs:
            has_payer = authorizes_payer(sig.account_signature)
            has_ix = (sig.account_signature.auth_bit.raw() & ((1 << AUTH_RESERVED_BIT) - 1)) != 0
            if has_payer and not has_ix:
                raise ValueError("gas payment mode conflict")


def validate_wire(tx: Transaction) -> None:
    _validate_wire(tx)


def validate_wire_with(tx: Transaction, sponsor_ix: list[int]) -> None:
    _validate_wire(tx, sponsor_ix)
