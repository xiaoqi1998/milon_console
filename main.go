package main

import (
	"fmt"
	"log"

	"milon-api-server/client"
	"milon-api-server/config"
	"milon-api-server/handler"
	"milon-api-server/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	nm := client.NewNetworkManager(cfg.DefaultNetwork)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware())
	r.Use(middleware.SetupCORS(cfg.AllowedOrigins))

	// Static files
	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
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
		api.GET("/transactions/:hash/events", transactionHandler.GetTransactionEvents)
		api.GET("/transactions/:hash/wait", transactionHandler.WaitForTransaction)

		// RPC
		api.GET("/rpc/blocks/:height", rpcHandler.GetBlock)
		api.GET("/rpc/resources/:hash", rpcHandler.GetResource)
		api.POST("/rpc/access-value", rpcHandler.GetAccessValue)

		// Contract (read-only view)
		api.POST("/read", contractHandler.ReadContract)

		// Contract (simulate and write)
		api.POST("/simulate", contractHandler.SimulateContract)
		api.POST("/write", contractHandler.WriteContract)
		api.POST("/write/multi-agent", contractHandler.WriteContractMultiAgent)
		api.POST("/write/multisig", contractHandler.WriteContractMultisig)

		// Transaction (simulate and submit)
		api.POST("/transactions/simulate", transactionHandler.SimulateTransaction)
		api.POST("/transactions/submit", transactionHandler.SubmitTransaction)

		// Utility
		api.POST("/util/address/derive", utilHandler.DeriveAddress)
		api.POST("/util/key/derive-public", utilHandler.DerivePublicKey)
		api.POST("/util/sign", utilHandler.SignMessage)
		api.POST("/util/verify", utilHandler.VerifySignature)
	}

	// Print startup banner
	fmt.Println("========================================")
	fmt.Println("  Milon API Server")
	fmt.Println("========================================")
	fmt.Printf("  Listening on:  http://localhost:%s\n", cfg.ServerPort)
	fmt.Println("  Default network:", cfg.DefaultNetwork)
	fmt.Println("  Enable util sign:", cfg.EnableUtilSign)
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
	fmt.Println("    GET  /api/transactions/:hash/events - Get transaction events")
	fmt.Println("    GET  /api/transactions/:hash/wait - Wait for transaction")
	fmt.Println("    GET  /api/rpc/blocks/:height      - Get block by height")
	fmt.Println("    GET  /api/rpc/resources/:hash     - Get resource by hash")
	fmt.Println("    POST /api/rpc/access-value        - Get access value")
	fmt.Println("    POST /api/read                    - Read contract (view)")
	fmt.Println("    POST /api/simulate                - Simulate contract")
	fmt.Println("    POST /api/write                   - Write contract")
	fmt.Println("    POST /api/write/multi-agent       - Write contract (dual sign)")
	fmt.Println("    POST /api/write/multisig          - Write contract (split)")
	fmt.Println("    POST /api/transactions/simulate   - Simulate raw transaction")
	fmt.Println("    POST /api/transactions/submit     - Submit raw transaction")
	fmt.Println("    POST /api/util/address/derive     - Derive address from public key")
	fmt.Println("    POST /api/util/key/derive-public  - Derive public key from private key")
	fmt.Println("    POST /api/util/sign               - Sign message (requires ENABLE_UTIL_SIGN)")
	fmt.Println("    POST /api/util/verify             - Verify signature")
	fmt.Println("    GET  /                            - Web console")
	fmt.Println("    GET  /static/*                    - Static files")
	fmt.Println("========================================")

	addr := ":" + cfg.ServerPort
	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
