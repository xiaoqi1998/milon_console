package api

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestRpcResponse_MarshalPostcard(t *testing.T) {
	t.Run("round trip with all fields (no error)", func(t *testing.T) {
		original := RpcResponse{
			RequestId: 1234567890,
			Status:    RpcResponseStatusOk,
			Body:      []byte("hello world"),
			Error:     nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (RpcResponse, error) {
			var rsp RpcResponse
			if err = rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Status, deserialized.Status)
		assert.Equal(t, original.Body, deserialized.Body)
		assert.Equal(t, original.Error, deserialized.Error)
	})

	t.Run("round trip with error", func(t *testing.T) {
		code := uint16(404)
		data := []byte("extra data")
		original := RpcResponse{
			RequestId: 9876543210,
			Status:    RpcResponseStatusNotFound,
			Body:      []byte("not found"),
			Error: &RpcResponseError{
				Message: "resource not available",
				Code:    &code,
				Data:    &data,
			},
		}

		dataBytes, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(dataBytes, func(d *postcard.Deserializer) (RpcResponse, error) {
			var rsp RpcResponse
			if err = rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Status, deserialized.Status)
		assert.Equal(t, original.Body, deserialized.Body)
		assert.Equal(t, original.Error, deserialized.Error)
	})

	t.Run("round trip with error and nil code and data", func(t *testing.T) {
		original := RpcResponse{
			RequestId: 555,
			Status:    RpcResponseStatusInternal,
			Body:      []byte{},
			Error: &RpcResponseError{
				Message: "internal error",
				Code:    nil,
				Data:    nil,
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (RpcResponse, error) {
			var rsp RpcResponse
			if err = rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Status, deserialized.Status)
		assert.Equal(t, original.Body, deserialized.Body)
		assert.Equal(t, original.Error, deserialized.Error)
	})

	t.Run("round trip with empty body", func(t *testing.T) {
		original := RpcResponse{
			RequestId: 1,
			Status:    RpcResponseStatusOk,
			Body:      []byte{},
			Error:     nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (RpcResponse, error) {
			var rsp RpcResponse
			if err = rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Status, deserialized.Status)
		assert.Equal(t, original.Body, deserialized.Body)
		assert.Equal(t, original.Error, deserialized.Error)
	})

	t.Run("round trip with nil body", func(t *testing.T) {
		original := RpcResponse{
			RequestId: 0,
			Status:    RpcResponseStatusOk,
			Body:      nil,
			Error:     nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (RpcResponse, error) {
			var rsp RpcResponse
			if err = rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.RequestId, deserialized.RequestId)
		assert.Equal(t, original.Status, deserialized.Status)
		assert.Equal(t, []byte{}, deserialized.Body)
		assert.Equal(t, original.Error, deserialized.Error)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		code := uint16(200)
		rsp1 := RpcResponse{
			RequestId: 100,
			Status:    RpcResponseStatusOk,
			Body:      []byte("test"),
			Error: &RpcResponseError{
				Message: "ok",
				Code:    &code,
				Data:    nil,
			},
		}
		rsp2 := RpcResponse{
			RequestId: 100,
			Status:    RpcResponseStatusOk,
			Body:      []byte("test"),
			Error: &RpcResponseError{
				Message: "ok",
				Code:    &code,
				Data:    nil,
			},
		}

		data1, err := postcard.SerializePostcard(&rsp1)
		assert.NoError(t, err)

		data2, err := postcard.SerializePostcard(&rsp2)
		assert.NoError(t, err)

		assert.Equal(t, data1, data2)
	})
}

func TestRpcResponse_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (RpcResponse, error) {
			var rsp RpcResponse
			if err := rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only request id", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{0x01}, func(d *postcard.Deserializer) (RpcResponse, error) {
			var rsp RpcResponse
			if err := rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := RpcResponse{
			RequestId: 1,
			Status:    RpcResponseStatusOk,
			Body:      []byte("test"),
			Error:     nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (RpcResponse, error) {
			var rsp RpcResponse
			if err = rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := RpcResponse{
			RequestId: 1,
			Status:    RpcResponseStatusOk,
			Body:      []byte("test"),
			Error:     nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (RpcResponse, error) {
			var rsp RpcResponse
			if err = rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, true)
		assert.NoError(t, err)
	})
}
