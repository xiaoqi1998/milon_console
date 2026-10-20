package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"milon-api-server/client"
	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
)

// ==================== 播种器:异步任务 + HTTP 端点 ====================
// POST /api/tool/seed           启动(同刻仅允许一个,重复返回 409)
// GET  /api/tool/seed/plan      计划预览(不上链)
// GET  /api/tool/seed/status/:id 进度快照(模块阶段/日志 tail/地址表)

const (
	seedStatePath   = "data/seed_state.json"
	seedLogRingSize = 500
	seedLogTailSize = 120
)

// SeedJobHandler 播种任务管理。
type SeedJobHandler struct {
	nm        *client.NetworkManager
	statePath string

	jobsMu    sync.Mutex
	jobs      map[string]*seedJob
	runningID string
}

// NewSeedJobHandler 创建绑定 NetworkManager 的播种任务处理器。
func NewSeedJobHandler(nm *client.NetworkManager) *SeedJobHandler {
	return &SeedJobHandler{
		nm:        nm,
		statePath: seedStatePath,
		jobs:      map[string]*seedJob{},
	}
}

type seedStartRequest struct {
	Preset     string          `json:"preset"`    // minimal|standard|full
	Overrides  json.RawMessage `json:"overrides"` // 对 preset 的深合并覆盖
	Config     *SeedConfig     `json:"config"`    // 完整配置直传(最高优先)
	Scale      string          `json:"scale"`     // 兼容:full→full,small→minimal
	Modules    []string        `json:"modules"`
	SeedPhrase string          `json:"seedPhrase"`
	Reset      bool            `json:"reset"`
}

// seedResolveConfig 请求 → 生效 SeedConfig。
// 优先级:Config > Preset+Overrides > Scale 映射 > 默认 standard。
func seedResolveConfig(req seedStartRequest) (SeedConfig, error) {
	if req.Config != nil {
		cfg := *req.Config
		if cfg.SeedPhrase == "" {
			cfg.SeedPhrase = req.SeedPhrase
		}
		if cfg.SeedPhrase == "" {
			cfg.SeedPhrase = SeedDefaultPhrase
		}
		return cfg, nil
	}
	var base SeedConfig
	switch {
	case req.Preset != "":
		b, err := SeedConfigForPreset(req.Preset)
		if err != nil {
			return SeedConfig{}, err
		}
		base = b
	case req.Scale == "full" || req.Scale == "small":
		preset := req.Scale
		if preset == "small" {
			preset = "minimal"
		}
		b, err := SeedConfigForPreset(preset)
		if err != nil {
			return SeedConfig{}, err
		}
		base = b
	default:
		base = SeedConfigDefaults()
	}
	if len(req.Overrides) > 0 {
		merged, err := seedMergeConfig(base, req.Overrides)
		if err != nil {
			return SeedConfig{}, err
		}
		base = merged
	}
	if req.SeedPhrase != "" {
		base.SeedPhrase = req.SeedPhrase
	}
	if base.SeedPhrase == "" {
		base.SeedPhrase = SeedDefaultPhrase
	}
	return base, nil
}

type seedJob struct {
	ID         string   `json:"id"`
	Scale      string   `json:"scale"`
	Modules    []string `json:"modules"`
	SeedPhrase string   `json:"seedPhrase"`
	Status     string   `json:"status"` // running | completed | failed
	Error      string   `json:"error,omitempty"`
	StartedAt  int64    `json:"startedAt"`
	FinishedAt int64    `json:"finishedAt"`
	Preset     string   `json:"preset,omitempty"`
	ConfigHash string   `json:"configHash,omitempty"`

	// plan 启动时已构建,发布后只读:状态回显(config/summary/地址表)直接读,不再重建计划
	plan *SeedPlan

	mu   sync.Mutex
	logs []string
}

func (j *seedJob) addLog(format string, a ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.logs = append(j.logs, time.Now().Format("15:04:05")+" "+fmt.Sprintf(format, a...))
	if len(j.logs) > seedLogRingSize {
		j.logs = j.logs[len(j.logs)-seedLogRingSize:]
	}
}

func (j *seedJob) logTail() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.logs) > seedLogTailSize {
		return append([]string{}, j.logs[len(j.logs)-seedLogTailSize:]...)
	}
	return append([]string{}, j.logs...)
}

