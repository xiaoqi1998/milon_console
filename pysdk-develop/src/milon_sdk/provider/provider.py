"""Provider: IDL 驱动的编解码引擎. Faithful port of gosdk-develop/provider/provider.go."""

from __future__ import annotations

import binascii
import json
import math
from typing import Any, Dict, List, Optional, Tuple

import base58

from ..crypto import (
    Address,
    PublicKey,
    PublicKeyType,
    Signature,
    SignatureType,
    new_address_from_bytes,
    new_public_key_from_bytes,
    new_public_key_from_string_relaxed,
    new_signature_from_bytes,
    new_signature_from_string_relaxed,
)
from ..postcard import Serializer
from .types import (
    IDL,
    Arg,
    Args,
    Constant,
    DecodedTaggedValue,
    EnumVariant,
    ErrorDef,
    Event,
    EventField,
    IDLType,
    Instruction,
    LookupPath,
    Metadata,
    Resource,
    ReturnValue,
    SignerLookup,
    SignerLookups,
    StructField,
)


def idl_from_json_dict(data: dict) -> IDL:
    """Go: json.Unmarshal(data, &IDL)（按 JSON tag 映射）。"""
    def _metadata(d: dict) -> Metadata:
        return Metadata(
            AppID=d.get("app_id", 0),
            Name=d.get("name", ""),
            Description=d.get("description", ""),
        )

    def _instruction(d: dict) -> Instruction:
        returns = d.get("returns")
        signer_lookups = d.get("signer_lookups")
        return Instruction(
            Args=[Arg(Name=a.get("name", ""), Role=a.get("role", ""), Type=a.get("type", "")) for a in (d.get("args") or [])],
            Discriminator=d.get("discriminator", 0),
            Handler=d.get("handler", ""),
            Kind=d.get("kind", ""),
            Name=d.get("name", ""),
            Returns=ReturnValue(Type=returns.get("type", "")) if returns else None,
            SignerLookups=(
                {
                    k: SignerLookup(
                        Path=LookupPath(Arg=v.get("path", {}).get("arg", ""), Type=v.get("path", {}).get("type", "")),
                        Res=v.get("res", 0),
                    )
                    for k, v in signer_lookups.items()
                }
                if signer_lookups
                else None
            ),
            Sponsor=bool(d.get("sponsor", False)),
        )

    def _idl_type(d: dict) -> IDLType:
        return IDLType(
            Fields=[StructField(Name=f.get("name", ""), Type=f.get("type", "")) for f in (d.get("fields") or [])],
            Variants=[
                EnumVariant(
                    Name=v.get("name", ""),
                    Kind=v.get("kind", ""),
                    Fields=[StructField(Name=f.get("name", ""), Type=f.get("type", "")) for f in (v.get("fields") or [])],
                )
                for v in (d.get("variants") or [])
            ],
            Kind=d.get("kind", ""),
            Name=d.get("name", ""),
            TypeTag=d.get("typeTag", 0),
        )

    def _resource(d: dict) -> Resource:
        return Resource(Name=d.get("name", ""), Type=d.get("type", ""), TypeTag=d.get("typeTag", 0))

    def _event(d: dict) -> Event:
        return Event(
            Name=d.get("name", ""),
            Fields=[EventField(Name=f.get("name", ""), Type=f.get("type", ""), Indexed=bool(f.get("indexed", False))) for f in (d.get("fields") or [])],
            TypeTag=d.get("typeTag", 0),
        )

    def _error(d: dict) -> ErrorDef:
        return ErrorDef(Code=d.get("code", 0), Message=d.get("message", ""), Name=d.get("name", ""))

    def _constant(d: dict) -> Constant:
        return Constant(Name=d.get("name", ""), Type=d.get("type", ""), Value=d.get("value"))

    return IDL(
        Metadata=_metadata(data.get("metadata") or {}),
        Instructions=[_instruction(i) for i in (data.get("instructions") or [])],
        Types=[_idl_type(t) for t in (data.get("types") or [])],
        Resources=[_resource(r) for r in (data.get("resources") or [])],
        Events=[_event(e) for e in (data.get("events") or [])],
        Errors=[_error(e) for e in (data.get("errors") or [])],
        Constants=[_constant(c) for c in (data.get("constants") or [])],
    )


