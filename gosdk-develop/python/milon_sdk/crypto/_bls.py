"""BLS12-381 后端（G2 签名 / G1 公钥，压缩 96/48 字节）。

对应 Go `supranational/blst`：
    - 公钥：blst.KeyGen(seed) → P1Affine.Compress()（48B）
    - 签名：P2Affine.Sign(sk, msg, nil).Compress()（96B）

跨语言 parity（已实证，见 parity/blsgen + tests/test_crypto.py::test_bls_byte_parity）：
- `blst.KeyGen` 与 `py_ecc.bls.G2Basic.KeyGen` 的标量派生一致（IETF BLS KeyGen），公钥 48B 逐字节一致。
- **Go SDK 传 DST=nil，blst 实际使用「空 DST」**（不是评估文档原先假设的
  `BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_NUL_`；已用 blst.HashToG2(msg, nil) 对比
  空/NUL/POP 三种 DST 实证：仅空 DST 匹配）。因此本模块用
  `py_ecc.bls.hash_to_curve.hash_to_G2(msg, b"", sha256)` 实现同一 hash-to-curve，
  签名 96B 与 Go blst 逐字节一致；G2Basic 自带的 POP DST 路径不兼容，已弃用。

因此 blst 未安装时，py_ecc 回退路径同样与 Go 字节兼容（无需 Phase 0 spike）。
"""
from __future__ import annotations

import hashlib

# Go SDK 调用 blst.Sign(sk, msg, nil)，nil DST 在 blst 中即「空 DST」。
# 用 py_ecc hash_to_G2 复刻时必须传 b""（而非 NUL/POP 字符串）才能逐字节一致。
BLST_DEFAULT_DST = b""

_BACKEND: str | None = None

try:  # 优先：与 Go 同源（blst 官方绑定，CGO）
    import blst  # type: ignore

    _BACKEND = "blst"
except ImportError:  # pragma: no cover
    blst = None
    try:
        import py_ecc  # type: ignore

        from py_ecc.bls import G2Basic  # type: ignore
        from py_ecc.bls.g2_primitives import (  # type: ignore
            G2_to_signature,
            pubkey_to_G1,
            signature_to_G2,
        )
        from py_ecc.bls.hash_to_curve import hash_to_G2 as _hash_to_G2  # type: ignore
        from py_ecc.optimized_bls12_381 import (  # type: ignore
            FQ12,
            G1,
            final_exponentiate,
            multiply,
            neg,
            pairing,
        )

        _BACKEND = "py_ecc"
        _g2_basic = G2Basic
    except ImportError:  # pragma: no cover
        py_ecc = None
        _g2_basic = None
        _BACKEND = None


def backend() -> str | None:
    return _BACKEND


def _py_ecc_public(seed: bytes) -> bytes:
    sk = _g2_basic.KeyGen(seed)
    return _g2_basic.SkToPk(sk)


def _py_ecc_sign(seed: bytes, msg: bytes) -> bytes:
    sk = _g2_basic.KeyGen(seed)
    q = _hash_to_G2(msg, BLST_DEFAULT_DST, hashlib.sha256)
    sig_point = multiply(q, sk)
    return bytes(G2_to_signature(sig_point))


def _py_ecc_verify(pub: bytes, sig: bytes, msg: bytes) -> bool:
    if len(pub) != 48 or len(sig) != 96:
        return False
    try:
        pk_point = pubkey_to_G1(pub)
        sig_point = signature_to_G2(sig)
        q = _hash_to_G2(msg, BLST_DEFAULT_DST, hashlib.sha256)
        # e(G1, sig) * e(neg(pk), H(m)) == 1  （与 py_ecc G2Basic._CoreVerify 一致）
        return bool(
            final_exponentiate(
                pairing(sig_point, G1, final_exponentiate=False)
                * pairing(q, neg(pk_point), final_exponentiate=False)
            )
            == FQ12.one()
        )
    except Exception:  # noqa: BLE001 非法点/压缩格式
        return False


def bls_public_from_seed(seed: bytes) -> bytes:
    """从 32 字节种子派生 48 字节压缩 G1 公钥（与 Go blst.KeyGen 一致）。"""
    if _BACKEND == "blst":
        sk = blst.KeyGen(seed)
        return bytes(blst.P1Affine().from_scalar(sk).compress())
    if _BACKEND == "py_ecc":
        return _py_ecc_public(seed)
    raise ImportError("BLS12-381 需要 'blst' 或 'py_ecc' 库")


def bls_sign(seed: bytes, msg: bytes) -> bytes:
    """对原始消息签名（G2，96 字节压缩；DST=blst 默认，与 Go 逐字节一致）。"""
    if _BACKEND == "blst":
        sk = blst.KeyGen(seed)
        return bytes(blst.P2Affine().Sign(sk, msg, None).compress())
    if _BACKEND == "py_ecc":
        return _py_ecc_sign(seed, msg)
    raise ImportError("BLS12-381 需要 'blst' 或 'py_ecc' 库")


def bls_verify(pub: bytes, sig: bytes, msg: bytes) -> bool:
    """验证 96 字节 G2 签名（与 Go blst Verify(checkPub, pk, checkSig, msg, nil) 一致）。"""
    if _BACKEND == "blst":
        pk = blst.P1Affine().uncompress(pub)
        if pk is None:
            return False
        s = blst.P2Affine().uncompress(sig)
        if s is None:
            return False
        return bool(s.verify(True, pk, True, msg, None))
    if _BACKEND == "py_ecc":
        return _py_ecc_verify(pub, sig, msg)
    raise ImportError("BLS12-381 需要 'blst' 或 'py_ecc' 库")
