package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

type EventsByTxHashReq struct {
	TxHash        TxHash
	TypeTagFilter *uint64
}

func (e *EventsByTxHashReq) MarshalPostcard(serializer *postcard.Serializer) error {
	serializer.SerializeFixedBytes(e.TxHash[:])

	if err := postcard.SerializeOption(serializer, e.TypeTagFilter, func(s *postcard.Serializer, filter uint64) error {
		return s.SerializeU64(filter)
	}); err != nil {
		return fmt.Errorf("failed to serialize TypeTagFilter: %w", err)
	}

	return nil
}

func (e *EventsByTxHashReq) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	txHash, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize TxHash: %w", err)
	}
	copy(e.TxHash[:], txHash)

	e.TypeTagFilter, err = postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (uint64, error) {
		return d.DeserializeU64()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize TypeTagFilter: %w", err)
	}

	return nil
}

type EventsByTxHash struct {
	Events []EventEntry
}

type EventEntry struct {
	BlockHeight uint64
	TxHash      TxHash
	TxIndex     uint32
	EventIndex  uint32
	Data        TypeTagWithData
}

func (r *EventsByTxHash) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := postcard.SerializeSeq(serializer, r.Events, func(s *postcard.Serializer, entry EventEntry) error {
		return entry.MarshalPostcard(s)
	}); err != nil {
		return fmt.Errorf("failed to serialize Events: %w", err)
	}
	return nil
}

func (r *EventsByTxHash) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	var err error
	r.Events, err = postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (EventEntry, error) {
		var entry EventEntry
		if err = entry.UnmarshalPostcard(d); err != nil {
			return entry, err
		}
		return entry, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Events: %w", err)
	}

	return nil
}

func (e *EventEntry) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := serializer.SerializeU64(e.BlockHeight); err != nil {
		return fmt.Errorf("failed to serialize BlockHeight: %w", err)
	}

	serializer.SerializeFixedBytes(e.TxHash[:])

	if err := serializer.SerializeU32(e.TxIndex); err != nil {
		return fmt.Errorf("failed to serialize TxIndex: %w", err)
	}

	if err := serializer.SerializeU32(e.EventIndex); err != nil {
		return fmt.Errorf("failed to serialize EventIndex: %w", err)
	}

	// serialize Data (type_tag + value)
	dataBytes := make([]byte, 0)
	typeTagSerialize := postcard.NewSerializer()
	err := typeTagSerialize.SerializeU64(e.Data.TypeTag)
	if err != nil {
		return fmt.Errorf("failed to serialize TypeTag: %w", err)
	}
	dataBytes = append(dataBytes, typeTagSerialize.Bytes()...)
	dataBytes = append(dataBytes, e.Data.Value...)

	if err = serializer.SerializeBytes(dataBytes); err != nil {
		return fmt.Errorf("failed to serialize Data: %w", err)
	}

	return nil
}

func (e *EventEntry) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	var err error

	e.BlockHeight, err = deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize BlockHeight: %w", err)
	}

	txHash, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize TxHash: %w", err)
	}
	copy(e.TxHash[:], txHash)

	e.TxIndex, err = deserializer.DeserializeU32()
	if err != nil {
		return fmt.Errorf("failed to deserialize TxIndex: %w", err)
	}

	e.EventIndex, err = deserializer.DeserializeU32()
	if err != nil {
		return fmt.Errorf("failed to deserialize EventIndex: %w", err)
	}

	// read the full Data (type_tag + value)
	rawData, err := deserializer.DeserializeBytes()
	if err != nil {
		return fmt.Errorf("failed to deserialize Data: %w", err)
	}
	if rawData == nil {
		rawData = []byte{}
	}

	// parse TypeTag and Value from Data
	if len(rawData) > 0 {
		typeTagReader := postcard.NewDeserializer(rawData)
		e.Data.TypeTag, err = typeTagReader.DeserializeU64()
		if err != nil {
			return fmt.Errorf("failed to parse TypeTag from Data: %w", err)
		}

		// Value is everything after type_tag
		e.Data.Value = rawData[typeTagReader.Offset():]
	} else {
		e.Data.Value = []byte{}
	}

	return nil
}
