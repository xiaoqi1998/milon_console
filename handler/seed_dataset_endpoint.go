package handler

import (
	"net/http"
	"os"
	"sort"
	"strings"

	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// ==================== 播种器:数据集总览(状态文件只读视图) ====================
// GET /api/tool/seed/dataset — 读断点状态文件,按 configHash 命名空间汇总,
// 供总览页展示"当前生效数据集"(各模块完成阶段 + 代币/SFT 地址等原始产物)。
// 只读不改状态;产物原样透出,展示层自行取用。

type seedDatasetEntry struct {
	ConfigHash string                    `json:"configHash"` // v2 为 16 hex;v1 遗留(无前缀键)为 ""
	Modules    map[string]map[string]any `json:"modules"`    // 模块 → 阶段 → 产物
	StageCount int                       `json:"stageCount"`
}

func (h *SeedJobHandler) SeedDataset(c *gin.Context) {
	st := LoadSeedState(h.statePath)
	entries := map[string]*seedDatasetEntry{}
	get := func(hash string) *seedDatasetEntry {
		if entries[hash] == nil {
			entries[hash] = &seedDatasetEntry{ConfigHash: hash, Modules: map[string]map[string]any{}}
		}
		return entries[hash]
	}
	for key, stages := range st.Data() {
		hash := ""
		module := key
		if i := strings.Index(key, ":"); i >= 0 {
			hash, module = key[:i], key[i+1:]
		}
		e := get(hash)
		e.Modules[module] = stages
		e.StageCount += len(stages)
	}
	hashes := make([]string, 0, len(entries))
	for hash := range entries {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)
	out := make([]seedDatasetEntry, 0, len(hashes))
	for _, hash := range hashes {
		out = append(out, *entries[hash])
	}
	var updatedAt int64
	if fi, err := os.Stat(h.statePath); err == nil {
		updatedAt = fi.ModTime().UnixMilli()
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"datasets":  out,
		"updatedAt": updatedAt,
	}, "ok"))
}
