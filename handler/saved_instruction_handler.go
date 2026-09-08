package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
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

// ---------- Data Model ----------

// SavedInstruction represents a stored IDL method call with bound account info.
type SavedInstruction struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// IDL method
	AppName    string        `json:"appName"`
	MethodName string        `json:"methodName"`
	Args       provider.Args `json:"args"`

	// Auto-detected from IDL: "view" or "entry"
	Kind string `json:"kind"`

	// Account / signing (for entry instructions)
	PaymentMode     string          `json:"paymentMode,omitempty"`
	PayerAddress    string          `json:"payerAddress,omitempty"`
	PayerPrivateKey string          `json:"payerPrivateKey,omitempty"`
	SignatureMode   json.RawMessage `json:"signatureMode,omitempty"`

	// For unified_dual_sign
	IxAddress       string          `json:"ixAddress,omitempty"`
	IxPrivateKey    string          `json:"ixPrivateKey,omitempty"`
	IxSignatureMode json.RawMessage `json:"ixSignatureMode,omitempty"`

	// For split mode
	OwnerAddress    string `json:"ownerAddress,omitempty"`
	OwnerPrivateKey string `json:"ownerPrivateKey,omitempty"`

	// For multi_signer mode
	Signers  []types.SignerEntry `json:"signers,omitempty"`
	GasPayer *types.SignerEntry  `json:"gasPayer,omitempty"`

	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

// ---------- Store ----------

// SavedInstructionStore persists saved instructions to a JSON file.
type SavedInstructionStore struct {
	mu       sync.RWMutex
	filePath string
	items    map[string]*SavedInstruction
}

// NewSavedInstructionStore creates a store backed by the given JSON file.
func NewSavedInstructionStore(filePath string) (*SavedInstructionStore, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	s := &SavedInstructionStore{
		filePath: filePath,
		items:    make(map[string]*SavedInstruction),
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *SavedInstructionStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	var list []*SavedInstruction
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("failed to parse %s: %w", s.filePath, err)
	}
	s.items = make(map[string]*SavedInstruction, len(list))
	for _, item := range list {
		s.items[item.ID] = item
	}
	return nil
}

func (s *SavedInstructionStore) save() error {
	list := make([]*SavedInstruction, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0o644)
}

func (s *SavedInstructionStore) List() []*SavedInstruction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*SavedInstruction, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	return list
}

func (s *SavedInstructionStore) Get(id string) (*SavedInstruction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	return item, ok
}

func (s *SavedInstructionStore) Create(item *SavedInstruction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return s.save()
}

func (s *SavedInstructionStore) Update(item *SavedInstruction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return s.save()
}

func (s *SavedInstructionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return s.save()
}

// ---------- Handler ----------

// SavedInstructionHandler provides CRUD + execute for saved instructions.
type SavedInstructionHandler struct {
	nm    *client.NetworkManager
	store *SavedInstructionStore
}

// NewSavedInstructionHandler creates the handler.
func NewSavedInstructionHandler(nm *client.NetworkManager, dataDir string) (*SavedInstructionHandler, error) {
	store, err := NewSavedInstructionStore(filepath.Join(dataDir, "saved_instructions.json"))
	if err != nil {
		return nil, err
	}
	return &SavedInstructionHandler{nm: nm, store: store}, nil
}

// --- Request DTOs ---

type createSavedInstructionRequest struct {
	Name            string              `json:"name" binding:"required"`
	Description     string              `json:"description"`
	AppName         string              `json:"appName" binding:"required"`
	MethodName      string              `json:"methodName" binding:"required"`
	Args            provider.Args       `json:"args"`
	PaymentMode     string              `json:"paymentMode"`
	PayerAddress    string              `json:"payerAddress"`
	PayerPrivateKey string              `json:"payerPrivateKey"`
	SignatureMode   json.RawMessage     `json:"signatureMode"`
	IxAddress       string              `json:"ixAddress"`
	IxPrivateKey    string              `json:"ixPrivateKey"`
	IxSignatureMode json.RawMessage     `json:"ixSignatureMode"`
	OwnerAddress    string              `json:"ownerAddress"`
	OwnerPrivateKey string              `json:"ownerPrivateKey"`
	Signers         []types.SignerEntry `json:"signers"`
	GasPayer        *types.SignerEntry  `json:"gasPayer"`
}

