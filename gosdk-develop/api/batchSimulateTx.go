package api

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	BatchSimulateTx (method 11)
	rust (api/rpc/server/src/dispatch.rs::batch_map)
	Vec<Transaction> -> Vec<Result<Vec<u8>, ApiFailure>>
	The Ok payload is a length-prefixed byte blob holding the framed
	SimulateReceipt, i.e. exactly the body the single simulate_tx method
	returns.
*/

type BatchSimulateTxItem struct {
	Value   *SimulateReceipt
	Failure *ApiFailure
}

func (item *BatchSimulateTxItem) MarshalPostcard(s *postcard.Serializer) error {
	if item.Value != nil {
		inner, err := postcard.SerializePostcard(item.Value)
		if err != nil {
			return fmt.Errorf("failed to serialize SimulateReceipt bytes: %w", err)
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
		return fmt.Errorf("BatchSimulateTxItem has neither Value nor Failure")
	}
	if err := s.SerializeEnumVariant(batchResultErr); err != nil {
		return fmt.Errorf("failed to serialize Err variant: %w", err)
	}
	if err := item.Failure.MarshalPostcard(s); err != nil {
		return fmt.Errorf("failed to serialize Err failure: %w", err)
	}
	return nil
}

func (item *BatchSimulateTxItem) UnmarshalPostcard(d *postcard.Deserializer) error {
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
		// The inner receipt carries type_tag values, so the outer TypeResolver
		// has to be passed down.
		inner := postcard.NewDeserializer(raw)
		inner.SetTypeResolver(d.TypeResolver())
		value := &SimulateReceipt{}
		if err = value.UnmarshalPostcard(inner); err != nil {
			return fmt.Errorf("failed to deserialize inner SimulateReceipt: %w", err)
		}
		item.Value = value
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
