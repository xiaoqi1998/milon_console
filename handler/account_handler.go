package handler

import (
	"encoding/hex"
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

// listResourcePathResponse is the DTO for ListResourcePathInfo with RsHash converted to hex.
type listResourcePathResponse struct {
	RsHash string `json:"rsHash"`
	Path   string `json:"path"`
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
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "address is required", nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.GetAccount(address, requestId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to get account: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.BodyAccountView, "ok"))
}

// GetAccountResources handles GET /api/accounts/:address/resources
func (h *AccountHandler) GetAccountResources(c *gin.Context) {
	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	result, err := mc.ListResourcePath(requestId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to list resource paths: "+err.Error(), nil))
		return
	}

	list := make([]listResourcePathResponse, 0, len(result.BodyListResourcePaths))
	for _, item := range result.BodyListResourcePaths {
		list = append(list, listResourcePathResponse{
			RsHash: hex.EncodeToString(item.RsHash[:]),
			Path:   item.Path,
		})
	}

	c.JSON(http.StatusOK, types.SuccessResponse(list, "ok"))
}

// GenerateAccount handles POST /api/accounts/generate
func (h *AccountHandler) GenerateAccount(c *gin.Context) {
	sk := crypto.NewClassicalSecretKey()

	classicalSk := crypto.AsClassicalSecretKey(sk)
	if classicalSk == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_INTERNAL, "failed to get classical secret key", nil))
		return
	}

	pk, err := classicalSk.Secp256k1Public()
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to derive public key: "+err.Error(), nil))
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

	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}
