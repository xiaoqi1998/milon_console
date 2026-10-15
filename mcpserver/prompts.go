package mcpserver

// prompts.go：MCP prompts 摆货（审计整改第 3 刀）。
// 审计指控"prompts/list 返回空、一个 prompt 没注册"。本文件挂六个
// 工作流模板——宿主客户端（Claude/CodeBuddy 等）把 prompt 作为用户可选
// 命令展示，参数由客户端 UI 收集后渲染成给 agent 的指令文本：
//   - milon-onboarding    首次接入行动清单（无参）
//   - milon-transfer      转账 + 到账确认（to/amount 必填）
//   - milon-sft-lifecycle SFT 全生命周期编排参数骨架
//   - milon-vc-issuance   VC 签发全流程参数骨架
//   - milon-error-triage  错误码排查流（query 必填）
//   - milon-guide         手册页渲染（topic 必填，q 可选过滤）
// 文案与 guide 工具同源（guide.go / workflows 页），不另立事实。

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterPrompts 注册全部 prompts。每个 prompt 返回一条 user 角色消息：
// 宿主把它作为用户指令注入对话，agent 按文本执行。
func RegisterPrompts(srv *mcp.Server) {
	srv.AddPrompt(&mcp.Prompt{
		Name:        "milon-onboarding",
		Title:       "Milon 首次接入行动清单",
		Description: "新会话接入 milon 链的开局指引：先看什么、按什么顺序调工具",
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return promptResult("Milon 接入行动清单", fmt.Sprintf(`你是首次接入 milon 链的 agent，按以下顺序开局：

1. guide {"topic":"overview"} 取工具地图（或直接读资源 milon://tools）
2. 查合约调用链：idl_apps → milon://idl/apps/{app}（或 idl_methods）→ contract_read / contract_write
3. 转账：transfer_mil → tx_track 确认；批量归集用 bulk_transfer（异步 jobId，bulk_transfer_status 轮询）
4. 整段流程直接用编排工具：sft_flow / vc_flow，别手工逐步拼
5. 请求级网络切换：请求头 X-Milon-Network（devNet/localNet）
6. 任何错误码先 error_lookup；手册没讲清的读源码 %s`, sourceRepoURL)), nil
	})

	srv.AddPrompt(&mcp.Prompt{
		Name:        "milon-transfer",
		Title:       "MIL 转账并确认到账",
		Description: "transfer_mil + tx_track 两步完成一笔转账并确认终态",
		Arguments: []*mcp.PromptArgument{
			{Name: "to", Title: "收款地址", Description: "接收方地址（base58）", Required: true},
			{Name: "amount", Title: "金额", Description: "转账数量（最小单位，1 MIL = 10^6）", Required: true},
			{Name: "from", Title: "付款方", Description: "付款地址或已配置操作账户引用（可缺省；私钥永不出现在指令里，调用时再给）", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		args := req.Params.Arguments
		to := args["to"]
		amount := args["amount"]
		from := args["from"]
		fromNote := "付款方私钥或配置账户引用在调用时由我提供"
		if from != "" {
			fromNote = "付款方为 " + from + "（私钥或配置账户引用在调用时由我提供）"
		}
		return promptResult("MIL 转账指令", fmt.Sprintf(`把 %s MIL（最小单位）转到 %s，%s。按以下顺序执行：

1. transfer_mil：参数 {"to":"%s","amount":%s}，另按需带 privateKey 或 fromAddress
   （fromAddress 命中服务端已配置操作账户时可免传私钥）
2. 拿到返回的 txHash 后立即 tx_track {"hash":"<txHash>"}，等 status=confirmed 才算完成
3. track 返回 not_found/timeout 时：等几秒重试 track，不要直接重发转账（可能重复转账）
4. 余额不足先 faucet_claim 领水再转账

金额单位注意：amount 是最小单位（1 MIL = 10^6），别把 MIL 数当最小单位传。`, amount, to, fromNote, to, amount)), nil
	})

	srv.AddPrompt(&mcp.Prompt{
		Name:        "milon-sft-lifecycle",
		Title:       "SFT 全生命周期编排",
		Description: "sft_flow 一次调用完成建 SFT/建 slot/分发/合并/转移；带重跑警告",
		Arguments: []*mcp.PromptArgument{
			{Name: "name", Title: "SFT 名称", Description: "新建 SFT 的 name（已存在 SFT 可省，用其地址）", Required: false},
			{Name: "symbol", Title: "SFT 符号", Description: "新建 SFT 的 symbol", Required: false},
			{Name: "recipients", Title: "分发对象", Description: "分发描述（如：12 个新账户各 1000 份）——由 agent 展开为 distributions 数组", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		args := req.Params.Arguments
		meta := ""
		if args["name"] != "" || args["symbol"] != "" {
			meta = fmt.Sprintf(" SFT 元数据：name=%s symbol=%s。", args["name"], args["symbol"])
		}
		dist := "分发对象由我在后续消息给出；没有就只走建 SFT + 建 slot 两步。"
		if args["recipients"] != "" {
			dist = "分发对象：" + args["recipients"] + "（展开成 distributions[{to,amount}] 数组，每笔 ≤20 上限）。"
		}
		return promptResult("SFT 编排指令", fmt.Sprintf(`用 sft_flow 完成 SFT 全生命周期。%s %s

要点：
- 不传的步骤参数即跳过；owner 领水与回读验证自动做
- 新建 SFT 会生成新资源账户，私钥随响应返回——妥善保存，丢了就丢资产
- 分发/合并/转移是链上状态变更：sft_flow 重跑会重复生效，重跑前必须跟我确认
- 每笔 Mint 出独立 token_id；合并（merge）只能并自己持有的份额
- 完成后报告：SFT 地址、slotId、各 token_id 与份额、资源账户私钥存放位置`, meta, dist)), nil
	})

	srv.AddPrompt(&mcp.Prompt{
		Name:        "milon-vc-issuance",
		Title:       "VC 凭证签发全流程",
		Description: "vc_flow 一次调用完成 DID 创建、组织注册、凭证签发与披露上链",
		Arguments: []*mcp.PromptArgument{
			{Name: "count", Title: "凭证张数", Description: "签发凭证张数（缺省 5，上限 20）", Required: false},
			{Name: "note", Title: "补充要求", Description: "有效期/凭证内容等补充要求", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		count := "缺省 5 张"
		if c := req.Params.Arguments["count"]; c != "" {
			count = c + " 张"
		}
		return promptResult("VC 签发指令", fmt.Sprintf(`用 vc_flow 完成 VC 签发全流程，签发 %s。%s

要点：
- issuer 与 user 双方私钥由我在调用时提供（VC 凭证签名仅支持 Ed25519 / FN-DSA-512）
- 流程：双方领水 → 双方建 DID → issuer 注册组织 → 链下签发 → user 逐张披露上链 → 回读验证
- 每步幂等（已存在自动跳过），可直接重跑
- 完成后报告：双方 DID、凭证张数与披露交易哈希`, count, orDash(req.Params.Arguments["note"]))), nil
	})

	srv.AddPrompt(&mcp.Prompt{
		Name:        "milon-error-triage",
		Title:       "Milon 错误排查",
		Description: "按错误码/错误名走排查流：error_lookup → limits → 链端确认",
		Arguments: []*mcp.PromptArgument{
			{Name: "query", Title: "错误码或关键词", Description: "十进制错误码（如 521）或名字子串（如 Cooldown）", Required: true},
			{Name: "context", Title: "出错场景", Description: "哪个工具/哪步报的错（可选）", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		q := req.Params.Arguments["query"]
		scene := ""
		if c := req.Params.Arguments["context"]; c != "" {
			scene = "出错场景：" + c + "。"
		}
		return promptResult("错误排查指令", fmt.Sprintf(`排查 milon 错误「%s」。%s 按以下顺序：

1. error_lookup {"query":"%s"}——拿来源（API 层还是哪个 IDL app）+ 码 + 名字 + 说明 + 处置建议
2. 交易类错误：tx_track 确认交易终态（not_found/timeout/confirmed），别只看调用返回
3. 仍不明：guide {"topic":"limits"} 看限制与不可逆清单；{"topic":"chain"} 看链特性
4. 手册没讲清的读源码 %s（REST 端点在 handler/、MCP 壳在 mcpserver/）`, q, scene, q, sourceRepoURL)), nil
	})

	srv.AddPrompt(&mcp.Prompt{
		Name:        "milon-guide",
		Title:       "Milon 快速手册",
		Description: "把 guide 主题页渲染为指令上下文（topic 必填六选一，q 可选过滤）",
		Arguments: []*mcp.PromptArgument{
			{Name: "topic", Title: "主题", Description: "overview / chain / workflows / limits / performance / keys", Required: true},
			{Name: "q", Title: "关键词", Description: "只保留包含该词的行（可选）", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		page, ok := guidePage(req.Params.Arguments["topic"], req.Params.Arguments["q"])
		if !ok {
			return nil, fmt.Errorf("%s", page)
		}
		return promptResult("Milon 手册页", page), nil
	})
}

// promptResult 构造单条 user 消息的 prompt 结果（error 恒 nil，
// 与 PromptHandler 签名对齐，调用点免写 ", nil"）。
func promptResult(description, text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{
		Description: description,
		Messages: []*mcp.PromptMessage{{
			Role:    "user",
			Content: &mcp.TextContent{Text: text},
		}},
	}
}

// orDash 空串占位（prompt 文案拼接用）。
func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return "补充要求：" + s + "。"
}
