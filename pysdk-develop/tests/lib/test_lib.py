"""Port of gosdk-develop/lib/test/*_test.go 核心用例。"""

import pytest

from milon_sdk.crypto import (
    Address,
    new_address_from_public_key,
    new_classical_secret_key,
    new_pure_classical_secret_key,
    new_signature_from_bytes,
)
from milon_sdk.lib import (
    AUTH_PAYER_BIT,
    AUTH_RESERVED_BIT,
    AccountSignature,
    AuthPayerBit,
    AuthReservedBit,
    AuthVoteBit,
    IxHashItem,
    MultisigKeySignatureMode,
    PubKeySignatureMode,
    RpcResponse,
    RpcResponseError,
    Transaction,
    TransactionSignatures,
    auth_ix,
    auth_ix_and_payer,
    auth_ixes,
    auth_payer,
    auth_vote_ixes,
    collect_ix_hashes,
    new_account_signature_builder,
    new_rpc_request,
    new_transaction_builder,
    set_chain_id,
    simulate_sign,
    sign,
    unsigned,
    vote_batch_hash,
    MethodType,
    Signer,
    SigningSlot,
)
from milon_sdk.postcard import Deserializer, Serializer
from milon_sdk.types.bitbap import Bitmap64

IX_1 = bytes([1] + list(range(31)))
IX_2 = bytes([2] + list(range(31)))
IX_3 = bytes([3] + list(range(31)))


def _classical_pair():
    sk = new_classical_secret_key()
    pk = sk.secp256k1_public()
    return sk, pk, new_address_from_public_key(pk)


def _ed_pair():
    sk = new_pure_classical_secret_key()
    pk = sk.ed25519_public()
    return sk, pk, new_address_from_public_key(pk)


def test_auth_ix():
    assert auth_ix(0).raw() == 1
    assert auth_ix(61).raw() == 1 << 61
    with pytest.raises(ValueError, match="ix index 62 out of range"):
        auth_ix(62)
    with pytest.raises(ValueError, match="ix index 63 out of range"):
        auth_ix(63)


def test_auth_ixes():
    assert auth_ixes([0, 2]).raw() == 0b101
    with pytest.raises(ValueError):
        auth_ixes([62])


def test_auth_payer():
    assert auth_payer().raw() == 1 << 63
    assert AuthPayerBit == 63
    assert AuthReservedBit == 62
    assert AuthVoteBit == 62


def test_auth_ix_and_payer():
    assert auth_ix_and_payer(0).raw() == (1 << 63) | 1
    with pytest.raises(ValueError):
        auth_ix_and_payer(62)


def test_auth_vote_ixes():
    b = auth_vote_ixes([0, 3])
    assert b.raw() == (1 << AuthVoteBit) | 0b1001


def test_auth_bit_constants():
    assert AUTH_PAYER_BIT == 63
    assert AUTH_RESERVED_BIT == 62


# ---- AccountSignature ----


def test_account_signature_add_multisig_key():
    sk, pk, addr = _classical_pair()
    sig = unsigned(auth_ix(0))
    real = sk.sign_secp256k1(b"m")
    sig.add_multisig_key(3, real)
    assert sig.SigBit.raw() == 1 << 3
    assert len(sig.Signatures) == 1
    with pytest.raises(ValueError, match="out of range"):
        sig.add_multisig_key(64, real)


def test_account_signature_authorizes():
    asig = AccountSignature(AuthBit=Bitmap64(1 | (1 << 63)))
    assert asig.authorizes_ix(0)
    assert not asig.authorizes_ix(1)
    assert asig.authorizes_payer()
    assert not asig.authorizes_vote()


def test_account_signature_auth_message():
    sk, pk, addr = _classical_pair()
    tx = Transaction(Stamp=5, Instructions=[IX_1, IX_2])
    tx_hash = tx.tx_hash()
    ix_hashes = tx.ix_hashes()

    asig = unsigned(auth_ix_and_payer(0))
    msg = asig.auth_message(addr, tx_hash, [IxHashItem(0, ix_hashes[0])])
    assert len(msg) == 32

    # 未授权 ix 报错
    with pytest.raises(ValueError, match="not authorized in auth_bit"):
        asig.auth_message(addr, tx_hash, [IxHashItem(1, ix_hashes[1])])

    # AuthMessageForTx 等价 CollectIxHashes + AuthMessage
    msg2 = asig.auth_message_for_tx(addr, tx_hash, ix_hashes)
    assert msg2 == msg


