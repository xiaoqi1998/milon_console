"""Port of gosdk-develop/crypto/test/hash_domain_test.go."""

from milon_sdk.crypto import hash32


def test_hash32_deterministic_and_domain_separated():
    msg = b"hello milon"

    h1 = hash32(b"domain-a", msg)
    h2 = hash32(b"domain-a", msg)
    h3 = hash32(b"domain-b", msg)

    assert h1 == h2
    assert h1 != h3
    assert len(h1) == 32


def test_hash32_parts_equivalent_to_concat():
    domain = b"milon.ix.v1"
    parts = [b"a", b"b", b"c"]

    concat = hash32(domain, b"".join(parts))
    split = hash32(domain, *parts)
    assert concat == split
