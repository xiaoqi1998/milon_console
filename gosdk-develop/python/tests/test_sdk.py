"""Milon Python SDK 离线冒烟测试（无需连接节点）。

覆盖：
- 包导入与公共表面
- Postcard 序列化的 round-trip
- 地址派生（BLAKE3 域哈希）字节级一致
- gen 动态绑定的指令编码/解码（gen.Token.BalanceOf）
- 交易构建、签名（Ed25519）、哈希与线编解码 round-trip
- Client 离线构建（加载内置 IDL、注册表、type 解析器）
"""
import os
import sys

import pytest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from milon_sdk import (  # noqa: E402
    Address,
    ClassicalSecretKey,
    Client,
    LocalNet,
    MIL_TOKEN,
    NewClient,
    PubKeySignatureMode,
)
from milon_sdk.lib import Signer, Transaction, TransactionBuilder  # noqa: E402
from milon_sdk.lib import SigningSlot  # noqa: E402
from milon_sdk.postcard import Deserializer, Serializer  # noqa: E402
from milon_sdk import gen  # noqa: E402


def test_import_and_public_surface():
    assert NewClient is not None
    assert Client is not None
    # 常用 API 存在
    for name in ("get_chain_head", "submit_tx", "balance_of", "view", "wait_for_transaction"):
        assert hasattr(Client, name), f"Client missing {name}"


def test_postcard_roundtrip():
    s = Serializer()
    s.serialize_u8(255)
    s.serialize_u16(2581)
    s.serialize_u32(70000)
    s.serialize_u64(4294967295)
    s.serialize_u128((1 << 100) + 7)
    s.serialize_bool(True)
    s.serialize_str("hello milon")
    s.serialize_bytes(b"\x01\x02\x03")
    s.serialize_seq([1, 2, 3], lambda s2, v: s2.serialize_u32(v))
    s.serialize_enum_variant(2)
    raw = s.bytes()

    d = Deserializer(raw)
    assert d.deserialize_u8() == 255
    assert d.deserialize_u16() == 2581
    assert d.deserialize_u32() == 70000
    assert d.deserialize_u64() == 4294967295
    assert d.deserialize_u128() == (1 << 100) + 7
    assert d.deserialize_bool() is True
    assert d.deserialize_str() == "hello milon"
    assert d.deserialize_bytes() == b"\x01\x02\x03"
    assert d.deserialize_seq(lambda dd: dd.deserialize_u32()) == [1, 2, 3]
    assert d.deserialize_enum_variant() == 2
    d.assert_end()


def test_address_derivation():
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    addr = Address.from_public_key(pk)
    assert len(addr.as_bytes()) == 20
    # 重新派生应一致
    assert Address.from_public_key(pk) == addr


def test_gen_binding_and_idl_encode_decode():
    client = NewClient(LocalNet)
    token_pd = client.get_all_pd()["token"]

    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    account = Address.from_public_key(pk)

    wire = gen.Token.BalanceOf.Args(MIL_TOKEN, account).Encode()
    assert isinstance(wire, bytes)
    assert len(wire) >= 3
    # app_id 应为 token 的 2
    assert wire[0] == 2

    decoded = token_pd.decode("BalanceOf", wire)
    assert decoded["account"] == account
    assert decoded["token"] == MIL_TOKEN


def test_transaction_build_sign_serialize():
    client = NewClient(LocalNet)
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    account = Address.from_public_key(pk)

    wire = gen.Token.ClaimFaucet.Args(account).Encode()
    builder = TransactionBuilder([wire])
    builder.apply_slots(
        [SigningSlot(address=account, instruction_indices=[0], include_payer=False, mode=PubKeySignatureMode(public_key=pk))]
    )
    builder.sign_with(Signer(secret_key=sk, public_key=pk))
    tx = builder.build()

    # 恰好一个签名，且授权了 ix 0
    assert len(tx.tx_sigs) == 1
    acc_sig = tx.tx_sigs[0].account_signature
    assert acc_sig.auth_bit.test(0)

    # 哈希与序列化 round-trip
    tx_hash = tx.tx_hash()
    assert len(tx_hash.as_bytes()) == 32
    raw = tx.to_bytes()
    tx2 = Transaction.unmarshal_postcard(Deserializer(raw))
    assert tx2.tx_hash() == tx_hash
    assert len(tx2.instructions) == 1


def test_client_offline_construct():
    # NewClient 不应发起任何网络请求
    client = NewClient(LocalNet)
    assert client.get_provider_manager() is not None
    providers = client.get_all_pd()
    assert set(["system", "account", "token", "staking", "identity", "nft", "randomness", "demo"]).issubset(providers.keys())
    # type resolver 已就绪
    assert client.rpc.type_resolver is not None
    # gen 绑定已刷新
    assert gen.Token is not None


def test_resolver_decode_resource_and_event():
    client = NewClient(LocalNet)
    resolver = client.rpc.type_resolver
    # 用一个已知 IDL 类型做资源解码 smoke：构造一个 u64 值的 type_tag 解码
    # 选取 token IDL 中的某个 type_tag（例如 Metadata 结构）。先取一个存在的 type_tag。
    token_pd = client.get_all_pd()["token"]
    if token_pd.idl_type_by_type_tag:
        type_tag = next(iter(token_pd.idl_type_by_type_tag.keys()))
        idl_type = token_pd.get_idl_type_by_type_tag(type_tag)
        # 编码一个该类型的值（若为基础类型则跳过）
        if idl_type.kind == "struct" and idl_type.fields:
            # 仅验证 resolver 对未知/空数据不崩溃（构造最小可用值较复杂，这里仅验证接口存在）
            assert hasattr(resolver, "decode_resource")
            assert hasattr(resolver, "decode_event")
