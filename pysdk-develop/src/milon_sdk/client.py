"""Client. Faithful port of gosdk-develop/client.go（options 简化为等价参数）."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Callable, Dict, Optional

from . import gen
from .lib import set_chain_id
from .network import Network
from .provider import IDLTypeResolver, new_idl_registry
from .rpc_client_v1 import RpcClientV1

# ClientOption — for NewClient（Python 侧以 (poll_period, poll_timeout) 等价表达）
ClientOption = Callable[["RpcClientV1"], None]


def with_client_poll_period(period: float) -> ClientOption:
    """Sets the default polling interval for wait_for_transaction (seconds)."""

    def _opt(rpc: RpcClientV1) -> None:
        rpc.pollPeriod = period

    return _opt


def with_client_poll_timeout(timeout: float) -> ClientOption:
    """Sets the default polling timeout for wait_for_transaction (seconds)."""

    def _opt(rpc: RpcClientV1) -> None:
        rpc.pollTimeout = timeout

    return _opt


@dataclass
class Client:
    RpcClient: RpcClientV1
    network: Network


def new_client(config: Network, *options: ClientOption) -> Client:
    set_chain_id(config.ChainId)

    rpc = RpcClientV1(
        network=config,
        poll_period=1.0,
        poll_timeout=30.0,
    )
    for opt in options:
        opt(rpc)

    # Go: rpc.LoadIDLsFromData(gen.DefaultIDLs)
    rpc.load_idls_from_data(gen.default_idls())

    rpc.typeResolver = IDLTypeResolver(providers=rpc.get_all_pd())

    try:
        rpc.providerManager = new_idl_registry(rpc.get_all_pd())
    except ValueError as exc:
        raise RuntimeError(str(exc)) from exc

    gen.bind_all(rpc.get_all_pd())

    return Client(RpcClient=rpc, network=config)


# ========================================
# RequestOption / WaitOption — for RPC calls
# （Go: type-safe option closures；Python 侧等价以 dict 载体 + 函数集，
#   request_id/poll 参数真实生效于 RpcClientV1 各方法）
# ========================================

RequestOption = Dict[str, Any]
WaitOption = Dict[str, Any]


def with_request_id(request_id: int) -> RequestOption:
    """Go: WithRequestID."""
    return {"request_id": request_id}


def with_context(ctx: Any) -> RequestOption:
    """Go: WithContext（Python 侧记录调用方上下文对象，传输层不消费）。"""
    return {"ctx": ctx}


def with_wait_request_id(request_id: int) -> WaitOption:
    """Go: WithWaitRequestID."""
    return {"request_id": request_id}


def with_wait_context(ctx: Any) -> WaitOption:
    """Go: WithWaitContext."""
    return {"ctx": ctx}


def with_wait_poll_period(period: float) -> WaitOption:
    """Go: WithWaitPollPeriod (seconds)."""
    return {"poll_period": period}


def with_wait_poll_timeout(timeout: float) -> WaitOption:
    """Go: WithWaitPollTimeout (seconds)."""
    return {"poll_timeout": timeout}


def apply_request_options(opts):
    """Go: applyRequestOptions — 合并 RequestOption 列表。"""
    merged: RequestOption = {}
    for o in opts or []:
        merged.update(o)
    return merged


def apply_wait_options(opts, default_period: float = 1.0, default_timeout: float = 30.0):
    """Go: applyWaitOptions — 合并 WaitOption 列表并补默认值。"""
    merged: WaitOption = {
        "poll_period": default_period,
        "poll_timeout": default_timeout,
    }
    for o in opts or []:
        merged.update(o)
    return merged
