"""跨语言对拍固定向量（2026-10-08 由 gosdk-develop 实际运行生成）。

源程序：Go 侧用 crypto.NewClassicalSecretKey 的 FromBytes(seed) + 各曲线签名，
seed = 0x11 * 32，msg = "hello milon"。Python 侧输出必须与这些向量逐字节一致。
"""

import pytest

from milon_sdk import crypto

SEED = b"\x11" * 32
MSG = b"hello milon"

# fmt: off
BLS_PK = bytes.fromhex(
    "8e5a712e4cb2c51893c27ae19afb3455f3efcc66030dc25e13eb1afc2edf3973"
    "17a0bb2d28a55513a32d7dcc404be3ba"
)
BLS_SIG = bytes.fromhex(
    "92e2b0089c13f8f98f41cdd186d7725614ac6ef2c1aaff5ffe33f8329d810e2d"
    "71c355a4ff367553ce05b8f1097af6330b983b6f8a528dc21763779efe0745fc5"
    "3c0f638c8f7ba6a9c2dc3f0e7de2fabba9c8845e94666734684ebbba8870144"
)
ED_PK = bytes.fromhex(
    "d04ab232742bb4ab3a1368bd4615e4e6d0224ab71a016baf8520a332c9778737"
)
ED_SIG = bytes.fromhex(
    "94401c9497c2a1115162f85d5dfa37ab0d59010216febf7c3e1c31461d3b1090"
    "c0c237687e4689a780b34991213f97479c9d81939df6fb7583e6fbea2ae41805"
)
SECP_PK = bytes.fromhex(
    "034f355bdcb7cc0af728ef3cceb9615d90684bb5b2ca5f859ab0f0b704075871aa"
)
SECP_SIG = bytes.fromhex(
    "5603bfa1b33fa1cc766ef1e5d3bd73ced886e4ec6b7eb5587138ca00b229a17d"
    "1784b105d4a8a26f3e66cd8275d8740c15e94844eae5ce8a7f43231f84c4e9351b"
)
ADDR_FROM_BLS_PK = bytes.fromhex("ba478627b8ba410435de22250a7908e646b11ce8")
# fmt: on


def _sk():
    sk = crypto.ClassicalSecretKey()
    sk.from_bytes(SEED)
    return sk


def test_bls_cross_language_vector():
    sk = _sk()
    pk = sk.bls12381_public()
    assert pk.Bytes == BLS_PK
    sig = sk.sign_bls12381(MSG)
    assert sig.Bytes == BLS_SIG
    sig.verify(MSG, pk)


def test_ed25519_cross_language_vector():
    sk = _sk()
    pk = sk.ed25519_public()
    assert pk.Bytes == ED_PK
    sig = sk.sign_ed25519(MSG)
    assert sig.Bytes == ED_SIG
    sig.verify(MSG, pk)


def test_secp256k1_cross_language_vector():
    sk = _sk()
    pk = sk.secp256k1_public()
    assert pk.Bytes == SECP_PK
    sig = sk.sign_secp256k1(MSG)
    assert sig.Bytes == SECP_SIG
    # V 字节 = 27/28（go-ethereum 惯例）
    assert sig.Bytes[64] in (27, 28)
    sig.verify(MSG, pk)


def test_address_cross_language_vector():
    sk = _sk()
    addr = crypto.new_address_from_public_key(sk.bls12381_public())
    assert addr.Bytes == ADDR_FROM_BLS_PK


def test_python_verifies_go_vectors():
    """Go 生成的签名必须能被 Python 验证（互操作硬证据）。"""
    pk_bls = crypto.new_public_key_from_bytes(BLS_PK)
    crypto.new_signature_from_bytes(BLS_SIG).verify(MSG, pk_bls)

    pk_ed = crypto.new_public_key_from_bytes(ED_PK)
    crypto.new_signature_from_bytes(ED_SIG).verify(MSG, pk_ed)

    pk_secp = crypto.new_public_key_from_bytes(SECP_PK)
    crypto.new_signature_from_bytes(SECP_SIG).verify(MSG, pk_secp)
