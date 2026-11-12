package api

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	BatchGetAccessValue (method 166)
	rust (api/rpc/wire/src/method.rs, sdk/rpc-client/src/lib.rs)
	request  Vec<BlobHash>
	response Vec<Result<(BlobHash, Option<Vec<u8>>), ApiFailure>>
	Ok carries the (BlobHash, Option<Vec<u8>>) tuple inline, which is exactly
	what the single get_access_value method returns.
*/

// BatchGetAccessValueItem is one Result<(BlobHash, Option<Vec<u8>>), ApiFailure>
// entry of a batch_get_access_value response. Exactly one field is non-nil: an
// unknown blob is Ok with a nil Value.Data, not a failure.
type BatchGetAccessValueItem struct {
	Value   *GetAccessValueInfo
	Failure *ApiFailure
}

func (item *BatchGetAccessValueItem) MarshalPostcard(serializer *postcard.Serializer) error {
	if item.Value != nil {
		if err := serializer.SerializeEnumVariant(batchResultOk); err != nil {
			return fmt.Errorf("failed to serialize Ok variant: %w", err)
		}
		if err := item.Value.MarshalPostcard(serializer); err != nil {
			return fmt.Errorf("failed to serialize Ok value: %w", err)
		}
		return nil
	}

	if item.Failure == nil {
		return fmt.Errorf("BatchGetAccessValueItem has neither Value nor Failure")
	}
	if err := serializer.SerializeEnumVariant(batchResultErr); err != nil {
		return fmt.Errorf("failed to serialize Err variant: %w", err)
	}
	if err := item.Failure.MarshalPostcard(serializer); err != nil {
		return fmt.Errorf("failed to serialize Err failure: %w", err)
	}
	return nil
}

func (item *BatchGetAccessValueItem) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	variant, err := deserializer.DeserializeEnumVariant()
	if err != nil {
		return fmt.Errorf("failed to deserialize Result variant: %w", err)
	}

	switch variant {
	case batchResultOk:
		value := &GetAccessValueInfo{}
		if err = value.UnmarshalPostcard(deserializer); err != nil {
			return fmt.Errorf("failed to deserialize Ok value: %w", err)
		}
		item.Value = value
		return nil
	case batchResultErr:
		failure := &ApiFailure{}
		if err = failure.UnmarshalPostcard(deserializer); err != nil {
			return fmt.Errorf("failed to deserialize Err failure: %w", err)
		}
		item.Failure = failure
		return nil
	default:
		return fmt.Errorf("unknown Result variant %d", variant)
	}
}
