package handler

import (
	"encoding/hex"
	"fmt"
	"net/http"

	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

// UtilHandler exposes utility endpoints (address/key derivation, sign, verify).
type UtilHandler struct {
	enableSign bool
}

// NewUtilHandler creates a UtilHandler with the given enableSign flag.
func NewUtilHandler(enableSign bool) *UtilHandler {
	return &UtilHandler{enableSign: enableSign}
}

// deriveAddressRequest is the request body for DeriveAddress.
type deriveAddressRequest struct {
	PublicKey string `json:"publicKey"`
	KeyType   string `json:"keyType"`
}

// deriveAddressResponse is the response for DeriveAddress.
type deriveAddressResponse struct {
	Address   string `json:"address"`
	PublicKey string `json:"publicKey"`
	KeyType   string `json:"keyType"`
}

// DeriveAddress handles POST /api/util/address/derive
func (h *UtilHandler) DeriveAddress(c *gin.Context) {
	var req deriveAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	if req.PublicKey == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "publicKey is required", nil))
		return
	}

	pk, err := crypto.NewPublicKeyFromStringRelaxed(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid publicKey: "+err.Error(), nil))
		return
	}

	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to derive address: "+err.Error(), nil))
		return
	}

	keyType := req.KeyType
	if keyType == "" {
		keyType = keyTypeName(pk.Variant)
	}

	resp := deriveAddressResponse{
		Address:   addr.ToBase58(),
		PublicKey: pk.ToHex(),
		KeyType:   keyType,
	}

	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// derivePublicKeyRequest is the request body for DerivePublicKey.
type derivePublicKeyRequest struct {
	PrivateKey string `json:"privateKey"`
	KeyType    string `json:"keyType"`
}

// derivePublicKeyResponse is the response for DerivePublicKey.
type derivePublicKeyResponse struct {
	PublicKey  string `json:"publicKey"`
	KeyType    string `json:"keyType"`
	PrivateKey string `json:"privateKey"`
}

// DerivePublicKey handles POST /api/util/key/derive-public
func (h *UtilHandler) DerivePublicKey(c *gin.Context) {
	var req derivePublicKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	if req.PrivateKey == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "privateKey is required", nil))
		return
	}

	if req.KeyType == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "keyType is required", nil))
		return
	}

	sk, err := crypto.SecretKeyerFromStringRelaxed(req.PrivateKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid privateKey: "+err.Error(), nil))
		return
	}

	classicalSk := crypto.AsClassicalSecretKey(sk)
	if classicalSk == nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "only classical secret keys support public key derivation", nil))
		return
	}

	pk, err := derivePublicKeyByType(classicalSk, req.KeyType)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	resp := derivePublicKeyResponse{
		PublicKey:  pk.ToHex(),
		KeyType:    req.KeyType,
		PrivateKey: sk.ToHex(),
	}

	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// signMessageRequest is the request body for SignMessage.
type signMessageRequest struct {
	PrivateKey string `json:"privateKey"`
	Message    string `json:"message"`
	KeyType    string `json:"keyType"`
}

// signMessageResponse is the response for SignMessage.
type signMessageResponse struct {
	Signature string `json:"signature"`
	PublicKey string `json:"publicKey"`
}

// SignMessage handles POST /api/util/sign
func (h *UtilHandler) SignMessage(c *gin.Context) {
	if !h.enableSign {
		c.JSON(http.StatusForbidden, types.ErrorResponse(types.ERR_UNAUTHORIZED, "sign endpoint is disabled", nil))
		return
	}

	var req signMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	if req.PrivateKey == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "privateKey is required", nil))
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "message is required", nil))
		return
	}

	if req.KeyType == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "keyType is required", nil))
		return
	}

	msg, err := hex.DecodeString(req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid hex message: "+err.Error(), nil))
		return
	}

	sk, err := crypto.SecretKeyerFromStringRelaxed(req.PrivateKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid privateKey: "+err.Error(), nil))
		return
	}

	classicalSk := crypto.AsClassicalSecretKey(sk)
	if classicalSk == nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "only classical secret keys support signing", nil))
		return
	}

	pk, err := derivePublicKeyByType(classicalSk, req.KeyType)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	sig, err := sk.SignFor(*pk, msg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to sign: "+err.Error(), nil))
		return
	}

	resp := signMessageResponse{
		Signature: sig.ToHex(),
		PublicKey: pk.ToHex(),
	}

	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// verifySignatureRequest is the request body for VerifySignature.
type verifySignatureRequest struct {
	PublicKey string `json:"publicKey"`
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

// verifySignatureResponse is the response for VerifySignature.
type verifySignatureResponse struct {
	Valid bool `json:"valid"`
}

// VerifySignature handles POST /api/util/verify
func (h *UtilHandler) VerifySignature(c *gin.Context) {
	var req verifySignatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	if req.PublicKey == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "publicKey is required", nil))
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "message is required", nil))
		return
	}

	if req.Signature == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "signature is required", nil))
		return
	}

	pk, err := crypto.NewPublicKeyFromStringRelaxed(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid publicKey: "+err.Error(), nil))
		return
	}

	msg, err := hex.DecodeString(req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid hex message: "+err.Error(), nil))
		return
	}

	sigBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid hex signature: "+err.Error(), nil))
		return
	}

	sig, err := crypto.NewSignatureFromBytes(sigBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid signature: "+err.Error(), nil))
		return
	}

	valid := sig.Verify(msg, pk) == nil

	resp := verifySignatureResponse{Valid: valid}
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// derivePublicKeyByType derives the public key from a classical secret key based on keyType.
func derivePublicKeyByType(sk *crypto.ClassicalSecretKey, keyType string) (*crypto.PublicKey, error) {
	switch keyType {
	case "secp256k1":
		return sk.Secp256k1Public()
	case "ed25519":
		return sk.Ed25519Public(), nil
	case "bls12381":
		return sk.BLS12381Public(), nil
	default:
		return nil, fmt.Errorf("unsupported keyType: %s (supported: secp256k1, ed25519, bls12381)", keyType)
	}
}

// keyTypeName returns the string name for a PublicKeyType.
func keyTypeName(variant crypto.PublicKeyType) string {
	switch variant {
	case crypto.PublicKeyTypeSecp256k1:
		return "secp256k1"
	case crypto.PublicKeyTypeEd25519:
		return "ed25519"
	case crypto.PublicKeyTypeBLS12381:
		return "bls12381"
	case crypto.PublicKeyTypeFnDsa512:
		return "fndsa512"
	default:
		return "unknown"
	}
}
