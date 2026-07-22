package handler

import (
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
)

// TransactionHandler exposes transaction query endpoints (read-only).
type TransactionHandler struct {
	nm *client.NetworkManager
}

// NewTransactionHandler creates a TransactionHandler bound to the given NetworkManager.
func NewTransactionHandler(nm *client.NetworkManager) *TransactionHandler {
	return &TransactionHandler{nm: nm}
}

// --- DTOs for TxHistory ---

type txHistorySignatureResponse struct {
	Signer  string `json:"signer"`
	AuthBit uint64 `json:"authBit"`
	SigBit  uint64 `json:"sigBit"`
}

type persistedValueResponse struct {
	Variant      uint32 `json:"variant"`
	TypeTag      uint64 `json:"typeTag,omitempty"`
	InlineData   string `json:"inlineData,omitempty"`
	ExternalHash string `json:"externalHash,omitempty"`
}

type accessRecordResponse struct {
	ResourceID    string                  `json:"resourceId"`
	FirstSnapshot *persistedValueResponse `json:"firstSnapshot"`
	LastWritten   *persistedValueResponse `json:"lastWritten"`
}

type txReceiptResponse struct {
	TxID   string                    `json:"txId"`
	TxHash string                    `json:"txHash"`
	State  uint8                     `json:"state"`
	Access []accessRecordResponse    `json:"access"`
	Events []typeTagWithDataResponse `json:"events"`
	Error  *uint16                   `json:"error"`
}

type txHistoryResponse struct {
	Stamp        uint64                       `json:"stamp"`
	Payer        *uint8                       `json:"payer"`
	Signatures   []txHistorySignatureResponse `json:"signatures"`
	Instructions []string                     `json:"instructions"`
	Receipt      txReceiptResponse            `json:"receipt"`
}

// --- DTOs for EventsByTxHash ---

type eventEntryResponse struct {
	BlockHeight uint64                  `json:"blockHeight"`
	TxHash      string                  `json:"txHash"`
	TxIndex     uint32                  `json:"txIndex"`
	EventIndex  uint32                  `json:"eventIndex"`
	Data        typeTagWithDataResponse `json:"data"`
}

type eventsByTxHashResponse struct {
	Events []eventEntryResponse `json:"events"`
}

// toTxHistoryResponse converts an api.TxHistory to a JSON-friendly DTO.
func toTxHistoryResponse(th *api.TxHistory) txHistoryResponse {
	if th == nil {
		return txHistoryResponse{}
	}

	sigs := make([]txHistorySignatureResponse, 0, len(th.Signatures))
	for _, sig := range th.Signatures {
		sigs = append(sigs, txHistorySignatureResponse{
			Signer:  sig.Signer.ToBase58(),
			AuthBit: uint64(sig.AuthBit),
			SigBit:  uint64(sig.SigBit),
		})
	}

	instrs := make([]string, 0, len(th.Instructions))
	for _, instr := range th.Instructions {
		instrs = append(instrs, hex.EncodeToString(instr))
	}

	access := make([]accessRecordResponse, 0, len(th.Receipt.Access))
	for _, rec := range th.Receipt.Access {
		access = append(access, accessRecordResponse{
			ResourceID:    hex.EncodeToString(rec.ResourceID[:]),
			FirstSnapshot: toPersistedValueResponse(rec.FirstSnapshot),
			LastWritten:   toPersistedValueResponse(&rec.LastWritten),
		})
	}

	events := make([]typeTagWithDataResponse, 0, len(th.Receipt.Events))
	for _, ev := range th.Receipt.Events {
		events = append(events, typeTagWithDataResponse{
			TypeTag: ev.TypeTag,
			Value:   hex.EncodeToString(ev.Value),
		})
	}

	return txHistoryResponse{
		Stamp:        th.Stamp,
		Payer:        th.Payer,
		Signatures:   sigs,
		Instructions: instrs,
		Receipt: txReceiptResponse{
			TxID:   hex.EncodeToString(th.Receipt.TxID[:]),
			TxHash: hex.EncodeToString(th.Receipt.TxHash[:]),
			State:  th.Receipt.State,
			Access: access,
			Events: events,
			Error:  th.Receipt.Error,
		},
	}
}

