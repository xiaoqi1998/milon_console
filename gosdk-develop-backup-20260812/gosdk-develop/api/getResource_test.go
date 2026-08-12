package api

import (
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetResource_MarshalPostcard(t *testing.T) {
	t.Run("round trip with data", func(t *testing.T) {
		original := GetResource{
			Data: TypeTagWithData{
				TypeTag: 42,
				Value:   []byte("resource data"),
			},
		}

		bytes, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(bytes, func(d *postcard.Deserializer) (GetResource, error) {
			var v GetResource
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("round trip with max uint64", func(t *testing.T) {
		original := GetResource{
			Data: TypeTagWithData{
				TypeTag: 18446744073709551615,
				Value:   []byte{0xFF, 0xFE, 0xFD, 0xFC},
			},
		}

		bytes, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(bytes, func(d *postcard.Deserializer) (GetResource, error) {
			var v GetResource
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		r1 := GetResource{
			Data: TypeTagWithData{
				TypeTag: 42,
				Value:   []byte("resource data"),
			},
		}
		r2 := GetResource{
			Data: TypeTagWithData{
				TypeTag: 42,
				Value:   []byte("resource data"),
			},
		}

		b1, err := postcard.SerializePostcard(&r1)
		assert.NoError(t, err)

		b2, err := postcard.SerializePostcard(&r2)
		assert.NoError(t, err)

		assert.Equal(t, b1, b2)
	})
}

func TestGetResource_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (GetResource, error) {
			var v GetResource
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only type tag", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{0x2A}, func(d *postcard.Deserializer) (GetResource, error) {
			var v GetResource
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := GetResource{
			Data: TypeTagWithData{
				TypeTag: 42,
				Value:   []byte("hello"),
			},
		}

		data, err := postcard.SerializePostcard(&original)

		assert.NoError(t, err)
		dataWithTrailing := append(data, 0x00)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (GetResource, error) {
			var v GetResource
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := GetResource{
			Data: TypeTagWithData{
				TypeTag: 42,
				Value:   []byte("hello"),
			},
		}
		data, err := postcard.SerializePostcard(&original)

		assert.NoError(t, err)
		dataWithTrailing := append(data, 0x00)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (GetResource, error) {
			var v GetResource
			if err = v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, true)
		assert.NoError(t, err)
	})
}
