"""BatchGetResourcePathInfo. Faithful port of gosdk-develop/api/batchGetResourcePathByHash.go."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, List

from .base import RS_HASH_LEN, RsHash, unmarshal_rs_hash_from_json_array


@dataclass
class BatchGetResourcePathInfo:
    RsHash: RsHash = b"\x00" * RS_HASH_LEN
    Path: str = ""  # valid when the result is Ok
    ErrMsg: str = ""  # valid when the result is Err


def unmarshal_batch_resource_path_list_from_raw_list(
    raw_list: List[List[Any]],
) -> List[BatchGetResourcePathInfo]:
    """Parses a list of (RsHash, Result<String, String>) from raw JSON data.

    rawList: [][]any format, each element is [rsHashBytes(list), result(dict
    with "Ok"|"Err" key)]
    """
    result: List[BatchGetResourcePathInfo] = []
    for item in raw_list:
        if len(item) < 2:
            raise ValueError("invalid BatchGetResourcePathInfo response")

        rs_hash_bytes_raw = item[0]
        if not isinstance(rs_hash_bytes_raw, list):
            raise ValueError("invalid BatchGetResourcePathInfo response")

        try:
            rs_hash = unmarshal_rs_hash_from_json_array(rs_hash_bytes_raw)
        except ValueError as exc:
            raise ValueError(f"failed to parse RsHash: {exc}") from exc

        # Result<String, String>: externally tagged enum {"Ok": path}|{"Err": msg}
        result_map = item[1]
        if not isinstance(result_map, dict):
            raise ValueError("invalid BatchGetResourcePathInfo response")

        info = BatchGetResourcePathInfo(RsHash=rs_hash)
        if isinstance(result_map.get("Ok"), str):
            info.Path = result_map["Ok"]
        elif isinstance(result_map.get("Err"), str):
            info.ErrMsg = result_map["Err"]
        result.append(info)
    return result
