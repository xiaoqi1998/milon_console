package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchEventsByTxHashItem decodes one
// Result<EventsByTxHashResponse, ApiFailure> item of a
// batch_events_by_tx_hash (26) response.
func decodeBatchEventsByTxHashItem(data []byte) (api.BatchEventsByTxHashItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchEventsByTxHashItem, error) {
		var item api.BatchEventsByTxHashItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

func TestBatchEventsByTxHashItem_WireFormat(t *testing.T) {
	t.Run("Ok variant prefixes the EventsByTxHash", func(t *testing.T) {
		var hash api.TxHash
		hash[0] = 0xAA
		item := api.BatchEventsByTxHashItem{Value: &api.EventsByTxHash{Events: []api.EventEntry{
			{BlockHeight: 1, TxHash: hash, TxIndex: 0, EventIndex: 0, Data: api.TypeTagWithData{TypeTag: 3, Value: []byte{0x11}}},
		}}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		tuple, err := postcard.SerializePostcard(item.Value)
		require.NoError(t, err)
		assert.Equal(t, append([]byte{0x00}, tuple...), data)

		decoded, err := decodeBatchEventsByTxHashItem(data)
		require.NoError(t, err)
		require.Len(t, decoded.Value.Events, 1)
		assert.Equal(t, hash, decoded.Value.Events[0].TxHash)
		assert.Equal(t, uint64(3), decoded.Value.Events[0].Data.TypeTag)
		assert.Equal(t, []byte{0x11}, decoded.Value.Events[0].Data.Value)
	})

	t.Run("Err variant carries ApiFailure", func(t *testing.T) {
		item := api.BatchEventsByTxHashItem{Failure: &api.ApiFailure{Message: "gone"}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		assert.Equal(t, byte(0x01), data[0])
	})

	t.Run("empty item cannot serialize", func(t *testing.T) {
		_, err := postcard.SerializePostcard(&api.BatchEventsByTxHashItem{})
		assert.ErrorContains(t, err, "neither Value nor Failure")
	})
}
