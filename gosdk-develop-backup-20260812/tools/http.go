package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

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

	// 3. Create HTTP client
	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	// 4. Send request
	var rsp *http.Response
	for i := 0; i < 1; i++ {
		rsp, err = client.Do(req)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	if err != nil {
		return
	}
	defer rsp.Body.Close()

	statusCode = rsp.StatusCode

	// 5. Read response
	responseBytes, err = io.ReadAll(rsp.Body)

	return
}

func HttpPostByJson(ctx context.Context, url string, data any, header map[string]string) (statusCode int, responseBytes []byte, err error) {
	// 1. Create an HTTP request with context
	var req *http.Request
	if data != nil {
		var jsonBytes []byte
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
	// 3. Create HTTP client
	client := &http.Client{}

	// 4. Send request
	var rsp *http.Response
	for i := 0; i < 3; i++ {
		rsp, err = client.Do(req)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	if err != nil {
		return
	}
	defer rsp.Body.Close()

	statusCode = rsp.StatusCode

	// 5. Read response
	responseBytes, err = io.ReadAll(rsp.Body)

	return
}
