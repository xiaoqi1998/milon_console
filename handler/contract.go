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
	"github.com/milon-labs/milon-go-sdk/provider"
)

// Payment mode constants supported by simulate/write endpoints.
const (
	PaymentModeUnifiedPayerAll     = "unified_payer_all"
	PaymentModeUnifiedDualSign     = "unified_dual_sign"
	PaymentModeUnifiedPayerOnlyGas = "unified_payer_only_gas"
	PaymentModeSplit               = "split"
)

// ContractHandler exposes contract read (view) endpoints.
type ContractHandler struct {
	nm *client.NetworkManager
}

// NewContractHandler creates a ContractHandler bound to the given NetworkManager.
func NewContractHandler(nm *client.NetworkManager) *ContractHandler {
	return &ContractHandler{nm: nm}
}

// readContractRequest is the request body for POST /api/read.
type readContractRequest struct {
	AppName      string        `json:"appName" binding:"required"`
	MethodName   string        `json:"methodName" binding:"required"`
	Args         provider.Args `json:"args"`
	PayerAddress string        `json:"payerAddress"`
}

// ReadContract handles POST /api/read
func (h *ContractHandler) ReadContract(c *gin.Context) {
	var req readContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "ReadContract", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	result, err := mc.BuildAndViewSingleIx(req.AppName, req.MethodName, req.Args, requestId)
	if err != nil {
		logSDKError(c, "ReadContract", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to read contract: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.BodyValues, "ok"))
}

// readContractMultiRequest is the request body for POST /api/read/multi.
type readContractMultiRequest struct {
	Instructions []readContractMultiItem `json:"instructions" binding:"required"`
}

type readContractMultiItem struct {
	AppName    string        `json:"appName" binding:"required"`
	MethodName string        `json:"methodName" binding:"required"`
	Args       provider.Args `json:"args"`
}

// ReadContractMulti handles POST /api/read/multi
// Executes multiple view queries in a single request using BuildAndViewMultiIx.
func (h *ContractHandler) ReadContractMulti(c *gin.Context) {
	var req readContractMultiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "ReadContractMulti", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	if len(req.Instructions) == 0 {
		err := fmt.Errorf("instructions cannot be empty")
		logParamError(c, "ReadContractMulti", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "instructions cannot be empty", nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	// Build wires for each instruction
	wires := make([]api.PackedInstruction, 0, len(req.Instructions))
	for i, ix := range req.Instructions {
		if ix.Args == nil {
			ix.Args = provider.Args{}
		}
		pd, err := mc.GetPdByIDLAppName(ix.AppName)
		if err != nil {
			logParamError(c, "ReadContractMulti", err)
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, fmt.Sprintf("failed to load IDL for app %q (instruction %d): %s", ix.AppName, i, err.Error()), nil))
			return
		}
		wire, err := pd.Encode(ix.MethodName, ix.Args)
		if err != nil {
			logParamError(c, "ReadContractMulti", err)
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, fmt.Sprintf("failed to encode instruction %d (%s.%s): %s", i, ix.AppName, ix.MethodName, err.Error()), nil))
			return
		}
		wires = append(wires, wire)
	}

	result, err := mc.BuildAndViewMultiIx(wires, requestId)
	if err != nil {
		logSDKError(c, "ReadContractMulti", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to read multi: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.HttpRspBody, "ok"))
}

// simulateContractRequest is the request body for POST /api/simulate.
type simulateContractRequest struct {
	AppName       string          `json:"appName" binding:"required"`
	MethodName    string          `json:"methodName" binding:"required"`
	Args          provider.Args   `json:"args"`
	PaymentMode   string          `json:"paymentMode" binding:"required"`
	PayerAddress  string          `json:"payerAddress"`
	SignatureMode json.RawMessage `json:"signatureMode"`
	// Fields for unified_dual_sign mode (optional)
	IxAddress       string          `json:"ixAddress"`
	IxSignatureMode json.RawMessage `json:"ixSignatureMode"`
	// OwnerAddress for split mode (optional, replaces PayerAddress)
	OwnerAddress string `json:"ownerAddress"`
}

