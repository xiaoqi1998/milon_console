package api

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestChainHead_MarshalPostcard(t *testing.T) {
	t.Run("round trip with all fields populated", func(t *testing.T) {
		original := ChainHead{
			ChainId:        900000001,
			BlockHeight:    1234567890,
			BlockHash:      TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TimestampMsecs: 9876543210,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (ChainHead, error) {
			var ch ChainHead
			if err = ch.UnmarshalPostcard(d); err != nil {
				return ch, err
			}
			return ch, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.ChainId, deserialized.ChainId)
		assert.Equal(t, original.BlockHeight, deserialized.BlockHeight)
		assert.Equal(t, original.BlockHash, deserialized.BlockHash)
		assert.Equal(t, original.TimestampMsecs, deserialized.TimestampMsecs)
	})

	t.Run("round trip with zero values", func(t *testing.T) {
		original := ChainHead{
			ChainId:        0,
			BlockHeight:    0,
			BlockHash:      TxHash{},
			TimestampMsecs: 0,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (ChainHead, error) {
			var ch ChainHead
			if err = ch.UnmarshalPostcard(d); err != nil {
				return ch, err
			}
			return ch, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.ChainId, deserialized.ChainId)
		assert.Equal(t, original.BlockHeight, deserialized.BlockHeight)
		assert.Equal(t, original.BlockHash, deserialized.BlockHash)
		assert.Equal(t, original.TimestampMsecs, deserialized.TimestampMsecs)
	})

	t.Run("round trip with max uint64 values", func(t *testing.T) {
		original := ChainHead{
			ChainId:        18446744073709551615,
			BlockHeight:    18446744073709551615,
			BlockHash:      TxHash{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			TimestampMsecs: 18446744073709551615,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (ChainHead, error) {
			var ch ChainHead
			if err = ch.UnmarshalPostcard(d); err != nil {
				return ch, err
			}
			return ch, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.ChainId, deserialized.ChainId)
		assert.Equal(t, original.BlockHeight, deserialized.BlockHeight)
		assert.Equal(t, original.BlockHash, deserialized.BlockHash)
		assert.Equal(t, original.TimestampMsecs, deserialized.TimestampMsecs)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		ch1 := ChainHead{
			ChainId:        100,
			BlockHeight:    200,
			BlockHash:      TxHash{0xAA, 0xBB, 0xCC},
			TimestampMsecs: 300,
		}

		ch2 := ChainHead{
			ChainId:        100,
			BlockHeight:    200,
			BlockHash:      TxHash{0xAA, 0xBB, 0xCC},
			TimestampMsecs: 300,
		}

		data1, err := postcard.SerializePostcard(&ch1)
		assert.NoError(t, err)

		data2, err := postcard.SerializePostcard(&ch2)
		assert.NoError(t, err)

		assert.Equal(t, data1, data2)
	})
}

func TestChainHead_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (ChainHead, error) {
			var ch ChainHead
			if err := ch.UnmarshalPostcard(d); err != nil {
				return ch, err
			}
			return ch, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only chain id", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{0x01}, func(d *postcard.Deserializer) (ChainHead, error) {
			var ch ChainHead
			if err := ch.UnmarshalPostcard(d); err != nil {
				return ch, err
			}
			return ch, nil
		}, false)
		assert.Error(t, err)

	})

	t.Run("truncated data - missing block hash", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{0x01, 0x01}, func(d *postcard.Deserializer) (ChainHead, error) {
			var ch ChainHead
			if err := ch.UnmarshalPostcard(d); err != nil {
				return ch, err
			}
			return ch, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := ChainHead{
			ChainId:        1,
			BlockHeight:    2,
			BlockHash:      TxHash{3, 4, 5},
			TimestampMsecs: 6,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (ChainHead, error) {
			var ch ChainHead
			if err = ch.UnmarshalPostcard(d); err != nil {
				return ch, err
			}
			return ch, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := ChainHead{
			ChainId:        1,
			BlockHeight:    2,
			BlockHash:      TxHash{3, 4, 5},
			TimestampMsecs: 6,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (ChainHead, error) {
			var ch ChainHead
			if err = ch.UnmarshalPostcard(d); err != nil {
				return ch, err
			}
			return ch, nil
		}, true)
		assert.NoError(t, err)
	})
}
