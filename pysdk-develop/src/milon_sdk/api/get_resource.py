"""GetResource. Faithful port of gosdk-develop/api/getResource.go."""

from __future__ import annotations

from dataclasses import dataclass, field

from ..postcard import Deserializer, Serializer
from .base import TypeTagWithData


@dataclass
class GetResource:
    Data: TypeTagWithData = field(default_factory=TypeTagWithData)

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u64(self.Data.TypeTag)
        except ValueError as exc:
            raise ValueError(f"failed to serialize TypeTag: {exc}") from exc
        serializer.serialize_fixed_bytes(self.Data.Value)

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.Data.TypeTag = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize TypeTag: {exc}") from exc

        remaining = deserializer.buffer()[deserializer.offset() :]
        self.Data.Value = bytes(remaining)
        try:
            deserializer.advance(len(remaining))
        except ValueError as exc:
            raise ValueError(
                f"failed to advance deserializer: {exc}"
            ) from exc
