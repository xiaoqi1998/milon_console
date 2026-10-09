"""rpcClientV1: HTTP JSON/postcard RPC 客户端. Faithful port of gosdk-develop/rpcClientV1.go."""

from __future__ import annotations

import itertools
import json
import time
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

import requests

from . import gen
from .api import (
    TX_STATE_PENDING,
    TX_STATE_SUCCESS,
    AccountView,
    Block,
    BlobHash,
    ChainHead,
    EventsByTxHash,
    EventsByTxHashReq,
    GetAccessValueInfo,
    GetResource,
    GetTxHistoryProof,
    MIL_TOKEN,
    PackedInstruction,
    RsHash,
    TxHistory,
    new_tx_hash_from_relaxed,
    new_tx_hash_or_tx_id_from_relaxed,
    unmarshal_batch_resource_path_list_from_raw_list,
)
from .crypto import (
    Address,
    SecretKeyer,
    new_address_from_public_key,
    new_address_from_relaxed,
)
from .lib import (
    CONTENT_TYPE_MILON_JSON,
    CONTENT_TYPE_MILON_POSTCARD,
    METHOD_TYPE_BATCH_GET_RESOURCE_PATH_BY_HASH,
    METHOD_TYPE_CHAIN_HEAD,
    METHOD_TYPE_EVENTS_BY_TX_HASH,
    METHOD_TYPE_GET_ACCESS_VALUE,
    METHOD_TYPE_GET_ACCOUNT,
    METHOD_TYPE_GET_BLOCK_BY_HEIGHT,
    METHOD_TYPE_GET_RESOURCE,
    METHOD_TYPE_GET_RESOURCE_PATH_BY_HASH,
    METHOD_TYPE_GET_TX_BY_HASH,
    METHOD_TYPE_GET_TX_HISTORY_PROOF,
    METHOD_TYPE_SIMULATE_TX,
    METHOD_TYPE_SUBMIT_TX,
    METHOD_TYPE_VIEW,
    RPC_RESPONSE_STATUS_OK,
    PubKeySignatureMode,
    RpcRequest,
    RpcResponse,
    Transaction,
    new_rpc_request,
    new_transaction_builder,
)
from .network import Network
from .postcard import (
    Deserializer,
    Serializer,
    TypeResolver,
    deserialize_postcard_with_resolver,
    deserialize_seq,
    serialize_seq,
)
from .provider import IDL, IDLRegistry, IDLTypeResolver, Provider, new_idl_registry, new_provider
from .types.bitbap import Bitmap64


@dataclass
class ChainHeadResult:
    HTTPResponseBody: bytes = b""
    BodyChainHead: Optional[ChainHead] = None


@dataclass
class SimulateTxResult:
    HTTPResponseBody: bytes = b""
    BodySimulateReceipt: Any = None


@dataclass
class ViewResult:
    HTTPResponseBody: bytes = b""


@dataclass
class GetResourceResult:
    HTTPResponseBody: bytes = b""
    BodyGetResource: Optional[GetResource] = None


@dataclass
class GetBlockByHeightResult:
    HTTPResponseBody: bytes = b""
    BodyBlock: Optional[Block] = None


@dataclass
class GetTxByHashResult:
    HTTPResponseBody: bytes = b""
    BodyTxHistory: Optional[TxHistory] = None


@dataclass
class GetAccountResult:
    HTTPResponseBody: bytes = b""
    BodyAccountView: Optional[AccountView] = None


@dataclass
class EventsByTxHashResult:
    HTTPResponseBody: bytes = b""
    BodyEventsByTxHash: Optional[EventsByTxHash] = None


@dataclass
class GetResourcePathByHashResult:
    HTTPResponseBody: bytes = b""
    Path: str = ""


@dataclass
class GetAccessValueResult:
    HTTPResponseBody: bytes = b""
    BodyGetAccessValues: List[GetAccessValueInfo] = field(default_factory=list)


@dataclass
class BatchGetResourcePathByHashResult:
    HTTPResponseBody: bytes = b""
    BodyBatchResourcePathList: list = field(default_factory=list)


@dataclass
class GetTxHistoryProofResult:
    HTTPResponseBody: bytes = b""
    BodyGetTxHistoryProof: Optional[GetTxHistoryProof] = None