class Provider:
    def __init__(self, idl: IDL) -> None:
        self.IDL = idl
        self.InstructionByName: Dict[str, Instruction] = {}
        self.InstructionByDiscriminator: Dict[int, Instruction] = {}
        self.IDLTypeByName: Dict[str, IDLType] = {}
        self.IDLTypeByTypeTag: Dict[int, IDLType] = {}
        self.EventByTypeTag: Dict[int, Event] = {}
        self.ResourceByName: Dict[str, Resource] = {}
        self.ResourceByTypeTag: Dict[int, Resource] = {}

        for instruction in idl.Instructions:
            self.InstructionByName[instruction.Name] = instruction
            self.InstructionByDiscriminator[instruction.Discriminator] = instruction

        for value in idl.Types:
            self.IDLTypeByName[value.Name] = value
            self.IDLTypeByTypeTag[value.TypeTag] = value

        for event in idl.Events:
            # Convert the event into an IDLType so it can be handled uniformly
            idl_type = IDLType(
                Name=event.Name,
                TypeTag=event.TypeTag,
                Kind="struct",  # Event is essentially a struct
                Fields=[StructField(Name=f.Name, Type=f.Type) for f in event.Fields],
            )

            # Register as IDLType only when it does not collide
            if idl_type.Name not in self.IDLTypeByName:
                self.IDLTypeByName[idl_type.Name] = idl_type
            if idl_type.TypeTag not in self.IDLTypeByTypeTag:
                self.IDLTypeByTypeTag[idl_type.TypeTag] = idl_type

            self.EventByTypeTag[event.TypeTag] = event

        for resource in idl.Resources:
            self.ResourceByName[resource.Name] = resource
            # Multiple resources may share one typeTag; first declaration wins
            if resource.TypeTag not in self.ResourceByTypeTag:
                self.ResourceByTypeTag[resource.TypeTag] = resource

    def app_id(self) -> int:
        return self.IDL.Metadata.AppID

    def get_instruction_by_name(self, name: str) -> Instruction:
        instruction = self.InstructionByName.get(name)
        if instruction is None:
            raise ValueError(f"IDL method not found: {name}")
        return instruction

    def get_idl_type_by_type_tag(self, type_tag: int) -> Optional[IDLType]:
        return self.IDLTypeByTypeTag.get(type_tag)

    def get_event_by_type_tag(self, type_tag: int) -> Optional[Event]:
        return self.EventByTypeTag.get(type_tag)

    def get_resource_by_name(self, name: str) -> Optional[Resource]:
        return self.ResourceByName.get(name)

    def get_resource_by_type_tag(self, type_tag: int) -> Optional[Resource]:
        return self.ResourceByTypeTag.get(type_tag)

    # ---- Encode ----

    def encode(self, instruction_name: str, args: Args) -> bytes:
        """Encodes instruction args into wire bytes for on-chain submission."""
        instruction = self.get_instruction_by_name(instruction_name)

        if instruction.Kind not in ("entry", "view"):
            raise ValueError(
                f"unsupported instruction kind: {instruction.Kind} "
                "(expected 'entry' or 'view')"
            )

        return self._encode_instruction(instruction, args)

    def _encode_instruction(self, instruction: Instruction, args: Args) -> bytes:
        serializer = Serializer()
        # 1. app_id (1 byte)
        serializer.serialize_u8(self.app_id())

        # 2. discriminator (u16 LE, 2 bytes)
        serializer.serialize_fixed_bytes(
            bytes(
                [
                    instruction.Discriminator & 0xFF,
                    (instruction.Discriminator >> 8) & 0xFF,
                ]
            )
        )

        # 3. args in IDL order
        for arg in instruction.Args:
            if arg.Name not in args:
                raise ValueError(f"missing IDL argument: {arg.Name}")
            self._serialize_value(serializer, arg.Type.strip(), args[arg.Name])

        return serializer.bytes()

    def _serialize_value(self, serializer: Serializer, arg_name: str, value: Any) -> None:
        # vec<T>: [len(varint)] + items...
        inner = parse_wrapped_type(arg_name, "vec")
        if inner is not None:
            try:
                items = _slice_values(value)
            except ValueError:
                raise ValueError(f"{arg_name} expects an array") from None

            serializer.serialize_u32(len(items))
            for item in items:
                self._serialize_value(serializer, inner, item)
            return

        # option<T>: [has_value(u8)] + [value if present]
        inner = parse_wrapped_type(arg_name, "option")
        if inner is not None:
            if value is None:
                serializer.serialize_bool(False)
                return
            serializer.serialize_bool(True)
            self._serialize_value(serializer, inner, value)
            return

        # map<K,V>: [len(varint)] + key/value pairs...
        key_type, value_type = parse_map_type(arg_name)
        if key_type is not None:
            try:
                entries = _map_entries(value)
            except ValueError:
                raise ValueError("map expects a map or entry array") from None

            serializer.serialize_u32(len(entries))
            for k, v in entries:
                self._serialize_value(serializer, key_type, k)
                self._serialize_value(serializer, value_type, v)
            return

        # tuple<T1,T2,...>: elements in order
        tuple_types = parse_tuple_type(arg_name)
        if tuple_types is not None:
            tuple_vals = _tuple_values(value, len(tuple_types))
            for item_type, item in zip(tuple_types, tuple_vals):
                self._serialize_value(serializer, item_type, item)
            return

        # custom IDL type (struct/enum/builtin)
        idl_type = self.IDLTypeByName.get(arg_name)
        if idl_type is not None:
            if idl_type.Kind == "struct":
                if not isinstance(value, dict):
                    raise ValueError(f"{arg_name} expects an object")
                for f in idl_type.Fields:
                    if f.Name not in value:
                        raise ValueError(f"missing struct field: {f.Name}")
                    self._serialize_value(serializer, f.Type, value[f.Name])
                return
            if idl_type.Kind == "enum":
                self._serialize_enum(serializer, idl_type, value)
                return
            if idl_type.Kind != "builtin":
                raise ValueError(
                    f"unsupported type kind: {idl_type.Kind} for type {arg_name}"
                )

        if arg_name in ("Address", "Signer", "AnySigner"):
            _serialize_address(serializer, value)
            return
        if arg_name == "PublicKey":
            _serialize_public_key(serializer, value)
            return
        if arg_name == "Signature":
            _serialize_signature(serializer, value)
            return
        if arg_name in ("String", "string"):
            serializer.serialize_str(_fmt_sprint(value))
            return
        if arg_name in ("bool", "boolean"):
            if not isinstance(value, bool):
                raise ValueError(f"{arg_name} expects a boolean")
            serializer.serialize_bool(value)
            return
        if arg_name == "u8":
            serializer.serialize_u8(_as_uint64(value, 0xFF, "u8"))
            return
        if arg_name == "u16":
            serializer.serialize_u16(_as_uint64(value, 0xFFFF, "u16"))
            return
        if arg_name == "u32":
            serializer.serialize_u32(_as_uint64(value, 0xFFFFFFFF, "u32"))
            return
        if arg_name in ("u64", "Bitmap64", "Amount", "Epoch"):
            serializer.serialize_u64(_as_uint64(value, 0xFFFFFFFFFFFFFFFF, "u64"))
            return
        if arg_name == "u128":
            serializer.serialize_u128(_as_big_int(value, False))
            return
        if arg_name == "i8":
            serializer.serialize_i8(_as_int64(value, -0x80, 0x7F, "i8"))
            return
        if arg_name == "i16":
            serializer.serialize_i16(_as_int64(value, -0x8000, 0x7FFF, "i16"))
            return
        if arg_name == "i32":
            serializer.serialize_i32(_as_int64(value, -0x80000000, 0x7FFFFFFF, "i32"))
            return
        if arg_name == "i64":
            serializer.serialize_i64(_as_int64(value, -0x8000000000000000, 0x7FFFFFFFFFFFFFFF, "i64"))
            return
        if arg_name == "bytes":
            if not isinstance(value, (bytes, bytearray)):
                raise ValueError("bytes expects a []byte slice")
            serializer.serialize_bytes(bytes(value))
            return
        if arg_name == "B96":
            _serialize_fixed_bytes_value(serializer, value, 12, "B96")
            return
        if arg_name == "B144":
            _serialize_fixed_bytes_value(serializer, value, 18, "B144")
            return
        if arg_name == "B160":
            _serialize_fixed_bytes_value(serializer, value, 20, "B160")
            return
        if arg_name == "B256":
            _serialize_fixed_bytes_value(serializer, value, 32, "B256")
            return
        raise ValueError(f"unsupported IDL type: {arg_name}")

    def _serialize_enum(self, serializer: Serializer, idl_type: IDLType, value: Any) -> None:
        variant_name, variant_value = _enum_variant_input(value)

        variant_index = -1
        variant = None
        for i, candidate in enumerate(idl_type.Variants):
            if candidate.Name.lower() == variant_name.lower():
                variant_index = i
                variant = candidate
                break
        if variant_index < 0:
            raise ValueError(f"unknown enum variant {idl_type.Name}.{variant_name}")

        serializer.serialize_enum_variant(variant_index)

        # unit: no associated data
        if variant.Kind == "unit":
            return

        if variant.Kind == "tuple":
            tuple_vals = _tuple_values(variant_value, len(variant.Fields))
            for f, item in zip(variant.Fields, tuple_vals):
                self._serialize_value(serializer, f.Type, item)
            return

        if not isinstance(variant_value, dict):
            raise ValueError(f"{idl_type.Name}.{variant.Name} expects an object")

        for f in variant.Fields:
            if f.Name not in variant_value:
                raise ValueError(f"missing enum field: {f.Name}")
            self._serialize_value(serializer, f.Type, variant_value[f.Name])

    # ---- Decode ----

    def decode(self, instruction_name: str, body: bytes) -> Args:
        """Decodes an encoded instruction body into its arguments."""
        instruction = self.get_instruction_by_name(instruction_name)

        offset = 0
        # at least 3 bytes: app_id + u16 discriminator
        if len(body) < 3:
            raise ValueError("empty body: need at least 3 bytes")

        app_id = body[offset]
        offset += 1
        if app_id != self.app_id():
            raise ValueError(
                f"app_id mismatch: expected {self.app_id()}, got {app_id}"
            )

        # discriminator (u16 LE)
        discriminator = body[offset] | (body[offset + 1] << 8)
        offset += 2

        if discriminator != instruction.Discriminator:
            raise ValueError(
                f"discriminator mismatch: expected {instruction.Discriminator}, "
                f"got {discriminator}"
            )

        args: Args = {}
        for arg in instruction.Args:
            try:
                value = self.deserialize_value(arg.Type, body, offset)
            except ValueError as exc:
                raise ValueError(
                    f"failed to decode argument {arg.Name}: {exc}"
                ) from exc
            offset = value[1]
            args[arg.Name] = value[0]

        if offset != len(body):
            raise ValueError(
                f"{len(body) - offset} trailing bytes after decoding"
            )

        return args

    def deserialize_value(self, idl_type_name: str, body: bytes, offset: int) -> Any:
        """Decodes one value by its IDL type name. Returns (value, new_offset)."""
        inner = parse_wrapped_type(idl_type_name, "vec")
        if inner is not None:
            length, offset = _decode_view_var_uint(body, offset)
            # Each element consumes at least 1 byte
            if length > len(body):
                raise ValueError(
                    f"vec length {length} exceeds input size {len(body)}"
                )
            items = []
            for _ in range(length):
                item, offset = self.deserialize_value(inner, body, offset)
                items.append(item)
            return items, offset

        inner = parse_wrapped_type(idl_type_name, "option")
        if inner is not None:
            has_value, offset = _decode_view_var_uint(body, offset)
            if has_value == 0:
                return None, offset
            return self.deserialize_value(inner, body, offset)

        key_type, value_type = parse_map_type(idl_type_name)
        if key_type is not None:
            length, offset = _decode_view_var_uint(body, offset)
            result: Dict[Any, Any] = {}
            for _ in range(length):
                k, offset = self.deserialize_value(key_type, body, offset)
                v, offset = self.deserialize_value(value_type, body, offset)
                result[k] = v
            return result, offset

        tuple_types = parse_tuple_type(idl_type_name)
        if tuple_types is not None:
            items = []
            for item_type in tuple_types:
                item, offset = self.deserialize_value(item_type, body, offset)
                items.append(item)
            return items, offset

        idl_type = self.IDLTypeByName.get(idl_type_name)
        if idl_type is not None:
            if idl_type.Kind == "struct":
                record: Dict[str, Any] = {}
                for f in idl_type.Fields:
                    value, offset = self.deserialize_value(f.Type, body, offset)
                    record[f.Name] = value
                return record, offset
            if idl_type.Kind == "enum":
                variant_index, offset = _decode_view_var_uint(body, offset)
                if variant_index >= len(idl_type.Variants):
                    raise ValueError(
                        f"invalid variant index {variant_index} for enum "
                        f"{idl_type_name} (has {len(idl_type.Variants)} variants)"
                    )
                variant = idl_type.Variants[variant_index]
                if variant.Kind == "unit":
                    return {"variant": variant.Name, "index": variant_index}, offset
                if variant.Kind == "struct":
                    record = {"variant": variant.Name, "index": variant_index}
                    for f in variant.Fields:
                        try:
                            value, offset = self.deserialize_value(f.Type, body, offset)
                        except ValueError as exc:
                            raise ValueError(
                                f"failed to deserialize field {f.Name} of "
                                f"variant {variant.Name}: {exc}"
                            ) from exc
                        record[f.Name] = value
                    return record, offset
                if variant.Kind == "tuple":
                    fields = []
                    for i, f in enumerate(variant.Fields):
                        try:
                            value, offset = self.deserialize_value(f.Type, body, offset)
                        except ValueError as exc:
                            raise ValueError(
                                f"failed to deserialize field {i} of variant "
                                f"{variant.Name}: {exc}"
                            ) from exc
                        fields.append(value)
                    return {
                        "variant": variant.Name,
                        "index": variant_index,
                        "fields": fields,
                    }, offset
                raise ValueError(
                    f"unsupported variant kind {variant.Kind} for "
                    f"{idl_type_name}::{variant.Name}"
                )
            if idl_type.Kind != "builtin":
                raise ValueError(
                    f"unsupported type kind: {idl_type.Kind} for type "
                    f"{idl_type_name}"
                )

        # primitive types
        if idl_type_name in ("Address", "Signer", "AnySigner"):
            if offset + 20 > len(body):
                raise ValueError("insufficient data for Address")
            addr_bytes = body[offset : offset + 20]
            offset += 20
            try:
                addr = new_address_from_bytes(addr_bytes)
            except ValueError as exc:
                raise ValueError(
                    f"failed to create Address from bytes: {exc}"
                ) from exc
            return addr, offset

        if idl_type_name == "PublicKey":
            variant_raw, offset = _decode_view_var_uint(body, offset)
            try:
                expected_len = {
                    int(PublicKeyType.SECP256K1): 33,
                    int(PublicKeyType.ED25519): 32,
                    int(PublicKeyType.BLS12381): 48,
                    int(PublicKeyType.FN_DSA512): 897,
                }[variant_raw]
            except KeyError:
                raise ValueError(f"unknown public key variant: {variant_raw}") from None

            if offset + expected_len > len(body):
                raise ValueError(
                    "insufficient data for PublicKey bytes: expected "
                    f"{expected_len}, got {len(body) - offset}"
                )
            pk_bytes = body[offset : offset + expected_len]
            offset += expected_len
            try:
                pk = new_public_key_from_bytes(pk_bytes)
            except ValueError as exc:
                raise ValueError(
                    f"failed to create PublicKey from bytes: {exc}"
                ) from exc
            return pk, offset

        if idl_type_name == "Signature":
            variant_raw, offset = _decode_view_var_uint(body, offset)
            try:
                expected_len = {
                    int(SignatureType.SECP256K1): 65,
                    int(SignatureType.ED25519): 64,
                    int(SignatureType.BLS12381): 96,
                    int(SignatureType.FN_DSA512): 666,
                }[variant_raw]
            except KeyError:
                raise ValueError(f"unknown signature variant: {variant_raw}") from None

            if offset + expected_len > len(body):
                raise ValueError(
                    "insufficient data for Signature bytes: expected "
                    f"{expected_len}, got {len(body) - offset}"
                )
            sig_bytes = body[offset : offset + expected_len]
            offset += expected_len
            try:
                sig = new_signature_from_bytes(sig_bytes)
            except ValueError as exc:
                raise ValueError(
                    f"failed to create Signature from bytes: {exc}"
                ) from exc
            return sig, offset

        if idl_type_name in ("String", "string"):
            length, offset = _decode_view_var_uint(body, offset)
            if offset + length > len(body):
                raise ValueError("insufficient data for String")
            s = body[offset : offset + length].decode("utf-8", errors="replace")
            offset += length
            return s, offset

        if idl_type_name in ("bool", "boolean"):
            if offset >= len(body):
                raise ValueError("insufficient data for bool")
            val = body[offset] != 0
            offset += 1
            return val, offset

        if idl_type_name == "u8":
            if offset >= len(body):
                raise ValueError("insufficient data for u8")
            val = body[offset]
            offset += 1
            return val, offset

        if idl_type_name == "u16":
            val, offset = _decode_view_var_uint(body, offset)
            return val & 0xFFFF, offset

        if idl_type_name == "u32":
            val, offset = _decode_view_var_uint(body, offset)
            return val & 0xFFFFFFFF, offset

        if idl_type_name in ("u64", "Bitmap64", "Amount", "Epoch"):
            return _decode_view_var_uint(body, offset)

        if idl_type_name == "u128":
            return _decode_view_var_uint128(body, offset)

        if idl_type_name == "i8":
            # i8 is a single byte; varint decoding would misread negatives.
            if offset >= len(body):
                raise ValueError("insufficient data for i8")
            val = body[offset]
            offset += 1
            return val - 0x100 if val >= 0x80 else val, offset

        if idl_type_name == "i16":
            val, offset = _decode_view_var_uint(body, offset)
            val &= 0xFFFF
            return val - 0x10000 if val >= 0x8000 else val, offset

        if idl_type_name == "i32":
            val, offset = _decode_view_var_uint(body, offset)
            val &= 0xFFFFFFFF
            return val - 0x100000000 if val >= 0x80000000 else val, offset

        if idl_type_name == "i64":
            val, offset = _decode_view_var_uint(body, offset)
            val &= 0xFFFFFFFFFFFFFFFF
            return (
                val - 0x10000000000000000
                if val >= 0x8000000000000000
                else val,
            ), offset

        if idl_type_name == "bytes":
            length, offset = _decode_view_var_uint(body, offset)
            if offset + length > len(body):
                raise ValueError("insufficient data for bytes")
            byte_list = body[offset : offset + length]
            offset += length
            return byte_list, offset

        if idl_type_name == "B96":
            return _deserialize_fixed_bytes_value(body, offset, 12, "B96")
        if idl_type_name == "B144":
            return _deserialize_fixed_bytes_value(body, offset, 18, "B144")
        if idl_type_name == "B160":
            return _deserialize_fixed_bytes_value(body, offset, 20, "B160")
        if idl_type_name == "B256":
            return _deserialize_fixed_bytes_value(body, offset, 32, "B256")

        raise ValueError(f"unsupported IDL type: {idl_type_name}")

    # ---- View decoding ----

    def decode_view_datas(self, instruction_name: str, body: bytes) -> List[DecodedTaggedValue]:
        """Decodes a view response body of Vec<Result<T>>."""
        instruction = self.get_instruction_by_name(instruction_name)

        if instruction.Kind != "view":
            raise ValueError(
                f"{instruction_name} is not a view instruction "
                f"(kind={instruction.Kind})"
            )

        return_type = (instruction.Returns.Type if instruction.Returns else "").strip()
        if return_type == "":
            raise ValueError(
                f"IDL view method {instruction_name} is missing returns.type"
            )

        offset = 0
        # 1. vec length (result count)
        result_count, offset = _decode_view_var_uint(body, offset)
        if result_count > len(body):
            raise ValueError(
                f"result count {result_count} exceeds input size {len(body)}"
            )

        results = []
        # 2. each Result item
        for i in range(result_count):
            try:
                value, offset = _decode_view_result_item(self, return_type, body, offset)
            except ValueError as exc:
                raise ValueError(f"failed to decode result[{i}]: {exc}") from exc
            results.append(DecodedTaggedValue(Value=value))

        if offset != len(body):
            raise ValueError(
                f"{len(body) - offset} trailing bytes after decoding"
            )

        return results

    def decode_view_data(self, instruction_name: str, body: bytes) -> Any:
        values = self.decode_view_datas(instruction_name, body)
        if len(values) == 0:
            raise ValueError(f"view {instruction_name} returned no values")
        return values[0].Value

    def decode_data_by_idl_type_name(self, idl_type_name: str, data: bytes) -> Any:
        if len(data) == 0:
            raise ValueError("empty resource data")

        offset = 0
        try:
            value, offset = self.deserialize_value(idl_type_name, data, offset)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize {idl_type_name}: {exc}"
            ) from exc

        if offset != len(data):
            raise ValueError(
                f"{len(data) - offset} trailing bytes after decoding "
                f"{idl_type_name}"
            )

        return value


