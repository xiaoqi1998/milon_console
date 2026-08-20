"""IDL 注册表与 type_tag 解析器（对应 Go provider/registry.go + idlTypeResolver.go）。

- IDLRegistry：按 app_id / 名称 / 事件 type_tag 建立全局索引，提供指令与视图解码。
- IDLTypeResolver：实现 postcard.TypeResolver 协议，把 type_tag 解析为
  消耗掉的原始字节区间，供 api 层解码内联资源与事件。
- IDL JSON 加载：从 provider/IDL/index.json 聚合所有内置 IDL。
"""
from __future__ import annotations

import json
import os
from typing import Any

from .idl_types import (
    Arg,
    Constant,
    ErrorDef,
    Event,
    EventField,
    IDL,
    IDLType,
    Instruction,
    Metadata,
    ReturnValue,
    StructField,
)
from .provider import Provider, _Offset


# ---------------------------------------------------------------- IDL JSON 加载


def load_idl_from_dict(data: dict) -> IDL:
    """把单个 IDL JSON 对象转为 IDL dataclass。"""
    meta = data.get("metadata", {})
    metadata = Metadata(
        app_id=meta.get("app_id", 0),
        name=meta.get("name", ""),
        description=meta.get("description", ""),
    )

    instructions: list[Instruction] = []
    for ins in data.get("instructions", []):
        args = [
            Arg(
                name=a.get("name", ""),
                role=a.get("role", "input"),
                type=a.get("type", ""),
            )
            for a in ins.get("args", [])
        ]
        returns_raw = ins.get("returns") or {}
        returns = ReturnValue(type=returns_raw.get("type", ""))
        instructions.append(
            Instruction(
                name=ins["name"],
                discriminator=ins["discriminator"],
                handler=ins.get("handler", ""),
                kind=ins.get("kind", "entry"),
                args=args,
                returns=returns,
                signer_lookups=ins.get("signer_lookups", {}),
                sponsor=ins.get("sponsor", False),
            )
        )

    types: list[IDLType] = []
    for t in data.get("types", []):
        fields = [
            StructField(name=f.get("name", ""), type=f.get("type", ""))
            for f in t.get("fields", [])
        ]
        variants = [_variant_from_dict(v) for v in t.get("variants", [])]
        types.append(
            IDLType(
                name=t["name"],
                kind=t.get("kind", "struct"),
                type_tag=t.get("typeTag", 0),
                fields=fields,
                variants=variants,
            )
        )

    events: list[Event] = []
    for e in data.get("events", []):
        efields = [
            EventField(
                name=f.get("name", ""),
                type=f.get("type", ""),
                indexed=f.get("indexed", False),
            )
            for f in e.get("fields", [])
        ]
        events.append(
            Event(name=e["name"], type_tag=e.get("typeTag", 0), fields=efields)
        )

    errors = [
        ErrorDef(
            code=e.get("code", 0),
            message=e.get("message", ""),
            name=e.get("name", ""),
        )
        for e in data.get("errors", [])
    ]
    constants = [
        Constant(
            name=c.get("name", ""),
            type=c.get("type", ""),
            value=c.get("value"),
        )
        for c in data.get("constants", [])
    ]

    return IDL(
        metadata=metadata,
        instructions=instructions,
        types=types,
        events=events,
        errors=errors,
        constants=constants,
    )


def _variant_from_dict(v: dict) -> Any:
    # 复用 idl_types.EnumVariant
    from .idl_types import EnumVariant

    return EnumVariant(
        name=v.get("name", ""),
        kind=v.get("kind", "unit"),
        fields=[
            StructField(name=f.get("name", ""), type=f.get("type", ""))
            for f in v.get("fields", [])
        ],
    )


def load_idl_from_file(path: str | os.PathLike) -> IDL:
    with open(path, "r", encoding="utf-8") as f:
        return load_idl_from_dict(json.load(f))


def load_all_idls(idl_dir: str | os.PathLike) -> list[IDL]:
    """按 index.json 聚合目录内所有内置 IDL。"""
    index_path = os.path.join(idl_dir, "index.json")
    with open(index_path, "r", encoding="utf-8") as f:
        index = json.load(f)
    idls: list[IDL] = []
    for app in index.get("apps", []):
        rel = app["idl"].lstrip("./")
        path = os.path.join(idl_dir, rel)
        idls.append(load_idl_from_file(path))
    return idls


# ---------------------------------------------------------------- IDLRegistry


