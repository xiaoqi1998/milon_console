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

	jobsMu sync.Mutex
	jobs   map[string]*bulkTransferJob
}

// NewBulkTransferHandler 创建绑定到 NetworkManager 的 BulkTransferHandler。
func NewBulkTransferHandler(nm *client.NetworkManager) *BulkTransferHandler {
	return &BulkTransferHandler{
		nm:   nm,
		jobs: make(map[string]*bulkTransferJob),
	}
}

// bulkTransferRequest 是 POST /api/tool/bulk-transfer 的请求体。
type bulkTransferRequest struct {
	Count       int    `json:"count" binding:"required"`
	ToAddress   string `json:"toAddress" binding:"required"`
	Concurrency int    `json:"concurrency"`
}

// 领水固定发放 10000 MIL（FAUCET_AMOUNT = 10000 * 1_000_000，decimals=6）。
// 每个账户领水后固定转走 9800 MIL，预留 200 MIL 作为 gas 手续费。
const (
	faucetAmount   uint64 = 10000 * 1_000_000 // 领水金额（最小单位）
	gasReserve     uint64 = 200 * 1_000_000   // 预留 gas（最小单位）
	transferAmount uint64 = faucetAmount - gasReserve
)

// bulkTransferResult 描述单个账户的处理结果。
type bulkTransferResult struct {
	Index          int    `json:"index"`
	Address        string `json:"address"`
	TransferTxHash string `json:"transferTxHash,omitempty"`
	Transferred    uint64 `json:"transferred"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// bulkTransferJob 描述一个异步批量转账任务的运行状态。
type bulkTransferJob struct {
	ID               string                `json:"id"`
	Status           string                `json:"status"` // running | completed
	Count            int                   `json:"count"`
	ToAddress        string                `json:"toAddress"`
	Done             int                   `json:"done"`
	Success          int                   `json:"success"`
	Failed           int                   `json:"failed"`
	TotalTransferred uint64                `json:"totalTransferred"`
	StartedAt        int64                 `json:"startedAt"`
	FinishedAt       int64                 `json:"finishedAt"`
	Results          []bulkTransferResult  `json:"results,omitempty"`

	mu sync.Mutex
}

func (j *bulkTransferJob) snapshot() bulkTransferJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	cp := *j
	return cp
}

// BulkTransfer 处理 POST /api/tool/bulk-transfer。
// 该接口为异步任务：校验参数后立即返回 jobId，后台 goroutine 并发执行
// 生成账户 -> 领水 -> 归集 MIL 的流程，调用方通过 GET /api/tool/bulk-transfer/:id 轮询进度。
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
	for i := range results {
		results[i].Index = i
	}

	jobID := fmt.Sprintf("%d", time.Now().UnixNano())
	job := &bulkTransferJob{
		ID:        jobID,
		Status:    "running",
		Count:     req.Count,
		ToAddress: req.ToAddress,
		Results:   results,
		StartedAt: time.Now().UnixMilli(),
	}

	h.jobsMu.Lock()
	h.jobs[jobID] = job
	h.jobsMu.Unlock()

	go h.run(job, mc, &toAddr, concurrency)

	c.JSON(http.StatusAccepted, types.SuccessResponse(gin.H{
		"jobId":  jobID,
		"status": "running",
		"count":  req.Count,
	}, "task started"))
}

// run 在后台并发执行批量转账，实时更新 job 进度。
func (h *BulkTransferHandler) run(job *bulkTransferJob, mc *milon.Client, toAddr *crypto.Address, concurrency int) {
	var doneCount int64
	var successCount int64
	var totalTransferred uint64

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < job.Count; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			res := h.processOne(mc, idx, toAddr)

			job.mu.Lock()
			job.Results[idx] = res
			job.Done = int(atomic.AddInt64(&doneCount, 1))
			if res.Success {
				job.Success = int(atomic.AddInt64(&successCount, 1))
				mu.Lock()
				totalTransferred += res.Transferred
				job.TotalTransferred = totalTransferred
				mu.Unlock()
			} else {
				job.Failed = int(atomic.LoadInt64(&doneCount)) - job.Success
			}
			job.mu.Unlock()
		}(i)
	}
	wg.Wait()

	job.mu.Lock()
	job.Status = "completed"
	job.FinishedAt = time.Now().UnixMilli()
	job.mu.Unlock()
}

// GetBulkTransferStatus 处理 GET /api/tool/bulk-transfer/:id，返回任务进度与结果。
func (h *BulkTransferHandler) GetBulkTransferStatus(c *gin.Context) {
	id := c.Param("id")

	h.jobsMu.Lock()
	job, ok := h.jobs[id]
	h.jobsMu.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "job not found: "+id, nil))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(job.snapshot(), "ok"))
}

// processOne 处理单个账户：生成密钥 -> 领水 -> 归集 9800 MIL 到目标地址（预留 200 gas）。
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

	// 1. 领水（claim_faucet 为 sponsor 交易，固定发放 10000 MIL）
	if err := mc.ClaimFaucet(sk, account, mode); err != nil {
		res.Error = "claim faucet: " + err.Error()
		return res
	}

	// 2. 归集 MIL 到目标地址，预留 gasReserve 作为手续费
	wire, err := gen.Token.Transfer.Args(account, api.MILToken, toAddr, transferAmount).Encode()
	if err != nil {
		res.Error = "encode transfer: " + err.Error()
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
