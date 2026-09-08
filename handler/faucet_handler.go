package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
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

	tx, err := buildClaimFaucetTx(mc, addr, sk, mode)
	if err != nil {
		logSDKError(c, "ClaimFaucet", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, err.Error(), nil))
		return
	}
	if err := submitClaimFaucetTx(mc, tx); err != nil {
		logSDKError(c, "ClaimFaucet", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to claim faucet: "+err.Error(), nil))
		return
	}
	txHash := txHashHex(tx)

	// Wait for the transaction to be confirmed (like the SDK's ClaimFaucet does internally)
	_, err = mc.WaitForTransaction(txHash, milon.WithWaitRequestID(lib.RequestID(1)))
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

// buildClaimFaucetTx builds the sponsored ClaimFaucet transaction, mirroring SDK
// rpcClientV1.ClaimFaucet: SplitPayerSelfPay with no gas signer (the claimer signs
// only ix bit0), because claim_faucet is sponsor=true in the IDL and the node
// honors it.
//
// Rule: a sponsored-form tx (no gas signer) must go through the sponsored-aware
// paths — ValidateWireWith(sponsorIxes) and SubmitTxWithSponsorIxes(tx, sponsorIxes).
// The non-sponsored paths (ValidateWire / SubmitTx) self-reject it with
// "gas signer required for ix 0".
func buildClaimFaucetTx(mc *milon.Client, addr crypto.Address, sk crypto.SecretKeyer, mode lib.AccountSignatureMode) (*lib.Transaction, error) {
	pd, ok := mc.GetAllPd()["token"]
	if !ok {
		return nil, fmt.Errorf("token IDL not found")
	}
	wire, err := pd.Encode("ClaimFaucet", provider.Args{"claimer": addr})
	if err != nil {
		return nil, fmt.Errorf("failed to encode ClaimFaucet: %w", err)
	}

	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		AddIxesSig(addr, sk, []uint8{0}, false, mode).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create tx: %w", err)
	}
	if err := tx.ValidateWireWith([]uint8{0}); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}
	return tx, nil
}

// submitClaimFaucetTx submits a sponsored ClaimFaucet tx with ix0 marked sponsored.
func submitClaimFaucetTx(mc *milon.Client, tx *lib.Transaction) error {
	return mc.SubmitTxWithSponsorIxes(tx, []uint8{0}, milon.WithRequestID(lib.RequestID(time.Now().UnixMilli())))
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

	balance, err := mc.BalanceOf(&addr)
	if err != nil {
		logSDKError(c, "GetBalance", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get balance: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"address": addrStr, "balance": balance}, "ok"))
}
