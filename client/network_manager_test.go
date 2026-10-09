package client

import (
	"strings"
	"testing"
)

// TestClientFor 锁定请求级网络解析的三态 + 缓存复用 + RPC 覆盖。
// 注意 milon.NewClient 只加载 IDL 不拨号，localNet(127.0.0.1:6280 无服务)
// 也能创建成功，后续请求才失败——测试环境安全。
func TestClientFor(t *testing.T) {
	nm := NewNetworkManager("devNet", "")

	// 空名 → 回落默认网络 devNet
	c1, cfg1, err := nm.ClientFor("")
	if err != nil {
		t.Fatalf("空名不应报错: %v", err)
	}
	if c1 == nil || cfg1.Name != "devNet" {
		t.Fatalf("空名应回落 devNet, got %q", cfg1.Name)
	}

	// 合法名 → 对应网络
	_, cfg2, err := nm.ClientFor("localNet")
	if err != nil {
		t.Fatalf("localNet 不应报错: %v", err)
	}
	if cfg2.Name != "localNet" {
		t.Fatalf("应返回 localNet, got %q", cfg2.Name)
	}

	// 同网复用同一 client 实例（缓存）
	c3, _, _ := nm.ClientFor("")
	if c1 != c3 {
		t.Fatal("同网络两次解析应返回同一 client 实例")
	}

	// 未知名 → 报错
	if _, _, err := nm.ClientFor("mainNet"); err == nil || !strings.Contains(err.Error(), "mainNet") {
		t.Fatalf("未知名应报错且含名字, got %v", err)
	}

	// MILON_RPC_URL 覆盖 devNet 端点后，ClientFor 返回覆盖后的配置
	nm2 := NewNetworkManager("devNet", "http://127.0.0.1:19999")
	_, cfg3, _ := nm2.ClientFor("devNet")
	if cfg3.RpcUrl != "http://127.0.0.1:19999" {
		t.Fatalf("devNet RpcUrl 应被覆盖, got %q", cfg3.RpcUrl)
	}
}
