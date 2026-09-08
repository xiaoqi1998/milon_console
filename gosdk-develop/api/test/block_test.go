package test_test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestBlock_MarshalPostcard(t *testing.T) {
	t.Run("round trip with all fields populated", func(t *testing.T) {
		original := api.Block{
			Number:    1234567890,
			Epoch:     42,
			Hash:      api.TxHash{1, 2, 3, 4, 5},
			PrevHash:  api.TxHash{6, 7, 8, 9, 10},
			StateHash: api.TxHash{11, 12, 13, 14, 15},
			TxRoot:    api.TxHash{16, 17, 18, 19, 20},
			TxCount:   7,
			Timestamp: 9876543210,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.Block, error) {
			var block api.Block
			if err = block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original, deserialized)
	})

	t.Run("max uint64 values", func(t *testing.T) {
		original := api.Block{
			Number:    18446744073709551615,
			Epoch:     18446744073709551615,
			Hash:      api.TxHash{1, 2, 3, 4, 5},
			PrevHash:  api.TxHash{6, 7, 8, 9, 10},
			StateHash: api.TxHash{11, 12, 13, 14, 15},
			TxRoot:    api.TxHash{16, 17, 18, 19, 20},
			TxCount:   4294967295,
			Timestamp: 18446744073709551615,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.Block, error) {
			var block api.Block
			if err = block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original, deserialized)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		block1 := api.Block{
			Number:    999,
			Epoch:     1,
			Hash:      api.TxHash{1, 2, 3},
			PrevHash:  api.TxHash{4, 5, 6},
			StateHash: api.TxHash{7, 8, 9},
			TxRoot:    api.TxHash{10, 11, 12},
			TxCount:   3,
			Timestamp: 888,
		}

		block2 := api.Block{
			Number:    999,
			Epoch:     1,
			Hash:      api.TxHash{1, 2, 3},
			PrevHash:  api.TxHash{4, 5, 6},
			StateHash: api.TxHash{7, 8, 9},
			TxRoot:    api.TxHash{10, 11, 12},
			TxCount:   3,
			Timestamp: 888,
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
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (api.Block, error) {
			var block api.Block
			if err := block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only number", func(t *testing.T) {
		// number=1 → 0x01, missing Epoch and the rest
		_, err := postcard.DeserializePostcard([]byte{0x01}, func(d *postcard.Deserializer) (api.Block, error) {
			var block api.Block
			if err := block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := api.Block{
			Number:    1234567890,
			Epoch:     42,
			Hash:      api.TxHash{1, 2, 3},
			PrevHash:  api.TxHash{4, 5, 6},
			StateHash: api.TxHash{7, 8, 9},
			TxRoot:    api.TxHash{10, 11, 12},
			TxCount:   3,
			Timestamp: 9876543210,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.Block, error) {
			var block api.Block
			if err = block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := api.Block{
			Number:    1234567890,
			Epoch:     42,
			Hash:      api.TxHash{1, 2, 3},
			PrevHash:  api.TxHash{4, 5, 6},
			StateHash: api.TxHash{7, 8, 9},
			TxRoot:    api.TxHash{10, 11, 12},
			TxCount:   3,
			Timestamp: 9876543210,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.Block, error) {
			var block api.Block
			if err := block.UnmarshalPostcard(d); err != nil {
				return block, err
			}
			return block, nil
		}, true)
		assert.NoError(t, err)
	})
}
