"""IDL 加载器：从 JSON 文件/目录构建 Provider（对应 Go LoadProviderFromFile + gen.DefaultIDLs）。

内置 IDL 目录随包分发（provider/IDL/*.json，源自 Go 仓库 provider/IDL/）。
也可通过 load_idls_from_dir 从任意目录加载（需 index.json 或直接扫描 *.idl.json）。
"""
from __future__ import annotations

import json
import os
from dataclasses import asdict
from typing import Any

from .idl_types import (
    IDL,
    Arg,
    Constant,
    EnumVariant,
    ErrorDef,
    Event,
    EventField,
    IDLType,
    Instruction,
    LookupPath,
    Metadata,
    ReturnValue,
    SignerLookup,
    StructField,
)
from .provider import Provider

_IDL_DIR = os.path.join(os.path.dirname(__file__), "IDL")


def _parse_idl(data: dict) -> IDL:
    meta = data.get("metadata", {}) or {}
    metadata = Metadata(
        app_id=meta.get("app_id", 0),
        name=meta.get("name", ""),
        description=meta.get("description", ""),
    )

    instructions: list[Instruction] = []
    for raw in data.get("instructions", []) or []:
        args = [
            Arg(name=a.get("name", ""), role=a.get("role", "input"), type=a.get("type", ""))
            for a in raw.get("args", []) or []
        ]
        returns_raw = raw.get("returns", {}) or {}
        signer_lookups = {}
        for role, cfg in (raw.get("signer_lookups", {}) or {}).items():
            path = cfg.get("path", {}) or {}
            signer_lookups[role] = SignerLookup(
                path=LookupPath(arg=path.get("arg", ""), type=path.get("type", "")),
                res=cfg.get("res", 0),
            )
        instructions.append(
            Instruction(
                name=raw.get("name", ""),
                discriminator=raw.get("discriminator", 0),
                handler=raw.get("handler", ""),
                kind=raw.get("kind", "entry"),
                args=args,
                returns=ReturnValue(type=returns_raw.get("type", "")),
                signer_lookups=signer_lookups,
                sponsor=raw.get("sponsor", False),
            )
        )

    idl_types = []
    for raw in data.get("types", []) or []:
        fields = [
            StructField(name=f.get("name", ""), type=f.get("type", ""))
            for f in (raw.get("fields", []) or [])
        ]
        variants = []
        for v in (raw.get("variants", []) or []):
            variants.append(
                EnumVariant(
                    name=v.get("name", ""),
                    kind=v.get("kind", "unit"),
                    fields=[StructField(name=f.get("name", ""), type=f.get("type", "")) for f in (v.get("fields", []) or [])],
                )
            )
        idl_types.append(
            IDLType(
                name=raw.get("name", ""),
                kind=raw.get("kind", ""),
                type_tag=raw.get("typeTag", 0),
                fields=fields,
                variants=variants,
            )
        )

    events = []
    for raw in data.get("events", []) or []:
        events.append(
            Event(
                name=raw.get("name", ""),
                type_tag=raw.get("typeTag", 0),
                fields=[
                    EventField(name=f.get("name", ""), type=f.get("type", ""), indexed=f.get("indexed", False))
                    for f in (raw.get("fields", []) or [])
                ],
            )
        )

    errors = [ErrorDef(code=e.get("code", 0), message=e.get("message", ""), name=e.get("name", "")) for e in (data.get("errors", []) or [])]
    constants = [Constant(name=c.get("name", ""), type=c.get("type", ""), value=c.get("value")) for c in (data.get("constants", []) or [])]

    return IDL(
        metadata=metadata,
        instructions=instructions,
        types=idl_types,
        events=events,
        errors=errors,
        constants=constants,
    )


def load_provider_from_file(path: str) -> Provider:
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return Provider(_parse_idl(data))


def load_idls_from_dir(directory: str | None = None) -> list[IDL]:
    """加载目录下所有 *.idl.json（按文件名排序，确定性）。"""
    directory = directory or _IDL_DIR
    if not os.path.isdir(directory):
        raise FileNotFoundError(f"IDL directory not found: {directory}")
    idls: list[IDL] = []
    index_path = os.path.join(directory, "index.json")
    if os.path.exists(index_path):
        # 优先按 index.json 的 apps 顺序加载
        with open(index_path, "r", encoding="utf-8") as f:
            index = json.load(f)
        for app in index.get("apps", []):
            idl_path = os.path.join(directory, app["idl"])
            if os.path.exists(idl_path):
                idls.append(_parse_idl(json.load(open(idl_path, "r", encoding="utf-8"))))
    else:
        for name in sorted(os.listdir(directory)):
            if name.endswith(".idl.json"):
                with open(os.path.join(directory, name), "r", encoding="utf-8") as f:
                    idls.append(_parse_idl(json.load(f)))
    if not idls:
        raise ValueError("empty IDL data")
    return idls


def load_default_idls() -> list[IDL]:
    """加载包内置 IDL（对应 Go gen.DefaultIDLs）。"""
    return load_idls_from_dir()
