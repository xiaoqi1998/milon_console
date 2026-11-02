package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	rust (api/rpc/wire/src/method.rs, sdk/rpc-client/src/lib.rs)
	// get_access_value (165):  request  BlobHash
	//                           response (BlobHash, Option<Vec<u8>>)
	// batch_get_access_value (166): request  Vec<BlobHash>
	//                               response Vec<Result<(BlobHash, Option<Vec<u8>>), ApiFailure>>
	// The Vec<u8> payload is postcard(dyn AnySerialize), i.e. (type_tag, value).
*/

// GetAccessValueInfo is the (BlobHash, Option<Vec<u8>>) tuple returned by
// get_access_value. Data is nil when the node has no blob for the hash.
type GetAccessValueInfo struct {
	BlobHash BlobHash
	Data     *TypeTagWithData
}

func (av *GetAccessValueInfo) MarshalPostcard(serializer *postcard.Serializer) error {
	serializer.SerializeFixedBytes(av.BlobHash[:])

	if av.Data != nil {
		if err := serializer.SerializeBool(true); err != nil {
			return fmt.Errorf("failed to serialize Data presence: %w", err)
		}

		// Wrap [typeTag + value] in Vec<u8>
		innerSerializer := postcard.NewSerializer()
		if err := innerSerializer.SerializeU64(av.Data.TypeTag); err != nil {
			return fmt.Errorf("failed to serialize Data TypeTag: %w", err)
		}
		innerSerializer.SerializeFixedBytes(av.Data.Value)

		if err := serializer.SerializeBytes(innerSerializer.Bytes()); err != nil {
			return fmt.Errorf("failed to serialize Data Vec<u8>: %w", err)
		}
	} else {
		if err := serializer.SerializeBool(false); err != nil {
			return fmt.Errorf("failed to serialize Data presence: %w", err)
		}
	}

	return nil
}

func (av *GetAccessValueInfo) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	blobHashBytes, err := deserializer.DeserializeFixedBytes(BlobHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize BlobHash: %w", err)
	}
	copy(av.BlobHash[:], blobHashBytes)

	hasData, err := deserializer.DeserializeBool()
	if err != nil {
		return fmt.Errorf("failed to deserialize Data presence: %w", err)
	}

	if hasData {
		// Read Vec<u8>: [length varint] + [data]
		rawData, err := deserializer.DeserializeBytes()
		if err != nil {
			return fmt.Errorf("failed to deserialize Data Vec<u8>: %w", err)
		}

		// Vec<u8> content is [typeTag varint + value]
		typeTagReader := postcard.NewDeserializer(rawData)
		typeTag, err := typeTagReader.DeserializeU64()
		if err != nil {
			return fmt.Errorf("failed to deserialize Data TypeTag from Vec<u8>: %w", err)
		}

		// Value is everything after type_tag
		valueBytes := make([]byte, typeTagReader.Remaining())
		copy(valueBytes, typeTagReader.Buffer()[typeTagReader.Offset():])

		av.Data = &TypeTagWithData{
			TypeTag: typeTag,
			Value:   valueBytes,
		}
	}

	return nil
}

// Result variant indexes of the Rust enum the node encodes for each batch item.
const (
	batchGetAccessValueOk  uint32 = 0
	batchGetAccessValueErr uint32 = 1
)

// BatchGetAccessValueItem is one Result<(BlobHash, Option<Vec<u8>>), ApiFailure>
// entry of a batch_get_access_value response. Exactly one field is non-nil: an
// unknown blob is Ok with a nil Value.Data, not a failure.
type BatchGetAccessValueItem struct {
	Value   *GetAccessValueInfo
	Failure *ApiFailure
}

func (item *BatchGetAccessValueItem) MarshalPostcard(serializer *postcard.Serializer) error {
	if item.Value != nil {
		if err := serializer.SerializeEnumVariant(batchGetAccessValueOk); err != nil {
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
	if err := serializer.SerializeEnumVariant(batchGetAccessValueErr); err != nil {
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
	case batchGetAccessValueOk:
		value := &GetAccessValueInfo{}
		if err = value.UnmarshalPostcard(deserializer); err != nil {
			return fmt.Errorf("failed to deserialize Ok value: %w", err)
		}
		item.Value = value
		return nil
	case batchGetAccessValueErr:
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
