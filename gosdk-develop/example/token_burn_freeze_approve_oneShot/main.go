package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/helper"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func example(networkConfig milon.NetworkConfig) {
	client := milon.NewMilonClient(networkConfig)

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	tokenAddress, _ := crypto.NewAddressFromPublicKey(tokenPk)

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	ownerAddress, _ := crypto.NewAddressFromPublicKey(ownerPk)

	userSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	userPk := userSk.Ed25519Public()
	userAddress, _ := crypto.NewAddressFromPublicKey(userPk)

	spenderSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	spenderPk := spenderSk.Ed25519Public()
	spenderAddress, _ := crypto.NewAddressFromPublicKey(spenderPk)

	fmt.Printf("tokenAddress = %v \n", tokenAddress)
	fmt.Printf("ownerAddress = %v \n", ownerAddress)
	fmt.Printf("userAddress = %v \n", userAddress)
	fmt.Printf("spenderAddress = %v \n\n", spenderAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(tokenSk, *tokenAddress, milon.PubKeySignatureMode{PublicKey: *tokenPk}); err != nil {
		panic("Failed to ClaimFaucet token:" + err.Error())
	}
	tokenBalance, err := client.AddressBalance(*tokenAddress)
	if err != nil {
		panic("Failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", tokenBalance)

	if err = client.ClaimFaucet(ownerSk, *ownerAddress, milon.PubKeySignatureMode{PublicKey: *ownerPk}); err != nil {
		panic("Failed to ClaimFaucet owner:" + err.Error())
	}
	ownerBalance, err := client.AddressBalance(*ownerAddress)
	if err != nil {
		panic("Failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	// 1. Simulate Create transaction
	simulateTransactionResult, err := client.BuildAndSimulateSingleIxSplit(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Burn Freeze Approve Token",
				"symbol":   "SBFA",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		*tokenAddress,
		milon.PubKeySignatureMode{PublicKey: *tokenPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create transaction for simulation: %v", err))
	}
	if simulateTransactionResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("simulation failed: %s", simulateTransactionResult.BodySimulateReceipt.Error.Message))
	}
	fmt.Printf("\n================ Simulation ================\n")
	fmt.Printf("Total gas fee: %d\n", simulateTransactionResult.BodySimulateReceipt.GasCharged)

	// 2. Create token on-chain (token is Signer)
	createSubmitResult, err := client.BuildAndSubmitSingleIxSplit(
		"token",
		"Create",
		provider.Args{
			"token": tokenAddress,
			"owner": ownerAddress,
			"metadata": map[string]any{
				"name":     "SDK Burn Freeze Approve Token",
				"symbol":   "SBFA",
				"decimals": 6,
				"icon":     "https://milon.test/token.png",
			},
		},
		tokenSk,
		*tokenAddress,
		milon.PubKeySignatureMode{PublicKey: *tokenPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit Create transaction: %v", err))
	}
	fmt.Printf("\nAnd we wait for the Create transaction %s to complete...\n", createSubmitResult.BodyTxHash)
	createTxResult, err := client.WaitForTransaction(createSubmitResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for Create transaction: %v", err))
	}
	if createTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Create transaction failed on chain: error = %v", createTxResult.BodyTxHistory.Receipt.Error))
	}
	helper.DisplayTxHistoryAndGetResourceAndGetAccessValue(client, createTxResult.BodyTxHistory)

	// 3. Mint tokens to user (signer_lookups: owner, use ownerSk)
	mintSubmitResult, err := client.BuildAndSubmitSingleIxSplit(
		"token",
		"Mint",
		provider.Args{
			"token":  tokenAddress,
			"to":     userAddress,
			"amount": uint64(1000),
		},
		ownerSk,
		*ownerAddress,
		milon.PubKeySignatureMode{PublicKey: *ownerPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit Mint transaction: %v", err))
	}
	fmt.Printf("\nAnd we wait for the Mint transaction %s to complete...\n", mintSubmitResult.BodyTxHash)
	mintTxResult, err := client.WaitForTransaction(mintSubmitResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for Mint transaction: %v", err))
	}
	if mintTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Mint transaction failed on chain: error = %v", mintTxResult.BodyTxHistory.Receipt.Error))
	}

	// 4. Burn tokens from user (holder is Signer, use userSk)
	burnSubmitResult, err := client.BuildAndSubmitSingleIxSplit(
		"token",
		"Burn",
		provider.Args{
			"holder": userAddress,
			"token":  tokenAddress,
			"amount": uint64(100),
		},
		userSk,
		*userAddress,
		milon.PubKeySignatureMode{PublicKey: *userPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit Burn transaction: %v", err))
	}
	fmt.Printf("\nAnd we wait for the Burn transaction %s to complete...\n", burnSubmitResult.BodyTxHash)
	burnTxResult, err := client.WaitForTransaction(burnSubmitResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for Burn transaction: %v", err))
	}
	if burnTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Burn transaction failed on chain: error = %v", burnTxResult.BodyTxHistory.Receipt.Error))
	}

	// 5. Freeze user tokens (signer_lookups: freezer=owner, use ownerSk)
	freezeSubmitResult, err := client.BuildAndSubmitSingleIxSplit(
		"token",
		"Freeze",
		provider.Args{
			"token":  tokenAddress,
			"holder": userAddress,
			"amount": uint64(200),
		},
		ownerSk,
		*ownerAddress,
		milon.PubKeySignatureMode{PublicKey: *ownerPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit Freeze transaction: %v", err))
	}
	fmt.Printf("\nAnd we wait for the Freeze transaction %s to complete...\n", freezeSubmitResult.BodyTxHash)
	freezeTxResult, err := client.WaitForTransaction(freezeSubmitResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for Freeze transaction: %v", err))
	}
	if freezeTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Freeze transaction failed on chain: error = %v", freezeTxResult.BodyTxHistory.Receipt.Error))
	}

	// 6. View FrozenOf user
	fmt.Printf("\n================ View FrozenOf ================\n")
	frozenOfResult, err := client.BuildAndViewSingleIx(
		"token",
		"FrozenOf",
		provider.Args{
			"token":   tokenAddress,
			"account": userAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view FrozenOf: " + err.Error())
	}
	if failure, ok := frozenOfResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("FrozenOf RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("user frozen amount: %+v \n", frozenOfResult.BodyValues.(uint64))
	}

	// 7. View BalanceOf user
	fmt.Printf("\n================ View BalanceOf user ================\n")
	userBalanceResult, err := client.BuildAndViewSingleIx(
		"token",
		"BalanceOf",
		provider.Args{
			"token":   tokenAddress,
			"account": userAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view BalanceOf for user: " + err.Error())
	}
	if failure, ok := userBalanceResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("user token RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("user token balance: %+v \n", userBalanceResult.BodyValues.(uint64))
	}

	// 8. Approve spender (owner is Signer, use ownerSk)
	approveSubmitResult, err := client.BuildAndSubmitSingleIxSplit(
		"token",
		"Approve",
		provider.Args{
			"owner":   ownerAddress,
			"token":   tokenAddress,
			"spender": spenderAddress,
			"amount":  uint64(300),
		},
		ownerSk,
		*ownerAddress,
		milon.PubKeySignatureMode{PublicKey: *ownerPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit Approve transaction: %v", err))
	}
	fmt.Printf("\nAnd we wait for the Approve transaction %s to complete...\n", approveSubmitResult.BodyTxHash)
	approveTxResult, err := client.WaitForTransaction(approveSubmitResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for Approve transaction: %v", err))
	}
	if approveTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Approve transaction failed on chain: error = %v", approveTxResult.BodyTxHistory.Receipt.Error))
	}

	// 9. View ApprovalOf
	fmt.Printf("\n================ View ApprovalOf ================\n")
	approvalOfResult, err := client.BuildAndViewSingleIx(
		"token",
		"ApprovalOf",
		provider.Args{
			"token":   tokenAddress,
			"owner":   ownerAddress,
			"spender": spenderAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view ApprovalOf: " + err.Error())
	}
	if failure, ok := approvalOfResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("ApprovalOf RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("approval amount: %+v \n", approvalOfResult.BodyValues.(uint64))
	}

	// 10. TransferFrom (spender is Signer, use spenderSk)
	transferFromSubmitResult, err := client.BuildAndSubmitSingleIxSplit(
		"token",
		"TransferFrom",
		provider.Args{
			"spender": spenderAddress,
			"token":   tokenAddress,
			"from":    ownerAddress,
			"amount":  uint64(50),
		},
		spenderSk,
		*spenderAddress,
		milon.PubKeySignatureMode{PublicKey: *spenderPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit TransferFrom transaction: %v", err))
	}
	fmt.Printf("\nAnd we wait for the TransferFrom transaction %s to complete...\n", transferFromSubmitResult.BodyTxHash)
	transferFromTxResult, err := client.WaitForTransaction(transferFromSubmitResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for TransferFrom transaction: %v", err))
	}
	if transferFromTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("TransferFrom transaction failed on chain: error = %v", transferFromTxResult.BodyTxHistory.Receipt.Error))
	}

	// 11. View BalanceOf owner
	fmt.Printf("\n================ View BalanceOf owner ================\n")
	ownerTokenBalanceResult, err := client.BuildAndViewSingleIx(
		"token",
		"BalanceOf",
		provider.Args{
			"token":   tokenAddress,
			"account": ownerAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view BalanceOf for owner: " + err.Error())
	}
	if failure, ok := ownerTokenBalanceResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("owner token RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("owner token balance: %+v \n", ownerTokenBalanceResult.BodyValues.(uint64))
	}

	// 12. View BalanceOf spender
	fmt.Printf("\n================ View BalanceOf spender ================\n")
	spenderTokenBalanceResult, err := client.BuildAndViewSingleIx(
		"token",
		"BalanceOf",
		provider.Args{
			"token":   tokenAddress,
			"account": spenderAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view BalanceOf for spender: " + err.Error())
	}
	if failure, ok := spenderTokenBalanceResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("spender token RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("spender token balance: %+v \n", spenderTokenBalanceResult.BodyValues.(uint64))
	}
}
