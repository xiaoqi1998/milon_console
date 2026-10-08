"""provider 类型定义. Faithful port of gosdk-develop/provider/types.go."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

Args = Dict[str, Any]


@dataclass
class Metadata:
    AppID: int = 0
    Name: str = ""
    Description: str = ""


@dataclass
class Arg:
    Name: str = ""
    Role: str = ""  # input + signer + any_signer
    Type: str = ""  # IDL type name (u8, u16, ...)


@dataclass
class ReturnValue:
    Type: str = ""


@dataclass
class LookupPath:
    Arg: str = ""  # argument name (e.g. "token")
    Type: str = ""  # argument type (e.g. "Address")


@dataclass
class SignerLookup:
    Path: LookupPath = field(default_factory=LookupPath)
    Res: int = 0  # resource ID (for access control)


SignerLookups = Dict[str, SignerLookup]


@dataclass
class Instruction:
    Args: List[Arg] = field(default_factory=list)
    Discriminator: int = 0
    Handler: str = ""
    Kind: str = ""  # entry + view
    Name: str = ""
    Returns: Optional[ReturnValue] = None  # required for view
    SignerLookups: Optional[SignerLookups] = None  # optional for entry
    Sponsor: bool = False  # optional for entry


@dataclass
class StructField:
    Name: str = ""
    Type: str = ""


@dataclass
class EnumVariant:
    Name: str = ""
    Kind: str = ""
    Fields: List[StructField] = field(default_factory=list)


@dataclass
class IDLType:
    Fields: List[StructField] = field(default_factory=list)  # only for Kind=struct
    Variants: List[EnumVariant] = field(default_factory=list)  # only for Kind=enum
    Kind: str = ""  # struct | enum | tuple | builtin | unit
    Name: str = ""
    TypeTag: int = 0


@dataclass
class Resource:
    """A persisted on-chain value declared in the IDL resources section.

    Name carries the "_" prefix (e.g. "_Sft"); multiple resources may share
    one TypeTag."""

    Name: str = ""
    Type: str = ""  # IDL type name (u8, u16, ...)
    TypeTag: int = 0


@dataclass
class EventField:
    Name: str = ""
    Type: str = ""
    Indexed: bool = False


@dataclass
class Event:
    Name: str = ""
    Fields: List[EventField] = field(default_factory=list)
    TypeTag: int = 0


@dataclass
class ErrorDef:
    Code: int = 0
    Message: str = ""
    Name: str = ""


@dataclass
class Constant:
    Name: str = ""
    Type: str = ""
    Value: Any = None


@dataclass
class IDL:
    Metadata: Metadata = field(default_factory=Metadata)
    Instructions: List[Instruction] = field(default_factory=list)
    Types: List[IDLType] = field(default_factory=list)
    Resources: List[Resource] = field(default_factory=list)
    Events: List[Event] = field(default_factory=list)
    Errors: List[ErrorDef] = field(default_factory=list)
    Constants: List[Constant] = field(default_factory=list)


@dataclass
class DecodedTaggedValue:
    """Decoded return value of a ViewMulti method."""

    Value: Any = None


# 定长字节数组（Go [N]byte；Python 侧以 bytes 表达）
B96 = bytes
B144 = bytes
B160 = bytes
B256 = bytes
