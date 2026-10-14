# MCP 客户端配置操作账户 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用户在自己 MCP 客户端配置（mcp.json `headers.X-Milon-Accounts`）携带多账户（地址+私钥，fndsa512 抗量子带公钥），服务端按请求过境解析，全部私钥入口支持「地址即引用」免私钥调用，多用户零互串。

**Architecture:** 复用请求级网络头三段式通道：`/mcp` 外层包装把 header 原文注入 ctx → `Executor.Call` 回环透传 → REST 侧 `ResolveAccounts` 中间件 base64/JSON 解析+校验进 gin context → 各 handler 在「私钥缺失」分支按地址查本请求账户集回填。服务端零持久化、零跨请求共享。

**Tech Stack:** Go + gin + milon-go-sdk（crypto 四曲线）；测试 `httptest` fake RPC 模式（参照 `handler/write_safe_test.go`）。

**Spec:** `docs/superpowers/specs/2026-10-14-mcp-account-config-design.md`

## Global Constraints

- 全程简体中文：注释、错误文案、commit message
- 构建 `CGO_ENABLED=1 go build ./...`（blst 依赖必须开 cgo）
- 显式私钥优先于配置账户（请求带了私钥就用请求的）
- 配置账户永不落盘、永不进日志、任何响应不回私钥
- 现有 63 个 MCP 工具行为不变（不带 header 时全链路零变化）
- 测试固定密钥素材（已验证可用，来自 write_safe_test.go）：secp256k1 私钥 hex `000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f`、公钥 hex `0x02f175c4673255dc8e674d70c3d6d45550ccebafb745fbd386fa52ce6f74bb2ef6`、地址 `2DSTxAj1h6JtrhyayNFo4heZBa29`
- fndsa512 测试用 `crypto.NewFnDsa512SecretKey()` 运行时生成（服务端 account_generate 同路径，速度可接受）
- header 名常量：`X-Milon-Accounts`；env 名：`MILON_ACCOUNTS`（raw JSON，非 base64）

---

### Task 1: types.ParseConfiguredAccounts — 配置账户解析与校验

**Files:**
- Create: `types/accounts.go`
- Test: `types/accounts_test.go`

**Interfaces:**
- Consumes: `types.ParseAddress`、`types.ParseSecretKey`、`types.ParsePublicKey`、`types.derivePubKeyByType`（均在 `types/conversion.go`，同包内可用私钥函数）
- Produces: `type ConfiguredAccount struct { Name, Address, PrivateKey, PublicKey, KeyType string }`（PublicKey 归一为 hex 全文，KeyType 归一为非空四曲线之一）；`func ParseConfiguredAccounts(raw []byte) ([]ConfiguredAccount, error)` —— 错误文案以 `X-Milon-Accounts[i]:` 开头定位条目

- [ ] **Step 1: 写失败测试**

```go
// types/accounts_test.go
package types

import (
	"strings"
	"testing"
)

const tPrivHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
const tAddr = "2DSTxAj1h6JtrhyayNFo4heZBa29"
const tPubHex = "0x02f175c4673255dc8e674d70c3d6d45550ccebafb745fbd386fa52ce6f74bb2ef6"

func TestParseConfiguredAccounts(t *testing.T) {
	t.Run("单账户解析且公钥地址一致", func(t *testing.T) {
		raw := `[{"name":"main","address":"` + tAddr + `","privateKey":"` + tPrivHex + `"}]`
		got, err := ParseConfiguredAccounts([]byte(raw))
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(got) != 1 || got[0].Address != tAddr || got[0].PrivateKey != tPrivHex {
			t.Fatalf("got %+v", got)
		}
		if got[0].KeyType != "secp256k1" {
			t.Fatalf("KeyType 应缺省归一为 secp256k1, got %q", got[0].KeyType)
		}
		if got[0].PublicKey == "" {
			t.Fatal("PublicKey 应被派生回填（hex 全文），供指纹与下游注入")
		}
	})
	t.Run("公钥显式传入且地址一致", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","publicKey":"` + tPubHex + `"}]`
		got, err := ParseConfiguredAccounts([]byte(raw))
		if err != nil || len(got) != 1 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("公钥地址不一致拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","publicKey":"0x02f175c4673255dc8e674d70c3d6d45550ccebafb745fbd386fa52ce6f74bb2ef5"}]`
		_, err := ParseConfiguredAccounts([]byte(raw))
		if err == nil || !strings.Contains(err.Error(), "X-Milon-Accounts[0]") {
			t.Fatalf("want 定位错误, got %v", err)
		}
	})
	t.Run("私钥派生地址不一致拒绝", func(t *testing.T) {
		raw := `[{"address":"NotABase58Addr!!","privateKey":"` + tPrivHex + `"}]`
		if _, err := ParseConfiguredAccounts([]byte(raw)); err == nil {
			t.Fatal("地址非法应拒绝")
		}
	})
	t.Run("fndsa512 缺 publicKey 拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","keyType":"fndsa512"}]`
		_, err := ParseConfiguredAccounts([]byte(raw))
		if err == nil || !strings.Contains(err.Error(), "fndsa512") {
			t.Fatalf("want fndsa512 提示, got %v", err)
		}
	})
	t.Run("非法 keyType 拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","keyType":"rsa"}]`
		if _, err := ParseConfiguredAccounts([]byte(raw)); err == nil {
			t.Fatal("非法曲线应拒绝")
		}
	})
	t.Run("地址重复拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `"},
		         {"address":"` + tAddr + `","privateKey":"` + tPrivHex + `"}]`
		if _, err := ParseConfiguredAccounts([]byte(raw)); err == nil {
			t.Fatal("重复地址应拒绝")
		}
	})
	t.Run("空数组放行", func(t *testing.T) {
		got, err := ParseConfiguredAccounts([]byte(`[]`))
		if err != nil || len(got) != 0 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("坏 JSON 拒绝", func(t *testing.T) {
		if _, err := ParseConfiguredAccounts([]byte(`{not json`)); err == nil {
			t.Fatal("坏 JSON 应拒绝")
		}
	})
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./types/ -run TestParseConfiguredAccounts -v`
Expected: FAIL（`ParseConfiguredAccounts` 未定义）

- [ ] **Step 3: 最小实现**

```go
// types/accounts.go
package types

import (
	"encoding/json"
	"fmt"

	"github.com/milon-labs/milon-go-sdk/crypto"
)

// ConfiguredAccount 是客户端配置的操作账户（X-Milon-Accounts / MILON_ACCOUNTS
// 的数组元素）。解析归一后 PublicKey 恒为 hex 全文（显式或从私钥派生），
// KeyType 恒为四曲线之一（缺省 secp256k1）——下游指纹与注入统一取用。
type ConfiguredAccount struct {
	Name       string `json:"name,omitempty"`
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey,omitempty"`
	KeyType    string `json:"keyType,omitempty"`
}

