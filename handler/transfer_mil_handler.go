package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// milTokenAddress 是原生 MIL 代币的资源地址（IDL 常量 MIL_TOKEN_ADDRESS）。
const milTokenAddress = "M11on1111111111111111111111"

// transferMilRequest is the request body for POST /api/tool/transfer-mil.
type transferMilRequest struct {
	To         string `json:"to" binding:"required"`
	Amount     uint64 `json:"amount" binding:"required"`
	PrivateKey string `json:"privateKey" binding:"required"`
	KeyType    string `json:"keyType,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
}

// TransferMil handles POST /api/tool/transfer-mil
// MIL 转账快捷封装（AI 高频且易错操作的快车道）：调用方只给 to/amount/
// privateKey，内部完成 密钥派生地址 → 填充 token.Transfer 的全部固定参数
// （MIL 代币地址 / unified_payer_all / 默认签名模式）→ 复用与 /api/write
// 完全相同的 dispatchSubmit 提交路径。
func (h *ContractHandler) TransferMil(c *gin.Context) {
	var req transferMilRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "TransferMil", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body", err.Error()))
		return
	}

	keyType := req.KeyType
	if keyType == "" {
		keyType = "secp256k1" // 与 account_generate 缺省一致
	}
	sk, err := crypto.SecretKeyerFromStringRelaxed(req.PrivateKey)
	if err != nil {
		logSDKError(c, "TransferMil", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid privateKey: "+err.Error(), nil))
		return
	}
	// fndsa512：Go SDK 无法从私钥派生公钥（FnDsa512Public 为 TODO not
	// implemented），必须显式传 publicKey（account_generate 已返回）；
	// 其余曲线内部派生。
	var pk *crypto.PublicKey
	if keyType == "fndsa512" {
		if req.PublicKey == "" {
			c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
				"keyType fndsa512 需显式传 publicKey（897 字节公钥，Go SDK 无法从私钥派生；account_generate 的返回里有）", nil))
			return
		}
		pk, err = crypto.NewPublicKeyFromStringRelaxed(req.PublicKey)
	} else {
		pk, err = derivePublicKeyByType(sk, keyType)
	}
	if err != nil {
		logParamError(c, "TransferMil", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		logSDKError(c, "TransferMil", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid privateKey: "+err.Error(), nil))
		return
	}
	fromAddr := addr.ToBase58()

	// 公钥模式 + normalizeSignatureModeForAccount 自动修正（已上链账户自动
	// 升级为签名者列表模式），与 /api/write 的 pubkey 形态一致。
	writeReq := writeContractRequest{
		AppName:         "token",
		MethodName:      "Transfer",
		Args:            provider.Args{"from": fromAddr, "token": milTokenAddress, "to": req.To, "amount": req.Amount},
		PaymentMode:     PaymentModeUnifiedPayerAll,
		PayerPrivateKey: req.PrivateKey,
		PayerAddress:    fromAddr,
		SignatureMode:   json.RawMessage(`{"type":"pubkey","publicKey":"` + pk.ToHex() + `"}`),
	}

	mc := middleware.ClientFrom(c)
	requestId := lib.RequestID(time.Now().UnixMilli())

	txHash, tx, err := h.dispatchSubmit(mc, &writeReq, requestId)
	if err != nil {
		logSDKError(c, "TransferMil", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "failed to transfer: "+withChainErrorHint(err.Error()), nil))
		return
	}

	logBusinessInfo(c, "TransferMil", "txHash", txHash, "from", fromAddr, "to", req.To, "amount", req.Amount)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"txHash": txHash,
		"from":   fromAddr,
		"to":     req.To,
		"amount": req.Amount,
		"token":  milTokenAddress,
		"rawTx":  serializeTx(tx),
		"tip":    "用 tx_track 工具（GET /api/transactions/{hash}/track）跟踪确认",
	}, "ok"))
}
