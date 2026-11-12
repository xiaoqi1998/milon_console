package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchGetResourceItem decodes one Result<Vec<u8>, ApiFailure> item of a
// batch_get_resource (151) response.
func decodeBatchGetResourceItem(data []byte) (api.BatchGetResourceItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchGetResourceItem, error) {
		var item api.BatchGetResourceItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

func TestBatchGetResourceItem_WireFormat(t *testing.T) {
	res := &api.GetResource{Data: api.TypeTagWithData{TypeTag: 42, Value: []byte{0xAA, 0xBB, 0xCC}}}

	t.Run("Ok variant wraps a length-prefixed GetResource body", func(t *testing.T) {
		item := api.BatchGetResourceItem{Value: res}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		inner, err := postcard.SerializePostcard(res)
		require.NoError(t, err)

		// variant(0) + Vec<u8>{len varint + inner}
		lenPrefix := postcard.NewSerializer()
		require.NoError(t, lenPrefix.SerializeBytes(inner))
		expected := append([]byte{0x00}, lenPrefix.Bytes()...)
		assert.Equal(t, expected, data)

		decoded, err := decodeBatchGetResourceItem(data)
		require.NoError(t, err)
		require.NotNil(t, decoded.Value)
		assert.Equal(t, uint64(42), decoded.Value.Data.TypeTag)
		assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, decoded.Value.Data.Value)
	})

	t.Run("Err variant carries ApiFailure", func(t *testing.T) {
		item := api.BatchGetResourceItem{Failure: &api.ApiFailure{Message: "missing"}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		assert.Equal(t, byte(0x01), data[0])
	})

	t.Run("empty item cannot serialize", func(t *testing.T) {
		_, err := postcard.SerializePostcard(&api.BatchGetResourceItem{})
		assert.ErrorContains(t, err, "neither Value nor Failure")
	})
}
