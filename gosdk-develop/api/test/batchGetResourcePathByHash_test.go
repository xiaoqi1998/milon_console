package test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawRPathHash builds the JSON byte array of an RsHash. The node reports every
// JSON number as float64, which is what the unmarshal helper expects.
func rawRPathHash(lastByte byte) []interface{} {
	raw := make([]interface{}, api.RsHashLen)
	for i := range raw {
		raw[i] = float64(0)
	}
	raw[api.RsHashLen-1] = float64(lastByte)
	return raw
}

// rawRPathItem builds one [rsHashBytes, Result] element of a
// batch_get_resource_path_by_hash (160) response.
func rawRPathItem(rsHash, result interface{}) []any {
	return []any{rsHash, result}
}

func TestUnmarshalBatchResourcePathListFromRawList(t *testing.T) {
	t.Run("Ok and Err items", func(t *testing.T) {
		rawList := [][]any{
			rawRPathItem(rawRPathHash(1), map[string]interface{}{"Ok": "/token/_Owner/a"}),
			rawRPathItem(rawRPathHash(2), map[string]interface{}{"Err": "not found"}),
		}

		infos, err := api.UnmarshalBatchResourcePathListFromRawList(rawList)
		require.NoError(t, err)
		require.Len(t, infos, 2)

		assert.Equal(t, byte(1), infos[0].RsHash[api.RsHashLen-1])
		assert.Equal(t, "/token/_Owner/a", infos[0].Path)
		assert.Empty(t, infos[0].ErrMsg)

		assert.Equal(t, byte(2), infos[1].RsHash[api.RsHashLen-1])
		assert.Equal(t, "not found", infos[1].ErrMsg)
		assert.Empty(t, infos[1].Path)
	})

	t.Run("result carrying neither Ok nor Err is rejected", func(t *testing.T) {
		// A miss must never be reported as an Ok with an empty path.
		rawList := [][]any{
			rawRPathItem(rawRPathHash(3), map[string]interface{}{}),
		}

		_, err := api.UnmarshalBatchResourcePathListFromRawList(rawList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "neither Ok nor Err")
	})

	t.Run("non-string Ok path is rejected", func(t *testing.T) {
		rawList := [][]any{
			rawRPathItem(rawRPathHash(4), map[string]interface{}{"Ok": float64(7)}),
		}

		_, err := api.UnmarshalBatchResourcePathListFromRawList(rawList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid BatchGetResourcePathInfo Ok path")
	})

	t.Run("non-string Err message is rejected", func(t *testing.T) {
		rawList := [][]any{
			rawRPathItem(rawRPathHash(5), map[string]interface{}{"Err": nil}),
		}

		_, err := api.UnmarshalBatchResourcePathListFromRawList(rawList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid BatchGetResourcePathInfo Err message")
	})

	t.Run("item without a result is rejected", func(t *testing.T) {
		rawList := [][]any{{rawRPathHash(6)}}

		_, err := api.UnmarshalBatchResourcePathListFromRawList(rawList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid BatchGetResourcePathInfo response")
	})

	t.Run("non-array RsHash is rejected", func(t *testing.T) {
		rawList := [][]any{
			rawRPathItem("not-an-array", map[string]interface{}{"Ok": "/p"}),
		}

		_, err := api.UnmarshalBatchResourcePathListFromRawList(rawList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid BatchGetResourcePathInfo response")
	})

	t.Run("oversized RsHash is rejected", func(t *testing.T) {
		rawHash := rawRPathHash(7)
		rawHash = append(rawHash, float64(0))

		rawList := [][]any{
			rawRPathItem(rawHash, map[string]interface{}{"Ok": "/p"}),
		}

		_, err := api.UnmarshalBatchResourcePathListFromRawList(rawList)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse RsHash")
	})

	t.Run("empty list", func(t *testing.T) {
		infos, err := api.UnmarshalBatchResourcePathListFromRawList(nil)
		require.NoError(t, err)
		assert.Empty(t, infos)
	})
}
