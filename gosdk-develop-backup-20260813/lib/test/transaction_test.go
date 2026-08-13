package test

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/lib"
	"testing"
	"time"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

// signIx builds an ix-only signature via the low-level Sign.
func signIx(tx *lib.Transaction, account crypto.Address, sk crypto.SecretKeyer, ixIndex uint8, mode lib.AccountSignatureMode) (*lib.AccountSignature, error) {
	if int(ixIndex) >= len(tx.Instructions) {
		return nil, fmt.Errorf("ix index %d out of range", ixIndex)
	}

	authBit, err := lib.AuthIx(ixIndex)
	if err != nil {
		return nil, err
	}

	return lib.Sign(account, sk, authBit, tx.TxHash(), []lib.IxHashItem{{Index: ixIndex, Hash: tx.IxHashes()[ixIndex]}}, mode)
}

// signPayer builds a payer-only (bit63) signature via the low-level Sign.
func signPayer(tx *lib.Transaction, account crypto.Address, sk crypto.SecretKeyer, mode lib.AccountSignatureMode) (*lib.AccountSignature, error) {
	return lib.Sign(account, sk, lib.AuthPayer(), tx.TxHash(), []lib.IxHashItem{}, mode)
}

// signIxAndPayer builds an ix + bit63 signature via the low-level Sign.
func signIxAndPayer(tx *lib.Transaction, account crypto.Address, sk crypto.SecretKeyer, ixIndex uint8, mode lib.AccountSignatureMode) (*lib.AccountSignature, error) {
	if int(ixIndex) >= len(tx.Instructions) {
		return nil, fmt.Errorf("ix index %d out of range", ixIndex)
	}

	authBit, err := lib.AuthIxAndPayer(ixIndex)
	if err != nil {
		return nil, err
	}

	return lib.Sign(account, sk, authBit, tx.TxHash(), []lib.IxHashItem{{Index: ixIndex, Hash: tx.IxHashes()[ixIndex]}}, mode)
}

// signIxes builds a multi-ix signature (optionally with bit63) via the low-level Sign.
func signIxes(tx *lib.Transaction, account crypto.Address, sk crypto.SecretKeyer, ixIndices []uint8, includePayer bool, mode lib.AccountSignatureMode) (*lib.AccountSignature, error) {
	authBit, err := lib.AuthIxes(ixIndices)
	if err != nil {
		return nil, err
	}

	if includePayer {
		authBit = authBit.Set(lib.AuthPayerBit)
	}

	ixPart := make([]lib.IxHashItem, 0, len(ixIndices))
	for _, i := range ixIndices {
		ixPart = append(ixPart, lib.IxHashItem{Index: i, Hash: tx.IxHashes()[i]})
	}

	return lib.Sign(account, sk, authBit, tx.TxHash(), ixPart, mode)
}

// newTestTx builds a Transaction with an empty signature list, An optional stamp overrides the default.
func newTestTx(instructions []api.PackedInstruction, payer *crypto.Address, stamps ...lib.TransactionStamp) *lib.Transaction {
	stamp := lib.TransactionStamp(time.Now().UnixMilli())
	if len(stamps) > 0 {
		stamp = stamps[0]
	}
	return &lib.Transaction{
		Stamp:        stamp,
		Payer:        payer,
		Instructions: instructions,
		TxSigs:       make([]lib.TransactionSignatures, 0),
	}
}

func TestTransaction_TxHash(t *testing.T) {
	t.Run("different hash with different stamp", func(t *testing.T) {
		tx1 := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil, lib.TransactionStamp(1234567890))
		tx2 := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil, lib.TransactionStamp(9876543210))

		assert.NotEqual(t, tx1.TxHash(), tx2.TxHash())
	})

	t.Run("different hash with different instructions", func(t *testing.T) {
		tx1 := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil, lib.TransactionStamp(1234567890))
		tx2 := newTestTx([]api.PackedInstruction{{4, 5, 6}}, nil, lib.TransactionStamp(1234567890))

		assert.NotEqual(t, tx1.TxHash(), tx2.TxHash())
	})

	t.Run("different hash with different payer", func(t *testing.T) {
		tx1 := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil, lib.TransactionStamp(1234567890))

		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		payer, err := crypto.NewAddressFromPublicKey(sk.Ed25519Public())
		assert.NoError(t, err)

		tx2 := newTestTx([]api.PackedInstruction{{1, 2, 3}}, payer, lib.TransactionStamp(1234567890))

		assert.NotEqual(t, tx1.TxHash(), tx2.TxHash())
	})
}

