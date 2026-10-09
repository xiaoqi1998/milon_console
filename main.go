package main

import (
	"fmt"
	"log"
	"os"

	"milon-api-server/client"
	"milon-api-server/config"
	"milon-api-server/handler"
	"milon-api-server/mcpserver"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	nm := client.NewNetworkManager(cfg.DefaultNetwork, cfg.RpcUrl)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware(cfg.EnableBodyLog))
	r.Use(middleware.SetupCORS(cfg.AllowedOrigins))

	// Static files（no-cache：每次改动后浏览器重新校验，防跨部署跑旧 JS）
	staticGroup := r.Group("/static", middleware.NoCache())
	staticGroup.Static("/", "./static")
	r.GET("/", middleware.NoCache(), func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// API routes
	networkHandler := handler.NewNetworkHandler(nm)
	systemHandler := handler.NewSystemHandler(nm)
	accountHandler := handler.NewAccountHandler(nm)
	transactionHandler := handler.NewTransactionHandler(nm)
	rpcHandler := handler.NewRpcHandler(nm)
	contractHandler := handler.NewContractHandler(nm)
	utilHandler := handler.NewUtilHandler(cfg.EnableUtilSign)
	vcAttestationHandler := handler.NewVcAttestationHandler()
	mockHandler := handler.NewMockHandler()
	faucetHandler := handler.NewFaucetHandler(nm)
	viewHandler := handler.NewViewSingleHandler(nm)
	resourcePathHandler := handler.NewResourcePathHandler(nm)
	idlHandler := handler.NewIDLHandler(nm)
	bulkTransferHandler := handler.NewBulkTransferHandler(nm)
	vcFlowHandler := handler.NewVcFlowHandler(nm)
	sftFlowHandler := handler.NewSftFlowHandler(nm)
	didHandler := handler.NewDidHandler(nm)
	savedInstructionHandler, err := handler.NewSavedInstructionHandler(nm, "./data")
	if err != nil {
		log.Fatalf("failed to init saved instruction store: %v", err)
	}
	mcpToolsHandler := handler.NewMcpToolsHandler()

	api := r.Group("/api")
	{
		// Network management
		netGroup := api.Group("/network")
		{
			netGroup.GET("/list", networkHandler.ListNetworks)
			netGroup.GET("/current", networkHandler.GetCurrentNetwork)
			netGroup.POST("/switch", networkHandler.SwitchNetwork)
		}

		// System
		api.GET("/health", systemHandler.Health)
		api.GET("/chain-head", systemHandler.GetChainHead)

		// Account
		api.POST("/accounts/generate", accountHandler.GenerateAccount)
		api.GET("/accounts/:address", accountHandler.GetAccount)
		api.GET("/accounts/:address/resources", accountHandler.GetAccountResources)

		// Transaction
		api.GET("/transactions/:hash", transactionHandler.GetTransactionByHash)
		api.GET("/transactions/:hash/parse", transactionHandler.GetTransactionByHashParsed)
		api.GET("/transactions/:hash/events", transactionHandler.GetTransactionEvents)
		api.GET("/transactions/:hash/wait", transactionHandler.WaitForTransaction)

		// RPC
		api.GET("/rpc/blocks/:height", rpcHandler.GetBlock)
		api.GET("/rpc/resources/:hash", rpcHandler.GetResource)
		api.POST("/rpc/access-value", rpcHandler.GetAccessValue)

		// Contract (read-only view)
		api.POST("/read", contractHandler.ReadContract)
		api.POST("/read/multi", contractHandler.ReadContractMulti)

		// Contract (simulate and write)
		api.POST("/simulate", contractHandler.SimulateContract)
		api.POST("/simulate/multi", contractHandler.SimulateContractMulti)
		api.POST("/write", contractHandler.WriteContract)
		api.POST("/write/multi", contractHandler.WriteContractMulti)
		api.POST("/write/multi-agent", contractHandler.WriteContractMultiAgent)
		api.POST("/write/multisig", contractHandler.WriteContractMultisig)

		// Transaction (simulate, submit, inspect)
		api.POST("/transactions/simulate", transactionHandler.SimulateTransaction)
		api.POST("/transactions/submit", transactionHandler.SubmitTransaction)
		api.POST("/transactions/inspect", transactionHandler.InspectTransaction)

		// Utility
		api.POST("/util/address/derive", utilHandler.DeriveAddress)
		api.POST("/util/key/derive-public", utilHandler.DerivePublicKey)
		api.POST("/util/sign", utilHandler.SignMessage)
		api.POST("/util/verify", utilHandler.VerifySignature)

		// VC attestation (generate DiscloseVcAttestation credential args)
		api.POST("/util/vc-attestation", vcAttestationHandler.GenerateVcAttestation)

		// Mock (testing: set a JSON payload, get a unique URL that echoes it back verbatim)
		api.POST("/util/mock/set", mockHandler.SetMockResponse)
		api.GET("/util/mock/:id", mockHandler.GetMockResponseByID)

		// Faucet (gas claim and balance)
		api.POST("/faucet/claim", faucetHandler.ClaimFaucet)
		api.GET("/faucet/balance/:address", faucetHandler.GetBalance)

		// Low-level view (pre-built postcard)
		api.POST("/view/single", viewHandler.ViewSingle)
		api.POST("/view/multi", viewHandler.ViewMulti)

		// Resource path by hash
		api.GET("/rpc/resource-paths/:hash", resourcePathHandler.GetResourcePathByHash)

		// IDL metadata (discovery for all IDL apps & methods)
		api.GET("/idl/metadata", idlHandler.GetIDLMetadata)

		// Bulk transfer (generate accounts, claim faucet, consolidate MIL)
		api.POST("/tool/bulk-transfer", bulkTransferHandler.BulkTransfer)
		api.GET("/tool/bulk-transfer/:id", bulkTransferHandler.GetBulkTransferStatus)

		// VC flow (faucet + DID + credentials issuance + disclosure, end-to-end)
		api.POST("/tool/vc-flow", vcFlowHandler.VcFlow)
		api.POST("/tool/sft-flow", sftFlowHandler.SftFlow)

		// DID full lifecycle (create+alias+services+avatar aggregate, granular management, document query)
		didGroup := api.Group("/tool/did")
		{
			didGroup.POST("/create", didHandler.Create)
			didGroup.POST("/set-alias", didHandler.SetAlias)
			didGroup.POST("/add-service", didHandler.AddService)
			didGroup.POST("/update-service", didHandler.UpdateService)
			didGroup.POST("/remove-service", didHandler.RemoveService)
			didGroup.POST("/set-avatar-uri", didHandler.SetAvatarUri)
			didGroup.POST("/add-key", didHandler.AddKey)
			didGroup.POST("/update-key", didHandler.UpdateKey)
			didGroup.POST("/remove-key", didHandler.RemoveKey)
			didGroup.POST("/deactivate", didHandler.Deactivate)
			didGroup.GET("/name-binding", didHandler.NameBinding)
			didGroup.GET("/:address/document", didHandler.Document)
		}

		// Saved instructions (store & replay IDL method calls)
		savedGroup := api.Group("/saved-instructions")
		{
			savedGroup.POST("", savedInstructionHandler.CreateSavedInstruction)
			savedGroup.GET("", savedInstructionHandler.ListSavedInstructions)
			savedGroup.GET("/:id", savedInstructionHandler.GetSavedInstruction)
			savedGroup.PUT("/:id", savedInstructionHandler.UpdateSavedInstruction)
			savedGroup.DELETE("/:id", savedInstructionHandler.DeleteSavedInstruction)
			savedGroup.POST("/:id/execute", savedInstructionHandler.ExecuteSavedInstruction)
		}

		// MCP tool inventory (programmatic discovery of all /mcp tools)
		api.GET("/mcp/tools", mcpToolsHandler.ListTools)
	}

	// MCP 端点：把全部 REST 能力暴露为 MCP 工具（Streamable HTTP, stateless）
	r.Any("/mcp", gin.WrapH(mcpserver.NewMCPHandler(os.Getenv("MCP_AUTH_TOKEN"))))

	// Print startup banner
	fmt.Println("========================================")
	fmt.Println("  Milon API Server")
	fmt.Println("========================================")
	fmt.Printf("  Listening on:  http://localhost:%s\n", cfg.ServerPort)
	fmt.Println("  Default network:", cfg.DefaultNetwork)
	if cfg.RpcUrl != "" {
		fmt.Println("  RPC URL (env): ", cfg.RpcUrl)
	} else {
		nmClient, netCfg := nm.GetCurrent()
		_ = nmClient
		fmt.Println("  RPC URL:       ", netCfg.RpcUrl)
	}
	fmt.Println("  Enable util sign:", cfg.EnableUtilSign)
	fmt.Println("  Enable body log:", cfg.EnableBodyLog)
	fmt.Println("  Endpoints:")
	fmt.Println("    GET  /api/health                  - Health check")
	fmt.Println("    GET  /api/chain-head              - Get chain head")
	fmt.Println("    GET  /api/network/list            - List all networks")
	fmt.Println("    GET  /api/network/current         - Get current network")
	fmt.Println("    POST /api/network/switch          - Switch network")
	fmt.Println("    GET  /api/accounts/:address       - Get account info")
	fmt.Println("    GET  /api/accounts/:address/resources - List account resources")
	fmt.Println("    POST /api/accounts/generate       - Generate new account")
	fmt.Println("    GET  /api/transactions/:hash      - Get transaction by hash")
	fmt.Println("    GET  /api/transactions/:hash/parse - Get transaction with IDL-decoded output")
	fmt.Println("    GET  /api/transactions/:hash/events - Get transaction events")
	fmt.Println("    GET  /api/transactions/:hash/wait - Wait for transaction")
	fmt.Println("    GET  /api/rpc/blocks/:height      - Get block by height")
	fmt.Println("    GET  /api/rpc/resources/:hash     - Get resource by hash")
	fmt.Println("    POST /api/rpc/access-value        - Get access value")
	fmt.Println("    POST /api/read                    - Read contract (view)")
	fmt.Println("    POST /api/read/multi              - Read contract (multi-view)")
	fmt.Println("    POST /api/simulate                - Simulate contract")
	fmt.Println("    POST /api/simulate/multi          - Simulate multi-instruction batch")
	fmt.Println("    POST /api/write                   - Write contract")
	fmt.Println("    POST /api/write/multi             - Write multi-instruction batch (pack multiple ix into one tx)")
	fmt.Println("    POST /api/write/multi-agent       - Write contract (dual sign)")
	fmt.Println("    POST /api/write/multisig          - Write contract (split)")
	fmt.Println("    POST /api/transactions/simulate   - Simulate raw transaction")
	fmt.Println("    POST /api/transactions/submit     - Submit raw transaction")
	fmt.Println("    POST /api/transactions/inspect    - Inspect raw transaction")
	fmt.Println("    POST /api/util/address/derive     - Derive address from public key")
	fmt.Println("    POST /api/util/key/derive-public  - Derive public key from private key")
	fmt.Println("    POST /api/util/sign               - Sign message (requires ENABLE_UTIL_SIGN)")
	fmt.Println("    POST /api/util/verify             - Verify signature")
	fmt.Println("    POST /api/util/vc-attestation     - Generate DiscloseVcAttestation credential args")
	fmt.Println("    POST /api/util/mock/set           - Set mock response body, returns a unique URL")
	fmt.Println("    GET  /api/util/mock/:id           - Echo back the mock response body by id")
	fmt.Println("    POST /api/faucet/claim            - Claim faucet tokens")
	fmt.Println("    GET  /api/faucet/balance/:address - Get MIL balance")
	fmt.Println("    POST /api/view/single             - Low-level single view")
	fmt.Println("    POST /api/view/multi              - Low-level multi view")
	fmt.Println("    GET  /api/rpc/resource-paths/:hash- Get resource path by hash")
	fmt.Println("    GET  /api/idl/metadata            - IDL 方法元数据（app/方法/参数 schema）")
	fmt.Println("    POST /api/tool/bulk-transfer      - 批量生成账户并归集 MIL（异步任务）")
	fmt.Println("    GET  /api/tool/bulk-transfer/:id  - 查询批量归集任务进度")
	fmt.Println("    POST /api/tool/vc-flow            - VC 签发披露全流程（领水+DID+组织+凭证，同步）")
	fmt.Println("    POST /api/tool/sft-flow           - SFT 全流程（创建SFT+slot+分发+合并+转移，参数控制步骤）")
	fmt.Println("    POST /api/tool/did/create         - DID 一键创建（别名+服务+头像聚合，幂等补齐）")
	fmt.Println("    POST /api/tool/did/set-alias      - 设置/更换 DID 别名")
	fmt.Println("    POST /api/tool/did/add-service    - 添加 DID 服务端点")
	fmt.Println("    POST /api/tool/did/update-service - 更新指定服务端点")
	fmt.Println("    POST /api/tool/did/remove-service - 移除指定服务端点")
	fmt.Println("    POST /api/tool/did/set-avatar-uri - 设置 DID 头像 URI")
	fmt.Println("    POST /api/tool/did/add-key        - 添加 DID 密钥")
	fmt.Println("    POST /api/tool/did/update-key     - 更新 DID 密钥")
	fmt.Println("    POST /api/tool/did/remove-key     - 移除 DID 密钥")
	fmt.Println("    POST /api/tool/did/deactivate     - 停用 DID")
	fmt.Println("    GET  /api/tool/did/:address/document - 查询 DID 文档")
	fmt.Println("    GET  /api/tool/did/name-binding   - 按别名反查 DID 绑定")
	fmt.Println("    ANY  /mcp                         - MCP 端点（57 个工具，供 AI 编程代理接入）")
	fmt.Println("    GET  /api/mcp/tools               - MCP 工具清单（与 /mcp 注册表同源）")
	fmt.Println("    GET  /                            - Web console")
	fmt.Println("    GET  /static/*                    - Static files")
	fmt.Println("========================================")

	addr := ":" + cfg.ServerPort
	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
