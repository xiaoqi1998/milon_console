package milon

import (
	"fmt"
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestNewTransactionWithParam(t *testing.T) {
	instructions := []api.PackedInstruction{
		{1, 2, 3},
	}
	tx, err := NewTransactionWithParam(instructions, nil)
	assert.NoError(t, err)

	assert.Greater(t, tx.Stamp, uint64(0))
	assert.Equal(t, (*crypto.Address)(nil), tx.Payer)
	assert.Equal(t, instructions, tx.Instructions)
	assert.Equal(t, []TransactionSignatures{}, tx.TxSigs)
}

func TestNewTransactionWithStamp(t *testing.T) {
	t.Run("with payer set", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		instructions := []api.PackedInstruction{
			{10, 11, 12},
		}
		stamp := TransactionStamp(1234567890)
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{10, 11, 12}}, payer, stamp)
		assert.NoError(t, err)

		assert.Equal(t, stamp, tx.Stamp)
		assert.Equal(t, payer, tx.Payer)
		assert.Equal(t, instructions, tx.Instructions)
		assert.Equal(t, []TransactionSignatures{}, tx.TxSigs)
	})

	t.Run("without payer (split mode)", func(t *testing.T) {
		instructions := []api.PackedInstruction{
			{13, 14, 15},
		}
		stamp := TransactionStamp(9876543210)
		tx, err := NewTransactionWithParam(instructions, nil, stamp)
		assert.NoError(t, err)

		assert.Equal(t, stamp, tx.Stamp)
		assert.Equal(t, (*crypto.Address)(nil), tx.Payer)
		assert.Equal(t, instructions, tx.Instructions)
		assert.Equal(t, []TransactionSignatures{}, tx.TxSigs)
	})
}

func TestNewTransactionWithSigned(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(sk.Ed25519Public())
	assert.NoError(t, err)

	stamp := TransactionStamp(1234567890)
	instructions := []api.PackedInstruction{
		{19, 20, 21},
	}
	tx, err := NewTransactionWithParam(instructions, nil, stamp)
	assert.NoError(t, err)

	acSig, err := tx.SignIx(*owner, sk, 0, PubKeySignatureMode{PublicKey: *pubKey})
	assert.NoError(t, err)

	txSigs := []TransactionSignatures{{Address: *owner, AccountSignature: *acSig}}
	txSigned, err := NewTransactionWithParam(instructions, nil, stamp, txSigs)
	assert.NoError(t, err)

	assert.Equal(t, stamp, txSigned.Stamp)
	assert.Equal(t, (*crypto.Address)(nil), txSigned.Payer)
	assert.Equal(t, instructions, txSigned.Instructions)
	assert.Equal(t, txSigs, txSigned.TxSigs)
}

func TestNewTransactionFromBytes(t *testing.T) {
	stamp := TransactionStamp(1234567890)
	original, err := NewTransactionWithParam([]api.PackedInstruction{{16, 17, 18}}, nil, stamp)
	assert.NoError(t, err)

	data, err := original.ToBytes()
	assert.NoError(t, err)

	deserialized, err := NewTransactionFromBytes(data)
	assert.NoError(t, err)

	assert.Equal(t, original.Stamp, deserialized.Stamp)
	assert.Equal(t, original.Payer, deserialized.Payer)
	assert.Equal(t, original.Instructions, deserialized.Instructions)
	assert.Equal(t, original.TxSigs, deserialized.TxSigs)
}

func TestTransaction_TxHash(t *testing.T) {
	t.Run("different hash with different stamp", func(t *testing.T) {
		tx1, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil, TransactionStamp(1234567890))
		assert.NoError(t, err)

		tx2, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil, TransactionStamp(9876543210))
		assert.NoError(t, err)

		assert.NotEqual(t, tx1.TxHash(), tx2.TxHash())
	})

	t.Run("different hash with different instructions", func(t *testing.T) {
		tx1, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil, TransactionStamp(1234567890))
		assert.NoError(t, err)

		tx2, err := NewTransactionWithParam([]api.PackedInstruction{{4, 5, 6}}, nil, TransactionStamp(1234567890))
		assert.NoError(t, err)

		assert.NotEqual(t, tx1.TxHash(), tx2.TxHash())
	})

	t.Run("different hash with different payer", func(t *testing.T) {
		tx1, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil, TransactionStamp(1234567890))
		assert.NoError(t, err)

		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		payer, err := crypto.NewAddressFromPublicKey(sk.Ed25519Public())
		assert.NoError(t, err)

		tx2, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, payer, TransactionStamp(1234567890))
		assert.NoError(t, err)

		assert.NotEqual(t, tx1.TxHash(), tx2.TxHash())
	})
}