class IDLRegistry:
    """按 app_id / 名称 / 事件 type_tag 建立全局索引（对应 Go IDLRegistry）。"""

    def __init__(
        self,
        provider_by_name: dict[str, Provider],
        provider_by_app_id: dict[int, Provider],
        provider_by_event_type_tag: dict[int, Provider],
    ):
        self.provider_by_name = provider_by_name
        self.provider_by_app_id = provider_by_app_id
        self.provider_by_event_type_tag = provider_by_event_type_tag

    def get_provider_by_name(self, name: str) -> Provider | None:
        return self.provider_by_name.get(name)

    def get_provider_by_app_id(self, app_id: int) -> Provider | None:
        return self.provider_by_app_id.get(app_id)

    def get_provider_by_event_type_tag(self, type_tag: int) -> Provider | None:
        return self.provider_by_event_type_tag.get(type_tag)

    # ---------------------------------------------------------- 指令解码
    def decode_instruction(self, instruction: bytes) -> dict:
        if len(instruction) < 3:
            raise ValueError("empty instruction: need at least 3 bytes (app_id + discriminator)")
        app_id = instruction[0]
        discriminator = instruction[1] | (instruction[2] << 8)
        offset = 3
        pd = self.provider_by_app_id.get(app_id)
        if pd is None:
            raise ValueError(f"unknown app_id: {app_id}")
        matched = pd.instruction_by_discriminator.get(discriminator)
        if matched is None:
            raise ValueError(f"unknown discriminator: {discriminator} (app: {pd.idl.metadata.name})")
        args: dict = {}
        for arg in matched.args:
            off = pd.deserialize_value(arg.type, instruction, _Offset(offset))
            args[arg.name] = off.value
            offset = off.pos
        if offset != len(instruction):
            raise ValueError(f"{len(instruction) - offset} trailing bytes after decoding all arguments")
        return {
            "app_id": pd.app_id(),
            "app_name": pd.idl.metadata.name,
            "instruction_name": matched.name,
            "discriminator": discriminator,
            "args": args,
        }

    def decode_instructions(self, instructions: list[bytes]) -> list[dict]:
        """批量解码指令（对应 Go DecodeInstructions）。"""
        results: list[dict] = []
        for i, instr in enumerate(instructions):
            try:
                results.append(self.decode_instruction(instr))
            except ValueError as exc:
                raise ValueError(f"failed to decode instruction[{i}]: {exc}") from exc
        return results

    def decode_event_data_by_tag(self, type_tag: int, data: bytes) -> dict:
        """按事件 type_tag 解码事件数据（对应 Go DecodeEventDataByTag）。

        返回 {"app_id", "app_name", "event_name", "data"}；data 为字段名→值的映射。
        若数据开头带与 type_tag 一致的 varint 前缀则自动跳过。
        """
        pd = self.provider_by_event_type_tag.get(type_tag)
        if pd is None:
            raise ValueError(f"unknown type tag: {type_tag} (loaded {len(self.provider_by_app_id)} IDLs)")
        event = pd.event_by_type_tag.get(type_tag)
        if event is None:
            raise ValueError(f"unknown type tag: {type_tag}")

        offset = 0
        try:
            stored, pos = _decode_view_varuint(data, offset)
            if stored == type_tag:
                offset = pos
            else:
                offset = 0
        except Exception:  # noqa: BLE001
            offset = 0

        record: dict = {}
        for field in event.fields:
            if offset >= len(data):
                raise ValueError(f"insufficient data for field '{field.name}' ({field.type})")
            off = pd.deserialize_value(field.type, data, _Offset(offset))
            record[field.name] = off.value
            offset = off.pos
        if offset != len(data):
            raise ValueError(f"{len(data) - offset} trailing bytes after decoding event data")
        return {
            "app_id": pd.app_id(),
            "app_name": pd.idl.metadata.name,
            "event_name": event.name,
            "data": record,
        }

    # ---------------------------------------------------------- 格式化展示（对应 Go FormatDecoded*）
    def format_decoded_instruction(self, decoded: dict) -> str:
        """把 decode_instruction 的 dict 格式化为可读字符串（与 Go 输出一致）。"""
        app_id = decoded.get("app_id", 0)
        app_name = decoded.get("app_name", "")
        ins_name = decoded.get("instruction_name", "")
        disc = decoded.get("discriminator", 0)
        lines = [
            f"[{app_name}] {ins_name}",
            "Struct {",
            f"    appId: {app_id},",
            f'    appName: "{app_name}",',
            f'    instructionName: "{ins_name}",',
            f"    discriminator: {disc},",
            "    fields: [",
        ]
        args = decoded.get("args") or {}
        tokens = [
            f"        NamedToken {{\n"
            f'            name: "{name}",\n'
            f"            value: {self._format_value(args[name])},\n"
            f"        }}"
            for name in sorted(args)
        ]
        lines.append(",\n".join(tokens))
        lines.append("\n    ],\n}")
        return "\n".join(lines)

    def format_decoded_event(self, decoded: dict) -> str:
        """把 decode_event_data_by_tag 的 dict 格式化为可读字符串（与 Go 输出一致）。"""
        app_name = decoded.get("app_name", "")
        event_name = decoded.get("event_name", "")
        data = decoded.get("data")
        lines = [f"[{app_name}] {event_name}", "Struct {"]
        if isinstance(data, dict):
            inner = ",\n".join(
                f"    {k}: {self._format_value(data[k])}" for k in sorted(data)
            )
            if inner:
                lines.append(inner)
        else:
            lines.append(f"    value: {self._format_value(data)}")
        lines.append("}")
        return "\n".join(lines)

    def _format_value(self, value: Any) -> str:
        """镜像 Go formatValue：按类型输出可读表示。"""
        from ..crypto import Address, PublicKey

        if isinstance(value, Address):
            return f"Address({value.to_base58()})"
        if isinstance(value, PublicKey):
            return f"PublicKey({value.to_base58()})"
        if isinstance(value, str):
            return f'String("{value}")'
        if isinstance(value, bool):
            return f"Bool({value})"
        if isinstance(value, int):
            if value < 0:
                if value >= -(1 << 7):
                    return f"I8({value})"
                if value >= -(1 << 15):
                    return f"I16({value})"
                if value >= -(1 << 31):
                    return f"I32({value})"
                return f"I64({value})"
            if value < (1 << 8):
                return f"U8({value})"
            if value < (1 << 16):
                return f"U16({value})"
            if value < (1 << 32):
                return f"U32({value})"
            return f"U64({value})"
        if isinstance(value, (bytes, bytearray)):
            b = bytes(value)
            if len(b) == 12:
                return f"B96({b.hex()})"
            if len(b) == 18:
                return f"B144({b.hex()})"
            if len(b) == 20:
                return f"B160({b.hex()})"
            if len(b) == 32:
                return f"B256({b.hex()})"
            return f"Bytes({b.hex()})"
        if isinstance(value, list):
            return "[" + ", ".join(self._format_value(x) for x in value) + "]"
        if isinstance(value, dict):
            inner = ",\n".join(
                f"                {k}: {self._format_value(v)}" for k, v in sorted(value.items())
            )
            return "Struct {\n" + inner + "\n            }"
        return str(value)

    def decode_view_datas(self, app_name_and_instruction_names: list[str], body: bytes) -> list:
        """按 "app::method" 列表逐项解码视图响应（对应 Go DecodeViewDatas）。"""
        offset = 0
        result_count = _decode_view_varuint(body, offset)
        offset = result_count[1]
        if int(result_count[0]) != len(app_name_and_instruction_names):
            raise ValueError(
                f"result count {result_count[0]} does not match instruction count "
                f"{len(app_name_and_instruction_names)}"
            )
        results = []
        for name in app_name_and_instruction_names:
            parts = name.split("::")
            if len(parts) != 2:
                raise ValueError(f"invalid format {name!r} (expected appName::methodName)")
            pd = self.provider_by_name.get(parts[0])
            if pd is None:
                raise ValueError(f"unknown app {parts[0]!r}")
            ins = pd.instruction_by_name.get(parts[1])
            if ins is None:
                raise ValueError(f"unknown instruction {parts[1]!r}")
            if ins.kind != "view":
                raise ValueError(f"{name} kind={ins.kind}, expected view")
            return_type = ins.returns.type.strip()
            if not return_type:
                raise ValueError(f"{name} has no returns.type in IDL")
            off = _Offset(offset)
            off = _decode_view_result_item(pd, return_type, body, off)
            results.append(off.value)
            offset = off.pos
        if offset != len(body):
            raise ValueError(f"{len(body) - offset} trailing bytes after decoding view results")
        return results


