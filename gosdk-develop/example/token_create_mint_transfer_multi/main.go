package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

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
	if err := client.ClaimFaucet(tokenSk, *tokenAddress, lib.PubKeySignatureMode{PublicKey: *tokenPk}); err != nil {
		panic("Failed to ClaimFaucet token:" + err.Error())
	}
	tokenBalance, err := client.BalanceOf(*tokenAddress)
	if err != nil {
		panic("Failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", tokenBalance)

	if err = client.ClaimFaucet(ownerSk, *ownerAddress, lib.PubKeySignatureMode{PublicKey: *ownerPk}); err != nil {
		panic("Failed to ClaimFaucet owner:" + err.Error())
	}
	ownerBalance, err := client.BalanceOf(*ownerAddress)
	if err != nil {
		panic("Failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	if err = client.ClaimFaucet(user1Sk, *user1Address, lib.PubKeySignatureMode{PublicKey: *user1Pk}); err != nil {
		panic("Failed to ClaimFaucet user1:" + err.Error())
	}
	user1Balance, err := client.BalanceOf(*user1Address)
	if err != nil {
		panic("Failed to get user1 MIL:" + err.Error())
	}
	fmt.Printf("user1 MIL: %d\n", user1Balance)

	// 1. Look up token IDL provider (loaded during NewClient)
	pd, err := client.GetPdByIDLAppName("token")
	if err != nil {
		panic("Failed to to get IDL provider for 'token':" + err.Error())
	}

	// 2. Encode instructions (Create + Mint + Transfer)
	createWire, err := pd.Encode("Create", provider.Args{
		"token": tokenAddress,
		"owner": ownerAddress,
		"metadata": map[string]any{
			"name":     "Example Token",
			"symbol":   "Token",
			"decimals": 6,
			"icon":     "https://milon.test/token.png",
		},
	})
	if err != nil {
		panic("Failed to encode Create instruction:" + err.Error())
	}

	mintWire, err := pd.Encode("Mint", provider.Args{
		"token":  tokenAddress,
		"to":     user1Address,
		"amount": 1000,
	})
	if err != nil {
		panic("Failed to encode Mint instruction:" + err.Error())
	}

	transferWire, err := pd.Encode("Transfer", provider.Args{
		"from":   user1Address,
		"token":  tokenAddress,
		"to":     user2Address,
		"amount": 300,
	})
	if err != nil {
		panic("Failed to encode Transfer instruction:" + err.Error())
	}

	// 2. Define signing slots once (shared by simulate & real sign)
	slots := []lib.SigningSlot{
		{*tokenAddress, []uint8{0}, true, lib.PubKeySignatureMode{PublicKey: *tokenPk}},
		{*ownerAddress, []uint8{1}, true, lib.PubKeySignatureMode{PublicKey: *ownerPk}},
		{*user1Address, []uint8{2}, true, lib.PubKeySignatureMode{PublicKey: *user1Pk}},
	}

	// 3. Build transaction once, reuse the same builder for simulate & real sign (same Stamp -> same TxHash)
	builder := lib.NewTransactionBuilder([]api.PackedInstruction{createWire, mintWire, transferWire}).ApplySlots(slots)

	// 3.1 Simulate on-chain first (no private key needed, dry-run)
	simulateTx, err := builder.SimulateSlots().Build()
	if err != nil {
		panic("Failed to simulate transaction:" + err.Error())
	}
	simulateResult, err := client.SimulateTx(simulateTx)
	if err != nil {
		panic("Failed to simulate transaction on chain:" + err.Error())
	}
	if simulateResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Simulate failed on chain: error code = %d", simulateResult.BodySimulateReceipt.Error.Code))
	}
	fmt.Printf("Simulated transaction hash: %s, gas charged: %d\n", simulateTx.TxHash(), simulateResult.BodySimulateReceipt.GasCharged)

	// 3.2 Replace simulated signatures with real ones on the same transaction
	tx, err := builder.ResetSigs().
		SignWith(
			lib.Signer{SecretKey: user1Sk, PublicKey: *user1Pk},
			lib.Signer{SecretKey: ownerSk, PublicKey: *ownerPk},
			lib.Signer{SecretKey: tokenSk, PublicKey: *tokenPk},
		).
		Build()
	if err != nil {
		panic("Failed to build and sign transaction:" + err.Error())
	}

	// 4. Submit transaction on chain
	err = client.SubmitTx(tx)
	if err != nil {
		panic("Failed to submit transaction:" + err.Error())
	}

	// 5. Wait for the transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", tx.TxHash())
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash(), 1)
	if err != nil {
		panic("Failed to wait for transaction:" + err.Error())
	}
	if getTxByHashResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Transaction failed on chain: error code = %d", *getTxByHashResult.BodyTxHistory.Receipt.Error))
	}
	fmt.Printf("Submit transaction hash: %s, gas charged: %d\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	fmt.Printf("\n================ Final MIL ================\n")
	tokenBalance, err = client.BalanceOf(*tokenAddress)
	if err != nil {
		panic("Failed to get token MIL:" + err.Error())
	}
	fmt.Printf("token MIL: %d\n", tokenBalance)

	ownerBalance, err = client.BalanceOf(*ownerAddress)
	if err != nil {
		panic("Failed to get owner MIL:" + err.Error())
	}
	fmt.Printf("owner MIL: %d\n", ownerBalance)

	user1Balance, err = client.BalanceOf(*user1Address)
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
		fmt.Printf("view[%d] : %+v \n", i, decodedTaggedValue)

		if failure, ok := decodedTaggedValue.Value.(*api.TxFailurePayload); ok {
			fmt.Printf("❌ err = %+v \n", failure)
		}
	}
}
