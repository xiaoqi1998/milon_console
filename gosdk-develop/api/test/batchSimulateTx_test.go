package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchSimulateTxItem decodes one Result<Vec<u8>, ApiFailure> item of a
// batch_simulate_tx (11) response.
func decodeBatchSimulateTxItem(data []byte) (api.BatchSimulateTxItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchSimulateTxItem, error) {
		var item api.BatchSimulateTxItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

func TestBatchSimulateTxItem_WireFormat(t *testing.T) {
	receipt := &api.SimulateReceipt{
		GasCharged: 9,
	}
	copy(receipt.Magic[:], "MSDK")
	receipt.Version = 1
	receipt.State = 1

	t.Run("Ok variant wraps a length-prefixed SimulateReceipt", func(t *testing.T) {
		item := api.BatchSimulateTxItem{Value: receipt}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		inner, err := postcard.SerializePostcard(receipt)
		require.NoError(t, err)

		// variant(0) + Vec<u8>{len varint + inner}
		lenPrefix := postcard.NewSerializer()
		require.NoError(t, lenPrefix.SerializeBytes(inner))
		expected := append([]byte{0x00}, lenPrefix.Bytes()...)
		assert.Equal(t, expected, data)

		decoded, err := decodeBatchSimulateTxItem(data)
		require.NoError(t, err)
		assert.Nil(t, decoded.Failure)
		require.NotNil(t, decoded.Value)
		assert.Equal(t, receipt.Magic, decoded.Value.Magic)
		assert.Equal(t, uint8(1), decoded.Value.Version)
		assert.Equal(t, uint8(1), decoded.Value.State)
		assert.Equal(t, uint64(9), decoded.Value.GasCharged)
		assert.Empty(t, decoded.Value.Access)
		assert.Empty(t, decoded.Value.Events)
		assert.Nil(t, decoded.Value.Error)
	})

	t.Run("Ok payload of a failed receipt keeps the TxFailurePayload", func(t *testing.T) {
		failed := *receipt
		failed.State = 2
		failed.Error = &api.TxFailurePayload{Code: 4009, Message: "insufficient gas", Data: []byte{0x01, 0x02}}
		item := api.BatchSimulateTxItem{Value: &failed}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		decoded, err := decodeBatchSimulateTxItem(data)
		require.NoError(t, err)
		require.NotNil(t, decoded.Value.Error)
		assert.Equal(t, uint16(4009), decoded.Value.Error.Code)
		assert.Equal(t, "insufficient gas", decoded.Value.Error.Message)
		assert.Equal(t, []byte{0x01, 0x02}, decoded.Value.Error.Data)
	})

	t.Run("Err variant carries ApiFailure", func(t *testing.T) {
		item := api.BatchSimulateTxItem{Failure: &api.ApiFailure{Message: "unknown tx type"}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		assert.Equal(t, byte(0x01), data[0])
	})

	t.Run("empty item cannot serialize", func(t *testing.T) {
		_, err := postcard.SerializePostcard(&api.BatchSimulateTxItem{})
		assert.ErrorContains(t, err, "neither Value nor Failure")
	})
}
