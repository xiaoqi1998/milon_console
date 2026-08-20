"""协议基础类型（对应 Go api/base.go）。

定长类型：TxHash=32B / TxProofIdentifier=12B / TxId=12B / RsHash=18B / BlobHash=32B。
MIL 代币地址常量：M11on1111111111111111111111（Base58）。
"""
from __future__ import annotations

import base58  # type: ignore

from ..crypto.address import Address
from ..postcard import Deserializer, Serializer

TX_HASH_LEN = 32
TX_PROOF_IDENTIFIER_LEN = 12
TX_ID_LEN = 12
RS_HASH_LEN = 18
BLOB_HASH_LEN = 32

MIL = "M11on1111111111111111111111"
MIL_TOKEN = Address.from_base58(MIL)

PackedInstruction = bytes


class _FixedHash:
    """定长字节哈希基类。"""

    __slots__ = ("_bytes",)

    def __init__(self, raw: bytes):
        if len(raw) != self.LEN:  # type: ignore[attr-defined]
            raise ValueError(
                f"{type(self).__name__} expects {self.LEN} bytes, got {len(raw)}"
            )
        self._bytes = bytes(raw)

    def as_bytes(self) -> bytes:
        return self._bytes

    def to_hex(self) -> str:
        return self._bytes.hex()

    def to_base58(self) -> str:
        return base58.b58encode(self._bytes).decode("ascii")

    def __str__(self) -> str:
        return self.to_base58()

    def __repr__(self) -> str:
        return f"{type(self).__name__}({self.to_base58()})"

    def __bytes__(self) -> bytes:
        return self._bytes

    def __eq__(self, other: object) -> bool:
        if isinstance(other, _FixedHash):
            return type(other) is type(self) and self._bytes == other._bytes
        if isinstance(other, bytes):
            return self._bytes == other
        return NotImplemented

    def __hash__(self) -> int:
        return hash(self._bytes)

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.write(self._bytes)

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "_FixedHash":
        return cls(d.deserialize_fixed_bytes(cls.LEN))  # type: ignore[attr-defined]


class TxHash(_FixedHash):
    LEN = TX_HASH_LEN


class TxProofIdentifier(_FixedHash):
    LEN = TX_PROOF_IDENTIFIER_LEN


class TxId(_FixedHash):
    LEN = TX_ID_LEN


class RsHash(_FixedHash):
    LEN = RS_HASH_LEN


class BlobHash(_FixedHash):
    LEN = BLOB_HASH_LEN


# ---------------------------------------------------------- 宽松解析


def new_tx_hash_from_relaxed(value: object) -> TxHash:
    """接受 TxHash / bytes(32) / hex 字符串（先 hex 后 base58）。"""
    if isinstance(value, TxHash):
        return value
    if isinstance(value, bytes):
        return TxHash(value)
    if isinstance(value, str):
        s = value.strip()
        try:
            buf = bytes.fromhex(s)
        except ValueError:
            buf = base58.b58decode(s)
        if len(buf) != TX_HASH_LEN:
            raise ValueError(
                f"invalid decoded length: expected {TX_HASH_LEN}, got {len(buf)}"
            )
        return TxHash(buf)
    raise TypeError(f"unsupported type for TxHash: {type(value).__name__}")


def new_tx_hash_or_tx_id_from_relaxed(value: object) -> bytes:
    """接受 TxHash/TxId/bytes/hex/base58 字符串，返回原始字节。"""
    if isinstance(value, (TxHash, TxId)):
        return value.as_bytes()
    if isinstance(value, bytes):
        if len(value) in (TX_HASH_LEN, TX_ID_LEN):
            return value
        raise ValueError(
            f"invalid byte array length: expected {TX_HASH_LEN} or {TX_ID_LEN}, "
            f"got {len(value)}"
        )
    if isinstance(value, str):
        s = value.strip()
        try:
            buf = bytes.fromhex(s)
        except ValueError:
            buf = base58.b58decode(s)
        if len(buf) in (TX_HASH_LEN, TX_ID_LEN):
            return buf
        raise ValueError(
            f"invalid decoded length: expected {TX_HASH_LEN} or {TX_ID_LEN}, got {len(buf)}"
        )
    raise TypeError(f"unsupported type: {type(value).__name__}")


