"""交易构建器（对应 Go lib/transactionBuilder.go + lib/accountSignatureBuild.go）。

两种签名方式：
- 实时签名：AddPayerSig / AddIxAndPayerSig / AddIxesSig / SignWith
- 模拟签名：AddSimulate*（占位零签名，线尺寸一致，用于 gas 模拟）
构建器收集首个错误并在 Build() 时抛出（Go 的 errs 语义）。
"""
from __future__ import annotations

import time
from dataclasses import dataclass, field
from typing import Optional

from ..api.base import TxHash, PackedInstruction
from ..crypto.address import Address
from ..crypto.errors import MilonCryptoError
from ..crypto.keys import PublicKey
from ..crypto.secretkey import SecretKeyer
from ..postcard import Deserializer, Serializer
from ..types import Bitmap64
from .account_signature import (
    AUTH_PAYER_BIT,
    AUTH_RESERVED_BIT,
    AccountSignature,
    AccountSignatureMode,
    IxHashItem,
    auth_ix_and_payer,
    auth_ixes,
    auth_payer,
    sign,
    simulate_sign,
)
from .transaction import Transaction, TransactionSignatures


@dataclass
class SigningSlot:
    """声明一个账户的签名授权范围（不携带私钥）。"""

    address: Address
    instruction_indices: list[int] = field(default_factory=list)
    include_payer: bool = False
    mode: AccountSignatureMode | None = None


@dataclass
class Signer:
    secret_key: SecretKeyer
    public_key: PublicKey


class AccountSignatureBuilder:
    """流式构造 AccountSignature（对应 Go lib/accountSignatureBuild.go）。"""

    def __init__(self):
        self._auth_bit = Bitmap64(0)
        self._result: Optional[AccountSignature] = None
        self._err: Optional[Exception] = None

    def authorize_payer(self) -> "AccountSignatureBuilder":
        self._auth_bit = self._auth_bit | auth_payer()
        return self

    def authorize_ix(self, ix: int) -> "AccountSignatureBuilder":
        try:
            self._auth_bit = self._auth_bit | auth_ixes([ix])
        except MilonCryptoError as exc:
            self._err = exc
        return self

    def authorize_ixes(self, indices: list[int]) -> "AccountSignatureBuilder":
        try:
            self._auth_bit = self._auth_bit | auth_ixes(indices)
        except MilonCryptoError as exc:
            self._err = exc
        return self

    def authorize_ix_and_payer(self, ix: int) -> "AccountSignatureBuilder":
        try:
            self._auth_bit = self._auth_bit | auth_ix_and_payer(ix)
        except MilonCryptoError as exc:
            self._err = exc
        return self

    def sign(
        self,
        account: Address,
        sk: SecretKeyer,
        tx_hash: TxHash,
        ix_hashes: list[IxHashItem] | None,
        mode: AccountSignatureMode,
    ) -> "AccountSignatureBuilder":
        if self._err is not None:
            return self
        try:
            self._result = sign(account, sk, self._auth_bit, tx_hash, ix_hashes or [], mode)
        except Exception as exc:
            self._err = exc
        return self

    def simulate_sign(self, account: Address, mode: AccountSignatureMode) -> "AccountSignatureBuilder":
        if self._err is not None:
            return self
        try:
            self._result = simulate_sign(account, self._auth_bit, mode)
        except Exception as exc:
            self._err = exc
        return self

    def build(self) -> AccountSignature:
        if self._err is not None:
            raise self._err
        if self._result is None:
            raise MilonCryptoError("signature not built yet")
        return self._result