// ParseConfiguredAccounts 解析并校验一段账户 JSON 数组（raw 为 base64 解码后
// 的 JSON 原文，或 MILON_ACCOUNTS 环境变量原文）。
// 校验（任一失败即整包拒绝，错误定位到条目下标）：
//  1. address 非空且为合法地址；
//  2. keyType 归一到四曲线之一（secp256k1/ed25519/bls12381/fndsa512）；
//  3. privateKey 非空且可解析；
//  4. 公钥确定：显式 publicKey 优先；fndsa512 缺 publicKey 直接拒绝
//     （Go 无法从其私钥派生）；其余曲线从私钥按曲线派生；
//  5. 公钥派生地址须与 address 一致（防配错 key）；
//  6. 数组内地址唯一。
func ParseConfiguredAccounts(raw []byte) ([]ConfiguredAccount, error) {
	var accounts []ConfiguredAccount
	if err := json.Unmarshal(raw, &accounts); err != nil {
		return nil, fmt.Errorf("X-Milon-Accounts 不是合法 JSON 数组: %w", err)
	}
	seen := make(map[string]bool, len(accounts))
	for i := range accounts {
		a := &accounts[i]
		where := fmt.Sprintf("X-Milon-Accounts[%d]", i)
		if a.Address == "" {
			return nil, fmt.Errorf("%s: address is required", where)
		}
		addr, err := ParseAddress(a.Address)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid address: %w", where, err)
		}
		if a.KeyType == "" {
			a.KeyType = "secp256k1"
		}
		switch a.KeyType {
		case "secp256k1", "ed25519", "bls12381", "fndsa512":
		default:
			return nil, fmt.Errorf("%s: unsupported keyType %q (supported: secp256k1/ed25519/bls12381/fndsa512)", where, a.KeyType)
		}
		if a.PrivateKey == "" {
			return nil, fmt.Errorf("%s: privateKey is required", where)
		}
		sk, err := ParseSecretKey(a.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid privateKey: %w", where, err)
		}
		var pk *crypto.PublicKey
		if a.PublicKey != "" {
			pk, err = ParsePublicKey(a.PublicKey)
			if err != nil {
				return nil, fmt.Errorf("%s: invalid publicKey: %w", where, err)
			}
		} else if a.KeyType == "fndsa512" {
			return nil, fmt.Errorf("%s: keyType fndsa512 必须显式携带 publicKey（Go 无法从私钥派生；account_generate 的返回里有）", where)
		} else {
			pk, err = derivePubKeyByType(sk, a.KeyType)
			if err != nil {
				return nil, fmt.Errorf("%s: 无法从私钥派生公钥: %w", where, err)
			}
		}
		derived, err := crypto.NewAddressFromPublicKey(pk)
		if err != nil {
			return nil, fmt.Errorf("%s: 公钥无法派生地址: %w", where, err)
		}
		if derived.String() != addr.String() {
			return nil, fmt.Errorf("%s: 公钥派生地址 %s 与配置 address %s 不一致（key 配错）", where, derived.String(), a.Address)
		}
		a.PublicKey = pk.ToHex() // 归一为 hex 全文，供指纹与注入
		if seen[a.Address] {
			return nil, fmt.Errorf("%s: 重复地址 %s", where, a.Address)
		}
		seen[a.Address] = true
	}
	return accounts, nil
}
```

注：`crypto.Address` 的字符串化方法若非 `String()`（可能是 `ToBase58()`），以编译器提示为准改为对应方法——两处 `derived.String()`/`addr.String()` 同步。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./types/ -run TestParseConfiguredAccounts -v`
Expected: PASS 全部子测试

- [ ] **Step 5: Commit**

```bash
git add types/accounts.go types/accounts_test.go
git commit -m "feat(types): 配置账户解析校验 ParseConfiguredAccounts——四曲线归一/fndsa512 必带公钥/公钥地址一致性/地址唯一，错误定位到条目下标"
```

---

### Task 2: middleware.ResolveAccounts — header/env 解析进请求 context

**Files:**
- Create: `middleware/accounts.go`
- Test: `middleware/accounts_test.go`

**Interfaces:**
- Consumes: `types.ParseConfiguredAccounts`（Task 1）
- Produces:
  - `const AccountsHeader = "X-Milon-Accounts"`、`const AccountsEnv = "MILON_ACCOUNTS"`
  - `func ResolveAccounts() gin.HandlerFunc`（挂 `/api` 组）
  - `func AccountsFrom(c *gin.Context) []types.ConfiguredAccount`（无则 nil）
  - `func AccountsSourceFrom(c *gin.Context) string`（"header" / "env" / ""）

- [ ] **Step 1: 写失败测试**

```go
// middleware/accounts_test.go
package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

func b64(payload string) string { return base64.StdEncoding.EncodeToString([]byte(payload)) }

func newAccountsRouter() (*gin.Engine, *[]types.ConfiguredAccount) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api", ResolveAccounts())
	var got []types.ConfiguredAccount
	api.GET("/probe", func(c *gin.Context) {
		got = AccountsFrom(c)
		c.JSON(200, gin.H{"source": AccountsSourceFrom(c), "n": len(got)})
	})
	return r, &got
}

const tPrivHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
const tAddr = "2DSTxAj1h6JtrhyayNFo4heZBa29"

func TestResolveAccounts(t *testing.T) {
	payload := `[{"name":"main","address":"` + tAddr + `","privateKey":"` + tPrivHex + `"}]`

	t.Run("带 header 解析注入", func(t *testing.T) {
		r, got := newAccountsRouter()
		req := httptest.NewRequest("GET", "/api/probe", nil)
		req.Header.Set(AccountsHeader, b64(payload))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		if len(*got) != 1 || (*got)[0].Address != tAddr {
			t.Fatalf("got %+v", *got)
		}
		var resp struct{ Source string `json:"source"` }
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Source != "header" {
			t.Fatalf("source want header got %q", resp.Source)
		}
	})
	t.Run("无 header 无 env 放行空集", func(t *testing.T) {
		t.Setenv(AccountsEnv, "")
		r, got := newAccountsRouter()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/probe", nil))
		if w.Code != 200 || len(*got) != 0 {
			t.Fatalf("code=%d got %+v", w.Code, *got)
		}
	})
	t.Run("无 header 有 env 生效", func(t *testing.T) {
		t.Setenv(AccountsEnv, payload) // env 是 raw JSON，非 base64
		r, got := newAccountsRouter()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/probe", nil))
		if w.Code != 200 || len(*got) != 1 {
			t.Fatalf("code=%d got %+v", w.Code, *got)
		}
	})
	t.Run("header 优先于 env", func(t *testing.T) {
		t.Setenv(AccountsEnv, `[]`)
		r, got := newAccountsRouter()
		req := httptest.NewRequest("GET", "/api/probe", nil)
		req.Header.Set(AccountsHeader, b64(payload))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if len(*got) != 1 {
			t.Fatalf("header 应优先, got %+v", *got)
		}
	})
	t.Run("坏 base64 400", func(t *testing.T) {
		r, _ := newAccountsRouter()
		req := httptest.NewRequest("GET", "/api/probe", nil)
		req.Header.Set(AccountsHeader, "!!!not-base64!!!")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "X-Milon-Accounts") {
			t.Fatalf("want 400+定位 got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("校验失败 400 且定位条目", func(t *testing.T) {
		r, _ := newAccountsRouter()
		req := httptest.NewRequest("GET", "/api/probe", nil)
		req.Header.Set(AccountsHeader, b64(`[{"address":"`+tAddr+`","privateKey":"`+tPrivHex+`","keyType":"fndsa512"}]`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "X-Milon-Accounts[0]") {
			t.Fatalf("want 400+条目定位 got %d: %s", w.Code, w.Body.String())
		}
	})
}

// 不变量：账户头与私钥永不进日志。logging.go 只记 body 不记 header，本测试锁死该事实。
func TestAccountsHeaderNeverLogged(t *testing.T) {
	var buf strings.Builder
	old := redirectLogOutput(&buf) // 见下方辅助；无此辅助则按 Step 3 补
	defer old()
	r, _ := newAccountsRouter()
	req := httptest.NewRequest("GET", "/api/probe", nil)
	hv := b64(`[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `"}]`)
	req.Header.Set(AccountsHeader, hv)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out := buf.String()
	if strings.Contains(out, tPrivHex) || strings.Contains(out, hv) {
		t.Fatalf("日志泄露账户/私钥: %s", out)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./middleware/ -run 'TestResolveAccounts|TestAccountsHeaderNeverLogged' -v`
Expected: FAIL（`ResolveAccounts` 等未定义；`redirectLogOutput` 未定义）

- [ ] **Step 3: 最小实现**

```go
// middleware/accounts.go
package middleware

import (
	"encoding/base64"
	"net/http"
	"os"

	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// 请求级配置账户（spec: 2026-10-14-mcp-account-config）：
// 客户端在 MCP 配置 headers 里携带 X-Milon-Accounts（base64(JSON 数组)），
// 服务端按请求过境解析——零持久化、零跨请求共享，多用户天然隔离。
// 自托管单用户可用 MILON_ACCOUNTS 环境变量（raw JSON）兜底；header 优先。

const (
	// AccountsHeader 是客户端配置账户头名。
	AccountsHeader = "X-Milon-Accounts"
	// AccountsEnv 是自托管环境变量名（raw JSON，非 base64）。
	AccountsEnv = "MILON_ACCOUNTS"
	// CtxAccountsKey / CtxAccountsSourceKey 是 context 注入键。
	CtxAccountsKey       = "milon_configured_accounts"
	CtxAccountsSourceKey = "milon_configured_accounts_source"
)

// ResolveAccounts 解析账户头（或 env 兜底）注入 context；校验失败 400 快速失败
// （错误定位到 X-Milon-Accounts[i] 条目）。挂在 /api 组，与 ResolveNetwork 并列。
func ResolveAccounts() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader(AccountsHeader)
		source := "header"
		if raw == "" {
			raw = os.Getenv(AccountsEnv)
			if raw == "" {
				c.Next() // 无配置账户：现有行为完全不变
				return
			}
			source = "env"
		} else {
			decoded, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest,
					types.ErrorResponse(types.ERR_INVALID_PARAMETER, "X-Milon-Accounts 不是合法 base64（应为 base64(JSON 数组)）: "+err.Error(), nil))
				return
			}
			raw = string(decoded)
		}
		accounts, err := types.ParseConfiguredAccounts([]byte(raw))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
			return
		}
		c.Set(CtxAccountsKey, accounts)
		c.Set(CtxAccountsSourceKey, source)
		c.Next()
	}
}

