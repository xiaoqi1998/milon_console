"""ResolveResourcePaths. Faithful port of gosdk-develop/resolveResourcePaths.go."""

from __future__ import annotations

from typing import Dict, Set

from .api import RsHash

# 单次 BatchGetResourcePathByHash 的最大 hash 数（防请求体过大/超时）。
RESOLVE_PATH_BATCH_SIZE = 500


def resolve_resource_paths(client, rs_hash_set: Set[RsHash]) -> Dict[RsHash, str]:
    """按块批量查询资源路径; RPC 失败抛错, 绝不静默跳过。"""
    path_map: Dict[RsHash, str] = {}
    if len(rs_hash_set) == 0:
        return path_map

    rs_hash_list = list(rs_hash_set)

    for start in range(0, len(rs_hash_list), RESOLVE_PATH_BATCH_SIZE):
        end = min(start + RESOLVE_PATH_BATCH_SIZE, len(rs_hash_list))

        try:
            batch_result = client.RpcClient.batch_get_resource_path_by_hash(
                rs_hash_list[start:end]
            )
        except ValueError as exc:
            raise ValueError(
                f"BatchGetResourcePathByHash failed (items {start}-{end} of "
                f"{len(rs_hash_list)}): {exc}"
            ) from exc

        for info in batch_result.BodyBatchResourcePathList:
            if info.ErrMsg != "":
                path_map[info.RsHash] = "<err: " + info.ErrMsg + ">"
                continue
            path_map[info.RsHash] = info.Path

    return path_map
