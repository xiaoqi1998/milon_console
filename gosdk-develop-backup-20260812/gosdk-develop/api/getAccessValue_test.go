package api

import (
	"bytes"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetAccessValueInfo_MarshalPostcard(t *testing.T) {
	t.Run("round trip with nil Data", func(t *testing.T) {
		original := GetAccessValueInfo{
			BlobHash: BlobHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			Data:     nil,
		}

		buf, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(buf, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
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
		original := GetAccessValueInfo{
			BlobHash: BlobHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			Data: &TypeTagWithData{
				TypeTag: 42,
				Value:   value,
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
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
		original := GetAccessValueInfo{
			BlobHash: BlobHash{1, 2, 3},
			Data: &TypeTagWithData{
				TypeTag: 100,
				Value:   []byte{},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
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
		original := GetAccessValueInfo{
			BlobHash: BlobHash{},
			Data:     nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
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
		original := GetAccessValueInfo{
			BlobHash: BlobHash{0xFF, 0xEE},
			Data: &TypeTagWithData{
				TypeTag: 18446744073709551615, // max uint64
				Value:   []byte{0x00, 0x01, 0x02},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
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
		v1 := GetAccessValueInfo{
			BlobHash: BlobHash{1, 2, 3},
			Data: &TypeTagWithData{
				TypeTag: 42,
				Value:   value,
			},
		}
		v2 := GetAccessValueInfo{
			BlobHash: BlobHash{1, 2, 3},
			Data: &TypeTagWithData{
				TypeTag: 42,
				Value:   value,
			},
		}

		b1, err := postcard.SerializePostcard(&v1)
		assert.NoError(t, err)

		b2, err := postcard.SerializePostcard(&v2)
		assert.NoError(t, err)

		assert.True(t, bytes.Equal(b1, b2))
	})
}

func TestGetAccessValueInfo_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated blob hash", func(t *testing.T) {
		_, err := postcard.DeserializePostcard(make([]byte, 16), func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
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
		_, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
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
		_, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
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
		_, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := GetAccessValueInfo{
			BlobHash: BlobHash{1},
			Data:     nil,
		}
		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := GetAccessValueInfo{
			BlobHash: BlobHash{1},
			Data:     nil,
		}
		data, err := postcard.SerializePostcard(&original)

		assert.NoError(t, err)
		dataWithTrailing := append(data, 0xFF, 0xFE)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (GetAccessValueInfo, error) {
			var v GetAccessValueInfo
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, true)
		assert.NoError(t, err)
	})
}
