"""BLS12-381 原语（与 gosdk 的 blst 调用逐字节兼容）。

跨语言对拍实证（2026-10-08，固定 seed=0x11*32，见 tests/crypto/test_vectors.py）：
- KeyGen：blst.KeyGen(seed) = BLS 签名规范 draft-04 版 HKDF（盐先 SHA-256、
  extract(IKM||0x00)、expand(I2OSP(48,2))、OS2IP(OKM) mod r）。
- Sign：blst P2Affine.Sign(sk, msg, nil) 的 nil DST 在绑定层不做替换，
  C 侧以**空 DST** 调 hash_to_field；map 为 SSWU（3-isogeny）。
- 公钥/签名压缩为 ZCash 格式（48B / 96B），与 py_ecc 序列化一致。
"""

from __future__ import annotations

import hashlib
import hmac as _hmac
from typing import Any, Tuple

from py_ecc.bls.g2_primitives import (
    G1_to_pubkey,
    G2_to_signature,
    pubkey_to_G1,
    signature_to_G2,
)
from py_ecc.bls.hash_to_curve import hash_to_G2
from py_ecc.optimized_bls12_381 import (
    FQ12,
    G1,
    final_exponentiate,
    multiply,
    neg,
    normalize,
    pairing,
)

# BLS12-381 子群阶 r（hex 直读，勿用十进制字面量——曾因 typo 引发对拍事故）
R = 0x73EDA753299D7D483339D80809A1D80553BDA402FFFE5BFEFFFFFFFF00000001

# gosdk 经 blst 绑定传 nil DST 时的实际取值：空 DST
BLS_DST = b""

G1Point = Tuple[Any, ...]


def derive_master_sk(seed: bytes) -> int:
    """blst.KeyGen(ikm)：BLS 签名规范 draft-04 版 HKDF-MOD-R。"""
    salt = b"BLS-SIG-KEYGEN-SALT-"
    sk = 0
    while sk == 0:
        salt = hashlib.sha256(salt).digest()
        prk = _hmac.new(salt, seed + b"\x00", hashlib.sha256).digest()
        okm = b""
        t = b""
        counter = 1
        while len(okm) < 48:
            t = _hmac.new(prk, t + b"\x00\x30" + bytes([counter]), hashlib.sha256).digest()
            okm += t
            counter += 1
        sk = int.from_bytes(okm[:48], "big") % R
    return sk


def sk_to_pk(sk: int) -> bytes:
    """公钥 = G1 生成元×标量，压缩 48 字节。"""
    return G1_to_pubkey(multiply(G1, sk))


def hash_msg_to_g2(msg: bytes) -> Any:
    """blst Sign/Verify 用的 hash-to-G2：空 DST、SSWU、含 cofactor clear。"""
    return hash_to_G2(msg, BLS_DST, hashlib.sha256)


def sign(sk: int, msg: bytes) -> bytes:
    """签名 = G2 hash_to_G2(msg)×标量，压缩 96 字节。"""
    return G2_to_signature(multiply(hash_msg_to_g2(msg), sk))


def verify(pubkey48: bytes, msg: bytes, sig96: bytes) -> bool:
    """basic 方案配对验证：e(sig, g1) · e(H(msg), pk) == 1。"""
    try:
        sig_pt = signature_to_G2(sig96)
        pk_pt = pubkey_to_G1(pubkey48)
        c = final_exponentiate(
            pairing(sig_pt, G1, final_exponentiate=False)
            * pairing(hash_msg_to_g2(msg), neg(pk_pt), final_exponentiate=False)
        )
        return c == FQ12.one()
    except Exception:
        return False


def uncompress_g1(data: bytes) -> G1Point:
    """48 字节压缩 → py_ecc G1 仿射点（Go: blst P1Affine.Uncompress）。"""
    return normalize(pubkey_to_G1(data))


def compress_g1(point: G1Point) -> bytes:
    """py_ecc G1 点（Jacobi 3 元组或仿射 2 元组）→ 48 字节压缩。"""
    if len(point) == 2:
        # 仿射 → Jacobi (x, y, 1)
        from py_ecc.optimized_bls12_381 import FQ

        point = (point[0], point[1], FQ.one())
    return G1_to_pubkey(point)
