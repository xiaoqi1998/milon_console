package test

import (
	"bytes"
	"github.com/milon-labs/milon-go-sdk/lib"
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/stretchr/testify/assert"
)

// verifySig checks a stored signature against the auth message of the given owner/ix part.
func verifySig(t *testing.T, sig *lib.AccountSignature, owner crypto.Address, txHash api.TxHash, ixPart []lib.IxHashItem, pk *crypto.PublicKey) {
	t.Helper()
	msg, err := sig.AuthMessage(owner, txHash, ixPart)
	assert.NoError(t, err)
	assert.NoError(t, sig.Signatures[0].Verify(msg[:], pk))
}

func TestTransactionBuilder_AddSignature(t *testing.T) {
	sk, pub, owner := newTestKeyPair(t)
	wire := []api.PackedInstruction{{1, 2, 3}}

	// Build a real pre-signed signature against the same tx hash.
	b := lib.NewTransactionBuilder(wire)
	sig, err := lib.NewAccountSignatureBuilder().
		AuthorizeIxAndPayer(0).
		Sign(*owner, sk, b.Tx().TxHash(), []lib.IxHashItem{{Index: 0, Hash: b.Tx().IxHashes()[0]}}, lib.PubKeySignatureMode{PublicKey: *pub}).
		Build()
	assert.NoError(t, err)

	tx, err := b.AddSignature(*owner, *sig).Build()
	assert.NoError(t, err)
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *owner, tx.TxSigs[0].Address)
	verifySig(t, &tx.TxSigs[0].AccountSignature, *owner, tx.TxHash(), []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}, pub)
	assert.NoError(t, tx.ValidateWire())
}

func TestTransactionBuilder_SimulateSlots(t *testing.T) {
	_, payerPub, payer := newTestKeyPair(t)
	_, alicePub, alice := newTestKeyPair(t)
	_, bobPub, bob := newTestKeyPair(t)

	wire := []api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}

	t.Run("multiple slots with different auth shapes", func(t *testing.T) {
		// UnifiedPayerSeparateIx mode: payer signs gas (bit63) only, ix signed by a separate executor account.
		b := lib.NewTransactionBuilder(wire).
			WithPayer(payer).
			ApplySlots([]lib.SigningSlot{
				{Address: *payer, Mode: lib.PubKeySignatureMode{PublicKey: *payerPub}},                                 // payer only
				{Address: *alice, InstructionIndices: []uint8{0}, Mode: lib.PubKeySignatureMode{PublicKey: *alicePub}}, // ix 0 only
				{Address: *bob, InstructionIndices: []uint8{1}, Mode: lib.PubKeySignatureMode{PublicKey: *bobPub}},     // ix 1 only
			}).
			SimulateSlots()
		tx, err := b.Build()
		assert.NoError(t, err)

		assert.Len(t, tx.TxSigs, 3)
		assert.Equal(t, *payer, tx.TxSigs[0].Address)
		assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
		assert.False(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
		assert.True(t, tx.TxSigs[1].AccountSignature.AuthorizesIx(0))
		assert.True(t, tx.TxSigs[2].AccountSignature.AuthorizesIx(1))

		// Simulated signatures are zero-filled placeholders.
		for _, ts := range tx.TxSigs {
			assert.Len(t, ts.AccountSignature.Signatures, 1)
			assert.Equal(t, crypto.SignatureEd25519Size, len(ts.AccountSignature.Signatures[0].Bytes))
			assert.True(t, bytes.Equal(make([]byte, crypto.SignatureEd25519Size), ts.AccountSignature.Signatures[0].Bytes))
		}

		// ValidateWire passes: payer's bit63 covers gas for the whole tx.
		assert.NoError(t, tx.ValidateWire())
	})

	t.Run("single ix with IncludePayer routes to IxAndPayer signature", func(t *testing.T) {
		// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
		singleIxWire := []api.PackedInstruction{{1, 2, 3}}
		b := lib.NewTransactionBuilder(singleIxWire).
			ApplySlots([]lib.SigningSlot{
				{Address: *alice, InstructionIndices: []uint8{0}, IncludePayer: true, Mode: lib.PubKeySignatureMode{PublicKey: *alicePub}},
			}).
			SimulateSlots()
		tx, err := b.Build()
		assert.NoError(t, err)

		assert.Len(t, tx.TxSigs, 1)
		assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
		assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
		assert.NoError(t, tx.ValidateWire())
	})

	t.Run("multi-ix slot routes to ixes signature", func(t *testing.T) {
		// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
		b := lib.NewTransactionBuilder(wire).
			ApplySlots([]lib.SigningSlot{
				{Address: *alice, InstructionIndices: []uint8{0, 1}, IncludePayer: true, Mode: lib.PubKeySignatureMode{PublicKey: *alicePub}},
			}).
			SimulateSlots()
		tx, err := b.Build()
		assert.NoError(t, err)

		assert.Len(t, tx.TxSigs, 1)
		assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
		assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(1))
		assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
		assert.NoError(t, tx.ValidateWire())
	})
}

