"""GetTxHistoryProof. Faithful port of gosdk-develop/api/get_tx_history_proof.go."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List

from ..postcard import (
    Deserializer,
    Serializer,
    deserialize_seq,
    serialize_seq,
)
from .base import TX_HASH_LEN, TxHash
from .block import Block


@dataclass
class GetTxHistoryProof:
    Block: Block = field(default_factory=Block)
    Index: int = 0
    Siblings: List[TxHash] = field(default_factory=list)
    # History: TxHistory（Go 侧以原始字节过渡，见源码 todo 注释）
    History: bytes = b""

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            self.Block.marshal_postcard(serializer)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Block: {exc}") from exc
        try:
            serializer.serialize_u32(self.Index)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Index: {exc}") from exc
        try:
            serialize_seq(
                serializer, self.Siblings, lambda s, x: s.serialize_fixed_bytes(x)
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize Siblings: {exc}"
            ) from exc

        # todo----（与 Go 源一致：History 以 Vec<u8> 序列化）
        try:
            serializer.serialize_bytes(self.History)
        except ValueError as exc:
            raise ValueError(f"failed to serialize History: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        # 1. Block
        block = Block()
        try:
            block.unmarshal_postcard(deserializer)
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Block: {exc}") from exc
        self.Block = block

        # 2. Index (u32)
        try:
            self.Index = deserializer.deserialize_u32()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Index: {exc}") from exc

        # 3. Siblings (Vec<TxHash>)
        def _read_sibling(d: Deserializer) -> TxHash:
            try:
                return bytes(d.deserialize_fixed_bytes(TX_HASH_LEN))
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize Sibling: {exc}"
                ) from exc

        try:
            self.Siblings = deserialize_seq(deserializer, _read_sibling)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Siblings: {exc}"
            ) from exc

        # todo----（与 Go 源一致：History 以 Vec<u8> 反序列化）
        try:
            self.History = deserializer.deserialize_bytes()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize History: {exc}"
            ) from exc
