"""RPC 响应类型与 Postcard 编解码（对应 Go api/*.go）。

每个类型实现 unmarshal_postcard(cls, d)，字段顺序与 Go 严格一致。
"""
from __future__ import annotations

from ..crypto.address import Address
from ..crypto.keys import PublicKey
from ..postcard import Deserializer, Serializer
from ..types import Bitmap64
from .base import (
    BLOB_HASH_LEN,
    RS_HASH_LEN,
    TX_HASH_LEN,
    TX_ID_LEN,
    BlobHash,
    RsHash,
    TxHash,
    TxId,
    AccessRecord,
    TypeTagWithData,
    deserialize_access_record,
    deserialize_event_entry,
    read_any_serialize_value_with_type_tag,
    serialize_persisted_value,
    unmarshal_rs_hash_from_json_array,
)

# ---------------------------------------------------------- ChainHead


class ChainHead:
    __slots__ = ("chain_id", "block_height", "block_hash", "timestamp_msecs")

    def __init__(self, chain_id: int, block_height: int, block_hash: TxHash, timestamp_msecs: int):
        self.chain_id = chain_id
        self.block_height = block_height
        self.block_hash = block_hash
        self.timestamp_msecs = timestamp_msecs

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "ChainHead":
        chain_id = d.deserialize_u64()
        block_height = d.deserialize_u64()
        block_hash = TxHash(d.deserialize_fixed_bytes(TX_HASH_LEN))
        timestamp_msecs = d.deserialize_u64()
        return cls(chain_id, block_height, block_hash, timestamp_msecs)

    def __repr__(self) -> str:
        return (
            f"ChainHead(chain_id={self.chain_id}, block_height={self.block_height}, "
            f"block_hash={self.block_hash.to_base58()}, timestamp_msecs={self.timestamp_msecs})"
        )


# ---------------------------------------------------------- Block


class Block:
    __slots__ = ("number", "hash", "prev_hash", "state_hash", "tx_root", "tx_count", "timestamp")

    def __init__(self, number, hash_, prev_hash, state_hash, tx_root, tx_count, timestamp):
        self.number = number
        self.hash = hash_
        self.prev_hash = prev_hash
        self.state_hash = state_hash
        self.tx_root = tx_root
        self.tx_count = tx_count
        self.timestamp = timestamp

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "Block":
        number = d.deserialize_u64()
        hash_ = TxHash(d.deserialize_fixed_bytes(TX_HASH_LEN))
        prev_hash = TxHash(d.deserialize_fixed_bytes(TX_HASH_LEN))
        state_hash = TxHash(d.deserialize_fixed_bytes(TX_HASH_LEN))
        tx_root = TxHash(d.deserialize_fixed_bytes(TX_HASH_LEN))
        tx_count = d.deserialize_u32()
        timestamp = d.deserialize_u64()
        return cls(number, hash_, prev_hash, state_hash, tx_root, tx_count, timestamp)

    def __repr__(self) -> str:
        return f"Block(number={self.number}, hash={self.hash.to_base58()}, tx_count={self.tx_count})"


# ---------------------------------------------------------- TxHistory / TxReceipt

TX_STATE_PENDING = 0
TX_STATE_SUCCESS = 1
TX_STATE_FAILED = 2


class TxHistorySignature:
    __slots__ = ("signer", "auth_bit", "sig_bit")

    def __init__(self, signer: Address, auth_bit: Bitmap64, sig_bit: Bitmap64):
        self.signer = signer
        self.auth_bit = auth_bit
        self.sig_bit = sig_bit


class TxReceipt:
    __slots__ = ("tx_id", "tx_hash", "state", "access", "events", "error", "gas_charged")

    def __init__(self, tx_id, tx_hash, state, access, events, error, gas_charged):
        self.tx_id = tx_id
        self.tx_hash = tx_hash
        self.state = state
        self.access = access
        self.events = events
        self.error = error
        self.gas_charged = gas_charged

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer, resolver=None) -> "TxReceipt":
        tx_id = TxId(d.deserialize_fixed_bytes(TX_ID_LEN))
        tx_hash = TxHash(d.deserialize_fixed_bytes(TX_HASH_LEN))
        state = d.deserialize_u8()
        access = d.deserialize_seq(lambda dd: deserialize_access_record(dd, resolver))
        events = d.deserialize_seq(lambda dd: deserialize_event_entry(dd, resolver))
        error = d.deserialize_option(lambda dd: dd.deserialize_u16())
        gas_charged = d.deserialize_u64()
        return cls(tx_id, tx_hash, state, access, events, error, gas_charged)


