package api

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	BatchView (method 16)
	rust (api/rpc/server/src/dispatch.rs::batch_map)
	Vec<PackedInstruction> -> Vec<Result<Vec<u8>, ApiFailure>>
	Ok carries the view return value bytes, length-prefixed; those are exactly
	the whole body the single view method (15) returns. Data is a non-nil slice
	for an Ok item even when the view returns nothing (empty Vec<u8>).
*/

type BatchViewItem struct {
	Data    []byte
	Failure *ApiFailure
}

func (item *BatchViewItem) MarshalPostcard(s *postcard.Serializer) error {
	if item.Data != nil {
		if err := s.SerializeEnumVariant(batchResultOk); err != nil {
			return fmt.Errorf("failed to serialize Ok variant: %w", err)
		}
		if err := s.SerializeBytes(item.Data); err != nil {
			return fmt.Errorf("failed to serialize Ok value: %w", err)
		}
		return nil
	}
	if item.Failure == nil {
		return fmt.Errorf("BatchViewItem has neither Data nor Failure")
	}
	if err := s.SerializeEnumVariant(batchResultErr); err != nil {
		return fmt.Errorf("failed to serialize Err variant: %w", err)
	}
	if err := item.Failure.MarshalPostcard(s); err != nil {
		return fmt.Errorf("failed to serialize Err failure: %w", err)
	}
	return nil
}

func (item *BatchViewItem) UnmarshalPostcard(d *postcard.Deserializer) error {
	variant, err := d.DeserializeEnumVariant()
	if err != nil {
		return fmt.Errorf("failed to deserialize Result variant: %w", err)
	}
	switch variant {
	case batchResultOk:
		raw, err := d.DeserializeBytes()
		if err != nil {
			return fmt.Errorf("failed to deserialize Ok value: %w", err)
		}
		item.Data = raw
		return nil
	case batchResultErr:
		failure := &ApiFailure{}
		if err = failure.UnmarshalPostcard(d); err != nil {
			return fmt.Errorf("failed to deserialize Err failure: %w", err)
		}
		item.Failure = failure
		return nil
	default:
		return fmt.Errorf("unknown Result variant %d", variant)
	}
}
