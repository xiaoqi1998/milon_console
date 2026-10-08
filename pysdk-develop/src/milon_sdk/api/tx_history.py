"""TxHistory / TxReceipt. Faithful port of gosdk-develop/api/txHistory.go."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List, Optional

from ..crypto.address import Address, new_address_from_bytes
from ..postcard import (
    Deserializer,
    Serializer,
    deserialize_option,
    deserialize_seq,
    serialize_option,
    serialize_seq,
)
from ..types.bitbap import Bitmap64
from .base import (
    TX_HASH_LEN,
    TX_ID_LEN,
    AccessRecord,
    PackedInstruction,
    TxHash,
    TxId,
    TypeTagWithData,
    deserialize_access_record_no_len,
    deserialize_event_entry_no_len,
    serialize_persisted_value_no_len,
)

TX_STATE_PENDING = 0
TX_STATE_SUCCESS = 1
TX_STATE_FAILED = 2


@dataclass
class TxHistorySignature:
    Signer: Address = field(default_factory=Address)
    AuthBit: Bitmap64 = field(default_factory=Bitmap64)
    SigBit: Bitmap64 = field(default_factory=Bitmap64)


@dataclass
class TxReceipt:
    TxID: TxId = b"\x00" * TX_ID_LEN
    TxHash: TxHash = b"\x00" * TX_HASH_LEN
    State: int = 0  # TxStatePending / TxStateSuccess / TxStateFailed
    Access: List[AccessRecord] = field(default_factory=list)
    Events: List[TypeTagWithData] = field(default_factory=list)
    Error: Optional[int] = None
    GasCharged: int = 0

    def marshal_postcard(self, serializer: Serializer) -> None:
        # 1. TxID (12 bytes)
        serializer.serialize_fixed_bytes(self.TxID)

        # 2. TxHash (32 bytes)
        serializer.serialize_fixed_bytes(self.TxHash)

        # 3. State (u8)
        try:
            serializer.serialize_u8(self.State)
        except ValueError as exc:
            raise ValueError(f"failed to serialize State: {exc}") from exc

        # 4. Access records (Vec<AccessRecord>)
        def _write_access(s: Serializer, rec: AccessRecord) -> None:
            s.serialize_fixed_bytes(rec.ResourceID)
            try:
                serialize_option(
                    s,
                    rec.FirstSnapshot,
                    lambda ss, pv: serialize_persisted_value_no_len(ss, pv),
                )
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize FirstSnapshot: {exc}"
                ) from exc
            try:
                serialize_persisted_value_no_len(s, rec.LastWritten)
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize LastWritten: {exc}"
                ) from exc

        try:
            serialize_seq(serializer, self.Access, _write_access)
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize Access records: {exc}"
            ) from exc

        # 5. Events (Vec<AnySerializeOwned>) — value has no length prefix
        def _write_event(s: Serializer, event: TypeTagWithData) -> None:
            try:
                s.serialize_u64(event.TypeTag)
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize event TypeTag: {exc}"
                ) from exc
            s.serialize_fixed_bytes(event.Value)

        try:
            serialize_seq(serializer, self.Events, _write_event)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Events: {exc}") from exc

        # 6. Error (Option<u16>)
        try:
            serialize_option(
                serializer, self.Error, lambda s, c: s.serialize_u16(c)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize Error: {exc}") from exc

        # 7. GasCharged (u64)
        try:
            serializer.serialize_u64(self.GasCharged)
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize GasCharged: {exc}"
            ) from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        # 1. TxID (12 bytes)
        try:
            self.TxID = bytes(deserializer.deserialize_fixed_bytes(TX_ID_LEN))
        except ValueError as exc:
            raise ValueError(f"failed to deserialize TxID: {exc}") from exc

        # 2. TxHash (32 bytes)
        try:
            self.TxHash = bytes(
                deserializer.deserialize_fixed_bytes(TX_HASH_LEN)
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize TxHash: {exc}"
            ) from exc

        # 3. State (u8)
        try:
            self.State = deserializer.deserialize_u8()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize State: {exc}") from exc

        # 4. Access records (Vec<AccessRecord>)
        try:
            self.Access = deserialize_seq(
                deserializer, deserialize_access_record_no_len
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Access records: {exc}"
            ) from exc

        # 5. Events (Vec<AnySerializeOwned>)
        try:
            self.Events = deserialize_seq(
                deserializer, deserialize_event_entry_no_len
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Events: {exc}"
            ) from exc

        # 6. Error (Option<u16>)
        try:
            self.Error = deserialize_option(
                deserializer, lambda d: d.deserialize_u16()
            )
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Error: {exc}") from exc

        # 7. GasCharged (u64)
        try:
            self.GasCharged = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize GasCharged: {exc}"
            ) from exc


@dataclass
class TxHistory:
    Stamp: int = 0
    Payer: Optional[int] = None
    Signatures: List[TxHistorySignature] = field(default_factory=list)
    Instructions: List[PackedInstruction] = field(default_factory=list)
    Receipt: TxReceipt = field(default_factory=TxReceipt)

    def marshal_postcard(self, serializer: Serializer) -> None:
        # 1. Stamp (u64)
        try:
            serializer.serialize_u64(self.Stamp)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Stamp: {exc}") from exc

        # 2. Payer (Option<u8>)
        try:
            serialize_option(
                serializer, self.Payer, lambda s, p: s.serialize_u8(p)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize Payer: {exc}") from exc

        # 3. Signatures (Vec<TxHistorySignature>)
        def _write_sig(s: Serializer, sig: TxHistorySignature) -> None:
            s.serialize_fixed_bytes(sig.Signer.Bytes)
            try:
                s.serialize_u64(sig.AuthBit.raw())
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize AuthBit: {exc}"
                ) from exc
            try:
                s.serialize_u64(sig.SigBit.raw())
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize SigBit: {exc}"
                ) from exc

        try:
            serialize_seq(serializer, self.Signatures, _write_sig)
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize Signatures: {exc}"
            ) from exc

        # 4. Instructions (Vec<PackedInstruction>)
        try:
            serialize_seq(
                serializer, self.Instructions, lambda s, i: s.serialize_bytes(i)
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize Instructions: {exc}"
            ) from exc

        # 5. Receipt (TxReceipt)
        try:
            self.Receipt.marshal_postcard(serializer)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Receipt: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        # 1. Stamp (u64)
        try:
            self.Stamp = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Stamp: {exc}") from exc

        # 2. Payer (Option<u8>)
        try:
            self.Payer = deserialize_option(
                deserializer, lambda d: d.deserialize_u8()
            )
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Payer: {exc}") from exc

        # 3. Signatures (Vec<TxHistorySignature>)
        def _read_sig(d: Deserializer) -> TxHistorySignature:
            sig = TxHistorySignature()
            # Signer (Address, 20 bytes)
            try:
                signer_bytes = d.deserialize_fixed_bytes(20)
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize Signer: {exc}"
                ) from exc
            try:
                sig.Signer = new_address_from_bytes(signer_bytes)
            except ValueError as exc:
                raise ValueError(f"failed to parse Signer: {exc}") from exc

            # AuthBit (Bitmap64)
            try:
                sig.AuthBit = Bitmap64(d.deserialize_u64())
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize AuthBit: {exc}"
                ) from exc

            # SigBit (Bitmap64)
            try:
                sig.SigBit = Bitmap64(d.deserialize_u64())
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize SigBit: {exc}"
                ) from exc
            return sig

        try:
            self.Signatures = deserialize_seq(deserializer, _read_sig)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Signatures: {exc}"
            ) from exc

        # 4. Instructions (Vec<PackedInstruction>)
        def _read_instr(d: Deserializer) -> PackedInstruction:
            try:
                return d.deserialize_bytes()
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize Instruction: {exc}"
                ) from exc

        try:
            self.Instructions = deserialize_seq(deserializer, _read_instr)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Instructions: {exc}"
            ) from exc

        # 5. Receipt (TxReceipt)
        receipt = TxReceipt()
        try:
            receipt.unmarshal_postcard(deserializer)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Receipt: {exc}"
            ) from exc
        self.Receipt = receipt
