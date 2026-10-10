package handler

// 2026-10 AI 提速（Step 1）：缺参一次性全部报出。
// 此前 provider.Encode 遇到第一个缺失参数即返回（"missing IDL argument: from"），
// AI 要 N 轮往返才能凑齐参数。修复后：encodeWithCoercion 前置校验一次性列出
// 全部缺失参数（带类型）与已提供参数。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"

	"github.com/milon-labs/milon-go-sdk/provider"
)

func tokenProvider(t *testing.T) *provider.Provider {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(fake.Close)
	nm := client.NewNetworkManager("devNet", fake.URL)
	mc, _, err := nm.ClientFor("devNet")
	if err != nil {
		t.Fatal(err)
	}
	pd, ok := mc.GetAllPd()["token"]
	if !ok {
		t.Fatal("token app 不在 IDL 预置表中")
	}
	return pd
}

// TestMissingArgsReportedAllAtOnce：缺 from/token/amount 三个参数时，
// 一次报错必须同时列出三者（带类型）与已提供的 to。
func TestMissingArgsReportedAllAtOnce(t *testing.T) {
	pd := tokenProvider(t)
	_, err := encodeWithCoercion(pd, "Transfer", provider.Args{"to": "addr1"})
	if err == nil {
		t.Fatal("应报缺参错误")
	}
	msg := err.Error()
	for _, want := range []string{"from", "token", "amount"} {
		if !strings.Contains(msg, want) {
			t.Errorf("缺参报错应一次列出 %q: %s", want, msg)
		}
	}
	if !strings.Contains(msg, "to") {
		t.Errorf("报错应注明已提供参数 to（帮助定位漏了哪些）: %s", msg)
	}
}
