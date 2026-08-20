package handler

import (
	"encoding/hex"
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

// Payment mode constants supported by simulate/write endpoints.
const (
	PaymentModeUnifiedPayerAll     = "unified_payer_all"
	PaymentModeUnifiedDualSign     = "unified_dual_sign"
	PaymentModeUnifiedPayerOnlyGas = "unified_payer_only_gas"
	PaymentModeSplit               = "split"
	PaymentModeMultiSigner         = "multi_signer"
	PaymentModeSponsored           = "sponsored"
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
	requestId := lib.RequestID(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	pd, ok := mc.GetAllPd()[req.AppName]
	if !ok {
		logParamError(c, "ReadContract", fmt.Errorf("IDL app %q not found", req.AppName))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, fmt.Sprintf("failed to load IDL for app %q", req.AppName), nil))
		return
	}
	wire, err := pd.Encode(req.MethodName, req.Args)
	if err != nil {
		logParamError(c, "ReadContract", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, fmt.Sprintf("failed to encode instruction: %s", err.Error()), nil))
		return
	}

	result, err := mc.View([]api.PackedInstruction{wire}, milon.WithRequestID(requestId))
	if err != nil {
		logSDKError(c, "ReadContract", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to read contract: "+err.Error(), nil))
		return
	}

	bodyValues, err := pd.DecodeViewData(req.MethodName, result.HTTPResponseBody)
	if err != nil {
		logSDKError(c, "ReadContract", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to decode view values: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(bodyValues, "ok"))
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
// Executes multiple view queries in a single request using View.
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
	requestId := lib.RequestID(time.Now().UnixMilli())

	// Build wires for each instruction
	wires := make([]api.PackedInstruction, 0, len(req.Instructions))
	for i, ix := range req.Instructions {
		if ix.Args == nil {
			ix.Args = provider.Args{}
		}
		pd, ok := mc.GetAllPd()[ix.AppName]
		if !ok {
			logParamError(c, "ReadContractMulti", fmt.Errorf("IDL app %q not found", ix.AppName))
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, fmt.Sprintf("failed to load IDL for app %q (instruction %d)", ix.AppName, i), nil))
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

	result, err := mc.View(wires, milon.WithRequestID(requestId))
	if err != nil {
		logSDKError(c, "ReadContractMulti", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to read multi: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(result.HTTPResponseBody, "ok"))
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
	// Fields for multi_signer mode (optional)
	Signers  []types.SignerEntry `json:"signers"`
	GasPayer *types.SignerEntry  `json:"gasPayer"`
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
	requestId := lib.RequestID(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	if err := validatePaymentModeFields(req.PaymentMode, req.Signers, req.PayerAddress, "", false); err != nil {
		logParamError(c, "SimulateContract", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	result, tx, err := h.dispatchSimulate(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "SimulateContract", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to simulate contract: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"receipt": result.BodySimulateReceipt,
		"rawTx":   serializeTx(tx),
	}, "ok"))
}

// txHashHex converts a transaction's txHash to a hex string.
func txHashHex(tx *lib.Transaction) string {
	hash := tx.TxHash()
	return hex.EncodeToString(hash[:])
}

// rawSignatureEntry is the JSON-friendly view of a single crypto.Signature.
type rawSignatureEntry struct {
	Type  string `json:"type"`  // "ed25519"
	Bytes string `json:"bytes"` // hex-encoded signature bytes
}

// rawAccountSignatureEntry is the JSON-friendly view of an AccountSignature.
type rawAccountSignatureEntry struct {
	AuthBit    string              `json:"authBit"`    // hex of uint64 (e.g. "0x8000000000000001")
	SigBit     string              `json:"sigBit"`     // hex of uint64
	Signatures []rawSignatureEntry `json:"signatures"` // list of signatures
	PubKey     string              `json:"pubKey"`     // hex-encoded public key, empty if multisig mode
}

// rawTransactionSignatureEntry is the JSON-friendly view of a TransactionSignatures.
type rawTransactionSignatureEntry struct {
	Address          string                    `json:"address"` // 0x-prefixed address
	AccountSignature rawAccountSignatureEntry `json:"accountSignature"`
}

// rawTransaction is the JSON-friendly view of a Transaction.
type rawTransaction struct {
	Stamp        int64                       `json:"stamp"`        // unix milliseconds
	Payer        string                      `json:"payer"`        // 0x-prefixed address, empty if nil
	Instructions []string                   `json:"instructions"` // list of hex-encoded postcard bytes
	TxSigs       []rawTransactionSignatureEntry `json:"txSigs"`
	TxHash       string                      `json:"txHash"` // hex-encoded tx hash
}

// serializeTx converts a Transaction to a JSON-friendly map for inspection.
// Returns nil if tx is nil.
func serializeTx(tx *lib.Transaction) *rawTransaction {
	if tx == nil {
		return nil
	}

	payer := ""
	if tx.Payer != nil {
		payer = "0x" + tx.Payer.ToHex()
	}

	instrs := make([]string, 0, len(tx.Instructions))
	for _, ix := range tx.Instructions {
		instrs = append(instrs, "0x"+hex.EncodeToString(ix))
	}

	sigs := make([]rawTransactionSignatureEntry, 0, len(tx.TxSigs))
	for _, ts := range tx.TxSigs {
		entry := rawTransactionSignatureEntry{
			Address: "0x" + ts.Address.ToHex(),
		}
		entry.AccountSignature.AuthBit = fmt.Sprintf("0x%016x", ts.AccountSignature.AuthBit.Raw())
		entry.AccountSignature.SigBit = fmt.Sprintf("0x%016x", ts.AccountSignature.SigBit.Raw())
		for _, sig := range ts.AccountSignature.Signatures {
			entry.AccountSignature.Signatures = append(entry.AccountSignature.Signatures, rawSignatureEntry{
				Type:  "ed25519",
				Bytes: "0x" + hex.EncodeToString(sig.Bytes),
			})
		}
		if ts.AccountSignature.PubKey != nil {
			entry.AccountSignature.PubKey = "0x" + ts.AccountSignature.PubKey.ToHex()
		}
		sigs = append(sigs, entry)
	}

	txHash := tx.TxHash()
	return &rawTransaction{
		Stamp:        int64(tx.Stamp),
		Payer:        payer,
		Instructions: instrs,
		TxSigs:       sigs,
		TxHash:       "0x" + hex.EncodeToString(txHash[:]),
	}
}

// simulateAndReturn wraps SimulateTx to also return the built tx for raw inspection.
func simulateAndReturn(mc *milon.Client, tx *lib.Transaction, requestId lib.RequestID) (*milon.SimulateTxResult, *lib.Transaction, error) {
	result, err := mc.SimulateTx(tx, milon.WithRequestID(requestId))
	if err != nil {
		return nil, nil, err
	}
	return result, tx, nil
}

// submitAndReturn wraps SubmitTx to also return the built tx for raw inspection.
func submitAndReturn(mc *milon.Client, tx *lib.Transaction, requestId lib.RequestID) (*lib.Transaction, error) {
	if err := mc.SubmitTx(tx, milon.WithRequestID(requestId)); err != nil {
		return nil, err
	}
	return tx, nil
}

// validatePaymentModeFields validates payment-mode-specific fields before dispatch.
// For multi_signer: signers must not be empty.
// For split: payerAddress (or ownerAddress) is required; payerPrivateKey is required when requirePayerKey is true.
// For sponsored: payerAddress is required; payerPrivateKey is required when requirePayerKey is true.
func validatePaymentModeFields(paymentMode string, signers []types.SignerEntry, payerAddress, payerPrivateKey string, requirePayerKey bool) error {
	switch paymentMode {
	case PaymentModeMultiSigner:
		if len(signers) == 0 {
			return fmt.Errorf("signers cannot be empty")
		}
	case PaymentModeSplit:
		if payerAddress == "" {
			return fmt.Errorf("payerAddress (or ownerAddress) is required for split mode")
		}
		if requirePayerKey && payerPrivateKey == "" {
			return fmt.Errorf("payerPrivateKey is required for split mode")
		}
	case PaymentModeSponsored:
		if payerAddress == "" {
			return fmt.Errorf("payer is required for sponsored mode")
		}
		if requirePayerKey && payerPrivateKey == "" {
			return fmt.Errorf("payerPrivateKey is required for sponsored mode")
		}
	}
	return nil
}

// dispatchSimulate builds a simulated-signature transaction and runs it against the node's simulate endpoint.
// It selects the appropriate signing strategy based on paymentMode.
// Returns the simulate result and the built transaction (for raw inspection).
func (h *ContractHandler) dispatchSimulate(mc *milon.Client, req *simulateContractRequest, requestId lib.RequestID) (*milon.SimulateTxResult, *lib.Transaction, error) {
	// Load IDL and encode the instruction wire once.
	pd, ok := mc.GetAllPd()[req.AppName]
	if !ok {
		return nil, nil, fmt.Errorf("failed to load IDL: app %q not found", req.AppName)
	}
	wire, err := pd.Encode(req.MethodName, req.Args)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode instruction: %w", err)
	}
	instructions := []api.PackedInstruction{wire}

	switch req.PaymentMode {
	case PaymentModeUnifiedPayerAll:
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}
		builder := lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddSimulateIxAndPayerSig(payerAddr, 0, mode)
		tx, err := builder.Build()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create tx: %w", err)
		}
		if err := tx.ValidateWire(); err != nil {
			return nil, nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		return simulateAndReturn(mc, tx, requestId)

	case PaymentModeUnifiedDualSign:
		payerAddr, payerMode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}
		ixAddr, ixMode, err := h.parsePayerAndMode(req.IxAddress, req.IxSignatureMode)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid ix fields: %w", err)
		}
		builder := lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddSimulatePayerSig(payerAddr, payerMode).
			AddSimulateIxesSig(ixAddr, []uint8{0}, false, ixMode)
		tx, err := builder.Build()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create tx: %w", err)
		}
		if err := tx.ValidateWire(); err != nil {
			return nil, nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		return simulateAndReturn(mc, tx, requestId)

	case PaymentModeUnifiedPayerOnlyGas:
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}
		builder := lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddSimulatePayerSig(payerAddr, mode)
		tx, err := builder.Build()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create tx: %w", err)
		}
		if err := tx.ValidateWire(); err != nil {
			return nil, nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		return simulateAndReturn(mc, tx, requestId)

	case PaymentModeSplit:
		ownerAddrStr := req.OwnerAddress
		if ownerAddrStr == "" {
			ownerAddrStr = req.PayerAddress
		}
		ownerAddr, mode, err := h.parsePayerAndMode(ownerAddrStr, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}
		builder := lib.NewTransactionBuilder(instructions).
			WithPayer(&ownerAddr).
			AddSimulateIxAndPayerSig(ownerAddr, 0, mode)
		tx, err := builder.Build()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create tx: %w", err)
		}
		if err := tx.ValidateWire(); err != nil {
			return nil, nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		return simulateAndReturn(mc, tx, requestId)

	case PaymentModeMultiSigner:
		signerAddrs, _, signerModes, err := types.ParseSignerList(req.Signers, false)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid signers: %w", err)
		}

		var gasPayerAddr *crypto.Address
		var gasPayerMode lib.AccountSignatureMode
		if req.GasPayer != nil {
			addr, mode, err := h.parsePayerAndMode(req.GasPayer.Address, req.GasPayer.SignatureMode)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid gasPayer: %w", err)
			}
			gasPayerAddr = &addr
			gasPayerMode = mode
		}

		tx, err := h.buildMultiSignerSimulateTransaction(mc, req.AppName, req.MethodName, req.Args, signerAddrs, signerModes, gasPayerAddr, gasPayerMode)
		if err != nil {
			return nil, nil, err
		}

		return simulateAndReturn(mc, tx, requestId)

	case PaymentModeSponsored:
		if req.PayerAddress == "" {
			return nil, nil, fmt.Errorf("payer is required for sponsored mode")
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}

		tx, err := h.buildSponsoredSimulateTransaction(mc, req.AppName, req.MethodName, req.Args, payerAddr, mode)
		if err != nil {
			return nil, nil, err
		}

		return simulateAndReturn(mc, tx, requestId)

	default:
		return nil, nil, fmt.Errorf("unsupported paymentMode: %s", req.PaymentMode)
	}
}

