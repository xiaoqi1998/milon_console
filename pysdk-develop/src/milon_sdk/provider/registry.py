"""IDLRegistry. Faithful port of gosdk-develop/provider/registry.go."""

from __future__ import annotations

from typing import Any, Dict, List, Optional

from ..crypto import Address, PublicKey
from .provider import Provider, _decode_view_result_item, _decode_view_var_uint
from .types import DecodedTaggedValue


class IDLRegistry:
    def __init__(
        self,
        provider_by_app_id: Dict[int, Provider],
        provider_by_name: Dict[str, Provider],
        provider_by_event_type_tag: Dict[int, Provider],
        provider_by_resource_type_tag: Dict[int, Provider],
        provider_by_type_tag: Dict[int, Provider],
    ) -> None:
        self.providerByAppID = provider_by_app_id
        self.providerByName = provider_by_name
        self.providerByEventTypeTag = provider_by_event_type_tag
        self.providerByResourceTypeTag = provider_by_resource_type_tag
        self.providerByTypeTag = provider_by_type_tag


def new_idl_registry(provider_by_idl_name: Dict[str, Provider]) -> IDLRegistry:
    provider_by_app_id: Dict[int, Provider] = {}
    provider_by_name: Dict[str, Provider] = {}
    provider_by_event_type_tag: Dict[int, Provider] = {}
    provider_by_resource_type_tag: Dict[int, Provider] = {}
    provider_by_type_tag: Dict[int, Provider] = {}

    # Sort provider names so the indexes below are deterministic.
    names = sorted(provider_by_idl_name.keys())

    for idl_name in names:
        pd = provider_by_idl_name[idl_name]
        app_id = pd.app_id()
        if app_id in provider_by_app_id:
            raise ValueError(f"duplicate app_id: {app_id}")
        provider_by_app_id[app_id] = pd

        name = pd.IDL.Metadata.Name
        if name != "":
            if name in provider_by_name:
                raise ValueError(f"duplicate app name: {name}")
            provider_by_name[name] = pd

        # event typeTags must be unique across all loaded IDLs.
        for type_tag in pd.EventByTypeTag:
            if type_tag in provider_by_event_type_tag:
                raise ValueError(
                    f"duplicate event type_tag {type_tag} across loaded IDLs"
                )
            provider_by_event_type_tag[type_tag] = pd

        # resource typeTags may repeat across IDLs; first provider wins.
        for type_tag in pd.ResourceByTypeTag:
            if type_tag not in provider_by_resource_type_tag:
                provider_by_resource_type_tag[type_tag] = pd

        # type typeTags repeat across IDLs as well; first wins.
        for type_tag in pd.IDLTypeByTypeTag:
            if type_tag not in provider_by_type_tag:
                provider_by_type_tag[type_tag] = pd

    return IDLRegistry(
        provider_by_app_id,
        provider_by_name,
        provider_by_event_type_tag,
        provider_by_resource_type_tag,
        provider_by_type_tag,
    )


def decode_instructions(
    registry: IDLRegistry, instructions: List[bytes]
) -> List[Dict[str, Any]]:
    """Decodes multiple packed instructions in batch."""
    results = []
    for i, instr in enumerate(instructions):
        try:
            decoded = decode_instruction(registry, instr)
        except ValueError as exc:
            raise ValueError(
                f"failed to decode instruction[{i}]: {exc}"
            ) from exc
        results.append(decoded)
    return results


def decode_instruction(registry: IDLRegistry, instruction: bytes) -> Dict[str, Any]:
    if len(instruction) < 3:
        raise ValueError(
            "empty instruction: need at least 3 bytes (app_id + discriminator)"
        )

    # app_id (1 byte)
    app_id = instruction[0]
    offset = 1

    # discriminator (u16 LE, 2 bytes)
    discriminator = instruction[offset] | (instruction[offset + 1] << 8)
    offset += 2

    provider = registry.providerByAppID.get(app_id)
    if provider is None:
        raise ValueError(f"unknown app_id: {app_id}")

    matched_instruction = provider.InstructionByDiscriminator.get(discriminator)
    if matched_instruction is None:
        raise ValueError(
            f"unknown discriminator: {discriminator} "
            f"(app: {provider.IDL.Metadata.Name})"
        )

    args: Dict[str, Any] = {}
    for arg in matched_instruction.Args:
        try:
            value, offset = provider.deserialize_value(arg.Type, instruction, offset)
        except ValueError as exc:
            raise ValueError(
                f"failed to decode argument '{arg.Name}' ({arg.Type}): {exc}"
            ) from exc
        args[arg.Name] = value

    # Verify no unparsed data remains
    if offset != len(instruction):
        raise ValueError(
            f"{len(instruction) - offset} trailing bytes after decoding all "
            f"arguments"
        )

    return {
        "app_id": provider.IDL.Metadata.AppID,
        "app_name": provider.IDL.Metadata.Name,
        "instruction_name": matched_instruction.Name,
        "discriminator": discriminator,
        "args": args,
    }


