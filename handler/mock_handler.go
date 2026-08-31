package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"milon-api-server/types"

	"github.com/gin-gonic/gin"
)

// MockHandler 提供测试用的 mock 接口：
// POST /api/util/mock/set 保存一段 JSON 并返回专属链接，
// GET /api/util/mock/:id 访问该链接，原样返回当时设置的 JSON。
type MockHandler struct {
	mu    sync.RWMutex
	store map[string]json.RawMessage
}

// NewMockHandler 创建 MockHandler。
func NewMockHandler() *MockHandler {
	return &MockHandler{store: make(map[string]json.RawMessage)}
}

// newMockID 生成 32 位 hex 的唯一 mock id。
func newMockID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetMockResponse handles POST /api/util/mock/set
// 请求体为任意合法 JSON，字节级原样保存，成功后返回一个专属访问链接（链接路径携带唯一 id）。
func (h *MockHandler) SetMockResponse(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logParamError(c, "SetMockResponse", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "failed to read body: "+err.Error(), nil))
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		logParamError(c, "SetMockResponse", fmt.Errorf("request body is required"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "request body is required (valid JSON)", nil))
		return
	}
	if !json.Valid(body) {
		logParamError(c, "SetMockResponse", fmt.Errorf("invalid JSON body"))
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "request body is not valid JSON", nil))
		return
	}

	id, err := newMockID()
	if err != nil {
		logParamError(c, "SetMockResponse", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_INTERNAL, "failed to generate mock id: "+err.Error(), nil))
		return
	}

	h.mu.Lock()
	h.store[id] = append(json.RawMessage(nil), body...)
	h.mu.Unlock()

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/api/util/mock/%s", scheme, c.Request.Host, id)

	c.JSON(http.StatusOK, gin.H{"id": id, "url": url})
}

// GetMockResponseByID handles GET /api/util/mock/:id
// 返回指定 id 对应的 JSON，字节级原样（字段顺序、数字格式、缩进均保持）。
func (h *MockHandler) GetMockResponseByID(c *gin.Context) {
	id := c.Param("id")

	h.mu.RLock()
	body := append(json.RawMessage(nil), h.store[id]...)
	h.mu.RUnlock()

	if len(body) == 0 {
		c.JSON(http.StatusNotFound, types.ErrorResponse(types.ERR_NOT_FOUND, fmt.Sprintf("mock response %q not found: call POST /api/util/mock/set first", id), nil))
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}
