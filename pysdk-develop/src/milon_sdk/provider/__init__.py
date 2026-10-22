"""milon_sdk.provider — Faithful port of gosdk-develop/provider."""

from .types import (
    IDL,
    Arg,
    Args,
    B96,
    B144,
    B160,
    B256,
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
from .provider import (
    Provider,
    idl_from_json_dict,
    load_provider_from_file,
    new_provider,
)
from .registry import (
    IDLRegistry,
    decode_event_data_by_tag,
    decode_instruction,
    decode_instructions,
    decode_resource_data_by_tag,
    decode_view_datas,
    format_decoded_event,
    format_decoded_instruction,
    new_idl_registry,
)
from .idl_type_resolver import IDLTypeResolver

import json
import os
from typing import Dict, List

_IDL_DIR = os.path.join(os.path.dirname(__file__), "IDL")


def _load_index() -> dict:
    index_path = os.path.join(_IDL_DIR, "index.json")
    if not os.path.isfile(index_path):
        return {}
    with open(index_path, "r", encoding="utf-8") as f:
        return json.load(f)


def default_idl_app_names() -> List[str]:
    """index.json 注册的 app 名列表（顺序与 index 一致），即 Go 侧 gen 包级
    app 变量的数据来源。"""
    return [entry["name"] for entry in _load_index().get("apps", [])]


def load_default_idls() -> Dict[str, IDL]:
    """加载随包分发的 provider/IDL：以 index.json 为准（对应 Go 由 index.json
    生成的 gen.DefaultIDLs）。目录内未列入 index 的 IDL 文件不参与注册。"""
    idls: Dict[str, IDL] = {}
    for entry in _load_index().get("apps", []):
        path = os.path.normpath(os.path.join(_IDL_DIR, entry["idl"]))
        with open(path, "r", encoding="utf-8") as f:
            idls[entry["name"]] = idl_from_json_dict(json.load(f))
    return idls


__all__ = [n for n in dir() if not n.startswith("_")]
