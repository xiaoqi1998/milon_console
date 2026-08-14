package handler

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"

	milon "github.com/milon-labs/milon-go-sdk"
)

// BulkTransferHandler 提供批量账户生成、领水并归集 MIL 的能力。
type BulkTransferHandler struct {
	nm *client.NetworkManager
}

// NewBulkTransferHandler 创建绑定到 NetworkManager 的 BulkTransferHandler。
func NewBulkTransferHandler(nm *client.NetworkManager) *BulkTransferHandler {
	return &BulkTransferHandler{nm: nm}
}

// bulkTransferRequest 是 POST /api/tool/bulk-transfer 的请求体。
type bulkTransferRequest struct {
	Count       int    `json:"count" binding:"required"`
	ToAddress   string `json:"toAddress" binding:"required"`
	Concurrency int    `json:"concurrency"`
}

// bulkTransferResult 描述单个账户的处理结果。
type bulkTransferResult struct {
	Index          int    `json:"index"`
	Address        string `json:"address"`
	Balance        uint64 `json:"balance"`
	ClaimTxHash    string `json:"claimTxHash,omitempty"`
	TransferTxHash string `json:"transferTxHash,omitempty"`
	Transferred    uint64 `json:"transferred"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// BulkTransfer 处理 POST /api/tool/bulk-transfer。
// 生成 count 个账户，逐个领水，随后把每个账户的全部 MIL 归集到 toAddress。
func (h *BulkTransferHandler) BulkTransfer(c *gin.Context) {
	var req bulkTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "BulkTransfer", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}

	if req.Count <= 0 || req.Count > 5000 {
		logParamError(c, "BulkTransfer", fmt.Errorf("count out of range: %d", req.Count))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "count must be between 1 and 5000", nil))
		return
	}

	toAddr, err := types.ParseAddress(req.ToAddress)
	if err != nil {
		logParamError(c, "BulkTransfer", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid toAddress: "+err.Error(), nil))
		return
	}
	toAddrPtr := &toAddr

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 16
	}
	if concurrency > 128 {
		concurrency = 128
	}

	mc, _ := h.nm.GetCurrent()
	if mc == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "no active network client", nil))
		return
	}

	results := make([]bulkTransferResult, req.Count)
	var totalTransferred uint64
	var successCount int64

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	start := time.Now()
	for i := 0; i < req.Count; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			res := h.processOne(mc, idx, toAddrPtr)
			results[idx] = res

			if res.Success {
				atomic.AddInt64(&successCount, 1)
				mu.Lock()
				totalTransferred += res.Transferred
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	elapsed := time.Since(start)
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"count":            req.Count,
		"toAddress":        req.ToAddress,
		"successCount":     successCount,
		"failedCount":      int64(req.Count) - successCount,
		"totalTransferred": totalTransferred,
		"elapsedMs":        elapsed.Milliseconds(),
		"results":          results,
	}, "ok"))
}

// processOne 处理单个账户：生成密钥 -> 领水 -> 查询余额 -> 归集全部 MIL。
func (h *BulkTransferHandler) processOne(mc *milon.Client, idx int, toAddr *crypto.Address) bulkTransferResult {
	res := bulkTransferResult{Index: idx}

	sk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
	pk := sk.Ed25519Public()
	account, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		res.Error = "derive address: " + err.Error()
		return res
	}
	res.Address = account.ToBase58()

	mode := lib.PubKeySignatureMode{PublicKey: *pk}

	// 1. 领水（claim_faucet 为 sponsor 交易，gas 由链上赞助池支付）
	if err := mc.ClaimFaucet(sk, account, mode); err != nil {
		res.Error = "claim faucet: " + err.Error()
		return res
	}

	// 2. 查询 MIL 余额
	balance, err := mc.BalanceOf(account)
	if err != nil {
		res.Error = "balance of: " + err.Error()
		return res
	}
	res.Balance = balance

	if balance == 0 {
		res.Success = true
		res.Error = "zero balance, nothing to transfer"
		return res
	}

	// 3. 归集 MIL 到目标地址（MIL 即 gas 代币，需预留 gas 手续费）
	wire, err := gen.Token.Transfer.Args(account, api.MILToken, toAddr, balance).Encode()
	if err != nil {
		res.Error = "encode transfer: " + err.Error()
		return res
	}

	builder := lib.NewTransactionBuilder([]api.PackedInstruction{wire})

	// 3.1 模拟执行，拿到精确 gas 消耗
	simulateTx, err := builder.AddSimulateIxAndPayerSig(*account, 0, mode).Build()
	if err != nil {
		res.Error = "build simulate tx: " + err.Error()
		return res
	}
	simulateResult, err := mc.SimulateTx(simulateTx, lib.RequestID(time.Now().UnixMilli()))
	if err != nil {
		res.Error = "simulate transfer: " + err.Error()
		return res
	}
	gasCharged := simulateResult.BodySimulateReceipt.GasCharged

	// 3.2 转走扣除 gas 后的全部余额，避免 gas 不足导致交易失败
	if balance <= gasCharged {
		res.Success = true
		res.Error = "balance not enough to cover gas"
		return res
	}
	transferAmount := balance - gasCharged

	wire, err = gen.Token.Transfer.Args(account, api.MILToken, toAddr, transferAmount).Encode()
	if err != nil {
		res.Error = "encode final transfer: " + err.Error()
		return res
	}

	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		AddIxAndPayerSig(*account, sk, 0, mode).
		Build()
	if err != nil {
		res.Error = "build transfer tx: " + err.Error()
		return res
	}

	if err := mc.SubmitTx(tx, lib.RequestID(time.Now().UnixMilli())); err != nil {
		res.Error = "submit transfer: " + err.Error()
		return res
	}

	res.TransferTxHash = txHashHex(tx)
	if _, err := mc.WaitForTransaction(tx.TxHash(), lib.RequestID(1)); err != nil {
		res.Error = "transfer submitted but wait failed: " + err.Error()
		return res
	}

	res.Transferred = transferAmount
	res.Success = true
	return res
}
