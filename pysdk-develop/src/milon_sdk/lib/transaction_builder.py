"""TransactionBuilder. Faithful port of gosdk-develop/lib/transactionBuilder.go."""

from __future__ import annotations

import time
from dataclasses import dataclass, field
from typing import List, Optional

from ..api import PackedInstruction, TxHash
from ..crypto import (
    Address,
    SecretKeyer,
    new_address_from_public_key,
)
from .account_signature import (
    AUTH_PAYER_BIT,
    AUTH_RESERVED_BIT,
    AccountSignatureMode,
    IxHashItem,
    PubKeySignatureMode,
)
from .account_signature_build import new_account_signature_builder
from .transaction import Transaction, TransactionSignatures


@dataclass
class SigningSlot:
    """Declares a signature authorization for one account (without secret
    keys)."""

    Address: Address = None
    InstructionIndices: List[int] = field(default_factory=list)
    IncludePayer: bool = False
    Mode: AccountSignatureMode = None


@dataclass
class Signer:
    """A signer entry carrying a secret key and its matching public key."""

    SecretKey: SecretKeyer = None
    PublicKey: object = None


class TransactionBuilder:
    def __init__(self, instructions: List[PackedInstruction]) -> None:
        self.tx = Transaction(
            Stamp=int(time.time() * 1000),
            Instructions=instructions,
            TxSigs=[],
        )
        self.slots: List[SigningSlot] = []
        self.errs: List[ValueError] = []
        self._tx_hash: Optional[TxHash] = None
        self._ix_hashes: Optional[List[TxHash]] = None

    def _cached_tx_hash(self) -> TxHash:
        if self._tx_hash is None:
            self._tx_hash = self.tx.tx_hash()
        return self._tx_hash

    def _cached_ix_hashes(self) -> List[TxHash]:
        if self._ix_hashes is None:
            self._ix_hashes = self.tx.ix_hashes()
        return self._ix_hashes

    def with_payer(self, account: Optional[Address]) -> "TransactionBuilder":
        if self.errs:
            return self
        self.tx.Payer = account
        self._tx_hash = None
        return self

    def with_stamp(self, stamp: int) -> "TransactionBuilder":
        if self.errs:
            return self
        self.tx.Stamp = stamp
        self._tx_hash = None
        return self

    def add_signature(
        self, account: Address, account_sig
    ) -> "TransactionBuilder":
        if self.errs:
            return self
        self.tx.add_signature(account, account_sig)
        return self

    def apply_slots(self, slots: List[SigningSlot]) -> "TransactionBuilder":
        if self.errs:
            return self
        self.slots.extend(slots)
        return self

    def simulate_slots(self) -> "TransactionBuilder":
        if self.errs:
            return self
        for slot in self.slots:
            if len(slot.InstructionIndices) == 0:
                self.add_simulate_payer_sig(slot.Address, slot.Mode)
            elif len(slot.InstructionIndices) == 1 and slot.IncludePayer:
                self.add_simulate_ix_and_payer_sig(
                    slot.Address, slot.InstructionIndices[0], slot.Mode
                )
            else:
                self.add_simulate_ixes_sig(
                    slot.Address,
                    slot.InstructionIndices,
                    slot.IncludePayer,
                    slot.Mode,
                )
            if self.errs:
                return self
        return self

    def add_simulate_payer_sig(
        self, account: Address, mode: AccountSignatureMode
    ) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            sig = (
                new_account_signature_builder()
                .authorize_payer()
                .simulate_sign(account, mode)
                .build()
            )
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def add_simulate_ix_and_payer_sig(
        self, account: Address, ix_index: int, mode: AccountSignatureMode
    ) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            sig = (
                new_account_signature_builder()
                .authorize_ix_and_payer(ix_index)
                .simulate_sign(account, mode)
                .build()
            )
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def add_simulate_ixes_sig(
        self,
        account: Address,
        ix_indices: List[int],
        include_payer: bool,
        mode: AccountSignatureMode,
    ) -> "TransactionBuilder":
        if self.errs:
            return self
        sig_builder = new_account_signature_builder().authorize_ixes(ix_indices)
        if include_payer:
            sig_builder.authorize_payer()
        try:
            sig = sig_builder.simulate_sign(account, mode).build()
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def sign_with(self, *signers: Signer) -> "TransactionBuilder":
        """Signs all registered slots, matching each signer by address."""
        if self.errs:
            return self
        signer_map = {}
        for signer in signers:
            try:
                addr = new_address_from_public_key(signer.PublicKey)
            except ValueError as exc:
                self.errs.append(
                    ValueError(f"derive address from signer: {exc}")
                )
                return self
            signer_map[addr.Bytes] = signer.SecretKey

        for slot in self.slots:
            sk = signer_map.get(slot.Address.Bytes)
            if sk is None:
                self.errs.append(
                    ValueError(f"no signer found for address {slot.Address}")
                )
                return self
            if len(slot.InstructionIndices) == 0:
                self.add_payer_sig(slot.Address, sk, slot.Mode)
            elif len(slot.InstructionIndices) == 1 and slot.IncludePayer:
                self.add_ix_and_payer_sig(
                    slot.Address, sk, slot.InstructionIndices[0], slot.Mode
                )
            else:
                self.add_ixes_sig(
                    slot.Address,
                    sk,
                    slot.InstructionIndices,
                    slot.IncludePayer,
                    slot.Mode,
                )
            if self.errs:
                return self
        return self

    def add_payer_sig(
        self, account: Address, sk: SecretKeyer, mode: AccountSignatureMode
    ) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            sig = (
                new_account_signature_builder()
                .authorize_payer()
                .sign(account, sk, self._cached_tx_hash(), None, mode)
                .build()
            )
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def add_ix_and_payer_sig(
        self,
        account: Address,
        sk: SecretKeyer,
        ix_index: int,
        mode: AccountSignatureMode,
    ) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            ix_part = self._ix_hashes_for_indices([ix_index])
        except ValueError as exc:
            self.errs.append(exc)
            return self
        try:
            sig = (
                new_account_signature_builder()
                .authorize_ix_and_payer(ix_index)
                .sign(account, sk, self._cached_tx_hash(), ix_part, mode)
                .build()
            )
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def add_ixes_sig(
        self,
        account: Address,
        sk: SecretKeyer,
        ix_indices: List[int],
        include_payer: bool,
        mode: AccountSignatureMode,
    ) -> "TransactionBuilder":
        if self.errs:
            return self
        try:
            ix_part = self._ix_hashes_for_indices(ix_indices)
        except ValueError as exc:
            self.errs.append(exc)
            return self
        sig_builder = new_account_signature_builder().authorize_ixes(ix_indices)
        if include_payer:
            sig_builder.authorize_payer()
        try:
            sig = sig_builder.sign(
                account, sk, self._cached_tx_hash(), ix_part, mode
            ).build()
        except ValueError as exc:
            self.errs.append(exc)
            return self
        self.tx.add_signature(account, sig)
        return self

    def reset_sigs(self) -> "TransactionBuilder":
        """Clears all signatures (e.g., switch from simulated to real signing)."""
        if self.errs:
            return self
        self.tx.TxSigs = []
        return self

    def _ix_hashes_for_indices(self, ix_indices: List[int]) -> List[IxHashItem]:
        """Builds the IxHashItem list for the given instruction indices,
        rejecting reserved and out-of-range indices."""
        hashes = self._cached_ix_hashes()
        items = []
        for i in ix_indices:
            if i == AUTH_PAYER_BIT:
                raise ValueError(
                    f"ix index cannot be AuthPayerBit ({AUTH_PAYER_BIT})"
                )
            if i == AUTH_RESERVED_BIT:
                raise ValueError(
                    f"ix index cannot be AuthReservedBit ({AUTH_RESERVED_BIT})"
                )
            if i >= len(hashes):
                raise ValueError(
                    f"ix index {i} out of range (max {len(hashes) - 1})"
                )
            items.append(IxHashItem(Index=i, Hash=hashes[i]))
        return items

    def build(self) -> Transaction:
        """Finalizes and returns the Transaction; raises the first error
        encountered in the chain."""
        if self.errs:
            raise self.errs[0]
        return self.tx


def new_transaction_builder(
    instructions: List[PackedInstruction],
) -> TransactionBuilder:
    return TransactionBuilder(instructions)
