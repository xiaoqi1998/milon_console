"""FN-DSA-512 纯 Python 移植（接口占位）。

源：本机 Go module cache 的 go-fn-dsa@v0.2.0（Falcon-512）。
接口契约见 pysdk-develop/CONVENTIONS.md。

状态：待攻坚。完成前 keygen/sign/verify 抛 NotImplementedError；
crypto 中 FN-DSA 相关测试用例以 skip 标记，攻坚完成后解除。
"""

from __future__ import annotations

LOG_N = 9
SIGN_KEY_LEN = 1281
VRFY_KEY_LEN = 897
SIG_LEN = 666


def keygen(log_n: int = LOG_N, seed: bytes | None = None) -> tuple[bytes, bytes]:
    """生成 (sign_key, vrfy_key)；seed 提供时为确定性密钥生成。"""
    raise NotImplementedError("FN-DSA-512 纯 Python 移植进行中（见实施计划 Task 1.2）")


def sign(
    sign_key: bytes,
    ctx: bytes,
    hash_id: int,
    msg: bytes,
    seed: bytes | None = None,
) -> bytes:
    """签名，返回 666 字节签名。"""
    raise NotImplementedError("FN-DSA-512 纯 Python 移植进行中（见实施计划 Task 1.2）")


def verify(vrfy_key: bytes, ctx: bytes, hash_id: int, msg: bytes, sig: bytes) -> bool:
    raise NotImplementedError("FN-DSA-512 纯 Python 移植进行中（见实施计划 Task 1.2）")
