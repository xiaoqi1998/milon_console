package crypto

import "lukechampine.com/blake3"

const MilonRootDomainContext = "Milon-blake3"
const MilonIxHashDomainContext = "milon.ix.v1"
const MilonTxHashDomainContext = "milon.tx.v1"
const MilonTxAuthDomainContext = "milon.tx.auth.v1"
const MilonBlockHeaderDomainContext = "milon.block.header.v1"
const MilonTxHistoryDomainContext = "milon.tx-history.v1"
const MilonTxBatchHashDomainContext = "milon.tx-batch.v1"

const PkAddressDomainContext = "milon.address.pk.v1"

// Hash32Hasher 创建已写入 MILON_ROOT_DOMAIN 与 domain 的 Blake3 hasher，供增量 update 使用
func Hash32Hasher(domain []byte) *blake3.Hasher {
	hasher := blake3.New(32, nil)
	hasher.Write([]byte(MilonRootDomainContext))
	hasher.Write(domain)
	return hasher
}

// Hash32 计算 Blake3(MILON_ROOT_DOMAIN || domain || parts...)，输出 32 字节
func Hash32(domain []byte, parts ...[]byte) [32]byte {
	hasher := Hash32Hasher(domain)
	for _, part := range parts {
		hasher.Write(part)
	}

	var result [32]byte
	hasher.Sum(result[:0])
	return result
}

func Hash64(domain []byte, parts ...[]byte) [64]byte {
	hasher := Hash32Hasher(domain)
	for _, part := range parts {
		hasher.Write(part)
	}

	var result [64]byte
	hasher.Sum(result[:0])
	return result
}