type seedJobSnapshot struct {
	ID         string   `json:"id"`
	Scale      string   `json:"scale"`
	Modules    []string `json:"modules"`
	Status     string   `json:"status"`
	Error      string   `json:"error,omitempty"`
	StartedAt  int64    `json:"startedAt"`
	FinishedAt int64    `json:"finishedAt"`
	Preset     string       `json:"preset,omitempty"`
	ConfigHash string       `json:"configHash,omitempty"`
	Config     *SeedConfig  `json:"config,omitempty"`
	Summary    *SeedSummary `json:"summary,omitempty"`
	Stages     map[string][]string `json:"stages,omitempty"` // 模块 → 已完成阶段
	Logs       []string `json:"logs"`
	Accounts   map[string]string `json:"accounts,omitempty"` // 完成后关键地址
}

// Seed 启动播种任务(异步,立即返回 jobId)。
func (h *SeedJobHandler) Seed(c *gin.Context) {
	var req seedStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "Seed", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}
	cfg, err := seedResolveConfig(req)
	if err != nil {
		logParamError(c, "Seed", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	if errs := ValidateSeedConfig(cfg); len(errs) > 0 {
		logParamError(c, "Seed", fmt.Errorf("%v", errs))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "配置校验失败", errs))
		return
	}
	modules, err := seedNormalizeModules(req.Modules)
	if err != nil {
		logParamError(c, "Seed", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	plan, err := BuildSeedPlanV2(cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}

	mc := middleware.ClientFrom(c)
	if mc == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "no current network", nil))
		return
	}

	h.jobsMu.Lock()
	if h.runningID != "" {
		job := h.jobs[h.runningID]
		status := "running"
		if job != nil {
			status = job.Status
		}
		h.jobsMu.Unlock()
		c.JSON(http.StatusConflict, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
			fmt.Sprintf("已有播种任务在运行(id=%s, status=%s);等待完成或重启后重试", h.runningID, status), nil))
		return
	}
	jobID := fmt.Sprintf("%d", time.Now().UnixNano())
	job := &seedJob{
		ID: jobID, Scale: req.Scale, Modules: modules, SeedPhrase: cfg.SeedPhrase,
		Status: "running", StartedAt: time.Now().UnixMilli(),
		Preset: req.Preset, ConfigHash: plan.ConfigHash, plan: &plan,
	}
	h.jobs[jobID] = job
	h.runningID = jobID
	h.jobsMu.Unlock()

	statePath := h.statePath
	go func() {
		// 后台任务防护:任何 panic 转为任务失败,不得拖崩整个进程
		defer func() {
			if rec := recover(); rec != nil {
				job.addLog("✗ panic: %v", rec)
				job.mu.Lock()
				job.Status = "failed"
				job.Error = fmt.Sprintf("panic: %v", rec)
				job.FinishedAt = time.Now().UnixMilli()
				job.mu.Unlock()
				h.jobsMu.Lock()
				if h.runningID == jobID {
					h.runningID = ""
				}
				h.jobsMu.Unlock()
			}
		}()
		r := &seedRunner{
			mc:   mc,
			plan: plan,
			state: func() *SeedState {
				st := LoadSeedState(statePath)
				if req.Reset {
					st.Reset()
					_ = st.Save()
				}
				st.UsePrefix(plan.ConfigHash)
				return st
			}(),
			logf: job.addLog,
		}
		if err := r.run(modules); err != nil {
			job.addLog("✗ 失败: %v", err)
			job.mu.Lock()
			job.Status = "failed"
			job.Error = err.Error()
			job.FinishedAt = time.Now().UnixMilli()
			job.mu.Unlock()
		} else {
			job.addLog("============ 播种完成 ============")
			job.mu.Lock()
			job.Status = "completed"
			job.FinishedAt = time.Now().UnixMilli()
			job.mu.Unlock()
		}
		h.jobsMu.Lock()
		if h.runningID == jobID {
			h.runningID = ""
		}
		h.jobsMu.Unlock()
	}()

	c.JSON(http.StatusAccepted, types.SuccessResponse(gin.H{
		"jobId": jobID, "status": "running", "preset": req.Preset,
		"scale": req.Scale, "modules": modules,
	}, "seed started"))
}

