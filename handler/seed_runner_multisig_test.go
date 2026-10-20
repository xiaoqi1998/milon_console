// handler/seed_runner_multisig_test.go
package handler

import "testing"

func TestSeedMultisigSignersReady(t *testing.T) {
	// ListSigners 返回的签名者数与计划一致 → 视为已注册(幂等跳过)
	if !seedMultisigSignersReady(5, []any{"a", "b", "c", "d", "e"}) {
		t.Fatal("5/5 应判定已注册")
	}
	if seedMultisigSignersReady(5, []any{"a"}) {
		t.Fatal("1/5 不应判定已注册")
	}
	if seedMultisigSignersReady(5, nil) {
		t.Fatal("空视图不应判定已注册")
	}
}

func TestSeedMultisigWeights(t *testing.T) {
	w := seedMultisigWeights(4)
	if len(w) != 4 {
		t.Fatalf("长度 %d", len(w))
	}
	for _, x := range w {
		if x != 1 {
			t.Fatalf("等权应全 1, got %v", w)
		}
	}
}

func TestSeedMultisigViewSigners(t *testing.T) {
	vec := []any{"s1", "s2", "s3"}
	// 外层 []any 形态:ListSigners 返回 tuple<Account, vec<...>>,下标 1 为签名者 vec
	if got := seedMultisigViewSigners([]any{"acct0", vec}); len(got) != 3 {
		t.Fatalf("[]any 形态应取下标 1 的 vec, got %v", got)
	}
	// 外层 map 形态:键 "1" 为签名者 vec
	if got := seedMultisigViewSigners(map[string]any{"0": "acct0", "1": vec}); len(got) != 3 {
		t.Fatalf("map 形态应取键 1 的 vec, got %v", got)
	}
	// vec 本身 map 形态:键 "0"/"1"/"2" 还原为有序切片
	mvec := map[string]any{"0": "s1", "1": "s2", "2": "s3"}
	if got := seedMultisigViewSigners([]any{"acct0", mvec}); len(got) != 3 ||
		got[0] != "s1" || got[2] != "s3" {
		t.Fatalf("vec map 形态应还原有序切片, got %v", got)
	}
	// 兜底:形状不符/nil 返回 nil(预检退化为注册路径)
	if got := seedMultisigViewSigners([]any{"only-one"}); got != nil {
		t.Fatalf("不足两元应返回 nil, got %v", got)
	}
	if got := seedMultisigViewSigners(nil); got != nil {
		t.Fatalf("nil 应返回 nil, got %v", got)
	}
}

// TestSeedMultisigStageOrder 锁定多签模块阶段执行顺序:
// DID 必须先于 CreateMultisig 注册——注册后链端拒绝单公钥签名
// (285 PubkeyModeForbidden),顺序被无意调换会导致 DID 阶段必败。
func TestSeedMultisigStageOrder(t *testing.T) {
	want := []string{"faucet", "did", "create"}
	if len(seedMultisigStageOrder) != len(want) {
		t.Fatalf("阶段数不符: got %v, want %v", seedMultisigStageOrder, want)
	}
	for i, w := range want {
		if seedMultisigStageOrder[i] != w {
			t.Fatalf("阶段顺序错误(下标 %d): got %q, want %q —— DID 必须先于 create 注册",
				i, seedMultisigStageOrder[i], w)
		}
	}
}

// TestSeedMultisigSigForbidden 表驱动:多签账户单签失败是否可降级跳过
// (faucet/DID 两阶段共用)。仅「多签已激活,单公钥签名被拒」类错误可跳过
// (285/pubkey mode/signers list 语义),其余(网络错/别名占用/参数错)必须照常返回。
func TestSeedMultisigSigForbidden(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{"链端 285 数字码", `API returned error status 6: {code: 285, message: "PubkeyModeForbidden"}`, true},
		{"PubkeyModeForbidden 语义", "failed to resolve signature mode: publicKey is not in the on-chain signers list; the account is registered on chain, so pubkey mode is rejected by the chain (error 285 PubkeyModeForbidden)", true},
		{"signers list 语义小写", "publickey is not in the on-chain signers list of account", true},
		{"语义大写变体", "PUBKEY MODE forbidden for registered account", true},
		{"signers list 完整句", "The public key is not in the on-chain SIGNERS LIST of account 2n2Mnzb8", true},
		{"真机 20261020 完整报文(同短语换配置重跑)", "did ms01: failed to resolve signature mode for 2n2Mnzb8pRd4y6vP2Rs71hKNjQWf: publicKey 27aNwSdEGaV4zSB2wPfjpXLqb7guN61yFZG2htUiw1Ezc is not in the on-chain signers list of account 2n2Mnzb8pRd4y6vP2Rs71hKNjQWf (signers: [2AcgwZYnDKB488HmmzKJ7f6TYdJDch3MoEMVrqxnSrFpE EMHQWdS87vwUmghdfKbonp3R4TB187q9LT4mFD7kaxfh hFzcFP13WTHss1iUHbs32b14ghVVbiRMNfYnfZhouDSL 5dn7wqMkRqR6jXPLkviMY32MzFWzEp5ptBwM46mhXTgX g4xXUTzJ9ib7YnAe1Eag2BoxrRoZCsDz3chzSmSkdkEr]); the account is registered on chain, so pubkey mode is rejected by the chain (error 285 PubkeyModeForbidden) — check that signatureMode.publicKey matches the signing private key, or pass {\"type\":\"multisig\",\"index\":N} explicitly", true},
		{"别名占用 1028 不可跳过", `did admin: API returned error status 6: {code: 1028, message: "DID name already bound: seed-admin-3001"}`, false},
		{"网络错误不可跳过", "view account.ListSigners failed: connection refused", false},
		{"参数错误不可跳过", "invalid parameter: alias format", false},
		{"空消息不可跳过", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := seedMultisigSigForbidden(tc.msg); got != tc.want {
				t.Fatalf("seedMultisigSigForbidden(%q) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}
}
