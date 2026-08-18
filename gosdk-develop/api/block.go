package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

type Block struct {
	Number    uint64
	Hash      TxHash
	PrevHash  TxHash
	StateHash TxHash
	TxRoot    TxHash
	TxCount   uint32
	Timestamp uint64
}

func (b *Block) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := serializer.SerializeU64(b.Number); err != nil {
		return fmt.Errorf("failed to serialize Number: %w", err)
	}

	serializer.SerializeFixedBytes(b.Hash[:])

	serializer.SerializeFixedBytes(b.PrevHash[:])

	serializer.SerializeFixedBytes(b.StateHash[:])

	serializer.SerializeFixedBytes(b.TxRoot[:])

	if err := serializer.SerializeU32(b.TxCount); err != nil {
		return fmt.Errorf("failed to serialize TxCount: %w", err)
	}

	if err := serializer.SerializeU64(b.Timestamp); err != nil {
		return fmt.Errorf("failed to serialize Timestamp: %w", err)
	}

	return nil
}

func (b *Block) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	number, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Number: %w", err)
	}
	b.Number = number

	hash, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize Hash: %w", err)
	}
	copy(b.Hash[:], hash)

	prevHash, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize PrevHash: %w", err)
	}
	copy(b.PrevHash[:], prevHash)

	stateHash, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize StateHash: %w", err)
	}
	copy(b.StateHash[:], stateHash)

	txRoot, err := deserializer.DeserializeFixedBytes(TxHashLen)
	if err != nil {
		return fmt.Errorf("failed to deserialize TxRoot: %w", err)
	}
	copy(b.TxRoot[:], txRoot)

	txCount, err := deserializer.DeserializeU32()
	if err != nil {
		return fmt.Errorf("failed to deserialize TxCount: %w", err)
	}
	b.TxCount = txCount

	timestamp, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Timestamp: %w", err)
	}
	b.Timestamp = timestamp

	return nil
}
