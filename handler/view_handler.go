package handler

import (
	"encoding/base64"
	"net/http"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

// unmarshalWires deserializes the postcard-encoded wires list (count + wire bytes).
func unmarshalWires(data []byte) ([]api.PackedInstruction, error) {
	return postcard.DeserializePostcard(data, func(d *postcard.Deserializer) ([]api.PackedInstruction, error) {
		count, err := d.DeserializeU32()
		if err != nil {
			return nil, err
		}
		wires := make([]api.PackedInstruction, 0, count)
		for i := uint32(0); i < count; i++ {
			wire, err := d.DeserializeBytes()
			if err != nil {
				return nil, err
			}
			wires = append(wires, wire)
		}
		return wires, nil
	}, false)
}

// ViewSingleHandler exposes low-level view endpoints with pre-built postcards.
type ViewSingleHandler struct {
	nm *client.NetworkManager
}

// NewViewSingleHandler creates a ViewSingleHandler bound to the given NetworkManager.
func NewViewSingleHandler(nm *client.NetworkManager) *ViewSingleHandler {
	return &ViewSingleHandler{nm: nm}
}

// rawViewRequest is the request body for POST /api/view/single and /api/view/multi.
type rawViewRequest struct {
	TransactionPostcard string `json:"transactionPostcard" binding:"required"`
}

// ViewSingle handles POST /api/view/single
// Executes a low-level single-instruction view call with a pre-built postcard.
func (h *ViewSingleHandler) ViewSingle(c *gin.Context) {
	var req rawViewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "ViewSingle", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	postcardBytes, err := base64.StdEncoding.DecodeString(req.TransactionPostcard)
	if err != nil {
		logParamError(c, "ViewSingle", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid base64-encoded transactionPostcard: "+err.Error(), nil))
		return
	}

	wires, err := unmarshalWires(postcardBytes)
	if err != nil {
		logParamError(c, "ViewSingle", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to parse wires: "+err.Error(), nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := lib.RequestID(time.Now().UnixMilli())

	result, err := mc.View(wires, requestId)
	if err != nil {
		logSDKError(c, "ViewSingle", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to view single: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.HTTPResponseBody, "ok"))
}

// ViewMulti handles POST /api/view/multi
// Executes a low-level multi-instruction view call with a pre-built postcard.
func (h *ViewSingleHandler) ViewMulti(c *gin.Context) {
	var req rawViewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "ViewMulti", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	postcardBytes, err := base64.StdEncoding.DecodeString(req.TransactionPostcard)
	if err != nil {
		logParamError(c, "ViewMulti", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid base64-encoded transactionPostcard: "+err.Error(), nil))
		return
	}

	wires, err := unmarshalWires(postcardBytes)
	if err != nil {
		logParamError(c, "ViewMulti", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to parse wires: "+err.Error(), nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := lib.RequestID(time.Now().UnixMilli())

	result, err := mc.View(wires, requestId)
	if err != nil {
		logSDKError(c, "ViewMulti", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to view multi: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.HTTPResponseBody, "ok"))
}