func TestTransaction_AddSignature(t *testing.T) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
	assert.NoError(t, err)

	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	acSig, err := tx.SignIx(*owner, sk, 0, PubKeySignatureMode{PublicKey: *pubKey})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<0, acSig.AuthBit.Raw())
	assert.Equal(t, uint64(0), acSig.SigBit.Raw()) //PubKeySignatureMode always has SigBit=0
	assert.Equal(t, 1, len(acSig.Signatures))
	assert.Equal(t, pubKey, acSig.PubKey)

	tx.AddSignature(*owner, *acSig)

	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *owner, tx.TxSigs[0].Address)
	assert.Equal(t, *acSig, tx.TxSigs[0].AccountSignature)
}

func TestTransaction_PushSignatures(t *testing.T) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}, nil)
	assert.NoError(t, err)
	assert.Len(t, tx.TxSigs, 0)

	var txSigs []TransactionSignatures
	for i := 0; i < 2; i++ {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		acSig, err := tx.SignIx(*owner, sk, uint8(i), PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<i, acSig.AuthBit.Raw())
		assert.Equal(t, uint64(0), acSig.SigBit.Raw()) //PubKeySignatureMode always has SigBit=0
		assert.Equal(t, 1, len(acSig.Signatures))
		assert.Equal(t, pubKey, acSig.PubKey)

		txSigs = append(txSigs, TransactionSignatures{
			Address:          *owner,
			AccountSignature: *acSig,
		})
	}

	tx.PushSignatures(txSigs)
	assert.Len(t, tx.TxSigs, 2)

	for i := range tx.TxSigs {
		assert.Equal(t, txSigs[i].Address, tx.TxSigs[i].Address)
		assert.Equal(t, txSigs[i].AccountSignature, tx.TxSigs[i].AccountSignature)
	}
}

func TestTransaction_SignIx_PubKeySignatureMode(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
	assert.NoError(t, err)

	acSig, err := tx.SignIx(*owner, sk, 0, PubKeySignatureMode{PublicKey: *pubKey})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<0, acSig.AuthBit.Raw())
	assert.Equal(t, uint64(0), acSig.SigBit.Raw()) //PubKeySignatureMode always has SigBit=0
	assert.Equal(t, 1, len(acSig.Signatures))
	assert.Equal(t, pubKey, acSig.PubKey)
	assert.True(t, acSig.AuthorizesIx(0))

	// Verify signature
	msg, err := acSig.AuthMessage(*owner, tx.TxHash(), []IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}})
	assert.NoError(t, err)
	assert.NoError(t, acSig.Signatures[0].Verify(msg[:], pubKey))
}

