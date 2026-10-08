"""Port of gosdk-develop/crypto/test/secretkey_test.go."""

import pytest

from milon_sdk import crypto

FNDSA_PENDING = pytest.mark.skip(reason="待 _fndsa 纯 Python 移植完成后启用")

MSG = b"hello crypto"


def test_secret_keyer_classical():
    classical = crypto.new_classical_secret_key()
    assert classical.type() == crypto.SecretKeyType.CLASSICAL
    assert crypto.as_classical_secret_key(classical) is not None
    assert crypto.as_fn_dsa512_secret_key(classical) is None


@FNDSA_PENDING
def test_secret_keyer_classical_and_fn_dsa512_mixed():
    classical = crypto.new_classical_secret_key()
    fn_dsa, _, = crypto.new_fn_dsa512_secret_key()
    assert fn_dsa.type() == crypto.SecretKeyType.FN_DSA512
    assert crypto.as_classical_secret_key(fn_dsa) is None
    assert crypto.as_fn_dsa512_secret_key(fn_dsa) is not None


def test_secret_key_round_trip():
    cases = {
        "Classical": lambda: crypto.new_classical_secret_key(),
        "PureClassical": lambda: crypto.new_pure_classical_secret_key(),
    }
    for name, create_key in cases.items():
        sk1 = create_key()

        sk2 = crypto.secret_keyer_from_bytes(sk1.as_bytes())
        assert sk1.as_bytes() == sk2.as_bytes()

        sk3 = crypto.secret_keyer_from_string_relaxed(sk1.to_hex())
        assert sk1.as_bytes() == sk3.as_bytes()

        sk4 = crypto.secret_keyer_from_string_relaxed("0x" + sk1.to_hex())
        assert sk1.as_bytes() == sk4.as_bytes()

        sk5 = crypto.secret_keyer_from_string_relaxed(sk1.to_base58())
        assert sk1.as_bytes() == sk5.as_bytes()

        bracket_str = "[" + ",".join(str(b) for b in sk1.as_bytes()) + "]"
        sk6 = crypto.secret_keyer_from_string_relaxed(bracket_str)
        assert sk1.as_bytes() == sk6.as_bytes()


@FNDSA_PENDING
def test_secret_key_round_trip_fndsa():
    sk1, _ = crypto.new_fn_dsa512_secret_key()
    sk2 = crypto.secret_keyer_from_bytes(sk1.as_bytes())
    assert sk1.as_bytes() == sk2.as_bytes()
    sk5 = crypto.secret_keyer_from_string_relaxed(sk1.to_base58())
    assert sk1.as_bytes() == sk5.as_bytes()
    bracket_str = "[" + ",".join(str(b) for b in sk1.as_bytes()) + "]"
    sk6 = crypto.secret_keyer_from_string_relaxed(bracket_str)
    assert sk1.as_bytes() == sk6.as_bytes()


def test_secret_keyer_sign_verify():
    msg = MSG

    # Secp256k1
    sk = crypto.as_classical_secret_key(crypto.new_classical_secret_key())
    pk = sk.secp256k1_public()
    sig = sk.sign_secp256k1(msg)
    sig.verify(msg, pk)
    with pytest.raises(ValueError):
        sig.verify(b"other", pk)
    other_pk = crypto.new_pure_classical_secret_key().secp256k1_public()
    with pytest.raises(ValueError):
        sig.verify(msg, other_pk)

    # Ed25519
    sk = crypto.as_classical_secret_key(crypto.new_pure_classical_secret_key())
    sig = sk.sign_ed25519(msg)
    pk = sk.ed25519_public()
    sig.verify(msg, pk)
    with pytest.raises(ValueError):
        sig.verify(b"other", pk)
    with pytest.raises(ValueError):
        sig.verify(msg, crypto.new_pure_classical_secret_key().ed25519_public())

    # BLS12381
    sk = crypto.as_classical_secret_key(crypto.new_pure_classical_secret_key())
    sig = sk.sign_bls12381(msg)
    pk = sk.bls12381_public()
    sig.verify(msg, pk)
    with pytest.raises(ValueError):
        sig.verify(b"other", pk)
    with pytest.raises(ValueError):
        sig.verify(msg, crypto.new_pure_classical_secret_key().bls12381_public())


