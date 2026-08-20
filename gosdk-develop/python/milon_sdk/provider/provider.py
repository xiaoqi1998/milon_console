"""IDL 提供器：指令/视图/事件编解码（对应 Go provider/provider.go）。

⚠️ 字节级 parity 纪律：
- 编码路径使用 postcard.Serializer（u8 为单字节，u16/u32/u64 为变长）。
- 解码路径与 Go 完全一致，采用"offset + decodeViewVarUint"（LE 7-bit 变长）
  的偏移式解析——包括 Go 对 u8 也走变长读取这一行为，保证跨语言逐字节一致。
"""
from __future__ import annotations

import json
import math
from typing import Any, Optional

from ..crypto.address import Address
from ..crypto.keys import PublicKey, PublicKeyType
from ..postcard import Serializer
from .idl_types import IDL, IDLType, Instruction, StructField, DecodedTaggedValue

U128_MAX = (1 << 128) - 1


class Provider:
    def __init__(self, idl: IDL):
        self.idl = idl
        self.instruction_by_name: dict[str, Instruction] = {}
        self.instruction_by_discriminator: dict[int, Instruction] = {}
        self.idl_type_by_name: dict[str, IDLType] = {}
        self.idl_type_by_type_tag: dict[int, IDLType] = {}
        self.event_by_type_tag: dict[int, Any] = {}

        for ins in idl.instructions:
            self.instruction_by_name[ins.name] = ins
            self.instruction_by_discriminator[ins.discriminator] = ins

        for t in idl.types:
            self.idl_type_by_name[t.name] = t
            self.idl_type_by_type_tag[t.type_tag] = t

        for event in idl.events:
            # 事件本质上等同于 struct，统一按 IDLType 处理
            idl_type = IDLType(
                name=event.name,
                type_tag=event.type_tag,
                kind="struct",
                fields=[StructField(name=f.name, type=f.type) for f in event.fields],
            )
            if idl_type.name not in self.idl_type_by_name:
                self.idl_type_by_name[idl_type.name] = idl_type
            if idl_type.type_tag not in self.idl_type_by_type_tag:
                self.idl_type_by_type_tag[idl_type.type_tag] = idl_type
            self.event_by_type_tag[event.type_tag] = event

    # ---------------------------------------------------------- 访问
    def app_id(self) -> int:
        return self.idl.metadata.app_id

    def get_instruction_by_name(self, name: str) -> Instruction:
        ins = self.instruction_by_name.get(name)
        if ins is None:
            raise ValueError(f"IDL method not found: {name}")
        return ins

    def get_idl_type_by_type_tag(self, type_tag: int) -> Optional[IDLType]:
        return self.idl_type_by_type_tag.get(type_tag)

    def get_event_by_type_tag(self, type_tag: int):
        return self.event_by_type_tag.get(type_tag)

    # ================================================================
    # 编码（编码指令 → 线上字节）
    # ================================================================

    def encode(self, instruction_name: str, args: dict) -> bytes:
        instruction = self.get_instruction_by_name(instruction_name)
        if instruction.kind not in ("entry", "view"):
            raise ValueError(
                f"unsupported instruction kind: {instruction.kind} (expected 'entry' or 'view')"
            )
        return self._encode_instruction(instruction, args)

    def _encode_instruction(self, instruction: Instruction, args: dict) -> bytes:
        s = Serializer()
        # 1. app_id（1 字节）
        s.serialize_u8(self.app_id())
        # 2. discriminator（u16 LE，2 字节）
        s.write(bytes([instruction.discriminator & 0xFF, (instruction.discriminator >> 8) & 0xFF]))
        # 3. 按 IDL 顺序序列化 args
        for arg in instruction.args:
            if arg.name not in args:
                raise ValueError(f"missing IDL argument: {arg.name}")
            self.serialize_value(s, arg.type.strip(), args[arg.name])
        return s.bytes()

    def serialize_value(self, serializer: Serializer, type_name: str, value: Any) -> None:
        """写一个值（对应 Go serializeValue）。"""
        # vec<T>
        inner = parse_wrapped_type(type_name, "vec")
        if inner is not None:
            items = slice_values(value)
            serializer.serialize_u32(len(items))
            for item in items:
                self.serialize_value(serializer, inner, item)
            return

        # option<T>
        inner = parse_wrapped_type(type_name, "option")
        if inner is not None:
            if value is None or is_nil(value):
                serializer.serialize_bool(False)
                return
            serializer.serialize_bool(True)
            self.serialize_value(serializer, inner, value)
            return

        # map<K,V>
        parsed_map = parse_map_type(type_name)
        if parsed_map is not None:
            key_type, value_type = parsed_map
            entries = map_entries(value)
            serializer.serialize_u32(len(entries))
            for k, v in entries:
                self.serialize_value(serializer, key_type, k)
                self.serialize_value(serializer, value_type, v)
            return

        # tuple<T1,T2,...>
        tuple_types = parse_tuple_type(type_name)
        if tuple_types is not None:
            tup = tuple_values(value, len(tuple_types))
            for i, item_type in enumerate(tuple_types):
                self.serialize_value(serializer, item_type, tup[i])
            return

        # 自定义 IDL 类型
        idl_type = self.idl_type_by_name.get(type_name)
        if idl_type is not None:
            if idl_type.kind == "struct":
                record = value
                if not isinstance(record, dict):
                    raise ValueError(f"{type_name} expects an object")
                for field_ in idl_type.fields:
                    if field_.name not in record:
                        raise ValueError(f"missing struct field: {field_.name}")
                    self.serialize_value(serializer, field_.type, record[field_.name])
                return
            if idl_type.kind == "enum":
                self._serialize_enum(serializer, idl_type, value)
                return
            if idl_type.kind != "builtin":
                raise ValueError(f"unsupported type kind: {idl_type.kind} for type {type_name}")
            # builtin 落到下方基础类型分支

        # 基础类型
        if type_name in ("Address", "Signer", "AnySigner"):
            serialize_address(serializer, value)
        elif type_name == "PublicKey":
            serialize_public_key(serializer, value)
        elif type_name in ("String", "string"):
            serializer.serialize_str(str(value))
        elif type_name in ("bool", "boolean"):
            if not isinstance(value, bool):
                raise ValueError(f"{type_name} expects a boolean")
            serializer.serialize_bool(value)
        elif type_name == "u8":
            serializer.serialize_u8(as_uint64(value, 0xFF, "u8"))
        elif type_name == "u16":
            serializer.serialize_u16(as_uint64(value, 0xFFFF, "u16"))
        elif type_name == "u32":
            serializer.serialize_u32(as_uint64(value, 0xFFFFFFFF, "u32"))
        elif type_name in ("u64", "Bitmap64", "Amount", "Epoch"):
            serializer.serialize_u64(as_uint64(value, 0xFFFFFFFFFFFFFFFF, "u64"))
        elif type_name == "u128":
            serializer.serialize_u128(as_big_int(value, signed=False))
        elif type_name == "i8":
            serializer.serialize_i8(as_int64(value, -128, 127, "i8"))
        elif type_name == "i16":
            serializer.serialize_i16(as_int64(value, -32768, 32767, "i16"))
        elif type_name == "i32":
            serializer.serialize_i32(as_int64(value, -2147483648, 2147483647, "i32"))
        elif type_name == "i64":
            serializer.serialize_i64(as_int64(value, -9223372036854775808, 9223372036854775807, "i64"))
        elif type_name == "bytes":
            if not isinstance(value, (bytes, bytearray)):
                raise ValueError("bytes expects a byte slice")
            serializer.serialize_bytes(bytes(value))
        elif type_name == "B96":
            serialize_fixed_bytes_value(serializer, value, 12, "B96")
        elif type_name == "B144":
            serialize_fixed_bytes_value(serializer, value, 18, "B144")
        elif type_name == "B160":
            serialize_fixed_bytes_value(serializer, value, 20, "B160")
        elif type_name == "B256":
            serialize_fixed_bytes_value(serializer, value, 32, "B256")
        else:
            raise ValueError(f"unsupported IDL type: {type_name}")

    def _serialize_enum(self, serializer: Serializer, idl_type: IDLType, value: Any) -> None:
        variant_name, variant_value = enum_variant_input(value)
        variant_index = -1
        variant = None
        for i, candidate in enumerate(idl_type.variants):
            if candidate.name.lower() == variant_name.lower():
                variant_index = i
                variant = candidate
                break
        if variant_index < 0:
            raise ValueError(f"unknown enum variant {idl_type.name}.{variant_name}")

        serializer.serialize_enum_variant(variant_index)
        if variant.kind == "unit":
            return
        if variant.kind == "tuple":
            tup = tuple_values(variant_value, len(variant.fields))
            for i, field_ in enumerate(variant.fields):
                self.serialize_value(serializer, field_.type, tup[i])
            return
        record = variant_value
        if not isinstance(record, dict):
            raise ValueError(f"{idl_type.name}.{variant.name} expects an object")
        for field_ in variant.fields:
            if field_.name not in record:
                raise ValueError(f"missing enum field: {field_.name}")
            self.serialize_value(serializer, field_.type, record[field_.name])

    # ================================================================
    # 解码（偏移式，镜像 Go deserializeValue）
    # ================================================================

    def decode(self, instruction_name: str, body: bytes) -> dict:
        instruction = self.get_instruction_by_name(instruction_name)
        if len(body) < 3:
            raise ValueError("empty body: need at least 3 bytes")

        off = _Offset(0)
        app_id = body[off.pos]
        off.pos += 1
        if app_id != self.app_id():
            raise ValueError(f"app_id mismatch: expected {self.app_id()}, got {app_id}")

        discriminator = body[off.pos] | (body[off.pos + 1] << 8)
        off.pos += 2
        if discriminator != instruction.discriminator:
            raise ValueError(f"discriminator mismatch: expected {instruction.discriminator}, got {discriminator}")

        args: dict = {}
        for arg in instruction.args:
            off = self.deserialize_value(arg.type, body, off)
            args[arg.name] = off.value
        if off.pos != len(body):
            raise ValueError(f"{len(body) - off.pos} trailing bytes after decoding")
        return args

    def deserialize_value(self, type_name: str, body: bytes, off: "_Offset") -> _Offset:
        """偏移式解码一个值。返回带 .value 的新 _Offset。"""
        # vec<T>
        inner = parse_wrapped_type(type_name, "vec")
        if inner is not None:
            length = decode_view_varuint(body, off)
            if length > len(body):
                raise ValueError(f"vec length {length} exceeds input size {len(body)}")
            items = []
            for _ in range(length):
                off = self.deserialize_value(inner, body, off)
                items.append(off.value)
            return _Offset(off.pos, items)

        # option<T>
        inner = parse_wrapped_type(type_name, "option")
        if inner is not None:
            has_value = decode_view_varuint(body, off)
            if has_value == 0:
                return _Offset(off.pos, None)
            return self.deserialize_value(inner, body, off)

        # map<K,V>
        parsed_map = parse_map_type(type_name)
        if parsed_map is not None:
            key_type, value_type = parsed_map
            length = decode_view_varuint(body, off)
            result: dict = {}
            for _ in range(length):
                off = self.deserialize_value(key_type, body, off)
                key = off.value
                off = self.deserialize_value(value_type, body, off)
                result[key] = off.value
            return _Offset(off.pos, result)

        # tuple<T1,T2,...>
        tuple_types = parse_tuple_type(type_name)
        if tuple_types is not None:
            items = []
            for item_type in tuple_types:
                off = self.deserialize_value(item_type, body, off)
                items.append(off.value)
            return _Offset(off.pos, items)

        # 自定义 IDL 类型
        idl_type = self.idl_type_by_name.get(type_name)
        if idl_type is not None:
            if idl_type.kind == "struct":
                record: dict = {}
                for field_ in idl_type.fields:
                    off = self.deserialize_value(field_.type, body, off)
                    record[field_.name] = off.value
                return _Offset(off.pos, record)
            if idl_type.kind == "enum":
                return self._deserialize_enum(idl_type, body, off)
            if idl_type.kind != "builtin":
                raise ValueError(f"unsupported type kind: {idl_type.kind} for type {type_name}")

        # 基础类型
        if type_name in ("Address", "Signer", "AnySigner"):
            if off.pos + 20 > len(body):
                raise ValueError("insufficient data for Address")
            addr = Address.from_bytes(body[off.pos:off.pos + 20])
            return _Offset(off.pos + 20, addr)
        if type_name == "PublicKey":
            variant_raw = decode_view_varuint(body, off)
            expected_len = {
                PublicKeyType.SECP256K1: 33,
                PublicKeyType.ED25519: 32,
                PublicKeyType.BLS12381: 48,
                PublicKeyType.FNDSA512: 897,
            }.get(int(variant_raw))
            if expected_len is None:
                raise ValueError(f"unknown public key variant: {variant_raw}")
            if off.pos + expected_len > len(body):
                raise ValueError(f"insufficient data for PublicKey bytes: expected {expected_len}")
            pk = PublicKey(PublicKeyType(int(variant_raw)), body[off.pos:off.pos + expected_len])
            return _Offset(off.pos + expected_len, pk)
        if type_name in ("String", "string"):
            length = decode_view_varuint(body, off)
            if off.pos + length > len(body):
                raise ValueError("insufficient data for String")
            s = body[off.pos:off.pos + length].decode("utf-8")
            return _Offset(off.pos + length, s)
        if type_name in ("bool", "boolean"):
            if off.pos >= len(body):
                raise ValueError("insufficient data for bool")
            return _Offset(off.pos + 1, body[off.pos] != 0)
        # —— 数值类型统一走 varint（镜像 Go decodeViewVarUint，含 u8）——
        # 注意：必须先取出值（decode_view_varuint 会就地推进 off.pos），
        # 再构造 _Offset(off.pos, val)，否则 off.pos 会在取值前被求值（仍为旧值）。
        if type_name == "u8":
            val = decode_view_varuint(body, off)
            return _Offset(off.pos, val)
        if type_name == "u16":
            val = decode_view_varuint(body, off)
            return _Offset(off.pos, val)
        if type_name == "u32":
            val = decode_view_varuint(body, off)
            return _Offset(off.pos, val)
        if type_name in ("u64", "Bitmap64", "Amount", "Epoch"):
            val = decode_view_varuint(body, off)
            return _Offset(off.pos, val)
        if type_name == "u128":
            val = decode_view_varuint128(body, off)
            return _Offset(off.pos, val)
        if type_name == "i8":
            if off.pos >= len(body):
                raise ValueError("insufficient data for i8")
            val = body[off.pos]
            signed = val - 256 if val >= 128 else val
            return _Offset(off.pos + 1, signed)
        if type_name == "i16":
            val = decode_view_varuint(body, off)
            return _Offset(off.pos, val - 0x10000 if val >= 0x8000 else val)
        if type_name == "i32":
            val = decode_view_varuint(body, off)
            return _Offset(off.pos, val - 0x100000000 if val >= 0x80000000 else val)
        if type_name == "i64":
            val = decode_view_varuint(body, off)
            return _Offset(off.pos, val - 0x10000000000000000 if val >= 0x8000000000000000 else val)
        if type_name == "bytes":
            length = decode_view_varuint(body, off)
            if off.pos + length > len(body):
                raise ValueError("insufficient data for bytes")
            b = body[off.pos:off.pos + length]
            return _Offset(off.pos + length, b)
        if type_name in ("B96", "B144", "B160", "B256"):
            size = {"B96": 12, "B144": 18, "B160": 20, "B256": 32}[type_name]
            return deserialize_fixed_bytes_value(body, off, size, type_name)
        raise ValueError(f"unsupported IDL type: {type_name}")

    # 数值类型统一走 varint（Go decodeViewVarUint 行为，含 u8）
    def _deserialize_enum(self, idl_type: IDLType, body: bytes, off: "_Offset") -> _Offset:
        variant_index = decode_view_varuint(body, off)
        if variant_index >= len(idl_type.variants):
            raise ValueError(
                f"invalid variant index {variant_index} for enum {idl_type.name} "
                f"(has {len(idl_type.variants)} variants)"
            )
        variant = idl_type.variants[variant_index]
        if variant.kind == "unit":
            return _Offset(off.pos, {"variant": variant.name, "index": variant_index})
        if variant.kind == "struct":
            record: dict = {"variant": variant.name, "index": variant_index}
            for field_ in variant.fields:
                off = self.deserialize_value(field_.type, body, off)
                record[field_.name] = off.value
            return _Offset(off.pos, record)
        if variant.kind == "tuple":
            fields = []
            for field_ in variant.fields:
                off = self.deserialize_value(field_.type, body, off)
                fields.append(off.value)
            return _Offset(off.pos, {"variant": variant.name, "index": variant_index, "fields": fields})
        raise ValueError(f"unsupported variant kind {variant.kind} for {idl_type.name}::{variant.name}")

    # ================================================================
    # 视图解码（Vec<Result<T>>）
    # ================================================================

    def decode_view_datas(self, instruction_name: str, body: bytes) -> list[DecodedTaggedValue]:
        instruction = self.get_instruction_by_name(instruction_name)
        if instruction.kind != "view":
            raise ValueError(f"{instruction_name} is not a view instruction (kind={instruction.kind})")
        return_type = instruction.returns.type.strip()
        if not return_type:
            raise ValueError(f"IDL view method {instruction_name} is missing returns.type")

        off = _Offset(0)
        result_count = decode_view_varuint(body, off)
        if result_count > len(body):
            raise ValueError(f"result count {result_count} exceeds input size {len(body)}")

        results = []
        for _ in range(result_count):
            off = self._decode_view_result_item(return_type, body, off)
            results.append(DecodedTaggedValue(off.value))
        if off.pos != len(body):
            raise ValueError(f"{len(body) - off.pos} trailing bytes after decoding")
        return results

    def _decode_view_result_item(self, return_type: str, body: bytes, off: "_Offset") -> _Offset:
        """Result<T> 条目：变体 0=Ok(长度前缀+值)，1=Err(TxFailurePayload)。"""
        variant_index = decode_view_varuint(body, off)
        if variant_index == 0:
            ok_data_len = decode_view_varuint(body, off)
            if off.pos + ok_data_len > len(body):
                raise ValueError("insufficient data for Ok payload")
            ok_data = body[off.pos:off.pos + ok_data_len]
            inner = _Offset(0)
            inner = self.deserialize_value(return_type, ok_data, inner)
            if inner.pos != len(ok_data):
                raise ValueError(f"{len(ok_data) - inner.pos} trailing bytes after decoding Ok value")
            return _Offset(off.pos + ok_data_len, inner.value)
        if variant_index == 1:
            failure = self._decode_tx_failure_payload(body, off)
            return _Offset(off.pos, failure)
        raise ValueError(f"invalid result variant index: {variant_index}")

    def _decode_tx_failure_payload(self, body: bytes, off: "_Offset"):
        code = decode_view_varuint(body, off)
        message_len = decode_view_varuint(body, off)
        if off.pos + message_len > len(body):
            raise ValueError("insufficient data for failure message")
        message = body[off.pos:off.pos + message_len].decode("utf-8")
        off.pos += message_len
        data_len = decode_view_varuint(body, off)
        if off.pos + data_len > len(body):
            raise ValueError("insufficient data for failure data")
        data = body[off.pos:off.pos + data_len]
        off.pos += data_len
        return {"code": code, "message": message, "data": data}

    def decode_view_data(self, instruction_name: str, body: bytes) -> Any:
        values = self.decode_view_datas(instruction_name, body)
        if not values:
            raise ValueError(f"view {instruction_name} returned no values")
        return values[0].value

    def decode_data_by_idl_type_name(self, idl_type_name: str, data: bytes) -> Any:
        if not data:
            raise ValueError("empty resource data")
        off = _Offset(0)
        off = self.deserialize_value(idl_type_name, data, off)
        if off.pos != len(data):
            raise ValueError(f"{len(data) - off.pos} trailing bytes after decoding {idl_type_name}")
        return off.value


