"""批5 冒烟：new_client / gen typed builder / helper / resolve_resource_paths。"""

import pytest

import milon_sdk as m
from milon_sdk import Network, gen, new_client
from milon_sdk.helper import check_simulate_success, check_tx_success
from milon_sdk.api import TX_STATE_FAILED, TX_STATE_PENDING, TX_STATE_SUCCESS, TxHistory, TxReceipt
from milon_sdk.rpc_client_v1 import SimulateTxResult
from milon_sdk.api import SimulateReceipt


def test_new_client_devnet():
    c = new_client(m.DEV_NET)
    assert c.network.Name == "devNet"
    assert len(c.RpcClient.get_all_pd()) == 11
    assert c.RpcClient.get_provider_manager() is not None


def test_new_client_localnet_and_options():
    c = new_client(m.LOCAL_NET, m.with_client_poll_period(0.5), m.with_client_poll_timeout(10))
    assert c.RpcClient.pollPeriod == 0.5
    assert c.RpcClient.pollTimeout == 10


def test_network_constants():
    assert m.LOCAL_NET.ChainId == 900_000_001
    assert m.DEV_NET.RpcUrl == "https://devnet.milonlabs.com/v1/rpc"
    assert m.DEV_NET.GrpcAddr == "8.218.101.239:50051"


def test_gen_typed_builder_encode():
    addr = m.Address(bytes(range(1, 21)))
    wire = gen.TOKEN.CLAIM_FAUCET.args(claimer=addr).encode()
    # app_id=2 (token) + discriminator u16 LE
    assert wire[0] == 2
    assert len(wire) == 1 + 2 + 20


def test_gen_decode_roundtrip():
    addr = m.Address(bytes(range(1, 21)))
    inst = gen.TOKEN.CLAIM_FAUCET
    wire = inst.args(claimer=addr).encode()
    decoded = inst.decode(wire)
    assert decoded["claimer"].Bytes == addr.Bytes


def test_gen_attr_error_on_unknown():
    with pytest.raises(AttributeError, match="IDL method not found"):
        gen.TOKEN.NOSUCHTHING


def test_helper_check_tx_success():
    ok = TxHistory(Receipt=TxReceipt(State=TX_STATE_SUCCESS))
    check_tx_success(ok)
    with pytest.raises(RuntimeError, match="transaction failed on chain"):
        check_tx_success(TxHistory(Receipt=TxReceipt(State=TX_STATE_FAILED)))
    with pytest.raises(RuntimeError):
        check_tx_success(None)


def test_helper_check_simulate_success():
    ok = SimulateTxResult(BodySimulateReceipt=SimulateReceipt(State=TX_STATE_SUCCESS))
    check_simulate_success(ok)
    with pytest.raises(RuntimeError, match="simulate failed on chain"):
        check_simulate_success(
            SimulateTxResult(BodySimulateReceipt=SimulateReceipt(State=TX_STATE_FAILED))
        )


def test_gen_default_idls_match_registry():
    idls = gen.default_idls()
    assert set(idls.keys()) >= {"token", "account", "system", "demo"}
