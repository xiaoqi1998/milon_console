package handler

import (
	"fmt"
	"net/http"

	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// ==================== 播种器:派生账户总表(总览页数据源) ====================
// POST /api/tool/seed/accounts — 计划派生账户总表,不上链。
// 服务端只出 名称/角色/曲线/地址/公钥(SDK 口径);私钥列 = sha256(短语:角色:名字),
// 由前端本地派生——沿用"服务端永不返回私钥"不变量(seed_handler.go 地址表同款)。

type seedAccountRow struct {
	Name        string `json:"name"`
	Role        string `json:"role"` // admin | user | issuer | multisig | token | sft
	Curve       string `json:"curve"`
	SubjectType string `json:"subjectType,omitempty"`
	Alias       string `json:"alias,omitempty"`
	Address     string `json:"address,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
}

// SeedAccounts 请求体与 SeedPlanV2 同构(config 直传 / preset+overrides / scale 兼容)。
func (h *SeedJobHandler) SeedAccounts(c *gin.Context) {
	var req seedStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "SeedAccounts", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}
	cfg, err := seedResolveConfig(req)
	if err != nil {
		logParamError(c, "SeedAccounts", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	if errs := ValidateSeedConfig(cfg); len(errs) > 0 {
		logParamError(c, "SeedAccounts", fmt.Errorf("%v", errs))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "配置校验失败", errs))
		return
	}
	plan, err := BuildSeedPlanV2(cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	accts, err := SeedBuildAccounts(plan)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, err.Error(), nil))
		return
	}
	rows := make([]seedAccountRow, 0, len(accts))
	add := func(name string) {
		a := accts[name]
		rows = append(rows, seedAccountRow{
			Name: a.Name, Role: a.Role, Curve: a.Curve,
			SubjectType: a.SubjectType, Alias: a.Alias,
			Address: a.Address, PublicKey: a.PublicKey,
		})
	}
	add(plan.Admin.Name)
	for _, u := range plan.Users {
		add(u.Name)
	}
	for _, i := range plan.Issuers {
		add(i.Name)
	}
	for _, ms := range plan.Multisig {
		add(ms.Name)
	}
	for _, tk := range plan.Tokens {
		add(tk.Symbol)
	}
	for _, cm := range plan.Compliance {
		add(cm.Symbol)
	}
	for _, s := range plan.Sfts {
		add(s.Key)
	}
	c.JSON(http.StatusOK, types.SuccessResponse(gin.H{
		"seedPhrase": plan.SeedPhrase,
		"configHash": plan.ConfigHash,
		"accounts":   rows,
	}, "ok"))
}
