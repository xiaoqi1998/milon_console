"""SimulateReceipt. Faithful port of gosdk-develop/api/simulateReceipt.go."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List, Optional

from ..postcard import (
    Deserializer,
    Serializer,
    deserialize_option,
    deserialize_seq,
    serialize_option,
    serialize_seq,
)
from .base import (
    TX_HASH_LEN,
    TX_ID_LEN,
    AccessRecord,
    TxHash,
    TxId,
    TypeTagWithData,
    deserialize_access_record,
    deserialize_event_entry,
    serialize_persisted_value,
)


@dataclass
class TxFailurePayload:
    Code: int = 0
    Message: str = ""
    Data: bytes = b""

    def marshal_postcard(self, serializer: Serializer) -> None:
        # Code (u16, varint)
        try:
            serializer.serialize_u16(self.Code)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Code: {exc}") from exc

        # Message (String)
        try:
            serializer.serialize_str(self.Message)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Message: {exc}") from exc

        # Data (Vec<u8>)
        try:
            serializer.serialize_bytes(self.Data)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Data: {exc}") from exc

    def unmarshal_postcard(self, d: Deserializer) -> None:
        try:
            self.Code = d.deserialize_u16()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Code: {exc}") from exc

        try:
            self.Message = d.deserialize_str()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Message: {exc}"
            ) from exc

        try:
            self.Data = d.deserialize_bytes()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Data: {exc}") from exc


@dataclass
class SimulateReceipt:
    Magic: bytes = b"\x00" * 4
    Version: int = 0
    TxID: TxId = b"\x00" * TX_ID_LEN
    TxHash: TxHash = b"\x00" * TX_HASH_LEN
    State: int = 0
    Access: List[AccessRecord] = field(default_factory=list)
    Events: List[TypeTagWithData] = field(default_factory=list)
    Error: Optional[TxFailurePayload] = None
    GasCharged: int = 0

    def marshal_postcard(self, serializer: Serializer) -> None:
        # 1. Magic (4 bytes)
        serializer.serialize_fixed_bytes(self.Magic)

        # 2. Version (u8)
        try:
            serializer.serialize_u8(self.Version)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Version: {exc}") from exc

        # 3. TxID (12 bytes)
        serializer.serialize_fixed_bytes(self.TxID)

        # 4. TxHash (32 bytes)
        serializer.serialize_fixed_bytes(self.TxHash)

        # 5. State (u8)
        try:
            serializer.serialize_u8(self.State)
        except ValueError as exc:
            raise ValueError(f"failed to serialize State: {exc}") from exc

        # 6. Access records (Vec<AccessRecord>)
        def _write_access(s: Serializer, rec: AccessRecord) -> None:
            s.serialize_fixed_bytes(rec.ResourceID)
            try:
                serialize_option(
                    s,
                    rec.FirstSnapshot,
                    lambda ss, pv: serialize_persisted_value(ss, pv),
                )
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize FirstSnapshot: {exc}"
                ) from exc
            try:
                serialize_persisted_value(s, rec.LastWritten)
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

        # 7. Events (Vec<FramedDynamicValue>)
        def _write_event(s: Serializer, event: TypeTagWithData) -> None:
            try:
                s.serialize_u64(event.TypeTag)
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize event TypeTag: {exc}"
                ) from exc
            # body: Vec<u8>: length-prefixed payload
            try:
                s.serialize_bytes(event.Value)
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize event body: {exc}"
                ) from exc

        try:
            serialize_seq(serializer, self.Events, _write_event)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Events: {exc}") from exc

        # 8. Error Option<TxFailurePayload>
        try:
            serialize_option(
                serializer, self.Error, lambda s, p: p.marshal_postcard(s)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize Error: {exc}") from exc

        # 9. GasCharged (u64)
        try:
            serializer.serialize_u64(self.GasCharged)
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize GasCharged: {exc}"
            ) from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        # 1. Magic (4 bytes)
        try:
            self.Magic = bytes(deserializer.deserialize_fixed_bytes(4))
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Magic: {exc}") from exc

        # 2. Version (u8)
        try:
            self.Version = deserializer.deserialize_u8()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Version: {exc}") from exc

        # 3. TxID (12 bytes)
        try:
            self.TxID = bytes(
                deserializer.deserialize_fixed_bytes(TX_ID_LEN)
            )
        except ValueError as exc:
            raise ValueError(f"failed to deserialize TxID: {exc}") from exc

        # 4. TxHash (32 bytes)
        try:
            self.TxHash = bytes(
                deserializer.deserialize_fixed_bytes(TX_HASH_LEN)
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize TxHash: {exc}"
            ) from exc

        # 5. State (u8)
        try:
            self.State = deserializer.deserialize_u8()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize State: {exc}") from exc

        # 6. Access records Vec<AccessRecord>
        try:
            self.Access = deserialize_seq(
                deserializer, deserialize_access_record
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Access records: {exc}"
            ) from exc

        # 7. Events Vec<AnySerializeOwned>
        try:
            self.Events = deserialize_seq(
                deserializer, deserialize_event_entry
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Events: {exc}"
            ) from exc

        # 8. Error Option<TxFailurePayload>
        def _read_error(d: Deserializer) -> TxFailurePayload:
            p = TxFailurePayload()
            p.unmarshal_postcard(d)
            return p

        try:
            self.Error = deserialize_option(deserializer, _read_error)
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Error: {exc}") from exc

        # 9. GasCharged (u64)
        try:
            self.GasCharged = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize GasCharged: {exc}"
            ) from exc