class _Offset:
    """Go *int offset 的 Python 等价物：携带位置与最近一次解码值。"""

    __slots__ = ("pos", "value")

    def __init__(self, pos: int, value: Any = None):
        self.pos = pos
        self.value = value


# ================================================================
# 变长整数解码（镜像 Go decodeViewVarUint / decodeViewVarUint128）
# ================================================================


def decode_view_varuint(input_: bytes, offset: _Offset) -> int:
    value = 0
    shift = 0
    for _ in range(10):
        if offset.pos >= len(input_):
            raise ValueError("unexpected end of input")
        b = input_[offset.pos]
        offset.pos += 1
        value |= (b & 0x7F) << shift
        if b & 0x80 == 0:
            return value
        shift += 7
    raise ValueError("varint is too long")


def decode_view_varuint128(input_: bytes, offset: _Offset) -> int:
    value = 0
    n = 0
    while n < 19:
        if offset.pos >= len(input_):
            raise ValueError("unexpected end of input")
        b = input_[offset.pos]
        offset.pos += 1
        value |= (b & 0x7F) << (n * 7)
        if b & 0x80 == 0:
            return value
        n += 1
    raise ValueError("varint is too long")


# ================================================================
# 类型解析辅助（镜像 Go provider.go 各 parse 函数）
# ================================================================