// SeedStatus 返回任务进度快照。
func (h *SeedJobHandler) SeedStatus(c *gin.Context) {
	id := c.Param("id")
	h.jobsMu.Lock()
	job, ok := h.jobs[id]
	h.jobsMu.Unlock()
	if !ok {
		c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "job not found: "+id, nil))
		return
	}
	logs := job.logTail() // 先取日志再进临界区:mutex 非重入,嵌套取锁会死锁
	job.mu.Lock()
	snap := seedJobSnapshot{
		ID: job.ID, Scale: job.Scale, Modules: job.Modules, Status: job.Status,
		Error: job.Error, StartedAt: job.StartedAt, FinishedAt: job.FinishedAt,
		Logs: logs,
		Preset: job.Preset, ConfigHash: job.ConfigHash,
	}
	if job.plan != nil {
		// plan 发布后只读,读取无需额外同步
		snap.Config = job.plan.Config
		s := SeedPlanSummary(*job.plan)
		snap.Summary = &s
	}
	job.mu.Unlock()
	snap.Stages = map[string][]string{}
	st := LoadSeedState(h.statePath)
	for module, stages := range st.Data() {
		// v2 模块键为 "<configHash>:<module>":只回显本任务命名空间,显示时剥掉前缀;
		// 无前缀键属 v1 遗留数据,归 configHash 为空的旧任务所有。
		display := module
		if i := strings.Index(module, ":"); i >= 0 {
			if module[:i] != job.ConfigHash {
				continue
			}
			display = module[i+1:]
		} else if job.ConfigHash != "" {
			continue
		}
		names := make([]string, 0, len(stages))
		for name := range stages {
			names = append(names, name)
		}
		snap.Stages[display] = names
	}
	if snap.Status == "completed" && job.plan != nil {
		plan := job.plan
		// 地址表仅展示,私钥永不返回
		if accts, err := SeedBuildAccounts(*plan); err == nil {
			snap.Accounts = map[string]string{}
			snap.Accounts["admin"] = accts["admin"].Address
			for _, u := range plan.Users {
				snap.Accounts[u.Name] = accts[u.Name].Address
			}
			for _, tk := range plan.Tokens {
				snap.Accounts[tk.Symbol] = accts[tk.Symbol].Address
			}
			for _, s := range plan.Sfts {
				snap.Accounts["SFT:"+s.Symbol] = accts[s.Key].Address
			}
		}
	}
	c.JSON(http.StatusOK, types.SuccessResponse(snap, "ok"))
}

// SeedTemplatesList 列出内置模板。
func (h *SeedJobHandler) SeedTemplatesList(c *gin.Context) {
	c.JSON(http.StatusOK, types.SuccessResponse(SeedTemplates(), "ok"))
}

// SeedPlanV2 计划预览(不上链)。body: SeedConfig | {preset, overrides} | {scale}(兼容)。
func (h *SeedJobHandler) SeedPlanV2(c *gin.Context) {
	var req seedStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "SeedPlanV2", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}
	cfg, err := seedResolveConfig(req)
	if err != nil {
		logParamError(c, "SeedPlanV2", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	if errs := ValidateSeedConfig(cfg); len(errs) > 0 {
		logParamError(c, "SeedPlanV2", fmt.Errorf("%v", errs))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
			"配置校验失败", errs))
		return
	}
	plan, err := BuildSeedPlanV2(cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"plan": plan, "summary": SeedPlanSummary(plan),
	}, "ok"))
}

// SeedPlanPreview 返回计划预览(不上链)。
func (h *SeedJobHandler) SeedPlanPreview(c *gin.Context) {
	scale := c.DefaultQuery("scale", "full")
	if scale != "full" && scale != "small" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "scale must be full or small", nil))
		return
	}
	plan, err := BuildSeedPlan(scale, c.DefaultQuery("seedPhrase", SeedDefaultPhrase))
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(plan, "ok"))
}

// seedNormalizeModules 校验并补齐模块依赖(compliance → vc)。
func seedNormalizeModules(modules []string) ([]string, error) {
	valid := map[string]bool{}
	for _, m := range SeedModuleOrder {
		valid[m] = true
	}
	if len(modules) == 0 {
		return append([]string{}, SeedModuleOrder...), nil
	}
	has := map[string]bool{}
	for _, m := range modules {
		if !valid[m] {
			return nil, fmt.Errorf("未知模块 %q", m)
		}
		has[m] = true
	}
	if has["compliance"] && !has["vc"] {
		has["vc"] = true
	}
	out := []string{}
	for _, m := range SeedModuleOrder {
		if has[m] {
			out = append(out, m)
		}
	}
	return out, nil
}

var _ = milon.WithRequestID // 保留 SDK 引用
