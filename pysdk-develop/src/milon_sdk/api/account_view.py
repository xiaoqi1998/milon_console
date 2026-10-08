"""AccountView. Faithful port of gosdk-develop/api/accountView.go."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List

from ..crypto.address import Address
from ..postcard import Deserializer, Serializer, deserialize_seq, serialize_seq


@dataclass
class AccountView:
    Address: Address = field(default_factory=Address)
    Threshold: int = 0
    PublicKeysBs58: List[str] = field(default_factory=list)

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            self.Address.marshal_postcard(serializer)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Address: {exc}") from exc

        try:
            serializer.serialize_u8(self.Threshold)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Threshold: {exc}") from exc

        try:
            serialize_seq(
                serializer,
                self.PublicKeysBs58,
                lambda s, pk: s.serialize_str(pk),
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize PublicKeysBs58: {exc}"
            ) from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.Address.unmarshal_postcard(deserializer)
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Address: {exc}") from exc

        try:
            self.Threshold = deserializer.deserialize_u8()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize Threshold: {exc}"
            ) from exc

        try:
            self.PublicKeysBs58 = deserialize_seq(
                deserializer, lambda d: d.deserialize_str()
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize PublicKeysBs58: {exc}"
            ) from exc