func TestTransaction_SignIx_MultisigKeySignatureMode(t *testing.T) {
	// === Scenario ===
	// Simulates a multi-sig wallet controlled by 3 keys: key0, key1, key2
	// The multi-sig wallet address is jointly controlled by these 3 keys

	// Generate 3 keys
	sk0 := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey0 := sk0.Ed25519Public()

	sk1 := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey1 := sk1.Ed25519Public()

	sk2 := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey2 := sk2.Ed25519Public()

	// Multi-sig wallet address (in practice, this address must support multi-sig verification)
	// Simplified here: use the first key's address as the owner
	multisigWallet, err := crypto.NewAddressFromPublicKey(pubKey0)
	assert.NoError(t, err)

	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}, nil)
	assert.NoError(t, err)

	ixIdx := uint8(1) // The instruction index to sign

	// === Key0 signs (position 0 in multi-sig list) ===
	acSig0, err := tx.SignIx(*multisigWallet, sk0, ixIdx, MultisigKeySignatureMode{
		Index:     0,        // Position 0 in multi-sig list
		PublicKey: *pubKey0, // Public key of key0
	})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<ixIdx, acSig0.AuthBit.Raw()) // AuthBit = bit1 (authorizes ix=1)
	assert.Equal(t, uint64(1)<<0, acSig0.SigBit.Raw())      // SigBit = bit0 (multi-sig index 0)
	assert.Equal(t, 1, len(acSig0.Signatures))
	assert.Nil(t, acSig0.PubKey)
	assert.True(t, acSig0.AuthorizesIx(1))

	// Verify signature
	msg0, err := acSig0.AuthMessage(*multisigWallet, tx.TxHash(), []IxHashItem{{Index: ixIdx, Hash: tx.IxHashes()[ixIdx]}})
	assert.NoError(t, err)
	assert.NoError(t, acSig0.Signatures[0].Verify(msg0[:], pubKey0))
	fmt.Printf("key0 signature: AuthBit=%064b, SigBit=%064b\n", acSig0.AuthBit.Raw(), acSig0.SigBit.Raw())

	// === Key1 signs (position 1 in multi-sig list) ===
	acSig1, err := tx.SignIx(*multisigWallet, sk1, ixIdx, MultisigKeySignatureMode{
		Index:     1,        // Position 1 in multi-sig list
		PublicKey: *pubKey1, // Public key of key1
	})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<ixIdx, acSig1.AuthBit.Raw()) // AuthBit = bit1 (authorizes ix=1)
	assert.Equal(t, uint64(1)<<1, acSig1.SigBit.Raw())      // SigBit = bit1 (multi-sig index 1)
	assert.Equal(t, 1, len(acSig1.Signatures))
	assert.Nil(t, acSig1.PubKey)
	assert.True(t, acSig1.AuthorizesIx(1))

	// Verify signature
	msg1, err := acSig1.AuthMessage(*multisigWallet, tx.TxHash(), []IxHashItem{{Index: ixIdx, Hash: tx.IxHashes()[ixIdx]}})
	assert.NoError(t, err)
	assert.NoError(t, acSig1.Signatures[0].Verify(msg1[:], pubKey1))
	fmt.Printf("key1 signature: AuthBit=%064b, SigBit=%064b\n", acSig1.AuthBit.Raw(), acSig1.SigBit.Raw())

	// === Key2 signs (position 5 in multi-sig list, skipping some positions) ===
	acSig2, err := tx.SignIx(*multisigWallet, sk2, ixIdx, MultisigKeySignatureMode{
		Index:     5,        // Position 5 in multi-sig list
		PublicKey: *pubKey2, // Public key of key2
	})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<ixIdx, acSig2.AuthBit.Raw()) // AuthBit = bit1 (authorizes ix=1)
	assert.Equal(t, uint64(1)<<5, acSig2.SigBit.Raw())      // SigBit = bit5 (multi-sig index 5)
	assert.Equal(t, 1, len(acSig2.Signatures))
	assert.Nil(t, acSig2.PubKey)
	assert.True(t, acSig2.AuthorizesIx(1))

	// Verify signature
	msg2, err := acSig2.AuthMessage(*multisigWallet, tx.TxHash(), []IxHashItem{{Index: ixIdx, Hash: tx.IxHashes()[ixIdx]}})
	assert.NoError(t, err)
	assert.NoError(t, acSig2.Signatures[0].Verify(msg2[:], pubKey2))
	fmt.Printf("key2 signature: AuthBit=%064b, SigBit=%064b\n", acSig2.AuthBit.Raw(), acSig2.SigBit.Raw())

	// === Comparison: PubKeySignatureMode (single-signature mode) ===
	acSigSingle, err := tx.SignIx(*multisigWallet, sk0, ixIdx, PubKeySignatureMode{PublicKey: *pubKey0})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<ixIdx, acSigSingle.AuthBit.Raw()) // AuthBit = bit1 (authorizes ix=1)
	assert.Equal(t, uint64(0), acSigSingle.SigBit.Raw())         // PubKeySignatureMode always has SigBit=0
	assert.Equal(t, 1, len(acSigSingle.Signatures))
	assert.Equal(t, pubKey0, acSigSingle.PubKey)
	fmt.Printf("single-signature mode:   AuthBit=%064b, SigBit=%064b, PubKey!=nil(%v)\n", acSigSingle.AuthBit.Raw(), acSigSingle.SigBit.Raw(), acSigSingle.PubKey != nil)
}

