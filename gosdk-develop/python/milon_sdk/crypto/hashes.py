"""BLAKE3 域哈希（对应 Go crypto/hash_domain.go，字节级复刻）。

所有链上哈希统一为：
    digest = BLAKE3(MILON_ROOT || domain || parts...)

注意端序纪律：
- 哈希输入里的整数一律 BigEndian（见 lib.transaction 的 TxHash/IxHash/AuthMessage）；
- 线上 Postcard 的 u64 则是变长编码（LE 7-bit 组）——两者不可混淆。
"""
from __future__ import annotations

import blake3  # type: ignore

# 哈希域常量（与 Go 完全一致，勿改动）
MILON_ROOT_DOMAIN_CONTEXT = "Milon-blake3"
MILON_IX_HASH_DOMAIN_CONTEXT = "milon.ix.v1"
MILON_TX_HASH_DOMAIN_CONTEXT = "milon.tx.v1"
MILON_TX_AUTH_DOMAIN_CONTEXT = "milon.tx.auth.v1"
MILON_BLOCK_HEADER_DOMAIN_CONTEXT = "milon.block.header.v1"
MILON_TX_HISTORY_DOMAIN_CONTEXT = "milon.tx-history.v1"
MILON_TX_BATCH_HASH_DOMAIN_CONTEXT = "milon.tx-batch.v1"
MILON_PK_ADDRESS_DOMAIN_CONTEXT = "milon.address.pk.v1"

_ROOT = MILON_ROOT_DOMAIN_CONTEXT.encode("utf-8")
IX_HASH_DOMAIN = MILON_IX_HASH_DOMAIN_CONTEXT.encode("utf-8")
TX_HASH_DOMAIN = MILON_TX_HASH_DOMAIN_CONTEXT.encode("utf-8")
TX_AUTH_DOMAIN = MILON_TX_AUTH_DOMAIN_CONTEXT.encode("utf-8")
PK_ADDRESS_DOMAIN = MILON_PK_ADDRESS_DOMAIN_CONTEXT.encode("utf-8")


def hasher(domain: bytes) -> blake3.Hasher:
    """创建预置了 ROOT || domain 的 BLAKE3 hasher，支持增量写入。"""
    h = blake3.blake3(b"")  # type: ignore[attr-defined]
    h.update(_ROOT)
    h.update(domain)
    return h


def hash32(domain: bytes, *parts: bytes) -> bytes:
    """BLAKE3(MILON_ROOT || domain || parts...)，返回 32 字节摘要。"""
    h = hasher(domain)
    for part in parts:
        h.update(part)
    return h.digest()
