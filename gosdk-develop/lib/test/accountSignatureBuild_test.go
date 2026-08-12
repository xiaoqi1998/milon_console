package test

import (
	"bytes"
	"github.com/milon-labs/milon-go-sdk/lib"
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestKeyPair creates a classical key pair and derives its owner address.
func newTestKeyPair(t *testing.T) (crypto.SecretKeyer, *crypto.PublicKey, *crypto.Address) {
	t.Helper()
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	require.NoError(t, err)
	return sk, pubKey, owner
}

// testTxHash builds a deterministic tx hash for signing tests.
func testTxHash() api.TxHash {
	var h api.TxHash
	copy(h[:], "test_tx_hash_123456789012345678901234567")
	return h
}

// testIxHashItem builds a deterministic ix hash item for the given index.
func testIxHashItem(index uint8) lib.IxHashItem {
	var h api.TxHash
	copy(h[:], "test_ix_hash_1234567890123456789012345678")
	return lib.IxHashItem{Index: index, Hash: h}
}

func TestAccountSignatureBuilder_Authorize(t *testing.T) {
	t.Run("accumulate multiple ix authorizations", func(t *testing.T) {
		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxes([]uint8{0, 2}).
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0|uint64(1)<<2, sig.AuthBit.Raw())
		assert.Equal(t, uint64(0), sig.SigBit.Raw())
		assert.Empty(t, sig.Signatures)
		assert.Nil(t, sig.PubKey)
	})

	t.Run("authorize ix and payer", func(t *testing.T) {
		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxAndPayer(1).
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<1|uint64(1)<<lib.AuthPayerBit, sig.AuthBit.Raw())
		assert.True(t, sig.AuthorizesIx(1))
		assert.True(t, sig.AuthorizesPayer())
	})

	t.Run("authorize payer only", func(t *testing.T) {
		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizePayer().
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<lib.AuthPayerBit, sig.AuthBit.Raw())
		assert.False(t, sig.AuthorizesIx(0))
	})

	t.Run("invalid ix index records error", func(t *testing.T) {
		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxAndPayer(lib.AuthReservedBit).
			Build()
		assert.Error(t, err)
		assert.Nil(t, sig)
	})

	t.Run("chained calls stop after error", func(t *testing.T) {
		builder := lib.NewAccountSignatureBuilder().
			AuthorizeIxAndPayer(lib.AuthReservedBit). // invalid ix, records error
			AuthorizePayer()                          // short-circuited, must not take effect
		sig, err := builder.Build()
		assert.Error(t, err)
		assert.Nil(t, sig)
	})
}

func TestAccountSignatureBuilder_Sign(t *testing.T) {
	t.Run("sign ix with pubkey mode", func(t *testing.T) {
		sk, pubKey, owner := newTestKeyPair(t)
		txHash := testTxHash()
		ixPart := []lib.IxHashItem{testIxHashItem(0)}

		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxes([]uint8{0}).
			Sign(*owner, sk, txHash, ixPart, lib.PubKeySignatureMode{PublicKey: *pubKey}).
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0, sig.AuthBit.Raw())
		assert.Equal(t, uint64(0), sig.SigBit.Raw())
		assert.Len(t, sig.Signatures, 1)
		assert.Equal(t, pubKey, sig.PubKey)

		// Verify the signature against the auth message.
		msg, err := sig.AuthMessage(*owner, txHash, ixPart)
		assert.NoError(t, err)
		assert.NoError(t, sig.Signatures[0].Verify(msg[:], pubKey))
	})

	t.Run("sign ix and payer with pubkey mode", func(t *testing.T) {
		sk, pubKey, owner := newTestKeyPair(t)
		txHash := testTxHash()
		ixPart := []lib.IxHashItem{testIxHashItem(1)}

		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxAndPayer(1).
			Sign(*owner, sk, txHash, ixPart, lib.PubKeySignatureMode{PublicKey: *pubKey}).
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<1|uint64(1)<<lib.AuthPayerBit, sig.AuthBit.Raw())
		assert.True(t, sig.AuthorizesIx(1))
		assert.True(t, sig.AuthorizesPayer())
	})

	t.Run("sign with multisig mode", func(t *testing.T) {
		sk, pubKey, _ := newTestKeyPair(t)
		_, _, owner := newTestKeyPair(t) // multisig wallet address, different from the signer key
		txHash := testTxHash()
		ixPart := []lib.IxHashItem{testIxHashItem(0)}

		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxes([]uint8{0}).
			Sign(*owner, sk, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pubKey}).
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0, sig.AuthBit.Raw())
		assert.Equal(t, uint64(1)<<2, sig.SigBit.Raw())
		assert.Len(t, sig.Signatures, 1)
		assert.Nil(t, sig.PubKey)

		msg, err := sig.AuthMessage(*owner, txHash, ixPart)
		assert.NoError(t, err)
		assert.NoError(t, sig.Signatures[0].Verify(msg[:], pubKey))
	})

	t.Run("append multisig key via SignMultisigKey", func(t *testing.T) {
		sk0, pubKey0, _ := newTestKeyPair(t)
		sk1, pubKey1, _ := newTestKeyPair(t)
		_, _, owner := newTestKeyPair(t)
		txHash := testTxHash()
		ixPart := []lib.IxHashItem{testIxHashItem(0)}

		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxes([]uint8{0}).
			SignMultisigKey(*owner, sk0, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 0, PublicKey: *pubKey0}).
			SignMultisigKey(*owner, sk1, txHash, ixPart, lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pubKey1}).
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0|uint64(1)<<1, sig.SigBit.Raw())
		assert.Len(t, sig.Signatures, 2)

		// Both signatures verify against the same auth message.
		msg, err := sig.AuthMessage(*owner, txHash, ixPart)
		assert.NoError(t, err)
		assert.NoError(t, sig.Signatures[0].Verify(msg[:], pubKey0))
		assert.NoError(t, sig.Signatures[1].Verify(msg[:], pubKey1))
	})

	t.Run("mismatched owner fails", func(t *testing.T) {
		sk, pubKey, _ := newTestKeyPair(t)
		_, _, wrongOwner := newTestKeyPair(t)

		_, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxes([]uint8{0}).
			Sign(*wrongOwner, sk, testTxHash(), []lib.IxHashItem{testIxHashItem(0)}, lib.PubKeySignatureMode{PublicKey: *pubKey}).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "public key does not match owner address")
	})
}