// SimulateContract handles POST /api/simulate
// It builds a simulated-signature transaction and runs it against the node's simulate endpoint.
func (h *ContractHandler) SimulateContract(c *gin.Context) {
	var req simulateContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "SimulateContract", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	result, err := h.dispatchSimulate(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "SimulateContract", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to simulate contract: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.BodySimulateReceipt, "ok"))
}

// dispatchSimulate selects the appropriate BuildAndSimulateSingleIx* method based on paymentMode.
func (h *ContractHandler) dispatchSimulate(mc *milon.MolinClient, req *simulateContractRequest, requestId uint64) (*milon.SimulateTransactionResult, error) {
	switch req.PaymentMode {
	case PaymentModeUnifiedPayerAll:
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, err
		}
		return mc.BuildAndSimulateSingleIxUnifiedPayerAll(req.AppName, req.MethodName, req.Args, payerAddr, mode, requestId)

	case PaymentModeUnifiedDualSign:
		payerAddr, payerMode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, err
		}
		ixAddr, ixMode, err := h.parsePayerAndMode(req.IxAddress, req.IxSignatureMode)
		if err != nil {
			return nil, fmt.Errorf("invalid ix fields: %w", err)
		}
		return mc.BuildAndSimulateSingleIxUnifiedDualSign(req.AppName, req.MethodName, req.Args, payerAddr, payerMode, ixAddr, ixMode, requestId)

	case PaymentModeUnifiedPayerOnlyGas:
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, err
		}
		return mc.BuildAndSimulateSingleIxUnifiedPayerOnlyGas(req.AppName, req.MethodName, req.Args, payerAddr, mode, requestId)

	case PaymentModeSplit:
		ownerAddrStr := req.OwnerAddress
		if ownerAddrStr == "" {
			ownerAddrStr = req.PayerAddress
		}
		ownerAddr, mode, err := h.parsePayerAndMode(ownerAddrStr, req.SignatureMode)
		if err != nil {
			return nil, err
		}
		return mc.BuildAnSimulateSingleIxSplit(req.AppName, req.MethodName, req.Args, ownerAddr, mode, requestId)

	default:
		return nil, fmt.Errorf("unsupported paymentMode: %s", req.PaymentMode)
	}
}

// parsePayerAndMode parses address + signatureMode JSON into the SDK types.
func (h *ContractHandler) parsePayerAndMode(addrStr string, sigModeRaw json.RawMessage) (crypto.Address, milon.AccountSignatureMode, error) {
	if addrStr == "" {
		return crypto.Address{}, nil, fmt.Errorf("address is required")
	}
	addr, err := types.ParseAddress(addrStr)
	if err != nil {
		return crypto.Address{}, nil, fmt.Errorf("invalid address: %w", err)
	}
	mode, err := types.ParseSignatureModeFromJSON(sigModeRaw)
	if err != nil {
		return crypto.Address{}, nil, fmt.Errorf("invalid signatureMode: %w", err)
	}
	return addr, mode, nil
}

// writeContractRequest is the request body for POST /api/write.
type writeContractRequest struct {
	AppName         string          `json:"appName" binding:"required"`
	MethodName      string          `json:"methodName" binding:"required"`
	Args            provider.Args   `json:"args"`
	PaymentMode     string          `json:"paymentMode" binding:"required"`
	PayerPrivateKey string          `json:"payerPrivateKey"`
	PayerAddress    string          `json:"payerAddress"`
	SignatureMode   json.RawMessage `json:"signatureMode"`
	// Fields for unified_dual_sign mode (optional)
	IxAddress       string          `json:"ixAddress"`
	IxPrivateKey    string          `json:"ixPrivateKey"`
	IxSignatureMode json.RawMessage `json:"ixSignatureMode"`
	// Owner fields for split mode (optional)
	OwnerPrivateKey string `json:"ownerPrivateKey"`
	OwnerAddress    string `json:"ownerAddress"`
}

// WriteContract handles POST /api/write
// It builds and submits a real signed transaction based on paymentMode.
func (h *ContractHandler) WriteContract(c *gin.Context) {
	var req writeContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "WriteContract", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	result, err := h.dispatchSubmit(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "WriteContract", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to write contract: "+err.Error(), nil))
		return
	}

	logBusinessInfo(c, "WriteContract", "txHash", result.BodyTxHash, "appName", req.AppName, "methodName", req.MethodName)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"txHash": result.BodyTxHash}, "ok"))
}

