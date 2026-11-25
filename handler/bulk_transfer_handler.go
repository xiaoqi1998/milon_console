package handler

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"milon-api-server/client"
	"milon-api-server/middleware"
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
	Percent     int    `json:"percent"` // 归集百分比,缺省 98(预留 2% 作 gas)
}

// defaultTransferPercent 归集百分比缺省值。2026-11-25 水龙头降为单次 1 MIL,
// 原写死"领水 10000 转 9800 留 200"三常量在新规则下恒 513,改为按实际余额百分比归集。
const defaultTransferPercent = 98

// normalizeTransferPercent 缺省 98;合法 1-100,越界报错。
func normalizeTransferPercent(p int) (uint64, error) {
	if p == 0 {
		return defaultTransferPercent, nil
	}
	if p < 0 || p > 100 {
		return 0, fmt.Errorf("percent must be between 1 and 100, got %d", p)
	}
	return uint64(p), nil
}

// bulkTransferAmount 按余额百分比计算转出额(最小单位,整数向下取整)。
func bulkTransferAmount(bal, percent uint64) uint64 {
	return bal * percent / 100
}

// bulkTransferResult 描述单个账户的处理结果。
type bulkTransferResult struct {
	Index          int    `json:"index"`
	Address        string `json:"address"`
	TransferTxHash string `json:"transferTxHash,omitempty"`
	Balance        uint64 `json:"balance,omitempty"`  // 领水后实际余额
	Transferred    uint64 `json:"transferred"`        // 实际转出额(余额×percent%)
	Reserved       uint64 `json:"reserved,omitempty"` // 预留(余额−转出,作 gas)
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// bulkTransferJob 描述一个异步批量转账任务的运行状态。
type bulkTransferJob struct {
	ID               string                `json:"id"`
	Status           string                `json:"status"` // running | completed
	Count            int                   `json:"count"`
	ToAddress        string                `json:"toAddress"`
	Percent          uint64                `json:"percent"`
	Done             int                   `json:"done"`
	Success          int                   `json:"success"`
	Failed           int                   `json:"failed"`
	TotalTransferred uint64                `json:"totalTransferred"`
	StartedAt        int64                 `json:"startedAt"`
	FinishedAt       int64                 `json:"finishedAt"`
	Results          []bulkTransferResult  `json:"results,omitempty"`

	mu sync.Mutex
}

// bulkTransferJobSnapshot 是 bulkTransferJob 的无锁快照 DTO。此前 snapshot
// 直接 `cp := *j` 整值拷贝——把 sync.Mutex 一并复制（go vet: assignment copies
// lock value），复制出的锁零值无保护语义；逐字段拷贝导出字段，wire 输出不变。
type bulkTransferJobSnapshot struct {
	ID               string               `json:"id"`
	Status           string               `json:"status"` // running | completed
	Count            int                  `json:"count"`
	ToAddress        string               `json:"toAddress"`
	Percent          uint64               `json:"percent"`
	Done             int                  `json:"done"`
	Success          int                  `json:"success"`
	Failed           int                  `json:"failed"`
	TotalTransferred uint64               `json:"totalTransferred"`
	StartedAt        int64                `json:"startedAt"`
	FinishedAt       int64                `json:"finishedAt"`
	Results          []bulkTransferResult `json:"results,omitempty"`
}

func (j *bulkTransferJob) snapshot() bulkTransferJobSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return bulkTransferJobSnapshot{
		ID:               j.ID,
		Status:           j.Status,
		Count:            j.Count,
		ToAddress:        j.ToAddress,
		Percent:          j.Percent,
		Done:             j.Done,
		Success:          j.Success,
		Failed:           j.Failed,
		TotalTransferred: j.TotalTransferred,
		StartedAt:        j.StartedAt,
		FinishedAt:       j.FinishedAt,
		Results:          j.Results,
	}
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

	percent, err := normalizeTransferPercent(req.Percent)
	if err != nil {
		logParamError(c, "BulkTransfer", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 16
	}
	if concurrency > 128 {
		concurrency = 128
	}

	mc := middleware.ClientFrom(c)
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
		Percent:   percent,
		Results:   results,
		StartedAt: time.Now().UnixMilli(),
	}

	h.jobsMu.Lock()
	h.jobs[jobID] = job
	h.jobsMu.Unlock()

	go h.run(job, mc, &toAddr, concurrency, percent)

	c.JSON(http.StatusAccepted, types.SuccessResponse(gin.H{
		"jobId":  jobID,
		"status": "running",
		"count":  req.Count,
	}, "task started"))
}

