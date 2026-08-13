package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// maxRetries is the number of attempts for each HTTP request on network errors.
const maxRetries = 3

// httpClient is shared across calls so TCP connections (keep-alive) are reused.
var httpClient = &http.Client{
	Transport: &http.Transport{
		DisableCompression: true,
	},
}

// HttpPostByBytes sends an HTTP POST request with raw bytes as the body.
func HttpPostByBytes(ctx context.Context, url string, dataBytes []byte, header map[string]string) (statusCode int, responseBytes []byte, err error) {
	var body io.Reader
	if len(dataBytes) > 0 {
		body = bytes.NewReader(dataBytes)
	}
	return httpPost(ctx, url, body, header)
}

// HttpPostByJson sends an HTTP POST request with a JSON-encoded body.
func HttpPostByJson(ctx context.Context, url string, data any, header map[string]string) (statusCode int, responseBytes []byte, err error) {
	var body io.Reader
	if data != nil {
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(jsonBytes)
	}
	return httpPost(ctx, url, body, header)
}

// httpPost sends an HTTP POST request, retrying up to maxRetries times on network errors.
func httpPost(ctx context.Context, url string, body io.Reader, header map[string]string) (statusCode int, responseBytes []byte, err error) {
	// 1. Create an HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
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
		rsp, err = httpClient.Do(req)
		if err == nil {
			break
		}
		if i < maxRetries-1 {
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		return
	}
	defer rsp.Body.Close()

	statusCode = rsp.StatusCode

	// 4. Read response
	responseBytes, err = io.ReadAll(rsp.Body)

	return
}
