"""ListResourcePathInfo. Faithful port of gosdk-develop/api/listResourcePath.go."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, List

from .base import RS_HASH_LEN, RsHash, unmarshal_rs_hash_from_json_array


@dataclass
class ListResourcePathInfo:
    RsHash: RsHash = b"\x00" * RS_HASH_LEN
    Path: str = ""


def unmarshal_list_resource_path_list_from_raw_list(
    raw_list: List[List[Any]],
) -> List[ListResourcePathInfo]:
    """Parses a list of ListResourcePathInfo from raw JSON data.

    rawList: [][]any format, each element is [rsHashBytes(list), path(string)]
    """
    result: List[ListResourcePathInfo] = []
    for item in raw_list:
        # Verify the array has at least 2 elements
        if len(item) < 2:
            raise ValueError("invalid ListResourcePathInfo response")

        rs_hash_bytes_raw = item[0]
        if not isinstance(rs_hash_bytes_raw, list):
            raise ValueError("invalid ListResourcePathInfo response")

        try:
            rs_hash = unmarshal_rs_hash_from_json_array(rs_hash_bytes_raw)
        except ValueError as exc:
            raise ValueError(f"failed to parse RsHash: {exc}") from exc

        path_str = item[1]
        if not isinstance(path_str, str):
            raise ValueError("invalid ListResourcePathInfo response")

        result.append(ListResourcePathInfo(RsHash=rs_hash, Path=path_str))

    return result
