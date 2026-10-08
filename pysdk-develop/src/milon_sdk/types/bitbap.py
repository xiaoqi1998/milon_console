"""Bitmap64. Faithful port of gosdk-develop/types/bitbap.go."""

from __future__ import annotations

from ..postcard.serializer import Serializer
from ..postcard.deserializer import Deserializer


# Bits is the number of valid bit slots
BITS = 64


def _trailing_zeros64(value: int) -> int:
    """Go bits.TrailingZeros64 semantics: 64 when value == 0."""
    if value == 0:
        return 64
    return (value & -value).bit_length() - 1


class Bitmap64:
    """A 64-slot bitmap: bit i (from LSB) indicates whether index i is occupied."""

    def __init__(self, raw: int = 0) -> None:
        self._raw = raw & 0xFFFFFFFFFFFFFFFF

    def raw(self) -> int:
        """Returns the underlying uint64."""
        return self._raw

    def is_empty(self) -> bool:
        return self._raw == 0

    def test(self, bit: int) -> bool:
        """Checks whether the bit at the given position is 1."""
        if bit >= BITS:
            return False
        return (self._raw >> bit) & 1 != 0

    def set(self, bit: int) -> "Bitmap64":
        if bit >= BITS:
            return self
        return Bitmap64(self._raw | (1 << bit))

    def clear(self, bit: int) -> "Bitmap64":
        if bit >= BITS:
            return self
        return Bitmap64(self._raw & ~(1 << bit) & 0xFFFFFFFFFFFFFFFF)

    def count_ones(self) -> int:
        """Returns the number of set bits."""
        return self._raw.bit_count()

    def lowest_vacant_index(self) -> int:
        """Returns the lowest unset (vacant) index."""
        return _trailing_zeros64(~self._raw & 0xFFFFFFFFFFFFFFFF)

    def is_subset_of(self, other: "Bitmap64") -> bool:
        """Checks whether self is a subset of other."""
        return (self._raw & other._raw) == self._raw

    def iter_set_bits(self) -> list[int]:
        """Returns a list of indices where the bit is set."""
        value = self._raw
        result = []
        while value != 0:
            idx = (value & -value).bit_length() - 1
            result.append(idx)
            value &= value - 1  # clear the lowest set bit
        return result

    def __str__(self) -> str:
        return format(self._raw, "064b")

    def __repr__(self) -> str:
        return f"Bitmap64(0b{self._raw:064b})"

    def __eq__(self, other: object) -> bool:
        if isinstance(other, Bitmap64):
            return self._raw == other._raw
        if isinstance(other, int):
            return self._raw == other
        return NotImplemented

    def __hash__(self) -> int:
        return hash(self._raw)

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_u64(self._raw)

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            data = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Bitmap64: {exc}") from exc
        self._raw = data


def low_bits_mask(n: int) -> Bitmap64:
    """Returns a mask with the lowest n bits set to 1: (1 << n) - 1."""
    if n >= BITS:
        return Bitmap64(0xFFFFFFFFFFFFFFFF)
    return Bitmap64((1 << n) - 1)
