"""api 包基础类型. Faithful port of gosdk-develop/api/base.go."""

from __future__ import annotations

import binascii
from dataclasses import dataclass
from typing import Any, Optional

import base58

from ..postcard import Deserializer, Serializer, deserialize_option
from ..postcard.deserializer import Deserializer as _D
from ..postcard.serializer import Serializer as _S
from ..postcard.postcard import deserialize_option as _deser_opt

# 定长哈希类型（Go [N]byte 值类型 → Python bytes + 长度约定）
PackedInstruction = bytes

TX_HASH_LEN = 32
TX_PROOF_IDENTIFIER_LEN = 12
TX_ID_LEN = 12
RS_HASH_LEN = 18
BLOB_HASH_LEN = 32

TxHash = bytes
TxProofIdentifier = bytes
TxId = bytes
RsHash = bytes
BlobHash = bytes

MIL = "M11on1111111111111111111111"

from ..crypto.address import Address, new_address_from_bytes  # noqa: E402


def _init_mil_token() -> Address:
    try:
        return new_address_from_bytes(base58.b58decode(MIL))
    except ValueError as exc:
        raise RuntimeError(f"failed to decode MIL token address: {exc}") from exc


MIL_TOKEN: Address = _init_mil_token()


def tx_hash_str(tx_hash: bytes) -> str:
    """Go: TxHash.String()（Base58 格式）。"""
    return base58.b58encode(tx_hash).decode()


def tx_hash_to_hex(tx_hash: bytes) -> str:
    return tx_hash.hex()


def tx_hash_to_base58(tx_hash: bytes) -> str:
    return base58.b58encode(tx_hash).decode()


def tx_id_to_hex(tx_id: bytes) -> str:
    return tx_id.hex()


def tx_id_to_base58(tx_id: bytes) -> str:
    return base58.b58encode(tx_id).decode()


def new_tx_hash_from_relaxed(input: Any) -> TxHash:
    if isinstance(input, bytes):
        if len(input) != TX_HASH_LEN:
            raise ValueError(
                f"invalid hex decoded length: expected {TX_HASH_LEN}, got "
                f"{len(input)}"
            )
        return bytes(input)
    if isinstance(input, str):
        # try hex decode first
        try:
            buf = binascii.unhexlify(input)
        except (binascii.Error, ValueError):
            buf = None
        if buf is not None:
            if len(buf) != TX_HASH_LEN:
                raise ValueError(
                    f"invalid hex decoded length: expected {TX_HASH_LEN}, "
                    f"got {len(buf)}"
                )
            return bytes(buf)

        # try base58 decode if hex fails
        try:
            buf = base58.b58decode(input)
        except ValueError as exc:
            raise ValueError(
                f"invalid base58 decoded length: expected {TX_HASH_LEN}, got 0"
            ) from exc
        if len(buf) != TX_HASH_LEN:
            raise ValueError(
                f"invalid base58 decoded length: expected {TX_HASH_LEN}, got "
                f"{len(buf)}"
            )
        return bytes(buf)
    raise ValueError(
        f"unsupported type for TxHash: {type(input).__name__} "
        "(expected string or api.TxHash)"
    )


def new_tx_hash_or_tx_id_from_relaxed(input: Any) -> bytes:
    if isinstance(input, bytes):
        if len(input) in (TX_HASH_LEN, TX_ID_LEN):
            return bytes(input)
        raise ValueError(
            f"invalid byte array length: expected {TX_HASH_LEN} or "
            f"{TX_ID_LEN}, got {len(input)}"
        )
    if isinstance(input, str):
        try:
            buf = binascii.unhexlify(input)
        except (binascii.Error, ValueError):
            buf = None
        if buf is not None:
            if len(buf) in (TX_HASH_LEN, TX_ID_LEN):
                return bytes(buf)
            raise ValueError(
                f"invalid hex decoded length: expected {TX_HASH_LEN} or "
                f"{TX_ID_LEN}, got {len(buf)}"
            )

        try:
            buf = base58.b58decode(input)
        except ValueError as exc:
            raise ValueError(
                f"invalid base58 decoded length: expected {TX_HASH_LEN} or "
                f"{TX_ID_LEN}, got 0"
            ) from exc
        if len(buf) in (TX_HASH_LEN, TX_ID_LEN):
            return bytes(buf)
        raise ValueError(
            f"invalid base58 decoded length: expected {TX_HASH_LEN} or "
            f"{TX_ID_LEN}, got {len(buf)}"
        )
    raise ValueError(
        f"unsupported type for TxHash: {type(input).__name__} "
        "(expected string or api.TxHash)"
    )


