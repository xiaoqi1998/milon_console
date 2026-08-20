"""双线 RPC 客户端（对应 Go rpcClientV1.go）。

- JSON 线（Content-Type: application/x-milon+json）：method/request_id/body 以
  JSON 十进制整数数组发送；用于 ChainHead / View / 查询类方法。
- Postcard 线（Content-Type: application/x-milon+postcard）：RpcRequest 以
  自定义二进制协议发送；用于 SubmitTx / SimulateTx 等。
- 所有响应统一解码为 lib.RpcResponse，再按 method 解码业务体。
- WaitForTransaction：轮询 GetTxByHash 直至交易脱离 Pending 或超时。
"""
from __future__ import annotations

import inspect
import json
import threading
import time
import urllib.error
import urllib.request
from typing import Any, Optional

from .api import base as apibase
from .api import responses as apiresp
from .crypto import Address
from .crypto.errors import MilonCryptoError
from .lib import (
    CONTENT_TYPE_MILON_JSON,
    CONTENT_TYPE_MILON_POSTCARD,
    AccountSignatureMode,
    MethodType,
    PubKeySignatureMode,
    RpcRequest,
    RpcResponse,
    RpcResponseError,
    Signer,
    Transaction,
    TransactionBuilder,
    validate_wire,
    validate_wire_with,
)
from .lib.transaction_builder import SigningSlot
from .lib.chain import set_chain_id
from .postcard import Deserializer, Serializer

# 自包含的 request_id 生成器（rpc 便捷方法内部使用；client.py 另有独立实现，
# 因两模块相互引用不能共用）。高位为毫秒时间戳，低位为自增序号。
_request_id_lock = threading.Lock()
_request_id_seq = 0


def _next_request_id() -> int:
    global _request_id_seq
    with _request_id_lock:
        _request_id_seq = (_request_id_seq + 1) & 0xFFFFF
        seq = _request_id_seq
    return (int(time.time() * 1000) << 20) | seq


def _wait_opts(poll_period: float, poll_timeout: float) -> dict:
    """构造 WaitForTransaction 默认选项（便捷方法内部使用）。"""
    return {
        "request_id": _next_request_id(),
        "poll_period": poll_period,
        "poll_timeout": poll_timeout,
    }
from .provider import IDLRegistry, IDLTypeResolver, Provider
from . import gen

_HTTP_TIMEOUT = 30.0


# ---------------------------------------------------------------- HTTP 传输


def http_post_by_bytes(
    url: str,
    payload: bytes,
    headers: dict,
    timeout: float = _HTTP_TIMEOUT,
    retries: int = 3,
) -> tuple[Optional[int], Optional[bytes], Optional[Exception]]:
    """对应 Go tools.HttpPostByBytes：发送 POST，对 5xx 重试。

    返回 (status_code, body_bytes, error)。仅在传输层失败（连接错误）时 error 非空；
    HTTP 响应（含 5xx）均返回 status + body，由调用方判定。
    """
    last_err: Optional[Exception] = None
    for attempt in range(max(1, retries)):
        try:
            req = urllib.request.Request(url, data=payload, method="POST")
            for k, v in headers.items():
                req.add_header(k, v)
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                status = resp.status
                body = resp.read()
                if 500 <= status < 600 and attempt < retries - 1:
                    last_err = RuntimeError(f"5xx status {status}")
                    time.sleep(0.2 * (attempt + 1))
                    continue
                return status, body, None
        except urllib.error.HTTPError as e:  # 非 5xx 的 HTTP 错误也返回状态码
            if 500 <= e.code < 600 and attempt < retries - 1:
                last_err = e
                time.sleep(0.2 * (attempt + 1))
                continue
            return e.code, e.read(), None
        except Exception as e:  # 传输层错误
            return None, None, e
    return None, None, last_err


# ---------------------------------------------------------------- 结果类型
#
# 每个结果类同时暴露 Go 风格（HTTPResponseBody / BodyXxx）与 Python 风格
# （http_response_body / body_xxx）属性名，两套命名均可访问。


