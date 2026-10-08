"""Transaction. Faithful port of gosdk-develop/lib/transaction.go."""

from __future__ import annotations

import struct
from dataclasses import dataclass, field
from typing import List, Optional

from ..api import PackedInstruction, TxHash
from ..crypto import Address
from ..crypto.hash_domain import (
    IX_HASH_DOMAIN_BYTES,
    TX_HASH_DOMAIN_BYTES,
    hasher,
)
from ..postcard import (
    Deserializer,
    Serializer,
    deserialize_option,
    deserialize_seq,
    serialize_option,
    serialize_seq,
)
from .account_signature import (
    AUTH_PAYER_BIT,
    AUTH_RESERVED_BIT,
    AUTH_VOTE_BIT,
    AccountSignature,
)

from .chain_id import GetChainId, get_chain_id, set_chain_id


@dataclass
class TransactionSignatures:
    Address: Address = field(default_factory=Address)
    AccountSignature: AccountSignature = field(default_factory=AccountSignature)


@dataclass
class Transaction:
    Stamp: int = 0
    Payer: Optional[Address] = None
    Instructions: List[PackedInstruction] = field(default_factory=list)
    TxSigs: List[TransactionSignatures] = field(default_factory=list)

    def tx_hash(self) -> TxHash:
        """TxHash = Blake3(MILON_ROOT || TX_HASH_DOMAIN || GetChainId() || Stamp
        || [Payer] || ix_hashes...)。chain_id/stamp 为大端 8 字节。"""
        h = hasher(TX_HASH_DOMAIN_BYTES)

        h.update(struct.pack(">Q", GetChainId()))
        h.update(struct.pack(">Q", self.Stamp))

        if self.Payer is not None:
            h.update(self.Payer.as_bytes())

        for instruction in self.Instructions:
            h.update(self.ix_hash_from_wire(instruction))

        return h.digest()

    def add_signature(self, address: Address, account_sig: AccountSignature) -> None:
        self.TxSigs.append(
            TransactionSignatures(Address=address, AccountSignature=account_sig)
        )

    def ix_hashes(self) -> List[TxHash]:
        return [self.ix_hash_from_wire(i) for i in self.Instructions]

    def ix_hash_from_wire(self, wire: PackedInstruction) -> TxHash:
        """IxHash = Blake3(MILON_ROOT || IX_HASH_DOMAIN || GetChainId() || wire)。"""
        h = hasher(IX_HASH_DOMAIN_BYTES)
        h.update(struct.pack(">Q", GetChainId()))
        h.update(wire)
        return h.digest()

    def validate_wire(self) -> None:
        """Validates the transaction wire layer structure. Equivalent to
        validate_wire_with([])."""
        return self.validate_wire_with([])

    def validate_wire_with(self, sponsor_ix) -> None:
        """Validates the transaction wire layer structure with sponsored
        instructions.

        sponsorIx lists sponsored instruction indices, which skip the gas
        signature check. In UnifiedPayer mode it has no effect.
        """
        if len(self.Instructions) == 0:
            raise ValueError("empty instructions")

        if len(self.Instructions) > AUTH_RESERVED_BIT:
            raise ValueError(
                f"too many instructions: {len(self.Instructions)} "
                f"(max {AUTH_RESERVED_BIT})"
            )

        seen_ix = set()
        for wire in self.Instructions:
            h = self.ix_hash_from_wire(wire)
            if h in seen_ix:
                raise ValueError("duplicate ix hash")
            seen_ix.add(h)

        owners = set()
        for sig in self.TxSigs:
            if sig.Address.Bytes in owners:
                raise ValueError("duplicate signature owner")
            owners.add(sig.Address.Bytes)

            if sig.AccountSignature.AuthBit.raw() == 0:
                raise ValueError("empty auth bit")

            for i in range(64):
                if sig.AccountSignature.AuthBit.test(i):
                    if (
                        i != AUTH_PAYER_BIT
                        and i != AUTH_VOTE_BIT
                        and i >= len(self.Instructions)
                    ):
                        raise ValueError(f"auth ix index {i} out of range")

        sponsor_set = set(sponsor_ix)

        if self.Payer is not None:  # UnifiedPayer mode
            has_payer_sig = any(
                sig.Address.Bytes == self.Payer.Bytes
                and sig.AccountSignature.authorizes_payer()
                for sig in self.TxSigs
            )
            if not has_payer_sig:
                raise ValueError("payer signature required")
        else:  # SplitPayerSelfPay
            for i in range(len(self.Instructions)):
                if i in sponsor_set:
                    continue
                has_gas = any(
                    sig.AccountSignature.authorizes_payer()
                    and sig.AccountSignature.authorizes_ix(i)
                    for sig in self.TxSigs
                )
                if not has_gas:
                    raise ValueError(f"gas signer required for ix {i}")

            for sig in self.TxSigs:
                has_payer = sig.AccountSignature.authorizes_payer()
                has_ix = (
                    sig.AccountSignature.AuthBit.raw()
                    & ((1 << AUTH_RESERVED_BIT) - 1)
                ) != 0
                if has_payer and not has_ix:
                    raise ValueError("gas payment mode conflict")

    def to_bytes(self) -> bytes:
        serializer = Serializer()
        self.marshal_postcard(serializer)
        return serializer.bytes()

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u64(self.Stamp)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Stamp: {exc}") from exc

        try:
            serialize_option(
                serializer, self.Payer, lambda s, a: a.marshal_postcard(s)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize Payer: {exc}") from exc

        try:
            serialize_seq(
                serializer, self.Instructions, lambda s, w: s.serialize_bytes(w)
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize Instructions: {exc}"
            ) from exc

        def _write_sig(s: Serializer, sig: TransactionSignatures) -> None:
            try:
                sig.Address.marshal_postcard(s)
            except ValueError as exc:
                raise ValueError(f"failed to serialize Address: {exc}") from exc
            try:
                sig.AccountSignature.marshal_postcard(s)
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize AccountSignature: {exc}"
                ) from exc

        try:
            serialize_seq(serializer, self.TxSigs, _write_sig)
        except ValueError as exc:
            raise ValueError(f"failed to serialize TxSigs: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.Stamp = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Stamp: {exc}") from exc

        def _read_addr(d: Deserializer) -> Address:
            addr = Address()
            try:
                addr.unmarshal_postcard(d)
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize Address: {exc}"
                ) from exc
            return addr

        try:
            self.Payer = deserialize_option(deserializer, _read_addr)
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Payer: {exc}") from exc

        def _read_instr(d: Deserializer) -> PackedInstruction:
            try:
                return d.deserialize_bytes()
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize PackedInstruction: {exc}"
                ) from exc

        try:
            self.Instructions = deserialize_seq(deserializer, _read_instr)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Instructions: {exc}"
            ) from exc

        def _read_sig(d: Deserializer) -> TransactionSignatures:
            ts = TransactionSignatures()
            try:
                ts.Address.unmarshal_postcard(d)
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize Address: {exc}"
                ) from exc
            try:
                ts.AccountSignature.unmarshal_postcard(d)
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize AccountSignature: {exc}"
                ) from exc
            return ts

        try:
            self.TxSigs = deserialize_seq(deserializer, _read_sig)
        except ValueError as exc:
            raise ValueError(f"failed to deserialize TxSigs: {exc}") from exc