func TestTransaction_AddSignature(t *testing.T) {
	tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil)

	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	acSig, err := signIx(tx, *owner, sk, 0, lib.PubKeySignatureMode{PublicKey: *pubKey})
	assert.NoError(t, err)
	assert.Equal(t, uint64(1)<<0, acSig.AuthBit.Raw())
	assert.Equal(t, uint64(0), acSig.SigBit.Raw())
	assert.Equal(t, 1, len(acSig.Signatures))
	assert.Equal(t, pubKey, acSig.PubKey)

	tx.AddSignature(*owner, *acSig)

	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *owner, tx.TxSigs[0].Address)
	assert.Equal(t, *acSig, tx.TxSigs[0].AccountSignature)
}

func TestTransaction_IxHashes(t *testing.T) {
	tx := newTestTx([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, nil)

	hashes := tx.IxHashes()
	assert.Len(t, hashes, 3)

	for i, ix := range tx.Instructions {
		assert.Equal(t, tx.IxHashFromWire(ix), hashes[i])
	}
}

func TestTransaction_IxHashFromWire(t *testing.T) {
	t.Run("different ix_hash for different wire content", func(t *testing.T) {
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil)

		wire1 := api.PackedInstruction{1, 2, 3}
		wire2 := api.PackedInstruction{4, 5, 6}

		assert.NotEqual(t, tx.IxHashFromWire(wire1), tx.IxHashFromWire(wire2))
	})

	t.Run("ix_hash does not include payer", func(t *testing.T) {
		wire := api.PackedInstruction{1, 2, 3}

		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		tx1 := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil, lib.TransactionStamp(1234567890))
		tx2 := newTestTx([]api.PackedInstruction{{1, 2, 3}}, payer, lib.TransactionStamp(1234567890))

		assert.Equal(t, tx1.IxHashFromWire(wire), tx2.IxHashFromWire(wire))
	})
}

func TestTransaction_ValidateWire(t *testing.T) {
	t.Run("empty instructions should fail", func(t *testing.T) {
		tx := newTestTx([]api.PackedInstruction{}, nil)

		err := tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty instructions")
	})

	t.Run("too many instructions should fail", func(t *testing.T) {
		var instructions []api.PackedInstruction
		for i := 0; i < 64; i++ {
			instructions = append(instructions, []byte{byte(i)})
		}

		tx := newTestTx(instructions, nil)

		err := tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too many instructions")
	})

	t.Run("duplicate ix hash should fail", func(t *testing.T) {
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}, {1, 2, 3}}, nil) // Duplicate instructions

		err := tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate ix hash")
	})

	t.Run("valid transaction with payer signature (unified mode)", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		// UnifiedPayer mode
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, payer)

		// Payer signs
		payerSig, err := signPayer(tx, *payer, sk, lib.PubKeySignatureMode{PublicKey: *pubKey})
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

		// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil)

		// ix + payer signature
		acSig, err := signIxAndPayer(tx, *owner, sk, 0, lib.PubKeySignatureMode{PublicKey: *pubKey})
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

		// UnifiedPayer mode
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, payer)

		// Only sign ix, not payer
		ownerAcSig, err := signIx(tx, *payer, sk, 0, lib.PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		tx.AddSignature(*payer, *ownerAcSig)

		err = tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "payer signature required")
	})

	t.Run("split mode payer only bit63 fails", func(t *testing.T) {
		sk1 := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey1 := sk1.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey1)
		assert.NoError(t, err)

		// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil)

		// Payer only signs bit63
		payerSig, err := signPayer(tx, *payer, sk1, lib.PubKeySignatureMode{PublicKey: *pubKey1})
		assert.NoError(t, err)
		tx.AddSignature(*payer, *payerSig)

		err = tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")
	})

	t.Run("empty auth bit should fail", func(t *testing.T) {
		_, _, owner := newTestKeyPair(t)
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil)

		// AccountSignature with zero AuthBit
		tx.AddSignature(*owner, lib.AccountSignature{})

		err := tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty auth bit")
	})

	t.Run("duplicate signature owner should fail", func(t *testing.T) {
		sk, pubKey, owner := newTestKeyPair(t)
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil)

		acSig, err := signIx(tx, *owner, sk, 0, lib.PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		tx.AddSignature(*owner, *acSig)
		tx.AddSignature(*owner, *acSig) // same owner twice

		err = tx.ValidateWire()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate signature owner")
	})
}

