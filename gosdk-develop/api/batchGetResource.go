package api

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	BatchGetResource (method 151)
	rust (api/rpc/server/src/dispatch.rs::batch_map)
	Vec<RsHash> -> Vec<Result<Vec<u8>, ApiFailure>>
	handlers::get_resource_raw returns Option<Vec<u8>>, so the Ok payload is a
	length-prefixed byte blob that itself decodes into a GetResource, matching
	the body the single get_resource method returns.
*/

type BatchGetResourceItem struct {
	Value   *GetResource
	Failure *ApiFailure
}

func (item *BatchGetResourceItem) MarshalPostcard(s *postcard.Serializer) error {
	if item.Value != nil {
		inner, err := postcard.SerializePostcard(item.Value)
		if err != nil {
			return fmt.Errorf("failed to serialize GetResource bytes: %w", err)
		}
		if err := s.SerializeEnumVariant(batchResultOk); err != nil {
			return fmt.Errorf("failed to serialize Ok variant: %w", err)
		}
		if err := s.SerializeBytes(inner); err != nil {
			return fmt.Errorf("failed to serialize Vec<u8> Ok payload: %w", err)
		}
		return nil
	}
	if item.Failure == nil {
		return fmt.Errorf("BatchGetResourceItem has neither Value nor Failure")
	}
	if err := s.SerializeEnumVariant(batchResultErr); err != nil {
		return fmt.Errorf("failed to serialize Err variant: %w", err)
	}
	if err := item.Failure.MarshalPostcard(s); err != nil {
		return fmt.Errorf("failed to serialize Err failure: %w", err)
	}
	return nil
}

func (item *BatchGetResourceItem) UnmarshalPostcard(d *postcard.Deserializer) error {
	variant, err := d.DeserializeEnumVariant()
	if err != nil {
		return fmt.Errorf("failed to deserialize Result variant: %w", err)
	}
	switch variant {
	case batchResultOk:
		raw, err := d.DeserializeBytes()
		if err != nil {
			return fmt.Errorf("failed to deserialize Vec<u8> Ok payload: %w", err)
		}
		res := &GetResource{}
		if err = res.UnmarshalPostcard(postcard.NewDeserializer(raw)); err != nil {
			return fmt.Errorf("failed to deserialize inner GetResource: %w", err)
		}
		item.Value = res
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
