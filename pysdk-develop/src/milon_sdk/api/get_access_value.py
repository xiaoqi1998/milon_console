"""GetAccessValueInfo. Faithful port of gosdk-develop/api/getAccessValue.go."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from ..postcard import Deserializer, Serializer
from .base import BLOB_HASH_LEN, BlobHash, TypeTagWithData


@dataclass
class GetAccessValueInfo:
    BlobHash: BlobHash = b"\x00" * BLOB_HASH_LEN
    Data: Optional[TypeTagWithData] = None

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_fixed_bytes(self.BlobHash)

        if self.Data is not None:
            try:
                serializer.serialize_bool(True)
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize Data presence: {exc}"
                ) from exc

            # Wrap [typeTag + value] in Vec<u8>
            inner_serializer = Serializer()
            try:
                inner_serializer.serialize_u64(self.Data.TypeTag)
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize Data TypeTag: {exc}"
                ) from exc
            inner_serializer.serialize_fixed_bytes(self.Data.Value)

            try:
                serializer.serialize_bytes(inner_serializer.bytes())
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize Data Vec<u8>: {exc}"
                ) from exc
        else:
            try:
                serializer.serialize_bool(False)
            except ValueError as exc:
                raise ValueError(
                    f"failed to serialize Data presence: {exc}"
                ) from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.BlobHash = bytes(
                deserializer.deserialize_fixed_bytes(BLOB_HASH_LEN)
            )
        except ValueError as exc:
            raise ValueError(f"failed to deserialize BlobHash: {exc}") from exc

        try:
            has_data = deserializer.deserialize_bool()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Data presence: {exc}"
            ) from exc

        if has_data:
            # Read Vec<u8>: [length varint] + [data]
            try:
                raw_data = deserializer.deserialize_bytes()
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize Data Vec<u8>: {exc}"
                ) from exc

            # Vec<u8> content is [typeTag varint + value]
            type_tag_reader = Deserializer(raw_data)
            try:
                type_tag = type_tag_reader.deserialize_u64()
            except ValueError as exc:
                raise ValueError(
                    "failed to deserialize Data TypeTag from Vec<u8>: "
                    f"{exc}"
                ) from exc

            # Value is everything after type_tag
            value_bytes = bytes(
                type_tag_reader.buffer()[type_tag_reader.offset() :]
            )

            self.Data = TypeTagWithData(TypeTag=type_tag, Value=value_bytes)