func TestTransaction_ValidateWireWith(t *testing.T) {
	t.Run("sponsor single ix in split mode", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		user, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
		tx := newTestTx(
			[]api.PackedInstruction{
				{1, 2, 3}, // ix=0: sponsored
				{4, 5, 6}, // ix=1: user pays
				{7, 8, 9}, // ix=2: user pays
			},
			nil,
		)

		// User only signs ix=1 and ix=2
		sig, err := signIxes(tx, *user, sk, []uint8{1, 2}, true, lib.PubKeySignatureMode{PublicKey: *pubKey})
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

		// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
		tx := newTestTx(
			[]api.PackedInstruction{
				{1, 2, 3},    // ix=0: sponsored
				{4, 5, 6},    // ix=1: user pays
				{7, 8, 9},    // ix=2: sponsored
				{10, 11, 12}, // ix=3: user pays
			},
			nil,
		)

		// User only signs ix=1 and ix=3
		sig, err := signIxes(tx, *user, sk, []uint8{1, 3}, true, lib.PubKeySignatureMode{PublicKey: *pubKey})
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
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}, nil)

		// No signatures, but all instructions are sponsored
		err := tx.ValidateWireWith([]uint8{0, 1})
		assert.NoError(t, err)
	})

	t.Run("sponsor in unified mode", func(t *testing.T) {
		sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		pubKey := sk.Ed25519Public()
		payer, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}, payer)

		// Payer signs
		payerSig, err := signPayer(tx, *payer, sk, lib.PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		tx.AddSignature(*payer, *payerSig)

		// In unified mode, sponsorIx does not affect validation (payer covers all gas)
		err = tx.ValidateWireWith([]uint8{})
		assert.NoError(t, err)

		err = tx.ValidateWireWith([]uint8{0})
		assert.NoError(t, err)
	})

	t.Run("invalid sponsor in split mode", func(t *testing.T) {
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil)

		// Sponsor no instructions: fails (ix=0 missing signature)
		err := tx.ValidateWireWith([]uint8{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")

		// Sponsor non-existent ix=5 (current implementation does not validate sponsorIx bounds)
		// ix=0 still needs a signature, so it fails
		err = tx.ValidateWireWith([]uint8{5})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")
	})

	t.Run("sponsor with invalid index is ignored", func(t *testing.T) {
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil)

		// Sponsor non-existent ix=5 (ignored) + valid ix=0
		err := tx.ValidateWireWith([]uint8{0, 5})
		assert.NoError(t, err)

		// If only sponsoring invalid ix=5, ix=0 still needs a signature
		err = tx.ValidateWireWith([]uint8{5})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas signer required for ix 0")
	})

	t.Run("gas payment mode conflict should fail", func(t *testing.T) {
		skA, pubKeyA, ownerA := newTestKeyPair(t)
		skB, pubKeyB, ownerB := newTestKeyPair(t)
		tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil) // split mode

		// A pays gas for ix=0 (authorizes bit63 + bit0)
		acSigA, err := signIxAndPayer(tx, *ownerA, skA, 0, lib.PubKeySignatureMode{PublicKey: *pubKeyA})
		assert.NoError(t, err)
		tx.AddSignature(*ownerA, *acSigA)

		// B only authorizes bit63 (no ix): gas payment mode conflict in split mode
		acSigB, err := signPayer(tx, *ownerB, skB, lib.PubKeySignatureMode{PublicKey: *pubKeyB})
		assert.NoError(t, err)
		tx.AddSignature(*ownerB, *acSigB)

		err = tx.ValidateWireWith([]uint8{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gas payment mode conflict")
	})
}

