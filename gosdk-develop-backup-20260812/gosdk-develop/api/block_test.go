package api

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestBlock_MarshalPostcard(t *testing.T) {
	t.Run("round trip with all fields populated", func(t *testing.T) {
		var witnessSig [crypto.SignatureFnDsa512Size]byte
		for i := 0; i < len(witnessSig); i++ {
			witnessSig[i] = byte(i % 256)
		}

		original := Block{
			Number:    1234567890,
			Hash:      TxHash{1, 2, 3, 4, 5},
			PrevHash:  TxHash{6, 7, 8, 9, 10},
			Timestamp: 9876543210,
			TxProofIdentifiers: []TxProofIdentifier{
				{11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22},
				{23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34},
			},
			WitnessAddress:   crypto.Address{Bytes: [20]byte{1, 2, 3}},
			WitnessSignature: witnessSig,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (Block, error) {
			var block Block
			if err = block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.Number, deserialized.Number)
		assert.Equal(t, original.Hash, deserialized.Hash)
		assert.Equal(t, original.PrevHash, deserialized.PrevHash)
		assert.Equal(t, original.Timestamp, deserialized.Timestamp)
		assert.Equal(t, original.TxProofIdentifiers, deserialized.TxProofIdentifiers)
		assert.Equal(t, original.WitnessAddress.Bytes, deserialized.WitnessAddress.Bytes)
		assert.Equal(t, original.WitnessSignature, deserialized.WitnessSignature)
	})

	t.Run("round trip with empty TxProofIdentifiers", func(t *testing.T) {
		var witnessSig [crypto.SignatureFnDsa512Size]byte
		for i := 0; i < len(witnessSig); i++ {
			witnessSig[i] = byte(i % 256)
		}

		original := Block{
			Number:             1234567890,
			Hash:               TxHash{1, 2, 3, 4, 5},
			PrevHash:           TxHash{6, 7, 8, 9, 10},
			Timestamp:          9876543210,
			TxProofIdentifiers: []TxProofIdentifier{},
			WitnessAddress:     crypto.Address{Bytes: [20]byte{1, 2, 3}},
			WitnessSignature:   witnessSig,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (Block, error) {
			var block Block
			if err = block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.Number, deserialized.Number)
		assert.Equal(t, original.Hash, deserialized.Hash)
		assert.Equal(t, original.PrevHash, deserialized.PrevHash)
		assert.Equal(t, original.Timestamp, deserialized.Timestamp)
		assert.Equal(t, original.TxProofIdentifiers, deserialized.TxProofIdentifiers)
		assert.Equal(t, original.WitnessAddress.Bytes, deserialized.WitnessAddress.Bytes)
		assert.Equal(t, original.WitnessSignature, deserialized.WitnessSignature)
	})

	t.Run("round trip with single TxProofIdentifier", func(t *testing.T) {
		var witnessSig [crypto.SignatureFnDsa512Size]byte
		for i := 0; i < len(witnessSig); i++ {
			witnessSig[i] = byte(i % 256)
		}

		original := Block{
			Number:    1234567890,
			Hash:      TxHash{1, 2, 3, 4, 5},
			PrevHash:  TxHash{6, 7, 8, 9, 10},
			Timestamp: 9876543210,
			TxProofIdentifiers: []TxProofIdentifier{
				{11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22},
			},
			WitnessAddress:   crypto.Address{Bytes: [20]byte{1, 2, 3}},
			WitnessSignature: witnessSig,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (Block, error) {
			var block Block
			if err = block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.Number, deserialized.Number)
		assert.Equal(t, original.Hash, deserialized.Hash)
		assert.Equal(t, original.PrevHash, deserialized.PrevHash)
		assert.Equal(t, original.Timestamp, deserialized.Timestamp)
		assert.Equal(t, original.TxProofIdentifiers, deserialized.TxProofIdentifiers)
		assert.Equal(t, original.WitnessAddress.Bytes, deserialized.WitnessAddress.Bytes)
		assert.Equal(t, original.WitnessSignature, deserialized.WitnessSignature)
	})

	t.Run("max uint64 values", func(t *testing.T) {
		var witnessSig [crypto.SignatureFnDsa512Size]byte
		for i := 0; i < len(witnessSig); i++ {
			witnessSig[i] = byte(i % 256)
		}

		original := Block{
			Number:    18446744073709551615,
			Hash:      TxHash{1, 2, 3, 4, 5},
			PrevHash:  TxHash{6, 7, 8, 9, 10},
			Timestamp: 18446744073709551615,
			TxProofIdentifiers: []TxProofIdentifier{
				{11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22},
			},
			WitnessAddress:   crypto.Address{Bytes: [20]byte{1, 2, 3}},
			WitnessSignature: witnessSig,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (Block, error) {
			var block Block
			if err = block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.Number, deserialized.Number)
		assert.Equal(t, original.Hash, deserialized.Hash)
		assert.Equal(t, original.PrevHash, deserialized.PrevHash)
		assert.Equal(t, original.Timestamp, deserialized.Timestamp)
		assert.Equal(t, original.TxProofIdentifiers, deserialized.TxProofIdentifiers)
		assert.Equal(t, original.WitnessAddress.Bytes, deserialized.WitnessAddress.Bytes)
		assert.Equal(t, original.WitnessSignature, deserialized.WitnessSignature)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		var witnessSig [crypto.SignatureFnDsa512Size]byte
		for i := 0; i < len(witnessSig); i++ {
			witnessSig[i] = byte(i % 256)
		}

		block1 := Block{
			Number:    999,
			Hash:      TxHash{1, 2, 3},
			PrevHash:  TxHash{4, 5, 6},
			Timestamp: 888,
			TxProofIdentifiers: []TxProofIdentifier{
				{7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18},
			},
			WitnessAddress:   crypto.Address{Bytes: [20]byte{19, 20, 21}},
			WitnessSignature: witnessSig,
		}

		block2 := Block{
			Number:    999,
			Hash:      TxHash{1, 2, 3},
			PrevHash:  TxHash{4, 5, 6},
			Timestamp: 888,
			TxProofIdentifiers: []TxProofIdentifier{
				{7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18},
			},
			WitnessAddress:   crypto.Address{Bytes: [20]byte{19, 20, 21}},
			WitnessSignature: witnessSig,
		}

		data1, err := postcard.SerializePostcard(&block1)
		assert.NoError(t, err)

		data2, err := postcard.SerializePostcard(&block2)
		assert.NoError(t, err)

		assert.Equal(t, data1, data2)
	})

}

func TestBlock_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (Block, error) {
			var block Block
			if err := block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only number", func(t *testing.T) {
		// number=1 → 0x01
		_, err := postcard.DeserializePostcard([]byte{0x01}, func(d *postcard.Deserializer) (Block, error) {
			var block Block
			if err := block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		var witnessSig [crypto.SignatureFnDsa512Size]byte
		for i := 0; i < len(witnessSig); i++ {
			witnessSig[i] = byte(i % 256)
		}

		original := Block{
			Number:             1234567890,
			Hash:               TxHash{1, 2, 3},
			PrevHash:           TxHash{4, 5, 6},
			Timestamp:          9876543210,
			TxProofIdentifiers: []TxProofIdentifier{},
			WitnessAddress:     crypto.Address{Bytes: [20]byte{1, 2, 3}},
			WitnessSignature:   witnessSig,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (Block, error) {
			var block Block
			if err = block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		var witnessSig [crypto.SignatureFnDsa512Size]byte
		for i := 0; i < len(witnessSig); i++ {
			witnessSig[i] = byte(i % 256)
		}

		original := Block{
			Number:             1234567890,
			Hash:               TxHash{1, 2, 3},
			PrevHash:           TxHash{4, 5, 6},
			Timestamp:          9876543210,
			TxProofIdentifiers: []TxProofIdentifier{},
			WitnessAddress:     crypto.Address{Bytes: [20]byte{1, 2, 3}},
			WitnessSignature:   witnessSig,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (Block, error) {
			var block Block
			if err := block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, true)
		assert.NoError(t, err)
	})
}
