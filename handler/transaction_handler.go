package handler

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/provider"
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
	TxID       string                    `json:"txId"`
	TxHash     string                    `json:"txHash"`
	State      uint8                     `json:"state"`
	Access     []accessRecordResponse    `json:"access"`
	Events     []typeTagWithDataResponse `json:"events"`
	Error      *uint16                   `json:"error"`
	GasCharged uint64                    `json:"gasCharged"`
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
			TxID:       hex.EncodeToString(th.Receipt.TxID[:]),
			TxHash:     hex.EncodeToString(th.Receipt.TxHash[:]),
			State:      th.Receipt.State,
			Access:     access,
			Events:     events,
			Error:      th.Receipt.Error,
			GasCharged: th.Receipt.GasCharged,
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

// paramError is a minimal error type used for parameter-validation failures
// that have no underlying error value (e.g. empty path/query params), so they
// can be reported through logParamError without adding new imports.
type paramError string

func (e paramError) Error() string { return string(e) }

// unmarshalTransaction deserializes a postcard-encoded transaction.
func unmarshalTransaction(data []byte) (*lib.Transaction, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (*lib.Transaction, error) {
		tx := &lib.Transaction{}
		if err := tx.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return tx, nil
	}, false)
}

// GetTransactionByHash handles GET /api/transactions/:hash
func (h *TransactionHandler) GetTransactionByHash(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		logParamError(c, "GetTransactionByHash", paramError("hash is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "hash is required", nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := lib.RequestID(time.Now().UnixMilli())

	result, err := mc.GetTxByHash(hash, milon.WithRequestID(requestId))
	if err != nil {
		logSDKError(c, "GetTransactionByHash", err)
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
		logParamError(c, "GetTransactionEvents", paramError("hash is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "hash is required", nil))
		return
	}

	var typeTagFilter *uint64
	if typeTagStr := c.Query("typeTag"); typeTagStr != "" {
		filter, err := strconv.ParseUint(typeTagStr, 10, 64)
		if err != nil {
			logParamError(c, "GetTransactionEvents", err)
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid typeTag parameter", err.Error()))
			return
		}
		typeTagFilter = &filter
	}

	mc, _ := h.nm.GetCurrent()
	requestId := lib.RequestID(time.Now().UnixMilli())

	result, err := mc.EventsByTxHash(hash, typeTagFilter, milon.WithRequestID(requestId))
	if err != nil {
		logSDKError(c, "GetTransactionEvents", err)
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
		logParamError(c, "WaitForTransaction", paramError("hash is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "hash is required", nil))
		return
	}

	var options []milon.WaitOption
	if timeoutStr := c.Query("timeoutSecs"); timeoutStr != "" {
		secs, err := strconv.ParseUint(timeoutStr, 10, 64)
		if err != nil {
			logParamError(c, "WaitForTransaction", err)
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid timeoutSecs parameter", err.Error()))
			return
		}
		options = append(options, milon.WithWaitPollTimeout(time.Duration(secs)*time.Second))
	}

	mc, _ := h.nm.GetCurrent()
	requestId := lib.RequestID(time.Now().UnixMilli())

	waitOptions := append([]milon.WaitOption{milon.WithWaitRequestID(requestId)}, options...)
	result, err := mc.WaitForTransaction(hash, waitOptions...)
	if err != nil {
		logSDKError(c, "WaitForTransaction", err)
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
		logParamError(c, "SimulateTransaction", err)

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))

		return

	}

	postcardBytes, err := base64.StdEncoding.DecodeString(req.TransactionPostcard)

	if err != nil {
		logParamError(c, "SimulateTransaction", err)

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid base64-encoded transactionPostcard", err.Error()))

		return

	}

	tx, err := unmarshalTransaction(postcardBytes)
	if err != nil {
		logParamError(c, "SimulateTransaction", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to parse transaction", err.Error()))
		return
	}

	mc, _ := h.nm.GetCurrent()

	requestId := lib.RequestID(time.Now().UnixMilli())

	result, err := mc.SimulateTx(tx, milon.WithRequestID(requestId))

	if err != nil {
		logSDKError(c, "SimulateTransaction", err)

		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to simulate transaction: "+err.Error(), nil))

		return

	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.BodySimulateReceipt, "ok"))
}

// SubmitTransaction handles POST /api/transactions/submit
func (h *TransactionHandler) SubmitTransaction(c *gin.Context) {

	var req rawTransactionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "SubmitTransaction", err)

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))

		return

	}

	postcardBytes, err := base64.StdEncoding.DecodeString(req.TransactionPostcard)

	if err != nil {
		logParamError(c, "SubmitTransaction", err)

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid base64-encoded transactionPostcard", err.Error()))

		return

	}

	tx, err := unmarshalTransaction(postcardBytes)
	if err != nil {
		logParamError(c, "SubmitTransaction", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to parse transaction", err.Error()))
		return
	}

	mc, _ := h.nm.GetCurrent()

	requestId := lib.RequestID(time.Now().UnixMilli())

	if err := mc.SubmitTx(tx, milon.WithRequestID(requestId)); err != nil {
		logSDKError(c, "SubmitTransaction", err)

		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to submit transaction: "+err.Error(), nil))

		return

	}

	txHash := txHashHex(tx)
	logBusinessInfo(c, "SubmitTransaction", "txHash", txHash)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"txHash": txHash}, "ok"))
}