def _decode_view_varuint(input_: bytes, start: int):
    value = 0
    shift = 0
    pos = start
    for _ in range(10):
        if pos >= len(input_):
            raise ValueError("unexpected end of input")
        b = input_[pos]
        pos += 1
        value |= (b & 0x7F) << shift
        if b & 0x80 == 0:
            return value, pos
        shift += 7
    raise ValueError("varint is too long")


def _decode_view_result_item(pd: Provider, return_type: str, body: bytes, off: _Offset):
    variant_index = _decode_view_varuint(body, off.pos)
    off.pos = variant_index[1]
    if variant_index[0] == 0:
        ok_len = _decode_view_varuint(body, off.pos)
        off.pos = ok_len[1]
        ok_data = body[off.pos : off.pos + ok_len[0]]
        inner = pd.deserialize_value(return_type, ok_data, _Offset(0))
        if inner.pos != len(ok_data):
            raise ValueError(f"{len(ok_data) - inner.pos} trailing bytes after decoding Ok value")
        off.pos += ok_len[0]
        return _Offset(off.pos, inner.value)
    if variant_index[0] == 1:
        code = _decode_view_varuint(body, off.pos)
        off.pos = code[1]
        msg_len = _decode_view_varuint(body, off.pos)
        off.pos = msg_len[1]
        message = body[off.pos : off.pos + msg_len[0]].decode("utf-8")
        off.pos += msg_len[0]
        data_len = _decode_view_varuint(body, off.pos)
        off.pos = data_len[1]
        data = body[off.pos : off.pos + data_len[0]]
        off.pos += data_len[0]
        return _Offset(off.pos, {"code": code[0], "message": message, "data": data})
    raise ValueError(f"invalid result variant index: {variant_index[0]}")


