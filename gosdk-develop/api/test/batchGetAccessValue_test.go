package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchAccessValueItem decodes one
// Result<(BlobHash, Option<Vec<u8>>), ApiFailure> item of a
// batch_get_access_value (166) response.
func decodeBatchAccessValueItem(data []byte) (api.BatchGetAccessValueItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchGetAccessValueItem, error) {
		var item api.BatchGetAccessValueItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

// TestBatchGetAccessValueItem_WireFormat pins the Result encoding of each
// batch_get_access_value (166) item: variant 0 is the Ok tuple, variant 1 is
// the ApiFailure.
func TestBatchGetAccessValueItem_WireFormat(t *testing.T) {
	t.Run("Ok variant prefixes the tuple with variant 0", func(t *testing.T) {
		item := api.BatchGetAccessValueItem{
			Value: &api.GetAccessValueInfo{
				BlobHash: accessValueTestHash,
				Data:     &api.TypeTagWithData{TypeTag: 1, Value: []byte{0xAA, 0xBB}},
			},
		}

		data, err := postcard.SerializePostcard(&item)
		assert.NoError(t, err)

		tuple, err := postcard.SerializePostcard(item.Value)
		assert.NoError(t, err)
		assert.Equal(t, append([]byte{0x00}, tuple...), data)

		decoded, err := decodeBatchAccessValueItem(data)
		assert.NoError(t, err)
		assert.Nil(t, decoded.Failure)
		assert.Equal(t, accessValueTestHash, decoded.Value.BlobHash)
		assert.Equal(t, []byte{0xAA, 0xBB}, decoded.Value.Data.Value)
	})

	t.Run("Ok variant of an absent blob has nil Data", func(t *testing.T) {
		item := api.BatchGetAccessValueItem{
			Value: &api.GetAccessValueInfo{BlobHash: accessValueTestHash},
		}

		data, err := postcard.SerializePostcard(&item)
		assert.NoError(t, err)
		assert.Equal(t, append([]byte{0x00}, append(accessValueTestHash[:], 0x00)...), data)

		decoded, err := decodeBatchAccessValueItem(data)
		assert.NoError(t, err)
		assert.Nil(t, decoded.Value.Data)
		assert.Nil(t, decoded.Failure)
	})

	t.Run("Err variant carries the ApiFailure", func(t *testing.T) {
		item := api.BatchGetAccessValueItem{
			Failure: &api.ApiFailure{Message: "boom"},
		}

		data, err := postcard.SerializePostcard(&item)
		assert.NoError(t, err)
		// variant 1 + String("boom") + Option<u16> None + Option<Vec<u8>> None
		assert.Equal(t, []byte{0x01, 0x04, 'b', 'o', 'o', 'm', 0x00, 0x00}, data)

		decoded, err := decodeBatchAccessValueItem(data)
		assert.NoError(t, err)
		assert.Nil(t, decoded.Value)
		assert.Equal(t, "boom", decoded.Failure.Message)
		assert.Nil(t, decoded.Failure.Code)
		assert.Nil(t, decoded.Failure.Data)
	})

	t.Run("item without value and failure cannot serialize", func(t *testing.T) {
		item := api.BatchGetAccessValueItem{}

		_, err := postcard.SerializePostcard(&item)
		assert.ErrorContains(t, err, "neither Value nor Failure")
	})

	t.Run("unknown Result variant cannot deserialize", func(t *testing.T) {
		_, err := decodeBatchAccessValueItem([]byte{0x02})
		assert.ErrorContains(t, err, "unknown Result variant 2")
	})

	t.Run("batch sequence matches the node response layout", func(t *testing.T) {
		items := []*api.BatchGetAccessValueItem{
			{Value: &api.GetAccessValueInfo{BlobHash: accessValueTestHash}},
			{Failure: &api.ApiFailure{Message: "boom"}},
		}

		serializer := postcard.NewSerializer()
		require.NoError(t, postcard.SerializeSeq(serializer, items, func(s *postcard.Serializer, item *api.BatchGetAccessValueItem) error {
			return item.MarshalPostcard(s)
		}))

		okItem, err := postcard.SerializePostcard(items[0])
		require.NoError(t, err)
		errItem, err := postcard.SerializePostcard(items[1])
		require.NoError(t, err)
		assert.Equal(t, append([]byte{0x02}, append(okItem, errItem...)...), serializer.Bytes())

		decoded, err := postcard.DeserializeSeq(postcard.NewDeserializer(serializer.Bytes()), func(d *postcard.Deserializer) (*api.BatchGetAccessValueItem, error) {
			var item api.BatchGetAccessValueItem
			if err := item.UnmarshalPostcard(d); err != nil {
				return nil, err
			}
			return &item, nil
		})
		require.NoError(t, err)
		require.Len(t, decoded, 2)
		assert.Nil(t, decoded[0].Value.Data)
		assert.Equal(t, "boom", decoded[1].Failure.Message)
	})
}
