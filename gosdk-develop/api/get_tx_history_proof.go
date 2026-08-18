package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

type GetTxHistoryProof struct {
	Block    Block
	Index    uint32
	Siblings []TxHash
	//History  TxHistory
	History []byte
}

func (e *GetTxHistoryProof) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := e.Block.MarshalPostcard(serializer); err != nil {
		return fmt.Errorf("failed to serialize Block: %w", err)
	}
	if err := serializer.SerializeU32(e.Index); err != nil {
		return fmt.Errorf("failed to serialize Index: %w", err)
	}
	if err := postcard.SerializeSeq(serializer, e.Siblings, func(s *postcard.Serializer, sibling TxHash) error {
		serializer.SerializeFixedBytes(sibling[:])
		return nil
	}); err != nil {
		return fmt.Errorf("failed to serialize Siblings: %w", err)
	}

	//todo----
	//if err := e.History.MarshalPostcard(serializer); err != nil {
	//	return fmt.Errorf("failed to serialize History: %w", err)
	//}
	if err := serializer.SerializeBytes(e.History); err != nil {
		return fmt.Errorf("failed to serialize History: %w", err)
	}

	return nil
}

func (e *GetTxHistoryProof) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	// 1. Block
	var block Block
	if err := block.UnmarshalPostcard(deserializer); err != nil {
		return fmt.Errorf("failed to deserialize Block: %w", err)
	}
	e.Block = block

	// 2. Index (u32)
	index, err := deserializer.DeserializeU32()
	if err != nil {
		return fmt.Errorf("failed to deserialize Index: %w", err)
	}
	e.Index = index

	// 3. Siblings (Vec<TxHash>)
	siblings, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (TxHash, error) {
		var sibling TxHash
		siblingBytes, err := d.DeserializeFixedBytes(TxHashLen)
		if err != nil {
			return sibling, fmt.Errorf("failed to deserialize Sibling: %w", err)
		}
		copy(sibling[:], siblingBytes)
		return sibling, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Siblings: %w", err)
	}
	e.Siblings = siblings

	//todo----
	// 4. History (TxHistory)
	//var history TxHistory
	//if err = history.UnmarshalPostcard(deserializer); err != nil {
	//	return fmt.Errorf("failed to deserialize History: %w", err)
	//}
	//e.History = history

	history, err := deserializer.DeserializeBytes()
	if err != nil {
		return fmt.Errorf("failed to deserialize History: %w", err)
	}
	e.History = history

	return nil
}
