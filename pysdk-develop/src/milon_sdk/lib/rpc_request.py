"""RpcRequest. Faithful port of gosdk-develop/lib/rpcRequest.go."""

from __future__ import annotations

from dataclasses import dataclass
from enum import IntEnum

from ..postcard import Deserializer, Serializer

CONTENT_TYPE_MILON_POSTCARD = "application/x-milon+postcard"
CONTENT_TYPE_MILON_JSON = "application/x-milon+json"

METHOD_TYPE_CHAIN_HEAD = 1
METHOD_TYPE_SUBMIT_TX = 5
METHOD_TYPE_SIMULATE_TX = 10
METHOD_TYPE_VIEW = 15
METHOD_TYPE_GET_ACCOUNT = 20
METHOD_TYPE_EVENTS_BY_TX_HASH = 25
METHOD_TYPE_GET_BLOCK_BY_HEIGHT = 50
METHOD_TYPE_GET_TX_BY_HASH = 55
METHOD_TYPE_GET_TX_HISTORY_PROOF = 60
METHOD_TYPE_GET_RESOURCE = 150
METHOD_TYPE_GET_RESOURCE_PATH_BY_HASH = 155
METHOD_TYPE_BATCH_GET_RESOURCE_PATH_BY_HASH = 160
METHOD_TYPE_GET_ACCESS_VALUE = 165


class MethodType(IntEnum):
    CHAIN_HEAD = 1
    SUBMIT_TX = 5
    SIMULATE_TX = 10
    VIEW = 15
    GET_ACCOUNT = 20
    EVENTS_BY_TX_HASH = 25
    GET_BLOCK_BY_HEIGHT = 50
    GET_TX_BY_HASH = 55
    GET_TX_HISTORY_PROOF = 60
    GET_RESOURCE = 150
    GET_RESOURCE_PATH_BY_HASH = 155
    BATCH_GET_RESOURCE_PATH_BY_HASH = 160
    GET_ACCESS_VALUE = 165


def new_rpc_request(method: MethodType, request_id: int, body: bytes) -> "RpcRequest":
    return RpcRequest(Method=method, RequestId=request_id, Body=body)


@dataclass
class RpcRequest:
    Method: MethodType = MethodType(1)
    RequestId: int = 0
    Body: bytes = b""

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u16(int(self.Method))
        except ValueError as exc:
            raise ValueError(f"failed to serialize Method: {exc}") from exc
        try:
            serializer.serialize_u64(self.RequestId)
        except ValueError as exc:
            raise ValueError(f"failed to serialize RequestId: {exc}") from exc
        try:
            serializer.serialize_bytes(self.Body)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Body: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.Method = MethodType(deserializer.deserialize_u16())
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Method: {exc}") from exc
        try:
            self.RequestId = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize RequestId: {exc}"
            ) from exc
        try:
            self.Body = deserializer.deserialize_bytes() or b""
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Body: {exc}") from exc