def unmarshal_rs_hash_from_json_array(raw: list) -> RsHash:
    """Parses an RsHash from a JSON number array. Each element is a
    number converted to byte."""
    rs_hash = bytearray(RS_HASH_LEN)
    for i, b in enumerate(raw):
        if i >= RS_HASH_LEN:
            raise ValueError(f"rsHash byte array length exceeds {RS_HASH_LEN}")
        if isinstance(b, (int, float)):
            rs_hash[i] = int(b) & 0xFF
    return bytes(rs_hash)


@dataclass
class TypeTagWithData:
    """Contains type_tag and value bytes."""

    TypeTag: int = 0
    Value: bytes = b""


def deserialize_event_entry(d: Deserializer) -> TypeTagWithData:
    """Deserializes an event entry (type_tag + value) from postcard format.
    Used by SimulateReceipt."""
    try:
        type_tag = d.deserialize_u64()
    except ValueError as exc:
        raise ValueError(f"failed to deserialize event type_tag: {exc}") from exc

    # body: Vec<u8>: length prefix + payload bytes
    try:
        raw = d.deserialize_bytes()
    except ValueError as exc:
        raise ValueError(
            f"failed to read event body (type_tag={type_tag}): {exc}"
        ) from exc

    resolver = d.type_resolver()
    if resolver is not None:
        try:
            event_bytes, _ = resolver.decode_event(type_tag, raw)
        except ValueError as exc:
            raise ValueError(
                f"TypeResolver.DecodeEvent failed (type_tag={type_tag}): {exc}"
            ) from exc
        return TypeTagWithData(TypeTag=type_tag, Value=event_bytes)

    return TypeTagWithData(TypeTag=type_tag, Value=raw)


def deserialize_event_entry_no_len(d: Deserializer) -> TypeTagWithData:
    """Deserializes an event entry (type_tag + value, value WITHOUT length
    prefix) from postcard format. Used by TxHistory."""
    try:
        type_tag = d.deserialize_u64()
    except ValueError as exc:
        raise ValueError(f"failed to deserialize event type_tag: {exc}") from exc

    resolver = d.type_resolver()
    if resolver is not None:
        remaining = d.buffer()[d.offset() :]
        try:
            event_bytes, rest = resolver.decode_event(type_tag, remaining)
        except ValueError as exc:
            raise ValueError(
                f"TypeResolver.DecodeEvent failed (type_tag={type_tag}): {exc}"
            ) from exc

        consumed = len(remaining) - len(rest)
        try:
            d.advance(consumed)
        except ValueError as exc:
            raise ValueError(
                f"advance failed after DecodeEvent: {exc}"
            ) from exc
        return TypeTagWithData(TypeTag=type_tag, Value=event_bytes)

    try:
        val = d.deserialize_bytes()
    except ValueError as exc:
        raise ValueError(
            f"unknown event type_tag {type_tag} (no TypeResolver), fallback "
            f"DeserializeBytes failed: {exc}"
        ) from exc
    return TypeTagWithData(TypeTag=type_tag, Value=val)


@dataclass
class AccessRecord:
    ResourceID: RsHash = b"\x00" * RS_HASH_LEN
    FirstSnapshot: Optional["PersistedValue"] = None
    LastWritten: "PersistedValue" = None