// WriteContractMultiAgent handles POST /api/write/multi-agent
// Dedicated endpoint for unified_dual_sign mode (payer + ix are different accounts).
func (h *ContractHandler) WriteContractMultiAgent(c *gin.Context) {
	var req writeContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "WriteContractMultiAgent", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	if req.PaymentMode != PaymentModeUnifiedDualSign {
		err := fmt.Errorf("multi-agent endpoint requires paymentMode=unified_dual_sign")
		logParamError(c, "WriteContractMultiAgent", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "multi-agent endpoint requires paymentMode=unified_dual_sign", nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	result, err := h.dispatchSubmit(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "WriteContractMultiAgent", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to write contract: "+err.Error(), nil))
		return
	}

	logBusinessInfo(c, "WriteContractMultiAgent", "txHash", result.BodyTxHash, "appName", req.AppName, "methodName", req.MethodName)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"txHash": result.BodyTxHash}, "ok"))
}

// WriteContractMultisig handles POST /api/write/multisig
// Dedicated endpoint for split mode (owner pays gas + signs ix).
func (h *ContractHandler) WriteContractMultisig(c *gin.Context) {
	var req writeContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "WriteContractMultisig", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	if req.PaymentMode != PaymentModeSplit {
		err := fmt.Errorf("multisig endpoint requires paymentMode=split")
		logParamError(c, "WriteContractMultisig", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "multisig endpoint requires paymentMode=split", nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := uint64(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	result, err := h.dispatchSubmit(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "WriteContractMultisig", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to write contract: "+err.Error(), nil))
		return
	}

	logBusinessInfo(c, "WriteContractMultisig", "txHash", result.BodyTxHash, "appName", req.AppName, "methodName", req.MethodName)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"txHash": result.BodyTxHash}, "ok"))
}

// dispatchSubmit selects the appropriate BuildAndSubmitSingleIx* method based on paymentMode.
func (h *ContractHandler) dispatchSubmit(mc *milon.MolinClient, req *writeContractRequest, requestId uint64) (*milon.SubmitTransactionResult, error) {
	switch req.PaymentMode {
	case PaymentModeUnifiedPayerAll:
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, err
		}
		return mc.BuildAndSubmitSingleIxUnifiedPayerSignAll(req.AppName, req.MethodName, req.Args, payerSk, payerAddr, mode, requestId)

	case PaymentModeUnifiedDualSign:
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, payerMode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, err
		}
		ixSk, err := types.ParseSecretKey(req.IxPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid ixPrivateKey: %w", err)
		}
		ixAddr, ixMode, err := h.parsePayerAndMode(req.IxAddress, req.IxSignatureMode)
		if err != nil {
			return nil, fmt.Errorf("invalid ix fields: %w", err)
		}
		return mc.BuildAndSubmitSingleIxUnifiedDualSign(req.AppName, req.MethodName, req.Args, payerSk, payerAddr, payerMode, ixSk, ixAddr, ixMode, requestId)

	case PaymentModeUnifiedPayerOnlyGas:
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, err
		}
		return mc.BuildAndSubmitSingleIxUnifiedPayerOnlyGas(req.AppName, req.MethodName, req.Args, payerSk, payerAddr, mode, requestId)

	case PaymentModeSplit:
		ownerSkStr := req.OwnerPrivateKey
		if ownerSkStr == "" {
			ownerSkStr = req.PayerPrivateKey
		}
		ownerSk, err := types.ParseSecretKey(ownerSkStr)
		if err != nil {
			return nil, fmt.Errorf("invalid ownerPrivateKey: %w", err)
		}
		ownerAddrStr := req.OwnerAddress
		if ownerAddrStr == "" {
			ownerAddrStr = req.PayerAddress
		}
		ownerAddr, mode, err := h.parsePayerAndMode(ownerAddrStr, req.SignatureMode)
		if err != nil {
			return nil, err
		}
		return mc.BuildAndSubmitSingleIxSplit(req.AppName, req.MethodName, req.Args, ownerSk, ownerAddr, mode, requestId)

	default:
		return nil, fmt.Errorf("unsupported paymentMode: %s", req.PaymentMode)
	}
}
