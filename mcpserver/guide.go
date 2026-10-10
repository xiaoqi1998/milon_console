package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// guide.go —— 「文档即工具」：一次调用返回指定主题的整页使用手册。
// 与其它工具不同，guide 不映射任何 REST 端点：内容是本地常量、零 IO 秒回
// （TestGuideToolCallLocal 用不可达后端证明这一点）。六个主题页各自独立成页，
// AI 按需取一页而不是把 200KB 文档常驻上下文——与 idl_apps→idl_methods 的
// 渐进披露同构。
//
// 内容事实源：各工具 schema 描述与 handler 校验（tools.go 的 jsonschema 注释）。
// 改链参数/限制时同步改对应页并递增 guideDocVersion。

// guideDocVersion 手册内容版本（内容变更时递增，每页首行携带）。
const guideDocVersion = "2026.10.10"

// guideArgs 是 guide 的输入。topic 必填六选一；q 可选关键词过滤。
type guideArgs struct {
	Topic string `json:"topic" jsonschema:"必填：六选一——overview 工具地图 / chain 链特性与 paymentMode 六模式 / workflows 典型调用流程 / limits 限制与不可逆清单 / performance 提速技巧 / keys 密钥曲线与 fndsa512；不确定先取 overview"`
	Q     string `json:"q,omitempty" jsonschema:"可选：关键词过滤（大小写不敏感），只返回包含该词的行；缺省返回整页"`
}

// guidePage 取主题页；q 非空时只保留命中行。未知 topic 返回 ok=false 且
// 附可用清单（错误路径即发现路径，与 idl_metadata 传未知名报可用 app 同构）。
func guidePage(topic, q string) (string, bool) {
	page, ok := guideTopics[strings.ToLower(strings.TrimSpace(topic))]
	if !ok {
		return "未知 topic「" + topic + "」。可用主题：overview / chain / workflows / limits / performance / keys（各主题说明见 guide 工具描述）", false
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return page, true
	}
	needle := strings.ToLower(q)
	var kept []string
	for _, ln := range strings.Split(page, "\n") {
		if strings.Contains(strings.ToLower(ln), needle) {
			kept = append(kept, ln)
		}
	}
	if len(kept) == 0 {
		return "topic=" + topic + " 中无关键词「" + q + "」命中；换关键词或去掉 q 取整页", true
	}
	return strings.Join(kept, "\n"), true
}

// registerLocalTool 注册不经 REST 的本地工具：与 registerTool 同样进
// ToolInventory（Method=LOCAL / Path=(local)，满足清单五字段非空断言），
// 但回调直接返回 fn 结果，不构造回环请求。
func registerLocalTool[In any](srv *mcp.Server, name, desc string, fn func(args In) (text string, isErr bool)) {
	if !inventorySeen[name] {
		inventorySeen[name] = true
		inventory = append(inventory, ToolInfo{
			Name:        name,
			Description: desc,
			Group:       toolGroupOf(name),
			Method:      "LOCAL",
			Path:        "(local)",
		})
	}
	mcp.AddTool[In, any](srv, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, any, error) {
			text, isErr := fn(args)
			return &mcp.CallToolResult{
				IsError: isErr,
				Content: []mcp.Content{&mcp.TextContent{Text: text}},
			}, nil, nil
		})
}

