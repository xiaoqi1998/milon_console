"""milon_sdk.gen — Faithful (运行时构建) port of gosdk-develop/gen."""

from . import gen as _gen
from .gen import (
    Binder,
    GenApp,
    GenInstruction,
    TokenMetadata,
    bind_all,
    default_idls,
    get_app,
    register_app,
)


def __getattr__(app_name: str) -> GenApp:
    """PEP 562：包级 app 变量（``gen.VALIDATOR`` / ``from milon_sdk.gen import
    TOKEN``），集合跟随 provider/IDL/index.json。"""
    return getattr(_gen, app_name)


def __dir__() -> list:
    return sorted(set(globals()) | set(dir(_gen)))


__all__ = [n for n in dir() if not n.startswith("_")]
