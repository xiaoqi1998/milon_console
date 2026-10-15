package lib

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

type RpcResponseStatus = uint8

const (
	RpcResponseStatusOk          RpcResponseStatus = 0
	RpcResponseStatusInvalid     RpcResponseStatus = 1
	RpcResponseStatusNotFound    RpcResponseStatus = 2
	RpcResponseStatusDisabled    RpcResponseStatus = 3
	RpcResponseStatusUnavailable RpcResponseStatus = 4
	RpcResponseStatusInternal    RpcResponseStatus = 5
	RpcResponseStatusFailed      RpcResponseStatus = 6
)

type RpcResponse struct {
	RequestId uint64            `json:"request_id"`
	Status    RpcResponseStatus `json:"status"`
	Body      []byte            `json:"body"`
	Error     *api.ApiFailure   `json:"error"`
}

func (rsp *RpcResponse) MarshalPostcard(serializer *postcard.Serializer) error {
	err := serializer.SerializeU64(rsp.RequestId)
	if err != nil {
		return fmt.Errorf("failed to serialize RequestId: %w", err)
	}

	err = serializer.SerializeU8(rsp.Status)
	if err != nil {
		return fmt.Errorf("failed to serialize Status: %w", err)
	}

	err = serializer.SerializeBytes(rsp.Body)
	if err != nil {
		return fmt.Errorf("failed to serialize Body: %w", err)
	}

	err = postcard.SerializeOption(serializer, rsp.Error, func(s *postcard.Serializer, af api.ApiFailure) error {
		return af.MarshalPostcard(s)
	})
	if err != nil {
		return fmt.Errorf("failed to serialize Error: %w", err)
	}

	return nil
}

func (rsp *RpcResponse) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	var err error

	rsp.RequestId, err = deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Message: %w", err)
	}

	rsp.Status, err = deserializer.DeserializeU8()
	if err != nil {
		return fmt.Errorf("failed to deserialize Status: %w", err)
	}

	rsp.Body, err = deserializer.DeserializeBytes()
	if err != nil {
		return fmt.Errorf("failed to deserialize Body: %w", err)
	}
	if rsp.Body == nil {
		rsp.Body = []byte{}
	}

	rsp.Error, err = postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (api.ApiFailure, error) {
		var af api.ApiFailure
		if err := af.UnmarshalPostcard(d); err != nil {
			return api.ApiFailure{}, fmt.Errorf("failed to deserialize Error: %w", err)
		}
		return af, nil
	})

	return err
}
