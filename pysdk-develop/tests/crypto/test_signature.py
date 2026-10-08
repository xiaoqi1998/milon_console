"""Port of gosdk-develop/crypto/test/signature_test.go."""

import json

import pytest

from milon_sdk import crypto
from milon_sdk.postcard import Deserializer, Serializer

FNDSA_PENDING = pytest.mark.skip(reason="待 _fndsa 纯 Python 移植完成后启用")

MSG = b"hello crypto"


def _pair_secp():
    sk = crypto.new_classical_secret_key()
    pk = sk.secp256k1_public()
    return sk.sign_secp256k1(MSG), pk


def _pair_ed():
    sk = crypto.new_pure_classical_secret_key()
    return sk.sign_ed25519(MSG), sk.ed25519_public()


def _pair_bls():
    sk = crypto.new_pure_classical_secret_key()
    return sk.sign_bls12381(MSG), sk.bls12381_public()


def test_signature_sign_verify_and_round_trip():
    cases = {
        "Secp256k1": (crypto.SignatureType.SECP256K1, _pair_secp),
        "Ed25519": (crypto.SignatureType.ED25519, _pair_ed),
        "BLS12381": (crypto.SignatureType.BLS12381, _pair_bls),
    }
    for name, (sig_type, create_pair) in cases.items():
        sig1, pk = create_pair()
        assert sig1.Variant == sig_type
        sig1.verify(MSG, pk)

        sig2 = crypto.new_signature_from_bytes(sig1.as_bytes())
        assert sig1.Bytes == sig2.Bytes
        assert sig1.Variant == sig2.Variant

        sig3 = crypto.new_signature_from_string_relaxed(sig1.to_hex())
        assert sig1.Bytes == sig3.Bytes
        assert sig1.Variant == sig3.Variant

        sig4 = crypto.new_signature_from_string_relaxed("0x" + sig1.to_hex())
        assert sig1.Bytes == sig4.Bytes
        assert sig1.Variant == sig4.Variant

        sig5 = crypto.new_signature_from_string_relaxed(sig1.to_base58())
        assert sig1.Bytes == sig5.Bytes
        assert sig1.Variant == sig5.Variant


@FNDSA_PENDING
def test_signature_sign_verify_and_round_trip_fndsa():
    sker, pk = crypto.new_fn_dsa512_secret_key()
    sig1 = crypto.as_fn_dsa512_secret_key(sker).sign_fn_dsa512(MSG)
    assert sig1.Variant == crypto.SignatureType.FN_DSA512
    sig1.verify(MSG, pk)
    sig2 = crypto.new_signature_from_bytes(sig1.as_bytes())
    assert sig1.Bytes == sig2.Bytes
    assert sig1.Variant == sig2.Variant
    sig5 = crypto.new_signature_from_string_relaxed(sig1.to_base58())
    assert sig1.Bytes == sig5.Bytes


def test_signature_wrong_variant_conversions():
    sk = crypto.new_classical_secret_key()
    sig = sk.sign_secp256k1(b"z")

    sig.to_secp256k1()  # NoError

    with pytest.raises(ValueError, match="not an ed25519 signature"):
        sig.to_ed25519()
    with pytest.raises(ValueError, match="not a BLS signature"):
        sig.to_bls12381()
    with pytest.raises(ValueError, match="not a FN-DSA-512 signature"):
        sig.to_fn_dsa512()


def test_signature_from_bytes_wrong_len():
    with pytest.raises(ValueError):
        crypto.new_signature_from_bytes(b"")
    with pytest.raises(ValueError):
        crypto.new_signature_from_bytes(b"\x00" * 63)
    with pytest.raises(ValueError):
        crypto.new_signature_from_bytes(b"\x00" * 95)


def test_signature_from_string_relaxed_invalid_format():
    with pytest.raises(ValueError):
        crypto.new_signature_from_string_relaxed("not a valid hex string")
    with pytest.raises(ValueError):
        crypto.new_signature_from_string_relaxed("!!!invalid base58!!!")


def test_verify_batch_empty():
    crypto.verify_batch([], [], [])


def test_verify_batch_length_mismatch():
    sk = crypto.new_pure_classical_secret_key()
    pk = sk.ed25519_public()
    sig = sk.sign_ed25519(b"a")

    with pytest.raises(ValueError, match="length mismatch"):
        crypto.verify_batch([sig], [b"a", b"b"], [pk])


def _three_pairs(kind):
    if kind == "Secp256k1":
        sk = crypto.new_pure_classical_secret_key()
        pk = sk.secp256k1_public()
        msg = b"a"
        return sk.sign_secp256k1(msg), msg, pk
    if kind == "Ed25519":
        sk = crypto.new_pure_classical_secret_key()
        msg = b"b"
        return sk.sign_ed25519(msg), msg, sk.ed25519_public()
    if kind == "BLS12381":
        sk = crypto.new_pure_classical_secret_key()
        msg = b"c"
        return sk.sign_bls12381(msg), msg, sk.bls12381_public()
    raise AssertionError(kind)


def test_verify_batch_three():
    for kind in ("Secp256k1", "Ed25519", "BLS12381"):
        triples = [_three_pairs(kind) for _ in range(3)]
        crypto.verify_batch(
            [t[0] for t in triples],
            [t[1] for t in triples],
            [t[2] for t in triples],
        )


