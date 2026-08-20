"""Milon Python SDK 快速上手。

演示完整链路：构造客户端 → 派生地址 → 编码指令 → 构建/签名交易 → 提交链上。

离线可用的部分（地址派生、指令编码、交易构建/签名、哈希）不依赖网络，
可直接运行观察结果；涉及 RPC 的提交/查询调用已用 try/except 包裹，
在无网或 DevNet 不可达时会打印错误而非崩溃。

运行：
    python example/quickstart.py
"""
from __future__ import annotations

from milon_sdk import NewClient, DevNet, gen
from milon_sdk.crypto import ClassicalSecretKey, Address
from milon_sdk.lib.account_signature import PubKeySignatureMode
from milon_sdk.lib.transaction_builder import TransactionBuilder
from milon_sdk.api import base as api_base


def main() -> None:
    # 1) 构造客户端（自动加载内置 IDL，绑定 gen.Token / gen.Account ...）
    client = NewClient(DevNet)
    print(f"client ready, chain_id={client.rpc.network.chain_id}, rpc={client.rpc.network.rpc_url}")

    # 2) 密钥与地址派生（BLAKE3(ROOT || "milon.address.pk.v1" || pk)[:20]）
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    addr = Address.from_public_key(pk)
    print(f"address (base58): {addr.to_base58()}")
    print(f"address (hex)   : {addr.to_hex()}")

    # 3) 用动态 IDL 绑定编码指令（运行时绑定，零生成代码）
    balance_wire = (
        gen.Token.BalanceOf.Args(api_base.MIL_TOKEN, addr).Encode()
    )
    print(f"BalanceOf wire ({len(balance_wire)}B): {balance_wire.hex()}")

    # 4) 构建并签名交易（SplitPayerSelfPay：执行者同时授权 ix0 与 gas）
    tx = (
        TransactionBuilder([balance_wire])
        .with_stamp(1_700_000_000_000)
        .add_ixes_sig(addr, sk, [0], True, PubKeySignatureMode(public_key=pk))
        .build()
    )
    print(f"tx hash : {tx.tx_hash().to_base58()}")
    print(f"tx bytes: {tx.to_bytes().hex()}")

    # 5) 链上调用（需要可达的 DevNet；离线运行会捕获异常）
    try:
        balance = client.balance_of(addr)
        print(f"balance of {addr.to_base58()}: {balance}")
    except Exception as exc:  # noqa: BLE001
        print(f"[skip] BalanceOf RPC unavailable: {exc}")

    try:
        client.claim_faucet(sk, addr, PubKeySignatureMode(public_key=pk))
        print(f"faucet claimed for {addr.to_base58()}")
    except Exception as exc:  # noqa: BLE001
        print(f"[skip] ClaimFaucet RPC unavailable: {exc}")


if __name__ == "__main__":
    main()
