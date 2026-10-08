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
from typing import Dict

_IDL_DIR = os.path.join(os.path.dirname(__file__), "IDL")


def load_default_idls() -> Dict[str, IDL]:
    """加载随包分发的 provider/IDL/*.json（对应 Go gen.DefaultIDLs 的数据源）。"""
    idls: Dict[str, IDL] = {}
    if not os.path.isdir(_IDL_DIR):
        return idls
    for fname in sorted(os.listdir(_IDL_DIR)):
        if not fname.endswith(".idl.json") and fname != "index.json":
            continue
        if fname == "index.json":
            continue
        with open(os.path.join(_IDL_DIR, fname), "r", encoding="utf-8") as f:
            idls[fname[: -len(".idl.json")]] = idl_from_json_dict(json.load(f))
    return idls


__all__ = [n for n in dir() if not n.startswith("_")]