def decode_view_datas(
    registry: IDLRegistry, app_name_and_instruction_names: List[str], body: bytes
) -> List[DecodedTaggedValue]:
    """Decodes a view response body where each Result corresponds to a
    different method; instructionNames use "appName::methodName" format."""
    offset = 0

    # 1. vec length (result count)
    result_count, offset = _decode_view_var_uint(body, offset)

    if result_count != len(app_name_and_instruction_names):
        raise ValueError(
            f"result count {result_count} does not match instruction count "
            f"{len(app_name_and_instruction_names)}"
        )

    results = []

    # 2. each Result item
    for i in range(result_count):
        parts = app_name_and_instruction_names[i].split("::")
        if len(parts) != 2:
            raise ValueError(
                f"result[{i}]: invalid format "
                f"\"{app_name_and_instruction_names[i]}\" "
                "(expected appName::methodName)"
            )

        matched_provider = registry.providerByName.get(parts[0])
        if matched_provider is None:
            raise ValueError(f'result[{i}]: unknown app "{parts[0]}"')

        try:
            instruction = matched_provider.get_instruction_by_name(parts[1])
        except ValueError as exc:
            raise ValueError(f"result[{i}]: {exc}") from exc

        if instruction.Kind != "view":
            raise ValueError(
                f"result[{i}]: {app_name_and_instruction_names[i]} "
                f"kind={instruction.Kind}, expected view"
            )

        return_type = (instruction.Returns.Type if instruction.Returns else "").strip()
        if return_type == "":
            raise ValueError(
                f"result[{i}]: {app_name_and_instruction_names[i]} has no "
                f"returns.type in IDL"
            )

        try:
            value, offset = _decode_view_result_item(
                matched_provider, return_type, body, offset
            )
        except ValueError as exc:
            raise ValueError(f"result[{i}]: {exc}") from exc
        results.append(DecodedTaggedValue(Value=value))

    if offset != len(body):
        raise ValueError(
            f"{len(body) - offset} trailing bytes after decoding "
            f"{result_count} view results"
        )

    return results


def decode_event_data_by_tag(
    registry: IDLRegistry, type_tag: int, data: bytes
) -> Dict[str, Any]:
    """Decodes event data by its typeTag."""
    matched_provider = registry.providerByEventTypeTag.get(type_tag)
    if matched_provider is None:
        raise ValueError(
            f"unknown type tag: {type_tag} "
            f"(loaded {len(registry.providerByAppID)} IDLs)"
        )
    matched_event = matched_provider.get_event_by_type_tag(type_tag)

    # Skip a leading type_tag varint prefix when present.
    offset = 0
    try:
        stored_type_tag, offset = _decode_view_var_uint(data, offset)
        if stored_type_tag != type_tag:
            offset = 0
    except ValueError:
        offset = 0

    record: Dict[str, Any] = {}
    for field in matched_event.Fields:
        if offset >= len(data):
            raise ValueError(
                f"insufficient data for field '{field.Name}' ({field.Type})"
            )
        try:
            value, offset = matched_provider.deserialize_value(
                field.Type, data, offset
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to decode field '{field.Name}' ({field.Type}): {exc}"
            ) from exc
        record[field.Name] = value

    # Verify no unparsed data remains
    if offset != len(data):
        raise ValueError(
            f"{len(data) - offset} trailing bytes after decoding event data"
        )

    return {
        "app_id": matched_provider.IDL.Metadata.AppID,
        "app_name": matched_provider.IDL.Metadata.Name,
        "event_name": matched_event.Name,
        "data": record,
    }