// parsePayerAndMode parses address + signatureMode JSON into the SDK types.
func (h *ContractHandler) parsePayerAndMode(addrStr string, sigModeRaw json.RawMessage) (crypto.Address, lib.AccountSignatureMode, error) {
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

// buildMultiSignerTransaction builds a transaction with multiple signers signing the same ix (bit0).
// If gasPayer is provided, it signs bit63 (gas) and all signers sign bit0.
// If gasPayer is nil, signers[0] signs bit63+bit0 (via AddIxAndPayerSig) and the rest sign bit0 only.
func (h *ContractHandler) buildMultiSignerTransaction(mc *milon.Client, appName, methodName string, args provider.Args, signerAddrs []crypto.Address, signerSks []crypto.SecretKeyer, signerModes []lib.AccountSignatureMode, gasPayerAddr *crypto.Address, gasPayerSk crypto.SecretKeyer, gasPayerMode lib.AccountSignatureMode) (*lib.Transaction, error) {
	pd, ok := mc.GetAllPd()[appName]
	if !ok {
		return nil, fmt.Errorf("failed to load IDL: app %q not found", appName)
	}

	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire})

	if gasPayerAddr != nil {
		// gasPayer signs bit63 only
		builder.WithPayer(gasPayerAddr).
			AddPayerSig(*gasPayerAddr, gasPayerSk, gasPayerMode)
		// All signers sign bit0
		for i := range signerAddrs {
			builder.AddIxesSig(signerAddrs[i], signerSks[i], []uint8{0}, false, signerModes[i])
		}
	} else {
		// No gasPayer: signers[0] signs bit63+bit0, rest sign bit0
		builder.WithPayer(&signerAddrs[0]).
			AddIxAndPayerSig(signerAddrs[0], signerSks[0], 0, signerModes[0])
		for i := 1; i < len(signerAddrs); i++ {
			builder.AddIxesSig(signerAddrs[i], signerSks[i], []uint8{0}, false, signerModes[i])
		}
	}

	tx, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create tx: %w", err)
	}

	if err := tx.ValidateWire(); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	return tx, nil
}