def _decode_view_result_item(
    pd: Provider, return_type: str, body: bytes, offset: int
) -> Tuple[Any, int]:
    variant_index, offset = _decode_view_var_uint(body, offset)

    if variant_index == 0:
        ok_data_len, offset = _decode_view_var_uint(body, offset)
        if offset + ok_data_len > len(body):
            raise ValueError("insufficient data for Ok payload")
        ok_data = body[offset : offset + ok_data_len]
        offset += ok_data_len

        value_offset = 0
        try:
            value, value_offset = pd.deserialize_value(return_type, ok_data, value_offset)
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Ok value: {exc}") from exc
        if value_offset != len(ok_data):
            raise ValueError(
                f"{len(ok_data) - value_offset} trailing bytes after decoding "
                f"Ok value"
            )
        return value, offset

    if variant_index == 1:
        try:
            failure, offset = _decode_tx_failure_payload(body, offset)
        except ValueError as exc:
            raise ValueError(f"failed to decode Err payload: {exc}") from exc
        return failure, offset

    raise ValueError(f"invalid result variant index: {variant_index}")


def _decode_tx_failure_payload(body: bytes, offset: int):
    from ..api.simulate_receipt import TxFailurePayload

    code_raw, offset = _decode_view_var_uint(body, offset)
    code = code_raw & 0xFFFF

    message_len, offset = _decode_view_var_uint(body, offset)
    if offset + message_len > len(body):
        raise ValueError("insufficient data for failure message")
    message = body[offset : offset + message_len].decode("utf-8", errors="replace")
    offset += message_len

    data_len, offset = _decode_view_var_uint(body, offset)
    if offset + data_len > len(body):
        raise ValueError("insufficient data for failure data")
    data = body[offset : offset + data_len]
    offset += data_len

    return (
        TxFailurePayload(Code=code, Message=message, Data=bytes(data)),
        offset,
    )


