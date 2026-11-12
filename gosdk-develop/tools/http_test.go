package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// failingServer returns a server that counts requests and always answers 500.
func failingServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func TestHttpPostByBytesRetriesServerErrors(t *testing.T) {
	server, calls := failingServer(t)

	statusCode, _, err := HttpPostByBytes(context.Background(), server.URL, []byte("body"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusInternalServerError)
	}
	if got := atomic.LoadInt32(calls); got != maxRetries {
		t.Fatalf("requests = %d, want %d", got, maxRetries)
	}
}

func TestHttpPostByBytesNoRetrySendsOnce(t *testing.T) {
	server, calls := failingServer(t)

	statusCode, _, err := HttpPostByBytesNoRetry(context.Background(), server.URL, []byte("body"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusInternalServerError)
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Fatalf("requests = %d, want 1: non-idempotent requests must not be retried", got)
	}
}