// buildMultiSignerSimulateTransaction builds a simulated-signature transaction for multi_signer mode.
// Same as buildMultiSignerTransaction but uses AddSimulate* methods (no private keys needed).
func (h *ContractHandler) buildMultiSignerSimulateTransaction(mc *milon.Client, appName, methodName string, args provider.Args, signerAddrs []crypto.Address, signerModes []lib.AccountSignatureMode, gasPayerAddr *crypto.Address, gasPayerMode lib.AccountSignatureMode) (*lib.Transaction, error) {
	pd, ok := mc.GetAllPd()[appName]
	if !ok {
		return nil, fmt.Errorf("failed to load IDL: app %q not found", appName)
	}

	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire})

	if gasPayerAddr != nil {
		builder.WithPayer(gasPayerAddr).
			AddSimulatePayerSig(*gasPayerAddr, gasPayerMode)
		for i := range signerAddrs {
			builder.AddSimulateIxesSig(signerAddrs[i], []uint8{0}, false, signerModes[i])
		}
	} else {
		builder.WithPayer(&signerAddrs[0]).
			AddSimulateIxAndPayerSig(signerAddrs[0], 0, signerModes[0])
		for i := 1; i < len(signerAddrs); i++ {
			builder.AddSimulateIxesSig(signerAddrs[i], []uint8{0}, false, signerModes[i])
		}
	}

	tx, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create tx: %w", err)
	}

	if err := tx.ValidateWire(); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	return tx, nil
}