// run 在后台并发执行批量转账，实时更新 job 进度。
func (h *BulkTransferHandler) run(job *bulkTransferJob, mc *milon.Client, toAddr *crypto.Address, concurrency int, percent uint64) {
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

			res := h.processOne(mc, idx, toAddr, percent)

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

// processOne 处理单个账户：生成密钥 -> 领水 -> 按实际余额的 percent% 归集到目标地址。
// 百分比留成作 gas;余额过小(转出额取整为 0)时该户记失败,不影响其他账户。
func (h *BulkTransferHandler) processOne(mc *milon.Client, idx int, toAddr *crypto.Address, percent uint64) bulkTransferResult {
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

	// 1. 领水（统一 payer 模式构建；SDK 新版 ClaimFaucet 的无 gas 签名形式当前 devNet 不接受）
	claimTx, err := buildClaimFaucetTx(mc, *account, sk, mode)
	if err != nil {
		res.Error = "claim faucet: " + err.Error()
		return res
	}
	if err := submitClaimFaucetTx(mc, claimTx); err != nil {
		res.Error = "claim faucet submit: " + err.Error()
		return res
	}
	if _, err := mc.WaitForTransaction(txHashHex(claimTx), milon.WithWaitRequestID(lib.RequestID(1))); err != nil {
		res.Error = "claim faucet wait: " + err.Error()
		return res
	}

	// 2. 归集 MIL 到目标地址:按领水后实际余额的 percent% 转出,剩余留作 gas。
	//    领水后该账户可能已注册上链，此时公钥模式会被链端拒绝（错误 285 PubkeyModeForbidden），
	//    因此按链上状态重新解析签名模式（已上链 → 签名者列表模式）。
	bal, err := mc.BalanceOf(account)
	if err != nil {
		res.Error = "balance: " + err.Error()
		return res
	}
	res.Balance = bal
	amount := bulkTransferAmount(bal, percent)
	if amount == 0 {
		res.Error = fmt.Sprintf("balance too small to sweep (bal=%d, percent=%d%%)", bal, percent)
		return res
	}

	transferMode, err := normalizeSignatureModeForAccount(mc, *account, mode)
	if err != nil {
		res.Error = "resolve signature mode: " + err.Error()
		return res
	}

	wire, err := gen.Token.Transfer.Args(account, api.MILToken, toAddr, amount).Encode()
	if err != nil {
		res.Error = "encode transfer: " + err.Error()
		return res
	}

	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(account).
		AddIxAndPayerSig(*account, sk, 0, transferMode).
		Build()
	if err != nil {
		res.Error = "build transfer tx: " + err.Error()
		return res
	}

	if _, err := mc.SubmitTx(tx, milon.WithRequestID(lib.RequestID(time.Now().UnixMilli()))); err != nil {
		res.Error = "submit transfer: " + err.Error()
		return res
	}

	res.TransferTxHash = txHashHex(tx)
	if _, err := mc.WaitForTransaction(tx.TxHash(), milon.WithWaitRequestID(lib.RequestID(1))); err != nil {
		res.Error = "transfer submitted but wait failed: " + err.Error()
		return res
	}

	res.Transferred = amount
	res.Reserved = bal - amount
	res.Success = true
	return res
}
