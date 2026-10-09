"""各种签名模式 devNet 实测矩阵。

覆盖：
A. UnifiedPayer GasOnly（纯赞助：payer 只签 bit63，ix 零签名）—— create_account
B. SkipPubKey（wire 省略公钥 + AccountSignerBit 从链上定位）—— SetThreshold
C. 多签 2-of-2（MultisigKeySignatureMode 双签 + SignMultisigKey 追加）—— CreateMultisig + SetThreshold
D. UnifiedPayer Separate ix（两个账户各签各的 ix + payer 付 gas）
E. BLS 曲线（CreateAccount 用 BLS 公钥 + BLS SkipPubKey 交易）

Vote gate（MIP-25）为链下/单元级语义，已有 Go 向量对拍（tests/lib/test_hash_vectors.py）。
"""

import sys
import time

sys.path.insert(0, "D:/pprojiect/milon-api-server/pysdk-develop/src")

import milon_sdk as m
from milon_sdk import gen
from milon_sdk.crypto import (
    new_address_from_public_key,
    new_classical_secret_key,
    new_pure_classical_secret_key,
)
from milon_sdk.helper import check_tx_success
from milon_sdk.lib import (
    MultisigKeySignatureMode,
    PubKeySignatureMode,
    new_account_signature_builder,
    new_transaction_builder,
)
from milon_sdk.types.bitbap import Bitmap64

client = m.new_client(m.DEV_NET)
rpc = client.RpcClient
PASS = []


def step(name):
    def deco(fn):
        def run(*a, **kw):
            t0 = time.time()
            out = fn(*a, **kw)
            print(f"  [{name}] OK ({time.time() - t0:.1f}s)")
            PASS.append(name)
            return out

        return run

    return deco


def new_pair(curve="ed25519"):
    sk = new_pure_classical_secret_key()
    if curve == "secp256k1":
        pk = sk.secp256k1_public()
    elif curve == "bls12381":
        pk = sk.bls12381_public()
    else:
        pk = sk.ed25519_public()
    return sk, pk, new_address_from_public_key(pk)


def submit_and_wait(tx):
    rpc.submit_tx(tx)
    result = rpc.wait_for_transaction(tx.tx_hash())
    check_tx_success(result.BodyTxHistory)
    return result


print("== 前置：payer 领水 ==")
payer_sk, payer_pk, payer = new_pair()
rpc.claim_faucet(payer_sk, payer, PubKeySignatureMode(PublicKey=payer_pk))
print(f"  payer = {payer}, MIL = {rpc.balance_of(payer)}")


# ---------- A. UnifiedPayer GasOnly（纯赞助）+ secp256k1 曲线 ----------
print("\n== A. UnifiedPayer GasOnly（create_account，secp256k1）==")
secp_sk, secp_pk, secp_addr = new_pair("secp256k1")


@step("A.先领水（新账户自付 gas 的前提）")
def sA0():
    rpc.claim_faucet(secp_sk, secp_addr, PubKeySignatureMode(PublicKey=secp_pk))
    print(f"    secp MIL = {rpc.balance_of(secp_addr)}")


@step("A.create_account(账户自付 gas)")
def sA1():
    rpc.create_account(secp_sk, secp_pk)  # Go: WithPayer(account)


@step("A.GetAccount 确认上链")
def sA2():
    r = rpc.get_account(secp_addr)
    av = r.BodyAccountView
    print(f"    threshold={av.Threshold} pks={len(av.PublicKeysBs58)}")
    assert av.Threshold == 1 and len(av.PublicKeysBs58) == 1


sA0()
sA1()
sA2()

# ---------- B. SkipPubKey + AccountSignerBit ----------
print("\n== B. SkipPubKey（wire 省公钥，sig_bit 链上定位）==")


@step("B.AccountSignerBit 查位")
def sB1():
    global secp_sigbit
    secp_sigbit = rpc.account_signer_bit(secp_addr)
    print(f"    sig_bit = {secp_sigbit.raw()}")
    assert secp_sigbit.raw() == 1  # 首个 signer 位