// buildSponsoredTransaction builds a unified-mode transaction with sponsored ix validation.
// The payer signs bit63 (gas), and ix=0 is marked as sponsored (gas paid by sponsor pool).
func (h *ContractHandler) buildSponsoredTransaction(mc *milon.Client, appName, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddr crypto.Address, mode lib.AccountSignatureMode) (*lib.Transaction, error) {
	pd, ok := mc.GetAllPd()[appName]
	if !ok {
		return nil, fmt.Errorf("failed to load IDL: app %q not found", appName)
	}

	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(&payerAddr).
		AddPayerSig(payerAddr, payerSk, mode)

	tx, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create tx: %w", err)
	}

	if err := tx.ValidateWireWith([]uint8{0}); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	return tx, nil
}

// buildSponsoredSimulateTransaction builds a simulated-signature sponsored transaction.
func (h *ContractHandler) buildSponsoredSimulateTransaction(mc *milon.Client, appName, methodName string, args provider.Args, payerAddr crypto.Address, mode lib.AccountSignatureMode) (*lib.Transaction, error) {
	pd, ok := mc.GetAllPd()[appName]
	if !ok {
		return nil, fmt.Errorf("failed to load IDL: app %q not found", appName)
	}

	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(&payerAddr).
		AddSimulatePayerSig(payerAddr, mode)

	tx, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create tx: %w", err)
	}

	if err := tx.ValidateWireWith([]uint8{0}); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	return tx, nil
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
	// Fields for multi_signer mode (optional)
	Signers  []types.SignerEntry `json:"signers"`
	GasPayer *types.SignerEntry  `json:"gasPayer"`
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
	requestId := lib.RequestID(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	if err := validatePaymentModeFields(req.PaymentMode, req.Signers, req.PayerAddress, req.PayerPrivateKey, true); err != nil {
		logParamError(c, "WriteContract", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	txHash, tx, err := h.dispatchSubmit(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "WriteContract", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to write contract: "+err.Error(), nil))
		return
	}

	logBusinessInfo(c, "WriteContract", "txHash", txHash, "appName", req.AppName, "methodName", req.MethodName)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"txHash": txHash, "rawTx": serializeTx(tx)}, "ok"))
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
	requestId := lib.RequestID(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	txHash, tx, err := h.dispatchSubmit(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "WriteContractMultiAgent", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to write contract: "+err.Error(), nil))
		return
	}

	logBusinessInfo(c, "WriteContractMultiAgent", "txHash", txHash, "appName", req.AppName, "methodName", req.MethodName)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"txHash": txHash, "rawTx": serializeTx(tx)}, "ok"))
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
	requestId := lib.RequestID(time.Now().UnixMilli())

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	txHash, tx, err := h.dispatchSubmit(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "WriteContractMultisig", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to write contract: "+err.Error(), nil))
		return
	}

	logBusinessInfo(c, "WriteContractMultisig", "txHash", txHash, "appName", req.AppName, "methodName", req.MethodName)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"txHash": txHash, "rawTx": serializeTx(tx)}, "ok"))
}

