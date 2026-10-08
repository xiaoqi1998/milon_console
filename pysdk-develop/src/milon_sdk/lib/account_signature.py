"""AccountSignature. Faithful port of gosdk-develop/lib/accountSignature.go."""

from __future__ import annotations

import struct
from dataclasses import dataclass, field
from typing import List, Optional, Union

from ..api import PackedInstruction, TxHash
from ..crypto import (
    Address,
    PublicKey,
    SecretKeyer,
    Signature,
    SignatureType,
    new_address_from_public_key,
)
from ..crypto.hash_domain import (
    IX_HASH_DOMAIN_BYTES,
    TX_AUTH_DOMAIN_BYTES,
    TX_HASH_DOMAIN_BYTES,
    VOTE_BATCH_HASH_DOMAIN_BYTES,
    hasher,
)
from ..postcard import (
    Deserializer,
    Serializer,
    deserialize_option,
    deserialize_seq,
    serialize_option,
    serialize_seq,
)
from ..types.bitbap import Bitmap64
from .chain_id import GetChainId

AUTH_PAYER_BIT = 63

# AuthReservedBit is bit62: the vote gate flag (MIP-25). It is never used as
# an ix index, so it doubles as the ix range ceiling (ix 0..61).
AUTH_RESERVED_BIT = 62

# AuthVoteBit is the vote gate flag bit in auth_bit (bit62, MIP-25).
AUTH_VOTE_BIT = AUTH_RESERVED_BIT


@dataclass
class PubKeySignatureMode:
    """Signs with a single public key. When SkipPubKey is true, the public key
    is omitted from the wire format and SigBit must be resolved from the
    on-chain signers list."""

    PublicKey: PublicKey = None
    SkipPubKey: bool = False
    SigBit: Bitmap64 = None


@dataclass
class MultisigKeySignatureMode:
    """Signs as one participant of a multisig account. The public key is
    located on-chain by SigBit = 1 << Index in the signers list."""

    Index: int = 0
    PublicKey: PublicKey = None


AccountSignatureMode = Union[PubKeySignatureMode, MultisigKeySignatureMode]


@dataclass
class AccountSignature:
    AuthBit: Bitmap64 = field(default_factory=Bitmap64)
    SigBit: Bitmap64 = field(default_factory=Bitmap64)
    Signatures: List[Signature] = field(default_factory=list)
    PubKey: Optional[PublicKey] = None

    def add_multisig_key(self, key_index: int, signature: Signature) -> None:
        """Appends a multisig key signature under the same auth_bit."""
        if key_index >= 64:
            raise ValueError(f"key index {key_index} out of range (max 63)")
        if self.PubKey is not None:
            raise ValueError("pubkey mode cannot add multisig keys")
        self.SigBit = self.SigBit.set(key_index)
        self.Signatures.append(signature)

    def authorizes_ix(self, ix: int) -> bool:
        return self.AuthBit.test(ix)

    def authorizes_vote(self) -> bool:
        return self.AuthBit.test(AUTH_VOTE_BIT)

    def authorizes_payer(self) -> bool:
        return self.AuthBit.test(AUTH_PAYER_BIT)

    def auth_message(
        self, account: Address, tx_hash: TxHash, ix_hashes: List["IxHashItem"]
    ) -> TxHash:
        """Blake3(MILON_ROOT || TX_AUTH_DOMAIN || chain_id || owner || auth_bit
        || tx_hash || ixHashes)。chain_id/auth_bit 为大端/小端定长 8 字节。"""
        h = hasher(TX_AUTH_DOMAIN_BYTES)

        h.update(struct.pack(">Q", GetChainId()))
        h.update(account.as_bytes())
        h.update(struct.pack("<Q", self.AuthBit.raw()))
        h.update(tx_hash)

        for item in ix_hashes:
            if not self.AuthBit.test(item.Index):
                raise ValueError(
                    f"ix index {item.Index} is not authorized in auth_bit"
                )
            h.update(item.Hash)

        return h.digest()

    def auth_message_for_tx(
        self, account: Address, tx_hash: TxHash, ix_hashes: List[TxHash]
    ) -> TxHash:
        ix_part = collect_ix_hashes(self.AuthBit, ix_hashes)
        return self.auth_message(account, tx_hash, ix_part)

    def is_vote_gate_only(self) -> bool:
        """Vote-gated auth-bit-only ticket (MIP-25)."""
        return (
            self.authorizes_vote()
            and self.PubKey is None
            and len(self.Signatures) == 0
            and self.SigBit.raw() == 0
            and (self.AuthBit.raw() & ((1 << AUTH_RESERVED_BIT) - 1)) != 0
        )

    def marshal_postcard(self, serializer: Serializer) -> None:
        try:
            serializer.serialize_u64(self.AuthBit.raw())
        except ValueError as exc:
            raise ValueError(f"failed to serialize AuthBit: {exc}") from exc
        try:
            serializer.serialize_u64(self.SigBit.raw())
        except ValueError as exc:
            raise ValueError(f"failed to serialize SigBit: {exc}") from exc
        try:
            serialize_seq(
                serializer, self.Signatures, lambda s, sig: sig.marshal_postcard(s)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize TxSigs: {exc}") from exc
        try:
            serialize_option(
                serializer, self.PubKey, lambda s, pk: pk.marshal_postcard(s)
            )
        except ValueError as exc:
            raise ValueError(f"failed to serialize PubKey: {exc}") from exc

    def unmarshal_postcard(self, deserializer: Deserializer) -> None:
        try:
            self.AuthBit = Bitmap64(deserializer.deserialize_u64())
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize AuthBit: {exc}"
            ) from exc
        try:
            self.SigBit = Bitmap64(deserializer.deserialize_u64())
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize SigBit: {exc}"
            ) from exc

        def _read_sig(d: Deserializer) -> Signature:
            sig = Signature()
            sig.unmarshal_postcard(d)
            return sig

        try:
            self.Signatures = deserialize_seq(deserializer, _read_sig)
        except ValueError as exc:
            raise ValueError(f"failed to deserialize TxSigs: {exc}") from exc

        def _read_pk(d: Deserializer) -> PublicKey:
            pk = PublicKey()
            pk.unmarshal_postcard(d)
            return pk

        try:
            self.PubKey = deserialize_option(deserializer, _read_pk)
        except ValueError as exc:
            raise ValueError(
                f"failed to deserialize PubKey: {exc}"
            ) from exc


