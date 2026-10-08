"""EventsByTxHash. Faithful port of gosdk-develop/api/eventsByTxHash.go."""

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
from .base import TX_HASH_LEN, TxHash, TypeTagWithData


@dataclass
class EventsByTxHashReq:
    TxHash: TxHash = b"\x00" * TX_HASH_LEN
    TypeTagFilter: Optional[int] = None

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_fixed_bytes(self.TxHash)
        try:
            serialize_option(
                serializer, self.TypeTagFilter, lambda s, f: s.serialize_u64(f)
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize TypeTagFilter: {exc}"
            ) from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.TxHash = bytes(
                deserializer.deserialize_fixed_bytes(TX_HASH_LEN)
            )
        except ValueError as exc:
            raise ValueError(f"failed to deserialize TxHash: {exc}") from exc

        try:
            self.TypeTagFilter = deserialize_option(
                deserializer, lambda d: d.deserialize_u64()
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize TypeTagFilter: {exc}"
            ) from exc


@dataclass
class EventEntry:
    BlockHeight: int = 0
    TxHash: TxHash = b"\x00" * TX_HASH_LEN
    TxIndex: int = 0
    EventIndex: int = 0
    Data: TypeTagWithData = field(default_factory=TypeTagWithData)

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u64(self.BlockHeight)
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize BlockHeight: {exc}"
            ) from exc

        serializer.serialize_fixed_bytes(self.TxHash)

        try:
            serializer.serialize_u32(self.TxIndex)
        except ValueError as exc:
            raise ValueError(f"failed to serialize TxIndex: {exc}") from exc

        try:
            serializer.serialize_u32(self.EventIndex)
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize EventIndex: {exc}"
            ) from exc

        # serialize Data (type_tag + value)
        type_tag_serializer = Serializer()
        try:
            type_tag_serializer.serialize_u64(self.Data.TypeTag)
        except ValueError as exc:
            raise ValueError(f"failed to serialize TypeTag: {exc}") from exc
        data_bytes = type_tag_serializer.bytes() + self.Data.Value

        try:
            serializer.serialize_bytes(data_bytes)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Data: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.BlockHeight = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize BlockHeight: {exc}"
            ) from exc

        try:
            self.TxHash = bytes(
                deserializer.deserialize_fixed_bytes(TX_HASH_LEN)
            )
        except ValueError as exc:
            raise ValueError(f"failed to deserialize TxHash: {exc}") from exc

        try:
            self.TxIndex = deserializer.deserialize_u32()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize TxIndex: {exc}") from exc

        try:
            self.EventIndex = deserializer.deserialize_u32()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize EventIndex: {exc}"
            ) from exc

        # read the full Data (type_tag + value)
        try:
            raw_data = deserializer.deserialize_bytes()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Data: {exc}") from exc
        if raw_data is None:
            raw_data = b""

        # parse TypeTag and Value from Data
        if len(raw_data) > 0:
            type_tag_reader = Deserializer(raw_data)
            try:
                self.Data.TypeTag = type_tag_reader.deserialize_u64()
            except ValueError as exc:
                raise ValueError(
                    f"failed to parse TypeTag from Data: {exc}"
                ) from exc
            # Value is everything after type_tag
            self.Data.Value = raw_data[type_tag_reader.offset() :]
        else:
            self.Data.Value = b""


@dataclass
class EventsByTxHash:
    Events: List[EventEntry] = field(default_factory=list)

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serialize_seq(
                serializer, self.Events, lambda s, e: e.marshal_postcard(s)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize Events: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        def _read(d: Deserializer) -> EventEntry:
            entry = EventEntry()
            entry.unmarshal_postcard(d)
            return entry

        try:
            self.Events = deserialize_seq(deserializer, _read)
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Events: {exc}") from exc
