package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchGetBlockByHeightItem decodes one Result<Vec<u8>, ApiFailure> item
// of a batch_get_block_by_height (51) response.
func decodeBatchGetBlockByHeightItem(data []byte) (api.BatchGetBlockByHeightItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchGetBlockByHeightItem, error) {
		var item api.BatchGetBlockByHeightItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

func TestBatchGetBlockByHeightItem_WireFormat(t *testing.T) {
	block := &api.Block{Number: 100, Epoch: 2, Slot: 3, TxCount: 5, Timestamp: 1_700_000_000}

	t.Run("Ok variant wraps a length-prefixed Block", func(t *testing.T) {
		item := api.BatchGetBlockByHeightItem{Block: block}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		inner, err := postcard.SerializePostcard(block)
		require.NoError(t, err)

		// variant(0) + Vec<u8>{len varint + inner}; len(inner) > 127 so the
		// length prefix is a two-byte varint, built via the serializer.
		lenPrefix := postcard.NewSerializer()
		require.NoError(t, lenPrefix.SerializeBytes(inner))
		expected := append([]byte{0x00}, lenPrefix.Bytes()...)
		assert.Equal(t, expected, data)

		decoded, err := decodeBatchGetBlockByHeightItem(data)
		require.NoError(t, err)
		require.NotNil(t, decoded.Block)
		assert.Equal(t, uint64(100), decoded.Block.Number)
		assert.Equal(t, uint64(2), decoded.Block.Epoch)
		assert.Equal(t, uint32(5), decoded.Block.TxCount)
		assert.Equal(t, uint64(1_700_000_000), decoded.Block.Timestamp)
	})

	t.Run("Err variant carries ApiFailure", func(t *testing.T) {
		item := api.BatchGetBlockByHeightItem{Failure: &api.ApiFailure{Message: "no"}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		assert.Equal(t, byte(0x01), data[0])
	})

	t.Run("empty item cannot serialize", func(t *testing.T) {
		_, err := postcard.SerializePostcard(&api.BatchGetBlockByHeightItem{})
		assert.ErrorContains(t, err, "neither Block nor Failure")
	})
}
