// Package mcpserver 把现有 REST 能力薄适配为 MCP 工具。
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ctxKeyNetwork 是请求级网络名在 context 里的载体（Task 4 请求头方案：
// /mcp 请求的 X-Milon-Network 头注入，executor 回环透传，与 REST 侧同构）。
type ctxKeyNetwork struct{}

// networkFromCtx 取请求级网络名（来自 /mcp 请求的 X-Milon-Network 头）；
// 空串 = 未指定（回环不带头，走服务端默认网络）。
func networkFromCtx(ctx context.Context) string {
	s, _ := ctx.Value(ctxKeyNetwork{}).(string)
	return s
}

// RESTMapping 声明一个 MCP 工具对应的 REST 调用。
type RESTMapping struct {
	Method       string // "GET" / "POST" / "PUT" / "DELETE"
	PathTemplate string // 如 "/api/transactions/{hash}"
	PathParams   []string
	QueryParams  []string // 若 handler 读 c.Query，则列入；渲染为 query string
}

// CallOutcome 是一次 REST 调用的结果；IsError=true 时 Body 为 REST 错误 JSON 原文。
type CallOutcome struct {
	IsError bool
	Body    []byte
}

type Executor struct {
	baseURL string
	http    *http.Client
}

func NewExecutor(baseURL string, httpClient *http.Client) *Executor {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &Executor{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

func (e *Executor) Call(ctx context.Context, m RESTMapping, argsJSON json.RawMessage) (*CallOutcome, error) {
	var args map[string]any
	if len(argsJSON) > 0 {
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("工具参数不是合法 JSON 对象: %w", err)
		}
	}
	path := m.PathTemplate
	for _, p := range m.PathParams {
		v, ok := args[p]
		if !ok {
			return nil, fmt.Errorf("缺少必填参数 %s", p)
		}
		path = strings.ReplaceAll(path, "{"+p+"}", url.PathEscape(fmt.Sprintf("%v", v)))
		delete(args, p)
	}
	query := url.Values{}
	for _, q := range m.QueryParams {
		if v, ok := args[q]; ok {
			query.Set(q, fmt.Sprintf("%v", v))
			delete(args, q)
		}
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	// body 方法：POST 与 PUT 都把剩余参数组为 JSON body；GET/DELETE 无 body。
	hasBody := m.Method == "POST" || m.Method == "PUT"
	var body io.Reader
	if hasBody {
		b, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, m.Method, e.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	// 请求级网络（Task 4）：/mcp 请求带了 X-Milon-Network 就透传给回环 REST，
	// 由 REST 侧 ResolveNetwork 中间件解析——executor 自身不校验网络名合法性。
	if net := networkFromCtx(ctx); net != "" {
		req.Header.Set("X-Milon-Network", net)
	}
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("milon REST 后端不可达: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	return &CallOutcome{IsError: resp.StatusCode >= 300, Body: data}, nil
}