class TransactionBuilder:
    def __init__(self, instructions: list[bytes]):
        self.tx = Transaction(
            stamp=int(time.time() * 1000),
            payer=None,
            instructions=list(instructions),
            tx_sigs=[],
        )
        self.slots: list[SigningSlot] = []
        self.errs: list[Exception] = []
        self._tx_hash: Optional[TxHash] = None
        self._ix_hashes: Optional[list[TxHash]] = None

    # ---------------------------------------------------------- 惰性缓存
    def _cached_tx_hash(self) -> TxHash:
        if self._tx_hash is None:
            self._tx_hash = self.tx.tx_hash()
        return self._tx_hash

    def _cached_ix_hashes(self) -> list[TxHash]:
        if self._ix_hashes is None:
            self._ix_hashes = self.tx.ix_hashes()
        return self._ix_hashes

    def tx(self) -> Transaction:
        return self.tx

    # ---------------------------------------------------------- 配置
    def with_payer(self, account: Address) -> "TransactionBuilder":
        if self.errs:
            return self
        self.tx.payer = account
        self._tx_hash = None  # payer 变化 → 重算 txHash（ix_hashes 不变）
        return self

    def with_stamp(self, stamp: int) -> "TransactionBuilder":
        if self.errs:
            return self
        self.tx.stamp = stamp
        self._tx_hash = None
        return self

    def add_signature(self, account: Address, account_sig: AccountSignature) -> "TransactionBuilder":
        if self.errs:
            return self
        self.tx.add_signature(account, account_sig)
        return self

    def apply_slots(self, slots: list[SigningSlot]) -> "TransactionBuilder":
        if self.errs:
            return self
        self.slots.extend(slots)
        return self

    def simulate_slots(self) -> "TransactionBuilder":
        if self.errs:
            return self
        for slot in self.slots:
            if not slot.instruction_indices:
                self.add_simulate_payer_sig(slot.address, slot.mode)
            elif len(slot.instruction_indices) == 1 and slot.include_payer:
                self.add_simulate_ix_and_payer_sig(slot.address, slot.instruction_indices[0], slot.mode)
            else:
                self.add_simulate_ixes_sig(slot.address, slot.instruction_indices, slot.include_payer, slot.mode)
            if self.errs:
                return self
        return self

    def sign_with(self, *signers: Signer) -> "TransactionBuilder":
        if self.errs:
            return self
        signer_map: dict[Address, SecretKeyer] = {}
        for signer in signers:
            addr = Address.from_public_key(signer.public_key)
            signer_map[addr] = signer.secret_key
        for slot in self.slots:
            sk = signer_map.get(slot.address)
            if sk is None:
                self.errs.append(MilonCryptoError(f"no signer found for address {slot.address.to_base58()}"))
                return self
            if not slot.instruction_indices:
                self.add_payer_sig(slot.address, sk, slot.mode)
            elif len(slot.instruction_indices) == 1 and slot.include_payer:
                self.add_ix_and_payer_sig(slot.address, sk, slot.instruction_indices[0], slot.mode)
            else:
                self.add_ixes_sig(slot.address, sk, slot.instruction_indices, slot.include_payer, slot.mode)
            if self.errs:
                return self
        return self

    # ---------------------------------------------------------- 模拟签名
    def add_simulate_payer_sig(self, account: Address, mode: AccountSignatureMode) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            sig = AccountSignatureBuilder().authorize_payer().simulate_sign(account, mode).build()
        except Exception as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def add_simulate_ix_and_payer_sig(self, account: Address, ix_index: int, mode: AccountSignatureMode) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            sig = AccountSignatureBuilder().authorize_ix_and_payer(ix_index).simulate_sign(account, mode).build()
        except Exception as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def add_simulate_ixes_sig(
        self,
        account: Address,
        ix_indices: list[int],
        include_payer: bool,
        mode: AccountSignatureMode,
    ) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            builder = AccountSignatureBuilder().authorize_ixes(ix_indices)
            if include_payer:
                builder.authorize_payer()
            sig = builder.simulate_sign(account, mode).build()
        except Exception as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    # ---------------------------------------------------------- 实时签名
    def add_payer_sig(self, account: Address, sk: SecretKeyer, mode: AccountSignatureMode) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            sig = (
                AccountSignatureBuilder()
                .authorize_payer()
                .sign(account, sk, self._cached_tx_hash(), None, mode)
                .build()
            )
        except Exception as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def add_ix_and_payer_sig(self, account: Address, sk: SecretKeyer, ix_index: int, mode: AccountSignatureMode) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            ix_part = self._ix_hashes_for_indices([ix_index])
            sig = (
                AccountSignatureBuilder()
                .authorize_ix_and_payer(ix_index)
                .sign(account, sk, self._cached_tx_hash(), ix_part, mode)
                .build()
            )
        except Exception as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def add_ixes_sig(
        self,
        account: Address,
        sk: SecretKeyer,
        ix_indices: list[int],
        include_payer: bool,
        mode: AccountSignatureMode,
    ) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            ix_part = self._ix_hashes_for_indices(ix_indices)
            builder = AccountSignatureBuilder().authorize_ixes(ix_indices)
            if include_payer:
                builder.authorize_payer()
            sig = builder.sign(account, sk, self._cached_tx_hash(), ix_part, mode).build()
        except Exception as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def reset_sigs(self) -> "TransactionBuilder":
        if self.errs:
            return self
        self.tx.tx_sigs = []
        return self

    # ---------------------------------------------------------- 辅助
    def _ix_hashes_for_indices(self, ix_indices: list[int]) -> list[IxHashItem]:
        hashes = self._cached_ix_hashes()
        items: list[IxHashItem] = []
        for i in ix_indices:
            if i == AUTH_PAYER_BIT:
                raise MilonCryptoError(f"ix index cannot be AuthPayerBit ({AUTH_PAYER_BIT})")
            if i == AUTH_RESERVED_BIT:
                raise MilonCryptoError(f"ix index cannot be AuthReservedBit ({AUTH_RESERVED_BIT})")
            if i >= len(hashes):
                raise MilonCryptoError(f"ix index {i} out of range (max {len(hashes) - 1})")
            items.append(IxHashItem(index=i, hash=hashes[i]))
        return items

    def build(self) -> Transaction:
        if self.errs:
            raise self.errs[0]
        return self.tx
