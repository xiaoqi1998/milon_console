package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeNodeCapabilityView decodes the node_capability (30) response body,
// rejecting trailing bytes like the RPC response decoder does.
func decodeNodeCapabilityView(data []byte) (api.NodeCapabilityView, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.NodeCapabilityView, error) {
		var v api.NodeCapabilityView
		if err := v.UnmarshalPostcard(d); err != nil {
			return v, err
		}
		return v, nil
	}, false)
}

func TestNodeCapabilityView_RoundTrip(t *testing.T) {
	t.Run("empty view keeps nil BinaryDigest", func(t *testing.T) {
		original := api.NodeCapabilityView{}

		data, err := postcard.SerializePostcard(&original)
		require.NoError(t, err)

		// field layout: features(0) digest(0x00=None) height(0)
		// active(0) voting(0) supports_all(0x00) tx_unknown(0)
		assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, data)

		decoded, err := decodeNodeCapabilityView(data)
		require.NoError(t, err)
		assert.Nil(t, decoded.BinaryDigest)
		assert.Empty(t, decoded.Features)
		assert.False(t, decoded.SupportsAll)
	})

	t.Run("populated view round trips", func(t *testing.T) {
		digest := [32]byte{}
		for i := range digest {
			digest[i] = byte(i + 1)
		}
		original := api.NodeCapabilityView{
			Features:      []uint32{1, 2, 300},
			BinaryDigest:  &digest,
			Height:        42,
			MissingActive: []uint32{7},
			MissingVoting: []uint32{},
			SupportsAll:   true,
			TxUnknown: []api.TxUnknownCount{
				{Kind: "format", Value: "0x01", Count: 5},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		require.NoError(t, err)

		decoded, err := decodeNodeCapabilityView(data)
		require.NoError(t, err)
		assert.Equal(t, original.Features, decoded.Features)
		require.NotNil(t, decoded.BinaryDigest)
		assert.Equal(t, digest, *decoded.BinaryDigest)
		assert.Equal(t, original.Height, decoded.Height)
		assert.Equal(t, original.MissingActive, decoded.MissingActive)
		assert.Empty(t, decoded.MissingVoting)
		assert.True(t, decoded.SupportsAll)
		require.Len(t, decoded.TxUnknown, 1)
		assert.Equal(t, "format", decoded.TxUnknown[0].Kind)
		assert.Equal(t, "0x01", decoded.TxUnknown[0].Value)
		assert.Equal(t, uint64(5), decoded.TxUnknown[0].Count)
	})
}
