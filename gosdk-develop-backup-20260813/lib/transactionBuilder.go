package lib

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"time"
)

// SigningSlot declares a signature authorization for one account (without secret keys).
type SigningSlot struct {
	Address            crypto.Address
	InstructionIndices []uint8
	IncludePayer       bool
	Mode               AccountSignatureMode
}

// Signer is a signer entry carrying a secret key and its matching public key.
type Signer struct {
	SecretKey crypto.SecretKeyer
	PublicKey crypto.PublicKey
}

func NewTransactionBuilder(instructions []api.PackedInstruction) *TransactionBuilder {
	return &TransactionBuilder{
		tx: &Transaction{
			Stamp:        TransactionStamp(time.Now().UnixMilli()),
			Instructions: instructions,
			TxSigs:       make([]TransactionSignatures, 0),
		},
	}
}

type TransactionBuilder struct {
	tx    *Transaction
	slots []SigningSlot
	errs  []error
}

func (b *TransactionBuilder) Tx() *Transaction {
	return b.tx
}

// WithPayer sets the payer address
func (b *TransactionBuilder) WithPayer(account *crypto.Address) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.tx.Payer = account
	return b
}

// WithStamp sets the transaction timestamp.
func (b *TransactionBuilder) WithStamp(stamp TransactionStamp) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.tx.Stamp = stamp
	return b
}

// AddSignature manually adds a pre-built signature to the transaction.
func (b *TransactionBuilder) AddSignature(account crypto.Address, accountSig AccountSignature) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.tx.AddSignature(account, accountSig)
	return b
}

// ApplySlots registers signing slots, declaring each address's authorization scope (without secret keys).
func (b *TransactionBuilder) ApplySlots(slots []SigningSlot) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.slots = append(b.slots, slots...)
	return b
}

// SimulateSlots simulates signatures for all registered slots (no secret keys needed).
func (b *TransactionBuilder) SimulateSlots() *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	for _, slot := range b.slots {
		switch {
		case len(slot.InstructionIndices) == 0:
			b.AddSimulatePayerSig(slot.Address, slot.Mode)
		case len(slot.InstructionIndices) == 1 && slot.IncludePayer:
			b.AddSimulateIxAndPayerSig(slot.Address, slot.InstructionIndices[0], slot.Mode)
		default:
			b.AddSimulateIxesSig(slot.Address, slot.InstructionIndices, slot.IncludePayer, slot.Mode)
		}
		if len(b.errs) > 0 {
			return b
		}
	}
	return b
}

// AddSimulatePayerSig signs acSig payer (bit63) with simulate signing and adds the signature.
func (b *TransactionBuilder) AddSimulatePayerSig(account crypto.Address, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := NewAccountSignatureBuilder().
		AuthorizePayer().
		SimulateSign(account, mode).
		Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(account, *sig)
	return b
}

// AddSimulateIxAndPayerSig signs both instruction execution and gas with simulate signing.
func (b *TransactionBuilder) AddSimulateIxAndPayerSig(account crypto.Address, ixIndex uint8, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := NewAccountSignatureBuilder().
		AuthorizeIxAndPayer(ixIndex).
		SimulateSign(account, mode).
		Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(account, *sig)
	return b
}

// AddSimulateIxesSig signs multiple instructions (optionally including payer bit63) with simulate signing.
func (b *TransactionBuilder) AddSimulateIxesSig(account crypto.Address, ixIndices []uint8, includePayer bool, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sigBuilder := NewAccountSignatureBuilder().AuthorizeIxes(ixIndices)
	if includePayer {
		sigBuilder.AuthorizePayer()
	}
	sig, err := sigBuilder.SimulateSign(account, mode).Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(account, *sig)
	return b
}