func TestTransaction_SignIx_OutOfRange(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
	assert.NoError(t, err)

	_, err = tx.SignIx(*owner, sk, 5, PubKeySignatureMode{PublicKey: *pubKey})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestTransaction_SignPayer(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	payer, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
	assert.NoError(t, err)

	acSig, err := tx.SignPayer(*payer, sk, PubKeySignatureMode{PublicKey: *pubKey})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<AuthPayerBit, acSig.AuthBit.Raw())
	assert.Equal(t, uint64(0), acSig.SigBit.Raw()) //PubKeySignatureMode always has SigBit=0
	assert.Equal(t, 1, len(acSig.Signatures))
	assert.Equal(t, pubKey, acSig.PubKey)
	assert.True(t, acSig.AuthorizesPayer())

	// Verify signature
	msg, err := acSig.AuthMessage(*payer, tx.TxHash(), []IxHashItem{})
	assert.NoError(t, err)
	assert.NoError(t, acSig.Signatures[0].Verify(msg[:], pubKey))
}

func TestTransaction_SignIxAndPayer(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
	assert.NoError(t, err)

	acSig, err := tx.SignIxAndPayer(*owner, sk, 0, PubKeySignatureMode{PublicKey: *pubKey})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<0|uint64(1)<<AuthPayerBit, acSig.AuthBit.Raw())
	assert.Equal(t, uint64(0), acSig.SigBit.Raw()) //PubKeySignatureMode always has SigBit=0
	assert.Equal(t, 1, len(acSig.Signatures))
	assert.Equal(t, pubKey, acSig.PubKey)
	assert.True(t, acSig.AuthorizesPayer())
	assert.True(t, acSig.AuthorizesIx(0))

	// Verify signature
	msg, err := acSig.AuthMessage(*owner, tx.TxHash(), []IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}})
	assert.NoError(t, err)
	assert.NoError(t, acSig.Signatures[0].Verify(msg[:], pubKey))
}

func TestTransaction_SignIxes(t *testing.T) {
	t.Run("sign multiple ixes without payer", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, nil)
		assert.NoError(t, err)

		acSig, err := tx.SignIxes(*owner, sk, []uint8{0, 2}, false, PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0|uint64(1)<<2, acSig.AuthBit.Raw())
		assert.Equal(t, uint64(0), acSig.SigBit.Raw()) //PubKeySignatureMode always has SigBit=0
		assert.Equal(t, 1, len(acSig.Signatures))
		assert.Equal(t, pubKey, acSig.PubKey)
		assert.False(t, acSig.AuthorizesPayer())
		assert.True(t, acSig.AuthorizesIx(0))
		assert.True(t, acSig.AuthorizesIx(2))

		// Verify signature
		msg, err := acSig.AuthMessage(*owner, tx.TxHash(), []IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}, {Index: 2, Hash: tx.IxHashes()[2]}})
		assert.NoError(t, err)
		assert.NoError(t, acSig.Signatures[0].Verify(msg[:], pubKey))
	})

	t.Run("sign multiple ixes with payer", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, owner)
		assert.NoError(t, err)

		acSig, err := tx.SignIxes(*owner, sk, []uint8{0, 2}, true, PubKeySignatureMode{PublicKey: *pubKey})
		assert.Equal(t, uint64(1)<<0|uint64(1)<<2|uint64(1)<<AuthPayerBit, acSig.AuthBit.Raw())
		assert.Equal(t, uint64(0), acSig.SigBit.Raw()) //PubKeySignatureMode always has SigBit=0
		assert.Equal(t, 1, len(acSig.Signatures))
		assert.Equal(t, pubKey, acSig.PubKey)
		assert.True(t, acSig.AuthorizesPayer())
		assert.True(t, acSig.AuthorizesIx(0))
		assert.True(t, acSig.AuthorizesIx(2))

		// Verify signature
		msg, err := acSig.AuthMessage(*owner, tx.TxHash(), []IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}, {Index: 2, Hash: tx.IxHashes()[2]}})
		assert.NoError(t, err)
		assert.NoError(t, acSig.Signatures[0].Verify(msg[:], pubKey))
	})
}

func TestTransaction_SignIxGas(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
	assert.NoError(t, err)

	acSig, err := tx.SignIxGas(*owner, sk, 0, PubKeySignatureMode{PublicKey: *pubKey})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<0|uint64(1)<<AuthPayerBit, acSig.AuthBit.Raw())
	assert.Equal(t, uint64(0), acSig.SigBit.Raw()) //PubKeySignatureMode always has SigBit=0
	assert.Equal(t, 1, len(acSig.Signatures))
	assert.Equal(t, pubKey, acSig.PubKey)
	assert.True(t, acSig.AuthorizesPayer())
	assert.True(t, acSig.AuthorizesIx(0))

	// Verify signature
	msg, err := acSig.AuthMessage(*owner, tx.TxHash(), []IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}})
	assert.NoError(t, err)
	assert.NoError(t, acSig.Signatures[0].Verify(msg[:], pubKey))
}