func TestTransactionBuilder_SimulateThenSign(t *testing.T) {
	payerSk, payerPub, payer := newTestKeyPair(t)
	aliceSk, alicePub, alice := newTestKeyPair(t)

	wire := []api.PackedInstruction{{1, 2, 3}}
	slots := []lib.SigningSlot{
		{Address: *payer, Mode: lib.PubKeySignatureMode{PublicKey: *payerPub}},
		{Address: *alice, InstructionIndices: []uint8{0}, Mode: lib.PubKeySignatureMode{PublicKey: *alicePub}},
	}

	// 1. simulate: zero-filled placeholders
	b := lib.NewTransactionBuilder(wire).
		WithPayer(payer).
		ApplySlots(slots).
		SimulateSlots()
	simTx, err := b.Build()
	assert.NoError(t, err)
	assert.Len(t, simTx.TxSigs, 2)
	assert.True(t, bytes.Equal(make([]byte, crypto.SignatureEd25519Size), simTx.TxSigs[0].AccountSignature.Signatures[0].Bytes))

	// 2. reset placeholders and sign for real
	b.ResetSigs().
		SignWith(
			lib.Signer{SecretKey: payerSk, PublicKey: *payerPub},
			lib.Signer{SecretKey: aliceSk, PublicKey: *alicePub},
		)
	tx, err := b.Build()
	assert.NoError(t, err)
	assert.Len(t, tx.TxSigs, 2)
	// ResetSigs replaces the TxSigs slice in place (simTx and tx share the same
	// transaction), so compare against the known zero placeholder.
	assert.False(t, bytes.Equal(make([]byte, crypto.SignatureEd25519Size), tx.TxSigs[0].AccountSignature.Signatures[0].Bytes))
	verifySig(t, &tx.TxSigs[0].AccountSignature, *payer, tx.TxHash(), nil, payerPub)
	verifySig(t, &tx.TxSigs[1].AccountSignature, *alice, tx.TxHash(), []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}, alicePub)
}

