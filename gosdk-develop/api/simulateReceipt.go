package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	rust
		struct FramedSimulateReceipt {
			magic: [u8; 4],
			version: u8,
			tx_id: milon_transaction::TxID,
			tx_hash: [u8; 32],
			state: u8,
			access: Vec<FramedAccess>,
			events: Vec<FramedDynamicValue>,
			error: Option<TxFailurePayload>,
			gas_charged: u64,
		}

		struct FramedAccess {
			resource_id: RsHash,
			first_snapshot: Option<FramedPersistedValue>,
			last_written: FramedPersistedValue,
		}

		enum FramedPersistedValue {
			Inline(FramedDynamicValue),
			External([u8; 32]),
		}

		struct FramedDynamicValue {
			type_tag: u64,
			body: Vec<u8>,
		}

		pub struct TxFailurePayload {
			pub code: u16,
			pub message: String,
			pub data: Vec<u8>,
		}
*/

type SimulateReceipt struct {
	Magic      [4]byte
	Version    uint8
	TxID       TxId
	TxHash     TxHash
	State      uint8
	Access     []AccessRecord
	Events     []TypeTagWithData
	Error      *TxFailurePayload
	GasCharged uint64
}

type TxFailurePayload struct {
	Code    uint16
	Message string
	Data    []byte
}

func (r *SimulateReceipt) MarshalPostcard(serializer *postcard.Serializer) error {
	// 1. Magic (4 bytes)
	serializer.SerializeFixedBytes(r.Magic[:])

	// 2.Version (u8)
	if err := serializer.SerializeU8(r.Version); err != nil {
		return fmt.Errorf("failed to serialize Version: %w", err)
	}

	// 3. TxID (12 bytes)
	serializer.SerializeFixedBytes(r.TxID[:])

	// 4. TxHash (32 bytes)
	serializer.SerializeFixedBytes(r.TxHash[:])

	// 5. State (u8)
	if err := serializer.SerializeU8(r.State); err != nil {
		return fmt.Errorf("failed to serialize State: %w", err)
	}

	// 6. Access records (Vec<AccessRecord>)
	if err := postcard.SerializeSeq(serializer, r.Access, func(s *postcard.Serializer, rec AccessRecord) error {
		s.SerializeFixedBytes(rec.ResourceID[:])

		// FirstSnapshot: Option<PersistedValue>
		if err := postcard.SerializeOption(s, rec.FirstSnapshot, func(ss *postcard.Serializer, pv PersistedValue) error {
			return SerializePersistedValue(ss, pv)
		}); err != nil {
			return fmt.Errorf("failed to serialize FirstSnapshot: %w", err)
		}

		// LastWritten: PersistedValue
		if err := SerializePersistedValue(s, rec.LastWritten); err != nil {
			return fmt.Errorf("failed to serialize LastWritten: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to serialize Access records: %w", err)
	}

	// 7. Events (Vec<FramedDynamicValue>)
	if err := postcard.SerializeSeq(serializer, r.Events, func(s *postcard.Serializer, event TypeTagWithData) error {
		if err := s.SerializeU64(event.TypeTag); err != nil {
			return fmt.Errorf("failed to serialize event TypeTag: %w", err)
		}
		// body: Vec<u8>: length-prefixed payload
		if err := s.SerializeBytes(event.Value); err != nil {
			return fmt.Errorf("failed to serialize event body: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to serialize Events: %w", err)
	}

	// 8. Error Option<TxFailurePayload>
	if err := postcard.SerializeOption(serializer, r.Error, func(s *postcard.Serializer, p TxFailurePayload) error {
		return p.MarshalPostcard(s)
	}); err != nil {
		return fmt.Errorf("failed to serialize Error: %w", err)
	}

	// 9. GasCharged (u64)
	if err := serializer.SerializeU64(r.GasCharged); err != nil {
		return fmt.Errorf("failed to serialize GasCharged: %w", err)
	}

	return nil
}

func (p *TxFailurePayload) MarshalPostcard(serializer *postcard.Serializer) error {
	// Code (u16, varint)
	if err := serializer.SerializeU16(p.Code); err != nil {
		return fmt.Errorf("failed to serialize Code: %w", err)
	}

	// Message (String)
	if err := serializer.SerializeStr(p.Message); err != nil {
		return fmt.Errorf("failed to serialize Message: %w", err)
	}

	// Data (Vec<u8>)
	if err := serializer.SerializeBytes(p.Data); err != nil {
		return fmt.Errorf("failed to serialize Data: %w", err)
	}

	return nil
}

func (r *SimulateReceipt) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	// 1. Magic (4 bytes)
	magic, err := deserializer.DeserializeFixedBytes(4)
	if err != nil {
		return fmt.Errorf("failed to deserialize Magic: %w", err)
	}
	copy(r.Magic[:], magic)

	// 2.Version (u8)
	version, err := deserializer.DeserializeU8()
	if err != nil {
		return fmt.Errorf("failed to deserialize Version: %w", err)
	}
	r.Version = version

	// 3. TxID (12 bytes)
	txIdBytes, err := deserializer.DeserializeFixedBytes(TxIdLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize TxID: %w", err)
	}
	copy(r.TxID[:], txIdBytes)

	// 4. TxHash (32 bytes)
	txHashBytes, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize TxHash: %w", err)
	}
	copy(r.TxHash[:], txHashBytes)

	// 5. State (u8)
	state, err := deserializer.DeserializeU8()
	if err != nil {
		return fmt.Errorf("failed to deserialize State: %w", err)
	}
	r.State = state

	// 6. Access records Vec<AccessRecord>
	accessRecords, err := postcard.DeserializeSeq(deserializer, DeserializeAccessRecord)
	if err != nil {
		return fmt.Errorf("failed to deserialize Access records: %w", err)
	}
	r.Access = accessRecords

	// 7. Events Vec<AnySerializeOwned>
	events, err := postcard.DeserializeSeq(deserializer, DeserializeEventEntry)
	if err != nil {
		return fmt.Errorf("failed to deserialize Events: %w", err)
	}
	r.Events = events

	// 8. Error Option<TxFailurePayload>
	errorPayload, err := postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (TxFailurePayload, error) {
		var p TxFailurePayload
		if err := p.UnmarshalPostcard(d); err != nil {
			return p, err
		}
		return p, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Error: %w", err)
	}
	r.Error = errorPayload

	// 9. GasCharged (u64)
	gasCharged, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize GasCharged: %w", err)
	}
	r.GasCharged = gasCharged

	return nil
}

func (p *TxFailurePayload) UnmarshalPostcard(d *postcard.Deserializer) error {
	code, err := d.DeserializeU16()
	if err != nil {
		return fmt.Errorf("failed to deserialize Code: %w", err)
	}
	p.Code = code

	message, err := d.DeserializeStr()
	if err != nil {
		return fmt.Errorf("failed to deserialize Message: %w", err)
	}
	p.Message = message

	data, err := d.DeserializeBytes()
	if err != nil {
		return fmt.Errorf("failed to deserialize Data: %w", err)
	}
	p.Data = data
	return nil
}
