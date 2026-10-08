package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 镜像 REST request struct 的输入类型（json tag 与 REST 逐字一致）。
// 无参数的工具统一用 emptyArgs。

// emptyArgs 对应无参数的 REST 调用。
type emptyArgs struct{}

type networkSwitchArgs struct {
	Network string `json:"network" jsonschema:"目标网络名：devNet 或 localNet"`
}

type accountGenerateArgs struct {
	KeyType string `json:"keyType" jsonschema:"密钥曲线：secp256k1(缺省)/ed25519/fn-dsa-512"`
}

type addressArgs struct {
	Address string `json:"address"`
}

type faucetClaimArgs struct {
	PrivateKey    string `json:"privateKey" jsonschema:"领款账户私钥（hex 或 base58）"`
	Address       string `json:"address" jsonschema:"领款账户地址"`
	SignatureMode any    `json:"signatureMode" jsonschema:"签名模式对象，如 {\"variant\":0}；公钥模式或签名者列表模式"`
}

type txParseArgs struct {
	Hash   string `json:"hash"`
	Remote string `json:"remote,omitempty" jsonschema:"可选：true/1 时附取各 inline 写入资源的链上现值与外部 blob 值"`
}

type txEventsArgs struct {
	Hash    string `json:"hash"`
	TypeTag string `json:"typeTag,omitempty" jsonschema:"可选：按事件 typeTag 过滤（十进制整数）"`
}

type txWaitArgs struct {
	Hash        string `json:"hash"`
	TimeoutSecs string `json:"timeoutSecs,omitempty" jsonschema:"可选：等待超时秒数（十进制整数）"`
}

type hashArgs struct {
	Hash string `json:"hash"`
}

type heightArgs struct {
	Height string `json:"height"`
}

type accessValueArgs struct {
	BlobHashes []string `json:"blobHashes"`
}

// ==================== Task 3：合约/视图/原始交易家族（13 个）====================
// 字段 json tag 逐字镜像 handler 侧 request struct（唯一事实源），注释标注来源行号。
// 注意：handler 侧的 json.RawMessage 字段（signatureMode 等）这里镜像为 any——
// 实验证明 SDK 的 schema 推断会把 json.RawMessage（[]byte）判成 "null|array"，
// 客户端传 JSON 对象会被校验拒绝；any 的 wire 序列化与 RawMessage 完全等价
// （原始 JSON 值原样透传给 REST handler），且 Task 1 的 faucetClaimArgs 已有先例。

// signerEntry 镜像 types.SignerEntry（types/request.go:17，multi_signer 模式的签名者）。
// address/signatureMode 保持 required（镜像 handler 的 binding:"required"）；
// privateKey 仅写交易需要（模拟走模拟签名），标 omitempty 允许省略。
type signerEntry struct {
	Address       string `json:"address" jsonschema:"签名者地址"`
	PrivateKey    string `json:"privateKey,omitempty" jsonschema:"签名者私钥（写交易必填；模拟走模拟签名可省）"`
	SignatureMode any    `json:"signatureMode" jsonschema:"签名模式对象，如 {\"variant\":0}（公钥模式）或 {\"type\":\"multisig\",\"index\":N}"`
}

// contractReadArgs 镜像 readContractRequest（handler/contract.go:43，POST /api/read）。
type contractReadArgs struct {
	AppName      string         `json:"appName" jsonschema:"IDL 里的 app 名，先用 idl_metadata 查"`
	MethodName   string         `json:"methodName" jsonschema:"IDL 里的方法名"`
	Args         map[string]any `json:"args,omitempty" jsonschema:"方法参数对象（键名见 idl_metadata）"`
	PayerAddress string         `json:"payerAddress,omitempty"`
}

// readContractMultiItemArgs 镜像 readContractMultiItem（handler/contract.go:107）。
type readContractMultiItemArgs struct {
	AppName    string         `json:"appName"`
	MethodName string         `json:"methodName"`
	Args       map[string]any `json:"args,omitempty"`
}

// contractReadMultiArgs 镜像 readContractMultiRequest（handler/contract.go:103，POST /api/read/multi）。
type contractReadMultiArgs struct {
	Instructions []readContractMultiItemArgs `json:"instructions"`
}

