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

// RegisterTools 注册全部工具；本任务先注册基础 17 个，后续任务在此追加。
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
}
