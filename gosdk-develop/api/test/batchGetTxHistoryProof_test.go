package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeBatchGetTxHistoryProofItem decodes one
// Result<TxHistoryInclusionProof, ApiFailure> item of a
// batch_get_tx_history_proof (61) response.
func decodeBatchGetTxHistoryProofItem(data []byte) (api.BatchGetTxHistoryProofItem, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.BatchGetTxHistoryProofItem, error) {
		var item api.BatchGetTxHistoryProofItem
		if err := item.UnmarshalPostcard(d); err != nil {
			return item, err
		}
		return item, nil
	}, false)
}

func TestBatchGetTxHistoryProofItem_WireFormat(t *testing.T) {
	t.Run("Ok variant prefixes the proof", func(t *testing.T) {
		proof := &api.GetTxHistoryProof{
			Block:    api.Block{Number: 7, TxCount: 2},
			Index:    3,
			Siblings: []api.TxHash{{0x01, 0x02}},
			History:  []byte{0xAA, 0xBB},
		}
		item := api.BatchGetTxHistoryProofItem{Value: proof}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)

		tuple, err := postcard.SerializePostcard(proof)
		require.NoError(t, err)
		assert.Equal(t, append([]byte{0x00}, tuple...), data)

		decoded, err := decodeBatchGetTxHistoryProofItem(data)
		require.NoError(t, err)
		assert.Equal(t, uint64(7), decoded.Value.Block.Number)
		assert.Equal(t, uint32(3), decoded.Value.Index)
		assert.Equal(t, proof.Siblings, decoded.Value.Siblings)
		assert.Equal(t, proof.History, decoded.Value.History)
	})

	t.Run("Err variant carries ApiFailure", func(t *testing.T) {
		item := api.BatchGetTxHistoryProofItem{Failure: &api.ApiFailure{Message: "no proof"}}

		data, err := postcard.SerializePostcard(&item)
		require.NoError(t, err)
		assert.Equal(t, byte(0x01), data[0])
	})

	t.Run("empty item cannot serialize", func(t *testing.T) {
		_, err := postcard.SerializePostcard(&api.BatchGetTxHistoryProofItem{})
		assert.ErrorContains(t, err, "neither Value nor Failure")
	})
}
