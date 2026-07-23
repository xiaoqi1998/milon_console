package milon

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

const ContentTypeMilonPostcard = "application/x-milon+postcard"
const ContentTypeMilonJson = "application/x-milon+json"

type MethodType uint16

func (mt MethodType) MarshalPostcard(serializer *postcard.Serializer) error {
	return serializer.SerializeU16(uint16(mt))
}

func (mt *MethodType) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	value, err := deserializer.DeserializeU16()
	if err != nil {
		return fmt.Errorf("failed to deserialize MethodType: %w", err)
	}
	*mt = MethodType(value)
	return nil
}

const (
	MethodTypeChainHead             MethodType = 1
	MethodTypeSubmitTx              MethodType = 2
	MethodTypeSimulateTx            MethodType = 3
	MethodTypeView                  MethodType = 4
	MethodTypeGetResource           MethodType = 5
	MethodTypeGetBlockByHeight      MethodType = 6
	MethodTypeGetTxByHash           MethodType = 7
	MethodTypeGetAccount            MethodType = 8
	MethodTypeEventsByTxHash        MethodType = 9
	MethodTypeListResourcePath      MethodType = 10
	MethodTypeGetResourcePathByHash MethodType = 11
	MethodTypeGetAccessValue        MethodType = 12

	// Deprecated: Use MethodTypeGetAccessValue instead. Kept for backward compatibility.
	MethodGetAccessValue = MethodTypeGetAccessValue
)

func NewSubmitTransaction(method MethodType, requestId uint64, body []byte) *SubmitTransaction {
	tx := &SubmitTransaction{
		Method:    method,
		RequestId: requestId,
		Body:      body,
	}

	return tx
}

type SubmitTransaction struct {
	Method    MethodType
	RequestId uint64
	Body      []byte
}

func (st *SubmitTransaction) MarshalPostcard(serializer *postcard.Serializer) error {
	var err error

	if err = st.Method.MarshalPostcard(serializer); err != nil {
		return fmt.Errorf("failed to serialize Method: %w", err)
	}

	if err = serializer.SerializeU64(st.RequestId); err != nil {
		return fmt.Errorf("failed to serialize RequestId: %w", err)
	}

	if err = serializer.SerializeBytes(st.Body); err != nil {
		return fmt.Errorf("failed to serialize Body: %w", err)
	}

	return nil
}

func (st *SubmitTransaction) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	var err error

	if err = st.Method.UnmarshalPostcard(deserializer); err != nil {
		return fmt.Errorf("failed to deserialize Method: %w", err)
	}

	st.RequestId, err = deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize RequestId: %w", err)
	}

	st.Body, err = deserializer.DeserializeBytes()
	if err != nil {
		return fmt.Errorf("failed to deserialize Body: %w", err)
	}
	if st.Body == nil {
		st.Body = []byte{}
	}

	return nil
}
