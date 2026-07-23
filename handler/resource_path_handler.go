package handler

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/api"
)

// ResourcePathHandler exposes resource path query endpoints.
type ResourcePathHandler struct {
	nm *client.NetworkManager
}

// NewResourcePathHandler creates a ResourcePathHandler bound to the given NetworkManager.
func NewResourcePathHandler(nm *client.NetworkManager) *ResourcePathHandler {
	return &ResourcePathHandler{nm: nm}
}

// getResourcePathResponse is the DTO for a single resource path entry.
type getResourcePathResponse struct {
	RsHash string `json:"rsHash"`
	Path   string `json:"path"`
}

// GetResourcePathByHash handles GET /api/rpc/resource-paths/:hash
// Fetches a single resource path by its 18-byte RsHash.
func (h *ResourcePathHandler) GetResourcePathByHash(c *gin.Context) {
	hashStr := c.Param("hash")
	if hashStr == "" {
		logParamError(c, "GetResourcePathByHash", errors.New("hash is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "hash is required", nil))
		return
	}
	hashBytes, err := hex.DecodeString(hashStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid resource hash hex: "+err.Error(), nil))
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

	result, err := mc.GetResourcePathByHash(rsHash, requestId)
	if err != nil {
		logSDKError(c, "GetResourcePathByHash", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get resource path by hash: "+err.Error(), nil))
		return
	}

	// The SDK returns raw HTTP body bytes; parse as a single-element list.
	var rawList [][]any
	if err := json.Unmarshal(result.HttpRspBody, &rawList); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to parse resource path response: "+err.Error(), nil))
		return
	}

	list, err := api.UnmarshalListResourcePathListFromRawList(rawList)
	if err != nil || len(list) == 0 {
		c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_NOT_FOUND, "resource path not found for hash: "+hashStr, nil))
		return
	}

	item := list[0]
	resp := getResourcePathResponse{
		RsHash: hex.EncodeToString(item.RsHash[:]),
		Path:   item.Path,
	}

	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}