def test_account_signature_is_vote_gate_only():
    v = unsigned(auth_vote_ixes([1]))
    assert v.is_vote_gate_only()
    assert not unsigned(auth_ix(1)).is_vote_gate_only()
    with_pk = unsigned(auth_vote_ixes([1]))
    with_pk.PubKey = _classical_pair()[1]
    assert not with_pk.is_vote_gate_only()


def test_vote_batch_hash():
    tx = Transaction(Stamp=1, Instructions=[IX_1, IX_2, IX_3])
    hashes = tx.ix_hashes()
    h1 = vote_batch_hash(Bitmap64(0b011), hashes)
    h2 = vote_batch_hash(Bitmap64(0b011), hashes)
    assert h1 == h2
    assert h1 != vote_batch_hash(Bitmap64(0b001), hashes)
    # bit62/63 跳过
    assert vote_batch_hash(Bitmap64(0b011), hashes) == vote_batch_hash(
        Bitmap64(0b011 | (1 << 63) | (1 << 62)), hashes
    )


def test_collect_ix_hashes():
    tx = Transaction(Instructions=[IX_1, IX_2, IX_3])
    hashes = tx.ix_hashes()
    items = collect_ix_hashes(Bitmap64(0b101), hashes)
    assert [i.Index for i in items] == [0, 2]


# ---- Sign / SimulateSign ----


def test_sign_and_verify_roundtrip():
    sk, pk, addr = _classical_pair()
    tx = Transaction(Stamp=9, Instructions=[IX_1])
    tx_hash = tx.tx_hash()
    ix_hashes = collect_ix_hashes(auth_ix_and_payer(0), tx.ix_hashes())

    asig = sign(
        addr, sk, auth_ix_and_payer(0), tx_hash, ix_hashes,
        PubKeySignatureMode(PublicKey=pk),
    )
    assert asig.PubKey.Bytes == pk.Bytes
    assert asig.SigBit.raw() == 0
    assert len(asig.Signatures) == 1

    msg = asig.auth_message(addr, tx_hash, ix_hashes)
    asig.Signatures[0].verify(msg, asig.PubKey)


def test_sign_pubkey_mismatch():
    sk, pk, _ = _classical_pair()
    other_addr = new_address_from_public_key(_ed_pair()[1])
    with pytest.raises(ValueError, match="does not match owner address"):
        sign(
            other_addr, sk, auth_payer(), b"\x00" * 32, [],
            PubKeySignatureMode(PublicKey=pk),
        )


def test_sign_skip_pubkey_requires_sigbit():
    sk, pk, addr = _classical_pair()
    with pytest.raises(ValueError, match="SkipPubKey requires SigBit"):
        sign(
            addr, sk, auth_payer(), b"\x00" * 32, [],
            PubKeySignatureMode(PublicKey=pk, SkipPubKey=True),
        )
    asig = sign(
        addr, sk, auth_payer(), b"\x00" * 32, [],
        PubKeySignatureMode(PublicKey=pk, SkipPubKey=True, SigBit=Bitmap64(4)),
    )
    assert asig.PubKey is None
    assert asig.SigBit.raw() == 4


def test_sign_multisig_mode():
    sk, pk, addr = _classical_pair()
    asig = sign(
        addr, sk, auth_ix(0), b"\x11" * 32, [],
        MultisigKeySignatureMode(Index=5, PublicKey=pk),
    )
    assert asig.SigBit.raw() == 1 << 5
    assert asig.PubKey is None


def test_simulate_sign():
    sk, pk, addr = _classical_pair()
    asig = simulate_sign(addr, auth_ix_and_payer(0), PubKeySignatureMode(PublicKey=pk))
    assert len(asig.Signatures) == 1
    assert len(asig.Signatures[0].Bytes) == 65  # secp256k1 长度占位
    assert asig.Signatures[0].Bytes == b"\x00" * 65


# ---- Builder ----


def test_account_signature_builder_authorize_sign():
    sk, pk, addr = _classical_pair()
    tx = Transaction(Stamp=3, Instructions=[IX_1])
    sig = (
        new_account_signature_builder()
        .authorize_ix_and_payer(0)
        .sign(addr, sk, tx.tx_hash(), collect_ix_hashes(auth_ix_and_payer(0), tx.ix_hashes()), PubKeySignatureMode(PublicKey=pk))
        .build()
    )
    assert sig.AuthBit.raw() == auth_ix_and_payer(0).raw()
    assert len(sig.Signatures) == 1


