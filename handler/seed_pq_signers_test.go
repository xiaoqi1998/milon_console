package handler

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
)

// 链端 2026-11-12 新行为:CreateMultisig 的 signers 只接受 FN-DSA-512(PQ)公钥,
// 经典 ed25519 公钥被拒(273 Invalid public key)。播种器的多签签者改为运行时
// 随机生成的 PQ 密钥组,即生成即落盘 state,重跑幂等复用。

func TestSeedEnsurePqSignersGenerates(t *testing.T) {
	st := LoadSeedState(filepath.Join(t.TempDir(), "state.json"))
	keys, err := seedEnsurePqSigners(st, "ms01", 5)
	if err != nil {
		t.Fatalf("seedEnsurePqSigners: %v", err)
	}
	if len(keys) != 5 {
		t.Fatalf("len(keys) = %d, want 5", len(keys))
	}
	sk, err := hex.DecodeString(keys[0].PrivateKey)
	if err != nil {
		t.Fatalf("privateKey hex: %v", err)
	}
	if len(sk) != crypto.FnDsa512SignKeyLen {
		t.Fatalf("privateKey len = %d, want %d", len(sk), crypto.FnDsa512SignKeyLen)
	}
	pk, err := hex.DecodeString(keys[0].PublicKey)
	if err != nil {
		t.Fatalf("publicKey hex: %v", err)
	}
	if len(pk) != crypto.FnDsa512VrfyKeyLen {
		t.Fatalf("publicKey len = %d, want %d", len(pk), crypto.FnDsa512VrfyKeyLen)
	}
	if _, err := crypto.NewAddressFromRelaxed(keys[0].Address); err != nil {
		t.Fatalf("address 不可解析: %v", err)
	}
}

func TestSeedEnsurePqSignersReuseOnRerun(t *testing.T) {
	st := LoadSeedState(filepath.Join(t.TempDir(), "state.json"))
	keys1, err := seedEnsurePqSigners(st, "ms01", 5)
	if err != nil {
		t.Fatalf("第一次生成: %v", err)
	}
	keys2, err := seedEnsurePqSigners(st, "ms01", 5)
	if err != nil {
		t.Fatalf("第二次读取: %v", err)
	}
	for i := range keys1 {
		if keys1[i].PublicKey != keys2[i].PublicKey || keys1[i].PrivateKey != keys2[i].PrivateKey {
			t.Fatalf("重跑必须复用同一密钥组,第 %d 个不一致", i)
		}
	}
}

func TestSeedEnsurePqSignersTopsUp(t *testing.T) {
	st := LoadSeedState(filepath.Join(t.TempDir(), "state.json"))
	if _, err := seedEnsurePqSigners(st, "ms01", 2); err != nil {
		t.Fatalf("预置 2 个: %v", err)
	}
	keys, err := seedEnsurePqSigners(st, "ms01", 5)
	if err != nil {
		t.Fatalf("补齐到 5: %v", err)
	}
	if len(keys) != 5 {
		t.Fatalf("len(keys) = %d, want 5", len(keys))
	}
	// 去重校验:公钥两两不同
	set := map[string]bool{}
	for _, k := range keys {
		if set[k.PublicKey] {
			t.Fatalf("公钥重复: %s", k.PublicKey[:16])
		}
		set[k.PublicKey] = true
	}
}

func TestSeedEnsurePqSignersSeparatePerMultisig(t *testing.T) {
	st := LoadSeedState(filepath.Join(t.TempDir(), "state.json"))
	a, err := seedEnsurePqSigners(st, "ms01", 5)
	if err != nil {
		t.Fatalf("ms01: %v", err)
	}
	b, err := seedEnsurePqSigners(st, "ms02", 5)
	if err != nil {
		t.Fatalf("ms02: %v", err)
	}
	if a[0].PublicKey == b[0].PublicKey {
		t.Fatal("不同多签的 PQ 密钥组必须相互独立")
	}
}