# ---- varint helpers ----


def _decode_view_var_uint(data: bytes, offset: int) -> Tuple[int, int]:
    """Decodes a varint (uint64, at most 10 bytes)."""
    value = 0
    shift = 0
    for _ in range(10):
        if offset >= len(data):
            raise ValueError("unexpected end of input")
        b = data[offset]
        offset += 1
        value |= (b & 0x7F) << shift
        if (b & 0x80) == 0:
            return value, offset
        shift += 7
    raise ValueError("varint is too long")


def _decode_view_var_uint128(data: bytes, offset: int) -> Tuple[int, int]:
    """Decodes a varint into an int (u128: at most 19 bytes)."""
    groups = []
    for n in range(19):
        if offset >= len(data):
            raise ValueError("unexpected end of input")
        b = data[offset]
        offset += 1
        groups.append(b & 0x7F)
        if (b & 0x80) == 0:
            break
    else:
        raise ValueError("varint is too long")

    value = 0
    for i, g in enumerate(groups):
        value |= g << (i * 7)
    return value, offset


# ---- type-name parsing ----


def parse_wrapped_type(arg_name: str, wrapper: str) -> Optional[str]:
    """Parses wrapped types: vec<u8> → "u8"; case-insensitive prefix."""
    prefix = wrapper + "<"
    if len(arg_name) < len(prefix) + 1 or not arg_name.endswith(">"):
        return None
    if arg_name[: len(prefix)].lower() != prefix.lower():
        return None
    return arg_name[len(prefix) : len(arg_name) - 1].strip()


