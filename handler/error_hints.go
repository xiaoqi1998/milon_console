package handler

import "strings"

// withChainErrorHint 给链端/SDK 错误消息内联下一步动作提示（2026-10 AI 提速：
// 高频失败不再需要 AI 多调一轮 error_lookup 或盲目重试）。无匹配原样返回，
// 永不吞掉原始错误。
func withChainErrorHint(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "cooldown"):
		return msg + "；提示：水龙头 24h 冷却中，勿立即重试——剩余冷却秒数用 account_summary 查（faucetCooldownRemainSecs），或换一个新账户（account_generate + faucet_claim）"
	case strings.Contains(lower, "insufficient"):
		return msg + "；提示：余额不足——用 account_summary 查余额，faucet_claim 领水（每账户 24h 一次），或换用已有余额的账户"
	default:
		return msg
	}
}
