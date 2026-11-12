package api

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	BatchGetTxByHash (method 56)
	rust (api/rpc/server/src/dispatch.rs::batch_map)
	Vec<Vec<u8>> -> Vec<Result<TxHistory, ApiFailure>>
	Every request element is a tx hash or tx id byte string; Ok carries the
	TxHistory struct inline (no byte blob wrapper).
*/

type BatchGetTxByHashItem struct {
	Value   *TxHistory
	Failure *ApiFailure
}

func (item *BatchGetTxByHashItem) MarshalPostcard(s *postcard.Serializer) error {
	if item.Value != nil {
		if err := s.SerializeEnumVariant(batchResultOk); err != nil {
			return fmt.Errorf("failed to serialize Ok variant: %w", err)
		}
		if err := item.Value.MarshalPostcard(s); err != nil {
			return fmt.Errorf("failed to serialize Ok value: %w", err)
		}
		return nil
	}
	if item.Failure == nil {
		return fmt.Errorf("BatchGetTxByHashItem has neither Value nor Failure")
	}
	if err := s.SerializeEnumVariant(batchResultErr); err != nil {
		return fmt.Errorf("failed to serialize Err variant: %w", err)
	}
	if err := item.Failure.MarshalPostcard(s); err != nil {
		return fmt.Errorf("failed to serialize Err failure: %w", err)
	}
	return nil
}

func (item *BatchGetTxByHashItem) UnmarshalPostcard(d *postcard.Deserializer) error {
	variant, err := d.DeserializeEnumVariant()
	if err != nil {
		return fmt.Errorf("failed to deserialize Result variant: %w", err)
	}
	switch variant {
	case batchResultOk:
		value := &TxHistory{}
		if err = value.UnmarshalPostcard(d); err != nil {
			return fmt.Errorf("failed to deserialize Ok value: %w", err)
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