// InspectTransaction handles POST /api/transactions/inspect
// Parses a base64-encoded postcard transaction and returns its details without submitting.
func (h *TransactionHandler) InspectTransaction(c *gin.Context) {

	var req rawTransactionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "InspectTransaction", err)

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))

		return

	}

	postcardBytes, err := base64.StdEncoding.DecodeString(req.TransactionPostcard)

	if err != nil {
		logParamError(c, "InspectTransaction", err)

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid base64-encoded transactionPostcard", err.Error()))

		return

	}

	tx, err := unmarshalTransaction(postcardBytes)

	if err != nil {
		logParamError(c, "InspectTransaction", err)

		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to parse transaction", err.Error()))

		return

	}

	txHash := tx.TxHash()

	ixHashes := tx.IxHashes()

	ixHashHex := make([]string, 0, len(ixHashes))

	for _, h := range ixHashes {

		ixHashHex = append(ixHashHex, hex.EncodeToString(h[:]))

	}

	payer := ""

	if tx.Payer != nil {

		payer = tx.Payer.ToBase58()

	}

	valid := tx.ValidateWire() == nil

	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"txHash":   hex.EncodeToString(txHash[:]),
		"ixHashes": ixHashHex,
		"payer":    payer,
		"valid":    valid,
	}, "ok"))
}

// --- Parsed (human-readable) TxHistory, mirroring helper.DisplayTxHistory ---

type parsedInstructionResponse struct {
	Index       int            `json:"index"`
	Hex         string         `json:"hex"`
	Decoded     map[string]any `json:"decoded,omitempty"`
	Formatted   string         `json:"formatted,omitempty"`
	DecodeError string         `json:"decodeError,omitempty"`
}

type remoteValueResponse struct {
	TypeTag     uint64 `json:"typeTag,omitempty"`
	DataHex     string `json:"dataHex,omitempty"`
	IDLType     string `json:"idlType,omitempty"`
	Decoded     any    `json:"decoded,omitempty"`
	DecodeError string `json:"decodeError,omitempty"`
}

type parsedSnapshotResponse struct {
	Variant      uint32               `json:"variant"`
	VariantName  string               `json:"variantName"`
	TypeTag      uint64               `json:"typeTag,omitempty"`
	IDLType      string               `json:"idlType,omitempty"`
	DataHex      string               `json:"dataHex,omitempty"`
	ExternalHash string               `json:"externalHash,omitempty"`
	Decoded      any                  `json:"decoded,omitempty"`
	DecodeError  string               `json:"decodeError,omitempty"`
	Current      *remoteValueResponse `json:"current,omitempty"`
	AccessValue  *remoteValueResponse `json:"accessValue,omitempty"`
}

type parsedAccessRecordResponse struct {
	Index         int                     `json:"index"`
	ResourceID    string                  `json:"resourceId"`
	FirstSnapshot *parsedSnapshotResponse `json:"firstSnapshot"`
	LastWritten   *parsedSnapshotResponse `json:"lastWritten"`
}

type parsedEventResponse struct {
	Index       int            `json:"index"`
	TypeTag     uint64         `json:"typeTag"`
	ValueHex    string         `json:"valueHex"`
	Decoded     map[string]any `json:"decoded,omitempty"`
	Formatted   string         `json:"formatted,omitempty"`
	DecodeError string         `json:"decodeError,omitempty"`
}