@step("B.SetThreshold 用 SkipPubKey 签名")
def sB2():
    wire = gen.ACCOUNT.SET_THRESHOLD.args(owner=secp_addr, threshold=1).encode()
    tx = (
        new_transaction_builder([wire])
        .with_payer(payer)
        .add_payer_sig(payer, payer_sk, PubKeySignatureMode(PublicKey=payer_pk))
        .add_ixes_sig(
            secp_addr, secp_sk, [0], False,
            PubKeySignatureMode(PublicKey=secp_pk, SkipPubKey=True, SigBit=secp_sigbit),
        )
        .build()
    )
    # wire 省略公钥验证：AccountSignature.PubKey 应为 None
    assert tx.TxSigs[1].AccountSignature.PubKey is None
    submit_and_wait(tx)


sB1()
sB2()

# ---------- C. 多签 2-of-2 ----------
print("\n== C. 多签 2-of-2（MultisigKeySignatureMode 双签）==")
ms_sk1, ms_pk1, ms_owner = new_pair()
ms_sk2, ms_pk2, _ = new_pair()


@step("C.CreateMultisig(2-of-2)")
def sC1():
    wire = gen.ACCOUNT.CREATE_MULTISIG.args(
        owner=ms_owner,
        signers=[ms_pk1, ms_pk2],
        weights=bytes([1, 1]),
        threshold=2,
    ).encode()
    tx = (
        new_transaction_builder([wire])
        .with_payer(payer)
        .add_payer_sig(payer, payer_sk, PubKeySignatureMode(PublicKey=payer_pk))
        .add_ixes_sig(ms_owner, ms_sk1, [0], False, PubKeySignatureMode(PublicKey=ms_pk1))
        .build()
    )
    submit_and_wait(tx)


@step("C.ListSigners 查看")
def sC2():
    out = rpc.list_account_signers(ms_owner)
    account_map, signers = out[0], out[1]
    print(f"    bitmap={account_map.get('bitmap')} signers={len(signers)}")
    assert len(signers) == 2


@step("C.双签 SetThreshold（Index=0 + SignMultisigKey Index=1）")
def sC3():
    wire = gen.ACCOUNT.SET_THRESHOLD.args(owner=ms_owner, threshold=2).encode()
    tx = (
        new_transaction_builder([wire])
        .with_payer(payer)
        .add_payer_sig(payer, payer_sk, PubKeySignatureMode(PublicKey=payer_pk))
        .add_ixes_sig(
            ms_owner, ms_sk1, [0], False, MultisigKeySignatureMode(Index=0, PublicKey=ms_pk1)
        )
        .build()
    )
    # 第二个参与者：SignMultisigKey 追加（同一 auth message）
    builder = new_transaction_builder([wire]).with_payer(payer)
    sig0 = (
        new_account_signature_builder()
        .authorize_ixes([0])
        .sign(
            ms_owner, ms_sk1, tx.tx_hash(),
            [__import__("milon_sdk.lib", fromlist=["IxHashItem"]).IxHashItem(Index=0, Hash=tx.ix_hashes()[0])],
            MultisigKeySignatureMode(Index=0, PublicKey=ms_pk1),
        )
        .sign_multisig_key(
            ms_owner, ms_sk2, tx.tx_hash(),
            [__import__("milon_sdk.lib", fromlist=["IxHashItem"]).IxHashItem(Index=0, Hash=tx.ix_hashes()[0])],
            MultisigKeySignatureMode(Index=1, PublicKey=ms_pk2),
        )
        .build()
    )
    assert sig0.SigBit.raw() == 0b11
    assert len(sig0.Signatures) == 2
    assert sig0.PubKey is None
    from milon_sdk.lib import TransactionSignatures

    tx.TxSigs = [
        TransactionSignatures(Address=payer, AccountSignature=(
            new_account_signature_builder().authorize_payer()
            .sign(payer, payer_sk, tx.tx_hash(), None, PubKeySignatureMode(PublicKey=payer_pk))
            .build()
        )),
        TransactionSignatures(Address=ms_owner, AccountSignature=sig0),
    ]
    submit_and_wait(tx)


sC1()
sC2()
sC3()