class ChainHeadResult:
    def __init__(self, http_body: bytes, chain_head: "apiresp.ChainHead"):
        self.HTTPResponseBody = http_body
        self.BodyChainHead = chain_head

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_chain_head(self) -> "apiresp.ChainHead":
        return self.BodyChainHead


class SimulateTxResult:
    def __init__(self, http_body: bytes, receipt: "apiresp.SimulateReceipt"):
        self.HTTPResponseBody = http_body
        self.BodySimulateReceipt = receipt

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_simulate_receipt(self) -> "apiresp.SimulateReceipt":
        return self.BodySimulateReceipt


class ViewResult:
    def __init__(self, http_body: bytes):
        self.HTTPResponseBody = http_body

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody


class GetResourceResult:
    def __init__(self, http_body: bytes, resource: "apiresp.GetResource"):
        self.HTTPResponseBody = http_body
        self.BodyGetResource = resource

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_get_resource(self) -> "apiresp.GetResource":
        return self.BodyGetResource


class GetBlockByHeightResult:
    def __init__(self, http_body: bytes, block: "apiresp.Block"):
        self.HTTPResponseBody = http_body
        self.BodyBlock = block

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_block(self) -> "apiresp.Block":
        return self.BodyBlock


class GetTxByHashResult:
    def __init__(self, http_body: bytes, tx_history: "apiresp.TxHistory"):
        self.HTTPResponseBody = http_body
        self.BodyTxHistory = tx_history

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_tx_history(self) -> "apiresp.TxHistory":
        return self.BodyTxHistory


class GetAccountResult:
    def __init__(self, http_body: bytes, account_view: "apiresp.AccountView"):
        self.HTTPResponseBody = http_body
        self.BodyAccountView = account_view

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_account_view(self) -> "apiresp.AccountView":
        return self.BodyAccountView


class EventsByTxHashResult:
    def __init__(self, http_body: bytes, events: "apiresp.EventsByTxHash"):
        self.HTTPResponseBody = http_body
        self.BodyEventsByTxHash = events

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_events_by_tx_hash(self) -> "apiresp.EventsByTxHash":
        return self.BodyEventsByTxHash


class GetResourcePathByHashResult:
    def __init__(self, http_body: bytes, path: str):
        self.HTTPResponseBody = http_body
        self.Path = path

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def path(self) -> str:
        return self.Path


class GetAccessValueResult:
    def __init__(self, http_body: bytes, values: list):
        self.HTTPResponseBody = http_body
        self.BodyGetAccessValues = values

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_get_access_values(self) -> list:
        return self.BodyGetAccessValues


class BatchGetResourcePathByHashResult:
    def __init__(self, http_body: bytes, infos: list):
        self.HTTPResponseBody = http_body
        self.BodyBatchResourcePathList = infos

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_batch_resource_path_list(self) -> list:
        return self.BodyBatchResourcePathList


class GetTxHistoryProofResult:
    def __init__(self, http_body: bytes, proof: "apiresp.GetTxHistoryProof"):
        self.HTTPResponseBody = http_body
        self.BodyGetTxHistoryProof = proof

    @property
    def http_response_body(self) -> bytes:
        return self.HTTPResponseBody

    @property
    def body_get_tx_history_proof(self) -> "apiresp.GetTxHistoryProof":
        return self.BodyGetTxHistoryProof


# ---------------------------------------------------------------- RPC 客户端