@FNDSA_PENDING
def test_verify_batch_three_fndsa():
    sker, pk = crypto.new_fn_dsa512_secret_key()
    sk = crypto.as_fn_dsa512_secret_key(sker)
    msg = b"d"
    sig = sk.sign_fn_dsa512(msg)
    crypto.verify_batch([sig] * 3, [msg] * 3, [pk] * 3)


def test_verify_batch_all():
    sk1 = crypto.new_pure_classical_secret_key()
    pk1 = sk1.secp256k1_public()
    m1 = b"1"
    sig1 = sk1.sign_secp256k1(m1)

    sk2 = crypto.new_pure_classical_secret_key()
    pk2 = sk2.ed25519_public()
    m2 = b"2"
    sig2 = sk2.sign_ed25519(m2)

    sk3 = crypto.new_pure_classical_secret_key()
    pk3 = sk3.bls12381_public()
    m3 = b"3"
    sig3 = sk3.sign_bls12381(m3)

    crypto.verify_batch([sig1, sig2, sig3], [m1, m2, m3], [pk1, pk2, pk3])


@FNDSA_PENDING
def test_verify_batch_all_fndsa():
    sker4, pk4 = crypto.new_fn_dsa512_secret_key()
    sk4 = crypto.as_fn_dsa512_secret_key(sker4)
    m4 = b"4"
    sig4 = sk4.sign_fn_dsa512(m4)

    sk2 = crypto.new_pure_classical_secret_key()
    sig2 = sk2.sign_ed25519(b"2")
    crypto.verify_batch([sig2, sig4], [b"2", m4], [sk2.ed25519_public(), pk4])


def test_signature_verify_type_mismatch():
    sk = crypto.new_classical_secret_key()
    sig = sk.sign_ed25519(b"msg")

    pk = sk.secp256k1_public()
    with pytest.raises(ValueError, match="type mismatch"):
        sig.verify(b"msg", pk)


def test_signature_json_round_trip():
    cases = [
        lambda: crypto.new_classical_secret_key().sign_secp256k1(b"test"),
        lambda: crypto.new_classical_secret_key().sign_ed25519(b"test"),
        lambda: crypto.new_classical_secret_key().sign_bls12381(b"test"),
    ]
    for create_sig in cases:
        sig = create_sig()

        json_data = json.dumps(sig, default=lambda o: o.to_json_value())
        str_val = json.loads(json_data)
        assert sig.to_base58() == str_val

        decoded = crypto.Signature()
        decoded.from_json_value(json.loads(json_data))
        assert sig.Bytes == decoded.Bytes
        assert sig.Variant == decoded.Variant


@FNDSA_PENDING
def test_signature_json_round_trip_fndsa():
    sker, _ = crypto.new_fn_dsa512_secret_key()
    sig = crypto.as_fn_dsa512_secret_key(sker).sign_fn_dsa512(b"test")
    json_data = json.dumps(sig, default=lambda o: o.to_json_value())
    decoded = crypto.Signature()
    decoded.from_json_value(json.loads(json_data))
    assert sig.Bytes == decoded.Bytes


def _assert_postcard_roundtrip(sig, expected_len):
    assert len(sig.Bytes) == expected_len

    serializer = Serializer()
    sig.marshal_postcard(serializer)

    deserializer = Deserializer(serializer.bytes())
    decoded = crypto.Signature()
    decoded.unmarshal_postcard(deserializer)
    assert sig.Bytes == decoded.Bytes
    assert sig.Variant == decoded.Variant
    assert len(decoded.Bytes) == expected_len

    deserializer.assert_end()


def test_signature_postcard_round_trip():
    _assert_postcard_roundtrip(
        crypto.new_classical_secret_key().sign_secp256k1(b"test"),
        crypto.SIGNATURE_SECP256K1_SIZE,
    )
    _assert_postcard_roundtrip(
        crypto.new_pure_classical_secret_key().sign_ed25519(b"test"),
        crypto.SIGNATURE_ED25519_SIZE,
    )
    _assert_postcard_roundtrip(
        crypto.new_pure_classical_secret_key().sign_bls12381(b"test"),
        crypto.SIGNATURE_BLS12381_SIZE,
    )


@FNDSA_PENDING
def test_signature_postcard_round_trip_fndsa():
    sker, _ = crypto.new_fn_dsa512_secret_key()
    _assert_postcard_roundtrip(
        crypto.as_fn_dsa512_secret_key(sker).sign_fn_dsa512(b"test"),
        crypto.SIGNATURE_FN_DSA512_SIZE,
    )


def test_signature_deserialize_postcard_invalid_data():
    sig = crypto.Signature()
    with pytest.raises(ValueError, match="failed to deserialize signature variant"):
        sig.unmarshal_postcard(Deserializer(b""))

    with pytest.raises(ValueError, match="unknown signature variant"):
        sig.unmarshal_postcard(Deserializer(b"\x05"))

    with pytest.raises(ValueError, match="failed to deserialize signature Bytes"):
        sig.unmarshal_postcard(Deserializer(b"\x01\x02\x03\x04\x05"))