func TestTransaction_SerializeRoundTrip(t *testing.T) {
	tx := newTestTx([]api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}, nil)

	data, err := tx.ToBytes()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (*lib.Transaction, error) {
		var transaction lib.Transaction
		err = transaction.UnmarshalPostcard(d)
		return &transaction, err
	}, false)
	assert.NoError(t, err)

	assert.Equal(t, tx.Stamp, deserialized.Stamp)
	assert.Equal(t, tx.Payer, deserialized.Payer)
	assert.Equal(t, tx.Instructions, deserialized.Instructions)
	assert.Equal(t, tx.TxSigs, deserialized.TxSigs)
}

func TestTransaction_UnifiedPayerGasOnly(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	payer, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	// UnifiedPayerGasOnly mode: payer signs gas (bit63) only, ix needs no signature (pure sponsorship).
	tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, payer)

	payerSig, err := signPayer(tx, *payer, sk, lib.PubKeySignatureMode{PublicKey: *pubKey})
	assert.NoError(t, err)
	tx.AddSignature(*payer, *payerSig)

	// Validate transaction
	err = tx.ValidateWire()
	assert.NoError(t, err)

	// Verify signature properties
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *payer, tx.TxSigs[0].Address)
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
	assert.False(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	assert.Equal(t, uint64(1)<<lib.AuthPayerBit, tx.TxSigs[0].AccountSignature.AuthBit.Raw())
}

func TestTransaction_UnifiedPayerSignAll(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	payer, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	// UnifiedPayerSignAll mode: payer signs the ix bit(s) and gas bit (bit63) in a single signature.
	tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, payer)

	acSig, err := signIxAndPayer(tx, *payer, sk, 0, lib.PubKeySignatureMode{PublicKey: *pubKey})
	assert.NoError(t, err)
	tx.AddSignature(*payer, *acSig)

	// Validate transaction
	err = tx.ValidateWire()
	assert.NoError(t, err)

	// Verify signature properties
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *payer, tx.TxSigs[0].Address)
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	assert.Equal(t, (uint64(1)<<0)|(uint64(1)<<lib.AuthPayerBit), tx.TxSigs[0].AccountSignature.AuthBit.Raw())
}

func TestTransaction_UnifiedPayerSeparateIx(t *testing.T) {
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

	// Create UnifiedPayerSeparateIx mode: payer signs gas (bit63) only, ix signed by a separate executor account.
	tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, payer)

	// Payer signs (only bit63, pays gas)
	payerSig, err := signPayer(tx, *payer, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPubKey})
	assert.NoError(t, err)
	tx.AddSignature(*payer, *payerSig)

	// Token signs (only bit0, executes instruction)
	tokenAcSig, err := signIx(tx, *token, tokenSk, 0, lib.PubKeySignatureMode{PublicKey: *tokenPubKey})
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
	assert.Equal(t, uint64(1)<<lib.AuthPayerBit, tx.TxSigs[0].AccountSignature.AuthBit.Raw())

	// Token signature should be second
	assert.Equal(t, *token, tx.TxSigs[1].Address)
	assert.False(t, tx.TxSigs[1].AccountSignature.AuthorizesPayer())
	assert.True(t, tx.TxSigs[1].AccountSignature.AuthorizesIx(0))
	assert.Equal(t, uint64(1)<<0, tx.TxSigs[1].AccountSignature.AuthBit.Raw())
}

func TestTransaction_SplitPayerSelfPay(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
	tx := newTestTx([]api.PackedInstruction{{1, 2, 3}}, nil)

	acSig, err := signIxAndPayer(tx, *owner, sk, 0, lib.PubKeySignatureMode{PublicKey: *pubKey})
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
	assert.Equal(t, (uint64(1)<<0)|(uint64(1)<<lib.AuthPayerBit), tx.TxSigs[0].AccountSignature.AuthBit.Raw())
}
