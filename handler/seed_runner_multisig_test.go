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
