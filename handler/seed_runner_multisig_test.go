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
