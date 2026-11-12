package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchGetTxByHashItem decodes one Result<TxHistory, ApiFailure> item of
// a batch_get_tx_by_hash (56) response.
func decodeBatchGetTxByHashItem(data []byte) (api.BatchGetTxByHashItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchGetTxByHashItem, error) {
		var item api.BatchGetTxByHashItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

func TestBatchGetTxByHashItem_WireFormat(t *testing.T) {
	// Payer is an Option<u8> on the wire; use Some(1) for the fixture.
	payer := uint8(1)
	history := &api.TxHistory{Stamp: 1_735_689_600_000, Payer: &payer}

	t.Run("Ok variant prefixes the TxHistory tuple", func(t *testing.T) {
		item := api.BatchGetTxByHashItem{Value: history}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		tuple, err := postcard.SerializePostcard(history)
		require.NoError(t, err)
		assert.Equal(t, append([]byte{0x00}, tuple...), data)

		decoded, err := decodeBatchGetTxByHashItem(data)
		require.NoError(t, err)
		require.NotNil(t, decoded.Value)
		assert.Equal(t, history.Stamp, decoded.Value.Stamp)
	})

	t.Run("Err variant carries ApiFailure", func(t *testing.T) {
		item := api.BatchGetTxByHashItem{Failure: &api.ApiFailure{Message: "tx not found"}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		assert.Equal(t, byte(0x01), data[0])
	})

	t.Run("empty item cannot serialize", func(t *testing.T) {
		_, err := postcard.SerializePostcard(&api.BatchGetTxByHashItem{})
		assert.ErrorContains(t, err, "neither Value nor Failure")
	})
}
