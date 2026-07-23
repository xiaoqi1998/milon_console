package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
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
	fmt.Printf("user1Address = %v \n", user1Address)
	fmt.Printf("user2Address = %v \n", user2Address)
	fmt.Printf("user3Address = %v \n\n", user3Address)

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

	if err = client.ClaimFaucet(user1Sk, *user1Address, milon.PubKeySignatureMode{PublicKey: *user1Pk}); err != nil {
		panic("Failed to ClaimFaucet user1:" + err.Error())
	}
	user1Balance, err := client.AddressBalance(*user1Address)
	if err != nil {
		panic("Failed to get user1 MIL:" + err.Error())
	}
	fmt.Printf("user1 MIL: %d\n", user1Balance)

	// 1. Look up token IDL provider (loaded during NewMilonClient)
	pd, err := client.GetPdByIDLAppName("token")
	if err != nil {
		panic(fmt.Sprintf("failed to get IDL provider for 'token': %v", err))
	}

	// 2. Encode instructions (Create + Mint + Transfer)
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
		panic(fmt.Sprintf("failed to encode Create instruction: %v", err))
	}

	mintWire, err := pd.Encode("Mint", provider.Args{
		"token":  tokenAddress,
		"to":     user1Address,
		"amount": 1000,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to encode Mint instruction: %v", err))
	}

	transferWire, err := pd.Encode("Transfer", provider.Args{
		"from":   user1Address,
		"token":  tokenAddress,
		"to":     user2Address,
		"amount": 300,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to encode Transfer instruction: %v", err))
	}

	wires := make([]api.PackedInstruction, 0)
	skList := make([]crypto.SecretKeyer, 0)
	addressList := make([]crypto.Address, 0)
	acSigModList := make([]milon.AccountSignatureMode, 0)

	wires = append(wires, createWire)
	skList = append(skList, tokenSk)
	addressList = append(addressList, *tokenAddress)
	acSigModList = append(acSigModList, milon.PubKeySignatureMode{PublicKey: *tokenPk})

	wires = append(wires, mintWire)
	skList = append(skList, ownerSk)
	addressList = append(addressList, *ownerAddress)
	acSigModList = append(acSigModList, milon.PubKeySignatureMode{PublicKey: *ownerPk})

	wires = append(wires, transferWire)
	skList = append(skList, user1Sk)
	addressList = append(addressList, *user1Address)
	acSigModList = append(acSigModList, milon.PubKeySignatureMode{PublicKey: *user1Pk})

	// 3. Simulate transaction
	simulateTransactionResult, err := client.BuildAndSimulateMultiIxSplit(
		wires,
		addressList,
		acSigModList,
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create transaction for simulation: %v", err))
	}
	fmt.Printf("\n================ Simulation ================\n")
	fmt.Printf("Total gas fee: %d\n", simulateTransactionResult.BodySimulateReceipt.GasCharged)

	// 4. Create on-chain transaction
	submitTransactionResult, err := client.BuildAndSubmitMultiIxSplit(
		wires,
		skList,
		addressList,
		acSigModList,
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create on-chain transaction: %v", err))
	}

	// 5. Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitTransactionResult.BodyTxHash)
	getTxByHashResult, err := client.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getTxByHashResult.BodyTxHistory.Receipt.Error))
	}

	fmt.Printf("\n================ Final MIL ================\n")
	tokenBalance, err = client.AddressBalance(*tokenAddress)
	if err != nil {
		panic("Failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", tokenBalance)

	ownerBalance, err = client.AddressBalance(*ownerAddress)
	if err != nil {
		panic("Failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	user1Balance, err = client.AddressBalance(*user1Address)
	if err != nil {
		panic("Failed to get user1 MIL:" + err.Error())
	}
	fmt.Printf("user1 MIL: %d\n", user1Balance)

	fmt.Printf("\n================ token Balances ================\n")

	viewSingleTransactionResult1, err := client.BuildAndViewSingleIx(
		"token",
		"BalanceOf",
		provider.Args{
			"token":   tokenAddress,
			"account": user1Address,
		},
		1,
	)
	if err != nil {
		panic("failed to view BalanceOf for user1: " + err.Error())
	}

	if failure, ok := viewSingleTransactionResult1.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("user1 token RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("user1 token : %+v \n", viewSingleTransactionResult1.BodyValues.(uint64))
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
		panic("failed to view BalanceOf for user2: " + err.Error())
	}

	if failure, ok := viewTransactionResult2.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("user2 token RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("user2 token : %+v \n", viewTransactionResult2.BodyValues.(uint64))
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
		panic("failed to view BalanceOf for user3: " + err.Error())
	}

	if failure, ok := viewTransactionResult3.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("user3 token RPC query error: %+v \n", failure)
	} else {
		fmt.Printf("user3 token : %+v \n", viewTransactionResult3.BodyValues.(uint64))
	}

	fmt.Printf("\n================ Now do it again, but with a different method ================\n")

	wire1, err := pd.Encode("BalanceOf", provider.Args{
		"token":   tokenAddress,
		"account": user1Address,
	})
	if err != nil {
		panic("failed to encode BalanceOf for user1: " + err.Error())
	}

	wire2, err := pd.Encode("BalanceOf", provider.Args{
		"token":   tokenAddress,
		"account": user2Address,
	})
	if err != nil {
		panic("failed to encode BalanceOf for user2: " + err.Error())
	}

	wire3, err := pd.Encode("BalanceOf", provider.Args{
		"token":   tokenAddress,
		"account": user3Address,
	})
	if err != nil {
		panic("failed to encode BalanceOf for user3: " + err.Error())
	}

	wire4, err := pd.Encode("Metadata", provider.Args{
		"token": tokenAddress,
	})
	if err != nil {
		panic("failed to encode Metadata: " + err.Error())
	}

	wire5, err := pd.Encode("TotalSupply", provider.Args{
		"token": tokenAddress,
	})
	if err != nil {
		panic("failed to encode TotalSupply: " + err.Error())
	}

	viewMultiResult, err := client.BuildAndViewMultiIx(
		[]api.PackedInstruction{
			wire1,
			wire2,
			wire3,
			wire4,
			wire5,
		},
		1,
	)
	if err != nil {
		panic("failed to build and view multi-ix: " + err.Error())
	}

	decodedTaggedValueList, err := client.GetProviderManager().DecodeViewDatas(
		[]string{
			"token::BalanceOf",
			"token::BalanceOf",
			"token::BalanceOf",
			"token::Metadata",
			"token::TotalSupply",
		},
		viewMultiResult.HttpRspBody,
	)
	if err != nil {
		panic("failed to decode view data: " + err.Error())
	}

	for i, decodedTaggedValue := range decodedTaggedValueList {
		fmt.Printf("decodedTaggedValueList[%d] : %+v \n", i, decodedTaggedValue)

		if failure, ok := decodedTaggedValue.Value.(*api.TxFailurePayload); ok {
			fmt.Printf("❌ Instruction %d failed | err = %v \n", i, failure)
			fmt.Printf("    Error code = %d\n", failure.Code)
			fmt.Printf("    Error message = %s\n", failure.Message)
			fmt.Printf("    Additional data = %v\n\n", failure.Data)
		} else {
			fmt.Printf("✅ Instruction %d: value=%v \n\n", i, decodedTaggedValue.Value)
		}
	}
}
