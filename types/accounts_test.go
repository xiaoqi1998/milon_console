package types

import (
	"strings"
	"testing"
)

// 三常量须自洽：tAddr/tPubHex 均由 tPrivHex 按 secp256k1 派生
// （brief 原值公私钥不配对，按测试反馈修正）。
const tPrivHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
const tAddr = "491vGnPg2q1qsRtFRun8WNW8gxTD"
const tPubHex = "0x036d6caac248af96f6afa7f904f550253a0f3ef3f5aa2fe6838a95b216691468e2"

func TestParseConfiguredAccounts(t *testing.T) {
	t.Run("单账户解析且公钥地址一致", func(t *testing.T) {
		raw := `[{"name":"main","address":"` + tAddr + `","privateKey":"` + tPrivHex + `"}]`
		got, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(raw))
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(got) != 1 || got[0].Address != tAddr || got[0].PrivateKey != tPrivHex {
			t.Fatalf("got %+v", got)
		}
		if got[0].KeyType != "secp256k1" {
			t.Fatalf("KeyType 应缺省归一为 secp256k1, got %q", got[0].KeyType)
		}
		if got[0].PublicKey == "" {
			t.Fatal("PublicKey 应被派生回填（hex 全文），供指纹与下游注入")
		}
	})
	t.Run("公钥显式传入且地址一致", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","publicKey":"` + tPubHex + `"}]`
		got, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(raw))
		if err != nil || len(got) != 1 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("公钥地址不一致拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","publicKey":"0x036d6caac248af96f6afa7f904f550253a0f3ef3f5aa2fe6838a95b216691468e3"}]`
		_, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(raw))
		if err == nil || !strings.Contains(err.Error(), "X-Milon-Accounts[0]") {
			t.Fatalf("want 定位错误, got %v", err)
		}
	})
	t.Run("私钥派生地址不一致拒绝", func(t *testing.T) {
		raw := `[{"address":"NotABase58Addr!!","privateKey":"` + tPrivHex + `"}]`
		if _, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(raw)); err == nil {
			t.Fatal("地址非法应拒绝")
		}
	})
	t.Run("fndsa512 缺 publicKey 拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","keyType":"fndsa512"}]`
		_, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(raw))
		if err == nil || !strings.Contains(err.Error(), "fndsa512") {
			t.Fatalf("want fndsa512 提示, got %v", err)
		}
	})
	t.Run("非法 keyType 拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","keyType":"rsa"}]`
		if _, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(raw)); err == nil {
			t.Fatal("非法曲线应拒绝")
		}
	})
	t.Run("地址重复拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `"},
		         {"address":"` + tAddr + `","privateKey":"` + tPrivHex + `"}]`
		if _, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(raw)); err == nil {
			t.Fatal("重复地址应拒绝")
		}
	})
	t.Run("同一地址 base58 与 hex 双形态重复拒绝", func(t *testing.T) {
		// 同一地址以 base58 与 hex 两种编码各配一次，去重须按归一键而非原始串
		const tAddrHex = "e141b55a80fada3b8b11a68cd9ad0e88c162e014"
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `"},
		         {"address":"` + tAddrHex + `","privateKey":"` + tPrivHex + `"}]`
		_, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(raw))
		if err == nil || !strings.Contains(err.Error(), "X-Milon-Accounts[1]") {
			t.Fatalf("want 编码差异重复被拒且定位到 [1], got %v", err)
		}
	})
	t.Run("经典曲线私钥与显式公钥不配对拒绝", func(t *testing.T) {
		// 公钥与 address 配对、但 privateKey 是另一把（K2+P1）——显式公钥路径也须校验配对
		const tOtherPrivHex = "1111111111111111111111111111111111111111111111111111111111111111"
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tOtherPrivHex + `","publicKey":"` + tPubHex + `"}]`
		_, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(raw))
		if err == nil || !strings.Contains(err.Error(), "不配对") {
			t.Fatalf("want 私钥公钥不配对拒绝, got %v", err)
		}
	})
	t.Run("空数组放行", func(t *testing.T) {
		got, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(`[]`))
		if err != nil || len(got) != 0 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("坏 JSON 拒绝", func(t *testing.T) {
		if _, err := ParseConfiguredAccounts("X-Milon-Accounts", []byte(`{not json`)); err == nil {
			t.Fatal("坏 JSON 应拒绝")
		}
	})
	t.Run("超上限拒绝且前缀用来源名", func(t *testing.T) {
		// 65 个任意结构条目：上限在逐条校验之前拦截，条目无需合法
		entries := make([]string, 65)
		for i := range entries {
			entries[i] = `{"address":"x"}`
		}
		raw := []byte("[" + strings.Join(entries, ",") + "]")
		_, err := ParseConfiguredAccounts("MILON_ACCOUNTS", raw)
		if err == nil || !strings.Contains(err.Error(), "MILON_ACCOUNTS: 账户数 65 超过上限 64") {
			t.Fatalf("want 来源前缀+超上限文案, got %v", err)
		}
	})
}