// AccountsFrom 取当前请求的配置账户集；未携带时返回 nil。
func AccountsFrom(c *gin.Context) []types.ConfiguredAccount {
	if v, ok := c.Get(CtxAccountsKey); ok {
		if a, ok := v.([]types.ConfiguredAccount); ok {
			return a
		}
	}
	return nil
}

// AccountsSourceFrom 取账户来源（"header"/"env"/""）。
func AccountsSourceFrom(c *gin.Context) string {
	s, _ := c.Get(CtxAccountsSourceKey).(string)
	return s
}
```

同文件补日志捕获辅助（供 never-logged 测试）：

```go
// redirectLogOutput 把默认 slog 输出重定向到 buf，返回恢复函数。
func redirectLogOutput(buf *strings.Builder) func() {
	old := slog.Default()
	logger := slog.New(slog.NewTextHandler(buf, nil))
	slog.SetDefault(logger)
	return func() { slog.SetDefault(old) }
}
```

（import 补 `log/slog` 与 `strings`。）

- [ ] **Step 4: 挂载到 main.go**

`main.go` 约 62 行 `api.Use(middleware.ResolveNetwork(nm))` 之后加一行：

```go
	api.Use(middleware.ResolveAccounts())
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./middleware/ -run 'TestResolveAccounts|TestAccountsHeaderNeverLogged' -v && CGO_ENABLED=1 go build ./...`
Expected: PASS；构建成功

- [ ] **Step 6: Commit**

```bash
git add middleware/accounts.go middleware/accounts_test.go main.go
git commit -m "feat(middleware): ResolveAccounts 请求级账户解析——base64/env 双源,校验失败 400 定位条目,零持久化注入 gin context,锁死日志无私钥不变量"
```

---

### Task 3: handler 回填助手 — accountRole / fillFromConfiguredAccounts

**Files:**
- Create: `handler/accounts_fill.go`
- Test: `handler/accounts_fill_test.go`

**Interfaces:**
- Consumes: `middleware.AccountsFrom`（Task 2）、`types.ConfiguredAccount`（Task 1）
- Produces:
  - `type accountRole struct { Addr, Priv, Pub, KeyType *string; SigMode *json.RawMessage }`
  - `func fillFromConfiguredAccounts(c *gin.Context, roles ...accountRole)` —— 显式私钥优先；地址未命中不动（留给既有校验报错）；命中则回填 Priv/Pub/KeyType，SigMode 为空时注入 `{"type":"pubkey","publicKey":"<hex>"}`
  - `func lookupConfiguredAccount(c *gin.Context, addrStr string) *types.ConfiguredAccount`
  - `const configuredAccountHint = "；该地址未配置为操作账户——可在 MCP 客户端 mcp.json 的 milon.headers 配 X-Milon-Accounts（base64(JSON数组)，fndsa512 须带 publicKey）后免传私钥，或先调 account_config_list 核对已配置账户"`

- [ ] **Step 1: 写失败测试**

```go
// handler/accounts_fill_test.go
package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

const fPrivHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
const fAddr = "2DSTxAj1h6JtrhyayNFo4heZBa29"

func newFillCtx(t *testing.T, accounts []types.ConfiguredAccount) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.CtxAccountsKey, accounts)
	c.Set(middleware.CtxAccountsSourceKey, "header")
	return c
}

func TestFillFromConfiguredAccounts(t *testing.T) {
	acct := types.ConfiguredAccount{Name: "main", Address: fAddr, PrivateKey: fPrivHex,
		PublicKey: "0x02f175c4673255dc8e674d70c3d6d45550ccebafb745fbd386fa52ce6f74bb2ef6", KeyType: "secp256k1"}

	t.Run("空私钥+地址命中回填并注入签名模式", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		var priv, addr, kt string = "", fAddr, ""
		var sm json.RawMessage
		fillFromConfiguredAccounts(c, accountRole{Addr: &addr, Priv: &priv, SigMode: &sm, KeyType: &kt})
		if priv != fPrivHex || kt != "secp256k1" {
			t.Fatalf("priv=%q kt=%q", priv, kt)
		}
		var mode map[string]any
		if err := json.Unmarshal(sm, &mode); err != nil || mode["type"] != "pubkey" {
			t.Fatalf("SigMode 未注入: %v %v", sm, err)
		}
	})
	t.Run("显式私钥优先", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		priv, addr := "explicit", fAddr
		fillFromConfiguredAccounts(c, accountRole{Addr: &addr, Priv: &priv})
		if priv != "explicit" {
			t.Fatalf("显式私钥被覆盖: %q", priv)
		}
	})
	t.Run("地址未命中原样保留", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		priv, addr := "", "SomeOtherAddr1111111111111111111111"
		fillFromConfiguredAccounts(c, accountRole{Addr: &addr, Priv: &priv})
		if priv != "" {
			t.Fatalf("未命中不应回填: %q", priv)
		}
	})
	t.Run("无地址角色跳过", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		priv := ""
		fillFromConfiguredAccounts(c, accountRole{Priv: &priv})
		if priv != "" {
			t.Fatal("无地址不应回填")
		}
	})
	t.Run("fndsa512 回填 Pub 且不动显式 SigMode", func(t *testing.T) {
		fk, fpk, err := newFndsaTestAccount(t)
		if err != nil {
			t.Skipf("fndsa 密钥生成失败: %v", err)
		}
		c := newFillCtx(t, []types.ConfiguredAccount{*fk})
		var priv, pub, kt string
		existing := json.RawMessage(`{"type":"multisig","index":1}`)
		fillFromConfiguredAccounts(c, accountRole{Addr: &fk.Address, Priv: &priv, Pub: &pub, KeyType: &kt, SigMode: &existing})
		if priv != fk.PrivateKey || pub != fk.PublicKey || kt != "fndsa512" {
			t.Fatalf("priv=%q pub=%q kt=%q", priv, pub, kt)
		}
		if string(existing) != `{"type":"multisig","index":1}` {
			t.Fatalf("显式 SigMode 被覆盖: %s", existing)
		}
	})
	t.Run("lookupConfiguredAccount 按地址命中", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		if lookupConfiguredAccount(c, fAddr) == nil {
			t.Fatal("应命中")
		}
		if lookupConfiguredAccount(c, "nope") != nil || lookupConfiguredAccount(c, "") != nil {
			t.Fatal("不应命中")
		}
	})
}
```

同文件补 fndsa 测试账户辅助：

```go
// newFndsaTestAccount 运行时生成一对 fndsa512 密钥并组装为已归一的 ConfiguredAccount。
func newFndsaTestAccount(t *testing.T) (*types.ConfiguredAccount, *crypto.PublicKey, error) {
	t.Helper()
	sk, pk, err := crypto.NewFnDsa512SecretKey()
	if err != nil {
		return nil, nil, err
	}
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		return nil, nil, err
	}
	return &types.ConfiguredAccount{
		Name: "quantum", Address: addr.ToBase58(), PrivateKey: sk.ToHex(),
		PublicKey: pk.ToHex(), KeyType: "fndsa512",
	}, pk, nil
}
```

（import `github.com/milon-labs/milon-go-sdk/crypto`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run TestFillFromConfiguredAccounts -v`
Expected: FAIL（`accountRole`/`fillFromConfiguredAccounts` 未定义）

- [ ] **Step 3: 最小实现**