@dataclass
class IxHashItem:
    """An instruction hash paired with its ix index."""

    Index: int = 0
    Hash: TxHash = b"\x00" * 32


def auth_ix(ix: int) -> Bitmap64:
    """Creates the auth bitmap for a single ix (max ix = 61)."""
    if ix >= AUTH_RESERVED_BIT:
        raise ValueError(
            f"ix index {ix} out of range (max {AUTH_RESERVED_BIT - 1})"
        )
    return Bitmap64(1 << ix)


def auth_ixes(indices) -> Bitmap64:
    """Creates the auth bitmap for multiple ixs (max ix = 61)."""
    raw = 0
    for ix in indices:
        if ix >= AUTH_RESERVED_BIT:
            raise ValueError(
                f"ix index {ix} out of range (max {AUTH_RESERVED_BIT - 1})"
            )
        raw |= 1 << ix
    return Bitmap64(raw)


def auth_payer() -> Bitmap64:
    """Creates the payer auth bitmap (bit63)."""
    return Bitmap64(1 << AUTH_PAYER_BIT)


def auth_ix_and_payer(ix: int) -> Bitmap64:
    """Creates the combined ix + payer auth bitmap."""
    if ix >= AUTH_RESERVED_BIT:
        raise ValueError(
            f"ix index {ix} out of range (max {AUTH_RESERVED_BIT - 1})"
        )
    return Bitmap64((1 << ix) | (1 << AUTH_PAYER_BIT))


def auth_vote_ixes(indices) -> Bitmap64:
    """Creates the ix authorization bits plus the vote gate flag (bit62)."""
    ixbits = auth_ixes(indices)
    return Bitmap64(ixbits.raw() | (1 << AUTH_VOTE_BIT))


def unsigned(auth_bit: Bitmap64) -> AccountSignature:
    """Creates an unsigned AccountSignature with only the auth_bit set."""
    return AccountSignature(AuthBit=auth_bit, SigBit=Bitmap64(0))


