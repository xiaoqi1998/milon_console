"""Client. Faithful port of gosdk-develop/client.go（options 简化为等价参数）."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Callable, Optional

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
