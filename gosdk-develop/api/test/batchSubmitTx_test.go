package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchSubmitTxItem decodes one Result<[u8; 32], ApiFailure> item of a
// batch_submit_tx (6) response.
func decodeBatchSubmitTxItem(data []byte) (api.BatchSubmitTxItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchSubmitTxItem, error) {
		var item api.BatchSubmitTxItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

func TestBatchSubmitTxItem_WireFormat(t *testing.T) {
	t.Run("Ok variant appends the bare 32-byte hash", func(t *testing.T) {
		hash := api.TxHash{}
		for i := range hash {
			hash[i] = byte(i)
		}
		item := api.BatchSubmitTxItem{Value: &hash}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		// variant(0) + [u8;32] with no length prefix
		expected := append([]byte{0x00}, hash[:]...)
		assert.Equal(t, expected, data)

		decoded, err := decodeBatchSubmitTxItem(data)
		require.NoError(t, err)
		assert.Nil(t, decoded.Failure)
		require.NotNil(t, decoded.Value)
		assert.Equal(t, hash, *decoded.Value)
	})

	t.Run("Err variant carries ApiFailure", func(t *testing.T) {
		item := api.BatchSubmitTxItem{Failure: &api.ApiFailure{Message: "tx rejected"}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		assert.Equal(t, byte(0x01), data[0])

		decoded, err := decodeBatchSubmitTxItem(data)
		require.NoError(t, err)
		assert.Nil(t, decoded.Value)
		require.NotNil(t, decoded.Failure)
		assert.Equal(t, "tx rejected", decoded.Failure.Message)
	})

	t.Run("empty item cannot serialize", func(t *testing.T) {
		_, err := postcard.SerializePostcard(&api.BatchSubmitTxItem{})
		assert.ErrorContains(t, err, "neither Value nor Failure")
	})

	t.Run("unknown Result variant cannot deserialize", func(t *testing.T) {
		_, err := decodeBatchSubmitTxItem([]byte{0x02})
		assert.ErrorContains(t, err, "unknown Result variant 2")
	})
}
