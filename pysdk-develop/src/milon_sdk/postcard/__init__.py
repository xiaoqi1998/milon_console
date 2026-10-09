from .serializer import Marshaler, Serializer
from .deserializer import Deserializer as _D

from typing import Any, Callable, Protocol as _Protocol


class Unmarshaler(_Protocol):
    """Go: postcard.Unmarshaler (unmarshal_postcard consumer)."""

    def unmarshal_postcard(self, deserializer: _D) -> None: ...


SerializerFunc = Callable[[Serializer, Any], None]


def new_serializer_with_cap(cap: int = 0) -> Serializer:
    """Go: NewSerializerWithCap (bytearray auto-grows; cap for signature compat)."""
    return Serializer()

from .deserializer import Deserializer, TypeResolver
from .postcard import (
    serialize_postcard,
    serialize_seq,
    serialize_option,
    deserialize_postcard,
    deserialize_postcard_with_resolver,
    deserialize_value,
    deserialize_seq,
    deserialize_option,
)

__all__ = [
    "Marshaler",
    "Unmarshaler",
    "SerializerFunc",
    "new_serializer_with_cap",
    "Serializer",
    "Deserializer",
    "TypeResolver",
    "serialize_postcard",
    "serialize_seq",
    "serialize_option",
    "deserialize_postcard",
    "deserialize_postcard_with_resolver",
    "deserialize_value",
    "deserialize_seq",
    "deserialize_option",
]
