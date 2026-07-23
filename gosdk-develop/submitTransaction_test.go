package milon

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestNewSubmitTransaction(t *testing.T) {
	t.Run("create with non-empty body", func(t *testing.T) {
		body := []byte("hello world")
		tx := NewSubmitTransaction(1, 1234567890, body)

		assert.Equal(t, MethodType(1), tx.Method)
		assert.Equal(t, uint64(1234567890), tx.RequestId)
		assert.Equal(t, body, tx.Body)
	})

	t.Run("create with empty body", func(t *testing.T) {
		tx := NewSubmitTransaction(1, 100, []byte{})

		assert.Equal(t, MethodType(1), tx.Method)
		assert.Equal(t, uint64(100), tx.RequestId)
		assert.Equal(t, []byte{}, tx.Body)
	})

	t.Run("create with max uint16 method", func(t *testing.T) {
		tx := NewSubmitTransaction(65535, 0, []byte("test"))
		assert.Equal(t, MethodType(65535), tx.Method)
	})

	t.Run("create with max uint64 request id", func(t *testing.T) {
		tx := NewSubmitTransaction(0, 18446744073709551615, []byte("test"))
		assert.Equal(t, uint64(18446744073709551615), tx.RequestId)
	})

	t.Run("create with large body", func(t *testing.T) {
		body := make([]byte, 10000)
		for i := range body {
			body[i] = byte(i % 256)
		}
		tx := NewSubmitTransaction(255, 5555555555, body)

		assert.Equal(t, MethodType(255), tx.Method)
		assert.Equal(t, uint64(5555555555), tx.RequestId)
		assert.Equal(t, body, tx.Body)
	})
}

func TestSubmitTransaction_MarshalPostcard(t *testing.T) {
	t.Run("round trip with non-empty body", func(t *testing.T) {
		body := []byte("transaction body data")
		original := NewSubmitTransaction(42, 9876543210, body)

		data, err := postcard.SerializePostcard(original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err = st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.Method, deserialized.Method)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Body, deserialized.Body)
	})

	t.Run("round trip with empty body", func(t *testing.T) {
		original := NewSubmitTransaction(1, 100, []byte{})

		data, err := postcard.SerializePostcard(original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err = st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.Method, deserialized.Method)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Body, deserialized.Body)
	})

	t.Run("round trip with large body", func(t *testing.T) {
		body := make([]byte, 10000)
		for i := range body {
			body[i] = byte(i % 256)
		}
		original := NewSubmitTransaction(255, 5555555555, body)

		data, err := postcard.SerializePostcard(original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err = st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.Method, deserialized.Method)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Body, deserialized.Body)
	})

	t.Run("round trip with max uint16 method", func(t *testing.T) {
		body := []byte("max method test")
		original := NewSubmitTransaction(65535, 0, body)

		data, err := postcard.SerializePostcard(original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err = st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, MethodType(65535), deserialized.Method)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Body, deserialized.Body)
	})

	t.Run("round trip with max uint64 request id", func(t *testing.T) {
		body := []byte("max request id test")
		original := NewSubmitTransaction(0, 18446744073709551615, body)

		data, err := postcard.SerializePostcard(original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err = st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, uint64(18446744073709551615), deserialized.RequestId)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Body, deserialized.Body)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		body := []byte("deterministic test")
		tx1 := NewSubmitTransaction(10, 100, body)
		tx2 := NewSubmitTransaction(10, 100, body)

		data1, err := postcard.SerializePostcard(tx1)
		assert.NoError(t, err)

		data2, err := postcard.SerializePostcard(tx2)
		assert.NoError(t, err)

		assert.Equal(t, data1, data2)
	})
}

func TestSubmitTransaction_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err := st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only method", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{0x01}, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err := st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - missing body length", func(t *testing.T) {
		// method=1 → 0x01, requestId=1 → 0x01
		_, err := postcard.DeserializePostcard([]byte{0x01, 0x01}, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err := st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		body := []byte("test body")
		original := NewSubmitTransaction(5, 500, body)

		data, err := postcard.SerializePostcard(original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err := st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		body := []byte("test body")
		original := NewSubmitTransaction(5, 500, body)

		data, err := postcard.SerializePostcard(original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (SubmitTransaction, error) {
			var st SubmitTransaction
			if err := st.UnmarshalPostcard(d); err != nil {
				return st, err
			}
			return st, nil
		}, true)
		assert.NoError(t, err)
	})
}