func TestTransactionBuilder_SignWith(t *testing.T) {
	payerSk, payerPub, payer := newTestKeyPair(t)
	aliceSk, alicePub, alice := newTestKeyPair(t)
	bobSk, bobPub, bob := newTestKeyPair(t)
	participantSk, participantPub, wallet := newTestKeyPair(t)

	wire := []api.PackedInstruction{{1, 2, 3}, {4, 5, 6}}

	t.Run("real signatures for all slots", func(t *testing.T) {
		b := lib.NewTransactionBuilder(wire).
			WithPayer(payer).
			ApplySlots([]lib.SigningSlot{
				{Address: *payer, Mode: lib.PubKeySignatureMode{PublicKey: *payerPub}},
				{Address: *alice, InstructionIndices: []uint8{0}, Mode: lib.PubKeySignatureMode{PublicKey: *alicePub}},
				{Address: *bob, InstructionIndices: []uint8{1}, IncludePayer: true, Mode: lib.PubKeySignatureMode{PublicKey: *bobPub}},
			}).
			SignWith(
				lib.Signer{SecretKey: payerSk, PublicKey: *payerPub},
				lib.Signer{SecretKey: aliceSk, PublicKey: *alicePub},
				lib.Signer{SecretKey: bobSk, PublicKey: *bobPub},
			)
		tx, err := b.Build()
		assert.NoError(t, err)

		assert.Len(t, tx.TxSigs, 3)
		assert.Equal(t, *payer, tx.TxSigs[0].Address)
		assert.Equal(t, *alice, tx.TxSigs[1].Address)
		assert.Equal(t, *bob, tx.TxSigs[2].Address)

		// Each signature verifies against the auth message built from the tx.
		hashes := tx.IxHashes()
		verifySig(t, &tx.TxSigs[0].AccountSignature, *payer, tx.TxHash(), nil, payerPub)
		verifySig(t, &tx.TxSigs[1].AccountSignature, *alice, tx.TxHash(), []lib.IxHashItem{{Index: 0, Hash: hashes[0]}}, alicePub)
		verifySig(t, &tx.TxSigs[2].AccountSignature, *bob, tx.TxHash(), []lib.IxHashItem{{Index: 1, Hash: hashes[1]}}, bobPub)

		assert.NoError(t, tx.ValidateWire())
	})

	t.Run("multi-ix slot with IncludePayer signs all", func(t *testing.T) {
		// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
		b := lib.NewTransactionBuilder(wire).
			ApplySlots([]lib.SigningSlot{
				{Address: *alice, InstructionIndices: []uint8{0, 1}, IncludePayer: true, Mode: lib.PubKeySignatureMode{PublicKey: *alicePub}},
			}).
			SignWith(lib.Signer{SecretKey: aliceSk, PublicKey: *alicePub})
		tx, err := b.Build()
		assert.NoError(t, err)

		assert.Len(t, tx.TxSigs, 1)
		hashes := tx.IxHashes()
		verifySig(t, &tx.TxSigs[0].AccountSignature, *alice, tx.TxHash(), []lib.IxHashItem{{Index: 0, Hash: hashes[0]}, {Index: 1, Hash: hashes[1]}}, alicePub)
		assert.NoError(t, tx.ValidateWire())
	})

	t.Run("multisig slot", func(t *testing.T) {
		b := lib.NewTransactionBuilder(wire).
			ApplySlots([]lib.SigningSlot{
				{Address: *wallet, InstructionIndices: []uint8{0}, Mode: lib.MultisigKeySignatureMode{Index: 2, PublicKey: *participantPub}},
			}).
			SignWith(lib.Signer{SecretKey: participantSk, PublicKey: *participantPub})
		tx, err := b.Build()
		assert.NoError(t, err)

		assert.Len(t, tx.TxSigs, 1)
		sig := tx.TxSigs[0].AccountSignature
		assert.Equal(t, uint64(1)<<2, sig.SigBit.Raw())
		assert.Nil(t, sig.PubKey)
		assert.True(t, sig.AuthorizesIx(0))
		verifySig(t, &sig, *wallet, tx.TxHash(), []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}, participantPub)
	})
}

func TestTransactionBuilder_SignWithError(t *testing.T) {
	payerSk, payerPub, payer := newTestKeyPair(t)
	_, alicePub, alice := newTestKeyPair(t)

	b := lib.NewTransactionBuilder([]api.PackedInstruction{{1, 2, 3}}).
		ApplySlots([]lib.SigningSlot{
			{Address: *payer, Mode: lib.PubKeySignatureMode{PublicKey: *payerPub}},
			{Address: *alice, InstructionIndices: []uint8{0}, Mode: lib.PubKeySignatureMode{PublicKey: *alicePub}},
		}).
		SignWith(lib.Signer{SecretKey: payerSk, PublicKey: *payerPub})
	tx, err := b.Build()
	assert.Error(t, err)
	assert.Nil(t, tx)
	assert.Contains(t, err.Error(), "no signer found for address")

	// The first slot was signed before the failure; later calls short-circuit.
	assert.Len(t, b.Tx().TxSigs, 1)
}

