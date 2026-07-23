package handler

import (
	"fmt"
	"net/http"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

// AccountHandler exposes account-related endpoints.
type AccountHandler struct {
	nm *client.NetworkManager
}

// NewAccountHandler creates an AccountHandler bound to the given NetworkManager.
func NewAccountHandler(nm *client.NetworkManager) *AccountHandler {
	return &AccountHandler{nm: nm}
}

// generateAccountResponse is the response for generating a new account.
type generateAccountResponse struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	Address    string `json:"address"`
}

// GetAccount handles GET /api/accounts/:address
func (h *AccountHandler) GetAccount(c *gin.Context) {
	address := c.Param("address")
	if address == "" {
		logParamError(c, "GetAccount", fmt.Errorf("address is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "address is required", nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.GetAccount(address, requestId)
	if err != nil {
		logSDKError(c, "GetAccount", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get account: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.BodyAccountView, "ok"))
}

// GetAccountResources handles GET /api/accounts/:address/resources
func (h *AccountHandler) GetAccountResources(c *gin.Context) {
	address := c.Param("address")
	if address == "" {
		logParamError(c, "GetAccountResources", fmt.Errorf("address is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "address is required", nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.GetAccount(address, requestId)
	if err != nil {
		logSDKError(c, "GetAccountResources", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get account: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.BodyAccountView, "ok"))
}

// generateAccountRequest is the optional request body for GenerateAccount.
type generateAccountRequest struct {
	KeyType string `json:"keyType"`
}

// GenerateAccount handles POST /api/accounts/generate
func (h *AccountHandler) GenerateAccount(c *gin.Context) {
	var req generateAccountRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			logParamError(c, "GenerateAccount", err)
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
			return
		}
	}

	keyType := req.KeyType
	if keyType == "" {
		keyType = "secp256k1"
	}

	var sk crypto.SecretKeyer
	var pk *crypto.PublicKey

	switch keyType {
	case "secp256k1":
		sk = crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		if classicalSk == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_INTERNAL, "failed to get classical secret key", nil))
			return
		}
		var err error
		pk, err = classicalSk.Secp256k1Public()
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to derive public key: "+err.Error(), nil))
			return
		}
	case "ed25519":
		sk = crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		if classicalSk == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_INTERNAL, "failed to get classical secret key", nil))
			return
		}
		pk = classicalSk.Ed25519Public()
		if pk == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_INTERNAL, "failed to derive ed25519 public key", nil))
			return
		}
	case "bls12381":
		sk = crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		if classicalSk == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_INTERNAL, "failed to get classical secret key", nil))
			return
		}
		pk = classicalSk.BLS12381Public()
		if pk == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_INTERNAL, "failed to derive bls12381 public key", nil))
			return
		}
	case "fndsa512":
		var err error
		sk, pk, err = crypto.NewFnDsa512SecretKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to generate fndsa512 key: "+err.Error(), nil))
			return
		}
	default:
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "unsupported keyType: "+keyType+" (supported: secp256k1, ed25519, bls12381, fndsa512)", nil))
		return
	}

	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to derive address: "+err.Error(), nil))
		return
	}

	resp := generateAccountResponse{
		PrivateKey: sk.ToHex(),
		PublicKey:  pk.ToHex(),
		Address:    addr.ToBase58(),
	}

	logBusinessInfo(c, "GenerateAccount", "keyType", keyType, "address", resp.Address)
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}