// dispatchSubmit builds a fully-signed transaction based on paymentMode, submits it,
// and returns the hex-encoded transaction hash along with the built transaction (for raw inspection).
func (h *ContractHandler) dispatchSubmit(mc *milon.Client, req *writeContractRequest, requestId lib.RequestID) (string, *lib.Transaction, error) {
	// Load IDL and encode the instruction wire once.
	pd, ok := mc.GetAllPd()[req.AppName]
	if !ok {
		return "", nil, fmt.Errorf("failed to load IDL: app %q not found", req.AppName)
	}
	wire, err := pd.Encode(req.MethodName, req.Args)
	if err != nil {
		return "", nil, fmt.Errorf("failed to encode instruction: %w", err)
	}
	instructions := []api.PackedInstruction{wire}

	buildTx := func(build func() (*lib.Transaction, error)) (*lib.Transaction, error) {
		tx, err := build()
		if err != nil {
			return nil, err
		}
		if err := mc.SubmitTx(tx, milon.WithRequestID(requestId)); err != nil {
			return nil, err
		}
		return tx, nil
	}

	switch req.PaymentMode {
	case PaymentModeUnifiedPayerAll:
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}
		tx, err := buildTx(func() (*lib.Transaction, error) {
			tx, err := lib.NewTransactionBuilder(instructions).
				WithPayer(&payerAddr).
				AddIxAndPayerSig(payerAddr, payerSk, 0, mode).
				Build()
			if err != nil {
				return nil, fmt.Errorf("failed to create tx: %w", err)
			}
			return tx, nil
		})
		if err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeUnifiedDualSign:
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, payerMode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}
		ixSk, err := types.ParseSecretKey(req.IxPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid ixPrivateKey: %w", err)
		}
		ixAddr, ixMode, err := h.parsePayerAndMode(req.IxAddress, req.IxSignatureMode)
		if err != nil {
			return "", nil, fmt.Errorf("invalid ix fields: %w", err)
		}
		tx, err := buildTx(func() (*lib.Transaction, error) {
			tx, err := lib.NewTransactionBuilder(instructions).
				WithPayer(&payerAddr).
				AddPayerSig(payerAddr, payerSk, payerMode).
				AddIxesSig(ixAddr, ixSk, []uint8{0}, false, ixMode).
				Build()
			if err != nil {
				return nil, fmt.Errorf("failed to create tx: %w", err)
			}
			return tx, nil
		})
		if err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeUnifiedPayerOnlyGas:
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}
		tx, err := buildTx(func() (*lib.Transaction, error) {
			tx, err := lib.NewTransactionBuilder(instructions).
				WithPayer(&payerAddr).
				AddPayerSig(payerAddr, payerSk, mode).
				Build()
			if err != nil {
				return nil, fmt.Errorf("failed to create tx: %w", err)
			}
			return tx, nil
		})
		if err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeSplit:
		ownerSkStr := req.OwnerPrivateKey
		if ownerSkStr == "" {
			ownerSkStr = req.PayerPrivateKey
		}
		ownerSk, err := types.ParseSecretKey(ownerSkStr)
		if err != nil {
			return "", nil, fmt.Errorf("invalid ownerPrivateKey: %w", err)
		}
		ownerAddrStr := req.OwnerAddress
		if ownerAddrStr == "" {
			ownerAddrStr = req.PayerAddress
		}
		ownerAddr, mode, err := h.parsePayerAndMode(ownerAddrStr, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}
		tx, err := buildTx(func() (*lib.Transaction, error) {
			tx, err := lib.NewTransactionBuilder(instructions).
				WithPayer(&ownerAddr).
				AddIxAndPayerSig(ownerAddr, ownerSk, 0, mode).
				Build()
			if err != nil {
				return nil, fmt.Errorf("failed to create tx: %w", err)
			}
			return tx, nil
		})
		if err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeMultiSigner:
		signerAddrs, signerSks, signerModes, err := types.ParseSignerList(req.Signers, true)
		if err != nil {
			return "", nil, fmt.Errorf("invalid signers: %w", err)
		}

		var gasPayerAddr *crypto.Address
		var gasPayerSk crypto.SecretKeyer
		var gasPayerMode lib.AccountSignatureMode
		if req.GasPayer != nil {
			addr, mode, err := h.parsePayerAndMode(req.GasPayer.Address, req.GasPayer.SignatureMode)
			if err != nil {
				return "", nil, fmt.Errorf("invalid gasPayer: %w", err)
			}
			sk, err := types.ParseSecretKey(req.GasPayer.PrivateKey)
			if err != nil {
				return "", nil, fmt.Errorf("invalid gasPayer privateKey: %w", err)
			}
			gasPayerAddr = &addr
			gasPayerSk = sk
			gasPayerMode = mode
		}

		tx, err := h.buildMultiSignerTransaction(mc, req.AppName, req.MethodName, req.Args, signerAddrs, signerSks, signerModes, gasPayerAddr, gasPayerSk, gasPayerMode)
		if err != nil {
			return "", nil, err
		}

		if err := mc.SubmitTx(tx, milon.WithRequestID(requestId)); err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeSponsored:
		if req.PayerAddress == "" {
			return "", nil, fmt.Errorf("payer is required for sponsored mode")
		}
		if req.PayerPrivateKey == "" {
			return "", nil, fmt.Errorf("payerPrivateKey is required for sponsored mode")
		}
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}

		tx, err := h.buildSponsoredTransaction(mc, req.AppName, req.MethodName, req.Args, payerSk, payerAddr, mode)
		if err != nil {
			return "", nil, err
		}

		if err := mc.SubmitTx(tx, milon.WithRequestID(requestId)); err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	default:
		return "", nil, fmt.Errorf("unsupported paymentMode: %s", req.PaymentMode)
	}
}

