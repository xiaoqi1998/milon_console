package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxRetries      = 3
	maxResponseSize = 16 << 20 // 16 MiB
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 32,
		DisableCompression:  true,
	},
}

// HttpPostByBytes sends an HTTP POST request with raw bytes as the body.
func HttpPostByBytes(ctx context.Context, url string, dataBytes []byte, header map[string]string) (statusCode int, responseBytes []byte, err error) {
	bodyFactory := func() io.Reader {
		if len(dataBytes) > 0 {
			return bytes.NewReader(dataBytes)
		}
		return nil
	}
	return httpPost(ctx, url, bodyFactory, header)
}

// httpPost sends an HTTP POST request, retrying up to maxRetries times on
// network errors and 5xx responses with exponential backoff. The bodyFactory
// is called for each attempt so the request body is always fresh (avoids
// "body already closed" on retry). 5xx responses are fully read before retry
// so the keep-alive connection can be reused; the last attempt's response is
// returned as-is for the caller to surface.
func httpPost(ctx context.Context, url string, bodyFactory func() io.Reader, header map[string]string) (statusCode int, responseBytes []byte, err error) {
	for i := 0; i < maxRetries; i++ {
		var body io.Reader
		if bodyFactory != nil {
			body = bodyFactory()
		}

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
		if reqErr != nil {
			return 0, nil, reqErr
		}

		if header == nil {
			req.Header.Add("Content-Type", "application/json")
		} else {
			for key, value := range header {
				req.Header.Add(key, value)
			}
		}

		var rsp *http.Response
		rsp, err = httpClient.Do(req)
		if err != nil {
			// network-layer failure (dial, timeout, reset, ...)
			if ctx.Err() != nil {
				return 0, nil, ctx.Err()
			}
			if i < maxRetries-1 {
				if waitErr := retryBackoff(ctx, i); waitErr != nil {
					return 0, nil, waitErr
				}
			}
			continue
		}

		statusCode = rsp.StatusCode
		responseBytes, err = io.ReadAll(io.LimitReader(rsp.Body, maxResponseSize+1))
		rsp.Body.Close()
		if err != nil {
			return 0, nil, err
		}
		if len(responseBytes) > maxResponseSize {
			return 0, nil, fmt.Errorf("response body exceeds max size %d bytes", maxResponseSize)
		}

		// 5xx: transient node failures (overload, rolling upgrade, ...).
		// The body is fully read above so the connection can be reused.
		if statusCode >= 500 && statusCode < 600 && i < maxRetries-1 {
			if waitErr := retryBackoff(ctx, i); waitErr != nil {
				return 0, nil, waitErr
			}
			continue
		}

		return statusCode, responseBytes, nil
	}

	// All attempts failed with network errors; err holds the last failure.
	return statusCode, responseBytes, err
}

// retryBackoff waits with exponential backoff (100ms, 200ms, ...), returning
// early with ctx.Err() when the context is cancelled.
func retryBackoff(ctx context.Context, attempt int) error {
	backoff := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(backoff):
		return nil
	}
}