func TestTransactionBuilder_ErrorShortCircuit(t *testing.T) {
	t.Run("AuthReservedBit fails build", func(t *testing.T) {
		b := lib.NewTransactionBuilder([]api.PackedInstruction{{1, 2, 3}}).
			AddIxAndPayerSig(crypto.Address{}, crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()), lib.AuthReservedBit, lib.PubKeySignatureMode{})
		tx, err := b.Build()
		assert.Error(t, err)
		assert.Nil(t, tx)
		assert.Contains(t, err.Error(), "cannot be AuthReservedBit")
	})

	t.Run("AuthPayerBit fails build", func(t *testing.T) {
		b := lib.NewTransactionBuilder([]api.PackedInstruction{{1, 2, 3}}).
			AddIxAndPayerSig(crypto.Address{}, crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()), lib.AuthPayerBit, lib.PubKeySignatureMode{})
		tx, err := b.Build()
		assert.Error(t, err)
		assert.Nil(t, tx)
		assert.Contains(t, err.Error(), "cannot be AuthPayerBit")
	})

	t.Run("ix index out of range fails build", func(t *testing.T) {
		b := lib.NewTransactionBuilder([]api.PackedInstruction{{1, 2, 3}}).
			AddIxAndPayerSig(crypto.Address{}, crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()), 5, lib.PubKeySignatureMode{})
		tx, err := b.Build()
		assert.Error(t, err)
		assert.Nil(t, tx)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("subsequent calls are short-circuited after error", func(t *testing.T) {
		_, payerPub, payer := newTestKeyPair(t)
		b := lib.NewTransactionBuilder([]api.PackedInstruction{{1, 2, 3}}).
			AddIxAndPayerSig(*payer, crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey()), lib.AuthReservedBit, lib.PubKeySignatureMode{PublicKey: *payerPub})
		// These must be no-ops once errs is set.
		b.WithPayer(payer).AddSignature(*payer, lib.AccountSignature{}).ResetSigs()
		tx, err := b.Build()
		assert.Error(t, err)
		assert.Nil(t, tx)
		assert.Contains(t, err.Error(), "cannot be AuthReservedBit")
		assert.Nil(t, b.Tx().Payer)
		assert.Empty(t, b.Tx().TxSigs)
	})

	t.Run("ResetSigs is a no-op after error", func(t *testing.T) {
		payerSk, payerPub, payer := newTestKeyPair(t)
		_, alicePub, alice := newTestKeyPair(t)
		b := lib.NewTransactionBuilder([]api.PackedInstruction{{1, 2, 3}}).
			ApplySlots([]lib.SigningSlot{
				{Address: *payer, Mode: lib.PubKeySignatureMode{PublicKey: *payerPub}},
				{Address: *alice, InstructionIndices: []uint8{0}, Mode: lib.PubKeySignatureMode{PublicKey: *alicePub}},
			}).
			SignWith(lib.Signer{SecretKey: payerSk, PublicKey: *payerPub})
		assert.Len(t, b.Tx().TxSigs, 1)
		b.ResetSigs()
		assert.Len(t, b.Tx().TxSigs, 1)
	})

	t.Run("mismatched owner and signer key fails", func(t *testing.T) {
		_, _, owner := newTestKeyPair(t)
		wrongSk, wrongPub, _ := newTestKeyPair(t)
		b := lib.NewTransactionBuilder([]api.PackedInstruction{{1, 2, 3}}).
			AddPayerSig(*owner, wrongSk, lib.PubKeySignatureMode{PublicKey: *wrongPub})
		tx, err := b.Build()
		assert.Error(t, err)
		assert.Nil(t, tx)
		assert.Contains(t, err.Error(), "public key does not match owner address")
	})
}

func TestTransactionBuilder_UnifiedPayerGasOnly(t *testing.T) {
	payerSk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	payerPubKey := payerSk.Ed25519Public()
	payer, err := crypto.NewAddressFromPublicKey(payerPubKey)
	assert.NoError(t, err)

	wire := api.PackedInstruction{1, 2, 3}

	// UnifiedPayerGasOnly mode: payer signs gas (bit63) only, ix needs no signature (pure sponsorship).
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(payer).
		AddPayerSig(*payer, payerSk, lib.PubKeySignatureMode{PublicKey: *payerPubKey}).
		Build()
	assert.NoError(t, err)

	assert.Equal(t, payer, tx.Payer)
	assert.Len(t, tx.Instructions, 1)
	assert.Equal(t, wire, tx.Instructions[0])
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *payer, tx.TxSigs[0].Address)
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
	assert.False(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))

	// ValidateWire passes: unified mode only requires the payer's gas signature.
	assert.NoError(t, tx.ValidateWire())
}

