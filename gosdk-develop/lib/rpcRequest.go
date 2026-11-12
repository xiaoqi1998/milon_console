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
	MethodTypeChainHead           MethodType = 1
	MethodTypeSubmitTx            MethodType = 5
	MethodTypeBatchSubmitTx       MethodType = 6
	MethodTypeSimulateTx          MethodType = 10
	MethodTypeBatchSimulateTx     MethodType = 11
	MethodTypeView                MethodType = 15
	MethodTypeBatchView           MethodType = 16
	MethodTypeGetAccount          MethodType = 20
	MethodTypeBatchGetAccount     MethodType = 21
	MethodTypeEventsByTxHash      MethodType = 25
	MethodTypeBatchEventsByTxHash MethodType = 26
	MethodTypeNodeCapability      MethodType = 30

	// --- Query block / tx (50-149, step 5)
	MethodTypeGetBlockByHeight       MethodType = 50
	MethodTypeBatchGetBlockByHeight  MethodType = 51
	MethodTypeGetTxByHash            MethodType = 55
	MethodTypeBatchGetTxByHash       MethodType = 56
	MethodTypeGetTxHistoryProof      MethodType = 60
	MethodTypeBatchGetTxHistoryProof MethodType = 61

	// --- Resource raw (150-199, step 5)
	MethodTypeGetResource                MethodType = 150
	MethodTypeBatchGetResource           MethodType = 151
	MethodTypeGetResourcePathByHash      MethodType = 155
	MethodTypeBatchGetResourcePathByHash MethodType = 160
	MethodTypeGetAccessValue             MethodType = 165
	MethodTypeBatchGetAccessValue        MethodType = 166
)

// methodTypeNames maps every known RPC method to its name so error messages
// can identify the method instead of a bare number.
var methodTypeNames = map[MethodType]string{
	MethodTypeChainHead:                  "chain_head",
	MethodTypeSubmitTx:                   "submit_tx",
	MethodTypeBatchSubmitTx:              "batch_submit_tx",
	MethodTypeSimulateTx:                 "simulate_tx",
	MethodTypeBatchSimulateTx:            "batch_simulate_tx",
	MethodTypeView:                       "view",
	MethodTypeBatchView:                  "batch_view",
	MethodTypeGetAccount:                 "get_account",
	MethodTypeBatchGetAccount:            "batch_get_account",
	MethodTypeEventsByTxHash:             "events_by_tx_hash",
	MethodTypeBatchEventsByTxHash:        "batch_events_by_tx_hash",
	MethodTypeNodeCapability:             "node_capability",
	MethodTypeGetBlockByHeight:           "get_block_by_height",
	MethodTypeBatchGetBlockByHeight:      "batch_get_block_by_height",
	MethodTypeGetTxByHash:                "get_tx_by_hash",
	MethodTypeBatchGetTxByHash:           "batch_get_tx_by_hash",
	MethodTypeGetTxHistoryProof:          "get_tx_history_proof",
	MethodTypeBatchGetTxHistoryProof:     "batch_get_tx_history_proof",
	MethodTypeGetResource:                "get_resource",
	MethodTypeBatchGetResource:           "batch_get_resource",
	MethodTypeGetResourcePathByHash:      "get_resource_path_by_hash",
	MethodTypeBatchGetResourcePathByHash: "batch_get_resource_path_by_hash",
	MethodTypeGetAccessValue:             "get_access_value",
	MethodTypeBatchGetAccessValue:        "batch_get_access_value",
}

// String returns the RPC method name, falling back to the numeric value for
// methods that have no registered name.
func (mt MethodType) String() string {
	if name, ok := methodTypeNames[mt]; ok {
		return name
	}
	return fmt.Sprintf("method(%d)", uint16(mt))
}

func NewRpcRequest(method MethodType, requestId RequestID, body []byte) *RpcRequest {
	return &RpcRequest{
		Method:    method,
		RequestId: requestId,
		Body:      body,
	}
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
