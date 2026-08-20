"""评审修复的回归测试：helper 属性、ListResourcePathInfo、IDLRegistry 新方法、结果类蛇形别名。"""

from __future__ import annotations

import pytest

from milon_sdk import gen
from milon_sdk import helper
from milon_sdk.api import base as api_base
from milon_sdk.api.responses import (
    AccountView,
    EventEntry,
    EventsByTxHash,
    SimulateReceipt,
    TxFailurePayload,
    TxHistory,
    TxReceipt,
    unmarshal_list_resource_path_list_from_raw_list,
)
from milon_sdk.crypto import ClassicalSecretKey, Address
from milon_sdk.gen import DefaultIDLs, _build_providers
from milon_sdk.provider.registry import NewIDLRegistry
from milon_sdk.rpc import (
    EventsByTxHashResult,
    GetAccountResult,
    GetTxByHashResult,
    SimulateTxResult,
)


# ---------------------------------------------------------------- helper


def test_helper_check_tx_success_ok() -> None:
    result = GetTxByHashResult(b"", TxHistory(None, None, [], [], TxReceipt(None, None, 1, [], [], None, 0)))
    assert helper.check_tx_success(result) is result


def test_helper_check_tx_success_failed() -> None:
    result = GetTxByHashResult(b"", TxHistory(None, None, [], [], TxReceipt(None, None, 2, [], [], 500, 0)))
    with pytest.raises(RuntimeError, match="transaction not successful"):
        helper.check_tx_success(result)


def test_helper_check_simulate_success() -> None:
    ok = SimulateTxResult(b"", SimulateReceipt(None, None, 1, [], [], None, 0))
    assert helper.check_simulate_success(ok) is ok
    bad = SimulateTxResult(b"", SimulateReceipt(None, None, 2, [], [], TxFailurePayload(7, "boom", b""), 0))
    with pytest.raises(RuntimeError, match="simulation not successful"):
        helper.check_simulate_success(bad)


def test_helper_result_aliases() -> None:
    r = GetTxByHashResult(b"x", TxHistory(None, None, [], [], TxReceipt(None, None, 1, [], [], None, 0)))
    assert r.BodyTxHistory is r.body_tx_history
    assert r.HTTPResponseBody is r.http_response_body
    s = SimulateTxResult(b"x", SimulateReceipt(None, None, 1, [], [], None, 0))
    assert s.BodySimulateReceipt is s.body_simulate_receipt
    a = GetAccountResult(b"x", AccountView(None, 2, ["pk1"]))
    assert a.BodyAccountView is a.body_account_view
    assert a.body_account_view.threshold == 2
    e = EventsByTxHashResult(b"x", EventsByTxHash([EventEntry(1, None, 0, 0, None)]))
    assert e.BodyEventsByTxHash is e.body_events_by_tx_hash
    assert len(e.body_events_by_tx_hash.events) == 1


# ---------------------------------------------------------------- ListResourcePathInfo


def test_unmarshal_list_resource_path_list() -> None:
    raw = [[[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18], "/a/b"], [[9] * 18, "/c"]]
    infos = unmarshal_list_resource_path_list_from_raw_list(raw)
    assert len(infos) == 2
    assert infos[0].path == "/a/b"
    assert infos[1].rs_hash.to_hex().startswith("090909")


def test_unmarshal_list_resource_path_invalid() -> None:
    with pytest.raises(ValueError, match="invalid ListResourcePathInfo"):
        unmarshal_list_resource_path_list_from_raw_list([[[1], 123]])


# ---------------------------------------------------------------- IDLRegistry 新方法


@pytest.fixture(scope="module")
def registry():
    return NewIDLRegistry(_build_providers(DefaultIDLs))


def test_decode_instructions_and_format(registry) -> None:
    addr = Address.from_public_key(ClassicalSecretKey.generate().ed25519_public())
    wire = gen.Token.BalanceOf.Args(api_base.MIL_TOKEN, addr).Encode()
    decoded = registry.decode_instructions([wire])
    assert len(decoded) == 1
    assert decoded[0]["instruction_name"] == "BalanceOf"
    rendered = registry.format_decoded_instruction(decoded[0])
    assert rendered.startswith("[token] BalanceOf")
    assert "NamedToken {" in rendered
    assert "Address(" in rendered


def test_decode_event_data_by_tag_unknown(registry) -> None:
    with pytest.raises(ValueError, match="unknown type tag"):
        registry.decode_event_data_by_tag(123456789, b"\x00")
