package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newFakeBackend 起一个记录请求的假 REST 后端。
func newFakeBackend(t *testing.T, status int, respBody string) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			_, _ = r.Body.Read(body)
		}
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" "+string(body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestExecutorCall(t *testing.T) {
	ctx := context.Background()

	t.Run("GET 路径参数渲染", func(t *testing.T) {
		srv, seen := newFakeBackend(t, 200, `{"code":0}`)
		e := NewExecutor(srv.URL, srv.Client())
		out, err := e.Call(ctx, RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}", PathParams: []string{"hash"}},
			json.RawMessage(`{"hash":"abc"}`))
		if err != nil {
			t.Fatal(err)
		}
		if out.IsError || string(out.Body) != `{"code":0}` {
			t.Fatalf("out=%+v", out)
		}
		if *seen != nil && (*seen)[0] != "GET /api/transactions/abc? " {
			t.Fatalf("seen=%v", *seen)
		}
	})

	t.Run("POST body 排除路径参数", func(t *testing.T) {
		srv, seen := newFakeBackend(t, 200, `{}`)
		e := NewExecutor(srv.URL, srv.Client())
		_, err := e.Call(ctx, RESTMapping{Method: "POST", PathTemplate: "/api/faucet/claim"},
			json.RawMessage(`{"privateKey":"sk","address":"addr"}`))
		if err != nil {
			t.Fatal(err)
		}
		got := (*seen)[0]
		// JSON 对象无序：实现经 map 往返后 key 顺序会变，按键值比较而非字面全等。
		prefix := "POST /api/faucet/claim? "
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("got=%s", got)
		}
		var body map[string]string
		if err := json.Unmarshal([]byte(strings.TrimPrefix(got, prefix)), &body); err != nil {
			t.Fatalf("body 不是合法 JSON: %v", err)
		}
		if len(body) != 2 || body["privateKey"] != "sk" || body["address"] != "addr" {
			t.Fatalf("body=%v", body)
		}
	})

	t.Run("PUT 携带 JSON body 且排除路径参数", func(t *testing.T) {
		srv, seen := newFakeBackend(t, 200, `{}`)
		e := NewExecutor(srv.URL, srv.Client())
		// PUT 与 POST 同为 body 方法（saved-instructions 更新）；
		// id 已渲染进路径，不得残留在 body。
		_, err := e.Call(ctx, RESTMapping{Method: "PUT", PathTemplate: "/api/saved-instructions/{id}", PathParams: []string{"id"}},
			json.RawMessage(`{"id":"abc","name":"n2","description":"d2"}`))
		if err != nil {
			t.Fatal(err)
		}
		got := (*seen)[0]
		prefix := "PUT /api/saved-instructions/abc? "
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("got=%s", got)
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(got, prefix)), &body); err != nil {
			t.Fatalf("PUT 应携带合法 JSON body: %v", err)
		}
		if len(body) != 2 || body["name"] != "n2" || body["description"] != "d2" {
			t.Fatalf("body=%v（路径参数 id 应被排除）", body)
		}
	})

	t.Run("DELETE 无 body", func(t *testing.T) {
		srv, seen := newFakeBackend(t, 200, `{}`)
		e := NewExecutor(srv.URL, srv.Client())
		_, err := e.Call(ctx, RESTMapping{Method: "DELETE", PathTemplate: "/api/saved-instructions/{id}", PathParams: []string{"id"}},
			json.RawMessage(`{"id":"abc"}`))
		if err != nil {
			t.Fatal(err)
		}
		if got := (*seen)[0]; got != "DELETE /api/saved-instructions/abc? " {
			t.Fatalf("got=%q（DELETE 不应携带 body）", got)
		}
	})

	t.Run("非2xx 转工具错误且保留原文", func(t *testing.T) {
		srv, _ := newFakeBackend(t, 400, `{"code":1001,"message":"invalid privateKey"}`)
		e := NewExecutor(srv.URL, srv.Client())
		out, _ := e.Call(ctx, RESTMapping{Method: "POST", PathTemplate: "/api/faucet/claim"}, json.RawMessage(`{}`))
		if !out.IsError {
			t.Fatal("want IsError")
		}
		if string(out.Body) != `{"code":1001,"message":"invalid privateKey"}` {
			t.Fatalf("body=%s", out.Body)
		}
	})

	t.Run("缺路径参数返回错误", func(t *testing.T) {
		srv, _ := newFakeBackend(t, 200, `{}`)
		e := NewExecutor(srv.URL, srv.Client())
		_, err := e.Call(ctx, RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}", PathParams: []string{"hash"}}, json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("want error for missing path param")
		}
	})

	t.Run("后端不可达报可读错误", func(t *testing.T) {
		e := NewExecutor("http://127.0.0.1:1", &http.Client{})
		out, err := e.Call(ctx, RESTMapping{Method: "GET", PathTemplate: "/api/health"}, json.RawMessage(`{}`))
		if err == nil && !out.IsError {
			t.Fatal("want unreachable error")
		}
	})
}

// TestExecutorTimeoutMessage（2026-10 AI 易用性）：回环调用超时不得与
// "REST 后端不可达" 混淆——sft_flow 最多 20 笔分发同步等待，此前 120s 超时
// 报 "milon REST 后端不可达: context deadline exceeded" 误导排查。修复后：
// 默认超时提到 300s，超时错误单独措辞并注明时长。
func TestExecutorTimeoutMessage(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(slow.Close)
	e := NewExecutor(slow.URL, &http.Client{Timeout: 30 * time.Millisecond})
	_, err := e.Call(context.Background(), RESTMapping{Method: "GET", PathTemplate: "/api/network/list"}, nil)
	if err == nil {
		t.Fatal("want timeout error")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("超时错误应单独措辞: %v", err)
	}
	if strings.Contains(err.Error(), "不可达") {
		t.Fatalf("超时不应报成不可达: %v", err)
	}
}

// TestExecutorDefaultTimeout 锁定默认回环超时 300s（长任务 sft_flow 需要）。
func TestExecutorDefaultTimeout(t *testing.T) {
	e := NewExecutor("http://127.0.0.1:1", nil)
	if e.http.Timeout != 300*time.Second {
		t.Fatalf("默认超时=%v, want 300s", e.http.Timeout)
	}
}
