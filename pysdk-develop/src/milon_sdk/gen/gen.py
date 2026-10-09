"""gen 包（Python 侧为运行时构建方案）.

Go 侧 gen/idl_gen.go 由 tools/idlgen 从 provider/IDL/index.json 生成静态代码
（DEFAULT_IDLS + 类型化 instruction builder + 自定义类型 dataclass）。Python 侧
等价地**运行时**从随包分发的 provider/IDL/*.json 构建（数据同源，gosdk IDL
升级时 Python 侧自动同步），typed 访问语义与 Go 一致：

    gen.TOKEN.CLAIM_FAUCET.args(account).encode()
    gen.TOKEN.BALANCE_OF.decode_view(view_result.HTTPResponseBody)

Go 的 RegisterApp/BindAll 机制（每次 NewClient 重绑 provider）以
``register_app``/``bind_all`` 保留同语义。
"""

from __future__ import annotations

import threading
from typing import Any, Callable, Dict, Optional

from ..provider import (
    IDL,
    Provider,
    idl_from_json_dict,
    load_default_idls,
)

Binder = Callable[[Provider], None]

_binders_lock = threading.RLock()
_binders: Dict[str, Binder] = {}


def register_app(app_name: str, binder: Binder) -> None:
    """Registers the binder of a generated IDL app."""
    with _binders_lock:
        _binders[app_name] = binder


def bind_all(providers: Dict[str, Provider]) -> None:
    """Rebinds every registered generated app with the currently loaded
    providers."""
    with _binders_lock:
        for app_name, binder in _binders.items():
            pd = providers.get(app_name)
            if pd is None:
                raise ValueError(f"gen: provider for IDL app {app_name!r} not loaded")
            binder(pd)


class GenInstruction:
    """Typed instruction builder: ``gen.TOKEN.CLAIM_FAUCET.args(x).encode()``."""

    def __init__(self, app: "GenApp", instruction_name: str) -> None:
        self._app = app
        self._name = instruction_name
        self._pending_args: Dict[str, Any] = {}

    def args(self, **kwargs: Any) -> "GenInstruction":
        self._pending_args.update(kwargs)
        return self

    def encode(self) -> bytes:
        return self._app.provider.encode(self._name, self._pending_args)

    def decode(self, body: bytes) -> Dict[str, Any]:
        return self._app.provider.decode(self._name, body)

    def decode_view(self, http_response_body: bytes) -> Any:
        """Decodes a Vec<Result<T>> view response body（单值语义，与 Go
        DecodeView 一致：返回首个 Result 的值）。"""
        values = self._app.provider.decode_view_datas(self._name, http_response_body)
        if not values:
            raise ValueError(f"view {self._name} returned no values")
        return values[0].Value

    def decode_view_datas(self, http_response_body: bytes):
        return self._app.provider.decode_view_datas(self._name, http_response_body)


def _to_upper_snake(name: str) -> str:
    out = []
    for i, ch in enumerate(name):
        if ch.isupper() and i > 0 and not name[i - 1].isupper():
            out.append("_")
        out.append(ch.upper())
    return "".join(out)


class GenApp:
    """One IDL app's typed instruction namespace (attr 驱动，UPPER_SNAKE)。"""

    def __init__(self, app_name: str, provider: Optional[Provider] = None) -> None:
        self._app_name = app_name
        self.provider = provider
        self._instructions: Dict[str, GenInstruction] = {}

    def rebind(self, provider: Provider) -> None:
        self.provider = provider
        self._instructions.clear()

    def __getattr__(self, instruction_name: str) -> GenInstruction:
        # UPPER_SNAKE attr → Go PascalCase instruction 名匹配
        if self.provider is None:
            raise AttributeError(
                f"gen app {self._app_name!r} is not bound to a provider yet"
            )
        inst = self._instructions.get(instruction_name)
        if inst is None:
            # 大小写不敏感匹配（CLAIM_FAUCET ↔ ClaimFaucet）
            canonical = None
            target = instruction_name.lower()
            for name in self.provider.InstructionByName:
                if name.lower() == target or _to_upper_snake(name) == instruction_name:
                    canonical = name
                    break
            if canonical is None:
                raise AttributeError(
                    f"IDL method not found: {self._app_name}.{instruction_name}"
                )
            inst = GenInstruction(self, canonical)
            self._instructions[instruction_name] = inst
        return inst


# ---- 内置 app 对象（对齐 Go gen 包级变量：11 个 app 全量）----

_APPS = {
    "system": GenApp("system"),
    "account": GenApp("account"),
    "token": GenApp("token"),
    "staking": GenApp("staking"),
    "identity": GenApp("identity"),
    "sftoken": GenApp("sftoken"),
    "dex": GenApp("dex"),
    "keyless": GenApp("keyless"),
    "lucky_box": GenApp("lucky_box"),
    "social": GenApp("social"),
    "demo": GenApp("demo"),
}

# 模块级变量（UPPER_SNAKE；Go 侧为 PascalCase 包级变量）
SYSTEM = _APPS["system"]
ACCOUNT = _APPS["account"]
TOKEN = _APPS["token"]
STAKING = _APPS["staking"]
IDENTITY = _APPS["identity"]
SFTOKEN = _APPS["sftoken"]
DEX = _APPS["dex"]
KEYLESS = _APPS["keyless"]
LUCKY_BOX = _APPS["lucky_box"]
SOCIAL = _APPS["social"]
DEMO = _APPS["demo"]

for _name, _app in _APPS.items():
    register_app(_name, _app.rebind)


def _load_default_idls() -> Dict[str, IDL]:
    return load_default_idls()


def default_idls() -> Dict[str, IDL]:
    """DEFAULT_IDLS 的等价物（运行时加载）。"""
    return _load_default_idls()


def get_app(app_name: str) -> GenApp:
    name = app_name.lower()
    if name not in _APPS:
        raise ValueError(f"unknown gen app: {app_name}")
    return _APPS[name]


class TokenMetadata(dict):
    """gen.TokenMetadata 占位（Go 生成 dataclass）。Python 侧 decode_view 返回
    dict；此类仅保留类型名以对齐 Go 导出面。"""
