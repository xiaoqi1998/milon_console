package handler

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/api"
)

// RpcHandler exposes low-level RPC endpoints (read-only).
type RpcHandler struct {
	nm *client.NetworkManager
}

// NewRpcHandler creates an RpcHandler bound to the given NetworkManager.
func NewRpcHandler(nm *client.NetworkManager) *RpcHandler {
	return &RpcHandler{nm: nm}
}

// typeTagWithDataResponse is shared by multiple handlers in this package.
type typeTagWithDataResponse struct {
	TypeTag uint64 `json:"typeTag"`
	Value   string `json:"value"`
}

type blockResponse struct {
	Number             uint64   `json:"number"`
	Hash               string   `json:"hash"`
	PrevHash           string   `json:"prevHash"`
	Timestamp          uint64   `json:"timestamp"`
	TxProofIdentifiers []string `json:"txProofIdentifiers"`
	WitnessAddress     string   `json:"witnessAddress"`
	WitnessSignature   string   `json:"witnessSignature"`
}

type getResourceResponse struct {
	TypeTag uint64 `json:"typeTag"`
	Value   string `json:"value"`
}

type getAccessValueResponse struct {
	BlobHash string                   `json:"blobHash"`
	Data     *typeTagWithDataResponse `json:"data,omitempty"`
}

type getAccessValueRequest struct {
	BlobHashes []string `json:"blobHashes" binding:"required"`
}

// GetBlock handles GET /api/rpc/blocks/:height
func (h *RpcHandler) GetBlock(c *gin.Context) {
	heightStr := c.Param("height")
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid block height", err.Error()))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.GetBlock(height, requestId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get block: "+err.Error(), nil))
		return
	}

	b := result.BodyBlock
	txProofIds := make([]string, 0, len(b.TxProofIdentifiers))
	for _, id := range b.TxProofIdentifiers {
		txProofIds = append(txProofIds, hex.EncodeToString(id[:]))
	}

	resp := blockResponse{
		Number:             b.Number,
		Hash:               hex.EncodeToString(b.Hash[:]),
		PrevHash:           hex.EncodeToString(b.PrevHash[:]),
		Timestamp:          b.Timestamp,
		TxProofIdentifiers: txProofIds,
		WitnessAddress:     b.WitnessAddress.ToBase58(),
		WitnessSignature:   hex.EncodeToString(b.WitnessSignature[:]),
	}

	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// GetResource handles GET /api/rpc/resources/:hash
func (h *RpcHandler) GetResource(c *gin.Context) {
	hashStr := c.Param("hash")
	hashBytes, err := hex.DecodeString(hashStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid resource hash hex", err.Error()))
		return
	}

	if len(hashBytes) != api.RsHashLen {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid resource hash length: expected "+strconv.Itoa(api.RsHashLen)+", got "+strconv.Itoa(len(hashBytes)), nil))
		return
	}

	var rsHash api.RsHash
	copy(rsHash[:], hashBytes)

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.GetResource(rsHash, requestId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get resource: "+err.Error(), nil))
		return
	}

	gr := result.BodyGetResource
	resp := getResourceResponse{
		TypeTag: gr.Data.TypeTag,
		Value:   hex.EncodeToString(gr.Data.Value),
	}

	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// GetAccessValue handles POST /api/rpc/access-value
func (h *RpcHandler) GetAccessValue(c *gin.Context) {
	var req getAccessValueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	blobHashList := make([]api.BlobHash, 0, len(req.BlobHashes))
	for _, hashStr := range req.BlobHashes {
		hashBytes, err := hex.DecodeString(hashStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid blob hash hex: "+hashStr, err.Error()))
			return
		}
		if len(hashBytes) != api.BlobHashLen {
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid blob hash length for "+hashStr+": expected "+strconv.Itoa(api.BlobHashLen)+", got "+strconv.Itoa(len(hashBytes)), nil))
			return
		}
		var bh api.BlobHash
		copy(bh[:], hashBytes)
		blobHashList = append(blobHashList, bh)
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.GetAccessValue(blobHashList, requestId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get access value: "+err.Error(), nil))
		return
	}

	list := make([]getAccessValueResponse, 0, len(result.BodyGetAccessValues))
	for _, item := range result.BodyGetAccessValues {
		resp := getAccessValueResponse{
			BlobHash: hex.EncodeToString(item.BlobHash[:]),
		}
		if item.Data != nil {
			resp.Data = &typeTagWithDataResponse{
				TypeTag: item.Data.TypeTag,
				Value:   hex.EncodeToString(item.Data.Value),
			}
		}
		list = append(list, resp)
	}

	c.JSON(http.StatusOK, types.SuccessResponse(list, "ok"))
}