```go
// handler/accounts_fill.go
package handler

import (
	"encoding/json"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// configuredAccountHint 追加到既有「私钥必填」错误之后，引导用户走配置账户。
const configuredAccountHint = "；该地址未配置为操作账户——可在 MCP 客户端 mcp.json 的 milon.headers 配 X-Milon-Accounts（base64(JSON数组)，fndsa512 须带 publicKey）后免传私钥，或先调 account_config_list 核对已配置账户"

// accountRole 是一个签名角色在请求结构体里的字段锚点（指针指向请求字段）。
// Priv/Pub/SigMode 可为 nil（如 simulate 类请求无私钥字段，仅注入签名模式）。
type accountRole struct {
	Addr    *string
	Priv    *string
	Pub     *string
	KeyType *string
	SigMode *json.RawMessage
}

// lookupConfiguredAccount 按地址查本请求携带的配置账户集。
func lookupConfiguredAccount(c *gin.Context, addrStr string) *types.ConfiguredAccount {
	if addrStr == "" {
		return nil
	}
	accounts := middleware.AccountsFrom(c)
	for i := range accounts {
		if accounts[i].Address == addrStr {
			return &accounts[i]
		}
	}
	return nil
}

// fillFromConfiguredAccounts 「地址即引用」回填：私钥字段为空且地址命中配置账户
// 时注入私钥/公钥/曲线，签名模式为空时注入对应 pubkey 模式（fndsa512 场景关键，
// 其公钥无法从私钥派生）。显式私钥优先；未命中不动——由既有校验报错并附 hint。
func fillFromConfiguredAccounts(c *gin.Context, roles ...accountRole) {
	for _, r := range roles {
		if r.Priv != nil && *r.Priv != "" {
			continue // 显式私钥优先（用户单次意图覆盖常驻配置）
		}
		if r.Addr == nil || *r.Addr == "" {
			continue
		}
		acct := lookupConfiguredAccount(c, *r.Addr)
		if acct == nil {
			continue // 未命中：不回填，留给既有错误路径（文案已升级附 hint）
		}
		if r.Priv != nil {
			*r.Priv = acct.PrivateKey
		}
		if r.KeyType != nil && *r.KeyType == "" {
			*r.KeyType = acct.KeyType
		}
		if r.Pub != nil && *r.Pub == "" {
			*r.Pub = acct.PublicKey
		}
		if r.SigMode != nil && len(*r.SigMode) == 0 {
			m, err := json.Marshal(map[string]string{"type": "pubkey", "publicKey": acct.PublicKey})
			if err == nil {
				*r.SigMode = json.RawMessage(m)
			}
		}
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./handler/ -run TestFillFromConfiguredAccounts -v`
Expected: PASS（fndsa 子测试无 skip 则全绿；skip 亦算通过但需在 commit message 注明）

- [ ] **Step 5: Commit**

```bash
git add handler/accounts_fill.go handler/accounts_fill_test.go
git commit -m "feat(handler): 地址即引用回填助手——显式私钥优先/未命中不动/fndsa512 注入公钥与签名模式,附配置账户 hint 文案"
```

---

### Task 4: contract 家族接线（write/multi/safe/simulate×2）+ 错误文案升级

**Files:**
- Modify: `handler/contract.go`（WriteContract ≈959 行、WriteContractMulti ≈1520 行、WriteSafe ≈1763 行、SimulateContract ≈189 行、SimulateContractMulti ≈1360 行，各自 `ShouldBindJSON` 之后、`validatePaymentModeFields` 之前；错误文案 ≈336/343/1263 行；角色组构造函数放 dispatchSubmit 定义前）
- Modify: `types/conversion.go`（`ParseSignerList` 的 write 模式缺私钥错误 ≈177 行）
- Test: `handler/accounts_contract_test.go`

**Interfaces:**
- Consumes: `fillFromConfiguredAccounts`、`accountRole`、`configuredAccountHint`（Task 3）
- Produces: `func (h *ContractHandler) fillWriteContractRequest(c *gin.Context, req *writeContractRequest)`（WriteContract/WriteSafe 共用——WriteSafe ≈1765 行 bind 的就是 writeContractRequest）；`func (h *ContractHandler) fillMultiContractRequest(c *gin.Context, req *multiContractRequest)`（WriteContractMulti/SimulateContractMulti 共用——≈1300 行注释确认两端点同结构体）；`func (h *ContractHandler) fillSimulateRequest(c *gin.Context, req *simulateContractRequest)`（simulate 单笔，无私钥字段，Priv 传 nil 仅注入 SigMode/KeyType）

- [ ] **Step 1: 写失败测试**

```go
// handler/accounts_contract_test.go
package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

// fake RPC：与 write_safe_test.go 同款——返回协议错误让提交停在链端，
// 证明签名材料组装已通过（错误来自链，不是来自参数校验）。
func newAccountsContractRouter(t *testing.T) *gin.Engine {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq struct {
			RequestID uint64 `json:"request_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&rpcReq)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			RequestID uint64 `json:"request_id"`
			Status    uint8  `json:"status"`
		}{RequestID: rpcReq.RequestID, Status: 1})
	}))
	t.Cleanup(fake.Close)
	gin.SetMode(gin.TestMode)
	nm := client.NewNetworkManager("devNet", fake.URL)
	r := gin.New()
	api := r.Group("/api", middleware.ResolveNetwork(nm), middleware.ResolveAccounts())
	h := NewContractHandler(nm)
	api.POST("/write", h.WriteContract)
	api.POST("/simulate", h.SimulateContract)
	return r
}

func accountsHeader(payload string) string {
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

const cPrivHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
const cAddr = "2DSTxAj1h6JtrhyayNFo4heZBa29"

func TestContractWriteConfiguredAccount(t *testing.T) {
	r := newAccountsContractRouter(t)
	hdr := accountsHeader(`[{"name":"main","address":"` + cAddr + `","privateKey":"` + cPrivHex + `"}]`)
	body := `{"appName":"token","methodName":"Transfer","args":{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1},"paymentMode":"unified_payer_all","payerAddress":"` + cAddr + `"}`

	t.Run("无私钥+地址命中→越过参数校验进入链端路径", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/write", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(middleware.AccountsHeader, hdr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// 关键断言：不再是 400「payerPrivateKey is required」，而是 500 链端错误（fake 返回 Status:1）
		if w.Code == http.StatusBadRequest {
			t.Fatalf("仍被参数校验拦截: %s", w.Body.String())
		}
	})
	t.Run("无私钥+未命中→400 且文案含配置账户提示", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/write", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// 不带账户头（也不设 env），地址未命中
		t.Setenv(middleware.AccountsEnv, "")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		// unified_payer_all 地址已传、无私钥：链上账户未上链时 parsePayerAndMode 要求签名模式——
		// 该错误文案应含 hint 或 signatureMode 指引；此处宽松断言非 200 即可，
		// 严格断言在 split 模式（validatePaymentModeFields 必报 payerPrivateKey）
		body2 := `{"appName":"token","methodName":"Transfer","args":{},"paymentMode":"split","payerAddress":"` + cAddr + `"}`
		req2 := httptest.NewRequest(http.MethodPost, "/api/write", strings.NewReader(body2))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		if w2.Code != http.StatusBadRequest {
			t.Fatalf("split 缺私钥应 400, got %d", w2.Code)
		}
		if !strings.Contains(w2.Body.String(), "account_config_list") && !strings.Contains(w2.Body.String(), "X-Milon-Accounts") {
			t.Fatalf("错误文案缺配置账户提示: %s", w2.Body.String())
		}
	})
	t.Run("simulate 无私钥字段+地址命中+离线账户→注入签名模式组装成功", func(t *testing.T) {
		body3 := `{"appName":"token","methodName":"Transfer","args":{},"paymentMode":"unified_payer_all","payerAddress":"` + cAddr + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/simulate", strings.NewReader(body3))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(middleware.AccountsHeader, hdr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusBadRequest {
			t.Fatalf("simulate 仍被签名模式校验拦截: %s", w.Body.String())
		}
	})
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run TestContractWriteConfiguredAccount -v`
Expected: FAIL（无回填：第一个子测试 400「payerPrivateKey is required」；第三个子测试 400 signatureMode required）

- [ ] **Step 3: 实现——角色组构造器 + 五个入口各一行 + 文案升级**

`handler/contract.go` 新增（放 `dispatchSubmit` 定义之前）：

```go
// fillWriteContractRequest 「地址即引用」：按配置账户回填 write 请求五类角色。
func (h *ContractHandler) fillWriteContractRequest(c *gin.Context, req *writeContractRequest) {
	roles := []accountRole{
		{Addr: &req.PayerAddress, Priv: &req.PayerPrivateKey, SigMode: &req.SignatureMode, KeyType: &req.KeyType},
		{Addr: &req.IxAddress, Priv: &req.IxPrivateKey, SigMode: &req.IxSignatureMode, KeyType: &req.KeyType},
		{Addr: &req.OwnerAddress, Priv: &req.OwnerPrivateKey, KeyType: &req.KeyType},
	}
	for i := range req.Signers {
		s := &req.Signers[i]
		roles = append(roles, accountRole{Addr: &s.Address, Priv: &s.PrivateKey, SigMode: &s.SignatureMode, KeyType: &s.KeyType})
	}
	if req.GasPayer != nil {
		g := req.GasPayer
		roles = append(roles, accountRole{Addr: &g.Address, Priv: &g.PrivateKey, SigMode: &g.SignatureMode, KeyType: &g.KeyType})
	}
	fillFromConfiguredAccounts(c, roles...)
}
```

同文件再写两个（`multiContractRequest` 字段同构但无独立 SigMode 之外的差异；`simulateContractRequest` 无私钥字段，Priv 传 nil）：

```go
// fillMultiContractRequest multi 端点（write/multi 与 simulate/multi 共用请求体）。
func (h *ContractHandler) fillMultiContractRequest(c *gin.Context, req *multiContractRequest) {
	roles := []accountRole{
		{Addr: &req.PayerAddress, Priv: &req.PayerPrivateKey, SigMode: &req.SignatureMode, KeyType: &req.KeyType},
		{Addr: &req.IxAddress, Priv: &req.IxPrivateKey, SigMode: &req.IxSignatureMode, KeyType: &req.KeyType},
		{Addr: &req.OwnerAddress, Priv: &req.OwnerPrivateKey, KeyType: &req.KeyType},
	}
	for i := range req.Signers {
		s := &req.Signers[i]
		roles = append(roles, accountRole{Addr: &s.Address, Priv: &s.PrivateKey, SigMode: &s.SignatureMode, KeyType: &s.KeyType})
	}
	if req.GasPayer != nil {
		g := req.GasPayer
		roles = append(roles, accountRole{Addr: &g.Address, Priv: &g.PrivateKey, SigMode: &g.SignatureMode, KeyType: &g.KeyType})
	}
	fillFromConfiguredAccounts(c, roles...)
}

