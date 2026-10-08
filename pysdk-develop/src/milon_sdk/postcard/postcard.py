"""Postcard helpers. Faithful port of gosdk-develop/postcard/postcard.go."""

from __future__ import annotations

from typing import Callable, Optional, TypeVar

from .deserializer import Deserializer, TypeResolver
from .serializer import Marshaler, Serializer

T = TypeVar("T")


def serialize_postcard(value: Marshaler) -> bytes:
    serializer = Serializer()
    value.marshal_postcard(serializer)
    return serializer.bytes()


def serialize_seq(
    serializer: Serializer,
    values: list[T],
    fn: Callable[[Serializer, T], None],
) -> None:
    serializer.serialize_u32(len(values))
    for value in values:
        fn(serializer, value)


def serialize_option(
    serializer: Serializer,
    value: Optional[T],
    fn: Callable[[Serializer, T], None],
) -> None:
    serializer.serialize_bool(value is not None)
    if value is not None:
        fn(serializer, value)


def deserialize_postcard(
    data: bytes,
    fn: Callable[[Deserializer], T],
    allow_trailing: bool,
) -> T:
    """Deserializes Postcard-encoded data from a byte slice.

    It uses the provided deserializer function to extract structured data from
    the binary data, and can optionally verify data integrity (checks for
    trailing bytes).

    Parameters:
      - data: byte slice containing Postcard-encoded data
      - fn: deserializer function that extracts a value of the concrete type
        from the Deserializer
      - allow_trailing: whether unconsumed trailing bytes are allowed after
        deserialization
        - false: strict mode, all data must be consumed, otherwise an error is
          raised
        - true: lenient mode, trailing bytes are allowed without error
    """
    return deserialize_postcard_with_resolver(data, fn, allow_trailing, None)


def deserialize_postcard_with_resolver(
    data: bytes,
    fn: Callable[[Deserializer], T],
    allow_trailing: bool,
    resolver: Optional[TypeResolver],
) -> T:
    """Like deserialize_postcard, but injects a TypeResolver into the
    deserializer so type_tag-based decoding can resolve values against the
    caller's loaded IDLs."""
    deserializer = Deserializer(data)
    deserializer.set_type_resolver(resolver)
    value = fn(deserializer)
    if not allow_trailing:
        deserializer.assert_end()
    return value


def deserialize_value(
    deserializer: Deserializer,
    fn: Callable[[Deserializer], T],
) -> T:
    return fn(deserializer)


def deserialize_seq(
    deserializer: Deserializer,
    fn: Callable[[Deserializer], T],
) -> list[T]:
    length = deserializer.deserialize_u32()
    if length > deserializer.remaining():
        raise ValueError(
            f"seq length {length} exceeds remaining buffer {deserializer.remaining()}"
        )
    values: list[T] = []
    for _ in range(length):
        values.append(fn(deserializer))
    return values


def deserialize_option(
    deserializer: Deserializer,
    fn: Callable[[Deserializer], T],
) -> Optional[T]:
    has_value = deserializer.deserialize_bool()
    if not has_value:
        return None
    return fn(deserializer)
