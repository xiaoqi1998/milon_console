package types

import "encoding/json"

// NetworkSwitchRequest is the request body for switching the active network.
type NetworkSwitchRequest struct {
	Network string `json:"network" binding:"required"`
}

// PaginationQuery holds common pagination parameters.
type PaginationQuery struct {
	Limit  int `json:"limit" form:"limit"`
	Offset int `json:"offset" form:"offset"`
}

// SignerEntry represents a single signer in multi_signer payment mode.
type SignerEntry struct {
	Address       string          `json:"address" binding:"required"`
	PrivateKey    string          `json:"privateKey"`
	SignatureMode json.RawMessage `json:"signatureMode" binding:"required"`
}