def parse_wrapped_type(type_name: str, wrapper: str) -> Optional[str]:
    prefix = wrapper + "<"
    if len(type_name) < len(prefix) + 1 or not type_name.endswith(">"):
        return None
    if type_name[: len(prefix)].lower() != prefix.lower():
        return None
    return type_name[len(prefix):-1].strip()


def parse_map_type(type_name: str):
    inner = parse_wrapped_type(type_name, "map")
    if inner is None:
        return None
    parts = split_top_level(inner, ",")
    if len(parts) != 2:
        raise ValueError(f"invalid map type: {type_name}")
    return parts[0].strip(), parts[1].strip()


def parse_tuple_type(type_name: str):
    inner = parse_wrapped_type(type_name, "tuple")
    if inner is None:
        return None
    return [p.strip() for p in split_top_level(inner, ",")]


def split_top_level(value: str, separator: str) -> list[str]:
    parts: list[str] = []
    depth = 0
    start = 0
    for i, ch in enumerate(value):
        if ch == "<":
            depth += 1
        elif ch == ">":
            depth -= 1
        elif ch == separator and depth == 0:
            parts.append(value[start:i])
            start = i + 1
    parts.append(value[start:])
    return parts


# ================================================================
# 值规范化辅助（镜像 Go provider.go helpers）
# ================================================================