@dataclass
class PersistedValue:
    Variant: int = 0
    TypeTag: int = 0  # Inline type_tag (only valid when Variant==0)
    InlineData: bytes = b""  # Inline raw value bytes (only valid when Variant==0)
    ExternalHash: bytes = b"\x00" * BLOB_HASH_LEN  # External BlobHash (Variant==1)


def serialize_persisted_value(serializer: Serializer, pv: PersistedValue) -> None:
    """Serializes a PersistedValue; Inline values carry a length prefix.
    Used by SimulateReceipt."""
    try:
        serializer.serialize_u32(pv.Variant)
    except ValueError as exc:
        raise ValueError(f"failed to serialize variant: {exc}") from exc

    if pv.Variant == 0:
        # Inline(FramedDynamicValue): type_tag + body(Vec<u8>, length-prefixed)
        try:
            serializer.serialize_u64(pv.TypeTag)
        except ValueError as exc:
            raise ValueError(f"failed to serialize type_tag: {exc}") from exc
        try:
            serializer.serialize_bytes(pv.InlineData)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Inline body: {exc}") from exc
    elif pv.Variant == 1:
        # External(BlobHash)
        serializer.serialize_fixed_bytes(pv.ExternalHash)
    else:
        raise ValueError(f"unknown PersistedValue variant: {pv.Variant}")


def serialize_persisted_value_no_len(
    serializer: Serializer, pv: PersistedValue
) -> None:
    """Serializes a PersistedValue; Inline values carry no length prefix.
    Used by TxHistory."""
    try:
        serializer.serialize_u32(pv.Variant)
    except ValueError as exc:
        raise ValueError(f"failed to serialize variant: {exc}") from exc

    if pv.Variant == 0:
        # Inline(AnySerializeOwned): type_tag + value (no length prefix)
        try:
            serializer.serialize_u64(pv.TypeTag)
        except ValueError as exc:
            raise ValueError(f"failed to serialize type_tag: {exc}") from exc
        serializer.serialize_fixed_bytes(pv.InlineData)
    elif pv.Variant == 1:
        serializer.serialize_fixed_bytes(pv.ExternalHash)
    else:
        raise ValueError(f"unknown PersistedValue variant: {pv.Variant}")


def deserialize_access_record(d: Deserializer) -> AccessRecord:
    """Deserializes an AccessRecord from postcard format. Used by
    SimulateReceipt."""
    # ResourceID (18 bytes)
    try:
        rid = d.deserialize_fixed_bytes(RS_HASH_LEN)
    except ValueError as exc:
        raise ValueError(f"failed to deserialize ResourceID: {exc}") from exc

    # FirstSnapshot: Option<PersistedValue>
    try:
        first_snapshot = _deser_opt(d, _deserialize_persisted_value)
    except ValueError as exc:
        raise ValueError(f"failed to deserialize FirstSnapshot: {exc}") from exc

    # LastWritten: PersistedValue (non-Option)
    try:
        last_written = _deserialize_persisted_value(d)
    except ValueError as exc:
        raise ValueError(f"failed to deserialize LastWritten: {exc}") from exc

    return AccessRecord(
        ResourceID=bytes(rid), FirstSnapshot=first_snapshot, LastWritten=last_written
    )