func TestTransactionBuilder_UnifiedPayerSignAll(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	payer, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	wire := api.PackedInstruction{1, 2, 3, 4}

	// UnifiedPayerSignAll mode: payer signs the ix bit(s) and gas bit (bit63) in a single signature.
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(payer).
		AddIxAndPayerSig(*payer, sk, 0, lib.PubKeySignatureMode{PublicKey: *pubKey}).
		Build()
	assert.NoError(t, err)

	assert.Equal(t, payer, tx.Payer)
	assert.Len(t, tx.Instructions, 1)
	assert.Equal(t, wire, tx.Instructions[0])
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *payer, tx.TxSigs[0].Address)
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
	verifySig(t, &tx.TxSigs[0].AccountSignature, *payer, tx.TxHash(), []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}, pubKey)

	// ValidateWire passes in unified mode.
	assert.NoError(t, tx.ValidateWire())
}

func TestTransactionBuilder_UnifiedPayerSeparateIx(t *testing.T) {
	payerSk, payerPub, payer := newTestKeyPair(t)
	tokenSk, tokenPub, token := newTestKeyPair(t)

	wire := []api.PackedInstruction{{1, 2, 3}}

	// UnifiedPayerSeparateIx mode: payer signs gas (bit63) only, ix signed by a separate executor account.
	b := lib.NewTransactionBuilder(wire).
		WithPayer(payer).
		ApplySlots([]lib.SigningSlot{
			{Address: *payer, Mode: lib.PubKeySignatureMode{PublicKey: *payerPub}},
			{Address: *token, InstructionIndices: []uint8{0}, Mode: lib.PubKeySignatureMode{PublicKey: *tokenPub}},
		}).
		SignWith(
			lib.Signer{SecretKey: payerSk, PublicKey: *payerPub},
			lib.Signer{SecretKey: tokenSk, PublicKey: *tokenPub},
		)
	tx, err := b.Build()
	assert.NoError(t, err)

	assert.Len(t, tx.TxSigs, 2)

	// Payer signature: gas only (bit63).
	assert.Equal(t, *payer, tx.TxSigs[0].Address)
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
	assert.False(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	verifySig(t, &tx.TxSigs[0].AccountSignature, *payer, tx.TxHash(), nil, payerPub)

	// Token signature: ix 0 only.
	assert.Equal(t, *token, tx.TxSigs[1].Address)
	assert.False(t, tx.TxSigs[1].AccountSignature.AuthorizesPayer())
	assert.True(t, tx.TxSigs[1].AccountSignature.AuthorizesIx(0))
	verifySig(t, &tx.TxSigs[1].AccountSignature, *token, tx.TxHash(), []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}, tokenPub)

	assert.NoError(t, tx.ValidateWire())
}

func TestTransactionBuilder_SplitPayerSelfPay(t *testing.T) {
	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pubKey := sk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(pubKey)
	assert.NoError(t, err)

	wire := api.PackedInstruction{5, 6, 7, 8}

	// SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		AddIxAndPayerSig(*owner, sk, 0, lib.PubKeySignatureMode{PublicKey: *pubKey}).
		Build()
	assert.NoError(t, err)

	assert.Equal(t, (*crypto.Address)(nil), tx.Payer)
	assert.Len(t, tx.Instructions, 1)
	assert.Equal(t, wire, tx.Instructions[0])
	assert.Len(t, tx.TxSigs, 1)
	assert.Equal(t, *owner, tx.TxSigs[0].Address)
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesIx(0))
	assert.True(t, tx.TxSigs[0].AccountSignature.AuthorizesPayer())
	verifySig(t, &tx.TxSigs[0].AccountSignature, *owner, tx.TxHash(), []lib.IxHashItem{{Index: 0, Hash: tx.IxHashes()[0]}}, pubKey)

	// ValidateWire passes in split mode.
	assert.NoError(t, tx.ValidateWire())
}
