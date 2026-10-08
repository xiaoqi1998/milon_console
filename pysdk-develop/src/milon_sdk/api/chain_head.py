"""ChainHead. Faithful port of gosdk-develop/api/chainHead.go."""

from __future__ import annotations

from dataclasses import dataclass

from ..postcard import Deserializer, Serializer
from .base import TX_HASH_LEN, TxHash


@dataclass
class ChainHead:
    ChainId: int = 0
    BlockHeight: int = 0
    BlockHash: TxHash = b"\x00" * TX_HASH_LEN
    TimestampMsecs: int = 0

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u64(self.ChainId)
        except ValueError as exc:
            raise ValueError(f"failed to serialize ChainId: {exc}") from exc
        try:
            serializer.serialize_u64(self.BlockHeight)
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize BlockHeight: {exc}"
            ) from exc
        serializer.serialize_fixed_bytes(self.BlockHash)
        try:
            serializer.serialize_u64(self.TimestampMsecs)
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize TimestampMsecs: {exc}"
            ) from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.ChainId = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize ChainId: {exc}"
            ) from exc
        try:
            self.BlockHeight = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize BlockHeight: {exc}"
            ) from exc
        try:
            self.BlockHash = bytes(
                deserializer.deserialize_fixed_bytes(TX_HASH_LEN)
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize BlockHash: {exc}"
            ) from exc
        try:
            self.TimestampMsecs = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize TimestampMsecs: {exc}"
            ) from exc
