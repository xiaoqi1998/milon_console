// handler/seed_runner_vesting_test.go
package handler

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
)

// 539 判定:串与 T0 探针(devNet 实报)同构。
func TestSeedVestingAlreadyExists(t *testing.T) {
	real := `failed to write contract: submit_tx returned error status 6: {Message:simulate tx error: TxFailurePayload { code: 539, message: "vesting id already used" }}`
	if !seedVestingAlreadyExists(real) {
		t.Fatal("539 TxFailurePayload 应判定为计划已存在")
	}
	if !seedVestingAlreadyExists("vesting id already used (539)") {
		t.Fatal("539 裸文本也应判定")
	}
	if seedVestingAlreadyExists("TxFailurePayload { code: 537, message: \"InvalidVestingDefinition\" }") {
		t.Fatal("537 不应误判为已存在")
	}
	if seedVestingAlreadyExists("httpStatus 500") {
		t.Fatal("HTTP 500 不应误判")
	}
}

// recipients 参数映射:账号名 → 链上地址,托管总额 = Σ 每人 total。
func TestSeedVestingRecipientsArgs(t *testing.T) {
	v := SeedVestingSpec{
		Key: "vp01", Symbol: "ST02", StartDelaySeconds: 120,
		Recipients: []SeedVestingRecipientSpec{
			{Name: "test01", Total: 600, Initial: 60, InitialPeriodSeconds: 3, PeriodSeconds: 2, Periods: 15},
			{Name: "test02", Total: 800, Initial: 80, InitialPeriodSeconds: 2, PeriodSeconds: 3, Periods: 10},
		},
	}
	args, need := seedVestingRecipientsArgs(v, func(name string) string { return "ADDR-" + name })
	if need != 1400 {
		t.Fatalf("托管额应=Σtotal(1400), got %d", need)
	}
	if len(args) != 2 {
		t.Fatalf("应输出 2 条 recipient, got %d", len(args))
	}
	if args[0]["recipient"] != "ADDR-test01" || args[0]["name"] != "test01" ||
		args[0]["total_amount"] != uint64(600) || args[0]["initial_amount"] != uint64(60) ||
		args[0]["initial_period_seconds"] != uint64(3) || args[0]["period_seconds"] != uint64(2) ||
		args[0]["periods"] != uint64(15) {
		t.Fatalf("recipient 参数映射不符: %+v", args[0])
	}
	if args[1]["recipient"] != "ADDR-test02" {
		t.Fatalf("第二条映射不符: %+v", args[1])
	}
}

// CreateVesting 事件 → vesting_id(VestingCreated 事件 value 前 20 字节 base58)。
func TestSeedVestingIdFromEvents(t *testing.T) {
	// 20 字节确定性样例(与 Python 探针提取口径一致)
	raw := make([]byte, 20)
	for i := range raw {
		raw[i] = byte(i * 7)
	}
	addr, err := crypto.NewAddressFromBytes(raw)
	if err != nil {
		t.Fatalf("样例地址构造: %v", err)
	}
	events := []api.EventEntry{
		{Data: api.TypeTagWithData{TypeTag: 999, Value: []byte{1, 2, 3}}},
		{Data: api.TypeTagWithData{TypeTag: seedVestingCreatedTag, Value: raw}},
	}
	if got := seedVestingIdFromEventEntries(events); got != addr.ToBase58() {
		t.Fatalf("应提取 vesting_id=%s, got %s", addr.ToBase58(), got)
	}
	if got := seedVestingIdFromEventEntries(events[:1]); got != "" {
		t.Fatalf("无 VestingCreated 事件应返回空, got %s", got)
	}
	// value 不足 20 字节:安全返回空
	short := []api.EventEntry{{Data: api.TypeTagWithData{TypeTag: seedVestingCreatedTag, Value: raw[:10]}}}
	if got := seedVestingIdFromEventEntries(short); got != "" {
		t.Fatalf("短 value 应返回空, got %s", got)
	}
}