// SignWith signs all registered slots, matching each signer by address.
func (b *TransactionBuilder) SignWith(signers ...Signer) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	// Map addresses to secret keys.
	signerMap := make(map[crypto.Address]crypto.SecretKeyer, len(signers))
	for _, signer := range signers {
		addr, err := crypto.NewAddressFromPublicKey(&signer.PublicKey)
		if err != nil {
			b.errs = append(b.errs, fmt.Errorf("derive address from signer: %w", err))
			return b
		}
		signerMap[*addr] = signer.SecretKey
	}
	// Sign each slot.
	for _, slot := range b.slots {
		sk, ok := signerMap[slot.Address]
		if !ok {
			b.errs = append(b.errs, fmt.Errorf("no signer found for address %s", slot.Address))
			return b
		}
		switch {
		case len(slot.InstructionIndices) == 0:
			b.AddPayerSig(slot.Address, sk, slot.Mode)
		case len(slot.InstructionIndices) == 1 && slot.IncludePayer:
			b.AddIxAndPayerSig(slot.Address, sk, slot.InstructionIndices[0], slot.Mode)
		default:
			b.AddIxesSig(slot.Address, sk, slot.InstructionIndices, slot.IncludePayer, slot.Mode)
		}
		if len(b.errs) > 0 {
			return b
		}
	}
	return b
}

// AddPayerSig signs acSig payer (bit63) with real signing and adds the signature.
func (b *TransactionBuilder) AddPayerSig(account crypto.Address, sk crypto.SecretKeyer, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := NewAccountSignatureBuilder().
		AuthorizePayer().
		Sign(account, sk, b.tx.TxHash(), nil, mode).
		Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(account, *sig)
	return b
}

// AddIxAndPayerSig signs both instruction execution (ixIndex) and gas (bit63) with real signing.
func (b *TransactionBuilder) AddIxAndPayerSig(account crypto.Address, sk crypto.SecretKeyer, ixIndex uint8, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	ixPart, err := b.ixHashesForIndices([]uint8{ixIndex})
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	sig, err := NewAccountSignatureBuilder().
		AuthorizeIxAndPayer(ixIndex).
		Sign(account, sk, b.tx.TxHash(), ixPart, mode).
		Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(account, *sig)
	return b
}

// AddIxesSig signs multiple instructions (optionally including payer bit63) with real signing.
func (b *TransactionBuilder) AddIxesSig(account crypto.Address, sk crypto.SecretKeyer, ixIndices []uint8, includePayer bool, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	ixPart, err := b.ixHashesForIndices(ixIndices)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	sigBuilder := NewAccountSignatureBuilder().AuthorizeIxes(ixIndices)
	if includePayer {
		sigBuilder.AuthorizePayer()
	}
	sig, err := sigBuilder.Sign(account, sk, b.tx.TxHash(), ixPart, mode).Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(account, *sig)
	return b
}

// ResetSigs clears all signatures (e.g., switch from simulated to real signing).
func (b *TransactionBuilder) ResetSigs() *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.tx.TxSigs = make([]TransactionSignatures, 0)
	return b
}

// ixHashesForIndices builds the IxHashItem list for the given instruction indices,
// rejecting reserved and out-of-range indices (previously silently skipped).
func (b *TransactionBuilder) ixHashesForIndices(ixIndices []uint8) ([]IxHashItem, error) {
	hashes := b.tx.IxHashes()
	items := make([]IxHashItem, 0, len(ixIndices))
	for _, i := range ixIndices {
		if i == AuthPayerBit {
			return nil, fmt.Errorf("ix index cannot be AuthPayerBit (%d)", AuthPayerBit)
		}
		if i == AuthReservedBit {
			return nil, fmt.Errorf("ix index cannot be AuthReservedBit (%d)", AuthReservedBit)
		}
		if int(i) >= len(hashes) {
			return nil, fmt.Errorf("ix index %d out of range (max %d)", i, len(hashes)-1)
		}
		items = append(items, IxHashItem{Index: i, Hash: hashes[i]})
	}
	return items, nil
}

// Build finalizes and returns the Transaction. Returns the first error encountered in the chain.
func (b *TransactionBuilder) Build() (*Transaction, error) {
	if len(b.errs) > 0 {
		return nil, b.errs[0]
	}
	return b.tx, nil
}
