package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	rust (api/rpc/wire/src/method.rs, sdk/rpc-client/src/lib.rs)
	// get_access_value (165):  request  BlobHash
	//                           response (BlobHash, Option<Vec<u8>>)
	// The Vec<u8> payload is postcard(dyn AnySerialize), i.e. (type_tag, value).
	// batch_get_access_value (166) reuses this type as its Ok payload; see
	// batchGetAccessValue.go.
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
