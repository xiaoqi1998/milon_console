package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

const (
	TxStatePending uint8 = iota
	TxStateSuccess
	TxStateFailed
)

type TxHistoryView struct {
	Stamp        uint64 // 指令中最大时间戳
	Instructions []PackedInstruction
	Receipt      TxReceiptView
}

type TxReceiptView struct {
	TxId                  TxId
	TxHash                TxHash
	State                 uint8 // TxStatePending TxStateSuccess TxStateFailed
	AccessResourceChanges []AccessResourceChange
	Events                []TxReceiptViewEvent
	Error                 *uint16
}

type AccessResourceChange struct {
	ResourceHash RsHash
	Change       AccessChange
}

type AccessChange struct {
	Variant uint8   // 0=Inline, 1=BlobHash
	TypeTag uint64  // Type tag for Inline variant
	Before  *[]byte // Inline: value before change (optional, nil means no prior state); BlobHash: 32-byte hash (optional)
	After   []byte  // Inline: value after change; BlobHash: 32-byte hash
}

type TxReceiptViewEvent struct {
	EventIndex uint32
	TypeTag    uint64
	Data       []byte
}

func (thv *TxHistoryView) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := serializer.SerializeU64(thv.Stamp); err != nil {
		return fmt.Errorf("failed to serialize Stamp: %w", err)
	}

	if err := postcard.SerializeSeq(serializer, thv.Instructions, func(s *postcard.Serializer, inst PackedInstruction) error {
		return s.SerializeBytes(inst)
	}); err != nil {
		return fmt.Errorf("failed to serialize Instructions: %w", err)
	}

	if err := thv.Receipt.MarshalPostcard(serializer); err != nil {
		return fmt.Errorf("failed to serialize Receipt: %w", err)
	}

	return nil
}

func (trv *TxReceiptView) MarshalPostcard(serializer *postcard.Serializer) error {
	serializer.SerializeFixedBytes(trv.TxId[:])
	serializer.SerializeFixedBytes(trv.TxHash[:])

	if err := serializer.SerializeU8(trv.State); err != nil {
		return fmt.Errorf("failed to serialize State: %w", err)
	}

	if err := postcard.SerializeSeq(serializer, trv.AccessResourceChanges, func(s *postcard.Serializer, arc AccessResourceChange) error {
		s.SerializeFixedBytes(arc.ResourceHash[:])
		if err := arc.Change.MarshalPostcard(s); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to serialize AccessResourceChanges: %w", err)
	}

	if err := postcard.SerializeSeq(serializer, trv.Events, func(s *postcard.Serializer, ev TxReceiptViewEvent) error {
		return ev.MarshalPostcard(s)
	}); err != nil {
		return fmt.Errorf("failed to serialize Events: %w", err)
	}

	if err := postcard.SerializeOption(serializer, trv.Error, func(s *postcard.Serializer, errVal uint16) error {
		return s.SerializeU16(errVal)
	}); err != nil {
		return fmt.Errorf("failed to serialize Error: %w", err)
	}

	return nil
}

func (etd *TxReceiptViewEvent) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := serializer.SerializeU32(etd.EventIndex); err != nil {
		return fmt.Errorf("failed to serialize EventIndex: %w", err)
	}
	if err := serializer.SerializeU64(etd.TypeTag); err != nil {
		return fmt.Errorf("failed to serialize TypeTag: %w", err)
	}
	if err := serializer.SerializeBytes(etd.Data); err != nil {
		return fmt.Errorf("failed to serialize Data: %w", err)
	}
	return nil
}

func (ac *AccessChange) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := serializer.SerializeU32(uint32(ac.Variant)); err != nil {
		return fmt.Errorf("failed to serialize AccessChange variant: %w", err)
	}

	if ac.Variant == 0 {
		// Inline variant: { type_tag: u64, before: Option<Vec<u8>>, after: Vec<u8> }
		if err := serializer.SerializeU64(ac.TypeTag); err != nil {
			return fmt.Errorf("failed to serialize Inline type_tag: %w", err)
		}

		if err := postcard.SerializeOption(serializer, ac.Before, func(s *postcard.Serializer, value []byte) error {
			return s.SerializeBytes(value)
		}); err != nil {
			return fmt.Errorf("failed to serialize Inline before: %w", err)
		}

		if err := serializer.SerializeBytes(ac.After); err != nil {
			return fmt.Errorf("failed to serialize Inline after: %w", err)
		}

	} else if ac.Variant == 1 {
		// BlobHash variant: { before: Option<[u8; 32]>, after: [u8; 32] }
		if err := postcard.SerializeOption(serializer, ac.Before, func(s *postcard.Serializer, value []byte) error {
			s.SerializeFixedBytes(value)
			return nil
		}); err != nil {
			return fmt.Errorf("failed to serialize BlobHash before: %w", err)
		}

		serializer.SerializeFixedBytes(ac.After)

	} else {
		return fmt.Errorf("unknown AccessChange variant: %d", ac.Variant)
	}

	return nil
}

