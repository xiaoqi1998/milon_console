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
		got, err := ParseConfiguredAccounts([]byte(raw))
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
		got, err := ParseConfiguredAccounts([]byte(raw))
		if err != nil || len(got) != 1 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("公钥地址不一致拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","publicKey":"0x036d6caac248af96f6afa7f904f550253a0f3ef3f5aa2fe6838a95b216691468e3"}]`
		_, err := ParseConfiguredAccounts([]byte(raw))
		if err == nil || !strings.Contains(err.Error(), "X-Milon-Accounts[0]") {
			t.Fatalf("want 定位错误, got %v", err)
		}
	})
	t.Run("私钥派生地址不一致拒绝", func(t *testing.T) {
		raw := `[{"address":"NotABase58Addr!!","privateKey":"` + tPrivHex + `"}]`
		if _, err := ParseConfiguredAccounts([]byte(raw)); err == nil {
			t.Fatal("地址非法应拒绝")
		}
	})
	t.Run("fndsa512 缺 publicKey 拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","keyType":"fndsa512"}]`
		_, err := ParseConfiguredAccounts([]byte(raw))
		if err == nil || !strings.Contains(err.Error(), "fndsa512") {
			t.Fatalf("want fndsa512 提示, got %v", err)
		}
	})
	t.Run("非法 keyType 拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `","keyType":"rsa"}]`
		if _, err := ParseConfiguredAccounts([]byte(raw)); err == nil {
			t.Fatal("非法曲线应拒绝")
		}
	})
	t.Run("地址重复拒绝", func(t *testing.T) {
		raw := `[{"address":"` + tAddr + `","privateKey":"` + tPrivHex + `"},
		         {"address":"` + tAddr + `","privateKey":"` + tPrivHex + `"}]`
		if _, err := ParseConfiguredAccounts([]byte(raw)); err == nil {
			t.Fatal("重复地址应拒绝")
		}
	})
	t.Run("空数组放行", func(t *testing.T) {
		got, err := ParseConfiguredAccounts([]byte(`[]`))
		if err != nil || len(got) != 0 {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("坏 JSON 拒绝", func(t *testing.T) {
		if _, err := ParseConfiguredAccounts([]byte(`{not json`)); err == nil {
			t.Fatal("坏 JSON 应拒绝")
		}
	})
}