// ==================== 多指令打包（multi-instruction）====================
// 与单指令 write/simulate 的唯一区别：instructions 是数组，
// 每条指令分别 Encode 为 PackedInstruction，再一起塞进
// lib.NewTransactionBuilder，打包成单笔交易原子上链（参考 multi_ix_demo）。

// multiInstructionItem 描述一条待打包的指令（app 方法 + 参数）。
type multiInstructionItem struct {
	AppName    string        `json:"appName" binding:"required"`
	MethodName string        `json:"methodName" binding:"required"`
	Args       provider.Args `json:"args"`
}

// multiContractRequest 是 POST /api/write/multi 与 /api/simulate/multi 的请求体。
type multiContractRequest struct {
	Instructions []multiInstructionItem `json:"instructions" binding:"required"`
	PaymentMode  string                 `json:"paymentMode" binding:"required"`
	// unified_payer_all / unified_payer_only_gas / sponsored：payer 账户
	PayerPrivateKey string          `json:"payerPrivateKey"`
	PayerAddress    string          `json:"payerAddress"`
	SignatureMode   json.RawMessage `json:"signatureMode"`
	// unified_dual_sign：ix 执行账户（签所有指令）
	IxAddress       string          `json:"ixAddress"`
	IxPrivateKey    string          `json:"ixPrivateKey"`
	IxSignatureMode json.RawMessage `json:"ixSignatureMode"`
	// split：owner 账户（可选，缺省取 PayerAddress）
	OwnerPrivateKey string `json:"ownerPrivateKey"`
	OwnerAddress    string `json:"ownerAddress"`
	// multi_signer：多个签名账户
	Signers  []types.SignerEntry `json:"signers"`
	GasPayer *types.SignerEntry  `json:"gasPayer"`
}

// encodeMultiInstructions 将多条指令分别编码为 PackedInstruction，下标即指令序号。
func (h *ContractHandler) encodeMultiInstructions(mc *milon.Client, items []multiInstructionItem) ([]api.PackedInstruction, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("instructions cannot be empty")
	}
	wires := make([]api.PackedInstruction, 0, len(items))
	for i, ix := range items {
		if ix.Args == nil {
			ix.Args = provider.Args{}
		}
		pd, ok := mc.GetAllPd()[ix.AppName]
		if !ok {
			return nil, fmt.Errorf("failed to load IDL: app %q not found (instruction %d)", ix.AppName, i)
		}
		wire, err := pd.Encode(ix.MethodName, ix.Args)
		if err != nil {
			return nil, fmt.Errorf("failed to encode instruction %d (%s.%s): %w", i, ix.AppName, ix.MethodName, err)
		}
		wires = append(wires, wire)
	}
	return wires, nil
}

// allIxIndices 返回 [0, 1, ..., n-1]，即全部指令的下标集合，供签名时声明授权范围。
func allIxIndices(n int) []uint8 {
	idx := make([]uint8, n)
	for i := range idx {
		idx[i] = uint8(i)
	}
	return idx
}

// SimulateContractMulti 处理 POST /api/simulate/multi。
// 一次请求携带多条指令，打包成单笔交易做链上模拟（dry-run），不消耗 gas。
func (h *ContractHandler) SimulateContractMulti(c *gin.Context) {
	var req multiContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "SimulateContractMulti", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := lib.RequestID(time.Now().UnixMilli())

	if err := validatePaymentModeFields(req.PaymentMode, req.Signers, req.PayerAddress, "", false); err != nil {
		logParamError(c, "SimulateContractMulti", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	result, tx, err := h.dispatchSimulateMulti(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "SimulateContractMulti", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to simulate multi contract: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"receipt": result.BodySimulateReceipt,
		"rawTx":   serializeTx(tx),
	}, "ok"))
}