// fillSimulateRequest simulate 单笔：无私钥字段，回填仅注入签名模式（离线账户
// 原本要求显式 signatureMode，配置账户注入 pubkey 模式后纯地址可模拟）。
func (h *ContractHandler) fillSimulateRequest(c *gin.Context, req *simulateContractRequest) {
	roles := []accountRole{
		{Addr: &req.PayerAddress, SigMode: &req.SignatureMode, KeyType: &req.KeyType},
		{Addr: &req.IxAddress, SigMode: &req.IxSignatureMode, KeyType: &req.KeyType},
		{Addr: &req.OwnerAddress, KeyType: &req.KeyType},
	}
	for i := range req.Signers {
		s := &req.Signers[i]
		roles = append(roles, accountRole{Addr: &s.Address, SigMode: &s.SignatureMode, KeyType: &s.KeyType})
	}
	if req.GasPayer != nil {
		g := req.GasPayer
		roles = append(roles, accountRole{Addr: &g.Address, SigMode: &g.SignatureMode, KeyType: &g.KeyType})
	}
	fillFromConfiguredAccounts(c, roles...)
}
```

入口接线（各在 `ShouldBindJSON` 成功后、`validatePaymentModeFields` 调用前插一行）：

```go
	h.fillWriteContractRequest(c, &req)  // WriteContract ≈985 行 & WriteSafe ≈1775 行
	h.fillMultiContractRequest(c, &req)  // WriteContractMulti ≈1531 行 & SimulateContractMulti ≈1372 行
	h.fillSimulateRequest(c, &req)       // SimulateContract ≈405 行
```

错误文案升级（`configuredAccountHint` 追加）：

- `contract.go` validatePaymentModeFields 内两处：`"payerPrivateKey is required for split mode"` 与 `"payerPrivateKey is required for sponsored mode"` → `fmt.Errorf("payerPrivateKey is required for split mode%s", configuredAccountHint)`（sponsored 同理）
- `contract.go` ≈1263 sponsored 写分支：`"payerPrivateKey is required for sponsored mode"` 同理追加
- `types/conversion.go` `ParseSignerList`：`"signer[%d] privateKey is required for write mode"` → 追加 `%s` + `configuredAccountHint` 常量（在 types 包内新增同名提示串，内容一致；types 不 import handler，提示文案独立定义在 `types/accounts.go`：`const ConfiguredAccountHint = <同 handler 版本文案>`，conversion.go 与 handler 侧均引用各自包内常量以避免环）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./handler/ -run TestContractWriteConfiguredAccount -v && go test ./types/ ./handler/ 2>&1 | tail -5`
Expected: PASS 新测试；既有 handler/types 测试无回归

- [ ] **Step 5: Commit**

```bash
git add handler/contract.go types/conversion.go types/accounts.go handler/accounts_contract_test.go
git commit -m "feat(contract): 写/模拟五入口接入配置账户地址即引用——五类角色回填+签名模式注入,必填错误附配置账户 hint（types 侧独立常量防 import 环）"
```

---

### Task 5: transfer_mil fromAddress + faucet_claim 接线

**Files:**
- Modify: `handler/transfer_mil_handler.go`（transferMilRequest ≈21 行：去掉 PrivateKey 的 `binding:"required"`，新增 FromAddress 字段；TransferMil ≈46 行后接线）
- Modify: `handler/faucet_handler.go`（claim 请求结构 ≈35 行去 required；ClaimFaucet 接线）
- Test: `handler/accounts_transfer_test.go`

**Interfaces:**
- Consumes: Task 3 助手
- Produces: `transferMilRequest.FromAddress string json:"fromAddress,omitempty"`（MCP schema 同步：`transfer_mil` 工具参数增可选 fromAddress）

- [ ] **Step 1: 写失败测试**

```go
// handler/accounts_transfer_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/middleware"
)

// transfer_mil：无私钥+fromAddress 命中 → 越过私钥校验（fake RPC 报链错而非参数错）
func TestTransferMilFromAddressConfigured(t *testing.T) {
	r := newAccountsContractRouter(t) // 复用 Task 4 脚手架（fake RPC + accounts 中间件）
	body := `{"to":"2DSTxAj1h6JtrhyayNFo4heZBa29","amount":1,"fromAddress":"` + cAddr + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/tool/transfer-mil", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.AccountsHeader, accountsHeader(`[{"address":"`+cAddr+`","privateKey":"`+cPrivHex+`"}]`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "privateKey") {
		t.Fatalf("仍被 privateKey 校验拦截: %s", w.Body.String())
	}
	// 未命中：400 且文案含提示
	req2 := httptest.NewRequest(http.MethodPost, "/api/tool/transfer-mil", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	t.Setenv(middleware.AccountsEnv, "")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest || !strings.Contains(w2.Body.String(), "account_config_list") {
		t.Fatalf("want 400+hint got %d: %s", w2.Code, w2.Body.String())
	}
}

// faucet_claim：无私钥+address 命中 → 越过私钥解析
func TestFaucetClaimConfigured(t *testing.T) {
	r := newAccountsContractRouter(t)
	api := r.Routes() // 若无法直接取组，按 Task 4 脚手架在 newAccountsContractRouter 里补挂 /api/faucet/claim 路由
	_ = api
	body := `{"address":"` + cAddr + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/faucet/claim", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.AccountsHeader, accountsHeader(`[{"address":"`+cAddr+`","privateKey":"`+cPrivHex+`"}]`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "privateKey") {
		t.Fatalf("仍被 privateKey 校验拦截: %s", w.Body.String())
	}
}
```

（在 `newAccountsContractRouter` 里补挂两条路由，路径与 main.go 一致：`api.POST("/faucet/claim", NewFaucetHandler(nm).ClaimFaucet)` 与 `api.POST("/tool/transfer-mil", h.TransferMil)`——FaucetHandler 构造签名以 faucet_handler.go 实际为准。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run 'TestTransferMilFromAddressConfigured|TestFaucetClaimConfigured' -v`
Expected: FAIL（binding required 拦截 / 私钥解析报错）

