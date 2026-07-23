package milon

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"sync"
	"time"
)

var (
	ChainId      uint64 = 900_000_001
	ChainIdMutex sync.RWMutex
)

type TransactionStamp uint64

func SetChainId(id uint64) {
	ChainIdMutex.Lock()
	defer ChainIdMutex.Unlock()

	ChainId = id
}
func GetChainId() uint64 {
	ChainIdMutex.RLock()
	defer ChainIdMutex.RUnlock()

	return ChainId
}

func NewTransactionWithParam(instructions []api.PackedInstruction, payer *crypto.Address, options ...any) (*Transaction, error) {
	stamp := TransactionStamp(time.Now().UnixMilli())
	txSigs := make([]TransactionSignatures, 0)

	for opti, option := range options {
		switch ovalue := option.(type) {
		case TransactionStamp:
			stamp = ovalue
		case []TransactionSignatures:
			txSigs = append(txSigs, ovalue...)
		default:
			return nil, fmt.Errorf("NewTransactionWithParam arg [%d] unknown option type %T", opti+2, option)
		}
	}

	tx := &Transaction{
		Stamp:        stamp,
		Payer:        payer,
		Instructions: instructions,
		TxSigs:       make([]TransactionSignatures, 0),
	}

	if len(txSigs) > 0 {
		tx.TxSigs = txSigs
	}

	return tx, nil
}

func NewTransactionFromBytes(data []byte) (*Transaction, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (*Transaction, error) {
		var tx Transaction
		err := tx.UnmarshalPostcard(d)
		return &tx, err
	}, false)
}

type Transaction struct {
	Stamp        TransactionStamp
	Payer        *crypto.Address // non-nil = unified-payer mode, nil = split-payer mode
	Instructions []api.PackedInstruction
	TxSigs       []TransactionSignatures
}

type TransactionSignatures struct {
	Address          crypto.Address
	AccountSignature AccountSignature
}

// TxHash = Blake3(MILON_ROOT || TX_HASH_DOMAIN || chain_id || Stamp || [Payer] || ix_hashes...)
func (tx *Transaction) TxHash() api.TxHash {
	hasher := crypto.Hash32Hasher([]byte(crypto.MilonTxHashDomainContext))

	chainIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chainIDBytes, GetChainId()) //big-endian
	hasher.Write(chainIDBytes)

	stampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(stampBytes, uint64(tx.Stamp)) //big-endian
	hasher.Write(stampBytes)

	if tx.Payer != nil {
		payerBytes := tx.Payer.AsBytes()
		hasher.Write(payerBytes[:])
	}

	for _, instruction := range tx.Instructions {
		ixHash := tx.ixHashFromWire(instruction)
		hasher.Write(ixHash[:])
	}

	var hash api.TxHash
	hasher.Sum(hash[:0])
	return hash
}

// AddSignature add signature
func (tx *Transaction) AddSignature(address crypto.Address, accountSig AccountSignature) {
	tx.TxSigs = append(tx.TxSigs, TransactionSignatures{Address: address, AccountSignature: accountSig})
}

// PushSignatures batch push signatures
func (tx *Transaction) PushSignatures(entries []TransactionSignatures) {
	for _, entry := range entries {
		tx.AddSignature(entry.Address, entry.AccountSignature)
	}
}

// SignIx sign instruction at specified index
func (tx *Transaction) SignIx(owner crypto.Address, sk crypto.SecretKeyer, ixIndex uint8, mode AccountSignatureMode) (*AccountSignature, error) {
	if int(ixIndex) >= len(tx.Instructions) {
		return nil, fmt.Errorf("ix index %d out of range", ixIndex)
	}

	authBit, err := AuthIx(ixIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth bit: %w", err)
	}

	ixPart := []IxHashItem{
		{
			Index: ixIndex,
			Hash:  tx.ixHashFromWire(tx.Instructions[ixIndex]),
		},
	}

	return Sign(owner, sk, authBit, tx.TxHash(), ixPart, mode)
}