def test_account_signature_builder_error_short_circuit():
    sk, pk, addr = _classical_pair()
    b = new_account_signature_builder().authorize_ixes([99])  # 越界
    with pytest.raises(ValueError, match="out of range"):
        b.build()


def test_account_signature_builder_simulate_sign():
    sk, pk, addr = _classical_pair()
    sig = (
        new_account_signature_builder()
        .authorize_payer()
        .simulate_sign(addr, PubKeySignatureMode(PublicKey=pk))
        .build()
    )
    assert sig.authorizes_payer()
    assert len(sig.Signatures) == 1


def test_account_signature_builder_multisig():
    sk, pk, addr = _classical_pair()
    tx = Transaction(Stamp=1, Instructions=[IX_1])
    txh = tx.tx_hash()
    ixh = collect_ix_hashes(auth_ix(0), tx.ix_hashes())
    # 首签必须走 multisig 模式（PubKey=None 才允许追加多签）
    sig = (
        new_account_signature_builder()
        .authorize_ix_and_payer(0)
        .sign(addr, sk, txh, ixh, MultisigKeySignatureMode(Index=1, PublicKey=pk))
        .sign_multisig_key(addr, sk, txh, ixh, MultisigKeySignatureMode(Index=2, PublicKey=pk))
        .build()
    )
    assert sig.SigBit.raw() == (1 << 1) | (1 << 2)
    assert len(sig.Signatures) == 2
    assert sig.PubKey is None

    # simulate 多签占位
    sig2 = (
        new_account_signature_builder()
        .authorize_ix_and_payer(0)
        .sign(addr, sk, txh, ixh, MultisigKeySignatureMode(Index=1, PublicKey=pk))
        .simulate_sign_multisig_key(MultisigKeySignatureMode(Index=2, PublicKey=pk))
        .build()
    )
    assert len(sig2.Signatures) == 2
    assert len(sig2.Signatures[1].Bytes) == 65


# ---- Transaction / ValidateWire ----


def test_transaction_tx_hash():
    set_chain_id(900_000_001)
    tx = Transaction(Stamp=7, Instructions=[IX_1])
    h = tx.tx_hash()
    assert len(h) == 32
    assert tx.tx_hash() == h  # 稳定

    tx2 = Transaction(Stamp=8, Instructions=[IX_1])
    assert tx2.tx_hash() != h


def test_transaction_ix_hashes():
    tx = Transaction(Instructions=[IX_1, IX_2])
    hashes = tx.ix_hashes()
    assert len(hashes) == 2
    assert hashes[0] == tx.ix_hash_from_wire(IX_1)


def test_transaction_serialize_roundtrip():
    sk, pk, addr = _classical_pair()
    tx = Transaction(
        Stamp=11,
        Payer=addr,
        Instructions=[IX_1, IX_2],
        TxSigs=[
            TransactionSignatures(
                Address=addr,
                AccountSignature=sign(
                    addr, sk, auth_ix_and_payer(0), b"\x22" * 32, [],
                    PubKeySignatureMode(PublicKey=pk),
                ),
            )
        ],
    )
    data = tx.to_bytes()
    back = Transaction()
    back.unmarshal_postcard(Deserializer(data))
    assert back.tx_hash() == tx.tx_hash()
    assert back.Payer.Bytes == addr.Bytes
    assert back.Instructions == [IX_1, IX_2]
    assert back.TxSigs[0].AccountSignature.AuthBit.raw() == tx.TxSigs[0].AccountSignature.AuthBit.raw()


def test_transaction_validate_wire_unified_payer():
    sk, pk, addr = _classical_pair()
    tx = Transaction(
        Payer=addr,
        Instructions=[IX_1],
        TxSigs=[
            TransactionSignatures(
                Address=addr,
                AccountSignature=unsigned(auth_ix_and_payer(0)),
            )
        ],
    )
    tx.validate_wire()

    # 无 payer 签名
    tx2 = Transaction(
        Payer=addr,
        Instructions=[IX_1],
        TxSigs=[TransactionSignatures(Address=addr, AccountSignature=unsigned(auth_ix(0)))],
    )
    with pytest.raises(ValueError, match="payer signature required"):
        tx2.validate_wire()


