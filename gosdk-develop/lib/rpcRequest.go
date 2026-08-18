package lib

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

const ContentTypeMilonPostcard = "application/x-milon+postcard"
const ContentTypeMilonJson = "application/x-milon+json"

type MethodType uint16

func (mt *MethodType) MarshalPostcard(serializer *postcard.Serializer) error {
	return serializer.SerializeU16(uint16(*mt))
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
	// --- Core chain / execute (1-49, step 5)
	MethodTypeChainHead      MethodType = 1
	MethodTypeSubmitTx       MethodType = 5
	MethodTypeSimulateTx     MethodType = 10
	MethodTypeView           MethodType = 15
	MethodTypeGetAccount     MethodType = 20
	MethodTypeEventsByTxHash MethodType = 25

	// --- Query block / tx (50-149, step 5) -----------------------------
	MethodTypeGetBlockByHeight  MethodType = 50
	MethodTypeGetTxByHash       MethodType = 55
	MethodTypeGetTxHistoryProof MethodType = 60

	// --- Resource raw / batch (150-199, step 5) ------------------------
	MethodTypeGetResource                MethodType = 150
	MethodTypeGetResourcePathByHash      MethodType = 155
	MethodTypeBatchGetResourcePathByHash MethodType = 160
	MethodTypeGetAccessValue             MethodType = 165
)

func NewRpcRequest(method MethodType, requestId RequestID, body []byte) *RpcRequest {
	tx := &RpcRequest{
		Method:    method,
		RequestId: requestId,
		Body:      body,
	}

	return tx
}

type RpcRequest struct {
	Method    MethodType
	RequestId RequestID
	Body      []byte
}

func (req *RpcRequest) MarshalPostcard(serializer *postcard.Serializer) error {
	var err error

	if err = req.Method.MarshalPostcard(serializer); err != nil {
		return fmt.Errorf("failed to serialize Method: %w", err)
	}

	if err = serializer.SerializeU64(uint64(req.RequestId)); err != nil {
		return fmt.Errorf("failed to serialize RequestId: %w", err)
	}

	if err = serializer.SerializeBytes(req.Body); err != nil {
		return fmt.Errorf("failed to serialize Body: %w", err)
	}

	return nil
}

func (req *RpcRequest) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	var err error

	if err = req.Method.UnmarshalPostcard(deserializer); err != nil {
		return fmt.Errorf("failed to deserialize Method: %w", err)
	}

	requestId, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize RequestId: %w", err)
	}
	req.RequestId = RequestID(requestId)

	req.Body, err = deserializer.DeserializeBytes()
	if err != nil {
		return fmt.Errorf("failed to deserialize Body: %w", err)
	}
	if req.Body == nil {
		req.Body = []byte{}
	}

	return nil
}