type updateSavedInstructionRequest struct {
	Name            *string             `json:"name"`
	Description     *string             `json:"description"`
	Args            provider.Args       `json:"args"`
	PaymentMode     *string             `json:"paymentMode"`
	PayerAddress    *string             `json:"payerAddress"`
	PayerPrivateKey *string             `json:"payerPrivateKey"`
	SignatureMode   json.RawMessage     `json:"signatureMode"`
	IxAddress       *string             `json:"ixAddress"`
	IxPrivateKey    *string             `json:"ixPrivateKey"`
	IxSignatureMode json.RawMessage     `json:"ixSignatureMode"`
	OwnerAddress    *string             `json:"ownerAddress"`
	OwnerPrivateKey *string             `json:"ownerPrivateKey"`
	Signers         []types.SignerEntry `json:"signers"`
	GasPayer        *types.SignerEntry  `json:"gasPayer"`
}

// --- CRUD ---

// CreateSavedInstruction handles POST /api/saved-instructions
func (h *SavedInstructionHandler) CreateSavedInstruction(c *gin.Context) {
	var req createSavedInstructionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	mc, _ := h.nm.GetCurrent()

	// Validate IDL method exists and detect kind.
	pd, ok := mc.GetAllPd()[req.AppName]
	if !ok {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, fmt.Sprintf("IDL app %q not found", req.AppName), nil))
		return
	}
	instruction, err := pd.GetInstructionByName(req.MethodName)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, fmt.Sprintf("method %q not found in app %q: %s", req.MethodName, req.AppName, err.Error()), nil))
		return
	}

	if req.Args == nil {
		req.Args = provider.Args{}
	}

	now := time.Now().UnixMilli()
	item := &SavedInstruction{
		ID:              generateID(),
		Name:            req.Name,
		Description:     req.Description,
		AppName:         req.AppName,
		MethodName:      req.MethodName,
		Args:            req.Args,
		Kind:            instruction.Kind,
		PaymentMode:     req.PaymentMode,
		PayerAddress:    req.PayerAddress,
		PayerPrivateKey: req.PayerPrivateKey,
		SignatureMode:   req.SignatureMode,
		IxAddress:       req.IxAddress,
		IxPrivateKey:    req.IxPrivateKey,
		IxSignatureMode: req.IxSignatureMode,
		OwnerAddress:    req.OwnerAddress,
		OwnerPrivateKey: req.OwnerPrivateKey,
		Signers:         req.Signers,
		GasPayer:        req.GasPayer,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// For entry instructions, default paymentMode if not specified.
	if item.Kind == "entry" && item.PaymentMode == "" {
		item.PaymentMode = PaymentModeUnifiedPayerAll
	}

	if err := h.store.Create(item); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to save instruction: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(item, "created"))
}

// ListSavedInstructions handles GET /api/saved-instructions
func (h *SavedInstructionHandler) ListSavedInstructions(c *gin.Context) {
	list := h.store.List()
	c.JSON(http.StatusOK, types.SuccessResponse(list, "ok"))
}

// GetSavedInstruction handles GET /api/saved-instructions/:id
func (h *SavedInstructionHandler) GetSavedInstruction(c *gin.Context) {
	id := c.Param("id")
	item, ok := h.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "instruction not found: "+id, nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(item, "ok"))
}