def _resolve_mode(account: Address, mode: AccountSignatureMode):
    """Validates the signature mode against the owner and derives the signing
    public key, sigBit and PubKey field."""
    if isinstance(mode, PubKeySignatureMode):
        pk_addr = new_address_from_public_key(mode.PublicKey)
        if pk_addr.Bytes != account.Bytes:
            raise ValueError("public key does not match owner address")
        if mode.SkipPubKey:
            if mode.SigBit is None or mode.SigBit.raw() == 0:
                raise ValueError(
                    "SkipPubKey requires SigBit resolved from the on-chain "
                    "signers list"
                )
            return mode.PublicKey, mode.SigBit, None
        return mode.PublicKey, Bitmap64(0), mode.PublicKey
    if isinstance(mode, MultisigKeySignatureMode):
        if mode.Index >= 64:
            raise ValueError(
                f"multisig key index {mode.Index} out of range (max 63)"
            )
        return mode.PublicKey, Bitmap64(1 << mode.Index), None
    raise ValueError("invalid signature mode")


def sign(
    account: Address,
    sk: SecretKeyer,
    auth_bit: Bitmap64,
    tx_hash: TxHash,
    ix_hashes: List[IxHashItem],
    mode: AccountSignatureMode,
) -> AccountSignature:
    """Computes the auth message for the given auth context and signs it."""
    public_key, sig_bit, pub_key_field = _resolve_mode(account, mode)

    account_signature = AccountSignature(
        AuthBit=auth_bit, SigBit=sig_bit, PubKey=pub_key_field
    )

    try:
        auth_hash = account_signature.auth_message(account, tx_hash, ix_hashes)
    except ValueError as exc:
        raise ValueError(f"failed to compute auth message: {exc}") from exc

    try:
        signature = sk.sign_for(public_key, auth_hash)
    except ValueError as exc:
        raise ValueError(f"failed to sign message: {exc}") from exc

    account_signature.Signatures = [signature]
    return account_signature


def simulate_sign(
    account: Address, auth_bit: Bitmap64, mode: AccountSignatureMode
) -> AccountSignature:
    """Computes the auth context without signing: a zero-filled placeholder
    signature keeps the wire size identical for accurate gas simulation."""
    public_key, sig_bit, pub_key_field = _resolve_mode(account, mode)
    return AccountSignature(
        AuthBit=auth_bit,
        SigBit=sig_bit,
        Signatures=[placeholder_signature(public_key)],
        PubKey=pub_key_field,
    )


def placeholder_signature(pk: PublicKey) -> Signature:
    """Zero-filled signature whose length matches the public key type."""
    return Signature(SignatureType(int(pk.Variant)), b"\x00" * _signature_size_for_public_key(pk))


def _signature_size_for_public_key(pk: PublicKey) -> int:
    from ..crypto import (
        PUBLIC_KEY_BLS12381_SIZE,
        PUBLIC_KEY_ED25519_SIZE,
        PUBLIC_KEY_FN_DSA512_SIZE,
        PUBLIC_KEY_SECP256K1_SIZE,
        SIGNATURE_BLS12381_SIZE,
        SIGNATURE_ED25519_SIZE,
        SIGNATURE_FN_DSA512_SIZE,
        SIGNATURE_SECP256K1_SIZE,
        PublicKeyType,
    )

    return {
        PublicKeyType.SECP256K1: SIGNATURE_SECP256K1_SIZE,
        PublicKeyType.ED25519: SIGNATURE_ED25519_SIZE,
        PublicKeyType.BLS12381: SIGNATURE_BLS12381_SIZE,
        PublicKeyType.FN_DSA512: SIGNATURE_FN_DSA512_SIZE,
    }.get(PublicKeyType(int(pk.Variant)), 0)


def collect_ix_hashes(auth_bit: Bitmap64, ix_hashes: List[TxHash]) -> List[IxHashItem]:
    """Collects the IxHashItems for the ix bits set in auth_bit."""
    out = []
    for i in range(AUTH_RESERVED_BIT):
        if not auth_bit.test(i):
            continue
        if i < len(ix_hashes):
            out.append(IxHashItem(Index=i, Hash=ix_hashes[i]))
    return out


def vote_batch_hash(auth_bit: Bitmap64, all_ix_hashes: List[TxHash]) -> TxHash:
    """Computes the MIP-25 vote intent hash:
    Blake3(MILON_ROOT || VOTE_BATCH_HASH_DOMAIN || all_ix_hashes... ||
    auth_subset_ix_hashes...)。"""
    h = hasher(VOTE_BATCH_HASH_DOMAIN_BYTES)
    for x in all_ix_hashes:
        h.update(x)
    for i in range(64):
        if i == AUTH_PAYER_BIT or i == AUTH_VOTE_BIT or not auth_bit.test(i):
            continue
        if i >= len(all_ix_hashes):
            continue
        h.update(all_ix_hashes[i])
    return h.digest()