def decode_resource_data_by_tag(
    registry: IDLRegistry, type_tag: int, data: bytes
) -> Dict[str, Any]:
    """Decodes a persisted value by its typeTag. Resource declarations win;
    IDL types section is the fallback."""
    matched_provider = registry.providerByResourceTypeTag.get(type_tag)
    if matched_provider is not None:
        matched_resource = matched_provider.get_resource_by_type_tag(type_tag)

        offset = 0
        try:
            value, offset = matched_provider.deserialize_value(
                matched_resource.Type, data, offset
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to decode resource '{matched_resource.Name}' "
                f"({matched_resource.Type}): {exc}"
            ) from exc

        if offset != len(data):
            raise ValueError(
                f"{len(data) - offset} trailing bytes after decoding resource "
                f"data"
            )

        return {
            "app_id": matched_provider.IDL.Metadata.AppID,
            "app_name": matched_provider.IDL.Metadata.Name,
            "resource_name": matched_resource.Name,
            "resource_type": matched_resource.Type,
            "data": value,
        }

    matched_provider = registry.providerByTypeTag.get(type_tag)
    if matched_provider is not None:
        matched_type = matched_provider.get_idl_type_by_type_tag(type_tag)

        offset = 0
        try:
            value, offset = matched_provider.deserialize_value(
                matched_type.Name, data, offset
            )
        except ValueError as exc:
            raise ValueError(
                f"failed to decode type '{matched_type.Name}': {exc}"
            ) from exc

        if offset != len(data):
            raise ValueError(
                f"{len(data) - offset} trailing bytes after decoding type data"
            )

        return {
            "app_id": matched_provider.IDL.Metadata.AppID,
            "app_name": matched_provider.IDL.Metadata.Name,
            "resource_name": matched_type.Name,
            "resource_type": matched_type.Name,
            "data": value,
        }

    raise ValueError(
        f"unknown resource type tag: {type_tag} "
        f"(loaded {len(registry.providerByAppID)} IDLs)"
    )


def format_decoded_instruction(decoded: Dict[str, Any]) -> str:
    """Formats decoded instruction into readable string."""
    app_id = decoded.get("app_id", 0)
    app_name = decoded.get("app_name", "")
    instruction_name = decoded.get("instruction_name", "")
    discriminator = decoded.get("discriminator", 0)

    sb = []
    sb.append(f"[{app_name}] {instruction_name}\n")
    sb.append("Struct {\n")
    sb.append(f"    appId: {app_id},\n")
    sb.append(f'    appName: "{app_name}",\n')
    sb.append(f'    instructionName: "{instruction_name}",\n')
    sb.append(f"    discriminator: {discriminator},\n")
    sb.append("    fields: [\n")

    args = decoded.get("args") or {}
    arg_names = sorted(args.keys())

    first = True
    for name in arg_names:
        if not first:
            sb.append(",\n")
        first = False
        sb.append("        NamedToken {\n")
        sb.append(f'            name: "{name}",\n')
        sb.append(f"            value: {_format_value(args[name])},\n")
        sb.append("        }")

    sb.append("\n    ],\n")
    sb.append("}")

    return "".join(sb)


def format_decoded_event(decoded: Dict[str, Any]) -> str:
    """Formats decoded event data into a readable string."""
    app_name = decoded.get("app_name", "")
    event_name = decoded.get("event_name", "")
    data = decoded.get("data")

    sb = []
    sb.append(f"[{app_name}] {event_name}\n")
    sb.append("Struct {\n")

    if isinstance(data, dict):
        first = True
        for k in sorted(data.keys()):
            if not first:
                sb.append(",\n")
            first = False
            sb.append(f"    {k}: {_format_value(data[k])}")
    else:
        sb.append(f"    value: {_format_value(data)}")

    sb.append("\n}")
    return "".join(sb)


def _format_value(value: Any) -> str:
    # 注：Go 版按 Go 位宽类型分派（U8/U16/U32/U64），Python int 无位宽，
    # 统一按非负/负分派 U64/I64（值语义一致）。
    if isinstance(value, Address):
        return f"Address({value.to_base58()})"
    if isinstance(value, PublicKey):
        return f"PublicKey({value.to_base58()})"
    if isinstance(value, str):
        return f'String("{value}")'
    if isinstance(value, bool):
        return f"Bool({str(value).lower()})"
    if isinstance(value, int):
        return f"U64({value})" if value >= 0 else f"I64({value})"
    if isinstance(value, (bytes, bytearray)):
        return f"Bytes({bytes(value).hex()})"
    if isinstance(value, list):
        items = ", ".join(_format_value(item) for item in value)
        return f"[{items}]"
    if isinstance(value, dict):
        sb = ["Struct {\n"]
        first = True
        for k in sorted(value.keys()):
            if not first:
                sb.append(",\n")
            first = False
            sb.append(f"                {k}: {_format_value(value[k])}")
        sb.append("\n            }")
        return "".join(sb)
    return f"{value}"
