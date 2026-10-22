"""Address. Faithful port of gosdk-develop/crypto/address.go."""

from __future__ import annotations

import binascii
from typing import Any

import base58

from ..postcard.deserializer import Deserializer
from ..postcard.serializer import Serializer
from .error import InvalidPublicKeyError
from .hash_domain import (
    KEYLESS_ADDRESS_DOMAIN_BYTES,
    PK_ADDRESS_DOMAIN_BYTES,
    PK_ADDRESS_RETRY_DOMAIN_BYTES,
    SEED_ADDRESS32_DOMAIN_BYTES,
    SEED_ADDRESS_DOMAIN_BYTES,
    hash32,
)
from .publickey import PublicKey, PublicKeyType

# AddressRawLen raw byte length of a address
ADDRESS_RAW_LEN = 20

# AddressHexLen hexadecimal literal length of a address
ADDRESS_HEX_LEN = ADDRESS_RAW_LEN * 2

# ContractAppID is the App ID reserved for future third-party contracts; the
# whole address space whose first byte is 0xFF is reserved.
CONTRACT_APP_ID = 0xFF

_MAX_UINT64 = (1 << 64) - 1


def new_address_from_public_key(pk: PublicKey) -> "Address":
    """Derives an ordinary account address from a public key and guarantees
    is_app_reserved() is False.

    KeylessDerived keys use the dedicated KeylessAddressDomainBytes domain. The
    first candidate is the domain-separated Blake3 hash truncated to
    AddressRawLen bytes; when a candidate hits the reserved space it is
    re-hashed with PkAddressRetryDomainBytes, the original domain, the public
    key bytes and a big-endian u64 counter starting from 1. Retrying never
    changes the private key, mnemonic or public key.
    """
    domain = (
        KEYLESS_ADDRESS_DOMAIN_BYTES
        if pk.Variant == PublicKeyType.KEYLESS_DERIVED
        else PK_ADDRESS_DOMAIN_BYTES
    )
    key_bytes = pk.Bytes

    def candidate(counter: int) -> "Address":
        if counter == 0:
            digest = hash32(domain, key_bytes)
        else:
            counter_bytes = counter.to_bytes(8, "big")
            digest = hash32(
                PK_ADDRESS_RETRY_DOMAIN_BYTES, domain, key_bytes, counter_bytes
            )
        return Address(digest[:ADDRESS_RAW_LEN])

    counter = 0
    while True:
        address = candidate(counter)
        if not address.is_app_reserved():
            return address
        if counter == _MAX_UINT64:
            raise RuntimeError("public key address retry counter exhausted")
        counter += 1


def new_address_from_app_seed(app_id: int, seed: int) -> "Address":
    """Derives an App address from (appID, seed). This is a runtime-internal
    derivation without a corresponding private key, not a client API.
    Layout: [appID | seed u64 BE | Blake3(SeedAddressDomainBytes || appID ||
    seed)[0..11]]."""
    if not 0 <= app_id <= 0xFF:
        raise ValueError(f"invalid app id: expected u8, got {app_id}")
    if not 0 <= seed <= _MAX_UINT64:
        raise ValueError(f"invalid seed: expected u64, got {seed}")

    app_id_bytes = bytes([app_id])
    seed_bytes = seed.to_bytes(8, "big")
    digest = hash32(SEED_ADDRESS_DOMAIN_BYTES, app_id_bytes, seed_bytes)

    out = bytearray(ADDRESS_RAW_LEN)
    out[0] = app_id
    out[1:9] = seed_bytes
    out[9:] = digest[:11]
    return Address(bytes(out))


def new_address_from_app_seed32(app_id: int, seed: bytes) -> "Address":
    """Derives a truncated-hash address from (appID, seed) without embedding a
    recoverable seed; this derivation is not guaranteed to be app reserved."""
    if not 0 <= app_id <= 0xFF:
        raise ValueError(f"invalid app id: expected u8, got {app_id}")
    if len(seed) != 32:
        raise ValueError(f"invalid seed length: expected 32, got {len(seed)}")

    digest = hash32(SEED_ADDRESS32_DOMAIN_BYTES, bytes([app_id]), bytes(seed))
    return Address(digest[:ADDRESS_RAW_LEN])


def new_address_from_bytes(data: bytes) -> "Address":
    """Parses an address from a 20-byte slice."""
    if len(data) != ADDRESS_RAW_LEN:
        raise ValueError(
            f"invalid address length: expected {ADDRESS_RAW_LEN}, got "
            f"{len(data)}"
        )
    return Address(bytes(data))


