package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchViewItem decodes one Result<Vec<u8>, ApiFailure> item of a
// batch_view (16) response.
func decodeBatchViewItem(data []byte) (api.BatchViewItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchViewItem, error) {
		var item api.BatchViewItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

func TestBatchViewItem_WireFormat(t *testing.T) {
	t.Run("Ok variant length-prefixes the return value", func(t *testing.T) {
		returnValue := []byte{0x2A}
		item := api.BatchViewItem{Data: returnValue}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		// variant(0) + varint(1) + payload
		assert.Equal(t, []byte{0x00, 0x01, 0x2A}, data)

		decoded, err := decodeBatchViewItem(data)
		require.NoError(t, err)
		assert.Nil(t, decoded.Failure)
		assert.Equal(t, returnValue, decoded.Data)
	})

	t.Run("Ok variant stays non-nil for an empty return value", func(t *testing.T) {
		item := api.BatchViewItem{Data: []byte{}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		assert.Equal(t, []byte{0x00, 0x00}, data)

		decoded, err := decodeBatchViewItem(data)
		require.NoError(t, err)
		assert.Nil(t, decoded.Failure)
		require.NotNil(t, decoded.Data)
		assert.Empty(t, decoded.Data)
	})

	t.Run("Err variant carries ApiFailure", func(t *testing.T) {
		item := api.BatchViewItem{Failure: &api.ApiFailure{Message: "boom"}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		assert.Equal(t, byte(0x01), data[0])

		decoded, err := decodeBatchViewItem(data)
		require.NoError(t, err)
		assert.Nil(t, decoded.Data)
		require.NotNil(t, decoded.Failure)
		assert.Equal(t, "boom", decoded.Failure.Message)
	})

	t.Run("empty item cannot serialize", func(t *testing.T) {
		_, err := postcard.SerializePostcard(&api.BatchViewItem{})
		assert.ErrorContains(t, err, "neither Data nor Failure")
	})

	t.Run("unknown Result variant cannot deserialize", func(t *testing.T) {
		_, err := decodeBatchViewItem([]byte{0x02})
		assert.ErrorContains(t, err, "unknown Result variant 2")
	})

	t.Run("a whole batch keeps one Result per instruction", func(t *testing.T) {
		items := []*api.BatchViewItem{
			{Data: []byte{0x2A}},
			{Failure: &api.ApiFailure{Message: "boom"}},
		}

		serializer := postcard.NewSerializer()
		require.NoError(t, postcard.SerializeSeq(serializer, items, func(s *postcard.Serializer, item *api.BatchViewItem) error {
			return item.MarshalPostcard(s)
		}))

		decoded, err := postcard.DeserializeSeq(postcard.NewDeserializer(serializer.Bytes()), func(d *postcard.Deserializer) (*api.BatchViewItem, error) {
			var item api.BatchViewItem
			if err := item.UnmarshalPostcard(d); err != nil {
				return nil, err
			}
			return &item, nil
		})
		require.NoError(t, err)
		require.Len(t, decoded, 2)
		assert.Equal(t, []byte{0x2A}, decoded[0].Data)
		assert.Nil(t, decoded[0].Failure)
		assert.Nil(t, decoded[1].Data)
		require.NotNil(t, decoded[1].Failure)
		assert.Equal(t, "boom", decoded[1].Failure.Message)
	})
}