// toPersistedValueResponse converts a *PersistedValue to a JSON-friendly DTO.
func toPersistedValueResponse(pv *api.PersistedValue) *persistedValueResponse {
	if pv == nil {
		return nil
	}
	resp := &persistedValueResponse{
		Variant: pv.Variant,
	}
	switch pv.Variant {
	case 0:
		resp.TypeTag = pv.TypeTag
		resp.InlineData = hex.EncodeToString(pv.InlineData)
	case 1:
		resp.ExternalHash = hex.EncodeToString(pv.ExternalHash[:])
	}
	return resp
}

// toEventsByTxHashResponse converts an api.EventsByTxHash to a JSON-friendly DTO.
func toEventsByTxHashResponse(eb *api.EventsByTxHash) eventsByTxHashResponse {
	if eb == nil {
		return eventsByTxHashResponse{Events: []eventEntryResponse{}}
	}

	events := make([]eventEntryResponse, 0, len(eb.Events))
	for _, entry := range eb.Events {
		events = append(events, eventEntryResponse{
			BlockHeight: entry.BlockHeight,
			TxHash:      hex.EncodeToString(entry.TxHash[:]),
			TxIndex:     entry.TxIndex,
			EventIndex:  entry.EventIndex,
			Data: typeTagWithDataResponse{
				TypeTag: entry.Data.TypeTag,
				Value:   hex.EncodeToString(entry.Data.Value),
			},
		})
	}

	return eventsByTxHashResponse{Events: events}
}

// GetTransactionByHash handles GET /api/transactions/:hash
func (h *TransactionHandler) GetTransactionByHash(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "hash is required", nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.GetTxByHash(hash, requestId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get transaction: "+err.Error(), nil))
		return
	}

	resp := toTxHistoryResponse(result.BodyTxHistory)
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// GetTransactionEvents handles GET /api/transactions/:hash/events
func (h *TransactionHandler) GetTransactionEvents(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "hash is required", nil))
		return
	}

	var typeTagFilter *uint64
	if typeTagStr := c.Query("typeTag"); typeTagStr != "" {
		filter, err := strconv.ParseUint(typeTagStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid typeTag parameter", err.Error()))
			return
		}
		typeTagFilter = &filter
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.EventsByTxHash(hash, typeTagFilter, requestId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get transaction events: "+err.Error(), nil))
		return
	}

	resp := toEventsByTxHashResponse(result.BodyEventsByTxHash)
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// WaitForTransaction handles GET /api/transactions/:hash/wait
func (h *TransactionHandler) WaitForTransaction(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "hash is required", nil))
		return
	}

	var options []any
	if timeoutStr := c.Query("timeoutSecs"); timeoutStr != "" {
		secs, err := strconv.ParseUint(timeoutStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid timeoutSecs parameter", err.Error()))
			return
		}
		options = append(options, milon.PollTimeout(time.Duration(secs)*time.Second))
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.WaitForTransaction(hash, requestId, options...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to wait for transaction: "+err.Error(), nil))
		return
	}

	resp := toTxHistoryResponse(result.BodyTxHistory)
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// rawTransactionRequest is the request body for POST /api/transactions/simulate and /api/transactions/submit.
type rawTransactionRequest struct {
	TransactionPostcard string `json:"transactionPostcard" binding:"required"`
}

// SimulateTransaction handles POST /api/transactions/simulate
func (h *TransactionHandler) SimulateTransaction(c *gin.Context) {

	var req rawTransactionRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))

		return

	}

	postcardBytes, err := base64.StdEncoding.DecodeString(req.TransactionPostcard)

	if err != nil {

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid base64-encoded transactionPostcard", err.Error()))

		return

	}

	mc, _ := h.nm.GetCurrent()

	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.SimulateTx(postcardBytes, requestId)

	if err != nil {

		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to simulate transaction: "+err.Error(), nil))

		return

	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.BodySimulateReceipt, "ok"))
}

// SubmitTransaction handles POST /api/transactions/submit
func (h *TransactionHandler) SubmitTransaction(c *gin.Context) {

	var req rawTransactionRequest

	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))

		return

	}

	postcardBytes, err := base64.StdEncoding.DecodeString(req.TransactionPostcard)

	if err != nil {

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid base64-encoded transactionPostcard", err.Error()))

		return

	}

	mc, _ := h.nm.GetCurrent()

	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.SubmitTx(postcardBytes, requestId)

	if err != nil {

		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to submit transaction: "+err.Error(), nil))

		return

	}

	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"txHash": result.BodyTxHash}, "ok"))
}