// UpdateSavedInstruction handles PUT /api/saved-instructions/:id
func (h *SavedInstructionHandler) UpdateSavedInstruction(c *gin.Context) {
	id := c.Param("id")
	item, ok := h.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "instruction not found: "+id, nil))
		return
	}

	var req updateSavedInstructionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	// Apply partial updates.
	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Description != nil {
		item.Description = *req.Description
	}
	if req.Args != nil {
		item.Args = req.Args
	}
	if req.PaymentMode != nil {
		item.PaymentMode = *req.PaymentMode
	}
	if req.PayerAddress != nil {
		item.PayerAddress = *req.PayerAddress
	}
	if req.PayerPrivateKey != nil {
		item.PayerPrivateKey = *req.PayerPrivateKey
	}
	if req.SignatureMode != nil {
		item.SignatureMode = req.SignatureMode
	}
	if req.IxAddress != nil {
		item.IxAddress = *req.IxAddress
	}
	if req.IxPrivateKey != nil {
		item.IxPrivateKey = *req.IxPrivateKey
	}
	if req.IxSignatureMode != nil {
		item.IxSignatureMode = req.IxSignatureMode
	}
	if req.OwnerAddress != nil {
		item.OwnerAddress = *req.OwnerAddress
	}
	if req.OwnerPrivateKey != nil {
		item.OwnerPrivateKey = *req.OwnerPrivateKey
	}
	if req.Signers != nil {
		item.Signers = req.Signers
	}
	if req.GasPayer != nil {
		item.GasPayer = req.GasPayer
	}
	item.UpdatedAt = time.Now().UnixMilli()

	if err := h.store.Update(item); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to update instruction: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(item, "updated"))
}

// DeleteSavedInstruction handles DELETE /api/saved-instructions/:id
func (h *SavedInstructionHandler) DeleteSavedInstruction(c *gin.Context) {
	id := c.Param("id")
	if _, ok := h.store.Get(id); !ok {
		c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "instruction not found: "+id, nil))
		return
	}
	if err := h.store.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to delete instruction: "+err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{"id": id}, "deleted"))
}

// --- Execute ---

// ExecuteSavedInstruction handles POST /api/saved-instructions/:id/execute
// Query params:
//
//	mode = "read" | "simulate" | "send"  (default: auto — view→read, entry→simulate)
//	wait = "true" | "false"             (default: true for send mode)
func (h *SavedInstructionHandler) ExecuteSavedInstruction(c *gin.Context) {
	id := c.Param("id")
	item, ok := h.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "instruction not found: "+id, nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	requestId := lib.RequestID(time.Now().UnixMilli())

	// Determine execution mode.
	mode := c.DefaultQuery("mode", "")
	if mode == "" {
		if item.Kind == "view" {
			mode = "read"
		} else {
			mode = "simulate"
		}
	}

	switch mode {
	case "read":
		h.executeRead(c, mc, item, requestId)
	case "simulate":
		h.executeSimulate(c, mc, item, requestId)
	case "send":
		h.executeSend(c, mc, item, requestId)
	default:
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
			fmt.Sprintf("invalid mode %q: expected read, simulate, or send", mode), nil))
	}
}

func (h *SavedInstructionHandler) executeRead(c *gin.Context, mc *milon.Client, item *SavedInstruction, requestId lib.RequestID) {
	pd, ok := mc.GetAllPd()[item.AppName]
	if !ok {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, fmt.Sprintf("IDL app %q not found", item.AppName), nil))
		return
	}

	wire, err := encodeWithCoercion(pd, item.MethodName, item.Args)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to encode instruction: "+err.Error(), nil))
		return
	}

	result, err := mc.View([]api.PackedInstruction{wire}, milon.WithRequestID(requestId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to execute view: "+err.Error(), nil))
		return
	}

	// Try to decode the view result.
	bodyValues, decodeErr := pd.DecodeViewData(item.MethodName, result.HTTPResponseBody)
	if decodeErr != nil {
		// Return raw hex on decode failure (IDL drift tolerance).
		c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
			"mode":        "read",
			"raw":         hex.EncodeToString(result.HTTPResponseBody),
			"decodeError": decodeErr.Error(),
		}, "ok (raw fallback)"))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"mode":   "read",
		"result": bodyValues,
	}, "ok"))
}

