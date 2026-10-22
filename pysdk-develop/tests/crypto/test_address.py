"""Port of gosdk-develop/crypto/test/address_test.go + fn_dsa512_test.go."""

import json

import pytest

from milon_sdk import crypto
from milon_sdk.crypto import hash_domain
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


# ---- gosdk 20261015：合约保留地址区 + 派生重试 ----


def test_address_reserved_space_without_valid_seed_checksum():
    address = crypto.new_address_from_app_seed(crypto.CONTRACT_APP_ID, 1)
    corrupted = bytearray(address.Bytes)
    corrupted[19] ^= 1
    address = crypto.new_address_from_bytes(bytes(corrupted))

    _, _, ok = address.app_seed()
    assert ok is False
    assert address.is_app_reserved() is True

    restored = crypto.new_address_from_bytes(address.Bytes)
    assert address == restored
    assert address == crypto.new_address_from_relaxed(address.to_base58())


def test_address_seed_reservation_checks_entire_checksum():
    for app_id in range(256):
        for seed in (0, 1, (1 << 64) - 1):
            address = crypto.new_address_from_app_seed(app_id, seed)
            got_app_id, got_seed, ok = address.app_seed()
            assert ok is True
            assert got_app_id == app_id
            assert got_seed == seed
            assert address.is_app_reserved() is True

            for index in range(9, crypto.ADDRESS_RAW_LEN):
                corrupted = bytearray(address.Bytes)
                corrupted[index] ^= 1
                c = crypto.Address(bytes(corrupted))
                _, _, ok = c.app_seed()
                assert ok is False
                assert c.is_app_reserved() is (app_id == crypto.CONTRACT_APP_ID)

    assert crypto.Address().is_app_reserved() is False


def _find_reserved_first_candidate(domain: bytes) -> bytes:
    """返回首个地址候选命中保留区的 32 字节 fixture。"""
    for n in range(100_000):
        seed = n.to_bytes(8, "big") + b"\x00" * 24
        if crypto.hash32(domain, seed)[0] == crypto.CONTRACT_APP_ID:
            return seed
    raise AssertionError("fixture contains a reserved first candidate")


@pytest.mark.parametrize(
    "domain,build_pk",
    [
        (
            hash_domain.PK_ADDRESS_DOMAIN_BYTES,
            lambda seed: crypto.PublicKey(
                crypto.PublicKeyType.ED25519, seed
            ),
        ),
        (
            hash_domain.KEYLESS_ADDRESS_DOMAIN_BYTES,
            lambda seed: crypto.new_keyless_derived_public_key(seed),
        ),
    ],
    ids=["ed25519", "keyless"],
)
def test_address_public_key_retry(domain, build_pk):
    seed = _find_reserved_first_candidate(domain)
    pk = build_pk(seed)

    address = crypto.new_address_from_public_key(pk)
    assert address.is_app_reserved() is False
    assert crypto.new_address_from_public_key(pk) == address

    counter_bytes = (1).to_bytes(8, "big")
    digest = crypto.hash32(
        hash_domain.PK_ADDRESS_RETRY_DOMAIN_BYTES, domain, seed, counter_bytes
    )
    assert address == crypto.Address(digest[: crypto.ADDRESS_RAW_LEN])


def test_address_from_public_key_matches_single_hash_when_unreserved():
    seed = bytes([7]) * 32
    pk = crypto.PublicKey(crypto.PublicKeyType.ED25519, seed)
    direct = crypto.Address(
        crypto.hash32(hash_domain.PK_ADDRESS_DOMAIN_BYTES, seed)[
            : crypto.ADDRESS_RAW_LEN
        ]
    )
    if direct.is_app_reserved():
        pytest.skip("fixture hits the reserved space")

    assert crypto.new_address_from_public_key(pk) == direct


def test_address_all_public_key_types_derive_only_ordinary():
    classical = crypto.new_classical_secret_key()
    public_keys = [
        classical.secp256k1_public(),
        classical.ed25519_public(),
        classical.bls12381_public(),
        crypto.new_keyless_derived_public_key(bytes([7]) * 32),
    ]
    for pk in public_keys:
        address = crypto.new_address_from_public_key(pk)
        assert address.is_app_reserved() is False
        assert crypto.new_address_from_public_key(pk) == address


@FNDSA_PENDING
def test_address_fn_dsa512_derives_only_ordinary():
    _, fn_dsa_pk = crypto.new_fn_dsa512_secret_key()
    address = crypto.new_address_from_public_key(fn_dsa_pk)
    assert address.is_app_reserved() is False
    assert crypto.new_address_from_public_key(fn_dsa_pk) == address


def test_address_keyless_derived_disjoint_from_ed25519():
    seed = bytes([7]) * 32
    keyless = crypto.new_address_from_public_key(
        crypto.new_keyless_derived_public_key(seed)
    )
    ed25519_addr = crypto.new_address_from_public_key(
        crypto.PublicKey(crypto.PublicKeyType.ED25519, seed)
    )
    assert keyless != ed25519_addr
    assert (
        crypto.new_address_from_public_key(
            crypto.new_keyless_derived_public_key(seed)
        )
        == keyless
    )


def test_address_from_app_seed_deterministic_and_scoped():
    a1 = crypto.new_address_from_app_seed(255, 1)
    assert a1 == crypto.new_address_from_app_seed(255, 1)
    assert a1 != crypto.new_address_from_app_seed(255, 2)
    assert a1 != crypto.new_address_from_app_seed(2, 1)

    app_id, seed, ok = a1.app_seed()
    assert (ok, app_id, seed) == (True, 255, 1)
    app_id, seed, ok = crypto.new_address_from_app_seed(2, 1).app_seed()
    assert (ok, app_id, seed) == (True, 2, 1)

    ed = crypto.new_address_from_public_key(
        crypto.PublicKey(crypto.PublicKeyType.ED25519, bytes(32))
    )
    _, _, ok = ed.app_seed()
    assert ok is False


def test_address_from_app_seed_32_does_not_embed_seed():
    address = crypto.new_address_from_app_seed32(7, bytes([9]) * 32)
    assert address == crypto.new_address_from_app_seed32(7, bytes([9]) * 32)
    assert address != crypto.new_address_from_app_seed32(7, bytes([8]) * 32)
    _, _, ok = address.app_seed()
    assert ok is False
