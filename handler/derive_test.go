package handler

// 2026-10 AI 易用性修复（Step C）：地址/签名模式自动派生。
// 此前 unified_payer_all 写交易即使传了 payerPrivateKey 仍强制要求
// payerAddress + 带 publicKey 的 signatureMode（两者均可从私钥推导）；
// simulate（无私钥）在账户未上链时报 "unexpected end of JSON input" 类晦涩错误。
// 修复后：
//   - 地址缺省：从显式公钥或对应私钥（keyType，缺省 secp256k1）派生；
//   - signatureMode 缺省/缺 publicKey：从私钥派生（fndsa512 给明确指引）；
//   - 无私钥（simulate）：账户已上链且唯一签名者时缺省签名者列表模式，
//     否则给可自纠的错误。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"milon-api-server/client"
	"milon-api-server/types"

	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
)

const testSeedHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

// newDeriveHandler 组装假 RPC 后端 + ContractHandler（mc 可直连假后端）。
func newDeriveHandler(t *testing.T) (*ContractHandler, *milon.Client) {
	t.Helper()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1,"message":"fake"}`))
	}))
	t.Cleanup(fake.Close)
	nm := client.NewNetworkManager("devNet", fake.URL)
	mc, _, err := nm.ClientFor("devNet")
	if err != nil {
		t.Fatal(err)
	}
	h := NewContractHandler(nm)
	return h, mc
}

// TestParsePayerAndModeAutoDerive：地址/签名模式缺省时自动派生。
func TestParsePayerAndModeAutoDerive(t *testing.T) {
	h, mc := newDeriveHandler(t)
	sk, err := crypto.SecretKeyerFromStringRelaxed(testSeedHex)
	if err != nil {
		t.Fatal(err)
	}
	wantPk, err := derivePublicKeyByType(sk, "secp256k1")
	if err != nil {
		t.Fatal(err)
	}
	wantAddr, err := crypto.NewAddressFromPublicKey(wantPk)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("只给私钥：地址与公钥模式全部自动派生", func(t *testing.T) {
		addr, mode, err := h.parsePayerAndMode(mc, "", nil, sk, "")
		if err != nil {
			t.Fatalf("自动派生失败: %v", err)
		}
		if addr != *wantAddr {
			t.Fatalf("地址派生不符: got %s want %s", addr, *wantAddr)
		}
		pub, ok := mode.(lib.PubKeySignatureMode)
		if !ok {
			t.Fatalf("want PubKeySignatureMode, got %T", mode)
		}
		if pub.PublicKey.ToBase58() != wantPk.ToBase58() {
			t.Fatalf("公钥派生不符: got %s want %s", pub.PublicKey.ToBase58(), wantPk.ToBase58())
		}
	})

	t.Run("只给 signatureMode.publicKey：地址从公钥派生", func(t *testing.T) {
		addr, _, err := h.parsePayerAndMode(mc, "",
			json.RawMessage(`{"type":"pubkey","publicKey":"`+wantPk.ToBase58()+`"}`), sk, "")
		if err != nil {
			t.Fatalf("派生失败: %v", err)
		}
		if addr != *wantAddr {
			t.Fatalf("地址应从公钥派生: got %s want %s", addr, *wantAddr)
		}
	})

	t.Run("signatureMode 缺 publicKey：从私钥补齐", func(t *testing.T) {
		_, mode, err := h.parsePayerAndMode(mc, wantAddr.ToBase58(),
			json.RawMessage(`{"type":"pubkey"}`), sk, "")
		if err != nil {
			t.Fatalf("补齐失败: %v", err)
		}
		pub, ok := mode.(lib.PubKeySignatureMode)
		if !ok || pub.PublicKey.ToBase58() != wantPk.ToBase58() {
			t.Fatalf("publicKey 应从私钥补齐, got %T %v", mode, mode)
		}
	})

	t.Run("keyType=ed25519：按指定曲线派生", func(t *testing.T) {
		edPk, err := derivePublicKeyByType(sk, "ed25519")
		if err != nil {
			t.Fatal(err)
		}
		edAddr, err := crypto.NewAddressFromPublicKey(edPk)
		if err != nil {
			t.Fatal(err)
		}
		addr, _, err := h.parsePayerAndMode(mc, "", nil, sk, "ed25519")
		if err != nil {
			t.Fatalf("ed25519 派生失败: %v", err)
		}
		if addr != *edAddr {
			t.Fatalf("应按 ed25519 派生: got %s want %s", addr, *edAddr)
		}
	})
}

// TestParsePayerAndModeActionableErrors：不可派生时的错误必须可自纠。
func TestParsePayerAndModeActionableErrors(t *testing.T) {
	h, mc := newDeriveHandler(t)

	t.Run("无地址无私钥：指引传 payerAddress 或私钥", func(t *testing.T) {
		_, _, err := h.parsePayerAndMode(mc, "", nil, nil, "")
		if err == nil {
			t.Fatal("应报错")
		}
		if !strings.Contains(err.Error(), "payerAddress") || !strings.Contains(err.Error(), "payerPrivateKey") {
			t.Fatalf("错误应指明两种补齐路径: %v", err)
		}
	})

	t.Run("simulate 无私钥未上链：指引传 signatureMode 示例", func(t *testing.T) {
		sk, _ := crypto.SecretKeyerFromStringRelaxed(testSeedHex)
		pk, _ := derivePublicKeyByType(sk, "secp256k1")
		addr, _ := crypto.NewAddressFromPublicKey(pk)
		_, _, err := h.parsePayerAndMode(mc, addr.ToBase58(), nil, nil, "")
		if err == nil {
			t.Fatal("应报错")
		}
		if !strings.Contains(err.Error(), "signatureMode") || !strings.Contains(err.Error(), `"type":"pubkey"`) {
			t.Fatalf("错误应附 signatureMode 示例: %v", err)
		}
	})
}

// TestDefaultSignersModeFor：无私钥场景按链上签名者列表推导缺省模式（纯函数）。
func TestDefaultSignersModeFor(t *testing.T) {
	pk1, _ := crypto.NewPublicKeyFromStringRelaxed("0x02f175c4673255dc8e674d70c3d6d45550ccebafb745fbd386fa52ce6f74bb2ef6")
	pk2, _ := crypto.NewPublicKeyFromStringRelaxed("0x02f175c4673255dc8e674d70c3d6d45550ccebafb745fbd386fa52ce6f74bb2ef7")

	t.Run("唯一签名者：缺省签名者列表模式 index 0", func(t *testing.T) {
		mode, err := defaultSignersModeFor(crypto.Address{}, []string{pk1.ToBase58()})
		if err != nil {
			t.Fatalf("单签名者应可推导: %v", err)
		}
		ms, ok := mode.(lib.MultisigKeySignatureMode)
		if !ok || ms.Index != 0 || ms.PublicKey.ToBase58() != pk1.ToBase58() {
			t.Fatalf("want MultisigKey{0, pk1}, got %T %v", mode, mode)
		}
	})

	t.Run("多签名者：错误列出全部签名者并指引 index", func(t *testing.T) {
		_, err := defaultSignersModeFor(crypto.Address{}, []string{pk1.ToBase58(), pk2.ToBase58()})
		if err == nil {
			t.Fatal("多签名者应要求显式指定")
		}
		if !strings.Contains(err.Error(), "index") || !strings.Contains(err.Error(), pk1.ToBase58()) {
			t.Fatalf("错误应列签名者并指引 index: %v", err)
		}
	})

	t.Run("无签名者：可自纠错误", func(t *testing.T) {
		_, err := defaultSignersModeFor(crypto.Address{}, nil)
		if err == nil || !strings.Contains(err.Error(), "signatureMode") {
			t.Fatalf("应报 signatureMode 缺失: %v", err)
		}
	})
}

// TestParseSignerListDerivesMode：multi_signer 签名者缺 signatureMode 时从私钥派生。
func TestParseSignerListDerivesMode(t *testing.T) {
	sk, err := crypto.SecretKeyerFromStringRelaxed(testSeedHex)
	if err != nil {
		t.Fatal(err)
	}
	pk, _ := derivePublicKeyByType(sk, "secp256k1")
	addr, _ := crypto.NewAddressFromPublicKey(pk)

	signers := []types.SignerEntry{{Address: addr.ToBase58(), PrivateKey: testSeedHex}}
	addrs, _, modes, err := types.ParseSignerList(signers, true)
	if err != nil {
		t.Fatalf("signatureMode 缺省应从私钥派生: %v", err)
	}
	if modes[0] == nil {
		t.Fatal("签名模式不应为 nil")
	}
	pub, ok := modes[0].(lib.PubKeySignatureMode)
	if !ok || pub.PublicKey.ToBase58() != pk.ToBase58() {
		t.Fatalf("公钥应从私钥派生, got %T %v", modes[0], modes[0])
	}
	if len(addrs) != 1 {
		t.Fatalf("addrs 长度=%d", len(addrs))
	}

	// 无 signatureMode 且无私钥：明确报错（写模式）
	signers2 := []types.SignerEntry{{Address: addr.ToBase58()}}
	if _, _, _, err := types.ParseSignerList(signers2, true); err == nil {
		t.Fatal("无私钥且无 signatureMode 应报错")
	}
}
