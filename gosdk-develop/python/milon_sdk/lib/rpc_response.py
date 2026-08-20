"""RPC 响应线格式（对应 Go lib/rpcResponse.go）。

Postcard 线：
    request_id(u64 变长) + status(u8) + body(bytes) + error(option)
error 内部：message(str) + code(option u16) + data(option bytes)
"""
from __future__ import annotations

from ..postcard import Deserializer, Serializer

RPC_RESPONSE_STATUS_OK = 0
RPC_RESPONSE_STATUS_INVALID = 1
RPC_RESPONSE_STATUS_NOT_FOUND = 2
RPC_RESPONSE_STATUS_DISABLED = 3
RPC_RESPONSE_STATUS_UNAVAILABLE = 4
RPC_RESPONSE_STATUS_INTERNAL = 5
RPC_RESPONSE_STATUS_FAILED = 6


class RpcResponseError:
    __slots__ = ("message", "code", "data")

    def __init__(self, message: str, code: int | None = None, data: bytes | None = None):
        self.message = message
        self.code = code
        self.data = data

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "RpcResponseError":
        message = d.deserialize_str()
        code = d.deserialize_option(lambda dd: dd.deserialize_u16())
        data = d.deserialize_option(lambda dd: dd.deserialize_bytes())
        return cls(message, code, data)

    def __repr__(self) -> str:
        return f"RpcResponseError(message={self.message!r}, code={self.code})"


class RpcResponse:
    __slots__ = ("request_id", "status", "body", "error")

    def __init__(self, request_id: int, status: int, body: bytes, error: RpcResponseError | None = None):
        self.request_id = request_id
        self.status = status
        self.body = bytes(body)
        self.error = error

    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_u64(self.request_id)
        serializer.serialize_u8(self.status)
        serializer.serialize_bytes(self.body)
        serializer.serialize_option(self.error is not None, lambda s: self.error.marshal_postcard(s))

    @classmethod
    def unmarshal_postcard(cls, d: Deserializer) -> "RpcResponse":
        request_id = d.deserialize_u64()
        status = d.deserialize_u8()
        body = d.deserialize_bytes()
        error = d.deserialize_option(lambda dd: RpcResponseError.unmarshal_postcard(dd))
        return cls(request_id, status, body, error)

    def __repr__(self) -> str:
        return (
            f"RpcResponse(request_id={self.request_id}, status={self.status}, "
            f"body_len={len(self.body)}, error={self.error})"
        )
