"""HTTP 传输层（对应 Go tools/http.go 的 HttpPostByBytes）。

连接池 + 5xx 重试（3 次，指数退避）。Python 用 requests.Session 复用连接。
"""
from __future__ import annotations

import time

import requests

_SESSION = requests.Session()


def http_post_by_bytes(
    url: str,
    payload: bytes,
    headers: dict[str, str],
    timeout: float | None = None,
    max_retries: int = 3,
) -> tuple[int, bytes]:
    """POST 原始字节，返回 (HTTP 状态码, 响应字节)。5xx 自动重试。"""
    last_exc: Exception | None = None
    for attempt in range(max_retries):
        try:
            resp = _SESSION.post(url, data=payload, headers=headers, timeout=timeout)
        except requests.RequestException as exc:
            last_exc = exc
            if attempt < max_retries - 1:
                time.sleep(0.5 * (2 ** attempt))
                continue
            raise RuntimeError(f"RPC call failed: {exc}") from exc
        if resp.status_code >= 500 and attempt < max_retries - 1:
            last_exc = RuntimeError(f"server error status {resp.status_code}")
            time.sleep(0.5 * (2 ** attempt))
            continue
        return resp.status_code, resp.content
    raise RuntimeError(f"RPC call failed after {max_retries} attempts: {last_exc}")