func TestTransaction_IxHashes(t *testing.T) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, nil)
	assert.NoError(t, err)

	hashes := tx.IxHashes()
	assert.Len(t, hashes, 3)

	for i, ix := range tx.Instructions {
		assert.Equal(t, tx.ixHashFromWire(ix), hashes[i])
	}
}

func TestTransaction_ixHashFromWire(t *testing.T) {
	t.Run("different ix_hash for different wire content", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		wire1 := api.PackedInstruction{1, 2, 3}
		wire2 := api.PackedInstruction{4, 5, 6}

		assert.NotEqual(t, tx.ixHashFromWire(wire1), tx.ixHashFromWire(wire2))
	})

	t.Run("ix_hash does not include payer", func(t *testing.T) {
		wire := api.PackedInstruction{1, 2, 3}

		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		tx1, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil, TransactionStamp(1234567890))
		assert.NoError(t, err)

		tx2, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, payer, TransactionStamp(1234567890))
		assert.NoError(t, err)

		assert.Equal(t, tx1.ixHashFromWire(wire), tx2.ixHashFromWire(wire))
	})
}

func TestTransaction_ixPart(t *testing.T) {
	t.Run("collect single ix hash item", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, nil)
		assert.NoError(t, err)

		ixIndex := uint8(1)

		items, err := tx.ixPart([]uint8{1})
		assert.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, ixIndex, items[0].Index)
		assert.Equal(t, tx.IxHashes()[ixIndex], items[0].Hash)
	})

	t.Run("collect multiple ix hash items", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {10, 11, 12}}, nil)
		assert.NoError(t, err)

		items, err := tx.ixPart([]uint8{0, 2, 3})
		assert.NoError(t, err)
		assert.Len(t, items, 3)

		assert.Equal(t, uint8(0), items[0].Index)
		assert.Equal(t, tx.IxHashes()[0], items[0].Hash)

		assert.Equal(t, uint8(2), items[1].Index)
		assert.Equal(t, tx.IxHashes()[2], items[1].Hash)

		assert.Equal(t, uint8(3), items[2].Index)
		assert.Equal(t, tx.IxHashes()[3], items[2].Hash)
	})

	t.Run("empty indices", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		items, err := tx.ixPart([]uint8{})
		assert.NoError(t, err)
		assert.Equal(t, []IxHashItem{}, items)
	})

	t.Run("index out of range", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		_, err = tx.ixPart([]uint8{5})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("AuthPayerBit should fail", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		_, err = tx.ixPart([]uint8{AuthPayerBit})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be AuthPayerBit")
	})

	t.Run("preserve order", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {10, 11, 12}}, nil)
		assert.NoError(t, err)

		// Collect in specific order
		items, err := tx.ixPart([]uint8{3, 1, 0})
		assert.NoError(t, err)
		assert.Len(t, items, 3)

		// Preserve input order
		assert.Equal(t, uint8(3), items[0].Index)
		assert.Equal(t, uint8(1), items[1].Index)
		assert.Equal(t, uint8(0), items[2].Index)
	})
}

func TestTransaction_ResolvePayer(t *testing.T) {
	t.Run("resolve payer from payer-only signature", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		_, err = tx.ResolvePayer()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "payer signature required")

		// Sign payer
		sig, err := tx.SignPayer(*payer, sk, PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)

		tx.AddSignature(*payer, *sig)

		resolvedPayer, err := tx.ResolvePayer()
		assert.NoError(t, err)
		assert.Equal(t, *payer, resolvedPayer)
	})

	t.Run("resolve payer from ix+payer signature in split-payer mode", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		_, err = tx.ResolvePayer()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "payer signature required")

		// Sign ix and payer
		acSig, err := tx.SignIxAndPayer(*payer, sk, 0, PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		tx.AddSignature(*payer, *acSig)

		resolvedPayer, err := tx.ResolvePayer()
		assert.NoError(t, err)
		assert.Equal(t, *payer, resolvedPayer)
	})
}