// guideTopics 六个主题页。注意：内容在 Go 反引号原始字符串里，页内禁用反引号。
var guideTopics = map[string]string{

	"overview": `# Milon MCP 工具地图 (doc_version ` + guideDocVersion + `)

口诀：查合约先 idl_apps → idl_methods；读状态 contract_read（批量 contract_read_multi）；
写合约 contract_write（wait:true 省一次 tx_track）；转账 transfer_mil → tx_track；
整段流程直接用编排工具（sft_flow / vc_flow / bulk_transfer）。

## 账户与余额
account_generate 生成账户（四曲线可选）· account_info / account_resources 单查 ·
account_summary 一站聚合（余额+冻结+faucet 冷却+DID，子项失败不整体失败）

## 转账
transfer_mil 一行转账（自动补全部固定参数）· bulk_transfer 批量生成+领水+归集（异步 jobId，
bulk_transfer_status 轮询）

## 交易查询与收尾
tx_get / tx_events / tx_parse 查询解析 · tx_wait 等确认 ·
tx_track 收尾一步到位（get+wait+events 聚合）

## 合约调用
idl_apps / idl_methods / idl_metadata 元数据三级（先清单后明细）·
contract_read(_multi) 只读 · contract_simulate(_multi) dry-run 不耗 gas ·
contract_write(_multi) 真实上链 · contract_write_safe 先模拟再上链

## DID（12 个）
did_create 幂等创建（已有则补齐差异）· did_document 查文档 · did_name_binding 别名反查 ·
did_set_alias / add|update|remove_service / set_avatar_uri / add|update|remove_key 细粒度管理 ·
did_deactivate 不可逆停用

## VC / SFT / 编排
vc_flow 签发全流程（幂等可重跑）· vc_attestation 披露文档生成 ·
sft_flow SFT 全周期一次调用 · error_lookup 错误码翻译（API 层+全部 IDL app）

## 保存指令（重复操作复用）
saved_instruction_create 绑定一次 → saved_instruction_execute 反复执行 ·
list / get / update（部分更新）/ delete

## 密钥 / 网络 / 底层
util_derive_address（地址派生唯一正道）/ util_derive_public_key / util_sign / util_verify ·
network_list / network_current / network_switch ·
rpc_block / rpc_resource / rpc_resource_path / rpc_access_value ·
view_single / view_multi（预构建 wire）· tx_simulate_raw / tx_submit_raw / tx_inspect_raw（postcard 原始通道）`,

	"chain": `# Milon 链特性 (doc_version ` + guideDocVersion + `)

## 基础
- 网络：devNet / localNet。network_switch 改服务端默认；X-Milon-Network 请求头按请求覆盖
- chainId：900000001（vc_attestation 缺省值）
- 代币：MIL，6 位精度 → 1 MIL = 1,000,000 最小单位；所有 amount 参数一律最小单位
- 地址：base58，20 字节。派生必须用 util_derive_address 或 account_generate 的返回，勿本地计算
- 交易：postcard 格式；交易哈希 hex 或 base58 均可作为查询参数
- 密码曲线：secp256k1（缺省）/ ed25519 / bls12381 / fndsa512（抗量子；公钥 897 字节、
  完整私钥 1281 字节，公钥无法从私钥派生——必须显式传，详见 keys 主题）
- 同一 32 字节私钥在不同曲线下派生不同地址——address 永远以 account_generate 返回为准

## paymentMode 六模式（各模式字段组合的唯一权威来源=contract_write.paymentMode 描述）
- unified_payer_all：payerPrivateKey（payerAddress/signatureMode 可自动派生）——一个账户付 gas 并执行，最常用
- unified_dual_sign：另加 ixAddress/ixPrivateKey(/ixSignatureMode)——付 gas 与指令执行账户不同
- unified_payer_only_gas：payerPrivateKey + payerAddress + signatureMode——只代付 gas
- split：ownerPrivateKey(/ownerAddress)——owner 付 gas 并签指令
- multi_signer：signers[]（≥1 个且地址不重复，可加 gasPayer 独立代付）——多签者分别签名
- sponsored：payerAddress + signatureMode——赞助模式

## 账户模型
- 普通账户：一把私钥一个地址；链上可有签名者列表（multisig 模式按 index 签）
- SFT 资源账户：独立密钥对持有 SFT 合约资源；创建时 gas 由 owner 代付；
  每笔 Mint 产出独立 token_id，同 slot 可 merge/transfer
- DID 文档：keys / services / 头像 / 别名（「alias-数字」格式全局唯一）；
  key 与 service 的 id 由链上分配，从 did_document 取

## Gas 与确认
- faucet 领水有冷却（FaucetCooldownActive，冷却剩余见 account_summary）；批量归集每账户预留 200 gas
- 写交易 wait:true 同请求等确认（缺省 60s，waitTimeoutSecs 可调）；事后补查用 tx_track
- 服务端单次调用总超时 300s——超长任务用 bulk_transfer 异步或拆小步骤`,

	"workflows": `# 典型调用流程 (doc_version ` + guideDocVersion + `)

## 1. 普通转账
account_generate（或已有私钥）→ faucet_claim 领水 → transfer_mil(to, amount, privateKey) → tx_track(hash)

## 2. 合约只读（查状态）
idl_apps 看清单 → idl_methods(appName) 拿方法与参数名 → contract_read；
多条查询一次打包 contract_read_multi（省往返）

## 3. 安全写合约（推荐默认）
idl_methods 拿参数 → contract_write_safe（参数同 contract_write）：
服务端先 simulate，失败返回 stage=simulate_failed 不上链，通过才真签提交

## 4. 直接写（参数已确定）
contract_write（paymentMode + 对应签名字段）+ wait:true —— 同请求拿 confirmed，省一次 tx_track

## 5. 多指令原子写
contract_write_multi（instructions[] 打包单笔交易，要么全成要么全不入）

## 6. 双账户写（付 gas ≠ 执行）
contract_write，paymentMode=unified_dual_sign + payer* 字段 + ixAddress/ixPrivateKey

## 7. owner 付 gas / 多签
contract_write，paymentMode=split + owner* 字段；或 paymentMode=multi_signer + signers[]（可加 gasPayer）

## 8. SFT 全周期
sft_flow 一次调用：owner 领水 → 创建/复用 SFT（缺省生成新资源账户，私钥随响应返回）→
创建/复用 slot → 逐笔 Mint 分发（≤20 笔）→ 合并 → 转移 → 回读验证。
不传某步参数即跳过；注意分发/合并/转移是链上状态变更，重跑会重复生效

## 9. VC 签发
vc_flow 一次调用：双方领水 → 双方建 DID → issuer 注册组织 → 链下签发 N 张 → user 逐张披露上链 →
回读验证。每步幂等（已存在自动跳过），可直接重跑

## 10. 批量空投/归集
bulk_transfer(count, toAddress) 立即返回 jobId → bulk_transfer_status(id) 轮询进度与明细

## 11. 重复执行某调用
saved_instruction_create 绑定方法+账户+签名 → saved_instruction_execute(id, mode=send) 反复上链；
参数变了用 saved_instruction_update 只改 args

## 12. postcard 高级通道
tx_inspect_raw 解析（不上链）→ tx_simulate_raw 干跑 → tx_submit_raw 提交；
view_single / view_multi 走预构建 wire`,

	"limits": `# 限制与不可逆清单 (doc_version ` + guideDocVersion + `)

## 体量与分页
- idl_metadata 全量约 200KB——必须 apps= 过滤或先走 idl_apps；未知名 400 报可用清单
- sft_flow distributions 单次最多 20 笔（每笔一笔链上交易）
- vc_flow credentialCount 缺省 5、上限 20；validUntilMs 须为未来值，0/null=永久
- bulk_transfer count 1..5000；concurrency 缺省 16、上限 128
- 服务端单次调用总超时 300s；tx_wait / tx_track 等待缺省 60s

## faucet
- 每次领 10000 MIL；有冷却（FaucetCooldownActive，剩余时间看 account_summary）
- 批量归集每账户转出 9800、预留 200 gas

## 不可逆 / 不可覆盖
- did_deactivate 不可逆；停用后全部 identity 写操作被拒（错误 1026）
- did_remove_key 最后一把密钥被链端拒绝（错误 1042）
- saved_instruction_delete 不可恢复
- sft_flow 的分发/合并/转移重跑会重复生效（链上状态变更，无幂等）
- vc_flow / did_create 幂等可重跑（已存在自动跳过/补齐差异，不覆盖已有）

## 格式与长度
- DID 头像 URI 1-512 字节（错误 1045）；服务 label 与绝对 URI 必填（1047/1048）
- DID 别名「alias-数字」全局唯一，撞名自动换号重试（suffix 可显式指定）
- SFT name 1-128 字符、symbol 1-32 字符；DID key/service id 由链上分配（≤255）
- util_sign 需服务端开启 ENABLE_UTIL_SIGN，否则 403
- VC 凭证签名仅支持 Ed25519 / FN-DSA-512

## 错误排查
错误码一律 error_lookup（query=十进制码或名字子串），返回来源+说明+处置建议；
常见：521=VcRequired · 1026=DID 已停用 · 1042=最后一柄密钥 · Cooldown=领水冷却`,

	"performance": `# 提速技巧 (doc_version ` + guideDocVersion + `)

## 少跑往返
- 批量读：contract_read_multi（IDL 方法级打包）或 view_multi（预构建 wire 打包），别循环单发
- 聚合读：account_summary 一次顶四查（余额/冻结/冷却/DID）
- 元数据：先 idl_apps（轻清单）再 idl_methods（单 app），别拉 idl_metadata 全量

## 少等一轮
- contract_write / contract_write_multi 带 wait:true —— 同请求内等确认（缺省 60s），
  响应直接带 confirmed/waitError，省一次 tx_track
- 收尾只调一次 tx_track（get+wait+events 聚合），别三连发

## 少拼参数
- 转账用 transfer_mil（自动补 token 地址与签名模式），别手组 contract_write
- 重复操作 saved_instruction_create 绑一次，之后 execute 反复跑；改参 update 只动 args

## 少走弯路
- 写前先 contract_write_safe：simulate 失败不上链，白省一笔 gas 与一轮等待
- 整段流程直接编排：sft_flow / vc_flow 一次调用顶十几步（内部并行领水、幂等跳过）
- 批量场景 bulk_transfer 异步化，主线程不阻塞，jobId 轮询即可
- 拿不准用法先调 guide（本工具）取对应主题页；错误码一步 error_lookup，避免盲试

## 长任务
单次调用总超时 300s；超长任务拆小步骤或走 bulk_transfer 异步通道`,

	"keys": `# 密钥与曲线 (doc_version ` + guideDocVersion + `)

## 四曲线对照
- secp256k1：32 字节私钥（hex/base58），缺省曲线
- ed25519：32 字节私钥；VC 凭证签名支持
- bls12381：32 字节私钥；仅 util 层派生用
- fndsa512（抗量子）：完整私钥 1281 字节、公钥 897 字节；公钥无法从私钥派生——
  必须显式传 publicKey（account_generate 的返回里有）

## 三条铁律
1. 地址派生只用 util_derive_address（或直接取 account_generate 返回），不要本地计算
2. 同一 32 字节私钥在 secp256k1/ed25519/fn-dsa-512 下派生出不同地址——
   传 address 时永远用生成时返回的那个，fndsa512 完整私钥不适用本规则
3. fndsa512 场景任何「自动派生公钥」都不成立：util_derive_public_key 不支持该曲线，
   transfer_mil / util_sign / did / vc 系列的 publicKey 字段此时必填

## signatureMode（签名模式对象）
- pubkey 模式：type=pubkey + publicKey；缺省时服务端有私钥就自动派生公钥模式
  （fndsa512 除外，必须显式传）
- multisig 模式：type=multisig + index（0 起对应链上 signers 列表顺序），
  显式传 index 时 publicKey 可省（链端按索引验签）
- keyType 参数只在「自动派生」路径用到；显式给了公钥就不用传

## VC 特例
凭证签名仅支持 Ed25519 / FN-DSA-512；issuer 为 fndsa512 时 issuerPublicKey 必填`,
}
