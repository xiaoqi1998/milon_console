"""链 ID 全局状态（Go 位于 transaction.go，Python 侧拆出以避免与
account_signature 的循环导入）。"""

import threading

_chain_id_lock = threading.RLock()
_chain_id = 900_000_001


def set_chain_id(chain_id: int) -> None:
    global _chain_id
    with _chain_id_lock:
        _chain_id = chain_id


def GetChainId() -> int:
    with _chain_id_lock:
        return _chain_id


def get_chain_id() -> int:
    return GetChainId()