class TxHistory:
    __slots__ = ("stamp", "payer", "signatures", "instructions", "receipt")

    def __init__(self, stamp, payer, signatures, instructions, receipt):
        self.stamp = stamp
        self.payer = payer
        self.signatures = signatures
        self.instructions = instructions
        self.receipt = receipt

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer, resolver=None) -> "TxHistory":
        stamp = d.deserialize_u64()
        payer = d.deserialize_option(lambda dd: dd.deserialize_u8())
        signatures = d.deserialize_seq(_unmarshal_tx_history_signature)
        instructions = d.deserialize_seq(lambda dd: dd.deserialize_bytes())
        receipt = TxReceipt.unmarshal_postcard(d, resolver)
        return cls(stamp, payer, signatures, instructions, receipt)


def _unmarshal_tx_history_signature(d: Deserializer) -> TxHistorySignature:
    signer = Address.from_bytes(d.deserialize_fixed_bytes(20))
    auth_bit = Bitmap64(d.deserialize_u64())
    sig_bit = Bitmap64(d.deserialize_u64())
    return TxHistorySignature(signer, auth_bit, sig_bit)


# ---------------------------------------------------------- AccountView


class AccountView:
    __slots__ = ("address", "threshold", "public_keys_bs58")

    def __init__(self, address: Address, threshold: int, public_keys_bs58: list[str]):
        self.address = address
        self.threshold = threshold
        self.public_keys_bs58 = public_keys_bs58

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer, resolver=None) -> "AccountView":
        address = Address.unmarshal_postcard(d)
        threshold = d.deserialize_u8()
        public_keys = d.deserialize_seq(lambda dd: dd.deserialize_str())
        return cls(address, threshold, public_keys)

    def __repr__(self) -> str:
        return (
            f"AccountView(address={self.address.to_base58()}, threshold={self.threshold}, "
            f"signers={len(self.public_keys_bs58)})"
        )


# ---------------------------------------------------------- SimulateReceipt / TxFailurePayload


class TxFailurePayload:
    __slots__ = ("code", "message", "data")

    def __init__(self, code: int, message: str, data: bytes):
        self.code = code
        self.message = message
        self.data = bytes(data)

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "TxFailurePayload":
        code = d.deserialize_u16()
        message = d.deserialize_str()
        data = d.deserialize_bytes()
        return cls(code, message, data)

    def __repr__(self) -> str:
        return f"TxFailurePayload(code={self.code}, message={self.message!r})"


class SimulateReceipt:
    __slots__ = ("tx_id", "tx_hash", "state", "access", "events", "error", "gas_charged")

    def __init__(self, tx_id, tx_hash, state, access, events, error, gas_charged):
        self.tx_id = tx_id
        self.tx_hash = tx_hash
        self.state = state
        self.access = access
        self.events = events
        self.error = error  # Optional[TxFailurePayload]
        self.gas_charged = gas_charged

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer, resolver=None) -> "SimulateReceipt":
        tx_id = TxId(d.deserialize_fixed_bytes(TX_ID_LEN))
        tx_hash = TxHash(d.deserialize_fixed_bytes(TX_HASH_LEN))
        state = d.deserialize_u8()
        access = d.deserialize_seq(lambda dd: deserialize_access_record(dd, resolver))
        events = d.deserialize_seq(lambda dd: deserialize_event_entry(dd, resolver))
        error = d.deserialize_option(lambda dd: TxFailurePayload.unmarshal_postcard(dd))
        gas_charged = d.deserialize_u64()
        return cls(tx_id, tx_hash, state, access, events, error, gas_charged)


# ---------------------------------------------------------- EventsByTxHash


class EventsByTxHashReq:
    __slots__ = ("tx_hash", "type_tag_filter")

    def __init__(self, tx_hash: TxHash, type_tag_filter: int | None):
        self.tx_hash = tx_hash
        self.type_tag_filter = type_tag_filter

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.write(self.tx_hash.as_bytes())
        serializer.serialize_option(self.type_tag_filter is not None, lambda s: s.serialize_u64(self.type_tag_filter))


class EventEntry:
    __slots__ = ("block_height", "tx_hash", "tx_index", "event_index", "data")

    def __init__(self, block_height, tx_hash, tx_index, event_index, data: TypeTagWithData):
        self.block_height = block_height
        self.tx_hash = tx_hash
        self.tx_index = tx_index
        self.event_index = event_index
        self.data = data

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer, resolver=None) -> "EventEntry":
        block_height = d.deserialize_u64()
        tx_hash = TxHash(d.deserialize_fixed_bytes(TX_HASH_LEN))
        tx_index = d.deserialize_u32()
        event_index = d.deserialize_u32()
        raw_data = d.deserialize_bytes()
        type_tag = 0
        value = b""
        if raw_data:
            type_tag = Deserializer(raw_data).deserialize_u64()
            # 值 = type_tag 之后的所有字节（含可能嵌套的 resolver 内容）
            td = Deserializer(raw_data)
            type_tag = td.deserialize_u64()
            value = raw_data[td.offset():]
        return cls(block_height, tx_hash, tx_index, event_index, TypeTagWithData(type_tag, value))