func TestSeedPqSignerPublicKeys(t *testing.T) {
	st := LoadSeedState(filepath.Join(t.TempDir(), "state.json"))
	keys, err := seedEnsurePqSigners(st, "ms01", 2)
	if err != nil {
		t.Fatalf("seedEnsurePqSigners: %v", err)
	}
	pubs, err := seedPqSignerPublicKeys(keys)
	if err != nil {
		t.Fatalf("seedPqSignerPublicKeys: %v", err)
	}
	if len(pubs) != 2 {
		t.Fatalf("len(pubs) = %d, want 2", len(pubs))
	}
	for i := range pubs {
		if pubs[i].Variant != crypto.PublicKeyTypeFnDsa512 {
			t.Fatalf("pubs[%d].Variant = %d, want FnDsa512(%d)", i, pubs[i].Variant, crypto.PublicKeyTypeFnDsa512)
		}
		addr, err := crypto.NewAddressFromPublicKey(&pubs[i])
		if err != nil {
			t.Fatalf("派生地址: %v", err)
		}
		if addr.ToBase58() != keys[i].Address {
			t.Fatalf("pubs[%d] 派生地址 %s != 记录 %s", i, addr.ToBase58(), keys[i].Address)
		}
	}
}

func TestSeedPqSignersRoundTripThroughStateJSON(t *testing.T) {
	// state 落盘再加载(模拟进程重启),Get 出来的 any 走 JSON 往返后必须还能解析
	path := filepath.Join(t.TempDir(), "state.json")
	st := LoadSeedState(path)
	keys1, err := seedEnsurePqSigners(st, "ms01", 3)
	if err != nil {
		t.Fatalf("生成: %v", err)
	}
	st2 := LoadSeedState(path) // 重新从盘加载
	keys2, err := seedEnsurePqSigners(st2, "ms01", 3)
	if err != nil {
		t.Fatalf("重启后读取: %v", err)
	}
	raw, _ := json.Marshal(keys1)
	raw2, _ := json.Marshal(keys2)
	if string(raw) != string(raw2) {
		t.Fatal("state JSON 往返后密钥组必须逐字节一致")
	}
}

// 链端 2026-11-12 深夜进一步收紧:多签 owner 经典签名也会被回执翻转为
// 69 PqSignatureRequired(实测 ed25519 owner 交易提交时 confirmed、随后 tx_get
// state=2 error=69);全 PQ(owner+signers 均 fndsa512)稳定成功。
// 因此 ms 账户本体也走 PQ 密钥组(faucet/DID/create 全链路 PQ 签名)。
func TestSeedEnsurePqAccountGeneratesAndReuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st := LoadSeedState(path)
	sk1, pub1, adr1, err := seedEnsurePqAccount(st, "ms01")
	if err != nil {
		t.Fatalf("seedEnsurePqAccount: %v", err)
	}
	if crypto.AsFnDsa512SecretKey(sk1) == nil {
		t.Fatal("SK 必须是 FnDsa512SecretKey")
	}
	if pub1.Variant != crypto.PublicKeyTypeFnDsa512 {
		t.Fatalf("pub variant = %d, want FnDsa512", pub1.Variant)
	}
	if adr1.ToBase58() == "" {
		t.Fatal("地址为空")
	}
	// 重跑(新实例从盘加载)必须复用同一密钥
	st2 := LoadSeedState(path)
	sk2, pub2, adr2, err := seedEnsurePqAccount(st2, "ms01")
	if err != nil {
		t.Fatalf("重启后读取: %v", err)
	}
	if sk1.ToHex() != sk2.ToHex() {
		t.Fatal("重跑必须复用同一 PQ owner 私钥")
	}
	if pub1.ToBase58() != pub2.ToBase58() || adr1.ToBase58() != adr2.ToBase58() {
		t.Fatal("重跑必须复用同一公钥/地址")
	}
}

func TestSeedPqOwnerDistinctFromSigners(t *testing.T) {
	st := LoadSeedState(filepath.Join(t.TempDir(), "state.json"))
	sk, pub, _, err := seedEnsurePqAccount(st, "ms01")
	if err != nil {
		t.Fatalf("owner: %v", err)
	}
	keys, err := seedEnsurePqSigners(st, "ms01", 2)
	if err != nil {
		t.Fatalf("signers: %v", err)
	}
	if keys[0].PublicKey == pub.ToHex() {
		t.Fatal("owner 与签者密钥组必须独立")
	}
	_ = sk
}
