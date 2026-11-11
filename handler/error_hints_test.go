package handler

// 2026-10 AI 提速（Step 3）：链上错误内联提示。
// 高频失败（水龙头冷却/余额不足）此前只有裸链端错误串，AI 要么盲目重试要么
// 多调一轮 error_lookup。修复后：错误消息直接内联下一步动作提示。

import (
	"strings"
	"testing"
)

func TestChainErrorHints(t *testing.T) {
	cases := []struct {
		name, in, wantSub string
	}{
		{
			"水龙头冷却",
			"API returned error status 6: {Message:faucet cooldown Code:FaucetCooldownActive}",
			"24h 冷却",
		},
		{
			"余额不足",
			"API returned error status 6: {Message:insufficient balance for transfer}",
			"faucet_claim",
		},
		{
			"大小写不敏感的余额不足",
			"exec failed: Insufficient Balance",
			"account_summary",
		},
	}
	for _, c := range cases {
		got := withChainErrorHint(c.in)
		if !strings.Contains(got, c.wantSub) {
			t.Errorf("%s: 提示应含 %q, got %q", c.name, c.wantSub, got)
		}
		if !strings.Contains(got, c.in) {
			t.Errorf("%s: 不得吞掉原始错误信息", c.name)
		}
	}

	// 无匹配：原样返回
	plain := "some unrelated error"
	if got := withChainErrorHint(plain); got != plain {
		t.Errorf("无匹配应原样返回, got %q", got)
	}
}

// TestAppendIDLErrorHint：IDL 错误名自动对照（码表驱动，2026-11 六刀刀1a）。
func TestAppendIDLErrorHint(t *testing.T) {
	briefs := []idlErrBrief{
		{App: "credential", Code: 521, Name: "VcRequired", Message: "需要先持有合规凭证"},
		{App: "identity", Code: 1026, Name: "DidDeactivated", Message: "DID 已停用"},
	}
	cases := []struct{ name, in, wantSub string }{
		{"按错误名命中", "exec failed: VcRequired for account 2DS..", "VcRequired(code 521)"},
		{"错误名子串大小写不敏感", "API error: vcrequired state", "VcRequired(code 521)"},
		{"多命中都追加", "reject: VcRequired and DidDeactivated", "DidDeactivated(code 1026)"},
	}
	for _, c := range cases {
		got := appendIDLErrorHint(c.in, briefs)
		if !strings.Contains(got, c.wantSub) {
			t.Errorf("%s: 应含 %q, got %q", c.name, c.wantSub, got)
		}
		if !strings.Contains(got, c.in) {
			t.Errorf("%s: 不得吞掉原始错误信息, got %q", c.name, got)
		}
	}

	// 无命中：原样返回
	plain := "totally unrelated error"
	if got := appendIDLErrorHint(plain, briefs); got != plain {
		t.Errorf("无命中应原样返回, got %q", got)
	}
	// 空名跳过：不 panic、不误匹配
	if got := appendIDLErrorHint(plain, []idlErrBrief{{App: "x", Code: 1, Name: "", Message: "m"}}); got != plain {
		t.Errorf("空错误名应跳过, got %q", got)
	}
}