- [ ] **Step 3: 实现**

`transfer_mil_handler.go`：

```go
type transferMilRequest struct {
	To         string `json:"to" binding:"required"`
	Amount     uint64 `json:"amount" binding:"required"`
	// PrivateKey 与 FromAddress 二选一：后者须为已配置操作账户地址（地址即引用）。
	PrivateKey  string `json:"privateKey,omitempty"`
	FromAddress string `json:"fromAddress,omitempty"`
	KeyType     string `json:"keyType,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
}
```

`TransferMil` 在 bind 成功后、`keyType := req.KeyType` 之前插入：

```go
	// 地址即引用：无私钥且 fromAddress 命中配置账户 → 回填私钥/公钥/曲线
	fillFromConfiguredAccounts(c, accountRole{
		Addr: &req.FromAddress, Priv: &req.PrivateKey, Pub: &req.PublicKey, KeyType: &req.KeyType,
	})
	if req.PrivateKey == "" {
		logParamError(c, "TransferMil", fmt.Errorf("privateKey is required（或传 fromAddress 引用已配置账户）"+configuredAccountHint))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
			"privateKey is required（或传 fromAddress 引用已配置账户）"+configuredAccountHint, nil))
		return
	}
```

`faucet_handler.go`：claim 请求结构去掉 PrivateKey 的 `binding:"required"`（保留 Address required）；`ClaimFaucet` 在 bind 后插入：

```go
	fillFromConfiguredAccounts(c, accountRole{
		Addr: &req.Address, Priv: &req.PrivateKey, SigMode: &req.SignatureMode, KeyType: &req.KeyType,
	})
	if req.PrivateKey == "" {
		// 原“必填”语义改为显式校验，附配置账户 hint
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER,
			"privateKey is required（或地址为已配置操作账户，免私钥领水）"+configuredAccountHint, nil))
		return
	}
```

（以 faucet 请求结构实际字段为准——含 SignatureMode json.RawMessage 与 KeyType，见 faucet_handler.go ≈35-38。）

MCP schema 同步：`mcpserver/tools.go` 的 `transferMilArgs` 增 `FromAddress string json:"fromAddress,omitempty"`（jsonschema 描述：「可选：转出方地址（已配置操作账户时免传 privateKey）」）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./handler/ -run 'TestTransferMilFromAddressConfigured|TestFaucetClaimConfigured' -v && go test ./handler/ -run TestTransferMil ./... 2>&1 | tail -3`
Expected: PASS；既有 transfer_mil 测试无回归

- [ ] **Step 5: Commit**

```bash
git add handler/transfer_mil_handler.go handler/faucet_handler.go mcpserver/tools.go handler/accounts_transfer_test.go
git commit -m "feat(transfer/faucet): transfer_mil 增 fromAddress 地址即引用,faucet_claim 按地址回填——binding 放开改显式校验附 hint,MCP schema 同步"
```

---

### Task 6: did ×10 + sft_flow + vc_flow 接线

**Files:**
- Modify: `handler/did_handler.go`（10 个端点各自 bind 后插一行回填）
- Modify: `handler/sft_flow_handler.go`（validate 前）
- Modify: `handler/vc_flow_handler.go`
- Test: `handler/accounts_flows_test.go`

**Interfaces:**
- Consumes: Task 3 助手
- Produces: 无新导出（各 handler 内部接线）

- [ ] **Step 1: 写失败测试（代表三处：did_create / sft_flow owner / vc_flow user）**

```go
// handler/accounts_flows_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/middleware"
)

func TestFlowEntriesConfiguredAccount(t *testing.T) {
	r := newAccountsContractRouter(t)
	hdr := accountsHeader(`[{"address":"` + cAddr + `","privateKey":"` + cPrivHex + `"}]`)
	// 脚手架补挂（路径与 main.go 一致）：
	// api.POST("/tool/did/create", didH.Create)（did 十端点路由前缀 /api/tool/did/*）
	// api.POST("/tool/sft-flow", sftH.SftFlow)

	t.Run("did 缺私钥+地址命中→越过 privateKey is required", func(t *testing.T) {
		body := `{"address":"` + cAddr + `","alias":"tester"}` // privateKey 缺省
		req := httptest.NewRequest(http.MethodPost, "/api/tool/did/create", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(middleware.AccountsHeader, hdr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "privateKey is required") {
			t.Fatalf("仍被私钥校验拦截: %s", w.Body.String())
		}
	})
	t.Run("did 未命中→400 文案含 hint", func(t *testing.T) {
		t.Setenv(middleware.AccountsEnv, "")
		body := `{"address":"` + cAddr + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/tool/did/create", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "account_config_list") {
			t.Fatalf("want 400+hint got %d: %s", w.Code, w.Body.String())
		}
	})
}
```

（sft_flow/vc_flow 断言同型：请求体只给 ownerAddress/userAddress 不给私钥，命中 → 越过「ownerPrivateKey/userPrivateKey is required」；未命中 → 400+hint。路由挂载按 main.go 实际路径补进脚手架。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run TestFlowEntriesConfiguredAccount -v`
Expected: FAIL（私钥必填拦截）

- [ ] **Step 3: 实现**

did 十端点（`didCreateRequest`/`didActionRequest` 系）各自 `ShouldBindJSON` 成功后插：

```go
	fillFromConfiguredAccounts(c, accountRole{Addr: &req.Address, Priv: &req.PrivateKey, Pub: &req.PublicKey})
```

（did 请求无 KeyType 字段——曲线由公钥长度消歧，`resolveFlowParty` 已处理；Pub 回填对经典曲线无害。）

did 必填校验文案升级（`validateDidCreateRequest` ≈190 行等）：`"privateKey is required"` → `"privateKey is required" + configuredAccountHint`（仅 didCreate/各 action 的私钥错误处；地址错误不动）。

sft_flow（≈117 行 validate 前）：

```go
	fillFromConfiguredAccounts(c, accountRole{Addr: &req.OwnerAddress, Priv: &req.OwnerPrivateKey, Pub: &req.OwnerPublicKey})
```

vc_flow（≈67 行 validate 前）：

```go
	fillFromConfiguredAccounts(c,
		accountRole{Addr: &req.IssuerAddress, Priv: &req.IssuerPrivateKey, Pub: &req.IssuerPublicKey},
		accountRole{Addr: &req.UserAddress, Priv: &req.UserPrivateKey, Pub: &req.UserPublicKey},
	)
```

**vc_attestation 不接线**：其 subject 是 `subjectPrivateKey`/`subjectAddress` 二选一且「同时传以地址为准」（≈43-44 行）——地址命中时回填私钥无行为增益、地址缺省时无从回填；issuer 又是本地签发无地址字段。回填在此端点是纯噪音，YAGNI 裁掉（spec「全部私钥入口」按此边界理解，commit message 注明）。

对应必填错误文案各追加 `configuredAccountHint`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./handler/ -run 'TestFlowEntries|TestDid|TestSft|TestVc' -v 2>&1 | tail -8`
Expected: PASS 新测试；既有 did/sft/vc 测试无回归

- [ ] **Step 5: Commit**

```bash
git add handler/did_handler.go handler/sft_flow_handler.go handler/vc_flow_handler.go handler/accounts_flows_test.go
git commit -m "feat(flows): did×10/sft/vc 入口接入地址即引用回填,必填错误附配置账户 hint;vc_attestation 不接线(subject 二选一语义使回填无增益)"
```

---

### Task 7: saved_instruction execute（send 模式）按存储地址回填

**Files:**
- Modify: `handler/saved_instruction_handler.go`（Execute 的 send 路径：req 密钥合并进 item 后、`requireSendKeys` 前）
- Test: `handler/accounts_saved_exec_test.go`

**Interfaces:**
- Consumes: Task 3 助手；`SavedInstruction` 的地址字段（PayerAddress/IxAddress/OwnerAddress/Signers[].Address/GasPayer.Address，已确认存在）
- Produces: 无新导出

- [ ] **Step 1: 写失败测试**

```go
// handler/accounts_saved_exec_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/middleware"
)

