"""Client 门面（对应 Go client.go）。

- NewClient(network, *options) 构建 Client，加载内置 IDL、建立注册表与
  type_tag 解析器，并重新绑定 gen 动态调用入口。
- 提供与 Go 一一对应的公共方法（账户 / 交易 / 链查询 / 资源 / 合约视图）。
- 选项（functional options，对应 Go 的 RequestOption / WaitOption / ClientOption）：
  WithContext / WithRequestID / WithWaitContext / WithWaitRequestID /
  WithWaitPollPeriod / WithWaitPollTimeout / WithClientPollPeriod / WithClientPollTimeout。
"""
from __future__ import annotations

import threading
import time
from typing import Any, Callable, Optional

from .api import base as apibase
from .crypto import Address, SecretKeyer
from .crypto.errors import MilonCryptoError
from .lib import AccountSignatureMode, Transaction
from .lib.chain import set_chain_id
from .network import Network
from .postcard import TypeResolver  # for typing only
from .provider import IDLRegistry, IDLTypeResolver, NewIDLRegistry
from . import gen
from .rpc import RpcClientV1

RequestOption = Callable[[dict], None]
WaitOption = Callable[[dict], None]
ClientOption = Callable[[dict], None]

_request_id_lock = threading.Lock()
_request_id_seq = 0


def _next_request_id() -> int:
    global _request_id_seq
    with _request_id_lock:
        _request_id_seq = (_request_id_seq + 1) & 0xFFFFF
        seq = _request_id_seq
    return (int(time.time() * 1000) << 20) | seq


# ---------------------------------------------------------------- Client 选项


def WithClientPollPeriod(period: float) -> ClientOption:
    def _apply(o: dict) -> None:
        o["poll_period"] = period

    return _apply


def WithClientPollTimeout(timeout: float) -> ClientOption:
    def _apply(o: dict) -> None:
        o["poll_timeout"] = timeout

    return _apply


# ---------------------------------------------------------------- Request 选项


def apply_request_options(opts: tuple) -> dict:
    o = {"ctx": None, "request_id": _next_request_id()}
    for fn in opts:
        fn(o)
    return o


def WithContext(ctx: Any) -> RequestOption:
    def _apply(o: dict) -> None:
        o["ctx"] = ctx

    return _apply


def WithRequestID(rid: int) -> RequestOption:
    def _apply(o: dict) -> None:
        o["request_id"] = rid

    return _apply


# ---------------------------------------------------------------- Wait 选项


def _apply_wait_options(client: "Client", opts: tuple) -> dict:
    o = {
        "ctx": None,
        "request_id": _next_request_id(),
        "poll_period": client.rpc.poll_period,
        "poll_timeout": client.rpc.poll_timeout,
    }
    for fn in opts:
        fn(o)
    return o


def WithWaitContext(ctx: Any) -> WaitOption:
    def _apply(o: dict) -> None:
        o["ctx"] = ctx

    return _apply


def WithWaitRequestID(rid: int) -> WaitOption:
    def _apply(o: dict) -> None:
        o["request_id"] = rid

    return _apply


def WithWaitPollPeriod(period: float) -> WaitOption:
    def _apply(o: dict) -> None:
        o["poll_period"] = period

    return _apply


def WithWaitPollTimeout(timeout: float) -> WaitOption:
    def _apply(o: dict) -> None:
        o["poll_timeout"] = timeout

    return _apply


# ---------------------------------------------------------------- Client


