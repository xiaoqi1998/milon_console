package test

import (
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetAccessValueInfo_MarshalPostcard(t *testing.T) {
	t.Run("round trip with nil Data", func(t *testing.T) {
		original := api.GetAccessValueInfo{
			BlobHash: api.BlobHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			Data:     nil,
		}

		buf, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(buf, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.BlobHash, deserialized.BlobHash)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("round trip with non-nil Data", func(t *testing.T) {
		value := []byte("hello world")
		original := api.GetAccessValueInfo{
			BlobHash: api.BlobHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			Data: &api.TypeTagWithData{
				TypeTag: 42,
				Value:   value,
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.BlobHash, deserialized.BlobHash)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("round trip with empty value", func(t *testing.T) {
		original := api.GetAccessValueInfo{
			BlobHash: api.BlobHash{1, 2, 3},
			Data: &api.TypeTagWithData{
				TypeTag: 100,
				Value:   []byte{},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.BlobHash, deserialized.BlobHash)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("round trip with zero BlobHash", func(t *testing.T) {
		original := api.GetAccessValueInfo{
			BlobHash: api.BlobHash{},
			Data:     nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.BlobHash, deserialized.BlobHash)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("round trip with large typeTag", func(t *testing.T) {
		original := api.GetAccessValueInfo{
			BlobHash: api.BlobHash{0xFF, 0xEE},
			Data: &api.TypeTagWithData{
				TypeTag: 18446744073709551615, // max uint64
				Value:   []byte{0x00, 0x01, 0x02},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.BlobHash, deserialized.BlobHash)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		value := []byte("deterministic")
		v1 := api.GetAccessValueInfo{
			BlobHash: api.BlobHash{1, 2, 3},
			Data: &api.TypeTagWithData{
				TypeTag: 42,
				Value:   value,
			},
		}
		v2 := api.GetAccessValueInfo{
			BlobHash: api.BlobHash{1, 2, 3},
			Data: &api.TypeTagWithData{
				TypeTag: 42,
				Value:   value,
			},
		}

		b1, err := postcard.SerializePostcard(&v1)
		assert.NoError(t, err)

		b2, err := postcard.SerializePostcard(&v2)
		assert.NoError(t, err)

		assert.Equal(t, b1, b2)
	})
}

func TestGetAccessValueInfo_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated blob hash", func(t *testing.T) {
		_, err := postcard.DeserializePostcard(make([]byte, 16), func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("hasData true but missing Vec<u8>", func(t *testing.T) {
		// 32 bytes BlobHash + 1 byte hasData=true, then no more bytes
		data := make([]byte, 33)
		data[32] = 0x01 // hasData = true
		_, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("hasData true but Vec<u8> is empty", func(t *testing.T) {
		// 32 bytes BlobHash + hasData=true + Vec<u8> empty (length=0)
		data := make([]byte, 34)
		data[32] = 0x01 // hasData = true
		data[33] = 0x00 // Vec<u8> length = 0
		_, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("hasData true but Vec<u8> missing typeTag", func(t *testing.T) {
		// 32 bytes BlobHash + hasData=true + Vec<u8> len=1 but no valid typeTag
		data := make([]byte, 35)
		data[32] = 0x01 // hasData = true
		data[33] = 0x01 // Vec<u8> length = 1
		data[34] = 0x80 // partial varint (continuation bit set, no next byte)
		_, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := api.GetAccessValueInfo{
			BlobHash: api.BlobHash{1},
			Data:     nil,
		}
		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := api.GetAccessValueInfo{
			BlobHash: api.BlobHash{1},
			Data:     nil,
		}
		data, err := postcard.SerializePostcard(&original)

		assert.NoError(t, err)
		dataWithTrailing := append(data, 0xFF, 0xFE)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
			var v api.GetAccessValueInfo
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, true)
		assert.NoError(t, err)
	})
}

// decodeAccessValueInfo decodes a single (BlobHash, Option<Vec<u8>>) tuple,
// rejecting trailing bytes like the RPC response decoder does.
func decodeAccessValueInfo(data []byte) (api.GetAccessValueInfo, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.GetAccessValueInfo, error) {
		var v api.GetAccessValueInfo
		if err := v.UnmarshalPostcard(d); err != nil {
			return v, err
		}
		return v, nil
	}, false)
}

// accessValueTestHash is the BlobHash of the External last-written value of
// devnet tx 3779YZAxYMxvKUFr92rwnUdiq3p3adhYhC12DX6vVk4R, access record [1].
var accessValueTestHash = api.BlobHash{
	0xed, 0xce, 0xea, 0x7f, 0x27, 0x9e, 0xef, 0x19, 0x38, 0xe9, 0xfc, 0x05, 0xb5, 0x5a, 0xe6, 0x62,
	0xde, 0x6a, 0xb5, 0xfe, 0xc8, 0x35, 0x0e, 0xd7, 0xdb, 0x91, 0x81, 0x81, 0xb4, 0xa9, 0x91, 0x99,
}

// TestGetAccessValueInfo_WireFormat pins the exact bytes of the get_access_value
// (165) response tuple (BlobHash, Option<Vec<u8>>) rather than only checking
// round trips: the hash is bare (no length prefix) and the value is a
// length-prefixed Vec<u8> holding (type_tag, value).
func TestGetAccessValueInfo_WireFormat(t *testing.T) {
	t.Run("Option None is the hash plus a zero flag", func(t *testing.T) {
		info := api.GetAccessValueInfo{BlobHash: accessValueTestHash}

		data, err := postcard.SerializePostcard(&info)
		assert.NoError(t, err)
		assert.Equal(t, append(accessValueTestHash[:], 0x00), data)

		decoded, err := decodeAccessValueInfo(data)
		assert.NoError(t, err)
		assert.Equal(t, accessValueTestHash, decoded.BlobHash)
		assert.Nil(t, decoded.Data)
	})

	t.Run("Option Some is a length prefixed type_tag plus value", func(t *testing.T) {
		info := api.GetAccessValueInfo{
			BlobHash: accessValueTestHash,
			Data:     &api.TypeTagWithData{TypeTag: 1, Value: []byte{0xAA, 0xBB}},
		}

		data, err := postcard.SerializePostcard(&info)
		assert.NoError(t, err)

		expected := append(accessValueTestHash[:], 0x01) // Option Some
		expected = append(expected, 0x03)                // Vec<u8> length: 1 tag byte + 2 value bytes
		expected = append(expected, 0x01)                // type_tag varint
		expected = append(expected, 0xAA, 0xBB)
		assert.Equal(t, expected, data)

		decoded, err := decodeAccessValueInfo(data)
		assert.NoError(t, err)
		assert.Equal(t, accessValueTestHash, decoded.BlobHash)
		assert.Equal(t, uint64(1), decoded.Data.TypeTag)
		assert.Equal(t, []byte{0xAA, 0xBB}, decoded.Data.Value)
	})
}