func TestTransaction_ValidateWire(t *testing.T) {
	t.Run("empty instructions should fail", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{}, nil)
		assert.NoError(t, err)

		err = tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty instructions")
	})

	t.Run("too many instructions should fail", func(t *testing.T) {
		var instructions []api.PackedInstruction
		for i := 0; i < 64; i++ {
			instructions = append(instructions, []byte{byte(i)})
		}

		tx, err := NewTransactionWithParam(instructions, nil)
		assert.NoError(t, err)

		err = tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too many instructions")
	})

	t.Run("duplicate ix hash should fail", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {1, 2, 3}}, nil) // Duplicate instructions
		assert.NoError(t, err)

		err = tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate ix hash")
	})

	t.Run("valid transaction with payer signature (unified mode)", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		// Unified-payer mode
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, payer)
		assert.NoError(t, err)

		// Payer signs
		payerSig, err := tx.SignPayer(*payer, sk, PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		tx.AddSignature(*payer, *payerSig)

		err = tx.ValidateWire()
		assert.NoError(t, err)
	})

	t.Run("valid transaction with ix+payer signature (split mode)", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		// Split-payer mode
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		// ix + payer signature
		acSig, err := tx.SignIxAndPayer(*owner, sk, 0, PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		tx.AddSignature(*owner, *acSig)

		err = tx.ValidateWire()
		assert.NoError(t, err)
	})

	t.Run("missing payer signature in unified mode should fail", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		// Unified-payer mode
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, payer)
		assert.NoError(t, err)

		// Only sign ix, not payer
		ownerAcSig, err := tx.SignIx(*payer, sk, 0, PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		tx.AddSignature(*payer, *ownerAcSig)

		err = tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "payer signature required")
	})

	t.Run("missing gas signer in split mode should fail", func(t *testing.T) {
		sk1 := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey1 := sk1.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey1)
		assert.NoError(t, err)

		// Split-payer mode
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		// Payer only signs bit63
		payerSig, err := tx.SignPayer(*payer, sk1, PubKeySignatureMode{PublicKey: *pubKey1})
		assert.NoError(t, err)
		tx.AddSignature(*payer, *payerSig)

		err = tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")
	})
}

func TestTransaction_ValidateWireWith(t *testing.T) {
	t.Run("sponsor single ix in split mode", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		user, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		// Split-payer mode, 3 instructions
		tx, err := NewTransactionWithParam(
			[]api.PackedInstruction{
				{1, 2, 3}, // ix=0: sponsored
				{4, 5, 6}, // ix=1: user pays
				{7, 8, 9}, // ix=2: user pays
			},
			nil,
		)
		assert.NoError(t, err)

		// User only signs ix=1 and ix=2
		sig, err := tx.SignIxes(*user, sk, []uint8{1, 2}, true, PubKeySignatureMode{PublicKey: *pubKey})
		tx.AddSignature(*user, *sig)

		// Sponsor no instructions: fails (ix=0 missing signature)
		err = tx.ValidateWireWith([]uint8{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")

		// Sponsor ix=0: passes
		err = tx.ValidateWireWith([]uint8{0})
		assert.NoError(t, err)
	})

	t.Run("sponsor multiple ixes in split mode", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		user, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		// Split-payer mode, 4 instructions
		tx, err := NewTransactionWithParam(
			[]api.PackedInstruction{
				{1, 2, 3},    // ix=0: sponsored
				{4, 5, 6},    // ix=1: user pays
				{7, 8, 9},    // ix=2: sponsored
				{10, 11, 12}, // ix=3: user pays
			},
			nil,
		)
		assert.NoError(t, err)

		// User only signs ix=1 and ix=3
		sig, err := tx.SignIxes(*user, sk, []uint8{1, 3}, true, PubKeySignatureMode{PublicKey: *pubKey})
		tx.AddSignature(*user, *sig)

		// Sponsor no instructions: fails (ix=0 missing signature)
		err = tx.ValidateWireWith([]uint8{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")

		// Sponsor ix=0 and ix=2: passes
		err = tx.ValidateWireWith([]uint8{0, 2})
		assert.NoError(t, err)

		// Only sponsor ix=0: fails (ix=2 missing signature)
		err = tx.ValidateWireWith([]uint8{0})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 2")
	})

	t.Run("sponsor all ixes", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}, nil)
		assert.NoError(t, err)

		// No signatures, but all instructions are sponsored
		err = tx.ValidateWireWith([]uint8{0, 1})
		assert.NoError(t, err)
	})

	t.Run("sponsor in unified mode", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}, payer)
		assert.NoError(t, err)

		// Payer signs
		payerSig, err := tx.SignPayer(*payer, sk, PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		tx.AddSignature(*payer, *payerSig)

		// In unified mode, sponsorIx does not affect validation (payer covers all gas)
		err = tx.ValidateWireWith([]uint8{})
		assert.NoError(t, err)

		err = tx.ValidateWireWith([]uint8{0})
		assert.NoError(t, err)
	})

	t.Run("invalid sponsor in split mode", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		// Sponsor no instructions: fails (ix=0 missing signature)
		err = tx.ValidateWireWith([]uint8{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")

		// Sponsor non-existent ix=5 (current implementation does not validate sponsorIx bounds)
		// ix=0 still needs a signature, so it fails
		err = tx.ValidateWireWith([]uint8{5})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")
	})

	t.Run("sponsor with invalid index is ignored", func(t *testing.T) {
		tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
		assert.NoError(t, err)

		// Sponsor non-existent ix=5 (ignored) + valid ix=0
		err = tx.ValidateWireWith([]uint8{0, 5})
		assert.NoError(t, err)

		// If only sponsoring invalid ix=5, ix=0 still needs a signature
		err = tx.ValidateWireWith([]uint8{5})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")
	})
}

