package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchGetAccountItem decodes one Result<AccountView, ApiFailure> item
// of a batch_get_account (21) response, rejecting trailing bytes.
func decodeBatchGetAccountItem(data []byte) (api.BatchGetAccountItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchGetAccountItem, error) {
		var item api.BatchGetAccountItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

func TestBatchGetAccountItem_WireFormat(t *testing.T) {
	addr, err := crypto.NewAddressFromRelaxed(api.MIL)
	require.NoError(t, err)

	t.Run("Ok variant prefixes the AccountView", func(t *testing.T) {
		item := api.BatchGetAccountItem{Value: &api.AccountView{
			Address:        *addr,
			Threshold:      1,
			PublicKeysBs58: []string{"pk1", "pk2"},
		}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		tuple, err := postcard.SerializePostcard(item.Value)
		require.NoError(t, err)
		assert.Equal(t, append([]byte{0x00}, tuple...), data)

		decoded, err := decodeBatchGetAccountItem(data)
		require.NoError(t, err)
		assert.Nil(t, decoded.Failure)
		require.NotNil(t, decoded.Value)
		assert.Equal(t, uint8(1), decoded.Value.Threshold)
		assert.Equal(t, []string{"pk1", "pk2"}, decoded.Value.PublicKeysBs58)
	})

	t.Run("Err variant carries the ApiFailure", func(t *testing.T) {
		item := api.BatchGetAccountItem{Failure: &api.ApiFailure{Message: "not found"}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		// variant 1 + String("not found") + Option<u16> None + Option<Vec<u8>> None
		assert.Equal(t, byte(0x01), data[0])

		decoded, err := decodeBatchGetAccountItem(data)
		require.NoError(t, err)
		assert.Nil(t, decoded.Value)
		require.NotNil(t, decoded.Failure)
		assert.Equal(t, "not found", decoded.Failure.Message)
	})

	t.Run("empty item cannot serialize", func(t *testing.T) {
		_, err := postcard.SerializePostcard(&api.BatchGetAccountItem{})
		assert.ErrorContains(t, err, "neither Value nor Failure")
	})

	t.Run("unknown Result variant cannot deserialize", func(t *testing.T) {
		_, err := decodeBatchGetAccountItem([]byte{0x02})
		assert.ErrorContains(t, err, "unknown Result variant 2")
	})
}

func TestBatchGetAccountItem_SeqRoundTrip(t *testing.T) {
	items := []*api.BatchGetAccountItem{
		{Failure: &api.ApiFailure{Message: "one"}},
		{Failure: &api.ApiFailure{Message: "two"}},
	}

	serializer := postcard.NewSerializer()
	require.NoError(t, postcard.SerializeSeq(serializer, items, func(s *postcard.Serializer, item *api.BatchGetAccountItem) error {
		return item.MarshalPostcard(s)
	}))

	decoded, err := postcard.DeserializeSeq(postcard.NewDeserializer(serializer.Bytes()), func(d *postcard.Deserializer) (*api.BatchGetAccountItem, error) {
		var item api.BatchGetAccountItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &item, nil
	})
	require.NoError(t, err)
	require.Len(t, decoded, 2)
	assert.Equal(t, "one", decoded[0].Failure.Message)
	assert.Equal(t, "two", decoded[1].Failure.Message)
}
