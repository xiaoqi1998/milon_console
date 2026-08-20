"""运行时动态 IDL 绑定（对应 Go gen 包，但不移植代码生成器）。

设计要点（见迁移评估 §6.3）：
- 不移植 tools/idlgen，而是在 NewClient 时加载 IDL JSON，动态构造
  `gen.Token.BalanceOf.Args(...).Encode()` / `.DecodeView(...)` 形态。
- 零生成代码、与 IDL 自动同步、接口天然一致。
- gen.DefaultIDLs 为内置 IDL 列表（供 rpcClientV1.LoadIDLsFromData 使用）。
- bind_all(providers) 在每个 NewClient 时重新绑定，确保绑定到最新加载的 Provider。
"""
from __future__ import annotations

import os
import re
import sys
from typing import Any

from .provider import Provider, load_all_idls
from .provider.idl_types import IDL, Instruction


def _pascal(name: str) -> str:
    """把 IDL app 名转为 PascalCase（与 Go tools/idlgen 的 pascal() 对齐）。

    Go: strings.FieldsFunc(s, '_'|'-'|' ') 去分隔符后每词首字母大写。
    例：'token' -> 'Token'，'my_app' -> 'MyApp'，'nft' -> 'Nft'。
    """
    parts = re.split(r"[_\- ]+", name.strip())
    return "".join(p[:1].upper() + p[1:] for p in parts if p)

# 内置 IDL 目录（与本模块同级的 provider/IDL）
_IDL_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "provider", "IDL")


class _ArgsBinding:
    """gen.Token.BalanceOf.Args(a, b) → 持有参数，提供 .Encode()。"""

    def __init__(self, provider: Provider, instruction: Instruction, args: dict):
        self._provider = provider
        self._instruction = instruction
        self._args = args

    def Encode(self) -> bytes:
        return self._provider.encode(self._instruction.name, self._args)

    def __repr__(self) -> str:
        return f"<Args {self._instruction.name}({', '.join(self._args.keys())})>"


class _InstructionBinding:
    """gen.Token.BalanceOf → 提供 .Args(...) 与 .DecodeView(...)。"""

    def __init__(self, provider: Provider, instruction: Instruction):
        self._provider = provider
        self._instruction = instruction

    def Args(self, *positional: Any) -> _ArgsBinding:
        arg_names = [a.name for a in self._instruction.args]
        if len(positional) != len(arg_names):
            raise ValueError(
                f"{self._instruction.name} expects {len(arg_names)} positional args "
                f"({arg_names}), got {len(positional)}"
            )
        args = dict(zip(arg_names, positional))
        return _ArgsBinding(self._provider, self._instruction, args)

    def DecodeView(self, body: bytes) -> Any:
        return self._provider.decode_view_data(self._instruction.name, body)

    def __repr__(self) -> str:
        return f"<Instruction {self._instruction.name}>"


class _AppBinding:
    """gen.Token → 每个指令名暴露为一个 _InstructionBinding 属性。"""

    def __init__(self, provider: Provider):
        self._provider = provider
        for ins in provider.idl.instructions:
            setattr(self, ins.name, _InstructionBinding(provider, ins))

    def __repr__(self) -> str:
        names = [ins.name for ins in self._provider.idl.instructions]
        return f"<App {self._provider.idl.metadata.name}: {', '.join(names)}>"


# ---------------------------------------------------------------- 全局绑定状态

_PROVIDERS: dict[str, Provider] = {}
_APPS: dict[str, _AppBinding] = {}

# 内置 IDL 列表（模块导入即加载，供 rpcClientV1 使用）
DefaultIDLs: list[IDL] = load_all_idls(_IDL_DIR)


def _build_providers(idls: list[IDL]) -> dict[str, Provider]:
    providers: dict[str, Provider] = {}
    for idl in idls:
        name = idl.metadata.name
        if not name:
            raise ValueError("IDL metadata name is empty")
        providers[name] = Provider(idl)
    return providers


def bind_all(providers: dict[str, Provider]) -> None:
    """把每个 app 的指令绑定到最新加载的 Provider（对应 Go gen.BindAll）。

    以 PascalCase(app 名) 作为键，使 `gen.Token` / `gen.Account` 立即可用，
    并与 Go 生成代码（tools/idlgen）的命名保持一致。
    """
    global _PROVIDERS, _APPS
    _PROVIDERS = dict(providers)
    _APPS = {}
    module = sys.modules[__name__]
    # 清理上一次的 PascalCase 绑定，避免旧 app 名残留
    for old_name in list(_APPS.keys()):
        if old_name in module.__dict__:
            del module.__dict__[old_name]
    for raw_name, pd in providers.items():
        app_name = _pascal(raw_name)
        app = _AppBinding(pd)
        _APPS[app_name] = app
        setattr(module, app_name, app)  # 真实模块属性，gen.Token 直接命中