class Client:
    def __init__(self, rpc: RpcClientV1):
        self.rpc = rpc

    # ---------------------------------------------------------- 注册表
    def get_all_pd(self) -> dict:
        return self.rpc.get_all_pd()

    def get_provider_manager(self) -> Optional[IDLRegistry]:
        return self.rpc.get_provider_manager()

    # ---------------------------------------------------------- 账户
    def claim_faucet(self, account_sk: SecretKeyer, account: Address, mode: AccountSignatureMode) -> None:
        return self.rpc.claim_faucet(account_sk, account, mode)

    def create_account(self, account_sk: SecretKeyer, pk: Any) -> None:
        return self.rpc.create_account(account_sk, pk)

    def balance_of(self, account: Address) -> int:
        return self.rpc.balance_of(account)

    def list_account_signers(self, account: Address) -> list:
        return self.rpc.list_account_signers(account)

    def account_signer_bit(self, account: Address):
        return self.rpc.account_signer_bit(account)

    # ---------------------------------------------------------- 链
    def get_chain_head(self, *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.get_chain_head(o["request_id"])

    def submit_tx(self, transaction: Transaction, *opts: RequestOption) -> None:
        o = apply_request_options(opts)
        self.rpc.submit_tx(transaction, o["request_id"])

    def submit_tx_with_sponsor_ixes(self, tx: Transaction, sponsor_ixes: list[int], *opts: RequestOption) -> None:
        o = apply_request_options(opts)
        self.rpc.submit_tx_with_sponsor_ixes(tx, sponsor_ixes, o["request_id"])

    def simulate_tx(self, transaction: Transaction, *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.simulate_tx(transaction, o["request_id"])

    def view(self, wires: list[bytes], *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.view(wires, o["request_id"])

    def get_account(self, account_relaxed: Any, *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.get_account(account_relaxed, o["request_id"])

    def events_by_tx_hash(self, tx_hash_relaxed: Any, type_tag_filter: Optional[int], *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.events_by_tx_hash(tx_hash_relaxed, type_tag_filter, o["request_id"])

    def get_block_by_height(self, block_height: int, *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.get_block_by_height(block_height, o["request_id"])

    def get_tx_by_hash(self, tx_hash_or_tx_id_relaxed: Any, *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.get_tx_by_hash(tx_hash_or_tx_id_relaxed, o["request_id"])

    def get_tx_history_proof(self, tx_hash_or_tx_id_relaxed: Any, *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.get_tx_history_proof(tx_hash_or_tx_id_relaxed, o["request_id"])

    def get_resource(self, rs_hash: apibase.RsHash, *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.get_resource(rs_hash, o["request_id"])

    def get_resource_path_by_hash(self, rs_hash: apibase.RsHash, *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.get_resource_path_by_hash(rs_hash, o["request_id"])

    def batch_get_resource_path_by_hash(self, rs_hash_list: list[apibase.RsHash], *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.batch_get_resource_path_by_hash(rs_hash_list, o["request_id"])

    def get_access_value(self, blob_hash_list: list[apibase.BlobHash], *opts: RequestOption):
        o = apply_request_options(opts)
        return self.rpc.get_access_value(blob_hash_list, o["request_id"])

    def wait_for_transaction(self, tx_hash_or_tx_id_relaxed: Any, *opts: WaitOption):
        o = _apply_wait_options(self, opts)
        return self.rpc.wait_for_transaction(tx_hash_or_tx_id_relaxed, o)


def NewClient(config: Network, *options: ClientOption) -> Client:
    set_chain_id(config.chain_id)

    opts = {"poll_period": 1.0, "poll_timeout": 30.0}
    for fn in options:
        fn(opts)

    rpc = RpcClientV1(
        network=config,
        provider_by_idl_name={},
        poll_period=opts["poll_period"],
        poll_timeout=opts["poll_timeout"],
    )

    try:
        rpc.load_idls_from_data(gen.DefaultIDLs)
    except Exception as e:  # noqa: BLE001
        raise RuntimeError(f"failed to load generated IDLs: {e}")

    rpc.type_resolver = IDLTypeResolver(rpc.get_all_pd())

    try:
        rpc.provider_manager = NewIDLRegistry(rpc.get_all_pd())
    except Exception as e:  # noqa: BLE001
        raise RuntimeError(f"failed to build IDL registry: {e}")

    try:
        gen.bind_all(rpc.get_all_pd())
    except Exception as e:  # noqa: BLE001
        raise RuntimeError(f"failed to bind generated IDL apps: {e}")

    return Client(rpc)