// SimulateSignIx simulate sign instruction at specified index
func (tx *Transaction) SimulateSignIx(owner crypto.Address, ixIndex uint8, mode AccountSignatureMode) (*AccountSignature, error) {
	if int(ixIndex) >= len(tx.Instructions) {
		return nil, fmt.Errorf("ix index %d out of range", ixIndex)
	}

	authBit, err := AuthIx(ixIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth bit: %w", err)
	}

	return SimulateSign(owner, authBit, mode)
}

// SignPayer sign payer
func (tx *Transaction) SignPayer(payer crypto.Address, sk crypto.SecretKeyer, mode AccountSignatureMode) (*AccountSignature, error) {
	authBit := AuthPayer()

	return Sign(payer, sk, authBit, tx.TxHash(), []IxHashItem{}, mode)
}

// SimulateSignPayer simulate sign payer
func (tx *Transaction) SimulateSignPayer(payer crypto.Address, mode AccountSignatureMode) (*AccountSignature, error) {
	authBit := AuthPayer()

	return SimulateSign(payer, authBit, mode)
}

// SignIxAndPayer sign both ix and payer's bit63
func (tx *Transaction) SignIxAndPayer(owner crypto.Address, sk crypto.SecretKeyer, ixIndex uint8, mode AccountSignatureMode) (*AccountSignature, error) {
	if int(ixIndex) >= len(tx.Instructions) {
		return nil, fmt.Errorf("ix index %d out of range", ixIndex)
	}

	authBit, err := AuthIxAndPayer(ixIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth bit: %w", err)
	}

	ixPart, err := tx.ixPart([]uint8{ixIndex})
	if err != nil {
		return nil, fmt.Errorf("failed to collect ix part: %w", err)
	}

	return Sign(owner, sk, authBit, tx.TxHash(), ixPart, mode)
}

// SimulateSignIxAndPayer simulate sign both ix and payer's bit63
func (tx *Transaction) SimulateSignIxAndPayer(owner crypto.Address, ixIndex uint8, mode AccountSignatureMode) (*AccountSignature, error) {
	if int(ixIndex) >= len(tx.Instructions) {
		return nil, fmt.Errorf("ix index %d out of range", ixIndex)
	}

	authBit, err := AuthIxAndPayer(ixIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth bit: %w", err)
	}

	return SimulateSign(owner, authBit, mode)
}

// SignIxes sign multiple instructions (optionally include payer's bit63)
func (tx *Transaction) SignIxes(owner crypto.Address, sk crypto.SecretKeyer, ixIndices []uint8, includePayer bool, mode AccountSignatureMode) (*AccountSignature, error) {
	authBit, err := AuthIxes(ixIndices)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth bit: %w", err)
	}

	if includePayer {
		authBit = authBit.Set(AuthPayerBit)
	}

	ixPart, err := tx.ixPart(ixIndices)
	if err != nil {
		return nil, fmt.Errorf("failed to collect ix part: %w", err)
	}

	return Sign(owner, sk, authBit, tx.TxHash(), ixPart, mode)
}

// SimulateSignIxes simulate sign multiple instructions (optionally include payer's bit63)
func (tx *Transaction) SimulateSignIxes(owner crypto.Address, ixIndices []uint8, includePayer bool, mode AccountSignatureMode) (*AccountSignature, error) {
	authBit, err := AuthIxes(ixIndices)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth bit: %w", err)
	}

	if includePayer {
		authBit = authBit.Set(AuthPayerBit)
	}

	return SimulateSign(owner, authBit, mode)
}