func TestAccountSignatureBuilder_SimulateSign(t *testing.T) {
	t.Run("simulate ix and payer with pubkey mode", func(t *testing.T) {
		_, pubKey, owner := newTestKeyPair(t)

		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxAndPayer(0).
			SimulateSign(*owner, lib.PubKeySignatureMode{PublicKey: *pubKey}).
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0|uint64(1)<<lib.AuthPayerBit, sig.AuthBit.Raw())
		assert.Len(t, sig.Signatures, 1)
		assert.Equal(t, crypto.SignatureEd25519Size, len(sig.Signatures[0].Bytes))
		assert.True(t, bytes.Equal(make([]byte, crypto.SignatureEd25519Size), sig.Signatures[0].Bytes))
		assert.Equal(t, pubKey, sig.PubKey)
	})

	t.Run("simulate with multisig mode", func(t *testing.T) {
		_, pubKey, _ := newTestKeyPair(t)
		_, _, owner := newTestKeyPair(t)

		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxes([]uint8{0}).
			SimulateSign(*owner, lib.MultisigKeySignatureMode{Index: 3, PublicKey: *pubKey}).
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0, sig.AuthBit.Raw())
		assert.Equal(t, uint64(1)<<3, sig.SigBit.Raw())
		assert.Len(t, sig.Signatures, 1)
		assert.Nil(t, sig.PubKey)
	})

	t.Run("simulate multisig keys appends zero-filled placeholders", func(t *testing.T) {
		_, pk1, _ := newTestKeyPair(t)
		_, pk2, _ := newTestKeyPair(t)

		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxAndPayer(0).
			SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pk1}).
			SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pk2}).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, sig)

		// AuthBit: ix0 + payer (bit63); SigBit: exactly bit1 + bit2.
		assert.Equal(t, uint64(1)<<0|uint64(1)<<lib.AuthPayerBit, sig.AuthBit.Raw())
		assert.Equal(t, uint64(1)<<1|uint64(1)<<2, sig.SigBit.Raw())
		// Multisig: PubKey stays nil.
		assert.Nil(t, sig.PubKey)
		// Two zero-filled placeholders, lengths match the public key type.
		assert.Len(t, sig.Signatures, 2)
		for _, s := range sig.Signatures {
			assert.Equal(t, crypto.SignatureTypeEd25519, s.Variant)
			assert.Len(t, s.Bytes, crypto.SignatureEd25519Size)
			assert.True(t, bytes.Equal(s.Bytes, make([]byte, crypto.SignatureEd25519Size)))
		}
	})

	t.Run("multisig placeholder length follows public key type", func(t *testing.T) {
		_, pk, _ := newTestKeyPair(t)
		secpPk := crypto.PublicKey{Variant: crypto.PublicKeyTypeSecp256k1, Bytes: make([]byte, 33)}
		blsPk := crypto.PublicKey{Variant: crypto.PublicKeyTypeBLS12381, Bytes: make([]byte, 48)}

		sig, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxAndPayer(0).
			SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 0, PublicKey: *pk}).
			SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 1, PublicKey: secpPk}).
			SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 2, PublicKey: blsPk}).
			Build()
		assert.NoError(t, err)
		assert.Len(t, sig.Signatures, 3)
		assert.Equal(t, uint64(1)<<0|uint64(1)<<1|uint64(1)<<2, sig.SigBit.Raw())
		assert.Len(t, sig.Signatures[0].Bytes, crypto.SignatureEd25519Size)
		assert.Len(t, sig.Signatures[1].Bytes, crypto.SignatureSecp256k1Size)
		assert.Len(t, sig.Signatures[2].Bytes, crypto.SignatureBLS12381Size)
	})

	t.Run("multisig simulate error on wrong mode", func(t *testing.T) {
		_, pk, _ := newTestKeyPair(t)

		_, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxAndPayer(0).
			SimulateSignMultisigKey(lib.PubKeySignatureMode{PublicKey: *pk}).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires MultisigKeySignatureMode")
	})

	t.Run("multisig simulate error on index out of range", func(t *testing.T) {
		_, pk, _ := newTestKeyPair(t)

		_, err := lib.NewAccountSignatureBuilder().
			AuthorizeIxAndPayer(0).
			SimulateSignMultisigKey(lib.MultisigKeySignatureMode{Index: 64, PublicKey: *pk}).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})
}
