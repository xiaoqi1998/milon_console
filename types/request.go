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
	SignatureMode json.RawMessage `json:"signatureMode"`
	// KeyType 是 privateKey 的曲线（secp256k1 缺省/ed25519/bls12381/fndsa512），
	// 仅在 signatureMode 缺省、需从私钥派生公钥时使用——32 字节经典私钥在不同
	// 曲线下派生不同公钥/地址，无法从字节本身区分（2026-10 AI 易用性）。
	KeyType string `json:"keyType,omitempty"`
}
