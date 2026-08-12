package api

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestAccountView_MarshalPostcard(t *testing.T) {
	t.Run("round trip with all fields populated", func(t *testing.T) {
		original := AccountView{
			Address:   crypto.Address{Bytes: [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}},
			Threshold: 2,
			PublicKeysBs58: []string{
				"8MPNW4vNsTcctGqAtMmuu2ouaAn1oicf6fGymV7Wb6gn",
				"Eqj2KxqDwKTyCz8bXzKQqZh4nEh2vFmb6hVbBLxigBBn",
			},
		}

		//demo1
		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (AccountView, error) {
			var ac AccountView
			if err = ac.UnmarshalPostcard(d); err != nil {
				return ac, err
			}
			return ac, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.Address.Bytes, deserialized.Address.Bytes)
		assert.Equal(t, original.Threshold, deserialized.Threshold)
		assert.Equal(t, original.PublicKeysBs58, deserialized.PublicKeysBs58)

		//demo2
		serializer := postcard.NewSerializer()
		err = original.MarshalPostcard(serializer)
		assert.NoError(t, err)

		deserialized, err = postcard.DeserializePostcard(serializer.Bytes(), func(d *postcard.Deserializer) (AccountView, error) {
			var ac AccountView
			if err = ac.UnmarshalPostcard(d); err != nil {
				return ac, err
			}
			return ac, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.Address.Bytes, deserialized.Address.Bytes)
		assert.Equal(t, original.Threshold, deserialized.Threshold)
		assert.Equal(t, original.PublicKeysBs58, deserialized.PublicKeysBs58)

	})

	t.Run("round trip with empty public keys", func(t *testing.T) {
		original := AccountView{
			Address:        crypto.Address{Bytes: [20]byte{0xAA, 0xBB, 0xCC}},
			Threshold:      1,
			PublicKeysBs58: []string{},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (AccountView, error) {
			var ac AccountView
			if err = ac.UnmarshalPostcard(d); err != nil {
				return ac, err
			}
			return ac, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.Address.Bytes, deserialized.Address.Bytes)
		assert.Equal(t, original.Threshold, deserialized.Threshold)
		assert.Empty(t, deserialized.PublicKeysBs58)
	})

	t.Run("round trip with threshold at max uint8", func(t *testing.T) {
		original := AccountView{
			Address:        crypto.Address{Bytes: [20]byte{0xFF, 0xFF, 0xFF}},
			Threshold:      255,
			PublicKeysBs58: []string{"singleKey"},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (AccountView, error) {
			var ac AccountView
			if err = ac.UnmarshalPostcard(d); err != nil {
				return ac, err
			}
			return ac, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, uint8(255), deserialized.Threshold)
		assert.Len(t, deserialized.PublicKeysBs58, 1)
		assert.Equal(t, "singleKey", deserialized.PublicKeysBs58[0])
	})

	t.Run("round trip with nil public keys", func(t *testing.T) {
		original := AccountView{
			Address:        crypto.Address{Bytes: [20]byte{1, 2, 3}},
			Threshold:      1,
			PublicKeysBs58: nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (AccountView, error) {
			var ac AccountView
			if err = ac.UnmarshalPostcard(d); err != nil {
				return ac, err
			}
			return ac, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.Address.Bytes, deserialized.Address.Bytes)
		assert.Equal(t, original.Threshold, deserialized.Threshold)
		assert.Empty(t, deserialized.PublicKeysBs58)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		address := crypto.Address{Bytes: [20]byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}}

		ac1 := AccountView{
			Address:        address,
			Threshold:      3,
			PublicKeysBs58: []string{"key1", "key2", "key3"},
		}

		ac2 := AccountView{
			Address:        address,
			Threshold:      3,
			PublicKeysBs58: []string{"key1", "key2", "key3"},
		}

		data1, err := postcard.SerializePostcard(&ac1)
		assert.NoError(t, err)

		data2, err := postcard.SerializePostcard(&ac2)
		assert.NoError(t, err)

		assert.Equal(t, data1, data2)
	})
}

func TestAccountView_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (AccountView, error) {
			var ac AccountView
			if err := ac.UnmarshalPostcard(d); err != nil {
				return ac, err
			}
			return ac, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only address bytes", func(t *testing.T) {
		data := make([]byte, 20)
		_, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (AccountView, error) {
			var ac AccountView
			if err := ac.UnmarshalPostcard(d); err != nil {
				return ac, err
			}
			return ac, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := AccountView{
			Address:        crypto.Address{Bytes: [20]byte{1, 2, 3}},
			Threshold:      1,
			PublicKeysBs58: []string{},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (AccountView, error) {
			var ac AccountView
			if err = ac.UnmarshalPostcard(d); err != nil {
				return ac, err
			}
			return ac, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := AccountView{
			Address:        crypto.Address{Bytes: [20]byte{1, 2, 3}},
			Threshold:      1,
			PublicKeysBs58: []string{},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (AccountView, error) {
			var ac AccountView
			if err = ac.UnmarshalPostcard(d); err != nil {
				return ac, err
			}
			return ac, nil
		}, true)
		assert.NoError(t, err)
	})
}
