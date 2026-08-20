"""FN-DSA-512 后量子签名封装（对应 Go crypto/fn_dsa512.go）。

⚠️ 关键门控风险（见 PYTHON_MIGRATION_ASSESSMENT.md §4.1）：
Go 侧使用 `pornin/go-fn-dsa v0.2.0`（NIST 后量子草案 FIPS 206，尚不稳定），
调用参数为：原始消息(ctx=空)、HASH_ID_RAW(id=0)。Python 侧候选库为
`tectonic-bedrock-python`（`bedrock.FalconScheme.dsa_512()`），其参数化能否与
Go v0.2.0 逐字节对齐，**必须经 Phase 0 可行性 spike 实证**后才能用于签名上链。

本模块因此采用"可插拔"设计：若 `bedrock` 库可用则启用真实签名/验签；
否则抛 NotImplementedError 并给出明确指引（仅验签/预生成密钥对等降级方案）。
"""
from __future__ import annotations

import secrets
from typing import Optional, Tuple

from .errors import InvalidPublicKeyError, InvalidSignatureError, InvalidSecretKeyError

LOG_N = 9  # 2^9 = 512 维度
FNDSA512_SIGN_KEY_LEN = 1281
FNDSA512_VRFY_KEY_LEN = 897
FNDSA512_SIG_LEN = 666

# 延迟导入：库不存在时不阻断整个 SDK 导入
_BEDROCK = None
try:  # pragma: no cover - 依赖外部库
    import bedrock  # type: ignore

    _BEDROCK = bedrock
except ImportError:  # pragma: no cover
    _BEDROCK = None


def bedrock_available() -> bool:
    return _BEDROCK is not None


def _require_bedrock() -> None:
    if _BEDROCK is None:
        raise NotImplementedError(
            "FN-DSA-512 需要 'tectonic-bedrock-python' 库且其参数化须先通过跨语言"
            "parity spike 验证（Go go-fn-dsa v0.2.0: 原始消息 / 空域 / HASH_ID_RAW）。"
            "降级方案：(a) 仅做验签；(b) 用 Go 侧预生成的 FnDSA 密钥对文件加载。"
        )


def keygen512() -> Tuple[bytes, bytes]:
    """生成 FN-DSA-512 密钥对，返回 (签名密钥 1281B, 验证密钥 897B)。"""
    _require_bedrock()
    raise NotImplementedError(
        "tectonic-bedrock-python 的 FalconScheme.dsa_512() 密钥生成接口尚未实证对齐；"
        "请先在 Phase 0 spike 中确认与 Go fndsa.KeyGen(9) 字节兼容后再启用。"
    )


def sign512(sign_key: bytes, msg: bytes) -> bytes:
    """对原始消息签名（与 Go fndsa.Sign(rng, sk, [], 0, msg) 对应）。"""
    _require_bedrock()
    raise NotImplementedError(
        "FN-DSA-512 签名路径未通过跨语言 parity 验证，暂不可用。"
        "见 PYTHON_MIGRATION_ASSESSMENT.md §7 的降级决策。"
    )


def verify512(vrfy_key: bytes, sig: bytes, msg: bytes) -> bool:
    """验证 FN-DSA-512 签名（与 Go fndsa.Verify(vk, [], 0, msg, sig) 对应）。"""
    _require_bedrock()
    raise NotImplementedError(
        "FN-DSA-512 验签路径未通过跨语言 parity 验证，暂不可用。"
    )


def _validate_sign_key(raw: bytes) -> bytes:
    if len(raw) != FNDSA512_SIGN_KEY_LEN:
        raise InvalidSecretKeyError(
            f"expected {FNDSA512_SIGN_KEY_LEN} bytes, got {len(raw)}"
        )
    return bytes(raw)


def _validate_vrfy_key(raw: bytes) -> bytes:
    if len(raw) != FNDSA512_VRFY_KEY_LEN:
        raise InvalidPublicKeyError(
            f"expected {FNDSA512_VRFY_KEY_LEN} bytes, got {len(raw)}"
        )
    return bytes(raw)


def _validate_sig(raw: bytes) -> bytes:
    if len(raw) != FNDSA512_SIG_LEN:
        raise InvalidSignatureError(f"expected {FNDSA512_SIG_LEN} bytes, got {len(raw)}")
    return bytes(raw)