def parse_map_type(arg_name: str) -> Tuple[Optional[str], Optional[str]]:
    inner = parse_wrapped_type(arg_name, "map")
    if inner is None:
        return None, None
    parts = _split_top_level(inner, ",")
    if len(parts) != 2:
        raise ValueError(f"invalid map type: {arg_name}")
    return parts[0].strip(), parts[1].strip()


def parse_tuple_type(arg_name: str) -> Optional[List[str]]:
    inner = parse_wrapped_type(arg_name, "tuple")
    if inner is None:
        return None
    return [p.strip() for p in _split_top_level(inner, ",")]


def _split_top_level(value: str, separator: str) -> List[str]:
    parts = []
    depth = 0
    start = 0
    for i, char in enumerate(value):
        if char == "<":
            depth += 1
        if char == ">":
            depth -= 1
        if char == separator and depth == 0:
            parts.append(value[start:i])
            start = i + 1
    parts.append(value[start:])
    return parts


# ---- value coercion helpers ----


def _slice_values(value: Any) -> List[Any]:
    if isinstance(value, (list, tuple)):
        return list(value)
    raise ValueError("invalid slice")


def _map_entries(value: Any) -> List[Tuple[Any, Any]]:
    if value is None:
        raise ValueError("invalid map")
    if isinstance(value, dict):
        return list(value.items())
    if isinstance(value, (list, tuple)):
        entries = []
        for item in value:
            if isinstance(item, (list, tuple)):
                if len(item) != 2:
                    raise ValueError("invalid map entry")
                entries.append((item[0], item[1]))
            else:
                raise ValueError("invalid map entry")
        return entries
    raise ValueError("invalid map")