def new_address_from_relaxed(address_relaxed: Any) -> "Address":
    """Parses an address from an Address, a hex string (with or without 0x
    prefix), or a base58 string. Hex-shaped inputs try hex first and fall back
    to base58; other inputs try base58 first and fall back to hex."""
    if isinstance(address_relaxed, Address):
        return address_relaxed
    if isinstance(address_relaxed, str):
        str_val = address_relaxed.strip()

        hex_body = str_val
        if len(str_val) >= 2 and str_val[0] == "0" and str_val[1] in ("x", "X"):
            hex_body = str_val[2:]

        try_hex_first = (
            len(hex_body) == ADDRESS_HEX_LEN and _is_hex_string(hex_body)
        )
        if try_hex_first:
            try:
                return _new_address_from_hex(str_val)
            except ValueError:
                return _new_address_from_base58(str_val)

        try:
            return _new_address_from_base58(str_val)
        except ValueError:
            return _new_address_from_hex(str_val)
    raise ValueError(
        f"unsupported address input type {type(address_relaxed).__name__}"
    )


def _is_hex_string(s: str) -> bool:
    """Reports whether s consists only of hexadecimal digits."""
    return all(c in "0123456789abcdefABCDEF" for c in s)


def _new_address_from_hex(s: str) -> "Address":
    s = s.strip()
    hex_str = s
    if len(s) >= 2 and s[0] == "0" and s[1] in ("x", "X"):
        hex_str = s[2:]

    if len(hex_str) != ADDRESS_HEX_LEN:
        raise ValueError(
            f"invalid hex length: expected {ADDRESS_HEX_LEN}, got "
            f"{len(hex_str)}"
        )

    try:
        buf = binascii.unhexlify(hex_str)
    except (binascii.Error, ValueError) as exc:
        raise ValueError(f"invalid hex string: {exc}") from exc

    return new_address_from_bytes(buf)


def _new_address_from_base58(s: str) -> "Address":
    try:
        buf = base58.b58decode(s)
    except ValueError as exc:
        raise ValueError(
            f"invalid base58 decoded length: expected {ADDRESS_RAW_LEN}, got 0"
        ) from exc

    if len(buf) != ADDRESS_RAW_LEN:
        raise ValueError(
            f"invalid base58 decoded length: expected {ADDRESS_RAW_LEN}, got "
            f"{len(buf)}"
        )

    return new_address_from_bytes(buf)


class Address:
    def __init__(self, data: bytes = b"\x00" * ADDRESS_RAW_LEN) -> None:
        self.Bytes = bytes(data)

    @property
    def bytes_(self) -> bytes:
        return self.Bytes

    def as_bytes(self) -> bytes:
        return self.Bytes

    def to_hex(self) -> str:
        return self.Bytes.hex()

    def to_base58(self) -> str:
        return base58.b58encode(self.Bytes).decode()

    def is_app_reserved(self) -> bool:
        """Reports whether the address is forbidden for ordinary account
        registration and external authentication: every address whose first
        byte is 0xFF, plus any valid u64 seed address of any App, belongs to the
        reserved space. This check does not grant App authorization and does not
        restrict address parsing, receiving or balance queries."""
        if self.Bytes[0] == CONTRACT_APP_ID:
            return True
        _, _, ok = self.app_seed()
        return ok

    def app_seed(self):
        """Re-computes the u64 seed layout and returns (appID, seed, True) only
        when the whole checksum matches. Public-key derived addresses always
        return False; the 32-byte seed derivation does not encode this layout."""
        app_id = self.Bytes[0]
        seed = int.from_bytes(self.Bytes[1:9], "big")

        if self == new_address_from_app_seed(app_id, seed):
            return app_id, seed, True
        return 0, 0, False

    def __str__(self) -> str:
        return self.to_base58()

    def __repr__(self) -> str:
        return f"Address({self.Bytes.hex()})"

    def __eq__(self, other: object) -> bool:
        if isinstance(other, Address):
            return self.Bytes == other.Bytes
        return NotImplemented

    def __hash__(self) -> int:
        return hash(self.Bytes)

    # ---- JSON ----
    def to_json_value(self) -> str:
        return self.to_base58()

    def from_json_value(self, value: Any) -> None:
        if not isinstance(value, str):
            raise ValueError("address JSON must be a string")
        addr = new_address_from_relaxed(value)
        self.Bytes = addr.Bytes

    # ---- Postcard ----
    def marshal_postcard(self, serializer: Serializer) -> None:
        serializer.serialize_fixed_bytes(self.Bytes)

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            buf = deserializer.deserialize_fixed_bytes(ADDRESS_RAW_LEN)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize address bytes: {exc}"
            ) from exc
        addr = new_address_from_bytes(buf)
        self.Bytes = addr.Bytes