func TestTransaction_ToBytes(t *testing.T) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
	assert.NoError(t, err)

	data, err := tx.ToBytes()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	_, err = NewTransactionFromBytes(data)
	assert.NoError(t, err)
}

func TestTransaction_MarshalPostcard(t *testing.T) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}, nil)
	assert.NoError(t, err)

	data, err := postcard.SerializePostcard(tx)
	assert.NoError(t, err)

	deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (*Transaction, error) {
		var transaction Transaction
		err = transaction.UnmarshalPostcard(d)
		return &transaction, err
	}, false)
	assert.NoError(t, err)

	assert.Len(t, deserialized.Instructions, len(tx.Instructions))
	assert.Len(t, deserialized.TxSigs, len(tx.TxSigs))
	assert.Equal(t, tx.Stamp, deserialized.Stamp)
	assert.Equal(t, tx.Payer, deserialized.Payer)

	for i := range tx.Instructions {
		assert.Equal(t, tx.Instructions[i], deserialized.Instructions[i])
	}
}

// ********************************** Build *********************************//

func TestBuildSignedSingleIxUnified(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	payer, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	stamp := TransactionStamp(1234567890)
	wire := api.PackedInstruction{1, 2, 3, 4}

	// Unified-payer mode: set payer
	tx, err := BuildSingleIxUnifiedPayerSignAll(sk, *payer, PubKeySignatureMode{PublicKey: *pubKey}, wire, stamp)
	assert.NoError(t, err)

	// Verify transaction structure
	assert.Equal(t, stamp, tx.Stamp)
	assert.Equal(t, payer, tx.Payer)
	assert.Len(t, tx.Instructions, 1)
	assert.Equal(t, wire, tx.Instructions[0])

	// Verify signature
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *payer, tx.TxSigs[0].Address)
	assert.Equal(t, uint64(1)<<0|uint64(1)<<AuthPayerBit, tx.TxSigs[0].AccountSignature.AuthBit.Raw())
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())

	// Verify ResolvePayer works
	resolvedPayer, err := tx.ResolvePayer()
	assert.NoError(t, err)
	assert.Equal(t, *payer, resolvedPayer)

	// Verify ValidateWire passes (unified mode)
	err = tx.ValidateWire()
	assert.NoError(t, err)
}

func TestBuildSingleIxUnifiedPayerSignOnlyGas(t *testing.T) {
	payerSk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	payerPubKey := payerSk.Ed25519Public()
	payer, err := crypto.NewAddressFromPublicKey(payerPubKey)
	assert.NoError(t, err)

	wire := api.PackedInstruction{1, 2, 3}
	tx, err := BuildSingleIxUnifiedPayerSignOnlyGas(payerSk, *payer, PubKeySignatureMode{PublicKey: *payerPubKey}, wire)
	assert.NoError(t, err)

	// Verify instruction
	assert.Equal(t, payer, tx.Payer)
	assert.Len(t, tx.Instructions, 1)
	assert.Equal(t, wire, tx.Instructions[0])

	// Verify signature
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *payer, tx.TxSigs[0].Address)
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
	assert.False(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	assert.Equal(t, uint64(1)<<AuthPayerBit, tx.TxSigs[0].AccountSignature.AuthBit.Raw())

	// ResolvePayer should succeed
	resolvedPayer, err := tx.ResolvePayer()
	assert.NoError(t, err)
	assert.Equal(t, *payer, resolvedPayer)

	// ValidateWire should pass (unified mode only checks payer signed bit63)
	err = tx.ValidateWire()
	assert.NoError(t, err)
}

