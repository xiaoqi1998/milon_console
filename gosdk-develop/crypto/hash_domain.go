package crypto

import (
	"sync"

	"lukechampine.com/blake3"
)

const MilonRootDomainContext = "Milon-blake3"
const MilonIxHashDomainContext = "milon.ix.v1"
const MilonTxHashDomainContext = "milon.tx.v1"
const MilonTxAuthDomainContext = "milon.tx.auth.v1"
const MilonBlockHeaderDomainContext = "milon.block.header.v1"
const MilonTxHistoryDomainContext = "milon.tx-history.v1"
const MilonTxBatchHashDomainContext = "milon.tx-batch.v1"
const MilonPkAddressDomainContext = "milon.address.pk.v1"
const MilonPkAddressRetryDomainContext = "milon.address.pk.retry.v1"
const MilonKeylessAddressDomainContext = "milon.address.keyless.v1"
const MilonSeedAddressDomainContext = "milon.seed-address.v3"
const MilonSeedAddress32DomainContext = "milon.seed-address.v2"

// MilonVoteBatchHashDomainContext is the domain of the MIP-25 vote intent hash (vote_batch_hash: all ix hashes + the owner's gated ix subset).
const MilonVoteBatchHashDomainContext = "milon.ix-auth.batch.v1"

// Pre-allocated domain bytes to avoid per-call string->[]byte allocations in
// hot hash paths. Do not modify.
var (
	rootDomainBytes           = []byte(MilonRootDomainContext)
	IxHashDomainBytes         = []byte(MilonIxHashDomainContext)
	TxHashDomainBytes         = []byte(MilonTxHashDomainContext)
	TxAuthDomainBytes         = []byte(MilonTxAuthDomainContext)
	PkAddressDomainBytes      = []byte(MilonPkAddressDomainContext)
	PkAddressRetryDomainBytes = []byte(MilonPkAddressRetryDomainContext)
	KeylessAddressDomainBytes = []byte(MilonKeylessAddressDomainContext)
	SeedAddressDomainBytes    = []byte(MilonSeedAddressDomainContext)
	SeedAddress32DomainBytes  = []byte(MilonSeedAddress32DomainContext)
	VoteBatchHashDomainBytes  = []byte(MilonVoteBatchHashDomainContext)
)

// hasherPool reuses Blake3 hashers: a hasher carries much more internal state
// than its 32-byte digest, and transaction hashing acquires one several times
// per signed instruction.
var hasherPool = sync.Pool{
	New: func() any { return blake3.New(32, nil) },
}

// AcquireHasher returns a pooled hasher pre-seeded with MILON_ROOT_DOMAIN and
// the domain. Hand it back with ReleaseHasher once the digest has been copied
// out, typically via defer.
func AcquireHasher(domain []byte) *blake3.Hasher {
	hasher := hasherPool.Get().(*blake3.Hasher)
	hasher.Reset()
	hasher.Write(rootDomainBytes)
	hasher.Write(domain)
	return hasher
}

// ReleaseHasher returns a hasher obtained from AcquireHasher to the pool. The
// digest must already be copied out of it: the hasher is reset on the next
// acquire, and using it after release corrupts the next caller's digest.
func ReleaseHasher(hasher *blake3.Hasher) {
	if hasher == nil {
		return
	}
	hasherPool.Put(hasher)
}

// Hasher creates a Blake3 hasher pre-seeded with MILON_ROOT_DOMAIN and the domain, for incremental update use.
// The hasher stays owned by the caller, so it is not pooled; hot paths should
// use AcquireHasher/ReleaseHasher instead.
func Hasher(domain []byte) *blake3.Hasher {
	hasher := blake3.New(32, nil)
	hasher.Write(rootDomainBytes)
	hasher.Write(domain)
	return hasher
}

// Hash32 computes Blake3(MILON_ROOT_DOMAIN || domain || parts...), returning a 32-byte digest.
func Hash32(domain []byte, parts ...[]byte) [32]byte {
	hasher := AcquireHasher(domain)
	defer ReleaseHasher(hasher)

	for _, part := range parts {
		hasher.Write(part)
	}

	var result [32]byte
	hasher.Sum(result[:0])
	return result
}
