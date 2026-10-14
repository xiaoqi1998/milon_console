package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"milon-api-server/middleware"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

const fPrivHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
const fAddr = "491vGnPg2q1qsRtFRun8WNW8gxTD"

func newFillCtx(t *testing.T, accounts []types.ConfiguredAccount) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.CtxAccountsKey, accounts)
	c.Set(middleware.CtxAccountsSourceKey, "header")
	return c
}

// newFndsaTestAccount 运行时生成一对 fndsa512 密钥并组装为已归一的 ConfiguredAccount。
func newFndsaTestAccount(t *testing.T) (*types.ConfiguredAccount, *crypto.PublicKey, error) {
	t.Helper()
	sk, pk, err := crypto.NewFnDsa512SecretKey()
	if err != nil {
		return nil, nil, err
	}
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		return nil, nil, err
	}
	return &types.ConfiguredAccount{
		Name: "quantum", Address: addr.ToBase58(), PrivateKey: sk.ToHex(),
		PublicKey: pk.ToHex(), KeyType: "fndsa512",
	}, pk, nil
}

func TestFillFromConfiguredAccounts(t *testing.T) {
	acct := types.ConfiguredAccount{Name: "main", Address: fAddr, PrivateKey: fPrivHex,
		PublicKey: "0x036d6caac248af96f6afa7f904f550253a0f3ef3f5aa2fe6838a95b216691468e2", KeyType: "secp256k1"}

	t.Run("空私钥+地址命中回填并注入签名模式", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		var priv, addr, kt string = "", fAddr, ""
		var sm json.RawMessage
		fillFromConfiguredAccounts(c, accountRole{Addr: &addr, Priv: &priv, SigMode: &sm, KeyType: &kt})
		if priv != fPrivHex || kt != "secp256k1" {
			t.Fatalf("priv=%q kt=%q", priv, kt)
		}
		var mode map[string]any
		if err := json.Unmarshal(sm, &mode); err != nil || mode["type"] != "pubkey" {
			t.Fatalf("SigMode 未注入: %v %v", sm, err)
		}
	})
	t.Run("显式私钥优先", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		priv, addr := "explicit", fAddr
		fillFromConfiguredAccounts(c, accountRole{Addr: &addr, Priv: &priv})
		if priv != "explicit" {
			t.Fatalf("显式私钥被覆盖: %q", priv)
		}
	})
	t.Run("地址未命中原样保留", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		priv, addr := "", "SomeOtherAddr1111111111111111111111"
		fillFromConfiguredAccounts(c, accountRole{Addr: &addr, Priv: &priv})
		if priv != "" {
			t.Fatalf("未命中不应回填: %q", priv)
		}
	})
	t.Run("无地址角色跳过", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		priv := ""
		fillFromConfiguredAccounts(c, accountRole{Priv: &priv})
		if priv != "" {
			t.Fatal("无地址不应回填")
		}
	})
	t.Run("fndsa512 回填 Pub 且不动显式 SigMode", func(t *testing.T) {
		fk, fpk, err := newFndsaTestAccount(t)
		if err != nil {
			t.Skipf("fndsa 密钥生成失败: %v", err)
		}
		_ = fpk
		c := newFillCtx(t, []types.ConfiguredAccount{*fk})
		var priv, pub, kt string
		existing := json.RawMessage(`{"type":"multisig","index":1}`)
		fillFromConfiguredAccounts(c, accountRole{Addr: &fk.Address, Priv: &priv, Pub: &pub, KeyType: &kt, SigMode: &existing})
		if priv != fk.PrivateKey || pub != fk.PublicKey || kt != "fndsa512" {
			t.Fatalf("priv=%q pub=%q kt=%q", priv, pub, kt)
		}
		if string(existing) != `{"type":"multisig","index":1}` {
			t.Fatalf("显式 SigMode 被覆盖: %s", existing)
		}
	})
	t.Run("lookupConfiguredAccount 按地址命中", func(t *testing.T) {
		c := newFillCtx(t, []types.ConfiguredAccount{acct})
		if lookupConfiguredAccount(c, fAddr) == nil {
			t.Fatal("应命中")
		}
		if lookupConfiguredAccount(c, "nope") != nil || lookupConfiguredAccount(c, "") != nil {
			t.Fatal("不应命中")
		}
	})
}