def _tuple_values(value: Any, length: int) -> List[Any]:
    if isinstance(value, (list, tuple)):
        items = list(value)
        if len(items) != length:
            raise ValueError(f"tuple expects {length} values")
        return items
    if isinstance(value, dict):
        out = []
        for i in range(length):
            key = str(i)
            if key not in value:
                raise ValueError(f"missing tuple field: {key}")
            out.append(value[key])
        return out
    raise ValueError("tuple expects an array or object")


def _enum_variant_input(value: Any) -> Tuple[str, Any]:
    if isinstance(value, str):
        return value, None
    if not isinstance(value, dict):
        raise ValueError("enum expects a variant string or object")
    variant = value.get("variant")
    if isinstance(variant, str):
        if "value" in value:
            return variant, value["value"]
        if "fields" in value:
            return variant, value["fields"]
        return variant, {}
    if len(value) != 1:
        raise ValueError("enum object must contain exactly one variant")
    for key, inner in value.items():
        return key, inner
    raise ValueError("enum object must contain exactly one variant")


def _serialize_address(serializer: Serializer, value: Any) -> None:
    if isinstance(value, Address):
        buf = value.Bytes
    elif isinstance(value, (bytes, bytearray)):
        if len(value) != 20:
            raise ValueError("address must be 20 bytes")
        buf = bytes(value)
    elif isinstance(value, str):
        try:
            hex_buf = _decode_hex(value)
        except ValueError:
            hex_buf = None
        if hex_buf is not None and len(hex_buf) == 20:
            buf = hex_buf
        else:
            try:
                b58_buf = base58.b58decode(value)
            except ValueError:
                raise ValueError("address must be 20 bytes") from None
            if len(b58_buf) != 20:
                raise ValueError("address must be 20 bytes")
            buf = b58_buf
    else:
        raise ValueError(f"invalid type for Address: {type(value).__name__}")

    serializer.serialize_fixed_bytes(buf)


