package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ==================== B2:GET /api/tool/vc-by-account ====================
// 查 identity.DisclosedVcs 返回该地址持有(已披露)的证书;issuer 方向由前端
// 聚合(链端无按 issuer 枚举视图)。handler 的 viewFn 可注入以便离线单测。

func vcByAccountTestContext(h *VcByAccountHandler, addr string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/tool/vc-by-account?address="+addr, nil)
	return c, w
}

// TestVcByAccountParseRows 正常解析 DisclosedVcs 返回行。
func TestVcByAccountParseRows(t *testing.T) {
	value := []any{
		map[string]any{
			"credential_schema": "PqSmoke1",
			"issuer":            "3mJsUM2T3ETWu6BsZfLdu6ZYbE1Y",
			"credential_hash":   "0xcfc036",
			"valid_until_ms":    nil,
			"status":            "Active",
			"is_valid":          true,
		},
	}
	rows, err := vcByAccountParseRows(value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 1 || rows[0].Schema != "PqSmoke1" || rows[0].Issuer != "3mJsUM2T3ETWu6BsZfLdu6ZYbE1Y" || !rows[0].IsValid {
		t.Fatalf("行解析不符: %+v", rows)
	}
}

// TestVcByAccountParseRowsEmpty 空列表/缺字段容错。
func TestVcByAccountParseRowsEmpty(t *testing.T) {
	rows, err := vcByAccountParseRows(nil)
	if err != nil || rows != nil {
		t.Fatalf("nil 应得空表: %v %v", rows, err)
	}
	rows, err = vcByAccountParseRows([]any{map[string]any{"credential_schema": "S1"}})
	if err != nil || len(rows) != 1 || rows[0].Schema != "S1" {
		t.Fatalf("缺字段行应容错: %v %v", rows, err)
	}
}

// TestVcByAccountList 端点形状:缺地址 400;正常返回 held 数组。
func TestVcByAccountList(t *testing.T) {
	h := &VcByAccountHandler{viewFn: func(addr string) (any, error) {
		if addr != "3uo5PfaRw8oMD8sZP6CC9j9E1Nzn" {
			t.Fatalf("viewFn 收到意外地址 %s", addr)
		}
		return []any{map[string]any{"credential_schema": "S1", "issuer": "I1", "is_valid": true}}, nil
	}}
	c, w := vcByAccountTestContext(h, "3uo5PfaRw8oMD8sZP6CC9j9E1Nzn")
	h.List(c)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 %d: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); !contains2(body, `"held"`) || !contains2(body, `"S1"`) {
		t.Fatalf("响应缺 held/schema: %s", body)
	}

	// 缺地址 400
	c2, w2 := vcByAccountTestContext(h, "")
	c2.Request.URL.RawQuery = ""
	h.List(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("缺地址应 400, got %d", w2.Code)
	}
}

// TestVcByAccountListBadAddress 非法地址 400。
func TestVcByAccountListBadAddress(t *testing.T) {
	h := &VcByAccountHandler{viewFn: func(addr string) (any, error) {
		t.Fatal("非法地址不应触达链")
		return nil, nil
	}}
	c, w := vcByAccountTestContext(h, "not-base58!!")
	h.List(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法地址应 400, got %d", w.Code)
	}
}