def slice_values(value: Any) -> list:
    if isinstance(value, (list, tuple)):
        return list(value)
    raise ValueError("expects an array")


def map_entries(value: Any) -> list:
    if isinstance(value, dict):
        return [[k, v] for k, v in value.items()]
    if isinstance(value, (list, tuple)):
        out = []
        for item in value:
            if isinstance(item, (list, tuple)) and len(item) == 2:
                out.append([item[0], item[1]])
            else:
                raise ValueError("invalid map entry")
        return out
    raise ValueError("map expects a map or entry array")


def tuple_values(value: Any, length: int) -> list:
    if isinstance(value, (list, tuple)):
        if len(value) != length:
            raise ValueError(f"tuple expects {length} values")
        return list(value)
    if isinstance(value, dict):
        out = []
        for i in range(length):
            if str(i) not in value:
                raise ValueError(f"missing tuple field: {i}")
            out.append(value[str(i)])
        return out
    raise ValueError("tuple expects an array or object")


def enum_variant_input(value: Any):
    if isinstance(value, str):
        return value, None
    if isinstance(value, dict):
        if "variant" in value:
            variant = value["variant"]
            if "value" in value:
                return variant, value["value"]
            if "fields" in value:
                return variant, value["fields"]
            return variant, {}
        if len(value) == 1:
            for k, v in value.items():
                return k, v
        raise ValueError("enum object must contain exactly one variant")
    raise ValueError("enum expects a variant string or object")


