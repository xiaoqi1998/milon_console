package handler

import (
	"net/http"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// NetworkHandler exposes network management endpoints.
type NetworkHandler struct {
	nm *client.NetworkManager
}

// NewNetworkHandler creates a NetworkHandler bound to the given NetworkManager.
func NewNetworkHandler(nm *client.NetworkManager) *NetworkHandler {
	return &NetworkHandler{nm: nm}
}

// ListNetworks handles GET /api/network/list
func (h *NetworkHandler) ListNetworks(c *gin.Context) {
	networks := h.nm.ListNetworks()
	c.JSON(http.StatusOK, types.SuccessResponse(networks, "ok"))
}

// GetCurrentNetwork handles GET /api/network/current
func (h *NetworkHandler) GetCurrentNetwork(c *gin.Context) {
	_, cfg := h.nm.GetCurrent()
	info := client.NetworkInfo{
		Name:    cfg.Name,
		ChainId: cfg.ChainId,
		RpcUrl:  cfg.RpcUrl,
		InxUrl:  cfg.InxUrl,
		Current: true,
	}
	c.JSON(http.StatusOK, types.SuccessResponse(info, "ok"))
}

// SwitchNetwork handles POST /api/network/switch
func (h *NetworkHandler) SwitchNetwork(c *gin.Context) {
	var req types.NetworkSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	if err := h.nm.Switch(req.Network); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_NETWORK_ERROR, err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(nil, "switched to "+req.Network))
}