// SignIxGas sign ix + gas (split-payer mode, payer=None)
func (tx *Transaction) SignIxGas(owner crypto.Address, sk crypto.SecretKeyer, ixIndex uint8, mode AccountSignatureMode) (*AccountSignature, error) {
	if int(ixIndex) >= len(tx.Instructions) {
		return nil, fmt.Errorf("ix index %d out of range", ixIndex)
	}

	authBit, err := AuthIxAndPayer(ixIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth bit: %w", err)
	}

	ixPart := []IxHashItem{
		{
			Index: ixIndex,
			Hash:  tx.ixHashFromWire(tx.Instructions[ixIndex]),
		},
	}

	return Sign(owner, sk, authBit, tx.TxHash(), ixPart, mode)
}

// SimulateSignIxGas simulate sign ix + gas (split-payer mode, payer=None)
func (tx *Transaction) SimulateSignIxGas(owner crypto.Address, ixIndex uint8, mode AccountSignatureMode) (*AccountSignature, error) {
	if int(ixIndex) >= len(tx.Instructions) {
		return nil, fmt.Errorf("ix index %d out of range", ixIndex)
	}

	authBit, err := AuthIxAndPayer(ixIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth bit: %w", err)
	}

	return SimulateSign(owner, authBit, mode)
}

// IxHashes collect all instruction hashes
func (tx *Transaction) IxHashes() []api.TxHash {
	hashes := make([]api.TxHash, len(tx.Instructions))
	for i, instruction := range tx.Instructions {
		hashes[i] = tx.ixHashFromWire(instruction)
	}
	return hashes
}

// ixHashFromWire compute ix hash from PackedInstruction: IxHash = Blake3(MILON_ROOT || IX_HASH_DOMAIN || chain_id || wire)
func (tx *Transaction) ixHashFromWire(wire api.PackedInstruction) api.TxHash {
	hasher := crypto.Hash32Hasher([]byte(crypto.MilonIxHashDomainContext))

	chainIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chainIDBytes, GetChainId()) // big-endian
	hasher.Write(chainIDBytes)

	hasher.Write(wire)

	var hash api.TxHash
	hasher.Sum(hash[:0])
	return hash
}

// ixPart collect ix hash items for specified indices
func (tx *Transaction) ixPart(ixIndices []uint8) ([]IxHashItem, error) {
	hashes := tx.IxHashes()
	items := make([]IxHashItem, 0, len(ixIndices))

	for _, i := range ixIndices {
		if i == AuthPayerBit {
			return nil, fmt.Errorf("ix index cannot be AuthPayerBit (%d)", AuthPayerBit)
		}
		if int(i) >= len(hashes) {
			return nil, fmt.Errorf("ix index %d out of range (max %d)", i, len(hashes)-1)
		}
		items = append(items, IxHashItem{Index: i, Hash: hashes[i]})
	}

	return items, nil
}

// ResolvePayer derive payer address from bit63 signature
func (tx *Transaction) ResolvePayer() (crypto.Address, error) {
	for _, sig := range tx.TxSigs {
		if sig.AccountSignature.AuthorizesPayer() {
			return sig.Address, nil
		}
	}
	return crypto.Address{}, fmt.Errorf("payer signature required")
}

// ValidateWire validate transaction wire layer structure
//
// Validation includes:
// 1. instruction count: 1 <= len <= 63
// 2. instruction hash: no duplicates allowed
// 3. signature owner: same address can only have one signature
// 4. authorization bitmap: auth_bit non-empty, index not out of bounds
// 5. Gas signature check:
//   - Unified-payer mode (tx.Payer != nil): must have payer signature authorizing bit63
//   - Split-payer mode (tx.Payer == nil): each ix must have someone authorizing both bit63 + that ix
//
// Example:
//
//	tx := NewTransactionWithParam(instructions, payer)
//	// ... add signatures ...
//	if err := tx.ValidateWire(); err != nil {
//	    log.Printf("validation failed: %v", err)
//	}
func (tx *Transaction) ValidateWire() error {
	return tx.ValidateWireWith([]uint8{})
}

