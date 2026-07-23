package milon

import (
	"crypto/rand"
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func main() {
	//demo1()

	//Client_GetChainHead_GetBlock(DevNetConfig)

	Client_Token_Create_StepByStep(DevNetConfig)

	//Client_Token_Create_BuildAndSimulateSingleIxUnifiedPayerAll_BuildAndSubmitSingleIxUnifiedPayerSignAll(DevNetConfig)
	//Client_Token_Create_BuildAndSimulateSingleIxUnifiedDualSign_BuildAndSubmitSingleIxUnifiedDualSign(DevNetConfig)
	//Client_Account_Create_BuildAndSimulateSingleIxUnifiedDualSign_BuildAndSubmitSingleIxUnifiedPayerOnlyGas(DevNetConfig)
	//Client_Token_Create_BuildAndSimulateSingleIxSplit_BuildAndSubmitSingleIxSplit(DevNetConfig)

	//Client_Demo_InitPool_BatchCredit_BuildAndSimulateMultiIxUnified_BuildAndSubmitMultiIxUnified(DevNetConfig)
	//Client_Token_Create_Mint_Transfer_BuildAndSimulateMultiIxSplit_BuildAndSubmitMultiIxSplit_And_BuildAndViewSingleIx_And_BuildAndViewMultiIx_SameWires(DevNetConfig)

	//Client_Account_Create_And_BuildAndViewMultiIx_differentWires(DevNetConfig)

	//Client_Account_Create_4_Crypto(DevNetConfig)

	//Client_GetTxByHash(DevNetConfig, "8M12Gp7RqMQQ3uycyh19HTCX3QoDy9ZU1uin8TvEP5WC")
	//Client_GetAccount(DevNetConfig, "3o1yZH8purCJFFGc6Ai5VT1QbBPs")
	//Client_GetBlock(DevNetConfig, 10)
	//Client_EventsByTxHash(DevNetConfig, "apiHashBase58", nil)
	//Client_ListResourcePath_GetResourcePathByHash(DevNetConfig)
}

func Client_GetChainHead_GetBlock(config NetworkConfig) {
	client := NewMilonClient(config)

	fmt.Printf("\n\n ----------1.GetChainHead---------------------- \n\n")

	chainHeadResult, err := client.GetChainHead(1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("chainHeadResult : %+v \n\n", chainHeadResult)
	fmt.Printf("chainHeadResult.BodyChainHead : %+v \n\n", chainHeadResult.BodyChainHead)

	fmt.Printf("\n\n ----------2.GetBlock---------------------- \n\n")

	getBlockResult, err := client.GetBlock(chainHeadResult.BodyChainHead.BlockHeight, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("getBlockResult : %+v \n\n", getBlockResult)
	fmt.Printf("getBlockResult.BodyBlock : %+v \n", getBlockResult.BodyBlock)
}

func Client_Token_Create_StepByStep(config NetworkConfig) {
	client := NewMilonClient(config)

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	tokenAddress, _ := crypto.NewAddressFromPublicKey(tokenSk.Ed25519Public())

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	ownerAddress, _ := crypto.NewAddressFromPublicKey(ownerPk)

	payerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	payerPk := payerSk.Ed25519Public()
	payerAddress, _ := crypto.NewAddressFromPublicKey(payerPk)

	fmt.Printf("tokenAddress = %v \n", tokenAddress)
	fmt.Printf("ownerAddress = %v \n", ownerAddress)
	fmt.Printf("payerAddress = %v \n", payerAddress)

	// 1.加载pdl
	pd, err := client.GetPdByIDLAppName("token")
	if err != nil {
		panic(err)
	}

	// 2.创建1个指令
	wire, _ := pd.Encode(
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Multi Ix Token",
				"symbol":   "SMIX",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
	)

	// 3.模拟交易
	simulateTransaction, err := client.CreateTransactionWithParam([]api.PackedInstruction{wire}, payerAddress)
	if err != nil {
		panic(err)
	}

	payerSig, err := simulateTransaction.SimulateSignPayer(*payerAddress, PubKeySignatureMode{PublicKey: *payerPk})
	if err != nil {
		panic(err)
	}
	simulateTransaction.AddSignature(*payerAddress, *payerSig)

	tokenSig, err := simulateTransaction.SimulateSignIx(*tokenAddress, 0, PubKeySignatureMode{PublicKey: *tokenPk})
	if err != nil {
		panic(err)
	}
	simulateTransaction.AddSignature(*tokenAddress, *tokenSig)

	simulateTransactionPostcard, err := simulateTransaction.ToBytes()
	if err != nil {
		panic(err)
	}

	simulateTransactionResult, err := client.SimulateTx(simulateTransactionPostcard, 1)
	if err != nil {
		panic(err)
	}

	if simulateTransactionResult.BodySimulateReceipt.State != api.TxStateSuccess {
		fmt.Printf("simulateTransactionResult.BodySimulateReceipt.State != api.TxStateSuccess")
		return
	}

	// 4.创建 Transaction
	transaction, err := client.CreateTransactionWithParam([]api.PackedInstruction{wire}, payerAddress)
	if err != nil {
		panic(err)
	}

	// 5.payerSk 签名 payer
	err = client.SignPayerAndAddSignature(transaction, payerSk, *payerAddress, PubKeySignatureMode{PublicKey: *payerPk})
	if err != nil {
		panic(err)
	}

	// 6.ixSk 签名 指令
	err = client.SignIxAndAddSignature(transaction, 0, tokenSk, *tokenAddress, PubKeySignatureMode{PublicKey: *tokenPk})
	if err != nil {
		panic(err)
	}

	// 7.验证交易结构
	err = transaction.ValidateWire()
	if err != nil {
		panic(err)
	}

	// 8.序列化并上链
	transactionPostcard, err := transaction.ToBytes()
	if err != nil {
		panic(err)
	}

	submitTransactionResult, err := client.SubmitTx(transactionPostcard, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("submitTransactionResult = %+v \n\n", submitTransactionResult)

	fmt.Printf("\n\n ----------1.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------2.Client_GetTxByHash---------------------- \n\n")
	Client_GetTxByHash(config, submitTransactionResult.BodyTxHash)

	fmt.Printf("\n\n ----------3.Client_GetTxByHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)
}

func Client_Token_Create_BuildAndSimulateSingleIxUnifiedPayerAll_BuildAndSubmitSingleIxUnifiedPayerSignAll(config NetworkConfig) {
	client := NewMilonClient(config)

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	tokenAddress, _ := crypto.NewAddressFromPublicKey(tokenPk)

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	ownerAddress, _ := crypto.NewAddressFromPublicKey(ownerPk)

	fmt.Printf("tokenAddress = %v \n", tokenAddress)
	fmt.Printf("ownerAddress = %v \n", ownerAddress)

	fmt.Printf("\n\n ----------1.BuildAndSimulateSingleIxUnifiedPayerAll---------------------- \n\n")

	simulateTransactionResult, err := client.BuildAndSimulateSingleIxUnifiedPayerAll(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Multi Ix Token",
				"symbol":   "SMIX",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		*tokenAddress,
		PubKeySignatureMode{PublicKey: *tokenPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("simulateTransactionResult = %+v \n\n", simulateTransactionResult)
	fmt.Printf("simulateTransactionResult.BodySimulateReceipt = %+v \n\n", simulateTransactionResult.BodySimulateReceipt)

	Debug_SimulateTransactionResult(config, simulateTransactionResult.BodySimulateReceipt)

	fmt.Printf("\n\n ----------2.BuildAndSubmitSingleIxUnifiedPayerSignAll---------------------- \n\n")

	submitTransactionResult, err := client.BuildAndSubmitSingleIxUnifiedPayerSignAll(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Multi Ix Token",
				"symbol":   "SMIX",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		tokenSk,
		*tokenAddress,
		PubKeySignatureMode{PublicKey: *tokenPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("result = %+v \n\n", submitTransactionResult)

	fmt.Printf("\n\n ----------3.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------4.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)
}

func Client_Token_Create_BuildAndSimulateSingleIxUnifiedDualSign_BuildAndSubmitSingleIxUnifiedDualSign(config NetworkConfig) {
	client := NewMilonClient(config)

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	tokenAddress, _ := crypto.NewAddressFromPublicKey(tokenPk)

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	ownerAddress, _ := crypto.NewAddressFromPublicKey(ownerPk)

	payerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	payerPk := payerSk.Ed25519Public()
	payerAddress, _ := crypto.NewAddressFromPublicKey(payerPk)

	fmt.Printf("tokenAddress = %v \n", tokenAddress)
	fmt.Printf("ownerAddress = %v \n", ownerAddress)
	fmt.Printf("payerAddress = %v \n", payerAddress)

	fmt.Printf("\n\n ----------1.BuildAndSimulateSingleIxUnifiedDualSign---------------------- \n\n")

	simulateTransactionResult, err := client.BuildAndSimulateSingleIxUnifiedDualSign(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Multi Ix Token",
				"symbol":   "SMIX",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		*payerAddress,
		PubKeySignatureMode{PublicKey: *payerPk},
		*tokenAddress,
		PubKeySignatureMode{PublicKey: *tokenPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("simulateTransactionResult = %+v \n\n", simulateTransactionResult)
	fmt.Printf("simulateTransactionResult.BodySimulateReceipt = %+v \n\n", simulateTransactionResult.BodySimulateReceipt)
	Debug_SimulateTransactionResult(config, simulateTransactionResult.BodySimulateReceipt)

	fmt.Printf("\n\n ----------2.BuildAndSubmitSingleIxUnifiedDualSign---------------------- \n\n")

	submitTransactionResult, err := client.BuildAndSubmitSingleIxUnifiedDualSign(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Multi Ix Token",
				"symbol":   "SMIX",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		payerSk,
		*payerAddress,
		PubKeySignatureMode{PublicKey: *payerPk},
		tokenSk,
		*tokenAddress,
		PubKeySignatureMode{PublicKey: *tokenPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("submitTransactionResult = %+v \n\n", submitTransactionResult)

	fmt.Printf("\n\n ----------3.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------4.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)
}

func Client_Account_Create_BuildAndSimulateSingleIxUnifiedDualSign_BuildAndSubmitSingleIxUnifiedPayerOnlyGas(config NetworkConfig) {
	client := NewMilonClient(config)

	userSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	userPk, err := userSk.Secp256k1Public()
	if err != nil {
		panic(err)
	}
	userAddress, _ := crypto.NewAddressFromPublicKey(userPk)

	fmt.Printf("userPk = %v \n", userPk)
	fmt.Printf("userAddress = %v \n", userAddress)

	fmt.Printf("\n\n ----------1.BuildAndSimulateSingleIxUnifiedDualSign---------------------- \n\n")

	simulateTransactionResult, err := client.BuildAndSimulateSingleIxUnifiedPayerOnlyGas(
		"account",
		"Create",
		provider.Args{
			"owner_pk": userPk,
		},
		*userAddress,
		PubKeySignatureMode{PublicKey: *userPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("simulateTransactionResult = %+v \n\n", simulateTransactionResult)
	fmt.Printf("simulateTransactionResult.BodySimulateReceipt = %+v \n\n", simulateTransactionResult.BodySimulateReceipt)

	fmt.Printf("\n\n ----------2.BuildAndSubmitSingleIxUnifiedPayerOnlyGas----------------------- \n\n")

	submitTransactionResult, err := client.BuildAndSubmitSingleIxUnifiedPayerOnlyGas(
		"account",
		"Create",
		provider.Args{
			"owner_pk": userPk,
		},
		userSk,
		*userAddress,
		PubKeySignatureMode{PublicKey: *userPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("simulateTransactionResult = %+v \n\n", simulateTransactionResult)
	fmt.Printf("simulateTransactionResult.BodySimulateReceipt = %+v \n\n", simulateTransactionResult.BodySimulateReceipt)
	Debug_SimulateTransactionResult(config, simulateTransactionResult.BodySimulateReceipt)

	fmt.Printf("\n\n ----------3.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------4.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)

	fmt.Printf("\n\n ----------5.Client_GetAccount---------------------- \n\n")
	Client_GetAccount(config, userAddress.ToBase58())
}

func Client_Token_Create_BuildAndSimulateSingleIxSplit_BuildAndSubmitSingleIxSplit(config NetworkConfig) {
	client := NewMilonClient(config)

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	tokenAddress, _ := crypto.NewAddressFromPublicKey(tokenPk)

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	ownerAddress, _ := crypto.NewAddressFromPublicKey(ownerPk)

	fmt.Printf("tokenAddress = %v \n", tokenAddress)
	fmt.Printf("ownerAddress = %v \n", ownerAddress)

	fmt.Printf("\n\n ----------1.BuildAndSubmitSingleIxSplit---------------------- \n\n")

	simulateTransactionResult, err := client.BuildAndSimulateSingleIxSplit(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Multi Ix Token",
				"symbol":   "SMIX",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		*tokenAddress,
		PubKeySignatureMode{PublicKey: *tokenPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("simulateTransactionResult = %+v \n\n", simulateTransactionResult)
	fmt.Printf("simulateTransactionResult.BodySimulateReceipt = %+v \n\n", simulateTransactionResult.BodySimulateReceipt)
	Debug_SimulateTransactionResult(config, simulateTransactionResult.BodySimulateReceipt)

	fmt.Printf("\n\n ----------2.BuildAndSubmitSingleIxSplit---------------------- \n\n")

	submitTransactionResult, err := client.BuildAndSubmitSingleIxSplit(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Multi Ix Token",
				"symbol":   "SMIX",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		tokenSk,
		*tokenAddress,
		PubKeySignatureMode{PublicKey: *tokenPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("submitTransactionResult = %+v \n\n", submitTransactionResult)

	fmt.Printf("\n\n ----------3.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------4.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)
}

func Client_Demo_InitPool_BatchCredit_BuildAndSimulateMultiIxUnified_BuildAndSubmitMultiIxUnified(config NetworkConfig) {
	client := NewMilonClient(config)

	poolSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	poolPk := poolSk.Ed25519Public()
	poolAddress, _ := crypto.NewAddressFromPublicKey(poolPk)

	recipientSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	recipientPk := recipientSk.Ed25519Public()
	recipientAddress, _ := crypto.NewAddressFromPublicKey(recipientPk)

	fmt.Printf("poolAddress = %v \n", poolAddress)
	fmt.Printf("recipientAddress = %v \n", recipientAddress)

	// 1. 加载 IDL
	pd, err := client.GetPdByIDLAppName("demo")
	if err != nil {
		panic(err)
	}

	// 2. InitPool指令
	initPoolWire, err := pd.Encode("InitPool", provider.Args{
		"pool":  poolAddress,
		"label": "InitPool-label",
	})
	if err != nil {
		panic(err)
	}

	// 3. BatchCredit指令
	batchCreditWire, err := pd.Encode("BatchCredit", provider.Args{
		"pool":       poolAddress,
		"recipients": []crypto.Address{*recipientAddress},
		"amount":     123,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("\n\n ----------1.BuildAndSimulateMultiIxUnified---------------------- \n\n")

	simulateTransactionResult, err := client.BuildAndSimulateMultiIxUnified(
		[]api.PackedInstruction{
			initPoolWire,
			batchCreditWire,
		},
		*poolAddress,
		PubKeySignatureMode{PublicKey: *poolPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("simulateTransactionResult = %+v \n\n", simulateTransactionResult)
	fmt.Printf("simulateTransactionResult.BodySimulateReceipt = %+v \n\n", simulateTransactionResult.BodySimulateReceipt)

	Debug_SimulateTransactionResult(config, simulateTransactionResult.BodySimulateReceipt)

	fmt.Printf("\n\n ----------2.BuildAndSubmitMultiIxUnified---------------------- \n\n")

	submitTransactionResult, err := client.BuildAndSubmitMultiIxUnified(
		[]api.PackedInstruction{
			initPoolWire,
			batchCreditWire,
		},
		poolSk,
		*poolAddress,
		PubKeySignatureMode{PublicKey: *poolPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("submitTransactionResult = %+v \n\n", submitTransactionResult)

	fmt.Printf("\n\n ----------3.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------4.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)
}

func Client_Token_Create_Mint_Transfer_BuildAndSimulateMultiIxSplit_BuildAndSubmitMultiIxSplit_And_BuildAndViewSingleIx_And_BuildAndViewMultiIx_SameWires(config NetworkConfig) {
	client := NewMilonClient(config)

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	tokenAddress, _ := crypto.NewAddressFromPublicKey(tokenPk)

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	ownerAddress, _ := crypto.NewAddressFromPublicKey(ownerPk)

	payerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	payerPk := payerSk.Ed25519Public()
	payerAddress, _ := crypto.NewAddressFromPublicKey(payerPk)

	user1Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user1Pk := user1Sk.Ed25519Public()
	user1Address, _ := crypto.NewAddressFromPublicKey(user1Pk)

	user2Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user2Pk := user2Sk.Ed25519Public()
	user2Address, _ := crypto.NewAddressFromPublicKey(user2Pk)

	user3Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user3Pk := user3Sk.Ed25519Public()
	user3Address, _ := crypto.NewAddressFromPublicKey(user3Pk)

	fmt.Printf("tokenAddress = %v \n", tokenAddress)
	fmt.Printf("ownerAddress = %v \n", ownerAddress)
	fmt.Printf("payerAddress = %v \n", payerAddress)
	fmt.Printf("user1Address = %v \n", user1Address)
	fmt.Printf("user2Address = %v \n", user2Address)
	fmt.Printf("user3Address = %v \n", user3Address)

	wires := make([]api.PackedInstruction, 0)
	ownerSkList := make([]crypto.SecretKeyer, 0)
	ownerAddressList := make([]crypto.Address, 0)
	acSigModList := make([]AccountSignatureMode, 0)

	// 1. 加载 IDL
	pd, err := client.GetPdByIDLAppName("token")
	if err != nil {
		panic(err)
	}

	// 2. Create指令
	createWire, err := pd.Encode("Create", provider.Args{
		"token": tokenAddress,
		"owner": ownerAddress,
		"metadata": map[string]any{
			"name":     "SDK Multi Ix Token",
			"symbol":   "SMIX",
			"decimals": 6,
			"icon":     "https://milon.test/token.png",
		},
	})
	if err != nil {
		panic(err)
	}

	wires = append(wires, createWire)
	ownerSkList = append(ownerSkList, tokenSk)
	ownerAddressList = append(ownerAddressList, *tokenAddress)
	acSigModList = append(acSigModList, PubKeySignatureMode{PublicKey: *tokenPk})

	// 3. Mint指令
	mintWire, err := pd.Encode("Mint", provider.Args{
		"token":  tokenAddress,
		"to":     user1Address,
		"amount": 1000,
	})
	if err != nil {
		panic(err)
	}
	wires = append(wires, mintWire)
	ownerSkList = append(ownerSkList, ownerSk)
	ownerAddressList = append(ownerAddressList, *ownerAddress)
	acSigModList = append(acSigModList, PubKeySignatureMode{PublicKey: *ownerPk})

	// 4. transfer
	transferWire, err := pd.Encode("Transfer", provider.Args{
		"from":   user1Address,
		"token":  tokenAddress,
		"to":     user2Address,
		"amount": 300,
	})
	wires = append(wires, transferWire)
	ownerSkList = append(ownerSkList, user1Sk)
	ownerAddressList = append(ownerAddressList, *user1Address)
	acSigModList = append(acSigModList, PubKeySignatureMode{PublicKey: *user1Pk})

	fmt.Printf("\n\n ----------1.BuildAndSimulateMultiIxSplit---------------------- \n\n")

	simulateTransactionResult, err := client.BuildAndSimulateMultiIxSplit(
		wires,
		ownerAddressList,
		acSigModList,
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("simulateTransactionResult = %+v \n\n", simulateTransactionResult)
	fmt.Printf("simulateTransactionResult.BodySimulateReceipt = %+v \n\n", simulateTransactionResult.BodySimulateReceipt)
	Debug_SimulateTransactionResult(config, simulateTransactionResult.BodySimulateReceipt)

	fmt.Printf("\n\n ----------2.BuildAndSubmitMultiIxSplit---------------------- \n\n")

	submitTransactionResult, err := client.BuildAndSubmitMultiIxSplit(
		wires,
		ownerSkList,
		ownerAddressList,
		acSigModList,
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("submitTransactionResult = %+v \n\n", submitTransactionResult)

	fmt.Printf("\n\n ----------3.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------4.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)

	fmt.Printf("----------5. view(token.BalanceOf)----BuildAndViewSingleIx------------------- \n")

	viewTransactionResult1, err := client.BuildAndViewSingleIx(
		"token",
		"BalanceOf", provider.Args{
			"token":   tokenAddress,
			"account": user1Address,
		},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("viewTransactionResult1 : %+v \n", viewTransactionResult1)
	if failure, ok := viewTransactionResult1.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("user1的余额 rpc查询err : %+v \n\n", failure)
	} else {
		if balance, ok := viewTransactionResult1.BodyValues.(uint64); ok {
			fmt.Printf("user1的余额 : %+v \n\n", balance)
		} else {
			fmt.Printf("user1的余额查询返回未知类型 : %T\n\n", viewTransactionResult1.BodyValues)
		}
	}

	viewTransactionResult2, err := client.BuildAndViewSingleIx(
		"token",
		"BalanceOf", provider.Args{
			"token":   tokenAddress,
			"account": user2Address,
		},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("viewTransactionResult2 : %+v \n", viewTransactionResult2)
	if failure, ok := viewTransactionResult2.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("user2的余额 rpc查询err : %+v \n\n", failure)
	} else {
		if balance, ok := viewTransactionResult2.BodyValues.(uint64); ok {
			fmt.Printf("user2的余额 : %+v \n\n", balance)
		} else {
			fmt.Printf("user2的余额查询返回未知类型 : %T\n\n", viewTransactionResult2.BodyValues)
		}
	}

	viewTransactionResult3, err := client.BuildAndViewSingleIx(
		"token",
		"BalanceOf", provider.Args{
			"token":   tokenAddress,
			"account": user3Address,
		},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("viewTransactionResult3 : %+v \n", viewTransactionResult3)
	if failure, ok := viewTransactionResult3.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("user3的余额 rpc查询err : %+v \n\n", failure)
	} else {
		if balance, ok := viewTransactionResult3.BodyValues.(uint64); ok {
			fmt.Printf("user3的余额 : %+v \n\n", balance)
		} else {
			fmt.Printf("user3的余额查询返回未知类型 : %T\n\n", viewTransactionResult3.BodyValues)
		}
	}
	/*
		viewTransactionResult1 : &{HttpStatusCode:200 HttpRspBytes:[123 34 114 101 113 117 101 115 116 95 105 100 34 58 49 44 34 115 116 97 116 117 115 34 58 48 44 34 98 111 100 121 34 58 91 49 44 48 44 50 44 49 56 56 44 53 93 44 34 101 114 114 111 114 34 58 110 117 108 108 125] HttpRspBody:[1 0 2 188 5] BodyValues:[{Value:700}]}

		viewTransactionResult2 : &{HttpStatusCode:200 HttpRspBytes:[123 34 114 101 113 117 101 115 116 95 105 100 34 58 49 44 34 115 116 97 116 117 115 34 58 48 44 34 98 111 100 121 34 58 91 49 44 48 44 50 44 49 55 50 44 50 93 44 34 101 114 114 111 114 34 58 110 117 108 108 125] HttpRspBody:[1 0 2 172 2] BodyValues:[{Value:300}]}

		viewTransactionResult3 : &{HttpStatusCode:200 HttpRspBytes:[123 34 114 101 113 117 101 115 116 95 105 100 34 58 49 44 34 115 116 97 116 117 115 34 58 48 44 34 98 111 100 121 34 58 91 49 44 49 44 49 50 56 44 52 44 57 50 44 50 51 50 44 49 56 48 44 49 54 54 44 50 51 48 44 49 51 54 44 49 56 51 44 50 50 56 44 49 56 52 44 49 52 49 44 50 50 57 44 49 55 51 44 49 53 50 44 50 50 57 44 49 53 54 44 49 54 56 44 51 50 44 52 48 44 49 49 54 44 49 49 49 44 49 48 55 44 49 48 49 44 49 49 48 44 53 56 44 51 50 44 53 49 44 53 50 44 49 48 57 44 55 54 44 56 51 44 49 49 56 44 49 48 53 44 55 52 44 49 49 56 44 55 56 44 49 48 51 44 53 55 44 49 48 50 44 57 57 44 49 50 49 44 49 49 56 44 56 55 44 56 57 44 54 57 44 57 57 44 55 54 44 56 50 44 49 49 56 44 54 57 44 49 49 57 44 55 56 44 49 48 53 44 55 54 44 52 52 44 51 50 44 57 55 44 57 57 44 57 57 44 49 49 49 44 49 49 55 44 49 49 48 44 49 49 54 44 53 56 44 51 50 44 53 49 44 49 48 51 44 53 50 44 54 57 44 56 55 44 49 49 51 44 56 51 44 53 48 44 56 56 44 56 55 44 53 55 44 49 48 53 44 49 49 48 44 49 49 56 44 56 53 44 49 48 52 44 56 55 44 53 53 44 56 56 44 49 50 48 44 55 55 44 55 50 44 49 50 49 44 49 48 55 44 49 49 50 44 49 49 48 44 53 51 44 54 56 44 52 49 44 52 48 44 49 52 56 44 53 53 44 49 48 49 44 49 51 57 44 56 56 44 50 52 54 44 50 50 54 44 49 49 52 44 51 55 44 56 52 44 49 50 49 44 50 48 48 44 51 49 44 55 57 44 49 55 49 44 50 49 57 44 56 50 44 49 50 48 44 49 55 50 44 52 57 44 49 57 49 44 50 50 56 44 55 52 44 50 50 57 44 49 56 50 44 51 50 44 49 53 48 44 57 48 44 49 57 56 44 50 48 54 44 50 50 53 44 57 53 44 53 53 44 50 50 50 44 49 56 51 44 54 49 44 52 57 44 54 50 44 51 53 44 56 48 93 44 34 101 114 114 111 114 34 58 110 117 108 108 125] HttpRspBody:[1 1 128 4 92 232 180 166 230 136 183 228 184 141 229 173 152 229 156 168 32 40 116 111 107 101 110 58 32 51 52 109 76 83 118 105 74 118 78 103 57 102 99 121 118 87 89 69 99 76 82 118 69 119 78 105 76 44 32 97 99 99 111 117 110 116 58 32 51 103 52 69 87 113 83 50 88 87 57 105 110 118 85 104 87 55 88 120 77 72 121 107 112 110 53 68 41 40 148 55 101 139 88 246 226 114 37 84 121 200 31 79 171 219 82 120 172 49 191 228 74 229 182 32 150 90 198 206 225 95 55 222 183 61 49 62 35 80] BodySimulateReceipt:[{Value:0xc0002054a0}]}
	*/

	fmt.Printf("----------6. view(token.Metadata)----BuildAndViewSingleIx------------------- \n")

	viewTransactionResult4, err := client.BuildAndViewSingleIx(
		"token",
		"Metadata", provider.Args{
			"token": tokenAddress,
		},
		1,
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("viewTransactionResult4 : %+v \n\n", viewTransactionResult4)
	fmt.Printf("viewTransactionResult4.BodyValues : %#v \n", viewTransactionResult4.BodyValues)

	fmt.Printf("----------7. view(token.BalanceOf)----BuildAndViewMultiIx----DecodeViewDatas------------------- \n")

	user1BalanceWire, err := pd.Encode("BalanceOf", provider.Args{
		"token":   tokenAddress,
		"account": user1Address,
	})
	if err != nil {
		panic(err)
	}

	user2BalanceWire, err := pd.Encode("BalanceOf", provider.Args{
		"token":   tokenAddress,
		"account": user2Address,
	})
	if err != nil {
		panic(err)
	}

	user3BalanceWire, err := pd.Encode("BalanceOf", provider.Args{
		"token":   tokenAddress,
		"account": user3Address,
	})
	if err != nil {
		panic(err)
	}

	viewTransactionResult, err := client.BuildAndViewMultiIx([]api.PackedInstruction{
		user1BalanceWire,

		user3BalanceWire,
		user2BalanceWire,
	}, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("viewTransactionResult : %+v \n\n", viewTransactionResult)
	fmt.Printf("viewTransactionResult.HttpRspBody : %+v \n\n", viewTransactionResult.HttpRspBody)
	/*
		viewTransactionResult : &{HttpStatusCode:200 HttpRspBytes:[123 34 114 101 113 117 101 115 116 95 105 100 34 58 49 44 34 115 116 97 116 117 115 34 58 48 44 34 98 111 100 121 34 58 91 51 44 48 44 50 44 49 56 56 44 53 44 48 44 50 44 49 55 50 44 50 44 49 44 49 50 56 44 52 44 57 50 44 50 51 50 44 49 56 48 44 49 54 54 44 50 51 48 44 49 51 54 44 49 56 51 44 50 50 56 44 49 56 52 44 49 52 49 44 50 50 57 44 49 55 51 44 49 53 50 44 50 50 57 44 49 53 54 44 49 54 56 44 51 50 44 52 48 44 49 49 54 44 49 49 49 44 49 48 55 44 49 48 49 44 49 49 48 44 53 56 44 51 50 44 53 48 44 49 49 52 44 53 49 44 55 55 44 49 48 49 44 56 48 44 49 49 50 44 56 49 44 49 48 48 44 55 52 44 56 55 44 49 49 55 44 54 55 44 53 53 44 55 54 44 57 57 44 55 49 44 56 57 44 56 57 44 53 49 44 53 53 44 57 56 44 49 49 55 44 55 50 44 53 55 44 49 48 50 44 55 55 44 56 53 44 52 52 44 51 50 44 57 55 44 57 57 44 57 57 44 49 49 49 44 49 49 55 44 49 49 48 44 49 49 54 44 53 56 44 51 50 44 53 49 44 56 55 44 49 48 50 44 56 57 44 57 55 44 49 50 48 44 55 53 44 54 53 44 55 56 44 49 49 50 44 49 49 50 44 56 48 44 55 53 44 56 49 44 55 50 44 49 48 55 44 56 53 44 57 48 44 49 49 52 44 55 53 44 49 49 54 44 49 49 48 44 49 50 48 44 52 57 44 56 52 44 49 48 48 44 54 57 44 53 48 44 52 49 44 52 48 44 49 51 50 44 49 50 48 44 55 51 44 56 51 44 50 52 50 44 50 50 50 44 49 52 52 44 49 51 54 44 51 55 44 54 52 44 49 53 51 44 49 48 49 44 50 52 54 44 54 54 44 55 56 44 49 48 53 44 55 51 44 56 53 44 50 50 50 44 53 57 44 49 56 48 44 54 57 44 49 44 50 50 49 44 54 48 44 49 56 48 44 52 48 44 50 49 51 44 50 50 49 44 50 51 52 44 55 57 44 53 53 44 50 51 50 44 49 49 48 44 54 56 44 49 57 52 44 49 54 57 44 56 49 44 55 56 44 53 49 93 44 34 101 114 114 111 114 34 58 110 117 108 108 125] HttpRspBody:[3 0 2 188 5 0 2 172 2 1 128 4 92 232 180 166 230 136 183 228 184 141 229 173 152 229 156 168 32 40 116 111 107 101 110 58 32 50 114 51 77 101 80 112 81 100 74 87 117 67 55 76 99 71 89 89 51 55 98 117 72 57 102 77 85 44 32 97 99 99 111 117 110 116 58 32 51 87 102 89 97 120 75 65 78 112 112 80 75 81 72 107 85 90 114 75 116 110 120 49 84 100 69 50 41 40 132 120 73 83 242 222 144 136 37 64 153 101 246 66 78 105 73 85 222 59 180 69 1 221 60 180 40 213 221 234 79 55 232 110 68 194 169 81 78 51] BodySimulateReceipt:[]}
	*/

	// 解码所有指令的返回值
	decodeViewDataList, err := pd.DecodeViewDatas("BalanceOf", viewTransactionResult.HttpRspBody)
	if err != nil {
		panic(err)
	}
	fmt.Printf("decodeViewDataList : %+v \n\n", decodeViewDataList)

	for i, result := range decodeViewDataList {
		if failure, ok := result.Value.(*api.TxFailurePayload); ok {
			// 处理失败情况
			fmt.Printf("❌  指令 %d 失败 | err = %v \n", i, failure)
			fmt.Printf("    错误码 = %d\n", failure.Code)
			fmt.Printf("    错误消息 = %s\n", failure.Message)
			fmt.Printf("    附加数据 = %v\n\n", failure.Data)
		} else {
			// 处理成功情况
			balance, ok := result.Value.(uint64)
			if !ok {
				fmt.Printf("⚠️  指令 %d: 期望 uint64 但得到 %T\n", i, result.Value)
				continue
			}

			fmt.Printf("✅  指令 %d: 余额:%d \n\n", i, balance)
		}
	}
}

func Client_Account_Create_And_BuildAndViewMultiIx_differentWires(config NetworkConfig) {
	client := NewMilonClient(config)

	fmt.Printf("\n\n ----------1.BuildAndSubmitSingleIxSplit----------------------- \n\n")

	userSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	userPk := userSk.Ed25519Public()
	userAddress, _ := crypto.NewAddressFromPublicKey(userPk)

	fmt.Printf("userPk = %v \n", userPk)
	fmt.Printf("userAddress = %v \n\n", userAddress)

	submitTransactionResult, err := client.BuildAndSubmitSingleIxUnifiedPayerSignAll(
		"account",
		"Create",
		provider.Args{
			"owner_pk": userPk,
		},
		userSk,
		*userAddress,
		PubKeySignatureMode{PublicKey: *userPk},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\n\n ----------2.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------3.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)

	fmt.Printf("\n\n ----------4.Client_GetAccount---------------------- \n\n")
	Client_GetAccount(config, userAddress.ToBase58())

	fmt.Printf("\n\n ----------5.BuildAndViewMultiIx---------------------- \n\n")

	// 1. 加载 IDL
	pd, err := client.GetPdByIDLAppName("account")
	if err != nil {
		panic(err)
	}

	wire1, err := pd.Encode(
		"GetAccount",
		provider.Args{
			"owner": userAddress,
		},
	)
	if err != nil {
		panic(err)
	}

	wire2, err := pd.Encode(
		"ListSigners",
		provider.Args{
			"owner": userAddress,
		},
	)
	if err != nil {
		panic(err)
	}

	viewTransactionResult, err := client.BuildAndViewMultiIx([]api.PackedInstruction{wire1, wire2}, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("viewTransactionResult : %+v \n\n", viewTransactionResult)

	fmt.Printf("viewTransactionResult : %+v \n\n", viewTransactionResult)

	fmt.Printf("\n========== 由于两个指令返回类型不同（GetAccount 和 ListSigners），需要手动逐层解析 ==========\n")

	// BuildAndViewMultiIx 返回的是 Vec<Result<Vec<u8>, TxFailurePayload>>
	// 由于两个指令返回类型不同（GetAccount 和 ListSigners），需要手动逐层解析

	// 原始数据结构：
	// [结果数量(varint)] + [Result_0] + [Result_1]
	// 每个 Result:
	//   - Ok: [变体索引=0(varint)] + [Vec<u8>长度(varint)] + [实际数据...]
	//   - Err: [变体索引=1(varint)] + [TxFailurePayload...]

	offset := 0

	// 1. 读取结果数量
	resultCount, err := provider.DecodeViewVarUint(viewTransactionResult.HttpRspBody, &offset)
	if err != nil {
		panic(fmt.Sprintf("读取结果数量失败: %v", err))
	}
	fmt.Printf("📊 结果总数: %d\n\n", resultCount)

	// 2. 逐个处理每个结果
	for i := uint64(0); i < resultCount; i++ {
		fmt.Printf("=== 结果 [%d] ===\n", i)

		// 读取变体索引 (0=Ok, 1=Err)
		variantIndex, err := provider.DecodeViewVarUint(viewTransactionResult.HttpRspBody, &offset)
		if err != nil {
			panic(fmt.Sprintf("读取结果[%d]的变体索引失败: %v", i, err))
		}

		if variantIndex == 1 {
			// Err 情况：解析 TxFailurePayload
			fmt.Printf("❌ 指令 %d 执行失败\n", i)

			// 读取 TxFailurePayload: {code: u16, message: String, data: Vec<u8>}
			codeLow, _ := provider.DecodeViewVarUint(viewTransactionResult.HttpRspBody, &offset)
			codeHigh, _ := provider.DecodeViewVarUint(viewTransactionResult.HttpRspBody, &offset)
			code := codeLow | (codeHigh << 8)

			messageLen, _ := provider.DecodeViewVarUint(viewTransactionResult.HttpRspBody, &offset)
			messageBytes := make([]byte, messageLen)
			copy(messageBytes, viewTransactionResult.HttpRspBody[offset:offset+int(messageLen)])
			offset += int(messageLen)
			message := string(messageBytes)

			dataLen, _ := provider.DecodeViewVarUint(viewTransactionResult.HttpRspBody, &offset)
			offset += int(dataLen)

			fmt.Printf("   错误码: %d\n", code)
			fmt.Printf("   错误消息: %s\n", message)
			fmt.Printf("   附加数据: %d 字节\n\n", dataLen)

			continue
		}

		// Ok 情况：读取 Vec<u8> 的长度和数据
		okDataLen, err := provider.DecodeViewVarUint(viewTransactionResult.HttpRspBody, &offset)
		if err != nil {
			panic(fmt.Sprintf("读取结果[%d]的数据长度失败: %v", i, err))
		}

		okData := make([]byte, okDataLen)
		copy(okData, viewTransactionResult.HttpRspBody[offset:offset+int(okDataLen)])
		offset += int(okDataLen)

		fmt.Printf("✅ 指令 %d 执行成功\n", i)
		fmt.Printf("   数据类型: %s\n", map[uint64]string{0: "GetAccount", 1: "ListSigners"}[i])
		fmt.Printf("   数据长度: %d 字节\n", okDataLen)
		fmt.Printf("   原始数据(hex): %x\n", okData)

		// 根据指令索引使用不同的类型解码
		if i == 0 {
			// 第1个指令：GetAccount -> Account 结构体
			// 直接使用 postcard 反序列化，不需要 app_id 和 discriminator
			accountValue, err := postcard.DeserializePostcard(okData, func(d *postcard.Deserializer) (map[string]any, error) {
				result := make(map[string]any)

				// Account 结构: bitmap(u64), weight(u8), threshold(u8)
				bitmap, err := d.DeserializeU64()
				if err != nil {
					return nil, fmt.Errorf("deserialize bitmap: %w", err)
				}
				result["bitmap"] = bitmap

				weight, err := d.DeserializeU8()
				if err != nil {
					return nil, fmt.Errorf("deserialize weight: %w", err)
				}
				result["weight"] = weight

				threshold, err := d.DeserializeU8()
				if err != nil {
					return nil, fmt.Errorf("deserialize threshold: %w", err)
				}
				result["threshold"] = threshold

				return result, nil
			}, false)

			if err != nil {
				fmt.Printf("   ⚠️  解码 GetAccount 失败: %v\n\n", err)
			} else {
				fmt.Printf("   📋 解码后的 Account:\n")
				if bitmap, exists := accountValue["bitmap"]; exists {
					fmt.Printf("      - bitmap: %v\n", bitmap)
				}
				if weight, exists := accountValue["weight"]; exists {
					fmt.Printf("      - weight: %v\n", weight)
				}
				if threshold, exists := accountValue["threshold"]; exists {
					fmt.Printf("      - threshold: %v\n", threshold)
				}
				fmt.Printf("\n")
			}
		} else if i == 1 {
			// 第2个指令：ListSigners -> tuple<Account,vec<tuple<PublicKey,u8,u8>>>
			// 直接手动解析 postcard 数据
			fmt.Printf("   📋 解码后的 ListSigners:\n")

			listOffset := 0

			// 首先解析外层 tuple 的第一个元素：Account
			// Account 结构: bitmap(varint), weight(varint), threshold(varint)
			bitmap, _ := provider.DecodeViewVarUint(okData, &listOffset)
			weight, _ := provider.DecodeViewVarUint(okData, &listOffset)
			threshold, _ := provider.DecodeViewVarUint(okData, &listOffset)

			fmt.Printf("      [0] Account:\n")
			fmt.Printf("          - bitmap: %v\n", bitmap)
			fmt.Printf("          - weight: %v\n", weight)
			fmt.Printf("          - threshold: %v\n", threshold)

			// 解析外层 tuple 的第二个元素：vec<tuple<PublicKey,u8,u8>>
			signerCount, _ := provider.DecodeViewVarUint(okData, &listOffset)
			fmt.Printf("      [1] Signers 列表 (共 %d 个):\n", signerCount)

			for j := uint64(0); j < signerCount; j++ {
				fmt.Printf("          [%d] Signer:\n", j)

				// PublicKey 是变长编码，先读取长度标识
				// PublicKey 在 postcard 中是 bytes 类型，格式: len(varint) + data
				pubKeyLen, _ := provider.DecodeViewVarUint(okData, &listOffset)
				pubKeyBytes := make([]byte, pubKeyLen)
				copy(pubKeyBytes, okData[listOffset:listOffset+int(pubKeyLen)])
				pubKey := &crypto.PublicKey{Bytes: pubKeyBytes}
				listOffset += int(pubKeyLen)

				// weight (u8) - varint 编码
				signerWeight, _ := provider.DecodeViewVarUint(okData, &listOffset)

				// policy (u8) - varint 编码
				signerPolicy, _ := provider.DecodeViewVarUint(okData, &listOffset)

				fmt.Printf("              - PublicKey: %s\n", pubKey.ToBase58())
				fmt.Printf("              - weight: %v\n", signerWeight)
				fmt.Printf("              - policy: %v\n", signerPolicy)
			}

			fmt.Printf("\n")
		}
	}
}

func Client_Account_Create_4_Crypto(config NetworkConfig) {
	client := NewMilonClient(config)

	fmt.Printf("\n\n ----------1.1.BuildAndSubmitSingleIxSplit----------------------- \n\n")

	user1Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user1Pk := user1Sk.Ed25519Public()
	user1Address, _ := crypto.NewAddressFromPublicKey(user1Pk)

	fmt.Printf("user1Pk = %v \n", user1Pk)
	fmt.Printf("user1Address = %v \\n", user1Address)

	submitTransactionResult, err := client.BuildAndSubmitSingleIxUnifiedPayerSignAll(
		"account",
		"Create",
		provider.Args{
			"owner_pk": user1Pk,
		},
		user1Sk,
		*user1Address,
		PubKeySignatureMode{PublicKey: *user1Pk},
		1,
	)
	if err != nil {
		fmt.Printf("submitTransactionResult = %+v \n\n", submitTransactionResult)
		panic(err)
	}

	fmt.Printf("\n\n ----------1.2.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------1.4.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)

	fmt.Printf("\n\n ----------1.5.Client_GetAccount---------------------- \n\n")
	Client_GetAccount(config, user1Address.ToBase58())

	fmt.Printf("\n\n ----------2.1.BuildAndSubmitSingleIxUnifiedPayerOnlyGas----------------------- \n\n")

	user2Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user2Pk, err := user2Sk.Secp256k1Public()
	if err != nil {
		panic(err)
	}
	user2Address, _ := crypto.NewAddressFromPublicKey(user2Pk)

	fmt.Printf("user2Pk = %v \n", user2Pk)
	fmt.Printf("user2Address = %v \n\n", user2Address)

	submitTransactionResult, err = client.BuildAndSubmitSingleIxUnifiedPayerOnlyGas(
		"account",
		"Create",
		provider.Args{
			"owner_pk": user2Pk,
		},
		user2Sk,
		*user2Address,
		PubKeySignatureMode{PublicKey: *user2Pk},
		1,
	)
	if err != nil {
		fmt.Printf("submitTransactionResult = %+v \n\n", submitTransactionResult)
		panic(err)
	}

	fmt.Printf("\n\n ----------2.2.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err = client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------2.3.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)

	fmt.Printf("\n\n ----------2.4.Client_GetAccount---------------------- \n\n")
	Client_GetAccount(config, user2Address.ToBase58())

	fmt.Printf("\n\n ----------3.1.BuildAndSubmitSingleIxSplit----------------------- \n\n")

	user3Sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	user3Pk := user3Sk.BLS12381Public()
	user3Address, _ := crypto.NewAddressFromPublicKey(user3Pk)

	fmt.Printf("user3Pk = %v \n", user3Pk)
	fmt.Printf("user3Address = %v \n\n", user3Address)

	submitTransactionResult, err = client.BuildAndSubmitSingleIxSplit(
		"account",
		"Create",
		provider.Args{
			"owner_pk": user3Pk,
		},
		user3Sk,
		*user3Address,
		PubKeySignatureMode{PublicKey: *user3Pk},
		1,
	)
	if err != nil {
		fmt.Printf("submitTransactionResult = %+v \n\n", submitTransactionResult)
		panic(err)
	}

	fmt.Printf("\n\n ----------3.2.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err = client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------3.3.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)

	fmt.Printf("\n\n ----------3.3.Client_GetAccount---------------------- \n\n")
	Client_GetAccount(config, user3Address.ToBase58())

	fmt.Printf("\n\n ----------4.1.BuildAndSubmitSingleIxSplit----------------------- \n\n")

	user4Sk, user4Pk, err := crypto.NewFnDsa512SecretKey()
	if err != nil {
		panic(err)
	}
	user4Fk := crypto.AsFnDsa512SecretKey(user4Sk)
	user4Address, err := crypto.NewAddressFromPublicKey(user4Pk)
	if err != nil {
		panic(err)
	}

	fmt.Printf("user4Pk = %v \n", user4Pk)
	fmt.Printf("user4Address = %v \n\n", user4Address)

	submitTransactionResult, err = client.BuildAndSubmitSingleIxSplit(
		"account",
		"Create",
		provider.Args{
			"owner_pk": user4Pk,
		},
		user4Fk,
		*user4Address,
		PubKeySignatureMode{PublicKey: *user4Pk},
		1,
	)
	if err != nil {
		fmt.Printf("submitTransactionResult = %+v \n\n", submitTransactionResult)
		panic(err)
	}

	fmt.Printf("\n\n ----------4.2.WaitForTransaction---------------------- \n\n")
	getTxByHashResult, err = client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(err)
	}
	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)

	fmt.Printf("\n\n ----------4.3.Client_EventsByTxHash---------------------- \n\n")
	Client_EventsByTxHash(config, submitTransactionResult.BodyTxHash, nil)

	fmt.Printf("\n\n ----------4.3.Client_GetAccount---------------------- \n\n")
	Client_GetAccount(config, user4Address.ToBase58())
}

func Client_GetTxByHash(config NetworkConfig, txHash string) {
	client := NewMilonClient(config)

	getTxByHashResult, err := client.GetTxByHash(txHash, 1)
	if err != nil {
		panic(err)
	}

	Debug_GetTxByHashResult_GetResource_GetAccessValue(config, getTxByHashResult.BodyTxHistory)
}

func Client_GetAccount(config NetworkConfig, addressBase58 string) {
	client := NewMilonClient(config)

	getAccountResult, err := client.GetAccount(addressBase58, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("getAccountResult : %+v \n\n", getAccountResult)
	fmt.Printf("getAccountResult.BodyAccountView : %+v \n\n", getAccountResult.BodyAccountView)
}

func Client_EventsByTxHash(config NetworkConfig, txHash string, typeTagFilter *uint64) {
	client := NewMilonClient(config)

	eventsByTxHashResult, err := client.EventsByTxHash(txHash, typeTagFilter, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("eventsByTxHashResult : %+v \n\n", eventsByTxHashResult)
	fmt.Printf("eventsByTxHashResult.BodyEventsByTxHash : %+v \n\n", eventsByTxHashResult.BodyEventsByTxHash)

	// 使用 IDLManager 自动检测并解码指令
	idlManager, err := provider.NewIDLManager(client.RpcClient.GetAllPd())
	if err != nil {
		panic(err)
	}

	for i, eventEntry := range eventsByTxHashResult.BodyEventsByTxHash.Events {
		fmt.Printf("Events(事件) index=[%d]:\n", i)
		fmt.Printf("\t BlockHeight: %d\n", eventEntry.BlockHeight)
		fmt.Printf("\t TxHash: %v\n", eventEntry.TxHash)
		fmt.Printf("\t TxIndex : %d\n", eventEntry.TxIndex)

		fmt.Printf("\t EventIndex: %d\n", eventEntry.EventIndex)
		fmt.Printf("\t Data (hex): %x\n", eventEntry.Data)

		// 使用 IDLManager 通过 typeTag 解码事件数据
		decodedEvent, err := idlManager.DecodeEventDataByTag(eventEntry.Data.TypeTag, eventEntry.Data.Value)
		if err != nil {
			panic(err)
		}
		fmt.Printf("\t Data 反序列化 = %+v \n\n", decodedEvent)
	}
}

func Client_ListResourcePath_GetResourcePathByHash(config NetworkConfig) {
	client := NewMilonClient(config)

	listResourcePathResult, err := client.ListResourcePath(1)
	if err != nil {
		panic(err)
	}

	fmt.Printf("listResourcePathResult : %+v \n", listResourcePathResult)

	bodyListResourcePathLen := len(listResourcePathResult.BodyListResourcePaths)

	for i, value := range listResourcePathResult.BodyListResourcePaths {
		fmt.Printf("Resouce(资源) index=[%d]:\n", i)
		fmt.Printf("\t RsHash: %d\n", value.RsHash)
		fmt.Printf("\t Path: %v\n", value.Path)

		if i > bodyListResourcePathLen-3 {
			getResourcePathByHashResult, err := client.GetResourcePathByHash(value.RsHash, 1)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\t path : %v \n", string(getResourcePathByHashResult.HttpRspBody))
		}
	}
}

func demo1() {
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())

	//sk := &crypto.SecretKey{}
	//err := sk.FromStringRelaxed("0x70119711fcfce14858c4eee22c6aa6c626575eeee203e0887edc4f218480ce5a")
	//if err != nil {
	//	panic(err)
	//}

	// 从公钥派生地址
	addr, err := crypto.NewAddressFromPublicKey(sk.Ed25519Public())
	if err != nil {
		panic(err)
	}
	fmt.Printf("32字节的私钥: hex %x \n", sk.AsBytes())
	fmt.Printf("32字节的私钥 地址: base58 %v \n\n", addr)

	random32 := make([]byte, 32)
	_, err = rand.Read(random32[:])
	if err != nil {
		panic(err)
	}

	random42 := make([]byte, 42)
	_, err = rand.Read(random42[:])
	if err != nil {
		panic(err)
	}

	fmt.Printf("32字节的内容: hex %x \n", random32)
	fmt.Printf("42字节的内容: hex %x \n\n", random42)

	secp256k1Pk, err := sk.Secp256k1Public()
	if err != nil {
		panic(err)
	}
	fmt.Printf("secp256k1 公钥 base58: %v \n", secp256k1Pk.ToBase58())
	signature, err := sk.SignSecp256k1(random32)
	if err != nil {
		panic(err)
	}
	fmt.Printf("secp256k1 公钥 对 32字节的内容 签名后的信息 base58: %v \n", signature.ToBase58())
	if signature.Verify(random32, secp256k1Pk) != nil {
		panic(1)
	}
	signature, err = sk.SignSecp256k1(random42)
	if err != nil {
		panic(err)
	}
	fmt.Printf("secp256k1 公钥 对 42字节的内容 签名后的信息 base58: %v \n\n", signature.ToBase58())
	if signature.Verify(random42, secp256k1Pk) != nil {
		panic(11)
	}

	ed25519Pk := sk.Ed25519Public()
	fmt.Printf("ed25519 公钥 base58: %v \n", ed25519Pk.ToBase58())
	signature = sk.SignEd25519(random32)
	fmt.Printf("ed25519 公钥 对 32字节的内容 签名后的信息 base58: %v \n", signature.ToBase58())
	if signature.Verify(random32, ed25519Pk) != nil {
		panic(2)
	}
	signature = sk.SignEd25519(random42)
	fmt.Printf("ed25519 公钥 对 42字节的内容 签名后的信息 base58: %v \n\n", signature.ToBase58())
	if signature.Verify(random42, ed25519Pk) != nil {
		panic(22)
	}

	bls12381Pk := sk.BLS12381Public()
	fmt.Printf("bls12381 公钥 base58: %v \n", bls12381Pk.ToBase58())
	signature = sk.SignBLS12381(random32)

	fmt.Printf("bls12381 公钥 对 32字节的内容 签名后的信息 base58: %v \n", signature.ToBase58())
	if signature.Verify(random32, bls12381Pk) != nil {
		panic(3)
	}
	signature = sk.SignBLS12381(random42)
	fmt.Printf("bls12381 公钥 对 42字节的内容 签名后的信息 base58: %v \n", signature.ToBase58())
	if signature.Verify(random42, bls12381Pk) != nil {
		panic(33)
	}
}

func Debug_GetTxByHashResult_GetResource_GetAccessValue(config NetworkConfig, txHistory *api.TxHistory) {
	client := NewMilonClient(config)

	fmt.Printf("txHistory: %+v \n\n", txHistory)

	// 使用 IDLManager 自动检测并解码指令
	idlManager, err := provider.NewIDLManager(client.RpcClient.GetAllPd())
	if err != nil {
		panic(err)
	}

	// Instructions（指令记录）
	fmt.Printf("\nInstructions (%d):\n", len(txHistory.Instructions))
	for i, instruction := range txHistory.Instructions {
		fmt.Printf("\t [%d] instruction: \n", i)

		decodedInstruction, err := idlManager.DecodeInstruction(instruction)
		if err != nil {
			panic(err)
		}

		fmt.Printf("\t\t decodedInstruction = %#v \n", decodedInstruction)
		/*
			map[string]interface {}{"app_name":"token", "args":map[string]interface {}{"metadata":map[string]interface {}{"decimals":0x6, "icon":"https://milon.test/token.png", "name":"SDK Multi Ix Token", "symbol":"SMIX"}, "owner":(*crypto.Address)(0xc0000d6618), "token":(*crypto.Address)(0xc0000d65e8)}, "discriminator":0x0, "method_name":"Create"}

			map[string]interface{}{
				"app_name": "token",  // string 类型，值为 "token"
				"args": map[string]interface{}{
					"metadata": map[string]interface{}{
						"decimals": 0x6,                           // int，值为 6（十六进制）
						"icon":     "https://milon.test/token.png", // string
						"name":     "SDK Multi Ix Token",          // string
						"symbol":   "SMIX",                        // string
					},
					"owner": (*crypto.Address)(0xc0000d6618), // 指向 Address 的指针
					"token": (*crypto.Address)(0xc0000d65e8), // 指向 Address 的指针
				},
				"discriminator": 0x0,  // int，值为 0
				"method_name":   "Create", // string
			}
		*/

		fmt.Printf("\t\t FormatDecodedInstruction = %+v \n\n", idlManager.FormatDecodedInstruction(decodedInstruction))
	}

	// Access Records（资源访问记录）
	fmt.Printf("\nAccess Records (%d):\n", len(txHistory.Receipt.Access))
	for i, record := range txHistory.Receipt.Access {
		fmt.Printf("\t [%d] ResourceID: %x\n", i, record.ResourceID)

		// FirstSnapshot
		if record.FirstSnapshot != nil {
			fmt.Printf("\t\t FirstSnapshot: \n")

			switch record.FirstSnapshot.Variant {
			case 0: // Inline
				fmt.Printf("\t\t\t Inline(type_tag=%d, data_len=%d)\n", record.FirstSnapshot.TypeTag, len(record.FirstSnapshot.InlineData))
				fmt.Printf("\t\t\t Data: %v\n", record.FirstSnapshot.InlineData)

				var idlType *provider.IDLType
				var pd *provider.Provider

				for _, tmpProvider := range client.GetAllPd() {
					tmpIDLType, ok := tmpProvider.GetIDLTypeByTypeTag(record.FirstSnapshot.TypeTag)
					if ok {
						idlType = tmpIDLType
						pd = tmpProvider

						fmt.Printf("\t\t\t idlType %+v \n", idlType)
						break
					}
				}

				valueDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, record.FirstSnapshot.InlineData)
				if err != nil {
					panic(err)
				}
				fmt.Printf("\t\t\t Value Decoded (%s): %+v\n", idlType.Name, valueDecoded)

				getResourceResult, err := client.GetResource(record.ResourceID, 1)
				if err != nil {
					fmt.Printf("\t\t\t client.GetResource error: %+v \n", err)
				} else {
					valueDecoded, err = pd.DecodeDataByIDLTypeName(idlType.Name, getResourceResult.BodyGetResource.Data.Value)
					if err != nil {
						panic(err)
					}
					fmt.Printf("\t\t\t client.GetResource.BodyGetResource: %+v\n", getResourceResult.BodyGetResource)
					fmt.Printf("\t\t\t client.GetResource.BodyGetResource.Data.Value Decoded (%v): %v\n\n", idlType.Name, valueDecoded)
				}

			case 1: // External
				fmt.Printf("\t\t\t External(BlobHash=%x)\n", record.FirstSnapshot.ExternalHash)

				getAccessValueResult, err := client.RpcClient.GetAccessValue([]api.BlobHash{record.FirstSnapshot.ExternalHash}, 1)
				if err != nil {
					panic(err)
				}

				fmt.Printf("\t\t\t client.rpcClientV1.GetAccessValue: %+v \n\n", getAccessValueResult)
				for i2, value := range getAccessValueResult.BodyGetAccessValues {
					fmt.Printf("\t\t\t\t [%d] Value: %+v\n", i2, value.Data)
				}
			default:
				panic(fmt.Sprintf("Unknown(variant=%d)", record.FirstSnapshot.Variant))
			}
		} else {
			fmt.Printf("\t\t FirstSnapshot: None\n")
		}

		// LastWritten
		fmt.Printf("\t\t LastWritten: \n")
		switch record.LastWritten.Variant {
		case 0: // Inline
			fmt.Printf("\t\t\t Inline(type_tag=%d, data_len=%d)\n", record.LastWritten.TypeTag, len(record.LastWritten.InlineData))
			fmt.Printf("\t\t\t Data: %v\n", record.LastWritten.InlineData)

			var idlType *provider.IDLType
			var pd *provider.Provider

			for _, tmpProvider := range client.GetAllPd() {
				tmpIDLType, ok := tmpProvider.GetIDLTypeByTypeTag(record.LastWritten.TypeTag)
				if ok {
					idlType = tmpIDLType
					pd = tmpProvider

					fmt.Printf("\t\t\t idlType %+v \n", idlType)
					break
				}
			}

			valueDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, record.LastWritten.InlineData)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\t\t\t Value Decoded (%v): %v\n\n", idlType.Name, valueDecoded)

			getResourceResult, err := client.GetResource(record.ResourceID, 1)
			if err != nil {
				fmt.Printf("\t\t\t client.GetResource error: %+v \n", err)
			} else {
				valueDecoded, err = pd.DecodeDataByIDLTypeName(idlType.Name, getResourceResult.BodyGetResource.Data.Value)
				if err != nil {
					panic(err)
				}
				fmt.Printf("\t\t\t client.GetResource.BodyGetResource: %+v\n", getResourceResult.BodyGetResource)
				fmt.Printf("\t\t\t getResourceResult.BodyGetResource.Data.Value Decoded (%v): %v\n\n", idlType.Name, valueDecoded)
			}

		case 1: // External
			fmt.Printf("\t\t\t External(BlobHash=%v)\n", record.LastWritten.ExternalHash)

			getAccessValueResult, err := client.RpcClient.GetAccessValue([]api.BlobHash{record.LastWritten.ExternalHash}, 1)
			if err != nil {
				panic(err)
			}

			fmt.Printf("\t\t\t client.rpcClientV1.GetAccessValue: %+v \n\n", getAccessValueResult)
			for i2, value := range getAccessValueResult.BodyGetAccessValues {
				fmt.Printf("\t\t\t\t [%d] BlobHash: %+v\n", i2, value.BlobHash)
				fmt.Printf("\t\t\t\t [%d] Data: %+v\n", i2, value.Data)

				if value.Data != nil {
					for _, pd := range client.GetAllPd() {
						idlType, ok := pd.GetIDLTypeByTypeTag(value.Data.TypeTag)
						if ok {
							decodedValue, err := pd.DecodeDataByIDLTypeName(idlType.Name, value.Data.Value)
							if err != nil {
								panic(err)
							}

							fmt.Printf("\t\t\t\t [%d] Value Decoded (%s): %+v\n", i2, idlType.Name, decodedValue)
							break
						}
					}
				}

			}
		default:
			panic(fmt.Sprintf("Unknown(variant=%d)", record.LastWritten.Variant))
		}
	}

	// Events（事件列表）
	fmt.Printf("\nEvents (%d):\n", len(txHistory.Receipt.Events))
	for i, event := range txHistory.Receipt.Events {
		fmt.Printf("\t [%d] TypeTag: %d \n", i, event.TypeTag)
		fmt.Printf("\t\t ValueLen: %d bytes\n", len(event.Value))
		fmt.Printf("\t\t Value: %x\n", event.Value)

		decodedEvent, err := idlManager.DecodeEventDataByTag(event.TypeTag, event.Value)
		if err != nil {
			panic(err)
		}

		fmt.Printf("\t\t decodedEvent: %+v\n", decodedEvent)

		fmt.Printf("\t\t FormatDecodedEvent: %+v \n\n", idlManager.FormatDecodedEvent(decodedEvent))
	}
}

func Debug_SimulateTransactionResult(config NetworkConfig, simulateReceipt *api.SimulateReceipt) {
	client := NewMilonClient(config)

	// 使用 IDLManager 自动检测并解码指令
	idlManager, err := provider.NewIDLManager(client.RpcClient.GetAllPd())
	if err != nil {
		panic(err)
	}

	// 基本信息
	fmt.Printf("TxID:   %x\n", simulateReceipt.TxID)
	fmt.Printf("TxHash: %x\n", simulateReceipt.TxHash)
	fmt.Printf("State:  %d", simulateReceipt.State)
	switch simulateReceipt.State {
	case 0:
		fmt.Printf("(Pending) \n")
	case 1:
		fmt.Printf("(Success) \n")
	case 2:
		fmt.Printf("(Failed) \n")
	default:
		panic("(Unknown)")
	}

	// Access Records（资源访问记录）
	fmt.Printf("\nAccess Records (%d):\n", len(simulateReceipt.Access))
	for i, record := range simulateReceipt.Access {
		fmt.Printf("\t [%d] ResourceID: %x\n", i, record.ResourceID)

		// FirstSnapshot
		if record.FirstSnapshot != nil {
			fmt.Printf("\t\t FirstSnapshot: \n")

			switch record.FirstSnapshot.Variant {
			case 0: // Inline
				fmt.Printf("\t\t\t Inline(type_tag=%d, data_len=%d)\n", record.FirstSnapshot.TypeTag, len(record.FirstSnapshot.InlineData))
				fmt.Printf("\t\t\t Data: %v\n", record.FirstSnapshot.InlineData)

				for _, pd := range client.GetAllPd() {
					idlType, ok := pd.GetIDLTypeByTypeTag(record.FirstSnapshot.TypeTag)
					if ok {
						fmt.Printf("\t\t\t idlType %+v \n", idlType)

						beforeDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, record.FirstSnapshot.InlineData)
						if err != nil {
							panic(err)
						}
						fmt.Printf("\t\t\t Value Decoded (%s): %+v\n", idlType.Name, beforeDecoded)
						break
					}
				}

			case 1: // External
				fmt.Printf("\t\t\t External(BlobHash=%x)\n", record.FirstSnapshot.ExternalHash)
			default:
				panic(fmt.Sprintf("Unknown(variant=%d)", record.FirstSnapshot.Variant))
			}
		} else {
			fmt.Printf("\t\t FirstSnapshot: None\n")
		}

		// LastWritten
		fmt.Printf("\t\t LastWritten: \n")
		switch record.LastWritten.Variant {
		case 0: // Inline
			fmt.Printf("\t\t\t Inline(type_tag=%d, data_len=%d)\n", record.LastWritten.TypeTag, len(record.LastWritten.InlineData))
			fmt.Printf("\t\t\t Data: %v\n", record.LastWritten.InlineData)

			for _, pd := range client.GetAllPd() {
				idlType, ok := pd.GetIDLTypeByTypeTag(record.LastWritten.TypeTag)
				if ok {
					fmt.Printf("\t\t\t idlType %+v \n", idlType)

					beforeDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, record.LastWritten.InlineData)
					if err != nil {
						panic(err)
					}
					fmt.Printf("\t\t\t Value Decoded (%s): %+v\n", idlType.Name, beforeDecoded)
					break
				}
			}

		case 1: // External
			fmt.Printf("\t\t\t External(BlobHash=%v)\n", record.LastWritten.ExternalHash)
		default:
			panic(fmt.Sprintf("Unknown(variant=%d)", record.LastWritten.Variant))
		}
	}

	// Events（事件列表）
	fmt.Printf("\nEvents (%d):\n", len(simulateReceipt.Events))
	for i, event := range simulateReceipt.Events {
		fmt.Printf("\t [%d] TypeTag: %d \n", i, event.TypeTag)
		fmt.Printf("\t\t DataLen: %d bytes\n", len(event.Value))
		fmt.Printf("\t\t Data: %x\n", event.Value)

		decodedEvent, err := idlManager.DecodeEventDataByTag(event.TypeTag, event.Value)
		if err != nil {
			panic(err)
		}

		fmt.Printf("\t\t decodedEvent: %v\n", decodedEvent)
	}

	// 错误信息
	if simulateReceipt.Error != nil {
		fmt.Printf("\n❌ Error:\n")
		fmt.Printf("\t Code:    %d\n", simulateReceipt.Error.Code)
		fmt.Printf("\t Message: %s\n", simulateReceipt.Error.Message)
		fmt.Printf("\t Data:    %x\n", simulateReceipt.Error.Data)
	}
}
