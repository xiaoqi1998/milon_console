package lib

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"sync"
)

var (
	ChainId      uint64 = 900_000_001
	ChainIdMutex sync.RWMutex
)

type TransactionStamp uint64
type RequestID uint64

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

type Transaction struct {
	Stamp        TransactionStamp
	Payer        *crypto.Address
	Instructions []api.PackedInstruction
	TxSigs       []TransactionSignatures
}

type TransactionSignatures struct {
	Address          crypto.Address
	AccountSignature AccountSignature
}

// TxHash = Blake3(MILON_ROOT || TX_HASH_DOMAIN || GetChainId() || Stamp || [Payer] || ix_hashes...)
func (tx *Transaction) TxHash() api.TxHash {
	hasher := crypto.Hasher(crypto.TxHashDomainBytes)

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
		ixHash := tx.IxHashFromWire(instruction)
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

// IxHashes collect all instruction hashes
func (tx *Transaction) IxHashes() []api.TxHash {
	hashes := make([]api.TxHash, len(tx.Instructions))
	for i, instruction := range tx.Instructions {
		hashes[i] = tx.IxHashFromWire(instruction)
	}
	return hashes
}

// IxHashFromWire compute ix hash from PackedInstruction: IxHash = Blake3(MILON_ROOT || IX_HASH_DOMAIN || GetChainId() || wire)
func (tx *Transaction) IxHashFromWire(wire api.PackedInstruction) api.TxHash {
	hasher := crypto.Hasher(crypto.IxHashDomainBytes)

	chainIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chainIDBytes, GetChainId()) // big-endian
	hasher.Write(chainIDBytes)

	hasher.Write(wire)

	var hash api.TxHash
	hasher.Sum(hash[:0])
	return hash
}

// ValidateWire validates the transaction wire layer structure:
// instruction count/hash, signature owner, auth bitmap, and gas signature.
// Equivalent to ValidateWireWith(nil).
func (tx *Transaction) ValidateWire() error {
	return tx.ValidateWireWith([]uint8{})
}

// ValidateWireWith validates the transaction wire layer structure with
// sponsored instructions.
//
// sponsorIx lists sponsored instruction indices (e.g. []uint8{0, 2}), which
// skip the gas signature check. In UnifiedPayer mode it has no effect.
//
// Gas signature check:
//   - UnifiedPayer (tx.Payer != nil): payer must sign authorizing bit63
//   - SplitPayer (tx.Payer == nil): each unsponsored ix needs someone
//     authorizing both bit63 and that ix; authorizing only bit63 is a
//     gas payment mode conflict
func (tx *Transaction) ValidateWireWith(sponsorIx []uint8) error {
	if len(tx.Instructions) == 0 {
		return fmt.Errorf("empty instructions")
	}

	if len(tx.Instructions) > AuthReservedBit {
		return fmt.Errorf("too many instructions: %d (max %d)", len(tx.Instructions), AuthReservedBit)
	}

	seenIx := make(map[api.TxHash]bool)
	for _, wire := range tx.Instructions {
		h := tx.IxHashFromWire(wire)
		if seenIx[h] {
			return fmt.Errorf("duplicate ix hash")
		}
		seenIx[h] = true
	}

	owners := make(map[crypto.Address]bool)
	for _, sig := range tx.TxSigs {
		if owners[sig.Address] {
			return fmt.Errorf("duplicate signature owner")
		}
		owners[sig.Address] = true

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

	if tx.Payer != nil { // UnifiedPayer mode
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
	} else { // SplitPayerSelfPay
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
			hasIx := (sig.AccountSignature.AuthBit.Raw() & ((uint64(1) << AuthReservedBit) - 1)) != 0
			if hasPayer && !hasIx {
				return fmt.Errorf("gas payment mode conflict")
			}
		}
	}

	return nil
}

// ToBytes serialize to byte array
func (tx *Transaction) ToBytes() ([]byte, error) {
	est := 64 + len(tx.Instructions)*100 + len(tx.TxSigs)*800
	serializer := postcard.NewSerializerWithCap(est)
	if err := tx.MarshalPostcard(serializer); err != nil {
		return nil, err
	}
	return serializer.Bytes(), nil
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
