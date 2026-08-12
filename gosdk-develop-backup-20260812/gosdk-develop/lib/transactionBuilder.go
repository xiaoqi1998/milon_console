package lib

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"time"
)

// **************************************** TransactionBuilder ****************************************//

// SigningSlot 描述一个账户的签名授权（不含私钥）。
type SigningSlot struct {
	Address            crypto.Address
	InstructionIndices []uint8
	IncludePayer       bool
	Mode               AccountSignatureMode
}

// Signer 签名者条目，携带私钥和对应的公钥。
type Signer struct {
	SecretKey crypto.SecretKeyer
	PublicKey crypto.PublicKey
}

type TransactionBuilder struct {
	tx    *Transaction
	slots []SigningSlot
	errs  []error
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

// WithPayer sets the payer address (unified-payer mode).
func (b *TransactionBuilder) WithPayer(payer *crypto.Address) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.tx.Payer = payer
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

// ApplySlots 注册签名槽，声明每个地址的授权范围（不含私钥）。
func (b *TransactionBuilder) ApplySlots(slots []SigningSlot) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.slots = append(b.slots, slots...)
	return b
}

// SimulateSlots 统一模拟签名：对已注册的 slots 逐一模拟（无需私钥）。
func (b *TransactionBuilder) SimulateSlots() *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	for _, slot := range b.slots {
		b.applySimulateSlot(slot)
		if len(b.errs) > 0 {
			return b
		}
	}
	return b
}

func (b *TransactionBuilder) applySimulateSlot(slot SigningSlot) {
	switch {
	case len(slot.InstructionIndices) == 0:
		b.AddSimulatePayerSig(slot.Address, slot.Mode)
	case len(slot.InstructionIndices) == 1 && slot.IncludePayer:
		b.AddSimulateIxAndPayerSig(slot.Address, slot.InstructionIndices[0], slot.Mode)
	default:
		b.AddSimulateIxesSig(slot.Address, slot.InstructionIndices, slot.IncludePayer, slot.Mode)
	}
}

// AddSimulatePayerSig signs as payer (bit63) with simulate signing and adds the signature.
func (b *TransactionBuilder) AddSimulatePayerSig(payer crypto.Address, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := NewAccountSignatureBuilder().
		AuthorizePayer().
		SimulateSign(payer, mode).
		Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(payer, *sig)
	return b
}

// AddSimulateIxAndPayerSig signs both instruction execution and gas with simulate signing.
func (b *TransactionBuilder) AddSimulateIxAndPayerSig(owner crypto.Address, ixIndex uint8, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := NewAccountSignatureBuilder().
		AuthorizeIxAndPayer(ixIndex).
		SimulateSign(owner, mode).
		Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(owner, *sig)
	return b
}

// AddSimulateIxesSig signs multiple instructions (optionally including payer bit63) with simulate signing.
func (b *TransactionBuilder) AddSimulateIxesSig(owner crypto.Address, ixIndices []uint8, includePayer bool, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sigBuilder := NewAccountSignatureBuilder().AuthorizeIxes(ixIndices)
	if includePayer {
		sigBuilder.AuthorizePayer()
	}
	sig, err := sigBuilder.SimulateSign(owner, mode).Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(owner, *sig)
	return b
}

// SignWith 统一签名：根据地址自动匹配签名者，对已注册的 slots 逐一签名。
func (b *TransactionBuilder) SignWith(signers ...Signer) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	// 构建 address → secretKey 映射
	signerMap := make(map[crypto.Address]crypto.SecretKeyer, len(signers))
	for _, signer := range signers {
		addr, err := crypto.NewAddressFromPublicKey(&signer.PublicKey)
		if err != nil {
			b.errs = append(b.errs, fmt.Errorf("derive address from signer: %w", err))
			return b
		}
		signerMap[*addr] = signer.SecretKey
	}
	// 逐 slot 签名
	for _, slot := range b.slots {
		sk, ok := signerMap[slot.Address]
		if !ok {
			b.errs = append(b.errs, fmt.Errorf("no signer found for address %s", slot.Address))
			return b
		}
		b.applySlot(slot, sk)
		if len(b.errs) > 0 {
			return b
		}
	}
	return b
}
func (b *TransactionBuilder) applySlot(slot SigningSlot, sk crypto.SecretKeyer) {
	switch {
	case len(slot.InstructionIndices) == 0:
		b.AddPayerSig(slot.Address, sk, slot.Mode)
	case len(slot.InstructionIndices) == 1 && slot.IncludePayer:
		b.AddIxAndPayerSig(slot.Address, sk, slot.InstructionIndices[0], slot.Mode)
	default:
		b.AddIxesSig(slot.Address, sk, slot.InstructionIndices, slot.IncludePayer, slot.Mode)
	}
}

// AddPayerSig signs as payer (bit63) with real signing and adds the signature.
func (b *TransactionBuilder) AddPayerSig(payer crypto.Address, sk crypto.SecretKeyer, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := NewAccountSignatureBuilder().
		AuthorizePayer().
		Sign(payer, sk, b.tx.TxHash(), nil, mode).
		Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(payer, *sig)
	return b
}

// AddIxAndPayerSig signs both instruction execution (ixIndex) and gas (bit63) with real signing.
func (b *TransactionBuilder) AddIxAndPayerSig(owner crypto.Address, sk crypto.SecretKeyer, ixIndex uint8, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	ixPart := b.ixHashesForIndices([]uint8{ixIndex})
	sig, err := NewAccountSignatureBuilder().
		AuthorizeIxAndPayer(ixIndex).
		Sign(owner, sk, b.tx.TxHash(), ixPart, mode).
		Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(owner, *sig)
	return b
}

// AddIxesSig signs multiple instructions (optionally including payer bit63) with real signing.
func (b *TransactionBuilder) AddIxesSig(owner crypto.Address, sk crypto.SecretKeyer, ixIndices []uint8, includePayer bool, mode AccountSignatureMode) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	ixPart := b.ixHashesForIndices(ixIndices)
	sigBuilder := NewAccountSignatureBuilder().AuthorizeIxes(ixIndices)
	if includePayer {
		sigBuilder.AuthorizePayer()
	}
	sig, err := sigBuilder.Sign(owner, sk, b.tx.TxHash(), ixPart, mode).Build()
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.tx.AddSignature(owner, *sig)
	return b
}

// AddSignature manually adds a pre-built signature to the transaction.
func (b *TransactionBuilder) AddSignature(address crypto.Address, accountSig AccountSignature) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.tx.AddSignature(address, accountSig)
	return b
}

// PushSignatures batch adds pre-built signature entries.
func (b *TransactionBuilder) PushSignatures(entries []TransactionSignatures) *TransactionBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.tx.PushSignatures(entries)
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

// ixHashesForIndices builds IxHashItem list for the given instruction indices.
func (b *TransactionBuilder) ixHashesForIndices(ixIndices []uint8) []IxHashItem {
	hashes := b.tx.IxHashes()
	items := make([]IxHashItem, 0, len(ixIndices))
	for _, i := range ixIndices {
		if int(i) < len(hashes) {
			items = append(items, IxHashItem{Index: i, Hash: hashes[i]})
		}
	}
	return items
}

// Build finalizes and returns the Transaction. Returns the first error encountered in the chain.
func (b *TransactionBuilder) Build() (*Transaction, error) {
	if len(b.errs) > 0 {
		return nil, b.errs[0]
	}
	return b.tx, nil
}
