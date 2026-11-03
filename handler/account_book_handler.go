package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// ==================== 账号本:自定义测试账号 CRUD ====================
// 用户自有的密钥库(data/custom_accounts.json,已 gitignore),与播种派生账号并列展示。
// 私钥为用户主动录入(与 X-Milon-Accounts 头 / MILON_ACCOUNTS 环境变量同性质),
// 回显含私钥供控制台复制使用;播种器/账户配置回显的"服务端永不返回私钥"不变量不受影响。
// 仅给私钥的条目按 keyType(默认 secp256k1)走 SDK 派生补地址——与 SeedEnsureAccount 同口径。

// AccountBookEntry 账号本条目。Name 即 ID,同名 upsert 覆盖。
type AccountBookEntry struct {
	Name       string `json:"name"`
	Address    string `json:"address,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
	KeyType    string `json:"keyType,omitempty"` // 缺省按 secp256k1 派生;fndsa512 须自带 publicKey/address
	Note       string `json:"note,omitempty"`
}

// AccountBookHandler 账号本处理器(path 可注入以便测试)。
type AccountBookHandler struct {
	path string
	mu   sync.Mutex
}

// NewAccountBookHandler 创建默认路径的账号本处理器。
func NewAccountBookHandler() *AccountBookHandler {
	return &AccountBookHandler{path: "data/custom_accounts.json"}
}

// accountBookLoad 读盘;缺失/损坏视为空库(与 LoadSeedState 容错口径一致)。
func accountBookLoad(path string) []AccountBookEntry {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var rows []AccountBookEntry
	if json.Unmarshal(raw, &rows) != nil {
		return nil
	}
	return rows
}

// accountBookSave 原子落盘(tmp + rename)。
func accountBookSave(path string, rows []AccountBookEntry) error {
	raw, err := json.MarshalIndent(rows, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// accountBookNormalize 归一多形态导入:平铺数组 | {accounts:[...]} | {owner,users,newUsers}
// (sft_1000_testdata.json 三段式)| 单对象;并补派生地址。
func accountBookNormalize(body []byte) ([]AccountBookEntry, error) {
	var flat []AccountBookEntry
	if err := json.Unmarshal(body, &flat); err != nil || flat == nil {
		var wrap struct {
			Accounts []AccountBookEntry `json:"accounts"`
			Owner    *AccountBookEntry  `json:"owner"`
			Users    []AccountBookEntry `json:"users"`
			NewUsers []AccountBookEntry `json:"newUsers"`
		}
		if err2 := json.Unmarshal(body, &wrap); err2 != nil {
			var single AccountBookEntry
			if err3 := json.Unmarshal(body, &single); err3 != nil || single.Name == "" {
				return nil, errOr(err, err2, err3)
			}
			flat = []AccountBookEntry{single}
		} else {
			flat = flat[:0]
			if wrap.Owner != nil {
				// sft_1000_testdata 实档 owner 无 name 字段:缺名默认 "owner"
				if wrap.Owner.Name == "" {
					wrap.Owner.Name = "owner"
				}
				flat = append(flat, *wrap.Owner)
			}
			flat = append(flat, wrap.Accounts...)
			flat = append(flat, wrap.Users...)
			flat = append(flat, wrap.NewUsers...)
		}
	}
	out := make([]AccountBookEntry, 0, len(flat))
	for i := range flat {
		e := flat[i]
		if e.Name == "" {
			return nil, fmt.Errorf("条目[%d]: 缺 name(同名 upsert, name 即 ID)", i)
		}
		if e.PrivateKey == "" && e.Address == "" {
			return nil, fmt.Errorf("%s: 私钥与地址至少给一", e.Name)
		}
		if e.Address == "" {
			curve := e.KeyType
			if curve == "" {
				curve = "secp256k1"
			}
			acct := &SeedPlanAccount{Name: e.Name, Curve: curve, PrivateKey: e.PrivateKey}
			if err := SeedEnsureAccount(acct); err != nil {
				return nil, fmt.Errorf("%s: %v", e.Name, err)
			}
			e.Address = acct.Address
			if e.PublicKey == "" {
				e.PublicKey = acct.PublicKey
			}
		}
		if e.KeyType == "" {
			e.KeyType = "secp256k1"
		}
		out = append(out, e)
	}
	return out, nil
}

func errOr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return fmt.Errorf("无法识别的请求体")
}

// List GET /api/tool/account-book — 全量条目(按落盘顺序)。
func (h *AccountBookHandler) List(c *gin.Context) {
	h.mu.Lock()
	rows := accountBookLoad(h.path)
	h.mu.Unlock()
	if rows == nil {
		rows = []AccountBookEntry{}
	}
	c.JSON(http.StatusOK, types.SuccessResponse(rows, "ok"))
}

// Upsert POST /api/tool/account-book — 批量/单条录入,同名覆盖,返回全量。
func (h *AccountBookHandler) Upsert(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		logParamError(c, "AccountBookUpsert", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	incoming, err := accountBookNormalize(body)
	if err != nil {
		logParamError(c, "AccountBookUpsert", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	rows := accountBookLoad(h.path)
	idx := map[string]int{}
	for i, r := range rows {
		idx[r.Name] = i
	}
	for _, e := range incoming {
		if at, ok := idx[e.Name]; ok {
			rows[at] = e
		} else {
			rows = append(rows, e)
			idx[e.Name] = len(rows) - 1
		}
	}
	if err := accountBookSave(h.path, rows); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "落盘失败: "+err.Error(), nil))
		return
	}
	if rows == nil {
		rows = []AccountBookEntry{}
	}
	c.JSON(http.StatusOK, types.SuccessResponse(rows, "ok"))
}

// Remove DELETE /api/tool/account-book/:name — 删除单条,返回剩余全量。
func (h *AccountBookHandler) Remove(c *gin.Context) {
	name := c.Param("name")
	h.mu.Lock()
	defer h.mu.Unlock()
	rows := accountBookLoad(h.path)
	out := rows[:0:0]
	found := false
	for _, r := range rows {
		if r.Name == name {
			found = true
			continue
		}
		out = append(out, r)
	}
	if !found {
		c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "条目不存在: "+name, nil))
		return
	}
	if err := accountBookSave(h.path, out); err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "落盘失败: "+err.Error(), nil))
		return
	}
	c.JSON(http.StatusOK, types.SuccessResponse(out, "ok"))
}
