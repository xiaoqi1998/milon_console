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

	// operator: main signer, pays gas and acts as the staker (owner) for Stake
	operatorSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	operatorPk := operatorSk.Ed25519Public()
	operatorAddress, _ := crypto.NewAddressFromPublicKey(operatorPk)

	// validator: the validator address (Address input)
	validatorSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	validatorPk := validatorSk.Ed25519Public()
	validatorAddress, _ := crypto.NewAddressFromPublicKey(validatorPk)

	// consensus_account: additional Signer required by CreateValidator
	consensusSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	consensusPk := consensusSk.Ed25519Public()
	consensusAddress, _ := crypto.NewAddressFromPublicKey(consensusPk)

	fmt.Printf("operatorAddress   = %v \n", operatorAddress)
	fmt.Printf("validatorAddress  = %v \n", validatorAddress)
	fmt.Printf("consensusAddress  = %v \n\n", consensusAddress)

	fmt.Printf("\n================ Initial MIL ================\n")
	if err := client.ClaimFaucet(operatorSk, *operatorAddress, milon.PubKeySignatureMode{PublicKey: *operatorPk}); err != nil {
		panic("Failed to ClaimFaucet operator:" + err.Error())
	}
	operatorBalance, err := client.AddressBalance(*operatorAddress)
	if err != nil {
		panic("Failed to get operator MIL:" + err.Error())
	}
	fmt.Printf("operator MIL: %d\n", operatorBalance)

	// Placeholder bytes for validator consensus keys
	consensusPubkey := make([]byte, 32)
	blsPubkey := make([]byte, 32)
	networkAddress := []byte{10, 20, 30, 40, 50}

	// 1. Simulate CreateValidator transaction
	simulateCreateValidatorResult, err := client.BuildAndSimulateSingleIxSplit(
		"staking",
		"CreateValidator",
		provider.Args{
			"operator":            operatorAddress,
			"validator":           validatorAddress,
			"consensus_account":   consensusAddress,
			"consensus_pubkey":    consensusPubkey,
			"bls_pubkey":          blsPubkey,
			"network_address":     networkAddress,
			"commission_rate_bps": uint64(100),
		},
		*operatorAddress,
		milon.PubKeySignatureMode{PublicKey: *operatorPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create transaction for simulation: %v", err))
	}
	if simulateCreateValidatorResult.BodySimulateReceipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("simulation failed: %s", simulateCreateValidatorResult.BodySimulateReceipt.Error.Message))
	}
	fmt.Printf("\n================ Simulation (CreateValidator) ================\n")
	fmt.Printf("Total gas fee: %d\n", simulateCreateValidatorResult.BodySimulateReceipt.GasCharged)

	// 2. Submit CreateValidator transaction on-chain
	submitCreateValidatorResult, err := client.BuildAndSubmitSingleIxSplit(
		"staking",
		"CreateValidator",
		provider.Args{
			"operator":            operatorAddress,
			"validator":           validatorAddress,
			"consensus_account":   consensusAddress,
			"consensus_pubkey":    consensusPubkey,
			"bls_pubkey":          blsPubkey,
			"network_address":     networkAddress,
			"commission_rate_bps": uint64(100),
		},
		operatorSk,
		*operatorAddress,
		milon.PubKeySignatureMode{PublicKey: *operatorPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit transaction: %v", err))
	}

	// 3. Wait for the CreateValidator transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitCreateValidatorResult.BodyTxHash)
	getCreateValidatorTxResult, err := client.WaitForTransaction(submitCreateValidatorResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for transaction: %v", err))
	}
	if getCreateValidatorTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("transaction failed on chain: error = %v", getCreateValidatorTxResult.BodyTxHistory.Receipt.Error))
	}

	// 4. Query ValidatorProfile
	fmt.Printf("\n================ ValidatorProfile ================\n")
	validatorProfileResult, err := client.BuildAndViewSingleIx(
		"staking",
		"ValidatorProfile",
		provider.Args{
			"validator": validatorAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view ValidatorProfile: " + err.Error())
	}
	if failure, ok := validatorProfileResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("❌ ValidatorProfile query failed: %+v \n", failure)
	} else {
		fmt.Printf("✅ ValidatorProfile: %+v \n", validatorProfileResult.BodyValues)
	}

	// 5. Query ValidatorPool
	fmt.Printf("\n================ ValidatorPool ================\n")
	validatorPoolResult, err := client.BuildAndViewSingleIx(
		"staking",
		"ValidatorPool",
		provider.Args{
			"validator": validatorAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view ValidatorPool: " + err.Error())
	}
	if failure, ok := validatorPoolResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("❌ ValidatorPool query failed: %+v \n", failure)
	} else {
		fmt.Printf("✅ ValidatorPool: %+v \n", validatorPoolResult.BodyValues)
	}

	// 6. Submit Stake transaction (operator delegates to validator)
	submitStakeResult, err := client.BuildAndSubmitSingleIxSplit(
		"staking",
		"Stake",
		provider.Args{
			"owner":     operatorAddress,
			"validator": validatorAddress,
			"amount":    uint64(1000),
		},
		operatorSk,
		*operatorAddress,
		milon.PubKeySignatureMode{PublicKey: *operatorPk},
		1,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to build and submit Stake transaction: %v", err))
	}

	// 7. Wait for the Stake transaction to complete
	fmt.Printf("\nAnd we wait for the transaction %s to complete...\n", submitStakeResult.BodyTxHash)
	getStakeTxResult, err := client.WaitForTransaction(submitStakeResult.BodyTxHash, 1)
	if err != nil {
		panic(fmt.Sprintf("failed to wait for Stake transaction: %v", err))
	}
	if getStakeTxResult.BodyTxHistory.Receipt.State != api.TxStateSuccess {
		panic(fmt.Sprintf("Stake transaction failed on chain: error = %v", getStakeTxResult.BodyTxHistory.Receipt.Error))
	}

	// 8. Query StakePosition
	fmt.Printf("\n================ StakePosition ================\n")
	stakePositionResult, err := client.BuildAndViewSingleIx(
		"staking",
		"StakePosition",
		provider.Args{
			"owner":     operatorAddress,
			"validator": validatorAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view StakePosition: " + err.Error())
	}
	if failure, ok := stakePositionResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("❌ StakePosition query failed: %+v \n", failure)
	} else {
		fmt.Printf("✅ StakePosition: %+v \n", stakePositionResult.BodyValues)
	}

	// 9. Query PositionSummary
	fmt.Printf("\n================ PositionSummary ================\n")
	positionSummaryResult, err := client.BuildAndViewSingleIx(
		"staking",
		"PositionSummary",
		provider.Args{
			"owner":     operatorAddress,
			"validator": validatorAddress,
		},
		1,
	)
	if err != nil {
		panic("failed to view PositionSummary: " + err.Error())
	}
	if failure, ok := positionSummaryResult.BodyValues.(*api.TxFailurePayload); ok {
		fmt.Printf("❌ PositionSummary query failed: %+v \n", failure)
	} else {
		fmt.Printf("✅ PositionSummary: %+v \n", positionSummaryResult.BodyValues)
	}

	fmt.Printf("\n================ Final MIL ================\n")
	operatorBalance, err = client.AddressBalance(*operatorAddress)
	if err != nil {
		panic("Failed to get operator MIL:" + err.Error())
	}
	fmt.Printf("operator MIL: %d\n", operatorBalance)

	// Display TxHistory for CreateValidator
	helper.DisplayTxHistoryAndGetResourceAndGetAccessValue(client, getCreateValidatorTxResult.BodyTxHistory)

	// Display EventsByTxHash for CreateValidator
	if len(getCreateValidatorTxResult.BodyTxHistory.Receipt.Events) > 0 {
		helper.DisplayEventsByTxHash(client, submitCreateValidatorResult.BodyTxHash, nil)
	}
}
