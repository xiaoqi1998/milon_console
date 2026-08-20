"""链全局配置（对应 Go lib/transaction.go 中的 ChainId 全局）。

与 Go 一致：模块级全局 + 读写锁。NewClient 时会通过 set_chain_id 覆盖。
"""
from __future__ import annotations

import threading

_chain_id = 900_000_001
_lock = threading.RLock()


def set_chain_id(chain_id: int) -> None:
    global _chain_id
    with _lock:
        _chain_id = chain_id


def get_chain_id() -> int:
    with _lock:
        return _chain_id


TransactionStamp = int
RequestID = int
