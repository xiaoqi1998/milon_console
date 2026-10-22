"""RpcResponse. Faithful port of gosdk-develop/lib/rpcResponse.go."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional

from ..api import ApiFailure
from ..postcard import (
    Deserializer,
    Serializer,
    deserialize_option,
    serialize_option,
)

RPC_RESPONSE_STATUS_OK = 0
RPC_RESPONSE_STATUS_INVALID = 1
RPC_RESPONSE_STATUS_NOT_FOUND = 2
RPC_RESPONSE_STATUS_DISABLED = 3
RPC_RESPONSE_STATUS_UNAVAILABLE = 4
RPC_RESPONSE_STATUS_INTERNAL = 5
RPC_RESPONSE_STATUS_FAILED = 6


@dataclass
class RpcResponse:
    RequestId: int = 0
    Status: int = 0
    Body: bytes = b""
    Error: Optional[ApiFailure] = None

    def to_json_value(self) -> dict:
        return {
            "request_id": self.RequestId,
            "status": self.Status,
            "body": self.Body.hex(),
            "error": (
                {
                    "message": self.Error.Message,
                    "code": self.Error.Code,
                    "data": self.Error.Data.hex() if self.Error.Data else None,
                }
                if self.Error
                else None
            ),
        }

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u64(self.RequestId)
        except ValueError as exc:
            raise ValueError(
                f"failed to serialize RequestId: {exc}"
            ) from exc
        try:
            serializer.serialize_u8(self.Status)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Status: {exc}") from exc
        try:
            serializer.serialize_bytes(self.Body)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Body: {exc}") from exc
        try:
            serialize_option(
                serializer, self.Error, lambda s, e: e.marshal_postcard(s)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize Error: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.RequestId = deserializer.deserialize_u64()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Message: {exc}") from exc
        try:
            self.Status = deserializer.deserialize_u8()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Status: {exc}") from exc
        try:
            self.Body = deserializer.deserialize_bytes() or b""
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Body: {exc}") from exc

        def _read_err(d: Deserializer) -> ApiFailure:
            af = ApiFailure()
            try:
                af.unmarshal_postcard(d)
            except ValueError as exc:
                raise ValueError(f"failed to deserialize Error: {exc}") from exc
            return af

        try:
            self.Error = deserialize_option(deserializer, _read_err)
        except ValueError:
            raise