def serialize_address(serializer: Serializer, value: Any) -> None:
    if isinstance(value, Address):
        buf = value.as_bytes()
    elif isinstance(value, bytes):
        if len(value) != 20:
            raise ValueError("address must be 20 bytes")
        buf = value
    elif isinstance(value, str):
        try:
            buf = decode_hex(value)
            if len(buf) != 20:
                raise ValueError("address must be 20 bytes")
        except ValueError:
            from ..crypto.address import Address as A

            addr = A.from_base58(value)
            buf = addr.as_bytes()
    else:
        raise ValueError(f"invalid type for Address: {type(value).__name__}")
    serializer.write(buf)


def serialize_public_key(serializer: Serializer, value: Any) -> None:
    if isinstance(value, PublicKey):
        pk = value
    elif isinstance(value, str):
        pk = PublicKey.from_string_relaxed(value)
    elif isinstance(value, bytes):
        pk = PublicKey.from_bytes(value)
    else:
        raise ValueError(f"invalid type for PublicKey: {type(value).__name__}")
    serializer.serialize_u32(int(pk.variant))
    serializer.write(pk.as_bytes())


def decode_hex(value: str) -> bytes:
    body = value[2:] if len(value) >= 2 and value[:2] in ("0x", "0X") else value
    try:
        return bytes.fromhex(body)
    except ValueError as exc:
        raise ValueError("invalid hex string") from exc