def unmarshal_rs_hash_from_json_array(raw: list) -> RsHash:
    """从 JSON 数字数组解析 RsHash（JSON 数字 → 字节）。"""
    if len(raw) > RS_HASH_LEN:
        raise ValueError(f"rsHash byte array length exceeds {RS_HASH_LEN}")
    buf = bytearray(RS_HASH_LEN)
    for i, b in enumerate(raw):
        if isinstance(b, (int, float)):
            buf[i] = int(b) & 0xFF
    return RsHash(bytes(buf))


# ---------------------------------------------------------- 带 type_tag 的值


class TypeTagWithData:
    """type_tag + 值字节（type_tag 外置，值不含前缀）。"""

    __slots__ = ("type_tag", "value")

    def __init__(self, type_tag: int, value: bytes):
        self.type_tag = type_tag
        self.value = bytes(value)

    def __repr__(self) -> str:
        return f"TypeTagWithData(type_tag={self.type_tag}, value={self.value.hex()[:32]}...)"


def deserialize_event_entry(d: Deserializer, resolver=None) -> TypeTagWithData:
    """事件条目：[type_tag(u64)] + [值字节]；有 resolver 时按 type_tag 解码消耗长度。"""
    type_tag = d.deserialize_u64()
    if resolver is not None:
        remaining = d.buffer[d.offset():]
        value_bytes, rest = resolver.decode_event(type_tag, remaining)
        consumed = len(remaining) - len(rest)
        d.advance(consumed)
        return TypeTagWithData(type_tag, value_bytes)
    # 无 resolver 回退：Vec<u8> 形态
    return TypeTagWithData(type_tag, d.deserialize_bytes())


def read_any_serialize_value_with_type_tag(d: Deserializer, type_tag: int, resolver=None) -> bytes:
    """按 type_tag 读取值字节；有 resolver 时按 IDL 解码消耗长度。"""
    if resolver is not None:
        remaining = d.buffer[d.offset():]
        value_bytes, rest = resolver.decode_resource(type_tag, remaining)
        consumed = len(remaining) - len(rest)
        d.advance(consumed)
        return value_bytes
    return d.deserialize_bytes()


# ---------------------------------------------------------- 资源访问记录


class PersistedValue:
    """变体 0=Inline(type_tag+data)，1=External(BlobHash)。"""

    __slots__ = ("variant", "type_tag", "inline_data", "external_hash")

    def __init__(
        self,
        variant: int,
        type_tag: int = 0,
        inline_data: bytes = b"",
        external_hash: bytes | None = None,
    ):
        self.variant = variant
        self.type_tag = type_tag
        self.inline_data = bytes(inline_data)
        self.external_hash = external_hash


class AccessRecord:
    __slots__ = ("resource_id", "first_snapshot", "last_written")

    def __init__(self, resource_id: RsHash, first_snapshot, last_written: PersistedValue):
        self.resource_id = resource_id
        self.first_snapshot = first_snapshot  # Optional[PersistedValue]
        self.last_written = last_written


def _deserialize_persisted_value(d: Deserializer, resolver=None) -> PersistedValue:
    variant = d.deserialize_u32()
    if variant == 0:
        type_tag = d.deserialize_u64()
        data = read_any_serialize_value_with_type_tag(d, type_tag, resolver)
        return PersistedValue(variant=0, type_tag=type_tag, inline_data=data)
    if variant == 1:
        ext = d.deserialize_fixed_bytes(BLOB_HASH_LEN)
        return PersistedValue(variant=1, external_hash=ext)
    raise ValueError(f"unknown PersistedValue variant: {variant}")


def deserialize_access_record(d: Deserializer, resolver=None) -> AccessRecord:
    rid = d.deserialize_fixed_bytes(RS_HASH_LEN)
    first = d.deserialize_option(lambda dd: _deserialize_persisted_value(dd, resolver))
    last = _deserialize_persisted_value(d, resolver)
    return AccessRecord(RsHash(rid), first, last)


def serialize_persisted_value(serializer: Serializer, pv: PersistedValue) -> None:
    serializer.serialize_u32(pv.variant)
    if pv.variant == 0:
        serializer.serialize_u64(pv.type_tag)
        serializer.write(pv.inline_data)
    elif pv.variant == 1:
        serializer.write(pv.external_hash or b"")
    else:
        raise ValueError(f"unknown PersistedValue variant: {pv.variant}")