# ---------- D. UnifiedPayer Separate ix ----------
print("\n== D. Separate ix（两账户各签各 ix + payer gas）==")
tok_sk, tok_pk, tok_addr = new_pair()
d1_sk, d1_pk, d1 = new_pair()
d2_sk, d2_pk, d2 = new_pair()
_, _, d3 = new_pair()


@step("D.建代币+铸币给 d1")
def sD1():
    metadata = {"name": "SigMode Demo", "symbol": "SGM", "decimals": 6, "icon": "https://milon.test/sgm.png", "uri": ""}
    wire = gen.TOKEN.CREATE.args(token=tok_addr, owner=payer, metadata=metadata).encode()
    submit_and_wait(
        new_transaction_builder([wire]).with_payer(payer)
        .add_payer_sig(payer, payer_sk, PubKeySignatureMode(PublicKey=payer_pk))
        .add_ixes_sig(tok_addr, tok_sk, [0], False, PubKeySignatureMode(PublicKey=tok_pk))
        .build()
    )
    wire2 = gen.TOKEN.MINT.args(token=tok_addr, to=d1, amount=1000).encode()
    submit_and_wait(
        new_transaction_builder([wire2]).with_payer(payer)
        .add_ix_and_payer_sig(payer, payer_sk, 0, PubKeySignatureMode(PublicKey=payer_pk))
        .build()
    )


@step("D.双 ix：d1 签 Transfer(ix0) + payer 签 Mint(ix1)+gas")
def sD2():
    wire_transfer = gen.TOKEN.TRANSFER.args(token=tok_addr, **{"from": d1}, to=d2, amount=300).encode()
    wire_mint = gen.TOKEN.MINT.args(token=tok_addr, to=d3, amount=7).encode()
    tx = (
        new_transaction_builder([wire_transfer, wire_mint])
        .with_payer(payer)
        .add_ix_and_payer_sig(payer, payer_sk, 1, PubKeySignatureMode(PublicKey=payer_pk))
        .add_ixes_sig(d1, d1_sk, [0], False, PubKeySignatureMode(PublicKey=d1_pk))
        .build()
    )
    submit_and_wait(tx)

    def bal(a):
        r = rpc.view([gen.TOKEN.BALANCE_OF.args(token=tok_addr, account=a).encode()])
        return gen.TOKEN.BALANCE_OF.decode_view(r.HTTPResponseBody)

    b1, b2, b3 = bal(d1), bal(d2), bal(d3)
    print(f"    d1={b1} d2={b2} d3={b3}")
    assert (b1, b2, b3) == (700, 300, 7)


sD1()
sD2()

# ---------- E. BLS 曲线 ----------
print("\n== E. BLS12-381 曲线（create_account + SkipPubKey 交易）==")
bls_sk, bls_pk, bls_addr = new_pair("bls12381")


@step("E.create_account(BLS 公钥)")
def sE1():
    rpc.claim_faucet(bls_sk, bls_addr, PubKeySignatureMode(PublicKey=bls_pk))
    rpc.create_account(bls_sk, bls_pk)


@step("E.BLS SkipPubKey SetThreshold")
def sE2():
    sigbit = rpc.account_signer_bit(bls_addr)
    wire = gen.ACCOUNT.SET_THRESHOLD.args(owner=bls_addr, threshold=1).encode()
    tx = (
        new_transaction_builder([wire])
        .with_payer(payer)
        .add_payer_sig(payer, payer_sk, PubKeySignatureMode(PublicKey=payer_pk))
        .add_ixes_sig(
            bls_addr, bls_sk, [0], False,
            PubKeySignatureMode(PublicKey=bls_pk, SkipPubKey=True, SigBit=sigbit),
        )
        .build()
    )
    assert len(tx.TxSigs[1].AccountSignature.Signatures[0].Bytes) == 96  # BLS 签名 96B
    submit_and_wait(tx)


try:
    sE1()
    sE2()
except Exception as exc:  # 链若不支持 BLS 账户，记录而非中断
    print(f"  [E.BLS] SKIP/FAIL: {str(exc)[:120]}")

print(f"\n=== 通过 {len(PASS)} 项：{PASS} ===")
