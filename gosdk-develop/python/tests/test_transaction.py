"""交易层测试：哈希确定性、wire round-trip、4 种付款模式、校验。"""
from __future__ import annotations

import os
import sys

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from milon_sdk import gen  # noqa: E402
from milon_sdk.api import base as api_base  # noqa: E402
from milon_sdk.crypto import Address, ClassicalSecretKey  # noqa: E402
from milon_sdk.postcard import Deserializer  # noqa: E402
from milon_sdk.lib.account_signature import (  # noqa: E402
    AUTH_PAYER_BIT,
    AccountSignature,
    IxHashItem,
    MultisigKeySignatureMode,
    PubKeySignatureMode,
    auth_ixes,
    auth_payer,
    sign,
)
from milon_sdk.lib.chain import get_chain_id, set_chain_id  # noqa: E402
from milon_sdk.lib.transaction import Transaction, validate_wire, validate_wire_with  # noqa: E402
from milon_sdk.lib.transaction_builder import SigningSlot, Signer, TransactionBuilder  # noqa: E402


@pytest.fixture()
def ctx():
    bindings = gen.default_bindings()
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    addr = Address.from_public_key(pk)
    wire = bindings.token.BalanceOf.args(api_base.MIL_TOKEN, addr).encode()
    return {"bindings": bindings, "sk": sk, "pk": pk, "addr": addr, "wire": wire}


def test_tx_hash_deterministic(ctx) -> None:
    tx1 = TransactionBuilder([ctx["wire"]]).with_stamp(1000).add_ixes_sig(
        ctx["addr"], ctx["sk"], [0], False, PubKeySignatureMode(public_key=ctx["pk"])
    ).build()
    tx2 = TransactionBuilder([ctx["wire"]]).with_stamp(1000).add_ixes_sig(
        ctx["addr"], ctx["sk"], [0], False, PubKeySignatureMode(public_key=ctx["pk"])
    ).build()
    assert tx1.tx_hash() == tx2.tx_hash()
    assert tx1.to_bytes() == tx2.to_bytes()


def test_tx_hash_changes_with_stamp(ctx) -> None:
    tx1 = TransactionBuilder([ctx["wire"]]).with_stamp(1000).build()
    tx2 = TransactionBuilder([ctx["wire"]]).with_stamp(1001).build()
    assert tx1.tx_hash() != tx2.tx_hash()


def test_tx_hash_changes_with_payer(ctx) -> None:
    sk2 = ClassicalSecretKey.generate()
    addr2 = Address.from_public_key(sk2.ed25519_public())
    tx1 = TransactionBuilder([ctx["wire"]]).with_stamp(1000).build()
    tx2 = TransactionBuilder([ctx["wire"]]).with_stamp(1000).with_payer(addr2).build()
    assert tx1.tx_hash() != tx2.tx_hash()


def test_wire_roundtrip(ctx) -> None:
    tx = (
        TransactionBuilder([ctx["wire"]])
        .with_stamp(1700000000000)
        .add_ixes_sig(ctx["addr"], ctx["sk"], [0], True, PubKeySignatureMode(public_key=ctx["pk"]))
        .build()
    )
    raw = tx.to_bytes()
    parsed = Transaction.unmarshal_postcard(Deserializer(raw))
    assert parsed.stamp == tx.stamp
    assert parsed.payer == tx.payer
    assert parsed.instructions == tx.instructions
    assert len(parsed.tx_sigs) == len(tx.tx_sigs)
    assert parsed.tx_sigs[0].address == tx.tx_sigs[0].address
    assert parsed.tx_sigs[0].account_signature.auth_bit == tx.tx_sigs[0].account_signature.auth_bit
    assert parsed.tx_sigs[0].account_signature.sig_bit == tx.tx_sigs[0].account_signature.sig_bit
    assert parsed.tx_sigs[0].account_signature.signatures == tx.tx_sigs[0].account_signature.signatures
    assert parsed.tx_sigs[0].account_signature.pub_key == tx.tx_sigs[0].account_signature.pub_key


def test_split_payer_self_pay_signs_ix_and_gas(ctx) -> None:
    """SplitPayerSelfPay：执行者需同时授权 bit63(gas) 与该 ix。"""
    tx = (
        TransactionBuilder([ctx["wire"]])
        .with_stamp(123)
        .add_ixes_sig(ctx["addr"], ctx["sk"], [0], True, PubKeySignatureMode(public_key=ctx["pk"]))
        .build()
    )
    validate_wire(tx)  # payer=None + bit0+bit63 → OK
    sig = tx.tx_sigs[0].account_signature
    assert sig.auth_bit.test(0)
    assert sig.auth_bit.test(AUTH_PAYER_BIT)