// contractSimulateArgs 镜像 simulateContractRequest（handler/contract.go:165，POST /api/simulate）。
type contractSimulateArgs struct {
	AppName         string         `json:"appName"`
	MethodName      string         `json:"methodName"`
	Args            map[string]any `json:"args,omitempty"`
	PaymentMode     string         `json:"paymentMode" jsonschema:"unified_payer_all/unified_dual_sign/unified_payer_only_gas/split/multi_signer/sponsored"`
	PayerAddress    string         `json:"payerAddress,omitempty"`
	SignatureMode   any            `json:"signatureMode,omitempty" jsonschema:"签名模式对象，如 {\"variant\":0}；缺省公钥模式"`
	IxAddress       string         `json:"ixAddress,omitempty" jsonschema:"unified_dual_sign 专用：指令执行账户地址"`
	IxSignatureMode any            `json:"ixSignatureMode,omitempty"`
	OwnerAddress    string         `json:"ownerAddress,omitempty" jsonschema:"split 专用：owner 地址，缺省取 payerAddress"`
	Signers         []signerEntry  `json:"signers,omitempty" jsonschema:"multi_signer 专用：签名者列表"`
	GasPayer        *signerEntry   `json:"gasPayer,omitempty" jsonschema:"multi_signer 可选：独立 gas 代付账户"`
}

// multiInstructionItemArgs 镜像 multiInstructionItem（handler/contract.go:1125）。
type multiInstructionItemArgs struct {
	AppName    string         `json:"appName"`
	MethodName string         `json:"methodName"`
	Args       map[string]any `json:"args,omitempty"`
}

// contractMultiArgs 镜像 multiContractRequest（handler/contract.go:1132，
// POST /api/simulate/multi 与 /api/write/multi 共用）。
type contractMultiArgs struct {
	Instructions    []multiInstructionItemArgs `json:"instructions"`
	PaymentMode     string                     `json:"paymentMode" jsonschema:"unified_payer_all/unified_dual_sign/unified_payer_only_gas/split/multi_signer/sponsored"`
	PayerPrivateKey string                     `json:"payerPrivateKey,omitempty"`
	PayerAddress    string                     `json:"payerAddress,omitempty"`
	SignatureMode   any                        `json:"signatureMode,omitempty"`
	IxAddress       string                     `json:"ixAddress,omitempty" jsonschema:"unified_dual_sign 专用：指令执行账户"`
	IxPrivateKey    string                     `json:"ixPrivateKey,omitempty"`
	IxSignatureMode any                        `json:"ixSignatureMode,omitempty"`
	OwnerPrivateKey string                     `json:"ownerPrivateKey,omitempty" jsonschema:"split 专用：owner 私钥，缺省取 payerPrivateKey"`
	OwnerAddress    string                     `json:"ownerAddress,omitempty"`
	Signers         []signerEntry              `json:"signers,omitempty"`
	GasPayer        *signerEntry               `json:"gasPayer,omitempty"`
}

// contractWriteArgs 镜像 writeContractRequest（handler/contract.go:789，
// POST /api/write、/api/write/multi-agent（WriteContractMultiAgent:845）、
// /api/write/multisig（WriteContractMultisig:881）共用）。
type contractWriteArgs struct {
	AppName         string         `json:"appName"`
	MethodName      string         `json:"methodName"`
	Args            map[string]any `json:"args,omitempty"`
	PaymentMode     string         `json:"paymentMode" jsonschema:"unified_payer_all/unified_dual_sign/unified_payer_only_gas/split/multi_signer/sponsored"`
	PayerPrivateKey string         `json:"payerPrivateKey,omitempty"`
	PayerAddress    string         `json:"payerAddress,omitempty"`
	SignatureMode   any            `json:"signatureMode,omitempty"`
	IxAddress       string         `json:"ixAddress,omitempty" jsonschema:"unified_dual_sign 专用：指令执行账户"`
	IxPrivateKey    string         `json:"ixPrivateKey,omitempty"`
	IxSignatureMode any            `json:"ixSignatureMode,omitempty"`
	OwnerPrivateKey string         `json:"ownerPrivateKey,omitempty" jsonschema:"split 专用：owner 私钥，缺省取 payerPrivateKey"`
	OwnerAddress    string         `json:"ownerAddress,omitempty" jsonschema:"split 专用：owner 地址，缺省取 payerAddress"`
	Signers         []signerEntry  `json:"signers,omitempty" jsonschema:"multi_signer 专用：签名者列表"`
	GasPayer        *signerEntry   `json:"gasPayer,omitempty" jsonschema:"multi_signer 可选：独立 gas 代付账户"`
}

// rawTransactionArgs 镜像 rawTransactionRequest（handler/transaction_handler.go:293，
// POST /api/transactions/simulate、/submit（SubmitTransaction:348）、
// /inspect（InspectTransaction:399）共用）。
type rawTransactionArgs struct {
	TransactionPostcard string `json:"transactionPostcard" jsonschema:"base64 编码的交易 postcard 原文"`
}