func (h *SavedInstructionHandler) executeSimulate(c *gin.Context, mc *milon.Client, item *SavedInstruction, requestId lib.RequestID) {
	pd, ok := mc.GetAllPd()[item.AppName]
	if !ok {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, fmt.Sprintf("IDL app %q not found", item.AppName), nil))
		return
	}

	wire, err := encodeWithCoercion(pd, item.MethodName, item.Args)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to encode instruction: "+err.Error(), nil))
		return
	}
	instructions := []api.PackedInstruction{wire}

	tx, err := h.buildSimulateTx(item, instructions)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to build simulate tx: "+err.Error(), nil))
		return
	}

	result, err := mc.SimulateTx(tx, milon.WithRequestID(requestId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to simulate: "+err.Error(), nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"mode":    "simulate",
		"receipt": result.BodySimulateReceipt,
		"rawTx":   serializeTx(tx),
	}, "ok"))
}

func (h *SavedInstructionHandler) executeSend(c *gin.Context, mc *milon.Client, item *SavedInstruction, requestId lib.RequestID) {
	pd, ok := mc.GetAllPd()[item.AppName]
	if !ok {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, fmt.Sprintf("IDL app %q not found", item.AppName), nil))
		return
	}

	wire, err := encodeWithCoercion(pd, item.MethodName, item.Args)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to encode instruction: "+err.Error(), nil))
		return
	}
	instructions := []api.PackedInstruction{wire}

	tx, err := h.buildSignedTx(item, instructions)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to build signed tx: "+err.Error(), nil))
		return
	}

	if err := mc.SubmitTx(tx, milon.WithRequestID(requestId)); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to submit tx: "+err.Error(), nil))
		return
	}

	txHash := txHashHex(tx)

	// Optionally wait for confirmation.
	waitStr := c.DefaultQuery("wait", "true")
	var waitResult any
	if waitStr == "true" || waitStr == "1" {
		waitRes, waitErr := mc.WaitForTransaction(txHash, milon.WithWaitRequestID(lib.RequestID(1)))
		if waitErr != nil {
			c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
				"mode":      "send",
				"txHash":    txHash,
				"submitted": true,
				"waitError": waitErr.Error(),
			}, "submitted (wait failed)"))
			return
		}
		waitResult = waitRes.BodyTxHistory
	}

	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"mode":      "send",
		"txHash":    txHash,
		"submitted": true,
		"txHistory": waitResult,
	}, "ok"))
}

// --- Transaction builders ---

func (h *SavedInstructionHandler) buildSimulateTx(item *SavedInstruction, instructions []api.PackedInstruction) (*lib.Transaction, error) {
	switch item.PaymentMode {
	case PaymentModeUnifiedPayerAll, "":
		addr, mode, err := h.resolveAddressAndMode(item.PayerAddress, item.SignatureMode)
		if err != nil {
			return nil, err
		}
		return lib.NewTransactionBuilder(instructions).
			WithPayer(&addr).
			AddSimulateIxAndPayerSig(addr, 0, mode).
			Build()

	case PaymentModeUnifiedDualSign:
		payerAddr, payerMode, err := h.resolveAddressAndMode(item.PayerAddress, item.SignatureMode)
		if err != nil {
			return nil, err
		}
		ixAddr, ixMode, err := h.resolveAddressAndMode(item.IxAddress, item.IxSignatureMode)
		if err != nil {
			return nil, fmt.Errorf("invalid ix fields: %w", err)
		}
		return lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddSimulatePayerSig(payerAddr, payerMode).
			AddSimulateIxesSig(ixAddr, []uint8{0}, false, ixMode).
			Build()

	case PaymentModeUnifiedPayerOnlyGas:
		addr, mode, err := h.resolveAddressAndMode(item.PayerAddress, item.SignatureMode)
		if err != nil {
			return nil, err
		}
		return lib.NewTransactionBuilder(instructions).
			WithPayer(&addr).
			AddSimulatePayerSig(addr, mode).
			Build()

	case PaymentModeSplit:
		ownerAddrStr := item.OwnerAddress
		if ownerAddrStr == "" {
			ownerAddrStr = item.PayerAddress
		}
		addr, mode, err := h.resolveAddressAndMode(ownerAddrStr, item.SignatureMode)
		if err != nil {
			return nil, err
		}
		return lib.NewTransactionBuilder(instructions).
			AddSimulateIxesSig(addr, []uint8{0}, true, mode).
			Build()

	default:
		return nil, fmt.Errorf("unsupported paymentMode for simulate: %s", item.PaymentMode)
	}
}

