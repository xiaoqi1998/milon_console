package api

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	rust (api/rpc/wire/src/envelope.rs)
	pub struct ApiFailure {
		pub message: String,
		#[serde(default)]
		pub code: Option<u16>,
		#[serde(default)]
		pub data: Option<Vec<u8>>,
	}
*/

// ApiFailure is the structured failure carried by RPC responses: the error
// field of the response envelope and each Err item of a batch response body
// (e.g. batch_view returns Vec<Result<Vec<u8>, ApiFailure>>).
type ApiFailure struct {
	Message string
	Code    *uint16
	Data    *[]byte
}

func (f *ApiFailure) MarshalPostcard(serializer *postcard.Serializer) error {
	if err := serializer.SerializeStr(f.Message); err != nil {
		return fmt.Errorf("failed to serialize Message: %w", err)
	}

	if err := postcard.SerializeOption(serializer, f.Code, func(s *postcard.Serializer, code uint16) error {
		return s.SerializeU16(code)
	}); err != nil {
		return fmt.Errorf("failed to serialize Code: %w", err)
	}

	if err := postcard.SerializeOption(serializer, f.Data, func(s *postcard.Serializer, data []byte) error {
		return s.SerializeBytes(data)
	}); err != nil {
		return fmt.Errorf("failed to serialize Data: %w", err)
	}

	return nil
}

func (f *ApiFailure) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	var err error

	f.Message, err = deserializer.DeserializeStr()
	if err != nil {
		return fmt.Errorf("failed to deserialize Message: %w", err)
	}

	f.Code, err = postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (uint16, error) {
		return d.DeserializeU16()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Code: %w", err)
	}

	f.Data, err = postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) ([]byte, error) {
		return d.DeserializeBytes()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Data: %w", err)
	}

	return nil
}

// String implements the Stringer interface for concise error logging: the
// code is printed in decimal and the data payload in hexadecimal.
func (f *ApiFailure) String() string {
	var sb strings.Builder
	sb.WriteString("message=")
	sb.WriteString(strconv.Quote(f.Message))
	if f.Code != nil {
		sb.WriteString(" code=")
		sb.WriteString(strconv.FormatUint(uint64(*f.Code), 10))
	}
	if f.Data != nil {
		sb.WriteString(" data=")
		sb.WriteString(hex.EncodeToString(*f.Data))
	}
	return sb.String()
}