// rawViewArgs 镜像 rawViewRequest（handler/view_handler.go:48，
// POST /api/view/single 与 /api/view/multi（ViewMulti:93）共用）。
type rawViewArgs struct {
	TransactionPostcard string `json:"transactionPostcard" jsonschema:"base64 编码的视图 postcard 原文（单条或多条 wire 打包）"`
}

// registerTool 把"输入结构 → REST 映射"注册为 MCP 工具。
func registerTool[In any](srv *mcp.Server, exec *Executor, name, desc string, m RESTMapping) {
	mcp.AddTool[In, any](srv, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, any, error) {
			raw, err := json.Marshal(args)
			if err != nil {
				return nil, nil, err
			}
			out, err := exec.Call(ctx, m, raw)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{
				IsError: out.IsError,
				Content: []mcp.Content{&mcp.TextContent{Text: string(out.Body)}},
			}, nil, nil
		})
}

// BuildServer 构造注册了全部工具的 MCP Server。
// Task 7 的 server.go 会复用；当前先放 tools.go。
func BuildServer(baseURL string) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "milon-api-server", Version: "1.0"}, nil)
	RegisterTools(srv, NewExecutor(baseURL, nil))
	return srv
}

// NewMCPHandler 构造挂到 gin 的 /mcp handler；token 为空则不鉴权。
// 回环基地址优先取 MILON_REST_BASE_URL（测试注入），缺省 http://127.0.0.1:$SERVER_PORT。
func NewMCPHandler(token string) http.Handler {
	baseURL := os.Getenv("MILON_REST_BASE_URL")
	if baseURL == "" {
		port := os.Getenv("SERVER_PORT")
		if port == "" {
			port = "8080"
		}
		baseURL = "http://127.0.0.1:" + port
	}
	srv := BuildServer(baseURL)
	// Stateless：免 initialize、每请求临时会话；JSONResponse：POST 响应用
	// application/json 而非 SSE 流（协议 §2.1.5 允许，便于非流式客户端直读）。
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	if token == "" {
		return h
	}
	return bearerAuth(token, h)
}