class EventsByTxHash:
    __slots__ = ("events",)

    def __init__(self, events: list[EventEntry]):
        self.events = events

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer, resolver=None) -> "EventsByTxHash":
        events = d.deserialize_seq(lambda dd: EventEntry.unmarshal_postcard(dd, resolver))
        return cls(events)


# ---------------------------------------------------------- GetResource


class GetResource:
    __slots__ = ("data",)

    def __init__(self, data: TypeTagWithData):
        self.data = data

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer, resolver=None) -> "GetResource":
        type_tag = d.deserialize_u64()
        value = d.buffer[d.offset():]
        d.advance(len(value))
        return cls(TypeTagWithData(type_tag, value))


# ---------------------------------------------------------- GetTxHistoryProof


class GetTxHistoryProof:
    __slots__ = ("block", "index", "siblings", "history")

    def __init__(self, block: Block, index: int, siblings: list[TxHash], history: bytes):
        self.block = block
        self.index = index
        self.siblings = siblings
        self.history = history

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer, resolver=None) -> "GetTxHistoryProof":
        block = Block.unmarshal_postcard(d)
        index = d.deserialize_u32()
        siblings = d.deserialize_seq(lambda dd: TxHash(dd.deserialize_fixed_bytes(TX_HASH_LEN)))
        history = d.deserialize_bytes()
        return cls(block, index, siblings, history)


# ---------------------------------------------------------- GetAccessValue


class GetAccessValueInfo:
    __slots__ = ("blob_hash", "data")

    def __init__(self, blob_hash: BlobHash, data: TypeTagWithData | None):
        self.blob_hash = blob_hash
        self.data = data

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer, resolver=None) -> "GetAccessValueInfo":
        blob_hash = BlobHash(d.deserialize_fixed_bytes(BLOB_HASH_LEN))
        has_data = d.deserialize_bool()
        data = None
        if has_data:
            raw = d.deserialize_bytes()
            td = Deserializer(raw)
            type_tag = td.deserialize_u64()
            value = raw[td.offset():]
            data = TypeTagWithData(type_tag, value)
        return cls(blob_hash, data)


# ---------------------------------------------------------- BatchGetResourcePath


class BatchGetResourcePathInfo:
    __slots__ = ("rs_hash", "path", "err_msg")

    def __init__(self, rs_hash: RsHash, path: str = "", err_msg: str = ""):
        self.rs_hash = rs_hash
        self.path = path
        self.err_msg = err_msg

    @property
    def is_ok(self) -> bool:
        return self.err_msg == ""

    def __repr__(self) -> str:
        return f"BatchGetResourcePathInfo(rs_hash={self.rs_hash.to_base58()}, path={self.path!r})"


def unmarshal_batch_resource_path_list_from_raw_list(raw_list: list) -> list[BatchGetResourcePathInfo]:
    out: list[BatchGetResourcePathInfo] = []
    for item in raw_list:
        if len(item) < 2:
            raise ValueError("invalid BatchGetResourcePathInfo response")
        rs_hash = unmarshal_rs_hash_from_json_array(item[0])
        result_map = item[1]
        info = BatchGetResourcePathInfo(rs_hash)
        if isinstance(result_map, dict):
            if "Ok" in result_map and isinstance(result_map["Ok"], str):
                info.path = result_map["Ok"]
            elif "Err" in result_map and isinstance(result_map["Err"], str):
                info.err_msg = result_map["Err"]
        out.append(info)
    return out


# ---------------------------------------------------------- ListResourcePath（对应 Go api/listResourcePath.go）


class ListResourcePathInfo:
    """资源路径条目（对应 Go api.ListResourcePathInfo）：rs_hash + path。"""

    __slots__ = ("rs_hash", "path")

    def __init__(self, rs_hash: RsHash, path: str = ""):
        self.rs_hash = rs_hash
        self.path = path

    def __repr__(self) -> str:
        return f"ListResourcePathInfo(rs_hash={self.rs_hash.to_base58()}, path={self.path!r})"


def unmarshal_list_resource_path_list_from_raw_list(raw_list: list) -> list[ListResourcePathInfo]:
    """解析 [[rsHash字节数组, path字符串], ...] → 类型化列表（对应 Go UnmarshalListResourcePathListFromRawList）。"""
    out: list[ListResourcePathInfo] = []
    for item in raw_list:
        if len(item) < 2:
            raise ValueError("invalid ListResourcePathInfo response")
        rs_hash = unmarshal_rs_hash_from_json_array(item[0])
        path = item[1]
        if not isinstance(path, str):
            raise ValueError("invalid ListResourcePathInfo response")
        out.append(ListResourcePathInfo(rs_hash, path))
    return out