def test_transaction_validate_wire_split_payer_self_pay():
    sk, pk, addr = _classical_pair()
    tx = Transaction(
        Instructions=[IX_1],
        TxSigs=[
            TransactionSignatures(
                Address=addr,
                AccountSignature=unsigned(auth_ix_and_payer(0)),
            )
        ],
    )
    tx.validate_wire()

    # 只付 gas 不授权 ix → gas signer required
    tx2 = Transaction(
        Instructions=[IX_1],
        TxSigs=[TransactionSignatures(Address=addr, AccountSignature=unsigned(auth_payer()))],
    )
    with pytest.raises(ValueError, match="gas signer required for ix 0"):
        tx2.validate_wire()


def test_transaction_validate_wire_with_sponsor():
    sk, pk, addr = _classical_pair()
    tx = Transaction(
        Instructions=[IX_1, IX_2],
        TxSigs=[
            TransactionSignatures(Address=addr, AccountSignature=unsigned(auth_ix_and_payer(0)))
        ],
    )
    # ix1 sponsored → 无需 gas 签名
    tx.validate_wire_with([1])
    with pytest.raises(ValueError, match="gas signer required for ix 1"):
        tx.validate_wire()


def test_transaction_validate_wire_errors():
    with pytest.raises(ValueError, match="empty instructions"):
        Transaction(Instructions=[]).validate_wire()
    with pytest.raises(ValueError, match="too many instructions"):
        Transaction(Instructions=[bytes([i]) for i in range(63)]).validate_wire()
    sk, pk, addr = _classical_pair()
    # 重复 ix hash
    with pytest.raises(ValueError, match="duplicate ix hash"):
        Transaction(
            Instructions=[IX_1, IX_1],
            TxSigs=[TransactionSignatures(Address=addr, AccountSignature=unsigned(auth_ix_and_payer(0)))],
        ).validate_wire()
    # 重复 owner
    with pytest.raises(ValueError, match="duplicate signature owner"):
        Transaction(
            Payer=addr,
            Instructions=[IX_1],
            TxSigs=[
                TransactionSignatures(Address=addr, AccountSignature=unsigned(auth_ix_and_payer(0))),
                TransactionSignatures(Address=addr, AccountSignature=unsigned(auth_ix_and_payer(0))),
            ],
        ).validate_wire()
    # 空 auth bit
    with pytest.raises(ValueError, match="empty auth bit"):
        Transaction(
            Payer=addr,
            Instructions=[IX_1],
            TxSigs=[TransactionSignatures(Address=addr, AccountSignature=unsigned(Bitmap64(0)))],
        ).validate_wire()


def test_transaction_unified_payer_modes():
    sk, pk, addr = _classical_pair()
    # SignAll: payer 授权 bit63 + ix0
    tx = Transaction(
        Payer=addr,
        Instructions=[IX_1],
        TxSigs=[TransactionSignatures(Address=addr, AccountSignature=unsigned(auth_ix_and_payer(0)))],
    )
    tx.validate_wire()

    # Separate ix: 另一个账户授权 ix0（无 payer），payer 只授权 bit63 —— SplitPayer 下 payer-only 冲突
    other_addr = new_address_from_public_key(_ed_pair()[1])
    tx2 = Transaction(
        Instructions=[IX_1],
        TxSigs=[
            TransactionSignatures(Address=other_addr, AccountSignature=unsigned(auth_ix(0))),
            TransactionSignatures(Address=addr, AccountSignature=unsigned(auth_ix_and_payer(0))),
        ],
    )
    tx2.validate_wire()

    tx3 = Transaction(
        Instructions=[IX_1],
        TxSigs=[
            TransactionSignatures(Address=other_addr, AccountSignature=unsigned(auth_ix_and_payer(0))),
        ],
    )
    tx3.validate_wire()


# ---- RpcRequest / RpcResponse ----


def test_new_rpc_request_marshal():
    req = new_rpc_request(MethodType.SUBMIT_TX, 42, b"\x01\x02")
    ser = Serializer()
    req.marshal_postcard(ser)

    back = type(req)()
    back.unmarshal_postcard(Deserializer(ser.bytes()))
    assert back.Method == MethodType.SUBMIT_TX
    assert back.RequestId == 42
    assert back.Body == b"\x01\x02"


