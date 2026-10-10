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