// ValidateWireWith validate transaction wire layer structure, support sponsored instructions (sponsor_ix)
//
// Parameter sponsorIx:
//   - list of sponsored instruction indices, e.g., []uint8{0, 2} means ix=0 and ix=2 are externally sponsored
//   - sponsored instructions don't need gas signature (no need to authorize both bit63 + that ix)
//   - in unified-payer mode, this parameter does not affect validation
//
// Validation logic:
// 1. Basic structure check (same as ValidateWire)
//
// 2. Gas signature check (differs by mode):
//
//	a) Unified-payer mode (tx.Payer != nil):
//	   - check if payer address has signature authorizing bit63
//	   - sponsorIx parameter does not affect this mode's validation
//	   - applicable: platform uniformly pays all gas
//
//	b) Split-payer mode (tx.Payer == nil):
//	   - for each unsponsored ix, check if someone authorizes both bit63 + that ix
//	   - ponsored ix skips check
//	   - additional check: if someone only authorizes bit63 but no ix, report "gas payment mode conflict"
//	   - applicable: multiple parties each bear gas, or some instructions sponsored by third party
//
// Usage scenario examples:
//
// Scenario 1: normal validation (no sponsorship)
//
//	tx := NewTransactionWithParam(instructions, nil)
//	// ... user signs all instructions ...
//	err := tx.ValidateWireWith([]uint8{}) // equivalent to ValidateWire()
//
// Scenario 2: partial instructions sponsored by platform
//
//	tx := NewTransactionWithParam([]api.PackedInstruction{
//	    {1, 2, 3},  // ix=0: platform sponsored
//	    {4, 5, 6},  // ix=1: user self-paid
//	    {7, 8, 9},  // ix=2: platform sponsored
//	}, nil)
//	// user only signs ix=1
//	userSig := tx.SignIxAndPayer(user, sk, 1, mode)
//	tx.AddSignature(*user, *userSig)
//	// sponsor ix=0 and ix=2, validation passes
//	err := tx.ValidateWireWith([]uint8{0, 2})
//
// Scenario 3: unified-payer mode (sponsorIx has no effect)
//
//	tx := NewTransactionWithParam(instructions, payer)
//	payerSig := tx.SignPayerAndAddSignature(payer, sk, mode)
//	tx.AddSignature(*payer, *payerSig)
//	err := tx.ValidateWireWith([]uint8{0}) // sponsorIx ignored
func (tx *Transaction) ValidateWireWith(sponsorIx []uint8) error {
	if len(tx.Instructions) == 0 {
		return fmt.Errorf("empty instructions")
	}

	if len(tx.Instructions) > AuthPayerBit {
		return fmt.Errorf("too many instructions: %d (max %d)", len(tx.Instructions), AuthPayerBit)
	}

	seenIx := make(map[string]bool)
	for _, wire := range tx.Instructions {
		h := tx.ixHashFromWire(wire)
		key := string(h[:])
		if seenIx[key] {
			return fmt.Errorf("duplicate ix hash")
		}
		seenIx[key] = true
	}

	owners := make(map[string]bool)
	for _, sig := range tx.TxSigs {
		ownerKey := string(sig.Address.Bytes[:])
		if owners[ownerKey] {
			return fmt.Errorf("duplicate signature owner")
		}
		owners[ownerKey] = true

		if sig.AccountSignature.AuthBit.Raw() == 0 {
			return fmt.Errorf("empty auth bit")
		}

		for i := uint8(0); i < 64; i++ {
			if sig.AccountSignature.AuthBit.Test(i) {
				if i != AuthPayerBit && int(i) >= len(tx.Instructions) {
					return fmt.Errorf("auth ix index %d out of range", i)
				}
			}
		}
	}

	sponsorSet := make(map[uint8]bool)
	for _, idx := range sponsorIx {
		sponsorSet[idx] = true
	}

	if tx.Payer != nil { // Unified-payer mode
		// Only check that payer has signed and authorized bit63
		hasPayerSig := false
		for _, sig := range tx.TxSigs {
			if bytes.Equal(sig.Address.Bytes[:], tx.Payer.Bytes[:]) && sig.AccountSignature.AuthorizesPayer() {
				hasPayerSig = true
				break
			}
		}
		if !hasPayerSig {
			return fmt.Errorf("payer signature required")
		}
	} else { // Split-payer mode
		// For each ix, check if someone has authorized both bit63 and this ix
		for i := range tx.Instructions {
			ixIndex := uint8(i)
			if sponsorSet[ixIndex] {
				continue
			}
			hasGas := false
			for _, sig := range tx.TxSigs {
				if sig.AccountSignature.AuthorizesPayer() && sig.AccountSignature.AuthorizesIx(ixIndex) {
					hasGas = true
					break
				}
			}
			if !hasGas {
				return fmt.Errorf("gas signer required for ix %d", ixIndex)
			}
		}

		for _, sig := range tx.TxSigs {
			hasPayer := sig.AccountSignature.AuthorizesPayer()
			hasIx := (sig.AccountSignature.AuthBit.Raw() & ((uint64(1) << AuthPayerBit) - 1)) != 0
			if hasPayer && !hasIx {
				return fmt.Errorf("gas payment mode conflict")
			}
		}
	}

	return nil
}

