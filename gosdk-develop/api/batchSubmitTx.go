package api

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	BatchSubmitTx (method 6)
	rust (api/rpc/server/src/dispatch.rs::batch_map)
	Vec<Transaction> -> Vec<Result<[u8; 32], ApiFailure>>
	Ok is the accepted tx hash, encoded as [u8; 32] bare bytes (no length
	prefix), matching the body the single submit_tx method returns.
*/

type BatchSubmitTxItem struct {
	Value   *TxHash
	Failure *ApiFailure
}

func (item *BatchSubmitTxItem) MarshalPostcard(s *postcard.Serializer) error {
	if item.Value != nil {
		if err := s.SerializeEnumVariant(batchResultOk); err != nil {
			return fmt.Errorf("failed to serialize Ok variant: %w", err)
		}
		s.SerializeFixedBytes(item.Value[:])
		return nil
	}
	if item.Failure == nil {
		return fmt.Errorf("BatchSubmitTxItem has neither Value nor Failure")
	}
	if err := s.SerializeEnumVariant(batchResultErr); err != nil {
		return fmt.Errorf("failed to serialize Err variant: %w", err)
	}
	if err := item.Failure.MarshalPostcard(s); err != nil {
		return fmt.Errorf("failed to serialize Err failure: %w", err)
	}
	return nil
}

func (item *BatchSubmitTxItem) UnmarshalPostcard(d *postcard.Deserializer) error {
	variant, err := d.DeserializeEnumVariant()
	if err != nil {
		return fmt.Errorf("failed to deserialize Result variant: %w", err)
	}
	switch variant {
	case batchResultOk:
		// [u8; 32] is encoded as 32 bare bytes, no length prefix.
		hashBytes, err := d.DeserializeFixedBytes(TxHashLen)
		if err != nil {
			return fmt.Errorf("failed to deserialize Ok tx hash: %w", err)
		}
		value := &TxHash{}
		copy(value[:], hashBytes)
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