class RpcClientV1:
    def __init__(
        self,
        network: Network,
        poll_period: float = 1.0,
        poll_timeout: float = 30.0,
    ) -> None:
        self.network = network
        self.providerByIDLName: Dict[str, Provider] = {}
        self.providerManager: Optional[IDLRegistry] = None
        self.typeResolver: Optional[TypeResolver] = None
        self.pollPeriod = poll_period
        self.pollTimeout = poll_timeout

    # ---- IDL loading ----

    def load_idls_from_data(self, idls: Dict[str, IDL]) -> None:
        """Go: LoadIDLsFromData(idls []provider.IDL)（Python 侧以名字索引）。"""
        if not idls:
            raise ValueError("empty IDL data")
        for name, idl in idls.items():
            if idl.Metadata.Name == "":
                raise ValueError("IDL metadata name is empty")
            self.providerByIDLName[idl.Metadata.Name] = new_provider(idl)

    def get_all_pd(self) -> Dict[str, Provider]:
        return dict(self.providerByIDLName)

    def get_provider_manager(self) -> IDLRegistry:
        return self.providerManager

    # ---- transport ----

    def _call_json_rpc(self, method: int, body: bytes, request_id: int) -> RpcResponse:
        payload = (
            '{"method":'
            + str(int(method))
            + ',"request_id":'
            + str(request_id)
            + ',"body":['
            + ",".join(str(b) for b in body)
            + "]}"
        ).encode()
        try:
            resp = requests.post(
                self.network.RpcUrl,
                data=payload,
                headers={"Content-Type": CONTENT_TYPE_MILON_JSON},
                timeout=max(self.pollTimeout, 10.0),
            )
        except requests.RequestException as exc:
            raise ValueError(f"RPC call failed: {exc}") from exc
        if resp.status_code != 200:
            raise ValueError(f"API returned error statusCode: {resp.status_code}")

        try:
            data = resp.json()
        except ValueError as exc:
            raise ValueError(f"failed to unmarshal API response: {exc}") from exc
        api_response = RpcResponse(
            RequestId=data.get("request_id", 0),
            Status=data.get("status", 0),
            Body=bytes.fromhex(data["body"]) if isinstance(data.get("body"), str) else bytes(data.get("body") or []),
            Error=None,
        )
        if api_response.RequestId != request_id:
            raise ValueError(
                f"response request_id {api_response.RequestId} does not match "
                f"request {request_id}"
            )
        if api_response.Status != RPC_RESPONSE_STATUS_OK:
            err = data.get("error")
            if err:
                raise ValueError(
                    f"API returned error status {api_response.Status}: {err}"
                )
            raise ValueError(f"API returned error status: {api_response.Status}")
        return api_response

    def _call_postcard_rpc(self, method: int, body: bytes, request_id: int) -> RpcResponse:
        rpc_req = new_rpc_request(method, request_id, body)
        serializer = Serializer()
        rpc_req.marshal_postcard(serializer)

        try:
            resp = requests.post(
                self.network.RpcUrl,
                data=serializer.bytes(),
                headers={"Content-Type": CONTENT_TYPE_MILON_POSTCARD},
                timeout=max(self.pollTimeout, 10.0),
            )
        except requests.RequestException as exc:
            raise ValueError(f"RPC call failed: {exc}") from exc
        if resp.status_code != 200:
            raise ValueError(f"API returned error statusCode: {resp.status_code}")

        http_response = self._decode_postcard_body(resp.content, "API response", RpcResponse)
        if http_response.RequestId != request_id:
            raise ValueError(
                f"response request_id {http_response.RequestId} does not match "
                f"request {request_id}"
            )
        if http_response.Status != RPC_RESPONSE_STATUS_OK:
            if http_response.Error is not None:
                raise ValueError(
                    f"API returned error status {http_response.Status}: "
                    f"{http_response.Error.to_json_value() if hasattr(http_response.Error, 'to_json_value') else http_response.Error}"
                )
            raise ValueError(f"API returned error status: {http_response.Status}")
        return http_response

    def _decode_postcard_body(self, body: bytes, name: str, cls):
        from .postcard import Deserializer as _D

        def _fn(d: Deserializer):
            value = cls()
            value.unmarshal_postcard(d)
            return value

        try:
            return deserialize_postcard_with_resolver(body, _fn, False, self.typeResolver)
        except ValueError as exc:
            raise ValueError(f"failed to deserialize {name}: {exc}") from exc

    # ---- 高层 API ----

    def claim_faucet(self, account_sk: SecretKeyer, account: Address, mode) -> None:
        # 1. Encode instruction
        wire = gen.TOKEN.CLAIM_FAUCET.args(claimer=account).encode()

        # 2. SplitPayerSelfPay mode
        tx = new_transaction_builder([wire]).add_ixes_sig(account, account_sk, [0], False, mode).build()

        # 3. Submit
        self.submit_tx_with_sponsor_ixes(tx, [0])

        # 4. Wait
        self.wait_for_transaction(tx.tx_hash())

    def create_account(self, account_sk: SecretKeyer, pk) -> None:
        wire = gen.ACCOUNT.CREATE.args(owner_pk=pk).encode()
        account = new_address_from_public_key(pk)

        # UnifiedPayerGasOnly mode
        tx = (
            new_transaction_builder([wire])
            .with_payer(account)
            .add_payer_sig(account, account_sk, PubKeySignatureMode(PublicKey=pk))
            .build()
        )

        self.submit_tx(tx)
        self.wait_for_transaction(tx.tx_hash())

    def balance_of(self, account: Address, token: Any = None) -> int:
        wire = gen.TOKEN.BALANCE_OF.args(token=MIL_TOKEN, account=account).encode()
        view_tx_result = self.view([wire])
        return gen.TOKEN.BALANCE_OF.decode_view(view_tx_result.HTTPResponseBody)

    def list_account_signers(self, account: Address) -> List[Any]:
        wire = gen.ACCOUNT.LIST_SIGNERS.args(owner=account).encode()
        view_result = self.view([wire])
        out = gen.ACCOUNT.LIST_SIGNERS.decode_view(view_result.HTTPResponseBody)
        if len(out) < 2:
            raise ValueError(f"unexpected ListSigners result: {out}")
        return out

    def account_signer_bit(self, account: Address) -> Bitmap64:
        try:
            list_signers = self.list_account_signers(account)
        except ValueError as exc:
            raise ValueError(f"failed to get account list signers: {exc}") from exc

        if not list_signers or not isinstance(list_signers[0], dict):
            raise ValueError(f"unexpected account data: {list_signers[0] if list_signers else None}")
        account_map = list_signers[0]
        signers = list_signers[1] if len(list_signers) > 1 else None
        if not isinstance(signers, list):
            raise ValueError(f"unexpected signers list: {signers}")
        if len(signers) != 1:
            raise ValueError(
                f"account {account} has {len(signers)} signers; "
                f"AccountSignerBit supports single-signer accounts only"
            )

        bm = account_map.get("bitmap")
        if not isinstance(bm, int) or bm == 0:
            raise ValueError(f"unexpected account bitmap: {account_map.get('bitmap')}")
        # A single-signer account's bitmap has exactly one bit set.
        lowest = bm & -bm
        if bm != lowest:
            raise ValueError(
                f"account {account} bitmap {bm:#x} has multiple signer slots; "
                f"use multisig signing instead"
            )
        return Bitmap64(lowest)

    def token_metadata(self, token: Address):
        wire = gen.TOKEN.METADATA.args(token=token).encode()
        view_result = self.view([wire])
        return gen.TOKEN.METADATA.decode_view(view_result.HTTPResponseBody)

    def get_chain_head(self, request_id: Optional[int] = None) -> ChainHeadResult:
        api_response = self._call_json_rpc(METHOD_TYPE_CHAIN_HEAD, b"", self._next_id(request_id))
        chain_head = self._decode_postcard_body(api_response.Body, "ChainHead", ChainHead)
        return ChainHeadResult(
            HTTPResponseBody=api_response.Body, BodyChainHead=chain_head
        )

    def submit_tx(self, tx: Transaction, request_id: Optional[int] = None) -> None:
        try:
            tx.validate_wire()
        except ValueError as exc:
            raise ValueError(f"transaction validation failed: {exc}") from exc
        tx_postcard = tx.to_bytes()
        self._call_postcard_rpc(METHOD_TYPE_SUBMIT_TX, tx_postcard, self._next_id(request_id))

    def submit_tx_with_sponsor_ixes(
        self, tx: Transaction, sponsor_ixes, request_id: Optional[int] = None
    ) -> None:
        try:
            tx.validate_wire_with(sponsor_ixes)
        except ValueError as exc:
            raise ValueError(f"transaction validation failed: {exc}") from exc
        tx_postcard = tx.to_bytes()
        self._call_postcard_rpc(METHOD_TYPE_SUBMIT_TX, tx_postcard, self._next_id(request_id))

    def simulate_tx(self, transaction: Transaction, request_id: Optional[int] = None) -> SimulateTxResult:
        from .api import SimulateReceipt

        tx_postcard = transaction.to_bytes()
        http_response = self._call_postcard_rpc(METHOD_TYPE_SIMULATE_TX, tx_postcard, self._next_id(request_id))
        simulate_receipt = self._decode_postcard_body(http_response.Body, "SimulateReceipt", SimulateReceipt)
        return SimulateTxResult(
            HTTPResponseBody=http_response.Body,
            BodySimulateReceipt=simulate_receipt,
        )

    def view(self, wires: List[PackedInstruction], request_id: Optional[int] = None) -> ViewResult:
        serializer = Serializer()
        serializer.serialize_u32(len(wires))
        for w in wires:
            serializer.serialize_bytes(w)
        api_response = self._call_json_rpc(METHOD_TYPE_VIEW, serializer.bytes(), self._next_id(request_id))
        return ViewResult(HTTPResponseBody=api_response.Body)

    def get_account(self, account_relaxed: Any, request_id: Optional[int] = None) -> GetAccountResult:
        try:
            account = new_address_from_relaxed(account_relaxed)
        except ValueError as exc:
            raise ValueError(f"failed to decode accountRelaxed: {exc}") from exc
        serializer = Serializer()
        account.marshal_postcard(serializer)
        api_response = self._call_json_rpc(METHOD_TYPE_GET_ACCOUNT, serializer.bytes(), self._next_id(request_id))
        account_view = self._decode_postcard_body(api_response.Body, "AccountView", AccountView)
        return GetAccountResult(
            HTTPResponseBody=api_response.Body, BodyAccountView=account_view
        )

    def events_by_tx_hash(self, tx_hash_relaxed: Any, type_tag_filter: Optional[int] = None, request_id: Optional[int] = None) -> EventsByTxHashResult:
        try:
            tx_hash = new_tx_hash_from_relaxed(tx_hash_relaxed)
        except ValueError as exc:
            raise ValueError(f"failed to parse txHash: {exc}") from exc
        serializer = Serializer()
        req = EventsByTxHashReq(TxHash=tx_hash, TypeTagFilter=type_tag_filter)
        req.marshal_postcard(serializer)
        api_response = self._call_json_rpc(METHOD_TYPE_EVENTS_BY_TX_HASH, serializer.bytes(), self._next_id(request_id))
        events = self._decode_postcard_body(api_response.Body, "EventsByTxHash", EventsByTxHash)
        return EventsByTxHashResult(
            HTTPResponseBody=api_response.Body, BodyEventsByTxHash=events
        )

    def get_block_by_height(self, block_height: int, request_id: Optional[int] = None) -> GetBlockByHeightResult:
        serializer = Serializer()
        serializer.serialize_u64(block_height)
        api_response = self._call_json_rpc(METHOD_TYPE_GET_BLOCK_BY_HEIGHT, serializer.bytes(), self._next_id(request_id))
        block = self._decode_postcard_body(api_response.Body, "Block", Block)
        return GetBlockByHeightResult(HTTPResponseBody=api_response.Body, BodyBlock=block)

    def get_tx_by_hash(self, tx_hash_or_tx_id_relaxed: Any, request_id: Optional[int] = None) -> GetTxByHashResult:
        try:
            tx_hash_or_tx_id = new_tx_hash_or_tx_id_from_relaxed(tx_hash_or_tx_id_relaxed)
        except ValueError as exc:
            raise ValueError(f"failed to parse txHashOrTxId: {exc}") from exc
        serializer = Serializer()
        serializer.serialize_bytes(tx_hash_or_tx_id)
        api_response = self._call_json_rpc(METHOD_TYPE_GET_TX_BY_HASH, serializer.bytes(), self._next_id(request_id))
        tx_history = self._decode_postcard_body(api_response.Body, "TxHistory", TxHistory)
        return GetTxByHashResult(HTTPResponseBody=api_response.Body, BodyTxHistory=tx_history)

    def get_tx_history_proof(self, tx_hash_or_tx_id_relaxed: Any, request_id: Optional[int] = None) -> GetTxHistoryProofResult:
        try:
            tx_hash_or_tx_id = new_tx_hash_or_tx_id_from_relaxed(tx_hash_or_tx_id_relaxed)
        except ValueError as exc:
            raise ValueError(f"failed to parse txHashOrTxId: {exc}") from exc
        serializer = Serializer()
        serializer.serialize_bytes(tx_hash_or_tx_id)
        api_response = self._call_json_rpc(METHOD_TYPE_GET_TX_HISTORY_PROOF, serializer.bytes(), self._next_id(request_id))
        proof = self._decode_postcard_body(api_response.Body, "GetTxHistoryProof", GetTxHistoryProof)
        return GetTxHistoryProofResult(HTTPResponseBody=api_response.Body, BodyGetTxHistoryProof=proof)

    def get_resource(self, rs_hash: RsHash, request_id: Optional[int] = None) -> GetResourceResult:
        serializer = Serializer()
        serializer.serialize_fixed_bytes(rs_hash)
        api_response = self._call_json_rpc(METHOD_TYPE_GET_RESOURCE, serializer.bytes(), self._next_id(request_id))
        get_resource = self._decode_postcard_body(api_response.Body, "GetResource", GetResource)
        return GetResourceResult(HTTPResponseBody=api_response.Body, BodyGetResource=get_resource)

    def get_resource_path_by_hash(self, rs_hash: RsHash, request_id: Optional[int] = None) -> GetResourcePathByHashResult:
        api_response = self._call_json_rpc(METHOD_TYPE_GET_RESOURCE_PATH_BY_HASH, rs_hash, self._next_id(request_id))
        try:
            path = json.loads(api_response.Body.decode("utf-8"))
        except (ValueError, UnicodeDecodeError) as exc:
            raise ValueError(
                f"failed to unmarshal GetResourcePathByHash body: {exc}"
            ) from exc
        return GetResourcePathByHashResult(HTTPResponseBody=api_response.Body, Path=path)

    def batch_get_resource_path_by_hash(self, rs_hash_list: List[RsHash], request_id: Optional[int] = None) -> BatchGetResourcePathByHashResult:
        serializer = Serializer()
        serialize_seq(serializer, rs_hash_list, lambda s, h: s.serialize_fixed_bytes(h))
        api_response = self._call_json_rpc(METHOD_TYPE_BATCH_GET_RESOURCE_PATH_BY_HASH, serializer.bytes(), self._next_id(request_id))
        try:
            raw_list = json.loads(api_response.Body.decode("utf-8"))
        except (ValueError, UnicodeDecodeError) as exc:
            raise ValueError(
                f"failed to unmarshal BatchGetResourcePathByHash body: {exc}"
            ) from exc
        body = unmarshal_batch_resource_path_list_from_raw_list(raw_list)
        return BatchGetResourcePathByHashResult(HTTPResponseBody=api_response.Body, BodyBatchResourcePathList=body)

    def get_access_value(self, blob_hash_list: List[BlobHash], request_id: Optional[int] = None) -> GetAccessValueResult:
        serializer = Serializer()
        serialize_seq(serializer, blob_hash_list, lambda s, h: s.serialize_fixed_bytes(h))
        api_response = self._call_json_rpc(METHOD_TYPE_GET_ACCESS_VALUE, serializer.bytes(), self._next_id(request_id))

        deserializer = Deserializer(api_response.Body)

        def _read(d: Deserializer) -> GetAccessValueInfo:
            info = GetAccessValueInfo()
            try:
                info.unmarshal_postcard(d)
            except ValueError as exc:
                raise ValueError(
                    f"failed to deserialize GetAccessValueInfo: {exc}"
                ) from exc
            return info

        try:
            access_values = deserialize_seq(deserializer, _read)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize GetAccessValue sequence: {exc}"
            ) from exc

        return GetAccessValueResult(HTTPResponseBody=api_response.Body, BodyGetAccessValues=access_values)

    def wait_for_transaction(
        self, tx_hash_or_tx_id_relaxed: Any, poll_period: Optional[float] = None, poll_timeout: Optional[float] = None
    ) -> GetTxByHashResult:
        period = poll_period or self.pollPeriod
        timeout = poll_timeout or self.pollTimeout
        if period <= 0:
            raise ValueError(f"WaitForTransaction: invalid poll period {period}, must be positive")

        try:
            tx_hash_or_tx_id = new_tx_hash_or_tx_id_from_relaxed(tx_hash_or_tx_id_relaxed)
        except ValueError as exc:
            raise ValueError(f"failed to decode txHash: {exc}") from exc

        deadline = time.time() + timeout
        last_err: Optional[ValueError] = None

        while True:
            if time.time() > deadline:
                if last_err is not None:
                    raise ValueError(
                        f"WaitForTransaction timeout after {timeout}s, last "
                        f"error: {last_err}"
                    ) from last_err
                raise ValueError(f"WaitForTransaction timeout after {timeout}s")

            try:
                result = self.get_tx_by_hash(tx_hash_or_tx_id)
                if result.BodyTxHistory.Receipt.State != TX_STATE_PENDING:
                    return result
            except ValueError as exc:
                last_err = exc
            time.sleep(period)

    # request id 自增（Go: requestIDSeq atomic）
    _request_id_seq = itertools.count(1)

    def _next_id(self, explicit: Optional[int]) -> int:
        if explicit is not None:
            return explicit
        return next(RpcClientV1._request_id_seq)