// ToBytes serialize to byte array
func (tx *Transaction) ToBytes() ([]byte, error) {
	return postcard.SerializePostcard(tx)
}

func (tx *Transaction) MarshalPostcard(serializer *postcard.Serializer) error {
	err := serializer.SerializeU64(uint64(tx.Stamp))
	if err != nil {
		return fmt.Errorf("failed to serialize Stamp: %w", err)
	}

	err = postcard.SerializeOption(serializer, tx.Payer, func(s *postcard.Serializer, addr crypto.Address) error {
		return addr.MarshalPostcard(s)
	})
	if err != nil {
		return fmt.Errorf("failed to serialize Payer: %w", err)
	}

	err = postcard.SerializeSeq(serializer, tx.Instructions, func(s *postcard.Serializer, wire api.PackedInstruction) error {
		return s.SerializeBytes(wire)
	})
	if err != nil {
		return fmt.Errorf("failed to serialize Instructions: %w", err)
	}

	err = postcard.SerializeSeq(serializer, tx.TxSigs, func(s *postcard.Serializer, sig TransactionSignatures) error {
		if err = sig.Address.MarshalPostcard(s); err != nil {
			return fmt.Errorf("failed to serialize Address: %w", err)
		}
		if err = sig.AccountSignature.MarshalPostcard(s); err != nil {
			return fmt.Errorf("failed to serialize AccountSignature: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to serialize TxSigs: %w", err)
	}

	return nil
}

func (tx *Transaction) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	stamp, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Stamp: %w", err)
	}
	tx.Stamp = TransactionStamp(stamp)

	payer, err := postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (crypto.Address, error) {
		var addr crypto.Address
		if err = addr.UnmarshalPostcard(d); err != nil {
			return addr, fmt.Errorf("failed to deserialize Address: %w", err)
		}
		return addr, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Payer: %w", err)
	}
	tx.Payer = payer

	instructions, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (api.PackedInstruction, error) {
		wire, err := d.DeserializeBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize PackedInstruction: %w", err)
		}
		return wire, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Instructions: %w", err)
	}
	tx.Instructions = instructions

	signatures, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (TransactionSignatures, error) {
		var ts TransactionSignatures

		var addr crypto.Address
		if err = addr.UnmarshalPostcard(d); err != nil {
			return ts, fmt.Errorf("failed to deserialize Address: %w", err)
		}
		ts.Address = addr

		var accountSig AccountSignature
		if err = accountSig.UnmarshalPostcard(d); err != nil {
			return ts, fmt.Errorf("failed to deserialize AccountSignature: %w", err)
		}
		ts.AccountSignature = accountSig

		return ts, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize TxSigs: %w", err)
	}
	tx.TxSigs = signatures

	return nil
}

