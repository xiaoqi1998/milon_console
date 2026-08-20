"""跨语言黄金向量 parity 测试（固定种子，与 Go golden_gen.go 逐字节一致）。

这些向量由 Go 侧生成（go run parity/golden_gen.go），Python 必须完全复现：
postcard 变长整数、地址派生、指令 wire、IxHash/TxHash、完整交易线（含签名）。
"""
from __future__ import annotations

import os
import sys

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from milon_sdk import gen, postcard  # noqa: E402
from milon_sdk.api import base as api_base  # noqa: E402
from milon_sdk.crypto import Address, ClassicalSecretKey  # noqa: E402
from milon_sdk.lib.account_signature import PubKeySignatureMode  # noqa: E402
from milon_sdk.lib.transaction_builder import TransactionBuilder  # noqa: E402

FIXED_SEED = bytes(range(1, 33))
STAMP = 1700000000000

GOLDEN = {
    "address_base58": "2Pp9yR14jnCL8P2YCMWWSbhWJEvM",
    "address_hex": "6402ef52d980a1cc11ba56ea8cadef8168573162",
    "auth_bit": "1",
    "balance_of_wire": "0295e718c06a6895942a9af0191b0232462d6f7c4000006402ef52d980a1cc11ba56ea8cadef8168573162",
    "ed25519_pk": "9C6hybhQ6Aycep9jaUnP6uL9ZYvDjUp1aSkFWPUFJtpj",
    "ed25519_pk_hex": "79b5562e8fe654f94078b112e8a98ba7901f853ae695bed7e0e3910bad049664",
    "ix_hash": "4SJnwqKbnUSt1pcPVpj7eMBjnWL5UUS7F43AjMHq9Djv",
    "ix_hash_hex": "330eb2dbe2c8c886f9cb34c42c281d81b9ea5bca86677472225f9ef5b62434a9",
    "postcard_varints": {
        "0": "00",
        "127": "7f",
        "128": "8001",
        "16384": "808001",
        "2581": "9514",
        "4294967295": "ffffffff0f",
        "900000001": "81d293ad03",
    },
    "pubkey_postcard_hex": "0179b5562e8fe654f94078b112e8a98ba7901f853ae695bed7e0e3910bad049664",
    "sig_base58": "HjmvWmu3bAMvPZbByWWGraoGYGHU38CKqFwJMYx2jFK5TjBtDtEmVuDBUZCd1S9WF9ejKYFswzQjVK8baCNnQV2",
    "sig_hex": "0e6f0c0d5515690d2aee02ee1db26e21f24aaf09cfb2ecd99cfbe3c8f42a2552703559f6b24dba31684ba6d314bb29b7412b89f46c8c2adc36cc6be2f1894c0d",
    "tx_bytes_hex": "80d095ffbc3100012b0295e718c06a6895942a9af0191b0232462d6f7c4000006402ef52d980a1cc11ba56ea8cadef8168573162016402ef52d980a1cc11ba56ea8cadef8168573162010001010e6f0c0d5515690d2aee02ee1db26e21f24aaf09cfb2ecd99cfbe3c8f42a2552703559f6b24dba31684ba6d314bb29b7412b89f46c8c2adc36cc6be2f1894c0d010179b5562e8fe654f94078b112e8a98ba7901f853ae695bed7e0e3910bad049664",
    "tx_hash": "6zPfyQtbYyavn6gn4QxFhM7bbbgKyJTaPNkJntnZXcW8",
    "tx_hash_hex": "58fe2d0e0650a04e667d3ea0027143dc7a9d6bcbf876bc61f472abbecd1d6c95",
    "tx_stamp": 1700000000000,
}


def _build() -> dict:
    out: dict = {}

    out["postcard_varints"] = {}
    for v in (0, 127, 128, 2581, 16384, 4294967295, 900000001):
        s = postcard.Serializer()
        s.serialize_u64(v)
        out["postcard_varints"][str(v)] = s.bytes().hex()

    sk = ClassicalSecretKey(FIXED_SEED)
    pk = sk.ed25519_public()
    addr = Address.from_public_key(pk)
    out["ed25519_pk"] = pk.to_base58()
    out["ed25519_pk_hex"] = pk.to_hex()
    out["address_base58"] = addr.to_base58()
    out["address_hex"] = addr.to_hex()

    bindings = gen.default_bindings()
    out["balance_of_wire"] = bindings.token.BalanceOf.args(api_base.MIL_TOKEN, addr).encode().hex()

    wire = bytes.fromhex(out["balance_of_wire"])
    tx = (
        TransactionBuilder([wire])
        .with_stamp(STAMP)
        .add_ixes_sig(addr, sk, [0], False, PubKeySignatureMode(public_key=pk))
        .build()
    )
    out["tx_stamp"] = tx.stamp
    out["ix_hash"] = tx.ix_hash_from_wire(wire).to_base58()
    out["ix_hash_hex"] = tx.ix_hash_from_wire(wire).to_hex()
    out["tx_hash"] = tx.tx_hash().to_base58()
    out["tx_hash_hex"] = tx.tx_hash().to_hex()
    out["tx_bytes_hex"] = tx.to_bytes().hex()
    sig = tx.tx_sigs[0].account_signature
    out["sig_base58"] = sig.signatures[0].to_base58()
    out["sig_hex"] = sig.signatures[0].to_hex()
    out["auth_bit"] = str(sig.auth_bit.raw())

    s = postcard.Serializer()
    pk.marshal_postcard(s)
    out["pubkey_postcard_hex"] = s.bytes().hex()
    return out


@pytest.mark.parametrize("key", sorted(GOLDEN.keys()))
def test_golden_vector(key: str) -> None:
    got = _build()
    g = GOLDEN[key]
    m = got[key]
    if isinstance(g, dict):
        # 字典比较按内容（不依赖键顺序）
        assert m == g, f"golden vector mismatch: {key}"
    else:
        assert str(m) == str(g), f"golden vector mismatch: {key}"
