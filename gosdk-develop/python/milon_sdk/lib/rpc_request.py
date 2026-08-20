"""RPC 请求线格式（对应 Go lib/rpcRequest.go）。

Postcard 线（Content-Type: application/x-milon+postcard）：
    method(u16 变长) + request_id(u64 变长) + body(bytes)

MethodType 编号（1-165，与 Go 常量一一对应）：
    1=ChainHead 5=SubmitTx 10=SimulateTx 15=View 20=GetAccount 25=EventsByTxHash
    50=GetBlockByHeight 55=GetTxByHash 60=GetTxHistoryProof
    150=GetResource 155=GetResourcePathByHash 160=BatchGetResourcePathByHash 165=GetAccessValue
"""
from __future__ import annotations

from enum import IntEnum

from ..postcard import Deserializer, Serializer

CONTENT_TYPE_MILON_POSTCARD = "application/x-milon+postcard"
CONTENT_TYPE_MILON_JSON = "application/x-milon+json"


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


class RpcRequest:
    __slots__ = ("method", "request_id", "body")

    def __init__(self, method: MethodType, request_id: int, body: bytes):
        self.method = MethodType(method)
        self.request_id = request_id
        self.body = bytes(body)

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_u16(int(self.method))
        serializer.serialize_u64(self.request_id)
        serializer.serialize_bytes(self.body)

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "RpcRequest":
        method = MethodType(d.deserialize_u16())
        request_id = d.deserialize_u64()
        body = d.deserialize_bytes()
        return cls(method, request_id, body)
