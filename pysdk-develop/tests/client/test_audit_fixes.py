"""彻底检查轮新增回归：account_signer_bit 真实语义 / gen 11 apps / 顶层导出面。"""

import pytest

import milon_sdk as m
from milon_sdk import gen, new_client
from milon_sdk.crypto import Address
from milon_sdk.types.bitbap import Bitmap64


class _FakeRpc(m.RpcClientImpl):
    """绕过网络的桩客户端，直接喂 ListSigners 解码后的结构。"""

    def __init__(self, list_signers_result):
        self._result = list_signers_result

    def list_account_signers(self, account):
        return self._result


def test_account_signer_bit_single_signer():
    rpc = _FakeRpc([{"bitmap": 4}, [("pk-bytes", 1, 1)]])  # bit2 单签
    assert rpc.account_signer_bit(Address(bytes(20))) == Bitmap64(4)


def test_account_signer_bit_multi_signer_rejected():
    rpc = _FakeRpc([{"bitmap": 0b101}, [1, 2]])
    with pytest.raises(ValueError, match="single-signer accounts only"):
        rpc.account_signer_bit(Address(bytes(20)))


def test_account_signer_bit_bitmap_multiple_slots_rejected():
    # signers 恰好 1 个但 bitmap 多位
    rpc = _FakeRpc([{"bitmap": 0b11}, [1]])
    with pytest.raises(ValueError, match="multiple signer slots"):
        rpc.account_signer_bit(Address(bytes(20)))


def test_account_signer_bit_bad_bitmap_rejected():
    rpc = _FakeRpc([{"bitmap": 0}, [1]])
    with pytest.raises(ValueError, match="unexpected account bitmap"):
        rpc.account_signer_bit(Address(bytes(20)))
    rpc2 = _FakeRpc([{"other": 1}, [1]])
    with pytest.raises(ValueError, match="unexpected account bitmap"):
        rpc2.account_signer_bit(Address(bytes(20)))


def test_account_signer_bit_bad_shapes():
    with pytest.raises(ValueError, match="unexpected account data"):
        _FakeRpc(["not-a-map", [1]]).account_signer_bit(Address(bytes(20)))
    with pytest.raises(ValueError, match="unexpected signers list"):
        _FakeRpc([{"bitmap": 1}, "not-a-list"]).account_signer_bit(Address(bytes(20)))


def test_gen_eleven_apps_bound():
    c = new_client(m.DEV_NET)
    apps = [
        "SYSTEM", "ACCOUNT", "TOKEN", "STAKING", "IDENTITY", "SFTOKEN",
        "DEX", "KEYLESS", "LUCKY_BOX", "SOCIAL", "DEMO",
    ]
    for name in apps:
        app = getattr(gen, name)
        assert app.provider is not None, name
        assert len(app.provider.InstructionByName) > 0, name
    # get_app 反查
    assert gen.get_app("sftoken") is gen.SFTOKEN
    with pytest.raises(ValueError, match="unknown gen app"):
        gen.get_app("nosuch")


def test_top_level_exports_match_go_surface():
    for name in [
        "ChainHeadResult", "SimulateTxResult", "ViewResult",
        "GetResourceResult", "GetBlockByHeightResult", "GetTxByHashResult",
        "GetAccountResult", "EventsByTxHashResult",
        "GetResourcePathByHashResult", "GetAccessValueResult",
        "BatchGetResourcePathByHashResult", "GetTxHistoryProofResult",
        "TxHistorySignature", "RpcClientImpl",
        "RequestOption", "WaitOption",
        "with_request_id", "with_context",
        "with_wait_request_id", "with_wait_context",
        "with_wait_poll_period", "with_wait_poll_timeout",
        "apply_request_options", "apply_wait_options",
    ]:
        assert hasattr(m, name), name


def test_request_options_merging():
    o = m.apply_request_options([m.with_request_id(7), m.with_context("ctx-obj")])
    assert o == {"request_id": 7, "ctx": "ctx-obj"}
    w = m.apply_wait_options(
        [m.with_wait_poll_period(0.5)], default_period=1.0, default_timeout=30.0
    )
    assert w == {"poll_period": 0.5, "poll_timeout": 30.0}


def test_compat_aliases():
    from milon_sdk.types import new_bitmap64
    from milon_sdk.postcard import (
        Unmarshaler,
        SerializerFunc,
        new_serializer_with_cap,
    )
    from milon_sdk.api import TypeTagWithDataResolver
    from milon_sdk.crypto import (
        PublicKeyBytesFnDsa512,
        SecretKeyBytesFnDsa512,
        SignatureBytesFnDsa512,
    )
    from milon_sdk.lib import (
        RequestID,
        RpcResponseStatus,
        RpcResponseStatusOk,
        TransactionStamp,
    )

    assert new_bitmap64(9).raw() == 9
    assert new_serializer_with_cap().bytes() == b""
    assert RequestID is int and TransactionStamp is int
    assert RpcResponseStatusOk == 0
    assert SecretKeyBytesFnDsa512 is bytes