// dispatchSimulateMulti 根据 paymentMode 构建多指令模拟签名交易并执行 simulate。
func (h *ContractHandler) dispatchSimulateMulti(mc *milon.Client, req *multiContractRequest, requestId lib.RequestID) (*milon.SimulateTxResult, *lib.Transaction, error) {
	instructions, err := h.encodeMultiInstructions(mc, req.Instructions)
	if err != nil {
		return nil, nil, err
	}
	allIdx := allIxIndices(len(instructions))

	// 统一入口：Build -> ValidateWire -> SimulateTx
	build := func(tx *lib.Transaction, err error) (*milon.SimulateTxResult, *lib.Transaction, error) {
		if err != nil {
			return nil, nil, err
		}
		if err := tx.ValidateWire(); err != nil {
			return nil, nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		return simulateAndReturn(mc, tx, requestId)
	}

	switch req.PaymentMode {
	case PaymentModeUnifiedPayerAll:
		// payer 签全部指令 + gas（bit63）
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}
		return build(lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddSimulateIxesSig(payerAddr, allIdx, true, mode).
			Build())

	case PaymentModeUnifiedDualSign:
		// payer 只签 gas，ix 账户签全部指令
		payerAddr, payerMode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}
		ixAddr, ixMode, err := h.parsePayerAndMode(req.IxAddress, req.IxSignatureMode)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid ix fields: %w", err)
		}
		return build(lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddSimulatePayerSig(payerAddr, payerMode).
			AddSimulateIxesSig(ixAddr, allIdx, false, ixMode).
			Build())

	case PaymentModeUnifiedPayerOnlyGas:
		// payer 只签 gas（指令无签名要求）
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}
		return build(lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddSimulatePayerSig(payerAddr, mode).
			Build())

	case PaymentModeSplit:
		// owner 签全部指令 + gas
		ownerAddrStr := req.OwnerAddress
		if ownerAddrStr == "" {
			ownerAddrStr = req.PayerAddress
		}
		ownerAddr, mode, err := h.parsePayerAndMode(ownerAddrStr, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}
		return build(lib.NewTransactionBuilder(instructions).
			WithPayer(&ownerAddr).
			AddSimulateIxesSig(ownerAddr, allIdx, true, mode).
			Build())

	case PaymentModeMultiSigner:
		signerAddrs, _, signerModes, err := types.ParseSignerList(req.Signers, false)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid signers: %w", err)
		}
		builder := lib.NewTransactionBuilder(instructions)
		if req.GasPayer != nil {
			gasAddr, gasMode, err := h.parsePayerAndMode(req.GasPayer.Address, req.GasPayer.SignatureMode)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid gasPayer: %w", err)
			}
			builder.WithPayer(&gasAddr).
				AddSimulatePayerSig(gasAddr, gasMode)
			for i := range signerAddrs {
				builder.AddSimulateIxesSig(signerAddrs[i], allIdx, false, signerModes[i])
			}
		} else {
			// 无 gasPayer：signers[0] 签全部指令 + gas，其余签全部指令
			builder.WithPayer(&signerAddrs[0]).
				AddSimulateIxesSig(signerAddrs[0], allIdx, true, signerModes[0])
			for i := 1; i < len(signerAddrs); i++ {
				builder.AddSimulateIxesSig(signerAddrs[i], allIdx, false, signerModes[i])
			}
		}
		return build(builder.Build())

	case PaymentModeSponsored:
		// payer 只签 gas，全部指令由 sponsor 池代付
		if req.PayerAddress == "" {
			return nil, nil, fmt.Errorf("payer is required for sponsored mode")
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return nil, nil, err
		}
		tx, err := lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddSimulatePayerSig(payerAddr, mode).
			Build()
		if err != nil {
			return nil, nil, err
		}
		if err := tx.ValidateWireWith(allIdx); err != nil {
			return nil, nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		return simulateAndReturn(mc, tx, requestId)

	default:
		return nil, nil, fmt.Errorf("unsupported paymentMode: %s", req.PaymentMode)
	}
}

// WriteContractMulti 处理 POST /api/write/multi。
// 一次请求携带多条指令，打包成单笔交易签名后提交上链（原子执行，全部成功或全部失败）。
func (h *ContractHandler) WriteContractMulti(c *gin.Context) {
	var req multiContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "WriteContractMulti", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := lib.RequestID(time.Now().UnixMilli())

	if err := validatePaymentModeFields(req.PaymentMode, req.Signers, req.PayerAddress, req.PayerPrivateKey, true); err != nil {
		logParamError(c, "WriteContractMulti", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	txHash, tx, err := h.dispatchSubmitMulti(mc, &req, requestId)
	if err != nil {
		logSDKError(c, "WriteContractMulti", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to write multi contract: "+err.Error(), nil))
		return
	}

	logBusinessInfo(c, "WriteContractMulti", "txHash", txHash, "instructionCount", len(req.Instructions))
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"txHash": txHash, "rawTx": serializeTx(tx)}, "ok"))
}

