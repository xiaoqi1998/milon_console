"""Python ↔ Go 黄金向量对拍（PYTHON_MIGRATION_ASSESSMENT.md §6.4 的跨实现 parity）。

用法：
    python parity/check_parity.py            # 读取 parity/golden_go.json 对拍
    python parity/check_parity.py --embed    # 打印可嵌入 tests/ 的断言代码

说明：golden_go.json 由 Go 侧 golden_gen.go 生成（固定种子，确定性）。
"""
from __future__ import annotations

import json
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from milon_sdk import gen, postcard, lib  # noqa: E402
from milon_sdk.api import base as api_base  # noqa: E402
from milon_sdk.crypto import Address, ClassicalSecretKey  # noqa: E402
from milon_sdk.lib.account_signature import PubKeySignatureMode  # noqa: E402
from milon_sdk.lib.transaction_builder import TransactionBuilder  # noqa: E402

FIXED_SEED = bytes(range(1, 33))
STAMP = 1700000000000

HERE = os.path.dirname(os.path.abspath(__file__))


def load_golden() -> dict:
    with open(os.path.join(HERE, "golden_go.json"), "r", encoding="utf-8") as f:
        return json.load(f)


def compute() -> dict:
    out: dict = {}

    # 1. postcard 变长整数
    out["postcard_varints"] = {}
    for v in (0, 127, 128, 2581, 16384, 4294967295, 900000001):
        s = postcard.Serializer()
        s.serialize_u64(v)
        out["postcard_varints"][str(v)] = s.bytes().hex()

    # 2. 地址派生（固定种子）
    sk = ClassicalSecretKey(FIXED_SEED)
    pk = sk.ed25519_public()
    addr = Address.from_public_key(pk)
    out["ed25519_pk"] = pk.to_base58()
    out["ed25519_pk_hex"] = pk.to_hex()
    out["address_base58"] = addr.to_base58()
    out["address_hex"] = addr.to_hex()

    # 3. BalanceOf 指令 wire（运行时绑定 + 真实 IDL）
    bindings = gen.default_bindings()
    out["balance_of_wire"] = bindings.token.BalanceOf.args(api_base.MIL_TOKEN, addr).encode().hex()

    # 4. IxHash / TxHash / 完整交易线（确定性 stamp）
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

    # 5. PublicKey postcard
    s = postcard.Serializer()
    pk.marshal_postcard(s)
    out["pubkey_postcard_hex"] = s.bytes().hex()

    return out


def compare(verbose: bool = True) -> bool:
    golden = load_golden()
    mine = compute()
    ok = True
    for key in golden:
        g = golden[key]
        m = mine.get(key)
        if isinstance(g, dict):
            same = g == m
        else:
            same = str(g) == str(m)
        if same:
            if verbose:
                print(f"  [OK]   {key}")
        else:
            ok = False
            print(f"  [FAIL] {key}\n    go:      {g}\n    python:  {m}")
    print("---" * 10)
    print("PARITY: " + ("PASS" if ok else "FAIL"))
    return ok


def emit_asserts() -> None:
    """输出可直接嵌入 pytest 的黄金向量断言。"""
    mine = compute()
    print("# ===== 黄金向量（固定种子 [1..32] / stamp 1700000000000）=====")
    print(f'FIXED_SEED = bytes(range(1, 33))')
    print(f'STAMP = 1700000000000')
    for key, value in sorted(mine.items()):
        if isinstance(value, dict):
            print(f'GOLDEN["{key}"] = {json.dumps(value, sort_keys=True)}')
        elif isinstance(value, int):
            print(f'GOLDEN["{key}"] = {value}')
        else:
            print(f'GOLDEN["{key}"] = {json.dumps(value)}')


if __name__ == "__main__":
    if "--embed" in sys.argv:
        emit_asserts()
    else:
        ok = compare()
        sys.exit(0 if ok else 1)