@FNDSA_PENDING
def test_secret_keyer_sign_verify_fndsa():
    sker, pk = crypto.new_fn_dsa512_secret_key()
    sig = crypto.as_fn_dsa512_secret_key(sker).sign_fn_dsa512(MSG)
    sig.verify(MSG, pk)
    with pytest.raises(ValueError):
        sig.verify(b"other", pk)
    _, other_pk = crypto.new_fn_dsa512_secret_key()
    with pytest.raises(ValueError):
        sig.verify(MSG, other_pk)


def test_secret_key_from_bytes_too_short():
    short_bytes = b"\x00" * (crypto.CLASSICAL_KEY_SIZE - 1)
    sk1 = crypto.ClassicalSecretKey()
    with pytest.raises(ValueError):
        sk1.from_bytes(short_bytes)

    short_bytes = b"\x00" * (crypto.FN_DSA512_KEY_SIZE - 1)
    sk2 = crypto.FnDsa512SecretKey()
    with pytest.raises(ValueError):
        sk2.from_bytes(short_bytes)


def test_secret_keyer_from_string_relaxed_invalid_format():
    with pytest.raises(ValueError):
        crypto.secret_keyer_from_string_relaxed("not a valid hex string")
    with pytest.raises(ValueError):
        crypto.secret_keyer_from_string_relaxed("!!!invalid base58!!!")
    with pytest.raises(ValueError):
        crypto.secret_keyer_from_string_relaxed("[1,2,3]")


def test_classical_secret_key_native_secp256k1():
    sk = crypto.as_classical_secret_key(crypto.new_classical_secret_key())

    native = sk.to_secp256k1()

    decoded = crypto.ClassicalSecretKey()
    decoded.from_secp256k1_native(native)
    assert sk.as_bytes() == decoded.as_bytes()


def test_classical_secret_key_native_ed25519():
    sk = crypto.as_classical_secret_key(crypto.new_pure_classical_secret_key())

    native = sk.to_ed25519()

    decoded = crypto.ClassicalSecretKey()
    decoded.from_ed25519_native(native)
    assert sk.as_bytes() == decoded.as_bytes()


def test_secret_key_zeroize():
    sk = crypto.new_classical_secret_key()
    assert sk.as_bytes() != b"\x00" * crypto.CLASSICAL_KEY_SIZE
    sk.zeroize()
    assert sk.as_bytes() == b"\x00" * crypto.CLASSICAL_KEY_SIZE


@FNDSA_PENDING
def test_secret_key_zeroize_fndsa():
    sk, _ = crypto.new_fn_dsa512_secret_key()
    assert sk.as_bytes() != b"\x00" * crypto.FN_DSA512_KEY_SIZE
    sk.zeroize()
    assert sk.as_bytes() == b"\x00" * crypto.FN_DSA512_KEY_SIZE


def test_secret_keyer_sign_for():
    sk = crypto.as_classical_secret_key(crypto.new_classical_secret_key())
    msg = b"hello"

    pk = sk.secp256k1_public()
    sig = sk.sign_for(pk, msg)
    sig.verify(msg, pk)

    pk2 = sk.ed25519_public()
    sig2 = sk.sign_for(pk2, msg)
    sig2.verify(msg, pk2)

    pk3 = sk.bls12381_public()
    sig3 = sk.sign_for(pk3, msg)
    sig3.verify(msg, pk3)

    with pytest.raises(ValueError):
        sk.sign_for(
            crypto.PublicKey(crypto.PublicKeyType.FN_DSA512, b"\x00" * 897), msg
        )


@FNDSA_PENDING
def test_secret_keyer_sign_for_fndsa():
    fn_sk, fn_pk = crypto.new_fn_dsa512_secret_key()
    sig4 = fn_sk.sign_for(fn_pk, b"hello")
    sig4.verify(b"hello", fn_pk)
    with pytest.raises(ValueError):
        fn_sk.sign_for(
            crypto.new_pure_classical_secret_key().ed25519_public(), b"hello"
        )