type parsedTxHistoryResponse struct {
	Tx           txHistoryResponse            `json:"tx"`
	Instructions []parsedInstructionResponse  `json:"instructions"`
	Access       []parsedAccessRecordResponse `json:"access"`
	Events       []parsedEventResponse        `json:"events"`
}

// jsonDecodedValue converts SDK-decoded values into JSON-friendly ones:
// Address/PublicKey become base58 strings, byte slices and fixed-size byte
// arrays become hex strings, big.Int becomes a decimal string, and map keys
// are stringified. Slices and maps are converted recursively.
func jsonDecodedValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case crypto.Address:
		return t.ToBase58()
	case *crypto.Address:
		if t == nil {
			return nil
		}
		return t.ToBase58()
	case crypto.PublicKey:
		return t.ToBase58()
	case *crypto.PublicKey:
		if t == nil {
			return nil
		}
		return t.ToBase58()
	case *big.Int:
		return t.String()
	case []byte:
		return hex.EncodeToString(t)
	case provider.B96:
		return hex.EncodeToString(t[:])
	case provider.B144:
		return hex.EncodeToString(t[:])
	case provider.B160:
		return hex.EncodeToString(t[:])
	case provider.B256:
		return hex.EncodeToString(t[:])
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = jsonDecodedValue(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprintf("%v", jsonDecodedValue(k))] = jsonDecodedValue(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = jsonDecodedValue(val)
		}
		return out
	default:
		return v
	}
}

// findIDLTypeByTypeTag resolves the provider and IDL type registered for a
// resource typeTag across all loaded IDLs.
func findIDLTypeByTypeTag(mc *milon.Client, typeTag uint64) (*provider.Provider, *provider.IDLType) {
	for _, pd := range mc.GetAllPd() {
		if idlType, ok := pd.GetIDLTypeByTypeTag(typeTag); ok {
			return pd, idlType
		}
	}
	return nil, nil
}

// decodeTypedValue decodes data by the IDL type bound to typeTag, returning a
// JSON-friendly value; failures are reported as a string instead of an error
// so a single undecodable field never fails the whole request.
func decodeTypedValue(mc *milon.Client, typeTag uint64, data []byte) (decoded any, idlTypeName string, decodeErr string) {
	pd, idlType := findIDLTypeByTypeTag(mc, typeTag)
	if pd == nil {
		return nil, "", fmt.Sprintf("unknown type_tag %d (no matching IDL loaded)", typeTag)
	}
	value, err := pd.DecodeDataByIDLTypeName(idlType.Name, data)
	if err != nil {
		return nil, idlType.Name, err.Error()
	}
	return jsonDecodedValue(value), idlType.Name, ""
}

// fetchResourceValue reads the current on-chain value of an inline-written
// resource via GetResource (the remote enrichment for LastWritten).
func fetchResourceValue(mc *milon.Client, rsHash api.RsHash, requestId lib.RequestID) *remoteValueResponse {
	result, err := mc.GetResource(rsHash, milon.WithRequestID(requestId))
	if err != nil {
		return &remoteValueResponse{DecodeError: err.Error()}
	}
	data := result.BodyGetResource.Data
	resp := &remoteValueResponse{
		TypeTag: data.TypeTag,
		DataHex: hex.EncodeToString(data.Value),
	}
	resp.Decoded, resp.IDLType, resp.DecodeError = decodeTypedValue(mc, data.TypeTag, data.Value)
	return resp
}

// fetchAccessValue reads the off-chain blob value referenced by an
// external-written resource via GetAccessValue.
func fetchAccessValue(mc *milon.Client, blobHash [32]byte, requestId lib.RequestID) *remoteValueResponse {
	bh := api.BlobHash(blobHash)
	result, err := mc.GetAccessValue([]api.BlobHash{bh}, milon.WithRequestID(requestId))
	if err != nil {
		return &remoteValueResponse{DecodeError: err.Error()}
	}
	for _, item := range result.BodyGetAccessValues {
		if item.Data == nil {
			continue
		}
		resp := &remoteValueResponse{
			TypeTag: item.Data.TypeTag,
			DataHex: hex.EncodeToString(item.Data.Value),
		}
		resp.Decoded, resp.IDLType, resp.DecodeError = decodeTypedValue(mc, item.Data.TypeTag, item.Data.Value)
		return resp
	}
	return &remoteValueResponse{DecodeError: "no access value returned for blob"}
}

