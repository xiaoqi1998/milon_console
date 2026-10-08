"""Block. Faithful port of gosdk-develop/api/block.go."""

from __future__ import annotations

from dataclasses import dataclass, field

from ..postcard import Deserializer, Serializer
from .base import TX_HASH_LEN, TxHash


@dataclass
class Block:
    Number: int = 0
    Epoch: int = 0
    Slot: int = 0
    Hash: TxHash = b"\x00" * TX_HASH_LEN
    PrevHash: TxHash = b"\x00" * TX_HASH_LEN
    StateHash: TxHash = b"\x00" * TX_HASH_LEN
    TxRoot: TxHash = b"\x00" * TX_HASH_LEN
    TxCount: int = 0
    Timestamp: int = 0

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u64(self.Number)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Number: {exc}") from exc
        try:
            serializer.serialize_u64(self.Epoch)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Epoch: {exc}") from exc
        try:
            serializer.serialize_u64(self.Slot)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Slot: {exc}") from exc
        serializer.serialize_fixed_bytes(self.Hash)
        serializer.serialize_fixed_bytes(self.PrevHash)
        serializer.serialize_fixed_bytes(self.StateHash)
        serializer.serialize_fixed_bytes(self.TxRoot)
        try:
            serializer.serialize_u32(self.TxCount)
        except ValueError as exc:
            raise ValueError(f"failed to serialize TxCount: {exc}") from exc
        try:
            serializer.serialize_u64(self.Timestamp)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Timestamp: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.Number = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Number: {exc}") from exc
        try:
            self.Epoch = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Epoch: {exc}") from exc
        try:
            self.Slot = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Slot: {exc}") from exc
        for attr in ("Hash", "PrevHash", "StateHash", "TxRoot"):
            try:
                h = deserializer.deserialize_fixed_bytes(TX_HASH_LEN)
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize {attr}: {exc}"
                ) from exc
            setattr(self, attr, bytes(h))
        try:
            self.TxCount = deserializer.deserialize_u32()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize TxCount: {exc}"
            ) from exc
        try:
            self.Timestamp = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Timestamp: {exc}"
            ) from exc
