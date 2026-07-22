package handler

import (
	"encoding/hex"
	"net/http"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// SystemHandler exposes system-level endpoints.
type SystemHandler struct {
	nm *client.NetworkManager
}

// NewSystemHandler creates a SystemHandler bound to the given NetworkManager.
func NewSystemHandler(nm *client.NetworkManager) *SystemHandler {
	return &SystemHandler{nm: nm}
}

// chainHeadResponse is the DTO for ChainHead with byte fields converted to hex.
type chainHeadResponse struct {
	ChainId        uint64 `json:"chainId"`
	BlockHeight    uint64 `json:"blockHeight"`
	BlockHash      string `json:"blockHash"`
	TimestampMsecs uint64 `json:"timestampMsecs"`
}

// Health handles GET /api/health
func (h *SystemHandler) Health(c *gin.Context) {
	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.GetChainHead(requestId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get chain head: "+err.Error(), nil))
		return
	}

	ch := result.BodyChainHead
	resp := gin.H{
		"ok":          true,
		"chainId":     ch.ChainId,
		"blockHeight": ch.BlockHeight,
		"timestamp":   ch.TimestampMsecs,
	}
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// GetChainHead handles GET /api/chain-head
func (h *SystemHandler) GetChainHead(c *gin.Context) {
	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.GetChainHead(requestId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get chain head: "+err.Error(), nil))
		return
	}

	ch := result.BodyChainHead
	resp := chainHeadResponse{
		ChainId:        ch.ChainId,
		BlockHeight:    ch.BlockHeight,
		BlockHash:      hex.EncodeToString(ch.BlockHash[:]),
		TimestampMsecs: ch.TimestampMsecs,
	}
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}
