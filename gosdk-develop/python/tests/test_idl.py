"""IDL 编解码测试：指令 wire round-trip、视图解码、动态 gen 绑定。"""
from __future__ import annotations

import os
import sys

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from milon_sdk import gen  # noqa: E402
from milon_sdk.api import base as api_base  # noqa: E402
from milon_sdk.crypto import Address, ClassicalSecretKey  # noqa: E402


@pytest.fixture(scope="module")
def bindings():
    return gen.default_bindings()


def test_apps_loaded(bindings) -> None:
    apps = sorted(bindings.__dir__())
    assert "token" in apps and "account" in apps and "system" in apps


def test_balance_of_encode_decode_roundtrip(bindings) -> None:
    addr = Address.from_public_key(ClassicalSecretKey.generate().ed25519_public())
    wire = bindings.token.BalanceOf.args(api_base.MIL_TOKEN, addr).encode()
    # wire = app_id(1B) + discriminator u16 LE(2B) + 20B + 20B
    assert wire[0] == 2  # token app_id
    assert len(wire) == 3 + 20 + 20

    decoded = bindings.token.BalanceOf.decode(wire)
    assert decoded["token"] == api_base.MIL_TOKEN
    assert decoded["account"] == addr


def test_create_encode_decode_roundtrip(bindings) -> None:
    pk = ClassicalSecretKey.generate().ed25519_public()
    wire = bindings.account.Create.args(pk).encode()
    assert wire[0] == 1  # account app_id
    decoded = bindings.account.Create.decode(wire)
    assert decoded["owner_pk"] == pk


def test_view_decoding(bindings) -> None:
    """构造一个 view 返回体（Vec<Result<u64>>）并解码。"""
    # Ok(12345) 单结果：result_count(1) + variant(0) + len + value
    from milon_sdk.provider.provider import Provider
    from milon_sdk.provider.loader import load_default_idls

    pd = Provider(next(idl for idl in load_default_idls() if idl.metadata.name == "token"))
    body = _build_view_body(12345)
    values = pd.decode_view_datas("BalanceOf", body)
    assert len(values) == 1
    assert values[0].value == 12345


def test_view_decoding_failure(bindings) -> None:
    """Err(TxFailurePayload) 变体解码。"""
    from milon_sdk.provider.provider import Provider
    from milon_sdk.provider.loader import load_default_idls

    pd = Provider(next(idl for idl in load_default_idls() if idl.metadata.name == "token"))
    body = _build_view_body_failure(99, "insufficient balance")
    values = pd.decode_view_datas("BalanceOf", body)
    failure = values[0].value
    assert failure["code"] == 99
    assert failure["message"] == "insufficient balance"


def test_instruction_wire_structure(bindings) -> None:
    """app_id + discriminator(u16 LE) 前缀字节级校验。"""
    addr = Address.from_public_key(ClassicalSecretKey.generate().ed25519_public())
    wire = bindings.token.BalanceOf.args(api_base.MIL_TOKEN, addr).encode()
    ins = bindings.token.BalanceOf._instruction
    disc = ins.discriminator
    assert wire[1] == (disc & 0xFF)
    assert wire[2] == (disc >> 8) & 0xFF


def _view_serialize_u64(value: int) -> bytes:
    out = bytearray()
    while value >= 0x80:
        out.append((value & 0x7F) | 0x80)
        value >>= 7
    out.append(value)
    return bytes(out)


def _build_view_body(ok_value: int) -> bytes:
    body = bytearray()
    body += _view_serialize_u64(1)  # result_count
    body += _view_serialize_u64(0)  # Ok variant
    ok_data = _view_serialize_u64(ok_value)
    body += _view_serialize_u64(len(ok_data))
    body += ok_data
    return bytes(body)


def _build_view_body_failure(code: int, message: str) -> bytes:
    body = bytearray()
    body += _view_serialize_u64(1)  # result_count
    body += _view_serialize_u64(1)  # Err variant
    body += _view_serialize_u64(code)
    msg = message.encode()
    body += _view_serialize_u64(len(msg))
    body += msg
    body += _view_serialize_u64(0)  # data 长度 0
    return bytes(body)