func (h *SavedInstructionHandler) buildSignedTx(item *SavedInstruction, instructions []api.PackedInstruction) (*lib.Transaction, error) {
	switch item.PaymentMode {
	case PaymentModeUnifiedPayerAll, "":
		addr, mode, err := h.resolveAddressAndMode(item.PayerAddress, item.SignatureMode)
		if err != nil {
			return nil, err
		}
		sk, err := types.ParseSecretKey(item.PayerPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		tx, err := lib.NewTransactionBuilder(instructions).
			WithPayer(&addr).
			AddIxAndPayerSig(addr, sk, 0, mode).
			Build()
		if err != nil {
			return nil, err
		}
		if err := tx.ValidateWire(); err != nil {
			return nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		return tx, nil

	case PaymentModeUnifiedDualSign:
		payerAddr, payerMode, err := h.resolveAddressAndMode(item.PayerAddress, item.SignatureMode)
		if err != nil {
			return nil, err
		}
		payerSk, err := types.ParseSecretKey(item.PayerPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
		}
		ixAddr, ixMode, err := h.resolveAddressAndMode(item.IxAddress, item.IxSignatureMode)
		if err != nil {
			return nil, fmt.Errorf("invalid ix fields: %w", err)
		}
		ixSk, err := types.ParseSecretKey(item.IxPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid ixPrivateKey: %w", err)
		}
		tx, err := lib.NewTransactionBuilder(instructions).
			WithPayer(&payerAddr).
			AddPayerSig(payerAddr, payerSk, payerMode).
			AddIxesSig(ixAddr, ixSk, []uint8{0}, false, ixMode).
			Build()
		if err != nil {
			return nil, err
		}
		if err := tx.ValidateWire(); err != nil {
			return nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		return tx, nil

	case PaymentModeSplit:
		ownerAddrStr := item.OwnerAddress
		if ownerAddrStr == "" {
			ownerAddrStr = item.PayerAddress
		}
		ownerSkStr := item.OwnerPrivateKey
		if ownerSkStr == "" {
			ownerSkStr = item.PayerPrivateKey
		}
		addr, mode, err := h.resolveAddressAndMode(ownerAddrStr, item.SignatureMode)
		if err != nil {
			return nil, err
		}
		sk, err := types.ParseSecretKey(ownerSkStr)
		if err != nil {
			return nil, fmt.Errorf("invalid ownerPrivateKey: %w", err)
		}
		tx, err := lib.NewTransactionBuilder(instructions).
			AddIxesSig(addr, sk, []uint8{0}, true, mode).
			Build()
		if err != nil {
			return nil, err
		}
		if err := tx.ValidateWireWith([]uint8{0}); err != nil {
			return nil, fmt.Errorf("transaction validation failed: %w", err)
		}
		return tx, nil

	case PaymentModeMultiSigner:
		return h.buildMultiSignerTx(item, instructions)

	case PaymentModeSponsored:
		return h.buildSponsoredTx(item, instructions)

	default:
		return nil, fmt.Errorf("unsupported paymentMode for send: %s", item.PaymentMode)
	}
}

func (h *SavedInstructionHandler) buildMultiSignerTx(item *SavedInstruction, instructions []api.PackedInstruction) (*lib.Transaction, error) {
	if len(item.Signers) == 0 {
		return nil, fmt.Errorf("multi_signer mode requires at least one signer")
	}

	signerAddrs := make([]crypto.Address, 0, len(item.Signers))
	signerSks := make([]crypto.SecretKeyer, 0, len(item.Signers))
	signerModes := make([]lib.AccountSignatureMode, 0, len(item.Signers))

	for i, s := range item.Signers {
		addr, err := types.ParseAddress(s.Address)
		if err != nil {
			return nil, fmt.Errorf("signer[%d] invalid address: %w", i, err)
		}
		sk, err := types.ParseSecretKey(s.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("signer[%d] invalid privateKey: %w", i, err)
		}
		mode, err := types.ParseSignatureModeFromJSON(s.SignatureMode)
		if err != nil {
			return nil, fmt.Errorf("signer[%d] invalid signatureMode: %w", i, err)
		}
		signerAddrs = append(signerAddrs, addr)
		signerSks = append(signerSks, sk)
		signerModes = append(signerModes, mode)
	}

	builder := lib.NewTransactionBuilder(instructions)

	var gasPayerAddr *crypto.Address
	var gasPayerSk crypto.SecretKeyer
	var gasPayerMode lib.AccountSignatureMode

	if item.GasPayer != nil {
		addr, err := types.ParseAddress(item.GasPayer.Address)
		if err != nil {
			return nil, fmt.Errorf("gasPayer invalid address: %w", err)
		}
		sk, err := types.ParseSecretKey(item.GasPayer.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("gasPayer invalid privateKey: %w", err)
		}
		mode, err := types.ParseSignatureModeFromJSON(item.GasPayer.SignatureMode)
		if err != nil {
			return nil, fmt.Errorf("gasPayer invalid signatureMode: %w", err)
		}
		gasPayerAddr = &addr
		gasPayerSk = sk
		gasPayerMode = mode
		builder = builder.WithPayer(gasPayerAddr)
	}

	// First signer signs ix + payer (if no separate gas payer).
	if gasPayerAddr != nil {
		builder = builder.AddPayerSig(*gasPayerAddr, gasPayerSk, gasPayerMode)
		for i := range signerAddrs {
			builder = builder.AddIxesSig(signerAddrs[i], signerSks[i], []uint8{0}, false, signerModes[i])
		}
	} else {
		builder = builder.AddIxAndPayerSig(signerAddrs[0], signerSks[0], 0, signerModes[0])
		for i := 1; i < len(signerAddrs); i++ {
			builder = builder.AddIxesSig(signerAddrs[i], signerSks[i], []uint8{0}, false, signerModes[i])
		}
	}

	tx, err := builder.Build()
	if err != nil {
		return nil, err
	}
	if err := tx.ValidateWire(); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}
	return tx, nil
}

func (h *SavedInstructionHandler) buildSponsoredTx(item *SavedInstruction, instructions []api.PackedInstruction) (*lib.Transaction, error) {
	if item.PayerPrivateKey == "" {
		return nil, fmt.Errorf("sponsored mode requires payerPrivateKey")
	}
	addr, mode, err := h.resolveAddressAndMode(item.PayerAddress, item.SignatureMode)
	if err != nil {
		return nil, err
	}
	sk, err := types.ParseSecretKey(item.PayerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid payerPrivateKey: %w", err)
	}
	tx, err := lib.NewTransactionBuilder(instructions).
		WithPayer(&addr).
		AddIxAndPayerSig(addr, sk, 0, mode).
		Build()
	if err != nil {
		return nil, err
	}
	if err := tx.ValidateWire(); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}
	return tx, nil
}

func (h *SavedInstructionHandler) resolveAddressAndMode(addrStr string, sigModeRaw json.RawMessage) (crypto.Address, lib.AccountSignatureMode, error) {
	if addrStr == "" {
		return crypto.Address{}, nil, fmt.Errorf("address is required (payerAddress or ownerAddress)")
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

// ---------- Helpers ----------

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
