package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// FaucetHandler exposes faucet claim and balance query endpoints.
type FaucetHandler struct {
	nm *client.NetworkManager
}

// NewFaucetHandler creates a FaucetHandler bound to the given NetworkManager.
func NewFaucetHandler(nm *client.NetworkManager) *FaucetHandler {
	return &FaucetHandler{nm: nm}
}

// claimFaucetRequest is the request body for POST /api/faucet/claim.
type claimFaucetRequest struct {
	PrivateKey    string          `json:"privateKey" binding:"required"`
	Address       string          `json:"address" binding:"required"`
	SignatureMode json.RawMessage `json:"signatureMode" binding:"required"`
}

// ClaimFaucet handles POST /api/faucet/claim
// Claims gas tokens from the faucet for the given address.
func (h *FaucetHandler) ClaimFaucet(c *gin.Context) {
	var req claimFaucetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "ClaimFaucet", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	sk, err := types.ParseSecretKey(req.PrivateKey)
	if err != nil {
		logParamError(c, "ClaimFaucet", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid privateKey: "+err.Error(), nil))
		return
	}

	addr, err := types.ParseAddress(req.Address)
	if err != nil {
		logParamError(c, "ClaimFaucet", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid address: "+err.Error(), nil))
		return
	}

	mode, err := types.ParseSignatureModeFromJSON(req.SignatureMode)
	if err != nil {
		logParamError(c, "ClaimFaucet", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid signatureMode: "+err.Error(), nil))
		return
	}

	mc, _ := h.nm.GetCurrent()

	requestId := uint64(time.Now().UnixMilli())
	result, err := mc.BuildAndSubmitSingleIxSplit(
		"token",
		"ClaimFaucet",
		provider.Args{"claimer": addr},
		sk,
		addr,
		mode,
		requestId,
	)
	if err != nil {
		logSDKError(c, "ClaimFaucet", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to claim faucet: "+err.Error(), nil))
		return
	}
	txHash := result.BodyTxHash

	// Wait for the transaction to be confirmed (like the SDK's ClaimFaucet does internally)
	_, err = mc.WaitForTransaction(txHash, 1)
	if err != nil {
		logSDKError(c, "ClaimFaucet", err)
		// Still return the txHash so the caller can track it, but note the wait error
		c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
			"address": req.Address,
			"claimed": false,
			"txHash":  txHash,
			"error":   "transaction submitted but wait failed: " + err.Error(),
		}, "submitted"))
		return
	}

	logBusinessInfo(c, "ClaimFaucet", "address", req.Address, "txHash", txHash)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"address": req.Address,
		"claimed": true,
		"txHash":  txHash,
	}, "ok"))
}

// GetBalance handles GET /api/faucet/balance/:address
// Queries the MIL token balance of the given address.
func (h *FaucetHandler) GetBalance(c *gin.Context) {
	addrStr := c.Param("address")
	if addrStr == "" {
		logParamError(c, "GetBalance", fmt.Errorf("address is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "address is required", nil))
		return
	}

	addr, err := types.ParseAddress(addrStr)
	if err != nil {
		logParamError(c, "GetBalance", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid address: "+err.Error(), nil))
		return
	}

	mc, _ := h.nm.GetCurrent()

	balance, err := mc.AddressBalance(addr)
	if err != nil {
		logSDKError(c, "GetBalance", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get balance: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"address": addrStr, "balance": balance}, "ok"))
}
