package handler

import (
	"fmt"
	"strings"

	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

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

// idlErrBrief 是 IDL 错误码的最小画像（app 名 + 码 + 名 + 说明），
// 与 error_lookup 的 hit 同源数据（provider.ErrorDef）。
type idlErrBrief struct {
	App     string
	Code    int
	Name    string
	Message string
}

// appendIDLErrorHint 在错误消息中匹配 IDL 错误名（大小写不敏感子串），命中则
// 追加「链端错误对照」段（码+名+说明+来源 app）；无匹配原样返回，永不吞错
// （2026-11 六刀刀1a：码表驱动，全部链上错误码自动对照，不再只靠手工两条规则）。
func appendIDLErrorHint(msg string, briefs []idlErrBrief) string {
	lower := strings.ToLower(msg)
	var hits []string
	for _, b := range briefs {
		if b.Name != "" && strings.Contains(lower, strings.ToLower(b.Name)) {
			hits = append(hits, fmt.Sprintf("%s(code %d)：%s（app:%s）", b.Name, b.Code, b.Message, b.App))
		}
	}
	if len(hits) == 0 {
		return msg
	}
	return msg + "；链端错误对照：" + strings.Join(hits, "；")
}

// withChainErrorHintFor = withChainErrorHint（高频规则带处置建议）+ 全部 app 的
// IDL 错误码自动对照（与 error_lookup 同数据源）。从请求上下文聚合码表，
// client 不可用时退化为纯 withChainErrorHint。
func withChainErrorHintFor(c *gin.Context, msg string) string {
	msg = withChainErrorHint(msg)
	mc := middleware.ClientFrom(c)
	if mc == nil {
		return msg
	}
	var briefs []idlErrBrief
	for appName, pd := range mc.GetAllPd() {
		if pd == nil {
			continue
		}
		for _, e := range pd.IDL.Errors {
			briefs = append(briefs, idlErrBrief{App: appName, Code: e.Code, Name: e.Name, Message: e.Message})
		}
	}
	return appendIDLErrorHint(msg, briefs)
}