def test_unified_payer_mode(ctx) -> None:
    """UnifiedPayer：payer 只签 bit63。"""
    tx = (
        TransactionBuilder([ctx["wire"]])
        .with_payer(ctx["addr"])
        .with_stamp(123)
        .add_payer_sig(ctx["addr"], ctx["sk"], PubKeySignatureMode(public_key=ctx["pk"]))
        .build()
    )
    validate_wire(tx)
    sig = tx.tx_sigs[0].account_signature
    assert sig.auth_bit.test(AUTH_PAYER_BIT)


def test_validate_wire_missing_payer_sig(ctx) -> None:
    """UnifiedPayer 模式缺 payer 签名 → 校验失败。"""
    tx = (
        TransactionBuilder([ctx["wire"]])
        .with_payer(ctx["addr"])
        .with_stamp(123)
        .add_ixes_sig(ctx["addr"], ctx["sk"], [0], False, PubKeySignatureMode(public_key=ctx["pk"]))
        .build()
    )
    with pytest.raises(ValueError, match="payer signature required"):
        validate_wire(tx)


def test_validate_wire_sponsor_ixes(ctx) -> None:
    """赞助 ix 可跳过 gas 签名校验。"""
    tx = (
        TransactionBuilder([ctx["wire"]])
        .with_stamp(123)
        .add_ixes_sig(ctx["addr"], ctx["sk"], [0], False, PubKeySignatureMode(public_key=ctx["pk"]))
        .build()
    )
    # 无赞助：payer 签名缺失 → 失败
    with pytest.raises(ValueError, match="gas signer required"):
        validate_wire(tx)
    # 有赞助 [0]：通过
    validate_wire_with(tx, [0])


def test_sign_with_slots(ctx) -> None:
    tx = (
        TransactionBuilder([ctx["wire"]])
        .with_stamp(123)
        .apply_slots(
            [SigningSlot(address=ctx["addr"], instruction_indices=[0], include_payer=True,
                         mode=PubKeySignatureMode(public_key=ctx["pk"]))]
        )
        .sign_with(Signer(secret_key=ctx["sk"], public_key=ctx["pk"]))
        .build()
    )
    assert len(tx.tx_sigs) == 1
    validate_wire(tx)


def test_simulate_sign_placeholder(ctx) -> None:
    """模拟签名：零填充但长度与真实一致。"""
    real_tx = (
        TransactionBuilder([ctx["wire"]])
        .with_stamp(123)
        .add_simulate_ixes_sig(ctx["addr"], [0], True, PubKeySignatureMode(public_key=ctx["pk"]))
        .build()
    )
    real_sig = real_tx.tx_sigs[0].account_signature.signatures[0]
    assert len(real_sig.as_bytes()) == 64  # ed25519
    assert real_sig.as_bytes() == b"\x00" * 64


def test_multisig_mode(ctx) -> None:
    """多签模式：sig_bit = 1<<index，线格式不含 pubkey。"""
    pk2 = ClassicalSecretKey.generate().ed25519_public()
    tx = (
        TransactionBuilder([ctx["wire"]])
        .with_stamp(123)
        .add_ixes_sig(ctx["addr"], ctx["sk"], [0], True, MultisigKeySignatureMode(index=2, public_key=pk2))
        .build()
    )
    sig = tx.tx_sigs[0].account_signature
    assert sig.sig_bit.raw() == (1 << 2)
    assert sig.pub_key is None  # 多签模式不携带公钥


def test_auth_message_endianness() -> None:
    """AuthMessage 中 auth_bit 为 LittleEndian（与 TxHash 的 BE 相反）。"""
    from milon_sdk.crypto.hashes import MILON_TX_AUTH_DOMAIN_CONTEXT, hasher
    from milon_sdk.lib.chain import get_chain_id

    import struct

    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    addr = Address.from_public_key(pk)
    bindings = gen.default_bindings()
    wire = bindings.token.BalanceOf.args(api_base.MIL_TOKEN, addr).encode()
    tx = TransactionBuilder([wire]).with_stamp(5).build()
    tx_hash = tx.tx_hash()
    ix_hash = tx.ix_hash_from_wire(wire)
    auth = auth_ixes([0])
    # 手工装配 auth message 验证端序
    h = hasher(MILON_TX_AUTH_DOMAIN_CONTEXT.encode())
    h.update(struct.pack(">Q", get_chain_id()))
    h.update(addr.as_bytes())
    h.update(struct.pack("<Q", auth.raw()))  # LE！
    h.update(tx_hash.as_bytes())
    h.update(ix_hash.as_bytes())
    expected = h.digest()
    # auth_message 计算值应与手工装配一致
    manual = AccountSignature(auth_bit=auth)
    msg = manual.auth_message(addr, tx_hash, [IxHashItem(index=0, hash=ix_hash)])
    assert msg.as_bytes() == expected