def _deserialize_persisted_value(d: Deserializer) -> PersistedValue:
    try:
        variant = d.deserialize_u32()
    except ValueError as exc:
        raise ValueError(f"failed to read variant: {exc}") from exc

    if variant == 0:
        # Inline(FramedDynamicValue): type_tag + body(Vec<u8>, length-prefixed)
        try:
            type_tag = d.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to read type_tag: {exc}") from exc

        try:
            raw = d.deserialize_bytes()
        except ValueError as exc:
            raise ValueError(
                f"failed to read Inline body (type_tag={type_tag}): {exc}"
            ) from exc

        resolver = d.type_resolver()
        if resolver is not None:
            try:
                value_bytes, _ = resolver.decode_resource(type_tag, raw)
            except ValueError as exc:
                raise ValueError(
                    f"TypeTagWithDataResolver.DecodeResource failed "
                    f"(type_tag={type_tag}): {exc}"
                ) from exc
            inline_data = value_bytes
        else:
            inline_data = raw

        return PersistedValue(
            Variant=variant, TypeTag=type_tag, InlineData=inline_data
        )
    if variant == 1:
        # External(BlobHash)
        try:
            h = d.deserialize_fixed_bytes(BLOB_HASH_LEN)
        except ValueError as exc:
            raise ValueError(
                f"failed to read External BlobHash: {exc}"
            ) from exc
        return PersistedValue(Variant=variant, ExternalHash=bytes(h))
    raise ValueError(f"unknown PersistedValue variant: {variant}")


def deserialize_access_record_no_len(d: Deserializer) -> AccessRecord:
    """Deserializes an AccessRecord whose Inline/Event values carry NO length
    prefix (TxHistory/TxReceipt wire format). Used by TxHistory."""
    try:
        rid = d.deserialize_fixed_bytes(RS_HASH_LEN)
    except ValueError as exc:
        raise ValueError(f"failed to deserialize ResourceID: {exc}") from exc

    try:
        first_snapshot = _deser_opt(d, _deserialize_persisted_value_no_len)
    except ValueError as exc:
        raise ValueError(f"failed to deserialize FirstSnapshot: {exc}") from exc

    try:
        last_written = _deserialize_persisted_value_no_len(d)
    except ValueError as exc:
        raise ValueError(f"failed to deserialize LastWritten: {exc}") from exc

    return AccessRecord(
        ResourceID=bytes(rid), FirstSnapshot=first_snapshot, LastWritten=last_written
    )


def _deserialize_persisted_value_no_len(d: Deserializer) -> PersistedValue:
    try:
        variant = d.deserialize_u32()
    except ValueError as exc:
        raise ValueError(f"failed to read variant: {exc}") from exc

    if variant == 0:
        # Inline(AnySerializeOwned): type_tag + value (no length prefix)
        try:
            type_tag = d.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to read type_tag: {exc}") from exc

        resolver = d.type_resolver()
        if resolver is not None:
            remaining = d.buffer()[d.offset() :]
            try:
                value_bytes, rest = resolver.decode_resource(type_tag, remaining)
            except ValueError as exc:
                raise ValueError(
                    f"TypeTagWithDataResolver.DecodeResource failed "
                    f"(type_tag={type_tag}): {exc}"
                ) from exc

            consumed = len(remaining) - len(rest)
            try:
                d.advance(consumed)
            except ValueError as exc:
                raise ValueError(
                    f"Advance failed after DecodeResource: {exc}"
                ) from exc
            inline_data = value_bytes
        else:
            try:
                val = d.deserialize_bytes()
            except ValueError as exc:
                raise ValueError(
                    f"unknown type_tag {type_tag} (no TypeResolver), fallback "
                    f"DeserializeBytes failed: {exc}"
                ) from exc
            inline_data = val

        return PersistedValue(
            Variant=variant, TypeTag=type_tag, InlineData=inline_data
        )
    if variant == 1:
        try:
            h = d.deserialize_fixed_bytes(BLOB_HASH_LEN)
        except ValueError as exc:
            raise ValueError(
                f"failed to read External BlobHash: {exc}"
            ) from exc
        return PersistedValue(Variant=variant, ExternalHash=bytes(h))
    raise ValueError(f"unknown PersistedValue variant: {variant}")


class TypeTagWithDataResolver:
    """Deprecated: use postcard.TypeResolver, injected via
    postcard.Deserializer.set_type_resolver.（协议接口，Python 侧以 Protocol
    语义使用；保留类型以对齐 Go 导出面。）"""
