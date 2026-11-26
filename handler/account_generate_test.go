// handler/account_generate_test.go
package handler

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
)

// TestGenerateAccount_DefaultPostQuantum 锁定「生成账户缺省抗量子」(2026-11-26):
// 空 body 直接出 fndsa512 密钥对(私钥 1281 字节、响应带 897 字节公钥),
// 经典曲线须显式传 keyType 才会生成。
func TestGenerateAccount_DefaultPostQuantum(t *testing.T) {
	r := newAccountsContractRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/generate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			PrivateKey string `json:"privateKey"`
			PublicKey  string `json:"publicKey"`
			Address    string `json:"address"`
			KeyType    string `json:"keyType"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.KeyType != "fndsa512" {
		t.Fatalf("缺省 keyType = %q, want fndsa512(抗量子)", resp.Data.KeyType)
	}
	sk, err := hex.DecodeString(resp.Data.PrivateKey)
	if err != nil {
		t.Fatalf("privateKey 非 hex: %v", err)
	}
	if len(sk) != crypto.FnDsa512KeySize {
		t.Fatalf("fndsa512 私钥应 %d 字节, got %d", crypto.FnDsa512KeySize, len(sk))
	}
	pk, err := hex.DecodeString(resp.Data.PublicKey)
	if err != nil {
		t.Fatalf("publicKey 非 hex: %v", err)
	}
	if len(pk) == 0 {
		t.Fatal("fndsa512 无法从私钥反推公钥,响应必须带 publicKey")
	}
	if resp.Data.Address == "" {
		t.Fatal("缺 address")
	}

	// 显式经典曲线仍可用(兼容导入/演示),但必须传 keyType
	req2 := httptest.NewRequest(http.MethodPost, "/api/accounts/generate", strings.NewReader(`{"keyType":"ed25519"}`))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("显式 ed25519 want 200 got %d: %s", w2.Code, w2.Body.String())
	}
	var resp2 struct {
		Data struct {
			PrivateKey string `json:"privateKey"`
			KeyType    string `json:"keyType"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatal(err)
	}
	if resp2.Data.KeyType != "ed25519" {
		t.Fatalf("显式 keyType 被改写: %q", resp2.Data.KeyType)
	}
	sk2, _ := hex.DecodeString(resp2.Data.PrivateKey)
	if len(sk2) != 32 {
		t.Fatalf("ed25519 私钥应 32 字节, got %d", len(sk2))
	}
}
