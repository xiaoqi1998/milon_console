"""Port of gosdk-develop/crypto/test/address_test.go + fn_dsa512_test.go."""

import json

import pytest

from milon_sdk import crypto
from milon_sdk.postcard import Deserializer, Serializer

FNDSA_PENDING = pytest.mark.skip(reason="待 _fndsa 纯 Python 移植完成后启用")


def test_address_from_public_key_round_trip():
    cases = [
        lambda: crypto.new_classical_secret_key().secp256k1_public(),
        lambda: crypto.new_classical_secret_key().ed25519_public(),
        lambda: crypto.new_classical_secret_key().bls12381_public(),
    ]
    for create_pk in cases:
        addr1 = crypto.new_address_from_public_key(create_pk())
        assert len(addr1.Bytes) == crypto.ADDRESS_RAW_LEN

        addr2 = crypto.new_address_from_bytes(addr1.Bytes)
        assert addr1.to_base58() == addr2.to_base58()

        addr3 = crypto.new_address_from_relaxed(addr1.to_hex())
        assert addr1.to_base58() == addr3.to_base58()

        addr4 = crypto.new_address_from_relaxed("0x" + addr1.to_hex())
        assert addr1.to_base58() == addr4.to_base58()

        addr5 = crypto.new_address_from_relaxed(addr1.to_base58())
        assert addr1.to_base58() == addr5.to_base58()


@FNDSA_PENDING
def test_address_from_public_key_round_trip_fndsa():
    _, pk = crypto.new_fn_dsa512_secret_key()
    addr1 = crypto.new_address_from_public_key(pk)
    assert len(addr1.Bytes) == crypto.ADDRESS_RAW_LEN
    addr2 = crypto.new_address_from_bytes(addr1.Bytes)
    assert addr1.to_base58() == addr2.to_base58()


def test_address_json_round_trip():
    pk = crypto.new_pure_classical_secret_key().ed25519_public()
    addr = crypto.new_address_from_public_key(pk)

    json_data = json.dumps(addr, default=lambda o: o.to_json_value())

    decoded = crypto.Address()
    decoded.from_json_value(json.loads(json_data))
    assert addr == decoded


def test_address_postcard_round_trip():
    pk = crypto.new_pure_classical_secret_key().ed25519_public()
    addr = crypto.new_address_from_public_key(pk)

    serializer = Serializer()
    addr.marshal_postcard(serializer)

    data = serializer.bytes()
    assert len(data) == crypto.ADDRESS_RAW_LEN

    deserializer = Deserializer(data)
    decoded = crypto.Address()
    decoded.unmarshal_postcard(deserializer)
    assert addr == decoded

    deserializer.assert_end()


def test_address_from_bytes_error():
    with pytest.raises(ValueError):
        crypto.new_address_from_bytes(b"\x00" * (crypto.ADDRESS_RAW_LEN - 1))
    with pytest.raises(ValueError):
        crypto.new_address_from_bytes(b"\x00" * (crypto.ADDRESS_RAW_LEN + 1))


def test_address_from_relaxed_error():
    with pytest.raises(ValueError):
        crypto.new_address_from_relaxed(None)
    with pytest.raises(ValueError):
        crypto.new_address_from_relaxed(12345)
    with pytest.raises(ValueError):
        crypto.new_address_from_relaxed("0xzz")
    with pytest.raises(ValueError):
        crypto.new_address_from_relaxed("!!invalid base58!!")


# ==== fn_dsa512_test.go（整体待 _fndsa 完成后启用） ====


@FNDSA_PENDING
def test_keygen512():
    sign_key, vrfy_key = crypto.keygen_512()
    assert len(sign_key) == crypto.FN_DSA512_SIGN_KEY_LEN
    assert len(vrfy_key) == crypto.FN_DSA512_VRFY_KEY_LEN


@FNDSA_PENDING
def test_new_sign_key_512_from_bytes():
    sign_key, _ = crypto.keygen_512()
    decoded = crypto.new_sign_key_512_from_bytes(sign_key)
    assert sign_key == decoded


@FNDSA_PENDING
def test_new_vrfy_key_512_from_bytes():
    _, vrfy_key = crypto.keygen_512()
    decoded = crypto.new_vrfy_key_512_from_bytes(vrfy_key)
    assert vrfy_key == decoded


@FNDSA_PENDING
def test_sign_and_verify_512():
    sign_key, vrfy_key = crypto.keygen_512()
    msg = b"Hello, FN-DSA-512!"

    sig = crypto.sign_512(sign_key, msg)
    assert len(sig) == crypto.FN_DSA512_SIG_LEN
    crypto.verify_512(vrfy_key, sig, msg)

    with pytest.raises(ValueError):
        crypto.verify_512(vrfy_key, sig, b"Wrong message")


def test_new_sign_key_512_from_bytes_wrong_len():
    with pytest.raises(ValueError):
        crypto.new_sign_key_512_from_bytes(
            b"\x00" * (crypto.FN_DSA512_SIGN_KEY_LEN - 1)
        )


def test_new_vrfy_key_512_from_bytes_wrong_len():
    with pytest.raises(ValueError):
        crypto.new_vrfy_key_512_from_bytes(
            b"\x00" * (crypto.FN_DSA512_VRFY_KEY_LEN - 1)
        )
