package api

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestListResourcePathInfo_UnmarshalListResourcePathListFromRawList(t *testing.T) {
	t.Run("single entry", func(t *testing.T) {
		rawList := [][]any{
			{
				[]interface{}{float64(1), float64(2), float64(3)},
				"/path/to/resource",
			},
		}

		result, err := UnmarshalListResourcePathListFromRawList(rawList)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, RsHash{1, 2, 3}, result[0].RsHash)
		assert.Equal(t, "/path/to/resource", result[0].Path)
	})

	t.Run("multiple entries", func(t *testing.T) {
		rawList := [][]any{
			{
				[]interface{}{float64(10), float64(20)},
				"/first",
			},
			{
				[]interface{}{float64(30), float64(40), float64(50)},
				"/second",
			},
		}

		result, err := UnmarshalListResourcePathListFromRawList(rawList)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, RsHash{10, 20}, result[0].RsHash)
		assert.Equal(t, "/first", result[0].Path)
		assert.Equal(t, RsHash{30, 40, 50}, result[1].RsHash)
		assert.Equal(t, "/second", result[1].Path)
	})

	t.Run("empty list", func(t *testing.T) {
		result, err := UnmarshalListResourcePathListFromRawList([][]any{})
		assert.NoError(t, err)
		assert.Len(t, result, 0)
	})

	t.Run("full 18-byte RsHash", func(t *testing.T) {
		rsHashRaw := make([]interface{}, RsHashLen)
		for i := 0; i < RsHashLen; i++ {
			rsHashRaw[i] = float64(i + 1)
		}

		rawList := [][]any{
			{
				rsHashRaw,
				"full",
			},
		}

		result, err := UnmarshalListResourcePathListFromRawList(rawList)
		assert.NoError(t, err)
		assert.Len(t, result, 1)

		expected := RsHash{}
		for i := 0; i < RsHashLen; i++ {
			expected[i] = byte(i + 1)
		}
		assert.Equal(t, expected, result[0].RsHash)
		assert.Equal(t, "full", result[0].Path)
	})

	t.Run("error - item too short", func(t *testing.T) {
		rawList := [][]any{
			{
				[]interface{}{float64(1)},
			},
		}

		_, err := UnmarshalListResourcePathListFromRawList(rawList)
		assert.Error(t, err)
	})

	t.Run("error - rsHash not []interface{}", func(t *testing.T) {
		rawList := [][]any{
			{
				"not-an-array",
				"/path",
			},
		}

		_, err := UnmarshalListResourcePathListFromRawList(rawList)
		assert.Error(t, err)
	})

	t.Run("error - path not string", func(t *testing.T) {
		rawList := [][]any{
			{
				[]interface{}{float64(1)}, float64(999),
			},
		}

		_, err := UnmarshalListResourcePathListFromRawList(rawList)
		assert.Error(t, err)
	})

	t.Run("error - RsHash exceeds 18 bytes", func(t *testing.T) {
		rsHashRaw := make([]interface{}, RsHashLen+1)
		for i := 0; i <= RsHashLen; i++ {
			rsHashRaw[i] = float64(i)
		}
		rawList := [][]any{
			{
				rsHashRaw,
				"/path",
			},
		}

		_, err := UnmarshalListResourcePathListFromRawList(rawList)
		assert.Error(t, err)
	})
}
