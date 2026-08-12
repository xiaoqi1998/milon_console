package api

import (
	"fmt"
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
	Error     *RpcResponseError `json:"error"`
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

	err = postcard.SerializeOption(serializer, rsp.Error, func(s *postcard.Serializer, af RpcResponseError) error {
		return af.MarshalPostcard(serializer)
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

	rsp.Error, err = postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (RpcResponseError, error) {
		var af RpcResponseError
		err = af.UnmarshalPostcard(deserializer)
		if err != nil {
			return RpcResponseError{}, fmt.Errorf("failed to deserialize Error: %w", err)
		}
		return af, nil
	})

	return nil
}

type RpcResponseError struct {
	Message string
	Code    *uint16
	Data    *[]byte
}

func (rspErr *RpcResponseError) MarshalPostcard(serializer *postcard.Serializer) error {
	err := serializer.SerializeStr(rspErr.Message)
	if err != nil {
		return fmt.Errorf("failed to serialize Message: %w", err)
	}

	err = postcard.SerializeOption(serializer, rspErr.Code, func(s *postcard.Serializer, code uint16) error {
		return serializer.SerializeU16(code)
	})
	if err != nil {
		return fmt.Errorf("failed to serialize Code: %w", err)
	}

	err = postcard.SerializeOption(serializer, rspErr.Data, func(s *postcard.Serializer, data []byte) error {
		return serializer.SerializeBytes(data)
	})
	if err != nil {
		return fmt.Errorf("failed to serialize Data: %w", err)
	}

	return nil
}

func (rspErr *RpcResponseError) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	var err error

	rspErr.Message, err = deserializer.DeserializeStr()
	if err != nil {
		return fmt.Errorf("failed to deserialize Message: %w", err)
	}

	rspErr.Code, err = postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (uint16, error) {
		return deserializer.DeserializeU16()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Code: %w", err)
	}

	rspErr.Data, err = postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) ([]byte, error) {
		return deserializer.DeserializeBytes()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Data: %w", err)
	}

	return nil
}