// 场景：存储的 saved instruction 绑定 payerAddress=cAddr、paymentMode=unified_payer_all；
// send 模式请求体不带 payerPrivateKey，但请求携带该地址的配置账户 → 回填后越过
// requireSendKeys；未携带 → 400「send 模式需要私钥…」+ hint。
func TestSavedInstructionSendConfiguredAccount(t *testing.T) {
	r, h := newSavedInstrRouter(t) // 脚手架：saved instruction store 临时目录 + 路由挂载（参照 saved_instruction_secrets_test.go 现有模式）
	id := h.mustCreateEntry(t, &SavedInstruction{
		Name: "t", AppName: "token", MethodName: "Transfer", Kind: "entry",
		PaymentMode: "unified_payer_all", PayerAddress: cAddr,
	})
	body := `{"id":"` + id + `","mode":"send"}`

	req := httptest.NewRequest(http.MethodPost, "/api/saved-instructions/"+id+"/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.AccountsHeader, accountsHeader(`[{"address":"`+cAddr+`","privateKey":"`+cPrivHex+`"}]`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "send 模式需要私钥") {
		t.Fatalf("仍被密钥预检拦截: %s", w.Body.String())
	}

	t.Setenv(middleware.AccountsEnv, "")
	req2 := httptest.NewRequest(http.MethodPost, "/api/saved-instructions/"+id+"/execute", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest || !strings.Contains(w2.Body.String(), "account_config_list") {
		t.Fatalf("want 400+hint got %d: %s", w2.Code, w2.Body.String())
	}
}
```

（脚手架 `newSavedInstrRouter`/`mustCreateEntry` 参照 `saved_instruction_secrets_test.go` 的临时 store 模式实现——那个文件已有同型构造。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run TestSavedInstructionSendConfiguredAccount -v`
Expected: FAIL（「send 模式需要私钥：请求体缺少 payerPrivateKey」拦截）

- [ ] **Step 3: 实现**

Execute 的 send 分支：`req` 各密钥字段写入 `item` 之后、`requireSendKeys(item)` 之前插：

```go
	// 地址即引用：存储侧只绑地址无密钥（scrubSecrets 不变量），send 时按地址
	// 从本请求配置账户回填一次性私钥——仍过境即焚，不落盘。
	fillFromConfiguredAccounts(c,
		accountRole{Addr: &item.PayerAddress, Priv: &item.PayerPrivateKey, SigMode: &item.SignatureMode},
		accountRole{Addr: &item.IxAddress, Priv: &item.IxPrivateKey, SigMode: &item.IxSignatureMode},
		accountRole{Addr: &item.OwnerAddress, Priv: &item.OwnerPrivateKey},
	)
	for i := range item.Signers {
		s := &item.Signers[i]
		fillFromConfiguredAccounts(c, accountRole{Addr: &s.Address, Priv: &s.PrivateKey, SigMode: &s.SignatureMode})
	}
	if item.GasPayer != nil {
		g := item.GasPayer
		fillFromConfiguredAccounts(c, accountRole{Addr: &g.Address, Priv: &g.PrivateKey, SigMode: &g.SignatureMode})
	}
```

`requireSendKeys` 各错误文案追加 `configuredAccountHint`（≈476-502 行五处）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./handler/ -run 'TestSavedInstruction' -v 2>&1 | tail -5`
Expected: PASS 全部（含既有 secrets 测试无回归——回填只发生在请求 context，落盘前仍有 scrubSecrets）

- [ ] **Step 5: Commit**

```bash
git add handler/saved_instruction_handler.go handler/accounts_saved_exec_test.go
git commit -m "feat(saved-instructions): send 模式按存储地址从配置账户回填一次性私钥——过境即焚不变量保持,requireSendKeys 错误附 hint"
```

---

### Task 8: account_config_list 工具 + MCP 透传 + 工具计数 63→64

**Files:**
- Create: `handler/accounts_config_handler.go`
- Modify: `main.go`（路由 + 横幅 64）、`mcpserver/executor.go`（ctxKeyAccounts + 透传）、`mcpserver/tools.go`（外层注入 + 工具注册 + 计数注释）、`mcpserver/tools_test.go`（wantToolCount 64 + 新工具名 + 透传隔离用例）、`handler/mcp_tools_handler_test.go`（下限 64）
- Test: `handler/accounts_config_test.go`、`mcpserver/tools_test.go`（追加透传隔离用例）

**Interfaces:**
- Consumes: `middleware.AccountsFrom`/`AccountsSourceFrom`（Task 2）
- Produces:
  - REST `GET /api/accounts-config` → `List`（脱敏：name/address/keyType/publicKeyFingerprint(sha256 前 8 hex)/source）
  - MCP 工具 `account_config_list`（emptyArgs，GET 映射）
  - `mcpserver`：`ctxKeyAccounts` + `accountsFromCtx(ctx) string`

- [ ] **Step 1: 写失败测试**

```go
// handler/accounts_config_test.go
package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/middleware"
)

