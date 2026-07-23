package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

type GetResource struct {
	Data TypeTagWithData // [type_tag + value]
}

func (gr *GetResource) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := serializer.SerializeU64(gr.Data.TypeTag); err != nil {
		return fmt.Errorf("failed to serialize TypeTag: %w", err)
	}

	serializer.SerializeFixedBytes(gr.Data.Value)

	return nil
}

func (gr *GetResource) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	typeTag, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize TypeTag: %w", err)
	}
	gr.Data.TypeTag = typeTag

	remaining := deserializer.Buffer()[deserializer.Offset():]
	gr.Data.Value = make([]byte, len(remaining))
	copy(gr.Data.Value, remaining)
	deserializer.Advance(len(remaining))

	return nil
}
