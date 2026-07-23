package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// MaxResponseSize 限制 HTTP 响应体最大字节数（64MB），防止恶意响应导致 OOM。
const MaxResponseSize = 64 << 20

// defaultHttpClient 是包级复用的 HTTP 客户端，带超时和连接池。
var defaultHttpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DisableCompression:    true,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	},
}

// maxRetries 为网络错误时的最大重试次数（不含首次请求）。
const maxRetries = 3

// retryDelay 为重试之间的固定退避时间。
const retryDelay = time.Second

func HttpPostByBytes(ctx context.Context, url string, dataBytes []byte, header map[string]string) (statusCode int, responseBytes []byte, err error) {
	// 1. Create an HTTP request with context
	var req *http.Request
	if len(dataBytes) > 0 {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(dataBytes))
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	}
	if err != nil {
		return
	}

	// 2. Add request headers
	if header == nil {
		req.Header.Add("Content-Type", "application/json")
	} else {
		for key, value := range header {
			req.Header.Add(key, value)
		}
	}

	// 3. Send request with retry
	var rsp *http.Response
	for i := 0; i < maxRetries; i++ {
		rsp, err = defaultHttpClient.Do(req)
		if err != nil {
			// 重建请求体（Do 会消费 body）
			if len(dataBytes) > 0 {
				req, _ = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(dataBytes))
				if header == nil {
					req.Header.Add("Content-Type", "application/json")
				} else {
					for key, value := range header {
						req.Header.Add(key, value)
					}
				}
			}
			time.Sleep(retryDelay)
			continue
		}
		break
	}
	if err != nil {
		return
	}
	defer rsp.Body.Close()

	statusCode = rsp.StatusCode

	// 4. Read response with size limit
	responseBytes, err = io.ReadAll(io.LimitReader(rsp.Body, MaxResponseSize))

	return
}

func HttpPostByJson(ctx context.Context, url string, data any, header map[string]string) (statusCode int, responseBytes []byte, err error) {
	// 1. Create an HTTP request with context
	var req *http.Request
	var jsonBytes []byte
	if data != nil {
		jsonBytes, err = json.Marshal(data)
		if err != nil {
			return
		}

		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBytes))
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	}
	if err != nil {
		return
	}

	// 2. Add request headers
	if header == nil {
		req.Header.Add("Content-Type", "application/json")
	} else {
		for key, value := range header {
			req.Header.Add(key, value)
		}
	}

	// 3. Send request with retry
	var rsp *http.Response
	for i := 0; i < maxRetries; i++ {
		rsp, err = defaultHttpClient.Do(req)
		if err != nil {
			// 重建请求体（Do 会消费 body）
			if jsonBytes != nil {
				req, _ = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBytes))
				if header == nil {
					req.Header.Add("Content-Type", "application/json")
				} else {
					for key, value := range header {
						req.Header.Add(key, value)
					}
				}
			}
			time.Sleep(retryDelay)
			continue
		}
		break
	}
	if err != nil {
		return
	}
	defer rsp.Body.Close()

	statusCode = rsp.StatusCode

	// 4. Read response with size limit
	responseBytes, err = io.ReadAll(io.LimitReader(rsp.Body, MaxResponseSize))

	return
}
