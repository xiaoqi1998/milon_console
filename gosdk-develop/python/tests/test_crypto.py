"""crypto 层单元测试：地址派生、密钥解析、4 算法签名/验签。"""
from __future__ import annotations

import os
import sys

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from milon_sdk.crypto import (  # noqa: E402
    ADDRESS_RAW_LEN,
    Address,
    ClassicalSecretKey,
    PublicKey,
    PublicKeyType,
    Signature,
    SignatureType,
    hash32,
    MILON_PK_ADDRESS_DOMAIN_CONTEXT,
)


def test_address_from_public_key_is_20_bytes() -> None:
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    addr = Address.from_public_key(pk)
    assert len(addr.as_bytes()) == ADDRESS_RAW_LEN
    # 地址 = BLAKE3(ROOT || pk_domain || pk)[:20]
    expected = hash32(MILON_PK_ADDRESS_DOMAIN_CONTEXT.encode(), pk.as_bytes())[:20]
    assert addr.as_bytes() == expected


def test_address_parsing_roundtrip() -> None:
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    addr = Address.from_public_key(pk)
    assert Address.from_hex(addr.to_hex()) == addr
    assert Address.from_base58(addr.to_base58()) == addr
    assert Address.from_relaxed(addr.to_hex()) == addr
    assert Address.from_relaxed("0x" + addr.to_hex()) == addr
    assert Address.from_relaxed(addr.to_base58()) == addr
    assert Address.from_relaxed(addr) == addr


def test_address_equality_and_hash() -> None:
    sk = ClassicalSecretKey.generate()
    addr1 = Address.from_public_key(sk.ed25519_public())
    addr2 = Address.from_public_key(sk.ed25519_public())
    assert addr1 == addr2
    assert hash(addr1) == hash(addr2)
    assert len({addr1, addr2}) == 1  # 可哈希


def test_public_key_from_bytes_by_length() -> None:
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    parsed = PublicKey.from_bytes(pk.as_bytes())
    assert parsed.variant == PublicKeyType.ED25519
    assert parsed.as_bytes() == pk.as_bytes()
    with pytest.raises(Exception):
        PublicKey.from_bytes(b"\x00" * 31)


def test_public_key_string_relaxed() -> None:
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    assert PublicKey.from_string_relaxed(pk.to_hex()) == pk
    assert PublicKey.from_string_relaxed(pk.to_base58()) == pk


def test_secret_key_parsing_formats() -> None:
    sk = ClassicalSecretKey.generate()
    raw = sk.as_bytes()
    assert ClassicalSecretKey.from_bytes(raw).as_bytes() == raw
    assert ClassicalSecretKey.from_string_relaxed(sk.to_hex()).as_bytes() == raw
    assert ClassicalSecretKey.from_string_relaxed("0x" + sk.to_hex()).as_bytes() == raw
    assert ClassicalSecretKey.from_string_relaxed(sk.to_base58()).as_bytes() == raw
    array_fmt = "[" + ",".join(str(b) for b in raw) + "]"
    assert ClassicalSecretKey.from_string_relaxed(array_fmt).as_bytes() == raw


def test_ed25519_sign_verify() -> None:
    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    msg = b"hello milon"
    sig = sk.sign_ed25519(msg)
    assert sig.variant == SignatureType.ED25519
    assert len(sig.as_bytes()) == 64
    assert sig.verify(msg, pk) is True
    assert sig.verify(b"tampered", pk) is False


def test_secp256k1_sign_verify_and_recoverable_v() -> None:
    sk = ClassicalSecretKey.generate()  # 默认校验 secp256k1 标量
    pk = sk.secp256k1_public()
    assert pk.variant == PublicKeyType.SECP256K1
    assert len(pk.as_bytes()) == 33
    msg = b"milon secp256k1 message"
    sig = sk.sign_secp256k1(msg)
    assert sig.variant == SignatureType.SECP256K1
    assert len(sig.as_bytes()) == 65
    # V 必须是 27/28（以太坊风格可恢复签名）
    assert sig.as_bytes()[64] in (27, 28)
    assert sig.verify(msg, pk) is True
    assert sig.verify(b"tampered", pk) is False