// dispatchSubmitMulti 构建多指令签名交易并提交上链，返回交易哈希。
func (h *ContractHandler) dispatchSubmitMulti(mc *milon.Client, req *multiContractRequest, requestId lib.RequestID) (string, *lib.Transaction, error) {
	instructions, err := h.encodeMultiInstructions(mc, req.Instructions)
	if err != nil {
		return "", nil, err
	}
	allIdx := allIxIndices(len(instructions))

	buildTx := func(build func() (*lib.Transaction, error)) (*lib.Transaction, error) {
		tx, err := build()
		if err != nil {
			return nil, err
		}
		if err := mc.SubmitTx(tx, milon.WithRequestID(requestId)); err != nil {
			return nil, err
		}
		return tx, nil
	}

	switch req.PaymentMode {
	case PaymentModeUnifiedPayerAll:
		// payer 签全部指令 + gas（bit63）
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}
		tx, err := buildTx(func() (*lib.Transaction, error) {
			return lib.NewTransactionBuilder(instructions).
				WithPayer(&payerAddr).
				AddIxesSig(payerAddr, payerSk, allIdx, true, mode).
				Build()
		})
		if err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeUnifiedDualSign:
		// payer 只签 gas，ix 账户签全部指令
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, payerMode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}
		ixSk, err := types.ParseSecretKey(req.IxPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid ixPrivateKey: %w", err)
		}
		ixAddr, ixMode, err := h.parsePayerAndMode(req.IxAddress, req.IxSignatureMode)
		if err != nil {
			return "", nil, fmt.Errorf("invalid ix fields: %w", err)
		}
		tx, err := buildTx(func() (*lib.Transaction, error) {
			return lib.NewTransactionBuilder(instructions).
				WithPayer(&payerAddr).
				AddPayerSig(payerAddr, payerSk, payerMode).
				AddIxesSig(ixAddr, ixSk, allIdx, false, ixMode).
				Build()
		})
		if err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeUnifiedPayerOnlyGas:
		// payer 只签 gas
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}
		tx, err := buildTx(func() (*lib.Transaction, error) {
			return lib.NewTransactionBuilder(instructions).
				WithPayer(&payerAddr).
				AddPayerSig(payerAddr, payerSk, mode).
				Build()
		})
		if err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeSplit:
		// owner 签全部指令 + gas
		ownerSkStr := req.OwnerPrivateKey
		if ownerSkStr == "" {
			ownerSkStr = req.PayerPrivateKey
		}
		ownerSk, err := types.ParseSecretKey(ownerSkStr)
		if err != nil {
			return "", nil, fmt.Errorf("invalid ownerPrivateKey: %w", err)
		}
		ownerAddrStr := req.OwnerAddress
		if ownerAddrStr == "" {
			ownerAddrStr = req.PayerAddress
		}
		ownerAddr, mode, err := h.parsePayerAndMode(ownerAddrStr, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}
		tx, err := buildTx(func() (*lib.Transaction, error) {
			return lib.NewTransactionBuilder(instructions).
				WithPayer(&ownerAddr).
				AddIxesSig(ownerAddr, ownerSk, allIdx, true, mode).
				Build()
		})
		if err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeMultiSigner:
		signerAddrs, signerSks, signerModes, err := types.ParseSignerList(req.Signers, true)
		if err != nil {
			return "", nil, fmt.Errorf("invalid signers: %w", err)
		}

		var gasPayerAddr *crypto.Address
		var gasPayerSk crypto.SecretKeyer
		var gasPayerMode lib.AccountSignatureMode
		if req.GasPayer != nil {
			addr, mode, err := h.parsePayerAndMode(req.GasPayer.Address, req.GasPayer.SignatureMode)
			if err != nil {
				return "", nil, fmt.Errorf("invalid gasPayer: %w", err)
			}
			sk, err := types.ParseSecretKey(req.GasPayer.PrivateKey)
			if err != nil {
				return "", nil, fmt.Errorf("invalid gasPayer privateKey: %w", err)
			}
			gasPayerAddr = &addr
			gasPayerSk = sk
			gasPayerMode = mode
		}

		builder := lib.NewTransactionBuilder(instructions)
		if gasPayerAddr != nil {
			builder.WithPayer(gasPayerAddr).
				AddPayerSig(*gasPayerAddr, gasPayerSk, gasPayerMode)
			for i := range signerAddrs {
				builder.AddIxesSig(signerAddrs[i], signerSks[i], allIdx, false, signerModes[i])
			}
		} else {
			builder.WithPayer(&signerAddrs[0]).
				AddIxesSig(signerAddrs[0], signerSks[0], allIdx, true, signerModes[0])
			for i := 1; i < len(signerAddrs); i++ {
				builder.AddIxesSig(signerAddrs[i], signerSks[i], allIdx, false, signerModes[i])
			}
		}

		tx, err := builder.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to create tx: %w", err)
		}
		if err := mc.SubmitTx(tx, milon.WithRequestID(requestId)); err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	case PaymentModeSponsored:
		// payer 只签 gas，全部指令由 sponsor 池代付
		if req.PayerAddress == "" {
			return "", nil, fmt.Errorf("payer is required for sponsored mode")
		}
		if req.PayerPrivateKey == "" {
			return "", nil, fmt.Errorf("payerPrivateKey is required for sponsored mode")
		}
		payerSk, err := types.ParseSecretKey(req.PayerPrivateKey)
		if err != nil {
			return "", nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		payerAddr, mode, err := h.parsePayerAndMode(req.PayerAddress, req.SignatureMode)
		if err != nil {
			return "", nil, err
		}

		builder := lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddPayerSig(payerAddr, payerSk, mode)
		tx, err := builder.Build()
		if err != nil {
			return "", nil, fmt.Errorf("failed to create tx: %w", err)
		}
		if err := tx.ValidateWireWith(allIdx); err != nil {
			return "", nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		if err := mc.SubmitTx(tx, milon.WithRequestID(requestId)); err != nil {
			return "", nil, err
		}
		return txHashHex(tx), tx, nil

	default:
		return "", nil, fmt.Errorf("unsupported paymentMode: %s", req.PaymentMode)
	}
}