func TestBuildSignedSingleIxSplit(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	stamp := TransactionStamp(1234567890)
	wire := api.PackedInstruction{5, 6, 7, 8}

	// Split-payer mode: no payer set
	tx, err := BuildSingleIxSplitSign(sk, *owner, PubKeySignatureMode{PublicKey: *pubKey}, wire, stamp)
	assert.NoError(t, err)

	// Verify transaction structure
	assert.Equal(t, stamp, tx.Stamp)
	assert.Equal(t, (*crypto.Address)(nil), tx.Payer)
	assert.Equal(t, wire, tx.Instructions[0])

	// Verify signature
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *owner, tx.TxSigs[0].Address)
	assert.Equal(t, uint64(1)<<0|uint64(1)<<AuthPayerBit, tx.TxSigs[0].AccountSignature.AuthBit.Raw())
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())

	// Verify payer can be resolved from signature
	resolvedPayer, err := tx.ResolvePayer()
	assert.NoError(t, err)
	assert.Equal(t, *owner, resolvedPayer)

	// Verify ValidateWire passes (split mode)
	err = tx.ValidateWire()
	assert.NoError(t, err)
}

func TestTransactionUnifiedPaymentMode(t *testing.T) {
	// Payer key
	payerSk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	payerPubKey := payerSk.Ed25519Public()
	payer, err := crypto.NewAddressFromPublicKey(payerPubKey)
	assert.NoError(t, err)

	// Token key
	tokenSk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	tokenPubKey := tokenSk.Ed25519Public()
	token, err := crypto.NewAddressFromPublicKey(tokenPubKey)
	assert.NoError(t, err)

	// Create unified-payer mode transaction
	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, payer)
	assert.NoError(t, err)

	// Payer signs (only bit63, pays gas)
	payerSig, err := tx.SignPayer(*payer, payerSk, PubKeySignatureMode{PublicKey: *payerPubKey})
	assert.NoError(t, err)
	tx.AddSignature(*payer, *payerSig)

	// Token signs (only bit0, executes instruction)
	tokenAcSig, err := tx.SignIx(*token, tokenSk, 0, PubKeySignatureMode{PublicKey: *tokenPubKey})
	assert.NoError(t, err)
	tx.AddSignature(*token, *tokenAcSig)

	// Validate transaction
	err = tx.ValidateWire()
	assert.NoError(t, err)

	// Verify signature properties
	assert.Len(t, tx.TxSigs, 2)

	// Payer signature should be first
	assert.Equal(t, *payer, tx.TxSigs[0].Address)
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
	assert.False(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	assert.Equal(t, uint64(1)<<AuthPayerBit, tx.TxSigs[0].AccountSignature.AuthBit.Raw())

	// Token signature should be second
	assert.Equal(t, *token, tx.TxSigs[1].Address)
	assert.False(t, tx.TxSigs[1].AccountSignature.AuthorizesPayer())
	assert.True(t, tx.TxSigs[1].AccountSignature.AuthorizesIx(0))
	assert.Equal(t, uint64(1)<<0, tx.TxSigs[1].AccountSignature.AuthBit.Raw())
}

func TestTransactionSplitPaymentMode(t *testing.T) {
	// Owner key (pays gas and executes)
	ownerSk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	ownerPubKey := ownerSk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(ownerPubKey)
	assert.NoError(t, err)

	// Create split-payer mode transaction
	tx, err := NewTransactionWithParam([]api.PackedInstruction{{1, 2, 3}}, nil)
	assert.NoError(t, err)

	// Owner signs both ix and payer (bit63 + bit0)
	acSig, err := tx.SignIxAndPayer(*owner, ownerSk, 0, PubKeySignatureMode{PublicKey: *ownerPubKey})
	assert.NoError(t, err)
	tx.AddSignature(*owner, *acSig)

	// Validate transaction
	err = tx.ValidateWire()
	assert.NoError(t, err)

	// Verify signature properties
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *owner, tx.TxSigs[0].Address)
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	assert.Equal(t, uint64(1)<<0|uint64(1)<<AuthPayerBit, tx.TxSigs[0].AccountSignature.AuthBit.Raw())
}

// ********************************** Build *********************************//