def NewIDLRegistry(provider_by_name: dict[str, Provider]) -> IDLRegistry:
    provider_by_app_id: dict[int, Provider] = {}
    provider_by_event_type_tag: dict[int, Provider] = {}

    for name, pd in provider_by_name.items():
        app_id = pd.app_id()
        if app_id in provider_by_app_id:
            raise ValueError(f"duplicate app_id: {app_id}")
        provider_by_app_id[app_id] = pd

        # name 即 provider_by_name 的 key，天然唯一；保留显式校验以保持与 Go 一致
        if name and name in provider_by_name and provider_by_name[name] is not pd:
            raise ValueError(f"duplicate app name: {name}")

        for type_tag in pd.event_by_type_tag:
            if type_tag in provider_by_event_type_tag:
                raise ValueError(f"duplicate event type_tag {type_tag} across loaded IDLs")
            provider_by_event_type_tag[type_tag] = pd

    return IDLRegistry(provider_by_name, provider_by_app_id, provider_by_event_type_tag)


# ---------------------------------------------------------------- IDLTypeResolver


class IDLTypeResolver:
    """实现 postcard.TypeResolver 协议（对应 Go IDLTypeResolver）。"""

    def __init__(self, providers: dict[str, Provider]):
        self.providers = providers
        self._provider_by_type_tag: dict[int, Provider] | None = None
        self._provider_by_event_type_tag: dict[int, Provider] | None = None

    def _build(self) -> None:
        if self._provider_by_type_tag is not None:
            return
        pbt: dict[int, Provider] = {}
        pbet: dict[int, Provider] = {}
        for name in sorted(self.providers.keys()):
            pd = self.providers[name]
            for tt in pd.idl_type_by_type_tag:
                pbt.setdefault(tt, pd)
            for tt in pd.event_by_type_tag:
                pbet.setdefault(tt, pd)
        self._provider_by_type_tag = pbt
        self._provider_by_event_type_tag = pbet

    def decode_resource(self, type_tag: int, data: bytes) -> tuple[bytes, bytes]:
        self._build()
        assert self._provider_by_type_tag is not None
        pd = self._provider_by_type_tag.get(type_tag)
        if pd is None:
            raise ValueError(f"unknown resource type_tag {type_tag} (not found in any loaded IDL)")
        idl_type = pd.get_idl_type_by_type_tag(type_tag)
        if idl_type is None:
            raise ValueError(f"unknown resource type_tag {type_tag}")
        off = pd.deserialize_value(idl_type.name, data, _Offset(0))
        consumed = off.pos
        return data[:consumed], data[consumed:]

    def decode_event(self, type_tag: int, data: bytes) -> tuple[bytes, bytes]:
        self._build()
        assert self._provider_by_event_type_tag is not None
        pd = self._provider_by_event_type_tag.get(type_tag)
        if pd is None:
            raise ValueError(f"unknown event type_tag {type_tag} (not found in any loaded IDL)")
        event = pd.get_event_by_type_tag(type_tag)
        if event is None:
            raise ValueError(f"unknown event type_tag {type_tag}")
        off = _Offset(0)
        for field in event.fields:
            off = pd.deserialize_value(field.type, data, off)
        consumed = off.pos
        return data[:consumed], data[consumed:]