class RpcClientV1:
    def __init__(
        self,
        network: "Any",
        provider_by_idl_name: dict[str, Provider],
        poll_period: float = 1.0,
        poll_timeout: float = 30.0,
    ):
        self.network = network
        self.provider_by_idl_name = dict(provider_by_idl_name)
        self.provider_manager: Optional[IDLRegistry] = None
        self.type_resolver: Optional[IDLTypeResolver] = None
        self.poll_period = poll_period
        self.poll_timeout = poll_timeout

    # ---------------------------------------------------------- IDL 加载
    def load_idls_from_data(self, idls: list) -> None:
        if not idls:
            raise ValueError("empty IDL data")
        for idl in idls:
            if idl.metadata.name == "":
                raise ValueError("IDL metadata name is empty")
            self.provider_by_idl_name[idl.metadata.name] = Provider(idl)

    def get_all_pd(self) -> dict[str, Provider]:
        return dict(self.provider_by_idl_name)

    def get_provider_manager(self) -> Optional[IDLRegistry]:
        return self.provider_manager

    # ---------------------------------------------------------- JSON 线
    def _encode_json_rpc_request(self, method: int, request_id: int, body: bytes) -> bytes:
        envelope = {
            "method": int(method),
            "request_id": int(request_id),
            "body": list(body),
        }
        return json.dumps(envelope).encode("utf-8")

    def _parse_json_rpc_response(self, raw: bytes) -> RpcResponse:
        obj = json.loads(raw)
        rid = obj["request_id"]
        status = obj["status"]
        body = bytes(obj.get("body", []))
        err: Optional[RpcResponseError] = None
        raw_err = obj.get("error")
        if raw_err is not None:
            err = RpcResponseError(
                message=raw_err.get("message", ""),
                code=raw_err.get("code"),
                data=bytes(raw_err["data"]) if raw_err.get("data") is not None else None,
            )
        return RpcResponse(request_id=rid, status=status, body=body, error=err)

    def call_json_rpc(
        self,
        ctx: Any,
        method: MethodType,
        body: bytes,
        request_id: int,
    ) -> RpcResponse:
        payload = self._encode_json_rpc_request(method, request_id, body)
        status, resp_bytes, err = http_post_by_bytes(
            self.network.rpc_url, payload, {"Content-Type": CONTENT_TYPE_MILON_JSON}
        )
        if err is not None:
            raise RuntimeError(f"RPC call failed: {err}")
        if status != 200:
            raise RuntimeError(f"API returned error statusCode: {status}")
        api_response = self._parse_json_rpc_response(resp_bytes)
        if api_response.request_id != request_id:
            raise RuntimeError(
                f"response request_id {api_response.request_id} does not match request {request_id}"
            )
        if api_response.status != 0:
            if api_response.error is not None:
                raise RuntimeError(
                    f"API returned error status {api_response.status}: {api_response.error}"
                )
            raise RuntimeError(f"API returned error status: {api_response.status}")
        return api_response

    # ---------------------------------------------------------- Postcard 线
    def call_postcard_rpc(
        self,
        ctx: Any,
        method: MethodType,
        body: bytes,
        request_id: int,
    ) -> RpcResponse:
        rpc_req = RpcRequest(method, request_id, body)
        serializer = Serializer()
        rpc_req.marshal_postcard(serializer)
        payload = serializer.bytes()

        status, resp_bytes, err = http_post_by_bytes(
            self.network.rpc_url, payload, {"Content-Type": CONTENT_TYPE_MILON_POSTCARD}
        )
        if err is not None:
            raise RuntimeError(f"RPC call failed: {err}")
        if status != 200:
            raise RuntimeError(f"API returned error statusCode: {status}")

        api_response = self._decode_postcard_body(resp_bytes, RpcResponse, self.type_resolver)
        if api_response.request_id != request_id:
            raise RuntimeError(
                f"response request_id {api_response.request_id} does not match request {request_id}"
            )
        if api_response.status != 0:
            if api_response.error is not None:
                raise RuntimeError(
                    f"API returned error status {api_response.status}: {api_response.error}"
                )
            raise RuntimeError(f"API returned error status: {api_response.status}")
        return api_response

    def _decode_postcard_body(self, body: bytes, cls: type, resolver: Any) -> Any:
        d = Deserializer(body)
        d.type_resolver = resolver
        value = self._unmarshal(cls, d, resolver)
        d.assert_end()
        return value

    @staticmethod
    def _unmarshal(cls: type, d: Deserializer, resolver: Any) -> Any:
        # 注意：这些是 @classmethod，inspect.signature 会剔除 cls，
        # 因此签名实际为 (d, resolver=None)（2 个参数），不能用 >= 3 判断。
        # 直接检查是否接受 resolver 形参，避免把 resolver 静默丢弃。
        sig = inspect.signature(cls.unmarshal_postcard)
        if "resolver" in sig.parameters:
            return cls.unmarshal_postcard(d, resolver)
        return cls.unmarshal_postcard(d)

    # ---------------------------------------------------------- 账户类
    def claim_faucet(self, account_sk: Any, account: Address, mode: AccountSignatureMode) -> None:
        wire = gen.Token.ClaimFaucet.Args(account).Encode()
        builder = TransactionBuilder([wire])
        builder.apply_slots(
            [SigningSlot(address=account, instruction_indices=[0], include_payer=False, mode=mode)]
        )
        builder.sign_with(Signer(secret_key=account_sk, public_key=mode.public_key))
        tx = builder.build()
        self.submit_tx_with_sponsor_ixes(tx, [0], _next_request_id())
        self.wait_for_transaction(tx.tx_hash(), _wait_opts(self.poll_period, self.poll_timeout))

    def create_account(self, account_sk: Any, pk: Any) -> None:
        wire = gen.Account.Create.Args(pk).Encode()
        account = Address.from_public_key(pk)
        builder = TransactionBuilder([wire])
        builder.with_payer(account)
        builder.apply_slots(
            [SigningSlot(address=account, instruction_indices=[], include_payer=True, mode=PubKeySignatureMode(public_key=pk))]
        )
        builder.sign_with(Signer(secret_key=account_sk, public_key=pk))
        tx = builder.build()
        self.submit_tx(tx, _next_request_id())
        self.wait_for_transaction(tx.tx_hash(), _wait_opts(self.poll_period, self.poll_timeout))

    def balance_of(self, account: Address) -> int:
        wire = gen.Token.BalanceOf.Args(apibase.MIL_TOKEN, account).Encode()
        view_result = self.view([wire], _next_request_id())
        return gen.Token.BalanceOf.DecodeView(view_result.HTTPResponseBody)

    def list_account_signers(self, account: Address) -> list:
        wire = gen.Account.ListSigners.Args(account).Encode()
        view_result = self.view([wire], _next_request_id())
        return gen.Account.ListSigners.DecodeView(view_result.HTTPResponseBody)

    def account_signer_bit(self, account: Address) -> Any:
        from .types import Bitmap64

        list_signers = self.list_account_signers(account)
        if len(list_signers) < 2:
            raise ValueError(f"unexpected ListSigners result: {list_signers}")
        account_map = list_signers[0]
        if not isinstance(account_map, dict):
            raise ValueError(f"unexpected account data: {account_map}")
        signers = list_signers[1]
        if not isinstance(signers, list):
            raise ValueError(f"unexpected signers list: {signers}")
        if len(signers) != 1:
            raise ValueError(
                f"account {account} has {len(signers)} signers; AccountSignerBit supports single-signer accounts only"
            )
        bm = account_map.get("bitmap")
        if bm is None or bm == 0:
            raise ValueError(f"unexpected account bitmap: {account_map.get('bitmap')}")
        lowest = bm & -bm
        if bm != lowest:
            raise ValueError(
                f"account {account} bitmap {bm:#x} has multiple signer slots; use multisig signing instead"
            )
        return Bitmap64(lowest)

    # ---------------------------------------------------------- 链查询
    def get_chain_head(self, request_id: int) -> ChainHeadResult:
        api_response = self.call_json_rpc(None, MethodType.CHAIN_HEAD, b"", request_id)
        chain_head = self._decode_postcard_body(api_response.body, apiresp.ChainHead, self.type_resolver)
        return ChainHeadResult(api_response.body, chain_head)

    def submit_tx(self, transaction: Transaction, request_id: int) -> None:
        validate_wire(transaction)
        tx_postcard = transaction.to_bytes()
        self.call_postcard_rpc(None, MethodType.SUBMIT_TX, tx_postcard, request_id)

    def submit_tx_with_sponsor_ixes(
        self, transaction: Transaction, sponsor_ixes: list[int], request_id: int
    ) -> None:
        validate_wire_with(transaction, sponsor_ixes)
        tx_postcard = transaction.to_bytes()
        self.call_postcard_rpc(None, MethodType.SUBMIT_TX, tx_postcard, request_id)

    def simulate_tx(self, transaction: Transaction, request_id: int) -> SimulateTxResult:
        tx_postcard = transaction.to_bytes()
        http_response = self.call_postcard_rpc(None, MethodType.SIMULATE_TX, tx_postcard, request_id)
        receipt = self._decode_postcard_body(http_response.body, apiresp.SimulateReceipt, self.type_resolver)
        return SimulateTxResult(http_response.body, receipt)

    def view(self, wires: list[bytes], request_id: int) -> ViewResult:
        serializer = Serializer()
        serializer.serialize_u32(len(wires))
        for w in wires:
            serializer.serialize_bytes(w)
        api_response = self.call_json_rpc(None, MethodType.VIEW, serializer.bytes(), request_id)
        return ViewResult(api_response.body)

    def get_account(self, account_relaxed: Any, request_id: int) -> GetAccountResult:
        account = Address.from_relaxed(account_relaxed)
        serializer = Serializer()
        account.marshal_postcard(serializer)
        api_response = self.call_json_rpc(None, MethodType.GET_ACCOUNT, serializer.bytes(), request_id)
        account_view = self._decode_postcard_body(api_response.body, apiresp.AccountView, self.type_resolver)
        return GetAccountResult(api_response.body, account_view)

    def events_by_tx_hash(
        self, tx_hash_relaxed: Any, type_tag_filter: Optional[int], request_id: int
    ) -> EventsByTxHashResult:
        tx_hash = apibase.new_tx_hash_from_relaxed(tx_hash_relaxed)
        serializer = Serializer()
        req = apiresp.EventsByTxHashReq(tx_hash=tx_hash, type_tag_filter=type_tag_filter)
        req.marshal_postcard(serializer)
        api_response = self.call_json_rpc(None, MethodType.EVENTS_BY_TX_HASH, serializer.bytes(), request_id)
        events = self._decode_postcard_body(api_response.body, apiresp.EventsByTxHash, self.type_resolver)
        return EventsByTxHashResult(api_response.body, events)

    def get_block_by_height(self, block_height: int, request_id: int) -> GetBlockByHeightResult:
        serializer = Serializer()
        serializer.serialize_u64(block_height)
        api_response = self.call_json_rpc(None, MethodType.GET_BLOCK_BY_HEIGHT, serializer.bytes(), request_id)
        block = self._decode_postcard_body(api_response.body, apiresp.Block, self.type_resolver)
        return GetBlockByHeightResult(api_response.body, block)

    def get_tx_by_hash(self, tx_hash_or_tx_id_relaxed: Any, request_id: int) -> GetTxByHashResult:
        tx_hash_or_tx_id = apibase.new_tx_hash_or_tx_id_from_relaxed(tx_hash_or_tx_id_relaxed)
        serializer = Serializer()
        serializer.serialize_bytes(tx_hash_or_tx_id)
        api_response = self.call_json_rpc(None, MethodType.GET_TX_BY_HASH, serializer.bytes(), request_id)
        tx_history = self._decode_postcard_body(api_response.body, apiresp.TxHistory, self.type_resolver)
        return GetTxByHashResult(api_response.body, tx_history)

    def get_tx_history_proof(self, tx_hash_or_tx_id_relaxed: Any, request_id: int) -> GetTxHistoryProofResult:
        tx_hash_or_tx_id = apibase.new_tx_hash_or_tx_id_from_relaxed(tx_hash_or_tx_id_relaxed)
        serializer = Serializer()
        serializer.serialize_bytes(tx_hash_or_tx_id)
        api_response = self.call_json_rpc(None, MethodType.GET_TX_HISTORY_PROOF, serializer.bytes(), request_id)
        proof = self._decode_postcard_body(api_response.body, apiresp.GetTxHistoryProof, self.type_resolver)
        return GetTxHistoryProofResult(api_response.body, proof)

    def get_resource(self, rs_hash: apibase.RsHash, request_id: int) -> GetResourceResult:
        serializer = Serializer()
        serializer.write(rs_hash.as_bytes())
        api_response = self.call_json_rpc(None, MethodType.GET_RESOURCE, serializer.bytes(), request_id)
        resource = self._decode_postcard_body(api_response.body, apiresp.GetResource, self.type_resolver)
        return GetResourceResult(api_response.body, resource)

    def get_resource_path_by_hash(self, rs_hash: apibase.RsHash, request_id: int) -> GetResourcePathByHashResult:
        api_response = self.call_json_rpc(None, MethodType.GET_RESOURCE_PATH_BY_HASH, rs_hash.as_bytes(), request_id)
        path = json.loads(api_response.body.decode("utf-8"))
        return GetResourcePathByHashResult(api_response.body, path)

    def batch_get_resource_path_by_hash(
        self, rs_hash_list: list[apibase.RsHash], request_id: int
    ) -> BatchGetResourcePathByHashResult:
        serializer = Serializer()
        serializer.serialize_seq(rs_hash_list, lambda s, h: s.write(h.as_bytes()))
        api_response = self.call_json_rpc(
            None, MethodType.BATCH_GET_RESOURCE_PATH_BY_HASH, serializer.bytes(), request_id
        )
        raw_list = json.loads(api_response.body.decode("utf-8"))
        infos = apiresp.unmarshal_batch_resource_path_list_from_raw_list(raw_list)
        return BatchGetResourcePathByHashResult(api_response.body, infos)

    def get_access_value(self, blob_hash_list: list[apibase.BlobHash], request_id: int) -> GetAccessValueResult:
        serializer = Serializer()
        serializer.serialize_seq(blob_hash_list, lambda s, h: s.write(h.as_bytes()))
        api_response = self.call_json_rpc(None, MethodType.GET_ACCESS_VALUE, serializer.bytes(), request_id)
        deserializer = Deserializer(api_response.body)
        access_values = deserializer.deserialize_seq(lambda d: apiresp.GetAccessValueInfo.unmarshal_postcard(d, self.type_resolver))
        return GetAccessValueResult(api_response.body, access_values)

    # ---------------------------------------------------------- 等待交易
    def wait_for_transaction(self, tx_hash_or_tx_id_relaxed: Any, wait_opts: dict) -> GetTxByHashResult:
        poll_period = wait_opts.get("poll_period", self.poll_period)
        poll_timeout = wait_opts.get("poll_timeout", self.poll_timeout)
        request_id = wait_opts.get("request_id")
        ctx = wait_opts.get("ctx")

        if poll_period <= 0:
            raise ValueError(f"WaitForTransaction: invalid poll period {poll_period}, must be positive")

        tx_hash_or_tx_id = apibase.new_tx_hash_or_tx_id_from_relaxed(tx_hash_or_tx_id_relaxed)
        deadline = time.time() + poll_timeout
        last_err: Optional[Exception] = None

        while True:
            if time.time() > deadline:
                if last_err is not None:
                    raise RuntimeError(
                        f"WaitForTransaction timeout after {poll_timeout}s, last error: {last_err}"
                    )
                raise RuntimeError(f"WaitForTransaction timeout after {poll_timeout}s")

            try:
                result = self.get_tx_by_hash(tx_hash_or_tx_id, request_id)
            except Exception as e:  # noqa: BLE001
                last_err = e
                result = None

            if result is not None and result.BodyTxHistory.receipt.state != apiresp.TX_STATE_PENDING:
                return result

            time.sleep(poll_period)