def test_rpc_response_marshal_postcard():
    rsp = RpcResponse(
        RequestId=7,
        Status=0,
        Body=b"\x0a",
        Error=None,
    )
    ser = Serializer()
    rsp.marshal_postcard(ser)
    back = RpcResponse()
    back.unmarshal_postcard(Deserializer(ser.bytes()))
    assert back.RequestId == 7
    assert back.Body == b"\x0a"
    assert back.Error is None

    rsp2 = RpcResponse(
        RequestId=8,
        Status=6,
        Body=b"",
        Error=RpcResponseError(Message="boom", Code=5, Data=b"\x01"),
    )
    ser2 = Serializer()
    rsp2.marshal_postcard(ser2)
    back2 = RpcResponse()
    back2.unmarshal_postcard(Deserializer(ser2.bytes()))
    assert back2.Error.Message == "boom"
    assert back2.Error.Code == 5
    assert back2.Error.Data == b"\x01"


def test_rpc_response_deserialize_errors():
    rsp = RpcResponse()
    with pytest.raises(ValueError, match="failed to deserialize Message"):
        rsp.unmarshal_postcard(Deserializer(b""))


# ---- TransactionBuilder ----


def test_transaction_builder_add_signature():
    sk, pk, addr = _classical_pair()
    tx = Transaction(Stamp=1, Instructions=[IX_1])
    asig = unsigned(auth_ix(0))
    tx.add_signature(addr, asig)
    assert len(tx.TxSigs) == 1


def test_transaction_builder_simulate_slots():
    sk, pk, addr = _classical_pair()
    b = new_transaction_builder([IX_1]).with_payer(addr).apply_slots(
        [SigningSlot(Address=addr, InstructionIndices=[0], IncludePayer=True, Mode=PubKeySignatureMode(PublicKey=pk))]
    ).simulate_slots()
    tx = b.build()
    tx.validate_wire()
    assert len(tx.TxSigs) == 1


def test_transaction_builder_simulate_then_sign():
    sk, pk, addr = _classical_pair()
    b = new_transaction_builder([IX_1]).with_payer(addr).apply_slots(
        [SigningSlot(Address=addr, InstructionIndices=[0], IncludePayer=True, Mode=PubKeySignatureMode(PublicKey=pk))]
    )
    b.simulate_slots()
    b.reset_sigs()
    b.sign_with(Signer(SecretKey=sk, PublicKey=pk))
    tx = b.build()
    tx.validate_wire()
    # 真实签名非全零
    assert tx.TxSigs[0].AccountSignature.Signatures[0].Bytes != b"\x00" * 65


def test_transaction_builder_sign_with_error():
    sk, pk, addr = _classical_pair()
    other = new_pure_classical_secret_key()
    b = new_transaction_builder([IX_1]).apply_slots(
        [SigningSlot(Address=addr, InstructionIndices=[0], IncludePayer=False, Mode=PubKeySignatureMode(PublicKey=pk))]
    )
    b.sign_with(Signer(SecretKey=other, PublicKey=other.ed25519_public()))
    with pytest.raises(ValueError, match="no signer found for address"):
        b.build()


def test_transaction_builder_error_short_circuit():
    sk, pk, addr = _classical_pair()
    b = new_transaction_builder([IX_1])
    b.errs.append(ValueError("prior"))
    assert b.with_payer(addr) is b
    with pytest.raises(ValueError, match="prior"):
        b.build()


def test_transaction_builder_unified_payer_gas_only():
    sk, pk, addr = _classical_pair()
    b = new_transaction_builder([IX_1]).with_payer(addr).apply_slots(
        [SigningSlot(Address=addr, InstructionIndices=[], IncludePayer=False, Mode=PubKeySignatureMode(PublicKey=pk))]
    ).sign_with(Signer(SecretKey=sk, PublicKey=pk))
    tx = b.build()
    tx.validate_wire()


def test_transaction_builder_split_payer_self_pay():
    sk1, pk1, addr1 = _classical_pair()
    sk2, pk2, addr2 = _ed_pair()
    b = new_transaction_builder([IX_1, IX_2]).apply_slots(
        [
            SigningSlot(Address=addr1, InstructionIndices=[0], IncludePayer=True, Mode=PubKeySignatureMode(PublicKey=pk1)),
            SigningSlot(Address=addr2, InstructionIndices=[1], IncludePayer=True, Mode=PubKeySignatureMode(PublicKey=pk2)),
        ]
    ).sign_with(
        Signer(SecretKey=sk1, PublicKey=pk1),
        Signer(SecretKey=sk2, PublicKey=pk2),
    )
    tx = b.build()
    tx.validate_wire()
