"""Port of gosdk-develop/crypto/test/publickey_test.go."""

import json

import pytest

from milon_sdk import crypto
from milon_sdk.postcard import Deserializer, Serializer

FNDSA_PENDING = pytest.mark.skip(reason="待 _fndsa 纯 Python 移植完成后启用")


def _classical_pk_secp():
    return crypto.new_classical_secret_key().secp256k1_public()


def _pure_pk_ed():
    return crypto.new_pure_classical_secret_key().ed25519_public()


def _pure_pk_bls():
    return crypto.new_pure_classical_secret_key().bls12381_public()


@FNDSA_PENDING
def _fndsa_pk():
    _, pk = crypto.new_fn_dsa512_secret_key()
    return pk


def test_public_key_variant():
    pk1 = crypto.new_classical_secret_key().secp256k1_public()
    assert pk1.Variant == crypto.PublicKeyType.SECP256K1
    assert pk1.is_secp256k1()
    assert not pk1.is_ed25519()
    assert not pk1.is_bls12381()
    assert not pk1.is_fn_dsa512()

    pk2 = _pure_pk_ed()
    assert pk2.Variant == crypto.PublicKeyType.ED25519
    assert pk2.is_ed25519()
    assert not pk2.is_secp256k1()
    assert not pk2.is_bls12381()
    assert not pk2.is_fn_dsa512()

    pk3 = _pure_pk_bls()
    assert pk3.Variant == crypto.PublicKeyType.BLS12381
    assert pk3.is_bls12381()
    assert not pk3.is_secp256k1()
    assert not pk3.is_ed25519()
    assert not pk3.is_fn_dsa512()


@FNDSA_PENDING
def test_public_key_variant_fndsa():
    _, pk4 = crypto.new_fn_dsa512_secret_key()
    assert pk4.Variant == crypto.PublicKeyType.FN_DSA512
    assert not pk4.is_bls12381()
    assert not pk4.is_secp256k1()
    assert not pk4.is_ed25519()
    assert pk4.is_fn_dsa512()


def test_public_key_round_trip():
    cases = {
        "Secp256k1": _classical_pk_secp,
        "Ed25519": _pure_pk_ed,
        "BLS12381": _pure_pk_bls,
    }
    for name, create_pk in cases.items():
        pk1 = create_pk()

        pk2 = crypto.new_public_key_from_bytes(pk1.as_bytes())
        assert pk1.Variant == pk2.Variant
        assert pk1.Bytes == pk2.Bytes

        pk3 = crypto.new_public_key_from_string_relaxed(pk1.to_hex())
        assert pk1.Variant == pk3.Variant
        assert pk1.Bytes == pk3.Bytes

        pk4 = crypto.new_public_key_from_string_relaxed("0x" + pk1.to_hex())
        assert pk1.Variant == pk4.Variant
        assert pk1.Bytes == pk4.Bytes

        pk5 = crypto.new_public_key_from_string_relaxed(pk1.to_base58())
        assert pk1.Variant == pk5.Variant
        assert pk1.Bytes == pk5.Bytes


@FNDSA_PENDING
def test_public_key_round_trip_fndsa():
    pk1, _ = crypto.new_fn_dsa512_secret_key()
    pk1 = _fndsa_pk()
    pk2 = crypto.new_public_key_from_bytes(pk1.as_bytes())
    assert pk1.Bytes == pk2.Bytes
    pk3 = crypto.new_public_key_from_string_relaxed(pk1.to_hex())
    assert pk1.Bytes == pk3.Bytes
    pk5 = crypto.new_public_key_from_string_relaxed(pk1.to_base58())
    assert pk1.Bytes == pk5.Bytes


def test_public_key_from_bytes_wrong_len():
    with pytest.raises(ValueError):
        crypto.new_public_key_from_bytes(b"")
    with pytest.raises(ValueError):
        crypto.new_public_key_from_bytes(b"\x00" * 31)
    with pytest.raises(ValueError):
        crypto.new_public_key_from_bytes(b"\x00" * 47)