def is_nil(value: Any) -> bool:
    if value is None:
        return True
    return False


def as_uint64(value: Any, max_value: int, name: str) -> int:
    if isinstance(value, bool):
        raise ValueError(f"{name} out of range")
    if isinstance(value, int):
        if value < 0 or value > max_value:
            raise ValueError(f"{name} out of range")
        return value
    if isinstance(value, float):
        if value < 0 or value != math.trunc(value) or value > float(max_value):
            raise ValueError(f"{name} out of range")
        return int(value)
    if isinstance(value, str):
        try:
            n = int(value, 10)
        except ValueError:
            raise ValueError(f"{name} out of range")
        if n < 0 or n > max_value:
            raise ValueError(f"{name} out of range")
        return n
    raise ValueError(f"{name} out of range")


def as_big_int(value: Any, signed: bool = False) -> int:
    if isinstance(value, bool):
        raise ValueError("u128 out of range")
    if isinstance(value, int):
        if not signed and value < 0:
            raise ValueError("u128 out of range")
        return value
    if isinstance(value, float):
        if value != math.trunc(value) or (not signed and value < 0):
            raise ValueError("u128 out of range")
        return int(value)
    if isinstance(value, str):
        try:
            n = int(value, 10)
        except ValueError:
            raise ValueError("u128 out of range")
        if not signed and n < 0:
            raise ValueError("u128 out of range")
        return n
    raise ValueError("u128 out of range")


def as_int64(value: Any, min_value: int, max_value: int, name: str) -> int:
    if isinstance(value, bool):
        raise ValueError(f"{name} out of range")
    if isinstance(value, int):
        if value < min_value or value > max_value:
            raise ValueError(f"{name} out of range")
        return value
    if isinstance(value, float):
        if value != math.trunc(value) or value < min_value or value > max_value:
            raise ValueError(f"{name} out of range")
        return int(value)
    if isinstance(value, str):
        try:
            n = int(value, 10)
        except ValueError:
            raise ValueError(f"{name} out of range")
        if n < min_value or n > max_value:
            raise ValueError(f"{name} out of range")
        return n
    raise ValueError(f"{name} out of range")


def serialize_fixed_bytes_value(serializer: Serializer, value: Any, size: int, type_name: str) -> None:
    if isinstance(value, bytes) and len(value) == size:
        serializer.write(value)
        return
    raise ValueError(f"{type_name} expects exactly {size} bytes")


def deserialize_fixed_bytes_value(body: bytes, off: _Offset, size: int, type_name: str) -> _Offset:
    if off.pos + size > len(body):
        raise ValueError(f"insufficient data for {type_name}")
    b = body[off.pos:off.pos + size]
    return _Offset(off.pos + size, b)
