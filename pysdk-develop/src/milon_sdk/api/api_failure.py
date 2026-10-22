"""ApiFailure. Faithful port of gosdk-develop/api/apiFailure.go."""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Optional

from ..postcard import (
    Deserializer,
    Serializer,
    deserialize_option,
    serialize_option,
)

#   rust (api/rpc/wire/src/envelope.rs)
#   pub struct ApiFailure {
#       pub message: String,
#       #[serde(default)]
#       pub code: Option<u16>,
#       #[serde(default)]
#       pub data: Option<Vec<u8>>,
#   }


@dataclass
class ApiFailure:
    """RPC 响应携带的结构化失败：响应信封的 error 字段，以及批响应体
    （如 batch_view 返回 Vec<Result<Vec<u8>, ApiFailure>>）的每个 Err 项。"""

    Message: str = ""
    Code: Optional[int] = None
    Data: Optional[bytes] = None

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_str(self.Message)
        except ValueError as exc:
            raise ValueError(f"failed to serialize Message: {exc}") from exc

        try:
            serialize_option(
                serializer, self.Code, lambda s, code: s.serialize_u16(code)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize Code: {exc}") from exc

        try:
            serialize_option(
                serializer, self.Data, lambda s, data: s.serialize_bytes(data)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize Data: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.Message = deserializer.deserialize_str()
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Message: {exc}") from exc

        try:
            self.Code = deserialize_option(
                deserializer, lambda d: d.deserialize_u16()
            )
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Code: {exc}") from exc

        try:
            self.Data = deserialize_option(
                deserializer, lambda d: d.deserialize_bytes()
            )
        except ValueError as exc:
            raise ValueError(f"failed to deserialize Data: {exc}") from exc

    # 简洁错误日志的字符串表示：code 十进制、data 十六进制
    def __str__(self) -> str:
        parts = [f"message={json.dumps(self.Message)}"]
        if self.Code is not None:
            parts.append(f"code={self.Code}")
        if self.Data is not None:
            parts.append(f"data={self.Data.hex()}")
        return " ".join(parts)