// parsePersistedValue decodes a snapshot (inline value by its typeTag, or
// external blob hash). fetchCurrent enables the GetResource remote enrichment
// for inline LastWritten; fetchRemote gates the GetAccessValue call for
// external values.
func parsePersistedValue(mc *milon.Client, pv *api.PersistedValue, rsHash api.RsHash, fetchCurrent bool, remote bool, requestId lib.RequestID) *parsedSnapshotResponse {
	if pv == nil {
		return nil
	}
	resp := &parsedSnapshotResponse{Variant: pv.Variant}
	switch pv.Variant {
	case 0: // inline
		resp.VariantName = "inline"
		resp.TypeTag = pv.TypeTag
		resp.DataHex = hex.EncodeToString(pv.InlineData)
		resp.Decoded, resp.IDLType, resp.DecodeError = decodeTypedValue(mc, pv.TypeTag, pv.InlineData)
		if fetchCurrent {
			resp.Current = fetchResourceValue(mc, rsHash, requestId)
		}
	case 1: // external
		resp.VariantName = "external"
		resp.ExternalHash = hex.EncodeToString(pv.ExternalHash[:])
		if remote {
			resp.AccessValue = fetchAccessValue(mc, pv.ExternalHash, requestId)
		}
	default:
		resp.DecodeError = fmt.Sprintf("unknown variant %d", pv.Variant)
	}
	return resp
}

// GetTransactionByHashParsed handles GET /api/transactions/:hash/parse
// Returns the raw tx history together with an IDL-decoded, human-readable
// breakdown (instructions, access-record snapshots, events), mirroring the
// SDK helper DisplayTxHistory. With ?remote=true the current on-chain value
// of each inline-written resource (and external blob values) is fetched and
// decoded as well.
func (h *TransactionHandler) GetTransactionByHashParsed(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		logParamError(c, "GetTransactionByHashParsed", paramError("hash is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "hash is required", nil))
		return
	}
	remote := c.Query("remote") == "true" || c.Query("remote") == "1"

	mc, _ := h.nm.GetCurrent()
	requestId := lib.RequestID(time.Now().UnixMilli())

	result, err := mc.GetTxByHash(hash, milon.WithRequestID(requestId))
	if err != nil {
		logSDKError(c, "GetTransactionByHashParsed", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get transaction: "+err.Error(), nil))
		return
	}
	th := result.BodyTxHistory

	registry := mc.GetProviderManager()

	instructions := make([]parsedInstructionResponse, 0, len(th.Instructions))
	for i, instr := range th.Instructions {
		item := parsedInstructionResponse{Index: i, Hex: hex.EncodeToString(instr)}
		decoded, err := registry.DecodeInstruction(instr)
		if err != nil {
			item.DecodeError = err.Error()
		} else {
			item.Formatted = registry.FormatDecodedInstruction(decoded)
			if sanitized, ok := jsonDecodedValue(decoded).(map[string]any); ok {
				item.Decoded = sanitized
			}
		}
		instructions = append(instructions, item)
	}

	access := make([]parsedAccessRecordResponse, 0, len(th.Receipt.Access))
	for i, rec := range th.Receipt.Access {
		record := parsedAccessRecordResponse{
			Index:         i,
			ResourceID:    hex.EncodeToString(rec.ResourceID[:]),
			FirstSnapshot: parsePersistedValue(mc, rec.FirstSnapshot, rec.ResourceID, false, remote, requestId),
			LastWritten:   parsePersistedValue(mc, &rec.LastWritten, rec.ResourceID, remote, remote, requestId),
		}
		access = append(access, record)
	}

	events := make([]parsedEventResponse, 0, len(th.Receipt.Events))
	for i, ev := range th.Receipt.Events {
		item := parsedEventResponse{Index: i, TypeTag: ev.TypeTag, ValueHex: hex.EncodeToString(ev.Value)}
		decoded, err := registry.DecodeEventDataByTag(ev.TypeTag, ev.Value)
		if err != nil {
			item.DecodeError = err.Error()
		} else {
			item.Formatted = registry.FormatDecodedEvent(decoded)
			if sanitized, ok := jsonDecodedValue(decoded).(map[string]any); ok {
				item.Decoded = sanitized
			}
		}
		events = append(events, item)
	}

	resp := parsedTxHistoryResponse{
		Tx:           toTxHistoryResponse(th),
		Instructions: instructions,
		Access:       access,
		Events:       events,
	}
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}
