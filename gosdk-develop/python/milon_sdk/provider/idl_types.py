"""IDL 数据结构（对应 Go provider/types.go）。"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class Metadata:
    app_id: int
    name: str
    description: str = ""


@dataclass
class Arg:
    name: str
    role: str = "input"
    type: str = ""


@dataclass
class LookupPath:
    arg: str = ""
    type: str = ""


@dataclass
class SignerLookup:
    path: LookupPath = field(default_factory=LookupPath)
    res: int = 0


@dataclass
class ReturnValue:
    type: str = ""


@dataclass
class Instruction:
    name: str
    discriminator: int
    handler: str = ""
    kind: str = "entry"  # entry | view
    args: list[Arg] = field(default_factory=list)
    returns: ReturnValue = field(default_factory=ReturnValue)
    signer_lookups: dict[str, SignerLookup] = field(default_factory=dict)
    sponsor: bool = False


@dataclass
class StructField:
    name: str
    type: str = ""


@dataclass
class EnumVariant:
    name: str
    kind: str = "unit"  # unit | struct | tuple
    fields: list[StructField] = field(default_factory=list)


@dataclass
class IDLType:
    name: str
    kind: str  # struct | enum | tuple | builtin | unit
    type_tag: int = 0
    fields: list[StructField] = field(default_factory=list)
    variants: list[EnumVariant] = field(default_factory=list)


@dataclass
class EventField:
    name: str
    type: str = ""
    indexed: bool = False


@dataclass
class Event:
    name: str
    type_tag: int = 0
    fields: list[EventField] = field(default_factory=list)


@dataclass
class ErrorDef:
    code: int = 0
    message: str = ""
    name: str = ""


@dataclass
class Constant:
    name: str = ""
    type: str = ""
    value: Any = None


@dataclass
class IDL:
    metadata: Metadata = field(default_factory=Metadata)
    instructions: list[Instruction] = field(default_factory=list)
    types: list[IDLType] = field(default_factory=list)
    events: list[Event] = field(default_factory=list)
    errors: list[ErrorDef] = field(default_factory=list)
    constants: list[Constant] = field(default_factory=list)


Args = dict


@dataclass
class DecodedTaggedValue:
    value: Any
