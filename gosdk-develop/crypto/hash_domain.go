package crypto

import "lukechampine.com/blake3"

const MilonRootDomainContext = "Milon-blake3"
const MilonIxHashDomainContext = "milon.ix.v1"
const MilonTxHashDomainContext = "milon.tx.v1"
const MilonTxAuthDomainContext = "milon.tx.auth.v1"
const MilonBlockHeaderDomainContext = "milon.block.header.v1"
const MilonTxHistoryDomainContext = "milon.tx-history.v1"
const MilonTxBatchHashDomainContext = "milon.tx-batch.v1"
const MilonPkAddressDomainContext = "milon.address.pk.v1"

// Pre-allocated domain bytes to avoid per-call string->[]byte allocations in
// hot hash paths. Do not modify.
var (
	rootDomainBytes      = []byte(MilonRootDomainContext)
	IxHashDomainBytes    = []byte(MilonIxHashDomainContext)
	TxHashDomainBytes    = []byte(MilonTxHashDomainContext)
	TxAuthDomainBytes    = []byte(MilonTxAuthDomainContext)
	PkAddressDomainBytes = []byte(MilonPkAddressDomainContext)
)

// Hasher creates a Blake3 hasher pre-seeded with MILON_ROOT_DOMAIN and the domain, for incremental update use.
func Hasher(domain []byte) *blake3.Hasher {
	hasher := blake3.New(32, nil)
	hasher.Write(rootDomainBytes)
	hasher.Write(domain)
	return hasher
}

// Hash32 computes Blake3(MILON_ROOT_DOMAIN || domain || parts...), returning a 32-byte digest.
func Hash32(domain []byte, parts ...[]byte) [32]byte {
	hasher := Hasher(domain)
	for _, part := range parts {
		hasher.Write(part)
	}

	var result [32]byte
	hasher.Sum(result[:0])
	return result
}
