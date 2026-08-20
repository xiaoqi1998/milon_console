"""Bitmap64：64 槽位图（对应 Go types/bitbap.go）。

bit i（从 LSB 起）表示索引 i 是否被占用。
"""
from __future__ import annotations

from .postcard import Deserializer, Serializer

BITS = 64


class Bitmap64:
    __slots__ = ("_raw",)

    def __init__(self, raw: int = 0):
        if not (0 <= raw < (1 << 64)):
            raise ValueError(f"Bitmap64 raw out of range: {raw}")
        self._raw = raw

    # ---------------------------------------------------------- 构造
    @classmethod
    def new(cls, raw: int = 0) -> "Bitmap64":
        return cls(raw)

    @classmethod
    def low_bits_mask(cls, n: int) -> "Bitmap64":
        """低 n 位置 1：(1<<n)-1（n>=64 时全 1）。"""
        if n >= BITS:
            return cls((1 << 64) - 1)
        return cls((1 << n) - 1)

    # ---------------------------------------------------------- 查询
    def raw(self) -> int:
        return self._raw

    def is_empty(self) -> bool:
        return self._raw == 0

    def test(self, bit: int) -> bool:
        if not (0 <= bit < BITS):
            return False
        return (self._raw >> bit) & 1 != 0

    def count_ones(self) -> int:
        return self._raw.bit_count()

    def lowest_vacant_index(self) -> int:
        return (~self._raw & ((1 << 64) - 1)).bit_length() if self._raw != (1 << 64) - 1 else 64

    def is_subset_of(self, other: "Bitmap64") -> bool:
        return (self._raw & other._raw) == self._raw

    def iter_set_bits(self) -> list[int]:
        value = self._raw
        result: list[int] = []
        while value:
            idx = (value & -value).bit_length() - 1
            result.append(idx)
            value &= value - 1
        return result

    # ---------------------------------------------------------- 修改（返回新实例，与 Go 值语义一致）
    def set(self, bit: int) -> "Bitmap64":
        if not (0 <= bit < BITS):
            return self
        return Bitmap64(self._raw | (1 << bit))

    def clear(self, bit: int) -> "Bitmap64":
        if not (0 <= bit < BITS):
            return self
        return Bitmap64(self._raw & ~(1 << bit))

    # ---------------------------------------------------------- 展示
    def __str__(self) -> str:
        return f"{self._raw:064b}"

    def __repr__(self) -> str:
        return f"Bitmap64(0b{self._raw:064b})"

    def __int__(self) -> int:
        return self._raw

    def __eq__(self, other: object) -> bool:
        if isinstance(other, Bitmap64):
            return self._raw == other._raw
        if isinstance(other, int):
            return self._raw == other
        return NotImplemented

    def __hash__(self) -> int:
        return hash(self._raw)

    def __or__(self, other: "Bitmap64") -> "Bitmap64":
        return Bitmap64(self._raw | other._raw)

    def __and__(self, other: "Bitmap64") -> "Bitmap64":
        return Bitmap64(self._raw & other._raw)

    # ---------------------------------------------------------- 编解码
    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_u64(self._raw)

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "Bitmap64":
        return cls(d.deserialize_u64())
