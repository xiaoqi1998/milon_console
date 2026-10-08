"""Blake3 域哈希. Faithful port of gosdk-develop/crypto/hash_domain.go."""

from __future__ import annotations

import blake3

MILON_ROOT_DOMAIN_CONTEXT = "Milon-blake3"
MILON_IX_HASH_DOMAIN_CONTEXT = "milon.ix.v1"
MILON_TX_HASH_DOMAIN_CONTEXT = "milon.tx.v1"
MILON_TX_AUTH_DOMAIN_CONTEXT = "milon.tx.auth.v1"
MILON_BLOCK_HEADER_DOMAIN_CONTEXT = "milon.block.header.v1"
MILON_TX_HISTORY_DOMAIN_CONTEXT = "milon.tx-history.v1"
MILON_TX_BATCH_HASH_DOMAIN_CONTEXT = "milon.tx-batch.v1"
MILON_PK_ADDRESS_DOMAIN_CONTEXT = "milon.address.pk.v1"

# MilonVoteBatchHashDomainContext is the domain of the MIP-25 vote intent hash
# (vote_batch_hash: all ix hashes + the owner's gated ix subset).
MILON_VOTE_BATCH_HASH_DOMAIN_CONTEXT = "milon.ix-auth.batch.v1"

# Pre-allocated domain bytes to avoid per-call string->bytes allocations in
# hot hash paths. Do not modify.
ROOT_DOMAIN_BYTES = MILON_ROOT_DOMAIN_CONTEXT.encode()
IX_HASH_DOMAIN_BYTES = MILON_IX_HASH_DOMAIN_CONTEXT.encode()
TX_HASH_DOMAIN_BYTES = MILON_TX_HASH_DOMAIN_CONTEXT.encode()
TX_AUTH_DOMAIN_BYTES = MILON_TX_AUTH_DOMAIN_CONTEXT.encode()
PK_ADDRESS_DOMAIN_BYTES = MILON_PK_ADDRESS_DOMAIN_CONTEXT.encode()
VOTE_BATCH_HASH_DOMAIN_BYTES = MILON_VOTE_BATCH_HASH_DOMAIN_CONTEXT.encode()


def hasher(domain: bytes) -> blake3.blake3:
    """Creates a Blake3 hasher pre-seeded with MILON_ROOT_DOMAIN and the domain,
    for incremental update use."""
    h = blake3.blake3()
    h.update(ROOT_DOMAIN_BYTES)
    h.update(domain)
    return h


def hash32(domain: bytes, *parts: bytes) -> bytes:
    """Computes Blake3(MILON_ROOT_DOMAIN || domain || parts...), returning a
    32-byte digest."""
    h = hasher(domain)
    for part in parts:
        h.update(part)
    return h.digest()