def test_public_key_from_string_relaxed_invalid_format():
    with pytest.raises(ValueError):
        crypto.new_public_key_from_string_relaxed("not a valid hex string")
    with pytest.raises(ValueError):
        crypto.new_public_key_from_string_relaxed("!!!invalid base58!!!")
    with pytest.raises(ValueError):
        crypto.new_public_key_from_string_relaxed("[1,2,3]")


def test_public_key_wrong_variant_conversions():
    pk = _pure_pk_ed()

    pk.to_ed25519()  # NoError

    with pytest.raises(ValueError):
        pk.to_secp256k1()
    with pytest.raises(ValueError):
        pk.to_bls12381()


def test_public_key_to_native():
    # Secp256k1
    pk1 = _classical_pk_secp()
    native1 = pk1.to_secp256k1()
    pk1_decoded = crypto.PublicKey()
    pk1_decoded.from_secp256k1_native(native1)
    assert pk1.Bytes == pk1_decoded.as_bytes()

    # Ed25519
    pk2 = _pure_pk_ed()
    native2 = pk2.to_ed25519()
    pk2_decoded = crypto.PublicKey()
    pk2_decoded.from_ed25519_native(native2)
    assert pk2.Bytes == pk2_decoded.as_bytes()

    # BLS
    pk3 = _pure_pk_bls()
    native3 = pk3.to_bls12381()
    pk3_decoded = crypto.PublicKey()
    pk3_decoded.from_bls12381_native(native3)
    assert pk3.Bytes == pk3_decoded.as_bytes()


def _assert_json_roundtrip(pk):
    json_data = json.dumps(pk, default=lambda o: o.to_json_value())
    b58_str = json.loads(json_data)
    assert pk.to_base58() == b58_str

    decoded = crypto.PublicKey()
    decoded.from_json_value(json.loads(json_data))
    assert pk.Bytes == decoded.Bytes
    assert pk.Variant == decoded.Variant


def test_public_key_json_round_trip():
    _assert_json_roundtrip(_classical_pk_secp())
    _assert_json_roundtrip(_pure_pk_ed())
    _assert_json_roundtrip(_pure_pk_bls())


@FNDSA_PENDING
def test_public_key_json_round_trip_fndsa():
    _assert_json_roundtrip(_fndsa_pk())


def _assert_postcard_roundtrip(pk, expected_len):
    assert len(pk.Bytes) == expected_len

    serializer = Serializer()
    pk.marshal_postcard(serializer)

    deserializer = Deserializer(serializer.bytes())
    decoded = crypto.PublicKey()
    decoded.unmarshal_postcard(deserializer)
    assert pk.Bytes == decoded.Bytes
    assert pk.Variant == decoded.Variant
    assert len(decoded.Bytes) == expected_len

    deserializer.assert_end()


def test_public_key_postcard_round_trip():
    _assert_postcard_roundtrip(
        _classical_pk_secp(), crypto.PUBLIC_KEY_SECP256K1_SIZE
    )
    _assert_postcard_roundtrip(_pure_pk_ed(), crypto.PUBLIC_KEY_ED25519_SIZE)
    _assert_postcard_roundtrip(_pure_pk_bls(), crypto.PUBLIC_KEY_BLS12381_SIZE)


@FNDSA_PENDING
def test_public_key_postcard_round_trip_fndsa():
    _assert_postcard_roundtrip(_fndsa_pk(), crypto.PUBLIC_KEY_FN_DSA512_SIZE)


def test_public_key_deserialize_postcard_invalid_data():
    pk = crypto.PublicKey()
    with pytest.raises(ValueError, match="failed to deserialize public key variant"):
        pk.unmarshal_postcard(Deserializer(b""))

    with pytest.raises(ValueError, match="unknown public key variant"):
        pk.unmarshal_postcard(Deserializer(b"\x06"))

    with pytest.raises(ValueError, match="failed to deserialize public key Bytes"):
        pk.unmarshal_postcard(Deserializer(b"\x01\x02\x03\x04\x05"))
