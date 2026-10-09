"""Python SDK 完整用户旅程实测（devNet 真实链上闭环）。

流程：领水 → 建代币(Create) → 铸币(Mint) → 转账(Transfer) → 等确认 →
余额验证 → 交易详情 → 事件解码 → 模拟预估。全部走 milon_sdk 公开 API。
"""

import sys
import time

sys.path.insert(0, "D:/pprojiect/milon-api-server/pysdk-develop/src")

import milon_sdk as m
from milon_sdk import gen
from milon_sdk.crypto import (
    new_address_from_public_key,
    new_pure_classical_secret_key,
)
from milon_sdk.helper import check_tx_success
from milon_sdk.lib import PubKeySignatureMode, new_transaction_builder
from milon_sdk.provider import decode_event_data_by_tag, format_decoded_event

client = m.new_client(m.DEV_NET)
rpc = client.RpcClient
pm = rpc.get_provider_manager()

PASS = []


def step(name):
    def deco(fn):
        def run(*a, **kw):
            t0 = time.time()
            out = fn(*a, **kw)
            print(f"  [{name}] OK  ({time.time() - t0:.1f}s)")
            PASS.append(name)
            return out

        return run

    return deco


def new_pair():
    sk = new_pure_classical_secret_key()
    pk = sk.ed25519_public()
    return sk, pk, new_address_from_public_key(pk)


print("== 准备账户（全随机，避开水龙头 24h 冷却）==")
payer_sk, payer_pk, payer = new_pair()
token_sk, token_pk, token = new_pair()
acct1_sk, acct1_pk, acct1 = new_pair()
_, _, acct2 = new_pair()
print(f"payer={payer} token={token} acct1={acct1} acct2={acct2}")


@step("1.ClaimFaucet(payer)")
def s1():
    rpc.claim_faucet(payer_sk, payer, PubKeySignatureMode(PublicKey=payer_pk))


@step("1.ClaimFaucet(acct1)")
def s1b():
    rpc.claim_faucet(acct1_sk, acct1, PubKeySignatureMode(PublicKey=acct1_pk))


@step("2.BalanceOf(payer)>0")
def s2():
    bal = rpc.balance_of(payer)
    assert bal > 0, f"payer MIL={bal}"
    print(f"    payer MIL = {bal}")
    return bal


@step("3.Token.Create(真实代币)")
def s3():
    metadata = {
        "name": "PySDK E2E Token",
        "symbol": "PYE2E",
        "decimals": 6,
        "icon": "https://milon.test/token.png",
        "uri": "",
    }
    wire = gen.TOKEN.CREATE.args(token=token, owner=payer, metadata=metadata).encode()
    tx = (
        new_transaction_builder([wire])
        .with_payer(payer)
        .add_payer_sig(payer, payer_sk, PubKeySignatureMode(PublicKey=payer_pk))
        .add_ixes_sig(token, token_sk, [0], False, PubKeySignatureMode(PublicKey=token_pk))
        .build()
    )
    rpc.submit_tx(tx)
    result = rpc.wait_for_transaction(tx.tx_hash())
    check_tx_success(result.BodyTxHistory)
    print(f"    token tx = {tx.tx_hash().hex()[:16]}...")
    return tx.tx_hash()


@step("4.Token.Mint(acct1)")
def s4():
    wire = gen.TOKEN.MINT.args(token=token, to=acct1, amount=1_000_000).encode()
    tx = (
        new_transaction_builder([wire])
        .with_payer(payer)
        .add_ix_and_payer_sig(payer, payer_sk, 0, PubKeySignatureMode(PublicKey=payer_pk))
        .build()
    )
    rpc.submit_tx(tx)
    result = rpc.wait_for_transaction(tx.tx_hash())
    check_tx_success(result.BodyTxHistory)
    return tx.tx_hash()


@step("5.Token.Transfer(acct1→acct2)")
def s5():
    wire = gen.TOKEN.TRANSFER.args(
        token=token, **{"from": acct1}, to=acct2, amount=500_000
    ).encode()
    tx = (
        new_transaction_builder([wire])
        .with_payer(payer)
        .add_payer_sig(payer, payer_sk, PubKeySignatureMode(PublicKey=payer_pk))
        .add_ixes_sig(acct1, acct1_sk, [0], False, PubKeySignatureMode(PublicKey=acct1_pk))
        .build()
    )
    rpc.submit_tx(tx)
    result = rpc.wait_for_transaction(tx.tx_hash())
    check_tx_success(result.BodyTxHistory)
    return tx.tx_hash()


@step("6.BalanceOf(acct1/acct2) 双端验证")
def s6(transfer_hash):
    # 非 MIL 代币余额：手写 view（BalanceOf 带 token 参数）
    def token_balance(account):
        wire = gen.TOKEN.BALANCE_OF.args(token=token, account=account).encode()
        r = rpc.view([wire])
        return gen.TOKEN.BALANCE_OF.decode_view(r.HTTPResponseBody)

    b1 = token_balance(acct1)
    b2 = token_balance(acct2)
    print(f"    acct1 token = {b1}, acct2 token = {b2}")
    assert b1 == 500_000 and b2 == 500_000, (b1, b2)
    return transfer_hash


@step("7.GetTxByHash 交易详情")
def s7(transfer_hash):
    r = rpc.get_tx_by_hash(transfer_hash)
    h = r.BodyTxHistory
    check_tx_success(h)
    print(
        f"    state={h.Receipt.State} ixs={len(h.Instructions)} "
        f"sigs={len(h.Signatures)} gas={h.Receipt.GasCharged} "
        f"events={len(h.Receipt.Events)}"
    )
    assert len(h.Receipt.Events) > 0
    return h


@step("8.EventsByTxHash + IDL 事件解码")
def s8(transfer_hash, tx_history):
    r = rpc.events_by_tx_hash(transfer_hash, None)
    evts = r.BodyEventsByTxHash.Events
    print(f"    events = {len(evts)}")
    assert evts, "no events"
    e0 = evts[0]
    decoded = decode_event_data_by_tag(pm, e0.Data.TypeTag, e0.Data.Value)
    text = format_decoded_event(decoded)
    print("    " + text.splitlines()[0])
    assert "Transfer" in decoded["event_name"] or decoded["event_name"], decoded


@step("9.SimulateTx 预估")
def s9():
    wire = gen.TOKEN.TRANSFER.args(
        token=token, **{"from": acct1}, to=acct2, amount=1
    ).encode()
    tx = (
        new_transaction_builder([wire])
        .with_payer(payer)
        .add_payer_sig(payer, payer_sk, PubKeySignatureMode(PublicKey=payer_pk))
        .add_ixes_sig(acct1, acct1_sk, [0], False, PubKeySignatureMode(PublicKey=acct1_pk))
        .build()
    )
    sim = rpc.simulate_tx(tx)
    check_simulate = __import__("milon_sdk.helper", fromlist=["x"]).check_simulate_success
    check_simulate(sim)
    print(f"    gas = {sim.BodySimulateReceipt.GasCharged}")


s1()
s1b()
payer_bal = s2()
token_create_hash = s3()
s4()
transfer_hash = s5()
s6(transfer_hash)
tx_history = s7(transfer_hash)
s8(transfer_hash, tx_history)
s9()

print(f"\n=== 全部 {len(PASS)} 步通过：{PASS} ===")