def _serialize_public_key(serializer: Serializer, value: Any) -> None:
    if isinstance(value, PublicKey):
        pk = value
    elif isinstance(value, str):
        try:
            pk = new_public_key_from_string_relaxed(value)
        except ValueError as exc:
            raise ValueError(
                f"failed to parse public key from string: {exc}"
            ) from exc
    elif isinstance(value, (bytes, bytearray)):
        try:
            pk = new_public_key_from_bytes(bytes(value))
        except ValueError as exc:
            raise ValueError(
                f"failed to create PublicKey from bytes: {exc}"
            ) from exc
    else:
        raise ValueError(f"invalid type for PublicKey: {type(value).__name__}")

    # First serialize Variant (4 bytes)
    try:
        serializer.serialize_u32(int(pk.Variant))
    except ValueError as exc:
        raise ValueError(
            f"failed to serialize public key variant: {exc}"
        ) from exc
    # Then serialize byte data (fixed length, no length prefix)
    serializer.serialize_fixed_bytes(pk.Bytes)


def _serialize_signature(serializer: Serializer, value: Any) -> None:
    if isinstance(value, Signature):
        sig = value
    elif isinstance(value, str):
        try:
            sig = new_signature_from_string_relaxed(value)
        except ValueError as exc:
            raise ValueError(
                f"failed to parse signature from string: {exc}"
            ) from exc
    elif isinstance(value, (bytes, bytearray)):
        try:
            sig = new_signature_from_bytes(bytes(value))
        except ValueError as exc:
            raise ValueError(
                f"failed to create Signature from bytes: {exc}"
            ) from exc
    else:
        raise ValueError(f"invalid type for Signature: {type(value).__name__}")

    try:
        serializer.serialize_u32(int(sig.Variant))
    except ValueError as exc:
        raise ValueError(
            f"failed to serialize signature variant: {exc}"
        ) from exc
    serializer.serialize_fixed_bytes(sig.Bytes)


