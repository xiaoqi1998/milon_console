package api

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

type Block struct {
	Number             uint64
	Hash               TxHash
	PrevHash           TxHash
	Timestamp          uint64
	TxProofIdentifiers []TxProofIdentifier
	WitnessAddress     crypto.Address
	WitnessSignature   [crypto.SignatureFnDsa512Size]byte
}

func (b *Block) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := serializer.SerializeU64(b.Number); err != nil {
		return fmt.Errorf("failed to serialize Number: %w", err)
	}

	serializer.SerializeFixedBytes(b.Hash[:])

	serializer.SerializeFixedBytes(b.PrevHash[:])

	if err := serializer.SerializeU64(b.Timestamp); err != nil {
		return fmt.Errorf("failed to serialize Timestamp: %w", err)
	}

	if err := postcard.SerializeSeq(serializer, b.TxProofIdentifiers, func(s *postcard.Serializer, id TxProofIdentifier) error {
		s.SerializeFixedBytes(id[:])
		return nil
	}); err != nil {
		return fmt.Errorf("failed to serialize TxProofIdentifiers: %w", err)
	}

	if err := b.WitnessAddress.MarshalPostcard(serializer); err != nil {
		return fmt.Errorf("failed to serialize WitnessAddress: %w", err)
	}

	serializer.SerializeFixedBytes(b.WitnessSignature[:])

	return nil
}

func (b *Block) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	var err error

	b.Number, err = deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Number: %w", err)
	}

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

	b.Timestamp, err = deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Timestamp: %w", err)
	}

	b.TxProofIdentifiers, err = postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (TxProofIdentifier, error) {
		buf, err := d.DeserializeFixedBytes(TxProofIdentifierLen)
		if err != nil {
			return TxProofIdentifier{}, fmt.Errorf("failed to deserialize TxProofIdentifier element: %w", err)
		}
		var id TxProofIdentifier
		copy(id[:], buf)
		return id, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize TxProofIdentifiers: %w", err)
	}

	if err = b.WitnessAddress.UnmarshalPostcard(deserializer); err != nil {
		return fmt.Errorf("failed to deserialize WitnessAddress: %w", err)
	}

	witnessSig, err := deserializer.DeserializeFixedBytes(crypto.SignatureFnDsa512Size)
	if err != nil {
		return fmt.Errorf("failed to deserialize WitnessSignature: %w", err)
	}
	copy(b.WitnessSignature[:], witnessSig)

	return nil
}