def load_providers() -> dict[str, Provider]:
    """从内置 IDL 构建 Provider 映射。"""
    return _build_providers(DefaultIDLs)


# ================================================================
# Pythonic 绑定适配器（snake_case 形态，供测试/示例使用）
#
#   gen.default_bindings().token.BalanceOf.args(a, b).encode()
#   gen.default_bindings().token.BalanceOf.decode(wire)        # wire → 命名参数
#   gen.default_bindings().token.BalanceOf.decode_view(body)   # view 返回体解码
#
# 与上方 Go 风格 gen.Token.BalanceOf.Args().Encode() 并存（rpc.py 使用后者）。
# ================================================================


class _PyArgsBinding:
    """default_bindings().token.BalanceOf.args(...) → 持有参数，提供 .encode()。"""

    def __init__(self, provider: "Provider", instruction: "Instruction", args: dict):
        self._provider = provider
        self._instruction = instruction
        self._args = args

    def encode(self) -> bytes:
        return self._provider.encode(self._instruction.name, self._args)

    # Go 风格别名（兼容 rpc.py 写法）
    def Encode(self) -> bytes:
        return self.encode()


class _PyInstructionBinding:
    """default_bindings().token.BalanceOf → .args()/.decode()/.decode_view()。"""

    def __init__(self, provider: "Provider", instruction: "Instruction"):
        self._provider = provider
        self._instruction = instruction

    def args(self, *positional: Any) -> _PyArgsBinding:
        arg_names = [a.name for a in self._instruction.args]
        if len(positional) != len(arg_names):
            raise ValueError(
                f"{self._instruction.name} expects {len(arg_names)} positional args "
                f"({arg_names}), got {len(positional)}"
            )
        return _PyArgsBinding(self._provider, self._instruction, dict(zip(arg_names, positional)))

    # Go 风格别名
    def Args(self, *positional: Any) -> _PyArgsBinding:
        return self.args(*positional)

    def decode(self, wire: bytes) -> dict:
        return self._provider.decode(self._instruction.name, wire)

    def Decode(self, wire: bytes) -> dict:
        return self.decode(wire)

    def decode_view(self, body: bytes) -> Any:
        return self._provider.decode_view_data(self._instruction.name, body)

    def DecodeView(self, body: bytes) -> Any:
        return self.decode_view(body)


class _PyAppBinding:
    """default_bindings().token → 每个指令名暴露为 _PyInstructionBinding 属性。"""

    def __init__(self, provider: "Provider"):
        self._provider = provider
        for ins in provider.idl.instructions:
            setattr(self, ins.name, _PyInstructionBinding(provider, ins))

    def __dir__(self) -> list[str]:
        return [ins.name for ins in self._provider.idl.instructions]


class _BindingsContainer:
    """gen.default_bindings() 返回的容器：同时以 PascalCase 与 snake_case 暴露 app 名。"""

    def _add(self, name: str, app: _PyAppBinding) -> None:
        setattr(self, name, app)

    def __dir__(self) -> list[str]:
        return [n for n in self.__dict__ if not n.startswith("_")]


def default_bindings() -> _BindingsContainer:
    """返回 Pythonic 绑定容器（对应 Go gen 包在 NewClient 后可用的一套）。

    暴露：container.token.BalanceOf.args(...).encode() / .decode(wire) /
    .decode_view(body)；同时以 PascalCase（container.Token...）暴露，兼容两种习惯。
    """
    container = _BindingsContainer()
    for raw_name, provider in _PROVIDERS.items():
        app = _PyAppBinding(provider)
        container._add(raw_name, app)  # snake_case：token / account / system
        container._add(_pascal(raw_name), app)  # PascalCase：Token / Account / System
    return container


# ---------------------------------------------------------------- 懒加载属性访问

def __getattr__(name: str) -> Any:  # 模块级兜底（PEP 562）
    if name in _APPS:
        return _APPS[name]
    raise AttributeError(f"module 'milon_sdk.gen' has no attribute {name!r}")


# 模块导入即绑定一次，使 gen.Token 立即可用
bind_all(_build_providers(DefaultIDLs))