def test_secp256k1_32byte_hash_pass_through() -> None:
    """32 字节消息直接作为哈希（不再次 BLAKE3）。"""
    sk = ClassicalSecretKey.generate()
    pk = sk.secp256k1_public()
    msg = bytes(range(32))
    sig = sk.sign_secp256k1(msg)
    assert sig.verify(msg, pk) is True


def test_bls12381_sign_verify() -> None:
    sk = ClassicalSecretKey.generate()
    pk = sk.bls12381_public()
    assert pk.variant == PublicKeyType.BLS12381
    assert len(pk.as_bytes()) == 48
    msg = b"milon bls message"
    sig = sk.sign_bls12381(msg)
    assert sig.variant == SignatureType.BLS12381
    assert len(sig.as_bytes()) == 96
    assert sig.verify(msg, pk) is True
    assert sig.verify(b"tampered", pk) is False


def test_bls12381_byte_parity_with_go() -> None:
    """与 Go blst 黄金向量逐字节一致（向量由 python/parity/blsgen 生成）。

    关键事实：Go SDK 调 blst.Sign(sk, msg, nil)，nil DST 在 blst 中即「空 DST」，
    因此 py_ecc 路径也必须用空 DST（非 NUL/POP 字符串）。
    """
    from milon_sdk.crypto import _bls

    seed = bytes.fromhex("3031323334353637383961626364656630313233343536373839616263646566")
    msg = bytes.fromhex("68656c6c6f206d696c6f6e20626c73")
    go_pk = (
        "b3bf6ec94188a19d79e81f0fce0fa235e4c719ceeb03f9babda3a437170833a2670c2977"
        "bd9a18513de8a2618ed9f439"
    )
    go_sig = (
        "92825cdfbf6c28897f9ff9e399a27cbfd7d93c5819668c95818091fd6cde45c84f7cae4"
        "bc6dc3e660a347cbf8a0d28c9027a1a1db6e70f0110b1f1acdd48f28b783ffdae8b6137"
        "b84bbc558c82ae40bd3ee666f2288ad2e451dbab14f3ad492c"
    )
    pk = _bls.bls_public_from_seed(seed)
    sig = _bls.bls_sign(seed, msg)
    assert pk.hex() == go_pk
    assert sig.hex() == go_sig
    # 交叉验证：py_ecc 能验证真实的 Go blst 签名
    assert _bls.bls_verify(bytes.fromhex(go_pk), bytes.fromhex(go_sig), msg) is True
    assert _bls.bls_verify(bytes.fromhex(go_pk), sig, b"tampered") is False


def test_sign_for_dispatch() -> None:
    sk = ClassicalSecretKey.generate()
    assert sk.sign_for(sk.ed25519_public(), b"m").variant == SignatureType.ED25519
    assert sk.sign_for(sk.secp256k1_public(), b"m").variant == SignatureType.SECP256K1
    assert sk.sign_for(sk.bls12381_public(), b"m").variant == SignatureType.BLS12381


def test_signature_from_bytes_autodetect() -> None:
    sk = ClassicalSecretKey.generate()
    for sig in (sk.sign_ed25519(b"m"), sk.sign_secp256k1(b"m"), sk.sign_bls12381(b"m")):
        parsed = Signature.from_bytes(sig.as_bytes())
        assert parsed.variant == sig.variant
        assert parsed.as_bytes() == sig.as_bytes()
        assert Signature.from_string_relaxed(sig.to_base58()).as_bytes() == sig.as_bytes()


def test_signature_postcard_roundtrip() -> None:
    from milon_sdk.postcard import Deserializer, Serializer

    sk = ClassicalSecretKey.generate()
    sig = sk.sign_ed25519(b"roundtrip")
    s = Serializer()
    sig.marshal_postcard(s)
    d = Deserializer(s.bytes())
    assert Signature.unmarshal_postcard(d) == sig


def test_verify_batch() -> None:
    from milon_sdk.crypto.signature import verify_batch

    sk = ClassicalSecretKey.generate()
    pk = sk.ed25519_public()
    sigs = [sk.sign_ed25519(b"m1"), sk.sign_ed25519(b"m2")]
    msgs = [b"m1", b"m2"]
    pubs = [pk, pk]
    assert verify_batch(sigs, msgs, pubs) is True
    assert verify_batch(sigs, [b"m1", b"bad"], pubs) is False