def _as_uint64(value: Any, max_value: int, name: str) -> int:
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
            number = int(value, 10)
        except ValueError:
            raise ValueError(f"{name} out of range") from None
        if number < 0 or number > max_value:
            raise ValueError(f"{name} out of range")
        return number
    raise ValueError(f"{name} out of range")


def _as_big_int(value: Any, signed: bool) -> int:
    if isinstance(value, bool):
        raise ValueError("u128 out of range")
    if isinstance(value, int):
        if not signed and value < 0:
            raise ValueError("u128 out of range")
        return value
    if isinstance(value, float):
        if value != math.trunc(value):
            raise ValueError("u128 out of range")
        if not signed and value < 0:
            raise ValueError("u128 out of range")
        return int(value)
    if isinstance(value, str):
        try:
            number = int(value, 10)
        except ValueError:
            raise ValueError("u128 out of range") from None
        if not signed and number < 0:
            raise ValueError("u128 out of range")
        return number
    raise ValueError("u128 out of range")


def _as_int64(value: Any, min_value: int, max_value: int, name: str) -> int:
    if isinstance(value, bool):
        raise ValueError(f"{name} out of range")
    if isinstance(value, int):
        if value < min_value or value > max_value:
            raise ValueError(f"{name} out of range")
        return value
    if isinstance(value, float):
        if value != math.trunc(value) or value < float(min_value) or value > float(max_value):
            raise ValueError(f"{name} out of range")
        return int(value)
    if isinstance(value, str):
        try:
            number = int(value, 10)
        except ValueError:
            raise ValueError(f"{name} out of range") from None
        if number < min_value or number > max_value:
            raise ValueError(f"{name} out of range")
        return number
    raise ValueError(f"{name} out of range")


def _fmt_sprint(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


def _decode_hex(value: str) -> bytes:
    normalized = value
    if len(value) >= 2 and value[:2] in ("0x", "0X"):
        normalized = value[2:]
    try:
        return binascii.unhexlify(normalized)
    except (binascii.Error, ValueError):
        raise ValueError("invalid hex string") from None


def _serialize_fixed_bytes_value(serializer: Serializer, value: Any, size: int, type_name: str) -> None:
    if isinstance(value, (bytes, bytearray)):
        if len(value) != size:
            raise ValueError(
                f"{type_name} expects exactly {size} bytes, got {len(value)}"
            )
        serializer.serialize_fixed_bytes(bytes(value))
        return
    raise ValueError(f"{type_name} expects [{size}]byte or []byte")


def _deserialize_fixed_bytes_value(body: bytes, offset: int, size: int, type_name: str) -> Tuple[bytes, int]:
    if offset + size > len(body):
        raise ValueError(f"insufficient data for {type_name}")
    buf = body[offset : offset + size]
    return buf, offset + size


def load_provider_from_file(path: str) -> Provider:
    import os

    try:
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except OSError as exc:
        raise ValueError(f"failed to read IDL file {path}: {exc}") from exc
    except json.JSONDecodeError as exc:
        raise ValueError(f"failed to unmarshal IDL {path}: {exc}") from exc
    return Provider(idl_from_json_dict(data))


def new_provider(idl: IDL) -> Provider:
    return Provider(idl)