func TestAccountsConfigList(t *testing.T) {
	r := newAccountsContractRouter(t) // 脚手架补挂：api.GET("/accounts-config", NewAccountsConfigHandler().List)
	req := httptest.NewRequest("GET", "/api/accounts-config", nil)
	req.Header.Set(middleware.AccountsHeader, base64.StdEncoding.EncodeToString(
		[]byte(`[{"name":"main","address":"`+cAddr+`","privateKey":"`+cPrivHex+`"}]`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, banned := range []string{cPrivHex, "privateKey"} {
		if strings.Contains(body, banned) {
			t.Fatalf("响应泄露敏感字段 %q: %s", banned, body)
		}
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || len(resp.Data) != 1 {
		t.Fatalf("data=%v err=%v", resp.Data, err)
	}
	row := resp.Data[0]
	if row["address"] != cAddr || row["keyType"] != "secp256k1" {
		t.Fatalf("row=%v", row)
	}
	fp, _ := row["publicKeyFingerprint"].(string)
	if len(fp) != 8 {
		t.Fatalf("指纹应 8 hex: %q", fp)
	}

	// 空集：提示文案
	req2 := httptest.NewRequest("GET", "/api/accounts-config", nil)
	t.Setenv(middleware.AccountsEnv, "")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if !strings.Contains(w2.Body.String(), "X-Milon-Accounts") {
		t.Fatalf("空集应带配置指引: %s", w2.Body.String())
	}
}
```

`mcpserver/tools_test.go`：`wantToolCount` 63 → 64，`want` 列表加 `"account_config_list"`。

`mcpserver/tools_test.go` 追加多用户隔离用例（复用本文件进程内脚手架模式：`MILON_REST_BASE_URL` 注入 + `httptest.NewServer(NewMCPHandler(""))` 前端 + `rpcCall`，见 ≈122 行既有用例；**注意 handler 包 import mcpserver（mcp_tools_handler.go:6），本包测试不得 import handler，否则成环**——后端用自建记录器，不用真实 handler）：

```go
// 多用户零互串不变量：MCP 侧只透传不落状态——A 请求的账户头只在 A 的回环
// 出现，B 请求绝不受 A 残留影响（ctxKeyAccounts 逐请求注入的最直接证明）。
func TestMCPAccountsHeaderPassthroughIsolation(t *testing.T) {
	var mu sync.Mutex
	var seenHeaders []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenHeaders = append(seenHeaders, r.Header.Get("X-Milon-Accounts"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":[],"message":"ok"}`))
	}))
	t.Cleanup(backend.Close)
	t.Setenv("MILON_REST_BASE_URL", backend.URL)
	front := httptest.NewServer(NewMCPHandler(""))
	t.Cleanup(front.Close)

	hdrA := base64.StdEncoding.EncodeToString([]byte(`[{"address":"AddrA","privateKey":"aa"}]`))
	hdrB := base64.StdEncoding.EncodeToString([]byte(`[{"address":"AddrB","privateKey":"bb"}]`))

	callWith := func(hdr string) {
		req, _ := http.NewRequest(http.MethodPost, front.URL, strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"account_config_list","arguments":{}}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Milon-Accounts", hdr)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("front 不可达: %v", err)
		}
		defer resp.Body.Close()
		_, _ = io.ReadAll(resp.Body)
	}

	callWith(hdrA)
	callWith(hdrB)
	callWith(hdrA)

	mu.Lock()
	defer mu.Unlock()
	if len(seenHeaders) != 3 {
		t.Fatalf("回环请求数=%d want 3: %v", len(seenHeaders), seenHeaders)
	}
	if seenHeaders[0] != hdrA || seenHeaders[1] != hdrB || seenHeaders[2] != hdrA {
		t.Fatalf("账户头透传错序/残留: %v", seenHeaders)
	}
}
```

（import 补 `encoding/base64`、`io`、`net/http`、`strings`、`sync`。服务端 ctx 隔离语义已由 middleware/handler 侧测试覆盖，本测试锁 MCP 透传层逐请求无残留。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./handler/ -run TestAccountsConfigList -v && go test ./mcpserver/ -run 'TestToolsListBasics|TestMCPAccountsHeaderPassthroughIsolation' -v`
Expected: FAIL（handler/路由/工具不存在；计数 63≠64）

- [ ] **Step 3: 实现**

`handler/accounts_config_handler.go`：

```go
// Package handler 内：配置账户脱敏回显。
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// AccountsConfigHandler 回显当前请求（即当前用户客户端配置）携带的操作账户。
type AccountsConfigHandler struct{}

// NewAccountsConfigHandler 创建 handler。
func NewAccountsConfigHandler() *AccountsConfigHandler { return &AccountsConfigHandler{} }

// List handles GET /api/accounts-config（MCP 工具 account_config_list 同源）。
// 不变量：永不返回 privateKey 与 publicKey 全文——公钥只回 sha256 指纹前 8 hex。
func (h *AccountsConfigHandler) List(c *gin.Context) {
	accounts := middleware.AccountsFrom(c)
	source := middleware.AccountsSourceFrom(c)
	rows := make([]gin.H, 0, len(accounts))
	for _, a := range accounts {
		sum := sha256.Sum256([]byte(a.PublicKey))
		rows = append(rows, gin.H{
			"name": a.Name, "address": a.Address, "keyType": a.KeyType,
			"publicKeyFingerprint": hex.EncodeToString(sum[:4]),
			"source":               source,
		})
	}
	msg := "ok"
	if len(accounts) == 0 {
		msg = "本请求未携带配置账户：在 MCP 客户端 mcp.json 的 milon.headers 加 X-Milon-Accounts（base64(JSON数组)，元素 address/privateKey/keyType，fndsa512 须带 publicKey），自托管可设 MILON_ACCOUNTS"
	}
	c.JSON(http.StatusOK, types.SuccessResponse(rows, msg))
}
```

`main.go`：`api.GET("/accounts-config", accountsConfigHandler.List)`（handler 实例化放 ≈57 行区域；横幅 ≈263 行 `64 个工具`）。

`mcpserver/executor.go`：

```go
// ctxKeyAccounts 是 /mcp 请求的 X-Milon-Accounts 头原文载体（executor 回环
// 透传，REST 侧 ResolveAccounts 中间件解析——与网络头同构）。
type ctxKeyAccounts struct{}

func accountsFromCtx(ctx context.Context) string {
	s, _ := ctx.Value(ctxKeyAccounts{}).(string)
	return s
}
```

`Call` 内网络头透传段（≈103-105 行）后加：

```go
	// 配置账户头透传：MCP 侧不校验（校验单一发生在 REST 中间件，规则不漂移）。
	if acc := accountsFromCtx(ctx); acc != "" {
		req.Header.Set("X-Milon-Accounts", acc)
	}
```

`mcpserver/tools.go`：`withNetwork` 包装块（≈767-772 行）加：

```go
		if acc := r.Header.Get("X-Milon-Accounts"); acc != "" {
			r = r.WithContext(context.WithValue(r.Context(), ctxKeyAccounts{}, acc))
		}
```

工具注册（RegisterTools 内，network_list 附近）：

```go
	registerTool[emptyArgs](srv, exec, "account_config_list",
		"列出当前客户端配置的操作账户（mcp.json headers X-Milon-Accounts 携带；脱敏回显地址/曲线/公钥指纹，永不回私钥）；写工具私钥缺省时按地址自动引用这些账户",
		RESTMapping{Method: "GET", PathTemplate: "/api/accounts-config"})
```

计数注释同步：tools.go RegisterTools 头注释（63 → 64，注明 +account_config_list）、tools_test.go `wantToolCount = 64`、`handler/mcp_tools_handler_test.go` 下限 `< 64` 与注释。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./handler/ ./mcpserver/ ./middleware/ ./types/ 2>&1 | tail -6 && CGO_ENABLED=1 go build ./...`
Expected: 全 PASS；构建成功

- [ ] **Step 5: Commit**

```bash
git add handler/accounts_config_handler.go handler/accounts_config_test.go main.go mcpserver/executor.go mcpserver/tools.go mcpserver/tools_test.go handler/mcp_tools_handler_test.go
git commit -m "feat(mcp): account_config_list 脱敏回显工具(63→64)——MCP 外层注入 ctx+executor 回环透传+REST 同源端点;透传隔离测试锁定账户头逐请求无残留"
```

---

### Task 9: 文档同步（README / API.md / guide）

**Files:**
- Modify: `README.md`（部署/配置节：mcp.json headers 配置示例，含 fndsa512 完整示例 + MILON_ACCOUNTS）
- Modify: `API.md`（新端点 GET /api/accounts-config、X-Milon-Accounts 头契约、transfer_mil fromAddress、各写工具「地址即引用」说明、汇总表与 ENDPOINTS 清单）
- Modify: `mcpserver/guide.go`（keys 页新增「客户端配置账户」小节 + workflows 页补一条典型流 + doc_version bump + guide_test.go 锚点断言）

**Interfaces:**
- Consumes: Task 1-8 全部落地行为
- Produces: 文档（无代码接口）

- [ ] **Step 1: 写失败测试（guide 锚点）**

`mcpserver/guide_test.go` 追加（访问器用本文件 `guidePage(topic, q)`，与 TestGuidePageTopics 同款模式；同时把新锚点加进 TestGuidePageTopics 的 anchors 表）：

```go
func TestGuideAccountsSection(t *testing.T) {
	for _, topic := range []string{"keys", "workflows"} {
		page, ok := guidePage(topic, "")
		if !ok {
			t.Fatalf("主题 %s 不存在", topic)
		}
		if !strings.Contains(page, "X-Milon-Accounts") || !strings.Contains(page, "account_config_list") {
			t.Fatalf("%s 页应包含客户端配置账户内容: %.200s", topic, page)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./mcpserver/ -run TestGuideAccountsSection -v`
Expected: FAIL

- [ ] **Step 3: 实现文档**

guide.go：`guideDocVersion` bump（`2026.10.13` → `2026.10.14`）；keys 页末尾追加小节：

```markdown
## 客户端配置账户（地址即引用）
- mcp.json 的 milon.headers 加 X-Milon-Accounts：base64(JSON数组)，元素 {name?, address, privateKey, keyType?, publicKey?}
- fndsa512（抗量子）必须带 publicKey（Go 无法从私钥派生；account_generate 的返回里有）
- 写工具私钥缺省时按地址自动引用；显式私钥优先；account_config_list 脱敏核对（永不回私钥）
- 服务端零存储：账户随每个请求过境，多用户共用一台服务也不会互串
```

workflows 页补一条：`配置账户 → account_config_list 核对 → contract_write 只传地址免私钥`。

README/API.md 按上述 Files 说明落内容（含 mcp.json 完整 JSON 示例、fndsa512 示例、MILON_ACCOUNTS 自托管说明、安全边界三行：零持久化/日志不落/能带 header 即能签名）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./mcpserver/ -run TestGuide -v 2>&1 | tail -4 && CGO_ENABLED=1 go build ./... && go test ./... 2>&1 | tail -10`
Expected: 全绿

- [ ] **Step 5: Commit**

```bash
git add README.md API.md mcpserver/guide.go mcpserver/guide_test.go
git commit -m "docs: 配置账户全链文档——README/API.md 契约与示例(含 fndsa512),guide keys/workflows 页新增小节,doc_version 2026.10.14"
```

---

## 执行顺序与回滚

- Task 1 → 2 → 3 严格串行（依赖链）；Task 4-7 依赖 Task 3 可并行；Task 8 依赖 2；Task 9 收尾
- 每个 Task 独立 commit，任一步出问题 `git revert` 单提交即可回滚，不留半成品状态
