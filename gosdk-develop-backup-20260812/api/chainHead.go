package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

type ChainHead struct {
	ChainId        uint64
	BlockHeight    uint64
	BlockHash      TxHash
	TimestampMsecs uint64
}

func (ch *ChainHead) MarshalPostcard(serializer *postcard.Serializer) error {
	err := serializer.SerializeU64(ch.ChainId)
	if err != nil {
		return fmt.Errorf("failed to serialize ChainId: %w", err)
	}

	err = serializer.SerializeU64(ch.BlockHeight)
	if err != nil {
		return fmt.Errorf("failed to serialize BlockHeight: %w", err)
	}

	serializer.SerializeFixedBytes(ch.BlockHash[:])

	err = serializer.SerializeU64(ch.TimestampMsecs)
	if err != nil {
		return fmt.Errorf("failed to serialize TimestampMsecs: %w", err)
	}

	return nil
}

func (ch *ChainHead) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	var err error

	ch.ChainId, err = deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize ChainId: %w", err)
	}

	ch.BlockHeight, err = deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize BlockHeight: %w", err)
	}

	blockHash, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize BlockHash: %w", err)
	}
	for i := 0; i < len(blockHash); i++ {
		ch.BlockHash[i] = blockHash[i]
	}

	ch.TimestampMsecs, err = deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize TimestampMsecs: %w", err)
	}

	return nil
}