// **************************************** Build simulated transaction ****************************************//

// BuildSingleIxUnifiedSimulateSign Single-payer mode: build transaction structure with simulate signature (payer signs bit0 and bit63)
func BuildSingleIxUnifiedSimulateSign(payer crypto.Address, acSigMod AccountSignatureMode, wire api.PackedInstruction, options ...any) (*Transaction, error) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{wire}, &payer, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build Transaction: %w", err)
	}

	sig, err := tx.SimulateSignIxGas(payer, 0, acSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix gas: %w", err)
	}

	tx.AddSignature(payer, *sig)
	return tx, nil
}

// BuildSingleIxUnifiedSimulateSignOnlyGas single-payer mode: build transaction structure with simulate signature (payer only signs bit63)
func BuildSingleIxUnifiedSimulateSignOnlyGas(payer crypto.Address, acSigMod AccountSignatureMode, wire api.PackedInstruction, options ...any) (*Transaction, error) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{wire}, &payer, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build Transaction: %w", err)
	}

	sig, err := tx.SimulateSignPayer(payer, acSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix and payer: %w", err)
	}

	tx.AddSignature(payer, *sig)
	return tx, nil
}

// BuildSingleIxSplitSimulateSign Split-payer mode: build transaction structure with simulate signature (tx.Payer=nil)
func BuildSingleIxSplitSimulateSign(owner crypto.Address, acSigMod AccountSignatureMode, wire api.PackedInstruction, options ...any) (*Transaction, error) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{wire}, nil, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build Transaction: %w", err)
	}

	sig, err := tx.SimulateSignIxGas(owner, 0, acSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix gas: %w", err)
	}

	tx.AddSignature(owner, *sig)
	return tx, nil
}

// **************************************** Build simulated transaction ****************************************//

// **************************************** Build signature transaction ****************************************//

// BuildSingleIxUnifiedPayerSignAll Single-payer mode: payer signs both bit63(gas) and bit0(execution), set tx.Payer
func BuildSingleIxUnifiedPayerSignAll(payerSk crypto.SecretKeyer, payer crypto.Address, acSigMod AccountSignatureMode, wire api.PackedInstruction, options ...any) (*Transaction, error) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{wire}, &payer, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build Transaction: %w", err)
	}

	sig, err := tx.SignIxAndPayer(payer, payerSk, 0, acSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix and payer: %w", err)
	}

	tx.AddSignature(payer, *sig)
	return tx, nil
}

// BuildSingleIxUnifiedPayerSignOnlyGas Single-payer mode: payer only signs bit63(gas), no authorization for instruction execution
func BuildSingleIxUnifiedPayerSignOnlyGas(payerSk crypto.SecretKeyer, payer crypto.Address, acSigMod AccountSignatureMode, wire api.PackedInstruction, options ...any) (*Transaction, error) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{wire}, &payer, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build Transaction: %w", err)
	}

	sig, err := tx.SignPayer(payer, payerSk, acSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix and payer: %w", err)
	}

	tx.AddSignature(payer, *sig)
	return tx, nil
}

// BuildSingleIxSplitSign Split-payer mode: owner bears both gas and execution (tx.Payer=nil, sign bit63+bit0)
func BuildSingleIxSplitSign(ownerSk crypto.SecretKeyer, owner crypto.Address, acSigMod AccountSignatureMode, wire api.PackedInstruction, options ...any) (*Transaction, error) {
	tx, err := NewTransactionWithParam([]api.PackedInstruction{wire}, nil, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build Transaction: %w", err)
	}

	sig, err := tx.SignIxGas(owner, ownerSk, 0, acSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix gas: %w", err)
	}

	tx.AddSignature(owner, *sig)
	return tx, nil
}

// **************************************** Build signature transaction ****************************************//
