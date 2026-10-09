"""跨语言哈希向量（2026-10-09 由 gosdk-develop /tmp/milonvec/hashvec.go 生成）。

锁死 TxHash/IxHash/AuthMessage/VoteBatchHash 的域哈希实现（端序、字段顺序），
任何回归立即可见。
"""

import milon_sdk.lib as lib
from milon_sdk.crypto import PublicKey, PublicKeyType, new_address_from_public_key
from milon_sdk.lib import IxHashItem, unsigned, vote_batch_hash
from milon_sdk.types.bitbap import Bitmap64

GO = {
    "ixhash1": "a18582b32464a07fdb5b94613e3bb47a7b1fd198e7276a1ec353564911547047",
    "ixhash2": "301cdad84ff99a1076339ec8e0e9a4626ce84aa4ff94a1168ebc202bdc125e92",
    "txhash_nopayer": "6cf62fb6b8d76276e406852551e270f237846539672086e6377d7f01c69f1578",
    "txhash_payer": "feb9638085fae8d87e3cf25e74f48d95d15cc5b38ced449578b7e8c1bb2a9348",
    "authmsg": "2d59fe38082bab6682d2159584fdfc41c0d9a100224637bfd05fa73ee3c0cefe",
    "votebatch": "afceb73eebc352319228b2a966dc5638b17d81f0d1ee1dcdbc929db2798c9a33",
}


def setup_module(module):
    lib.set_chain_id(900000001)


def _fixtures():
    ix1 = bytes([1]) + b"\x00" * 31
    ix2 = bytes([2]) + b"\x00" * 31
    payer = new_address_from_public_key(
        PublicKey(PublicKeyType.ED25519, b"\x00" * 32)
    )
    return ix1, ix2, payer


def test_ix_hash_vectors():
    ix1, ix2, _ = _fixtures()
    tx = lib.Transaction(Stamp=5, Instructions=[ix1, ix2])
    assert tx.ix_hash_from_wire(ix1).hex() == GO["ixhash1"]
    assert tx.ix_hash_from_wire(ix2).hex() == GO["ixhash2"]


def test_tx_hash_vectors():
    ix1, _, payer = _fixtures()
    assert (
        lib.Transaction(Stamp=7, Instructions=[ix1]).tx_hash().hex()
        == GO["txhash_nopayer"]
    )
    tx_p = lib.Transaction(Stamp=7, Payer=payer, Instructions=[ix1])
    assert tx_p.tx_hash().hex() == GO["txhash_payer"]


def test_auth_message_vector():
    ix1, ix2, payer = _fixtures()
    tx = lib.Transaction(Stamp=5, Instructions=[ix1, ix2])
    tx_p = lib.Transaction(Stamp=7, Payer=payer, Instructions=[ix1])

    auth_bit = Bitmap64(1 | (1 << 63))
    ac_sig = unsigned(auth_bit)
    ix_hashes = tx.ix_hashes()
    msg = ac_sig.auth_message(
        payer, tx_p.tx_hash(), [IxHashItem(Index=0, Hash=ix_hashes[0])]
    )
    assert msg.hex() == GO["authmsg"]


def test_vote_batch_hash_vector():
    ix1, ix2, _ = _fixtures()
    tx = lib.Transaction(Stamp=5, Instructions=[ix1, ix2])
    assert vote_batch_hash(Bitmap64(0b011), tx.ix_hashes()).hex() == GO["votebatch"]
