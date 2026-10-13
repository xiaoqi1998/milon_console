package handler

import (
	"encoding/hex"
	"net/http"
	"time"

	"milon-api-server/client"
	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/lib"
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
	mc := middleware.ClientFrom(c)
	requestId := lib.RequestID(time.Now().UnixMilli())

	result, err := mc.GetChainHead(milon.WithRequestID(requestId))
	if err != nil {
		logSDKError(c, "Health", err)
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

// Version handles GET /api/version —— 自报构建指纹（commit/构建时间/二进制
// SHA256/开源仓库地址），供页面展示开源地址并支持用户本地构建比对核验。
// 不依赖链上网络 client。
func (h *SystemHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, types.SuccessResponse(GetBuildInfo(), "ok"))
}

// GetChainHead handles GET /api/chain-head
func (h *SystemHandler) GetChainHead(c *gin.Context) {
	mc := middleware.ClientFrom(c)
	requestId := lib.RequestID(time.Now().UnixMilli())

	result, err := mc.GetChainHead(milon.WithRequestID(requestId))
	if err != nil {
		logSDKError(c, "GetChainHead", err)
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