func (thv *TxHistoryView) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	stamp, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Stamp: %w", err)
	}
	thv.Stamp = stamp

	instructions, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (PackedInstruction, error) {
		wire, err := d.DeserializeBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize PackedInstruction: %w", err)
		}
		return wire, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Instructions: %w", err)
	}
	thv.Instructions = instructions
	if thv.Instructions == nil {
		thv.Instructions = []PackedInstruction{}
	}

	if err = thv.Receipt.UnmarshalPostcard(deserializer); err != nil {
		return fmt.Errorf("failed to deserialize Receipt: %w", err)
	}

	return nil
}

func (trv *TxReceiptView) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	txIdBytes, err := deserializer.DeserializeFixedBytes(TxIdLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize TxId: %w", err)
	}
	copy(trv.TxId[:], txIdBytes)

	txHashBytes, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize TxHash: %w", err)
	}
	copy(trv.TxHash[:], txHashBytes)

	state, err := deserializer.DeserializeU8()
	if err != nil {
		return fmt.Errorf("failed to deserialize State: %w", err)
	}
	trv.State = state

	// Parse AccessResourceChanges: Vec<(RsHash, AccessChange)>
	accessResources, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (AccessResourceChange, error) {
		var ar AccessResourceChange

		// Parse RsHash (18 bytes)
		hashBytes, err := d.DeserializeFixedBytes(RsHashLen)
		if err != nil {
			return ar, fmt.Errorf("failed to deserialize ResourceHash: %w", err)
		}
		copy(ar.ResourceHash[:], hashBytes)

		// Parse AccessChange enum
		if err = ar.Change.UnmarshalPostcard(d); err != nil {
			return ar, fmt.Errorf("failed to deserialize AccessChange: %w", err)
		}

		return ar, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize AccessResourceChanges: %w", err)
	}
	trv.AccessResourceChanges = accessResources

	events, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (TxReceiptViewEvent, error) {
		var event TxReceiptViewEvent
		if err = event.UnmarshalPostcard(d); err != nil {
			return event, err
		}
		return event, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Events: %w", err)
	}
	trv.Events = events

	errorVal, err := postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (uint16, error) {
		val, err := d.DeserializeU16()
		return val, err
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Error: %w", err)
	}
	trv.Error = errorVal

	return nil
}

func (ac *AccessChange) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	variant, err := deserializer.DeserializeU32()
	if err != nil {
		return fmt.Errorf("failed to deserialize AccessChange variant: %w", err)
	}

	ac.Variant = uint8(variant)

	if variant == 0 {
		// Inline variant: { type_tag: u64, before: Option<Vec<u8>>, after: Vec<u8> }
		typeTag, err := deserializer.DeserializeU64()
		if err != nil {
			return fmt.Errorf("failed to deserialize Inline type_tag: %w", err)
		}
		ac.TypeTag = typeTag

		// Parse before: Option<Vec<u8>>
		before, err := postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) ([]byte, error) {
			return d.DeserializeBytes()
		})
		if err != nil {
			return fmt.Errorf("failed to deserialize Inline before: %w", err)
		}
		ac.Before = before

		// Parse after: Vec<u8>
		after, err := deserializer.DeserializeBytes()
		if err != nil {
			return fmt.Errorf("failed to deserialize Inline after: %w", err)
		}
		ac.After = after

	} else if variant == 1 {
		// BlobHash variant: { before: Option<[u8; 32]>, after: [u8; 32] }

		// Parse before: Option<[u8; 32]>
		beforeOpt, err := postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) ([32]byte, error) {
			hashBytes, err := d.DeserializeFixedBytes(32)
			if err != nil {
				return [32]byte{}, err
			}
			var hash [32]byte
			copy(hash[:], hashBytes)
			return hash, nil
		})
		if err != nil {
			return fmt.Errorf("failed to deserialize BlobHash before: %w", err)
		}

		// Convert Option<[32]byte> to *[]byte
		if beforeOpt != nil {
			beforeBytes := beforeOpt[:]
			ac.Before = &beforeBytes
		} else {
			ac.Before = nil
		}

		// Parse after: [u8; 32]
		afterHash, err := deserializer.DeserializeFixedBytes(32)
		if err != nil {
			return fmt.Errorf("failed to deserialize BlobHash after: %w", err)
		}
		ac.After = afterHash

	} else {
		return fmt.Errorf("unknown AccessChange variant: %d", variant)
	}

	return nil
}

func (etd *TxReceiptViewEvent) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	eventIndex, err := deserializer.DeserializeU32()
	if err != nil {
		return fmt.Errorf("failed to deserialize EventIndex: %w", err)
	}
	etd.EventIndex = eventIndex

	typeTag, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize TypeTag: %w", err)
	}
	etd.TypeTag = typeTag

	data, err := deserializer.DeserializeBytes()
	if err != nil {
		return fmt.Errorf("failed to deserialize Data: %w", err)
	}
	etd.Data = data
	if etd.Data == nil {
		etd.Data = []byte{}
	}

	return nil
}
