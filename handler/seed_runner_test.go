package handler

import (
	"errors"
	"reflect"
	"testing"
)

func TestSeedCodesIn(t *testing.T) {
	err := errors.New(`failed to write contract: API returned error status 6: {Message:simulate tx error: TxFailurePayload { code: 521, message: "VC policy required" }`)
	got := seedCodesIn(err.Error())
	if !reflect.DeepEqual(got, []int{521}) {
		t.Fatalf("codes = %v, want [521]", got)
	}
	if got := seedCodesIn("余额不足 (可用: 0, code=513)"); len(got) != 1 || got[0] != 513 {
		t.Fatalf("513 提取失败: %v", got)
	}
	if got := seedCodesIn("no codes here"); len(got) != 0 {
		t.Fatalf("无码文本应返回空: %v", got)
	}
}

func TestSeedVcRejected(t *testing.T) {
	if !seedVcRejected("TxFailurePayload { code: 521, message: \"VC policy required\" }") {
		t.Fatal("521 应判定为合规拦截")
	}
	if !seedVcRejected("TxFailurePayload { code: 522, message: \"VcPolicyRequired\" }") {
		t.Fatal("522 应判定为合规拦截")
	}
	if seedVcRejected("TxFailurePayload { code: 513, message: \"余额不足\" }") {
		t.Fatal("513 不是合规拦截")
	}
}

func TestSeedTokenRejected(t *testing.T) {
	if !seedTokenAlreadyExists("TokenAlreadyExists (516)") {
		t.Fatal("516 应判定为已存在")
	}
	if seedTokenAlreadyExists("TokenNotFound (515)") {
		t.Fatal("515 不是已存在")
	}
}

func TestSeedMintDiffs(t *testing.T) {
	want := map[string]uint64{"a": 100, "b": 200, "c": 300}
	have := map[string]uint64{"a": 100, "b": 50, "c": 0}
	addrs, amounts := seedMintDiffs(want, have)
	if !reflect.DeepEqual(addrs, []string{"b", "c"}) {
		t.Fatalf("addrs = %v", addrs)
	}
	if !reflect.DeepEqual(amounts, []uint64{150, 300}) {
		t.Fatalf("amounts = %v", amounts)
	}
	addrs, amounts = seedMintDiffs(want, map[string]uint64{"a": 100, "b": 200, "c": 300})
	if len(addrs) != 0 || len(amounts) != 0 {
		t.Fatal("全部到位应返回空差额")
	}
}

func TestSeedFreezeAmount(t *testing.T) {
	if got := seedFreezeAmount(10_000_000_000); got != 3_333_333_333 {
		t.Fatalf("freeze = %d, want 3333333333", got)
	}
	if got := seedFreezeAmount(2); got != 0 {
		t.Fatalf("小额持有 freeze = %d, want 0(跳过)", got)
	}
}

func TestSeedModuleOrderV2(t *testing.T) {
	want := []string{"accounts", "multisig", "tokens", "sft", "vc", "compliance", "txs"}
	if len(SeedModuleOrder) != len(want) {
		t.Fatalf("模块数 %d", len(SeedModuleOrder))
	}
	for i := range want {
		if SeedModuleOrder[i] != want[i] {
			t.Fatalf("顺序不符: %v", SeedModuleOrder)
		}
	}
}

func TestSeedTierTargetMIL(t *testing.T) {
	if seedTierTargetMIL("hundred") != 100 || seedTierTargetMIL("thousand") != 1000 || seedTierTargetMIL("ten_k") != 10000 {
		t.Fatal("档位换算不符")
	}
	if seedTierTargetMIL("nope") != 0 {
		t.Fatal("未知档位应为 0")
	}
}

func TestSeedComplianceStageKey(t *testing.T) {
	if got := seedComplianceStageKey("credential", "SCUSD"); got != "credential:SCUSD" {
		t.Fatalf("got %q", got)
	}
}
