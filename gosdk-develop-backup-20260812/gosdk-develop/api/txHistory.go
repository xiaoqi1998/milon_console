package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/types"
)

type TxHistory struct {
	Stamp        uint64
	Payer        *uint8
	Signatures   []TxHistorySignature
	Instructions []PackedInstruction
	Receipt      TxReceipt
}

type TxHistorySignature struct {
	Signer  crypto.Address
	AuthBit types.Bitmap64
	SigBit  types.Bitmap64
}

type TxReceipt struct {
	TxID       TxId
	TxHash     TxHash
	State      uint8
	Access     []AccessRecord
	Events     []TypeTagWithData
	Error      *uint16
	GasCharged uint64
}

func (h *TxHistory) MarshalPostcard(serializer *postcard.Serializer) error {
	// 1. Stamp (u64)
	if err := serializer.SerializeU64(h.Stamp); err != nil {
		return fmt.Errorf("failed to serialize Stamp: %w", err)
	}

	// 2. Payer (Option<u8>)
	if err := postcard.SerializeOption(serializer, h.Payer, func(s *postcard.Serializer, payer uint8) error {
		return s.SerializeU8(payer)
	}); err != nil {
		return fmt.Errorf("failed to serialize Payer: %w", err)
	}

	// 3. Signatures (Vec<TxHistorySignature>)
	if err := postcard.SerializeSeq(serializer, h.Signatures, func(s *postcard.Serializer, sig TxHistorySignature) error {
		s.SerializeFixedBytes(sig.Signer.Bytes[:])
		if err := s.SerializeU64(uint64(sig.AuthBit)); err != nil {
			return fmt.Errorf("failed to serialize AuthBit: %w", err)
		}
		if err := s.SerializeU64(uint64(sig.SigBit)); err != nil {
			return fmt.Errorf("failed to serialize SigBit: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to serialize Signatures: %w", err)
	}

	// 4. Instructions (Vec<PackedInstruction>)
	if err := postcard.SerializeSeq(serializer, h.Instructions, func(s *postcard.Serializer, instr PackedInstruction) error {
		return s.SerializeBytes(instr)
	}); err != nil {
		return fmt.Errorf("failed to serialize Instructions: %w", err)
	}

	// 5. Receipt (TxReceipt)
	if err := h.Receipt.MarshalPostcard(serializer); err != nil {
		return fmt.Errorf("failed to serialize Receipt: %w", err)
	}

	return nil
}

func (r *TxReceipt) MarshalPostcard(serializer *postcard.Serializer) error {
	// 1. TxID (12 bytes)
	serializer.SerializeFixedBytes(r.TxID[:])

	// 2. TxHash (32 bytes)
	serializer.SerializeFixedBytes(r.TxHash[:])

	// 3. State (u8)
	if err := serializer.SerializeU8(r.State); err != nil {
		return fmt.Errorf("failed to serialize State: %w", err)
	}

	// 4. Access records (Vec<AccessRecord>)
	if err := postcard.SerializeSeq(serializer, r.Access, func(s *postcard.Serializer, rec AccessRecord) error {
		s.SerializeFixedBytes(rec.ResourceID[:])

		// FirstSnapshot: Option<PersistedValue>
		if err := postcard.SerializeOption(s, rec.FirstSnapshot, func(ss *postcard.Serializer, pv PersistedValue) error {
			return serializePersistedValue(ss, pv)
		}); err != nil {
			return fmt.Errorf("failed to serialize FirstSnapshot: %w", err)
		}

		// LastWritten: PersistedValue
		if err := serializePersistedValue(s, rec.LastWritten); err != nil {
			return fmt.Errorf("failed to serialize LastWritten: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to serialize Access records: %w", err)
	}

	// 5. Events (Vec<AnySerializeOwned>)
	if err := postcard.SerializeSeq(serializer, r.Events, func(s *postcard.Serializer, event TypeTagWithData) error {
		if err := s.SerializeU64(event.TypeTag); err != nil {
			return fmt.Errorf("failed to serialize event TypeTag: %w", err)
		}
		// AnySerializeOwned: value 没有 length prefix，直接写入
		s.SerializeFixedBytes(event.Value)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to serialize Events: %w", err)
	}

	// 6. Error (Option<u16>)
	if err := postcard.SerializeOption(serializer, r.Error, func(s *postcard.Serializer, code uint16) error {
		return s.SerializeU16(code)
	}); err != nil {
		return fmt.Errorf("failed to serialize Error: %w", err)
	}

	// 7.GasCharged (u64)
	if err := serializer.SerializeU64(r.GasCharged); err != nil {
		return fmt.Errorf("failed to serialize GasCharged: %w", err)
	}

	return nil
}

func serializePersistedValue(serializer *postcard.Serializer, pv PersistedValue) error {
	if err := serializer.SerializeU32(pv.Variant); err != nil {
		return fmt.Errorf("failed to serialize variant: %w", err)
	}

	switch pv.Variant {
	case 0:
		// Inline(AnySerializeOwned)
		if err := serializer.SerializeU64(pv.TypeTag); err != nil {
			return fmt.Errorf("failed to serialize type_tag: %w", err)
		}
		serializer.SerializeFixedBytes(pv.InlineData)
	case 1:
		// External(BlobHash)
		serializer.SerializeFixedBytes(pv.ExternalHash[:])
	default:
		return fmt.Errorf("unknown PersistedValue variant: %d", pv.Variant)
	}
	return nil
}

func (h *TxHistory) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	// 1. Stamp (u64)
	stamp, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Stamp: %w", err)
	}
	h.Stamp = stamp

	// 2. Payer (Option<u8>)
	payer, err := postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (uint8, error) {
		return d.DeserializeU8()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Payer: %w", err)
	}
	h.Payer = payer

	// 3. Signatures (Vec<TxHistorySignature>)
	signatures, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (TxHistorySignature, error) {
		var sig TxHistorySignature

		// Signer (Address, 20 bytes)
		signerBytes, err := d.DeserializeFixedBytes(20)
		if err != nil {
			return sig, fmt.Errorf("failed to deserialize Signer: %w", err)
		}
		signer, err := crypto.NewAddressFromBytes(signerBytes)
		if err != nil {
			return sig, fmt.Errorf("failed to parse Signer: %w", err)
		}
		sig.Signer = *signer

		// AuthBit (Bitmap64)
		authBit, err := d.DeserializeU64()
		if err != nil {
			return sig, fmt.Errorf("failed to deserialize AuthBit: %w", err)
		}
		sig.AuthBit = types.Bitmap64(authBit)

		// SigBit (Bitmap64)
		sigBit, err := d.DeserializeU64()
		if err != nil {
			return sig, fmt.Errorf("failed to deserialize SigBit: %w", err)
		}
		sig.SigBit = types.Bitmap64(sigBit)

		return sig, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Signatures: %w", err)
	}
	h.Signatures = signatures

	// 4. Instructions (Vec<PackedInstruction>)
	instructions, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (PackedInstruction, error) {
		var instr PackedInstruction

		instr, err = d.DeserializeBytes()
		if err != nil {
			return instr, fmt.Errorf("failed to deserialize Instruction: %w", err)
		}

		return instr, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Instructions: %w", err)
	}
	h.Instructions = instructions

	// 5. Receipt (TxReceipt)
	var receipt TxReceipt
	if err := receipt.UnmarshalPostcard(deserializer); err != nil {
		return fmt.Errorf("failed to deserialize Receipt: %w", err)
	}
	h.Receipt = receipt

	return nil
}

func (r *TxReceipt) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	// 1. TxID (12 bytes)
	txIdBytes, err := deserializer.DeserializeFixedBytes(TxIdLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize TxID: %w", err)
	}
	copy(r.TxID[:], txIdBytes)

	// 2. TxHash (32 bytes)
	txHashBytes, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize TxHash: %w", err)
	}
	copy(r.TxHash[:], txHashBytes)

	// 3. State (u8)
	state, err := deserializer.DeserializeU8()
	if err != nil {
		return fmt.Errorf("failed to deserialize State: %w", err)
	}
	r.State = state

	// 4. Access records (Vec<AccessRecord>)
	accessRecords, err := postcard.DeserializeSeq(deserializer, DeserializeAccessRecord)
	if err != nil {
		return fmt.Errorf("failed to deserialize Access records: %w", err)
	}
	r.Access = accessRecords

	// 5. Events (Vec<AnySerializeOwned>)
	events, err := postcard.DeserializeSeq(deserializer, DeserializeEventEntry)
	if err != nil {
		return fmt.Errorf("failed to deserialize Events: %w", err)
	}
	r.Events = events

	// 6. Error (Option<u16>)
	errorCode, err := postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (uint16, error) {
		return d.DeserializeU16()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Error: %w", err)
	}
	r.Error = errorCode

	// 7. GasCharged (u64)
	gasCharged, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize GasCharged: %w", err)
	}
	r.GasCharged = gasCharged

	return nil
}
