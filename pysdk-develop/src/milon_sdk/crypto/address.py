"""Address. Faithful port of gosdk-develop/crypto/address.go."""

from __future__ import annotations

import binascii
from typing import Any

import base58

from ..postcard.deserializer import Deserializer
from ..postcard.serializer import Serializer
from .error import InvalidPublicKeyError
from .hash_domain import PK_ADDRESS_DOMAIN_BYTES, hash32
from .publickey import PublicKey

# AddressRawLen raw byte length of a address
ADDRESS_RAW_LEN = 20

# AddressHexLen hexadecimal literal length of a address
ADDRESS_HEX_LEN = ADDRESS_RAW_LEN * 2


def new_address_from_public_key(pk: PublicKey) -> "Address":
    """Derives a address from a public key."""
    digest = hash32(PK_ADDRESS_DOMAIN_BYTES, pk.Bytes)
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
    prefix), or a base58 string."""
    if isinstance(address_relaxed, Address):
        return address_relaxed
    if isinstance(address_relaxed, str):
        str_val = address_relaxed.strip()

        hex_body = str_val
        if len(str_val) >= 2 and str_val[0] == "0" and str_val[1] in ("x", "X"):
            hex_body = str_val[2:]
        if len(hex_body) == ADDRESS_HEX_LEN:
            return _new_address_from_hex(hex_body)

        return _new_address_from_base58(str_val)
    raise ValueError(
        f"unsupported address input type {type(address_relaxed).__name__}"
    )


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