// bearerAuth 校验 Authorization: Bearer <token> 后转发给 MCP handler。
func bearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"invalid MCP token"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RegisterTools 注册全部工具；基础 17 个 + Task 3 合约/视图/原始交易 13 个，后续任务在此追加。
func RegisterTools(srv *mcp.Server, exec *Executor) {
	registerTool[emptyArgs](srv, exec, "network_list", "列出内置网络（devNet/localNet）及当前网络", RESTMapping{Method: "GET", PathTemplate: "/api/network/list"})
	registerTool[emptyArgs](srv, exec, "network_current", "查询当前网络", RESTMapping{Method: "GET", PathTemplate: "/api/network/current"})
	registerTool[networkSwitchArgs](srv, exec, "network_switch", "切换当前网络（devNet/localNet）", RESTMapping{Method: "POST", PathTemplate: "/api/network/switch"})
	registerTool[accountGenerateArgs](srv, exec, "account_generate", "生成新账户（返回私钥/公钥/地址）", RESTMapping{Method: "POST", PathTemplate: "/api/accounts/generate"})
	registerTool[addressArgs](srv, exec, "account_info", "查询账户信息", RESTMapping{Method: "GET", PathTemplate: "/api/accounts/{address}", PathParams: []string{"address"}})
	registerTool[addressArgs](srv, exec, "account_resources", "查询账户链上资源", RESTMapping{Method: "GET", PathTemplate: "/api/accounts/{address}/resources", PathParams: []string{"address"}})
	registerTool[faucetClaimArgs](srv, exec, "faucet_claim", "从 faucet 领取代币（需私钥签名，等待确认后返回 txHash）", RESTMapping{Method: "POST", PathTemplate: "/api/faucet/claim"})
	registerTool[addressArgs](srv, exec, "faucet_balance", "查询账户余额（faucet 口径）", RESTMapping{Method: "GET", PathTemplate: "/api/faucet/balance/{address}", PathParams: []string{"address"}})
	registerTool[hashArgs](srv, exec, "tx_get", "按哈希查询交易", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}", PathParams: []string{"hash"}})
	registerTool[txParseArgs](srv, exec, "tx_parse", "解析交易（结构化 postcard）", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/parse", PathParams: []string{"hash"}, QueryParams: []string{"remote"}})
	registerTool[txEventsArgs](srv, exec, "tx_events", "查询交易事件", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/events", PathParams: []string{"hash"}, QueryParams: []string{"typeTag"}})
	registerTool[txWaitArgs](srv, exec, "tx_wait", "等待交易确认（可能长轮询）", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/wait", PathParams: []string{"hash"}, QueryParams: []string{"timeoutSecs"}})
	registerTool[heightArgs](srv, exec, "rpc_block", "按高度查区块头", RESTMapping{Method: "GET", PathTemplate: "/api/rpc/blocks/{height}", PathParams: []string{"height"}})
	registerTool[hashArgs](srv, exec, "rpc_resource", "按哈希查链上资源原文", RESTMapping{Method: "GET", PathTemplate: "/api/rpc/resources/{hash}", PathParams: []string{"hash"}})
	registerTool[accessValueArgs](srv, exec, "rpc_access_value", "按 blob 哈希批量取 access value", RESTMapping{Method: "POST", PathTemplate: "/api/rpc/access-value"})
	registerTool[hashArgs](srv, exec, "rpc_resource_path", "按哈希查资源路径", RESTMapping{Method: "GET", PathTemplate: "/api/rpc/resource-paths/{hash}", PathParams: []string{"hash"}})
	registerTool[emptyArgs](srv, exec, "idl_metadata", "IDL 元数据发现：列出全部 app/方法/参数/返回值/signer 角色（调用合约工具前的第一站）", RESTMapping{Method: "GET", PathTemplate: "/api/idl/metadata"})

	// ---- Task 3：合约/视图/原始交易家族（13 个，全部 POST 纯 JSON body，无 query/path 参数）----

	// 合约只读（view）
	registerTool[contractReadArgs](srv, exec, "contract_read", "只读合约调用（按 IDL 方法名查询链上状态）", RESTMapping{Method: "POST", PathTemplate: "/api/read"})
	registerTool[contractReadMultiArgs](srv, exec, "contract_read_multi", "批量只读合约调用（多指令打包单次查询）", RESTMapping{Method: "POST", PathTemplate: "/api/read/multi"})

	// 合约模拟（dry-run，不消耗 gas）
	registerTool[contractSimulateArgs](srv, exec, "contract_simulate", "模拟执行合约写调用（dry-run，模拟签名，无需私钥）", RESTMapping{Method: "POST", PathTemplate: "/api/simulate"})
	registerTool[contractMultiArgs](srv, exec, "contract_simulate_multi", "多指令打包模拟执行（单笔交易原子 dry-run）", RESTMapping{Method: "POST", PathTemplate: "/api/simulate/multi"})

	// 合约写（真实上链）
	registerTool[contractWriteArgs](srv, exec, "contract_write", "构建并提交合约写交易（按 paymentMode 签名，真实上链）", RESTMapping{Method: "POST", PathTemplate: "/api/write"})
	registerTool[contractMultiArgs](srv, exec, "contract_write_multi", "多指令打包写交易（单笔交易原子上链）", RESTMapping{Method: "POST", PathTemplate: "/api/write/multi"})
	registerTool[contractWriteArgs](srv, exec, "contract_write_multi_agent", "双账户写交易（unified_dual_sign：付 gas 与指令执行账户不同）", RESTMapping{Method: "POST", PathTemplate: "/api/write/multi-agent"})
	registerTool[contractWriteArgs](srv, exec, "contract_write_multisig", "split 模式写交易（owner 付 gas 并签指令）", RESTMapping{Method: "POST", PathTemplate: "/api/write/multisig"})

	// 原始 postcard 交易（simulate/submit/inspect 共用 rawTransactionRequest）
	registerTool[rawTransactionArgs](srv, exec, "tx_simulate_raw", "模拟执行 postcard 原始交易（不消耗 gas）", RESTMapping{Method: "POST", PathTemplate: "/api/transactions/simulate"})
	registerTool[rawTransactionArgs](srv, exec, "tx_submit_raw", "提交 postcard 原始交易上链", RESTMapping{Method: "POST", PathTemplate: "/api/transactions/submit"})
	registerTool[rawTransactionArgs](srv, exec, "tx_inspect_raw", "解析 postcard 原始交易（返回 txHash/ixHashes/payer/valid，不上链）", RESTMapping{Method: "POST", PathTemplate: "/api/transactions/inspect"})

	// 低层视图（预构建 postcard）
	registerTool[rawViewArgs](srv, exec, "view_single", "低层视图调用（预构建 postcard wire）", RESTMapping{Method: "POST", PathTemplate: "/api/view/single"})
	registerTool[rawViewArgs](srv, exec, "view_multi", "低层批量视图调用（预构建 postcard 多 wire 打包）", RESTMapping{Method: "POST", PathTemplate: "/api/view/multi"})
}
