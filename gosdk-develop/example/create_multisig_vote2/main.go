package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/helper"
	"github.com/milon-labs/milon-go-sdk/lib"
)

func main() {
	example(milon.DevNet)
}

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// Create 4 signers: signerMultiSig is the multisig wallet creator, signerA/B/C are the participants
	signerMultiSig := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerA := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerB := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	signerC := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())

	pkMultiSig := signerMultiSig.Ed25519Public()
	pkA := signerA.Ed25519Public()
	pkB := signerB.Ed25519Public()
	pkC := signerC.Ed25519Public()

	accountA, _ := crypto.NewAddressFromPublicKey(pkA)
	accountB, _ := crypto.NewAddressFromPublicKey(pkB)
	accountC, _ := crypto.NewAddressFromPublicKey(pkC)
	accountMultiSig, _ := crypto.NewAddressFromPublicKey(pkMultiSig)

	fmt.Printf("pkA = %v \n", pkA)
	fmt.Printf("pkB = %v \n", pkB)
	fmt.Printf("pkC = %v \n\n", pkC)

	fmt.Printf("accountA = %v \n", accountA)
	fmt.Printf("accountMultiSig = %v \n\n", accountMultiSig)

	fmt.Printf("\n================ 1.CreateMultisig ================\n")

	// Claim initial MIL for the multisig wallet (still a regular account, signed solo by the creator)
	if err := client.ClaimFaucet(signerMultiSig, accountMultiSig, lib.PubKeySignatureMode{PublicKey: *pkMultiSig}); err != nil {
		panic("failed to ClaimFaucet MIL:" + err.Error())
	}
	accountMultiSigBalanceClaimFaucet, err := client.BalanceOf(accountMultiSig)
	if err != nil {
		panic("failed to get accountMultiSig MIL:" + err.Error())
	}

	// Create multisig: signers [pkA, pkB, pkC] (positions 0/1/2), weights [1,2,3], threshold 4.
	// Unified-payer: creator signs ix0 + gas (bit63) solo.
	wire, err := gen.Account.CreateMultisig.Args(accountMultiSig, []*crypto.PublicKey{pkA, pkB, pkC}, []uint8{1, 2, 3}, 4).Encode()
	if err != nil {
		panic("failed to encode CreateMultisig instruction:" + err.Error())
	}
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(accountMultiSig).
		AddIxAndPayerSig(*accountMultiSig, signerMultiSig, 0, lib.PubKeySignatureMode{PublicKey: *pkMultiSig}).
		Build()
	if err != nil {
		panic("failed to build and sign transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit transaction:" + err.Error())
	}
	getTxByHashResult, err := client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("CreateMultisig committed: tx %s, gas charged: %d\n\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	fmt.Printf("\n================ 2.transfer MIL By Vote ================\n")

	// 1. Encode the business instruction the multisig wallet intends to execute:
	//    transfer 1000 MIL from accountMultiSig to accountA. The same wire (same ix hash) must be used by the vote proposal and the final execute tx.
	transferWire, err := gen.Token.Transfer.Args(accountMultiSig, api.MILToken, accountA, 1000).Encode()
	if err != nil {
		panic("failed to encode Transfer instruction:" + err.Error())
	}
	transferIxHash := lib.NewTransactionBuilder([]api.PackedInstruction{transferWire}).Tx().IxHashes()[0]

	// 2. Build the intent: ix0 is vote-gated. intent_hash = vote_batch_hash
	//    (MIP-25) binds the whole execute tx (all ix hashes) plus the gated auth subset.
	authIx0, err := lib.AuthIx(0)
	if err != nil {
		panic("failed to build ix auth bit:" + err.Error())
	}
	intentHash := lib.VoteBatchHash(authIx0, []api.TxHash{transferIxHash})
	proposal := gen.AccountVoteProposal{
		Instructions: [][]byte{[]byte(transferWire)}, // must equal the execute tx instructions
		AuthBit:      authIx0.Raw(),                  // proposal auth_bit carries ix bits only (no bit62/63)
	}
	fmt.Printf("intent_hash = %s (vote over ix0 of the transfer)\n\n", intentHash)

	// 3. Top up signerB/signerC standalone accounts: each pays the gas of his own vote
	//    transaction (unified-payer per vote tx). Balances are kept for the final summary.
	if err = client.ClaimFaucet(signerB, accountB, lib.PubKeySignatureMode{PublicKey: *pkB}); err != nil {
		panic("failed to ClaimFaucet accountB MIL:" + err.Error())
	}
	if err = client.ClaimFaucet(signerC, accountC, lib.PubKeySignatureMode{PublicKey: *pkC}); err != nil {
		panic("failed to ClaimFaucet accountC MIL:" + err.Error())
	}
	accountBBalanceClaim, err := client.BalanceOf(accountB)
	if err != nil {
		panic("failed to get accountB MIL:" + err.Error())
	}
	accountCBalanceClaim, err := client.BalanceOf(accountC)
	if err != nil {
		panic("failed to get accountC MIL:" + err.Error())
	}

	// 4. First vote (vote_init) by participant signerB (signer index 1, weight 2).
	//    Unified-payer: accountMultiSig authorizes ix0 only (vote signature, no bit63);
	//    accountB (signerB's own account) pays gas (bit63). Without bit63 on the
	//    multisig sig, the any_signer partial rule applies: single member is enough.
	//    weight 2 < threshold 4 -> intent registered but not ready yet.
	voteInitWire, err := gen.Account.VoteInit.Args(accountMultiSig, intentHash, proposal).Encode()
	if err != nil {
		panic("failed to encode VoteInit instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{voteInitWire}).
		WithPayer(accountB).                                                                                               // unified-payer: gas from accountB
		AddPayerSig(*accountB, signerB, lib.PubKeySignatureMode{PublicKey: *pkB}).                                         // accountB: gas (bit63) only, unified-payer
		AddIxesSig(*accountMultiSig, signerB, []uint8{0}, false, lib.MultisigKeySignatureMode{Index: 1, PublicKey: *pkB}). // accountMultiSig: ix0 only, no bit63
		Build()
	if err != nil {
		panic("failed to build VoteInit transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit VoteInit transaction:" + err.Error())
	}
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("VoteInit committed: tx %s, gas charged: %d\n\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// 5. Second vote by participant signerC (signer index 2, weight 3).
	//    Unified-payer again: accountC pays the gas, accountMultiSig ix0 only.
	//    cumulative weight 2 + 3 >= threshold 4 -> ready.
	voteWire, err := gen.Account.Vote.Args(accountMultiSig, intentHash).Encode()
	if err != nil {
		panic("failed to encode Vote instruction:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{voteWire}).
		WithPayer(accountC).                                                                                               // unified-payer: gas from accountC
		AddPayerSig(*accountC, signerC, lib.PubKeySignatureMode{PublicKey: *pkC}).                                         // accountC: gas (bit63) only, unified-payer
		AddIxesSig(*accountMultiSig, signerC, []uint8{0}, false, lib.MultisigKeySignatureMode{Index: 2, PublicKey: *pkC}). // accountMultiSig: ix0 only, no bit63
		Build()
	if err != nil {
		panic("failed to build Vote transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit Vote transaction:" + err.Error())
	}
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("Vote committed: tx %s, gas charged: %d\n\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	// 6. Query on-chain vote progress: GetVote -> tuple<VoteMeta, sig_bit, ready>.
	getVoteWire, err := gen.Account.GetVote.Args(accountMultiSig, intentHash).Encode()
	if err != nil {
		panic("failed to encode GetVote view:" + err.Error())
	}
	viewResult, err := client.View([]api.PackedInstruction{getVoteWire})
	if err != nil {
		panic("failed to query GetVote view:" + err.Error())
	}
	voteView, err := gen.Account.GetVote.DecodeView(viewResult.HTTPResponseBody)
	if err != nil {
		panic("failed to decode GetVote view:" + err.Error())
	}
	sigBit, sigBitOK := voteView[1].(uint64)
	ready, readyOK := voteView[2].(bool)
	if !sigBitOK || !readyOK {
		panic("unexpected GetVote view payload")
	}
	fmt.Printf("vote progress: sig_bit=0b%b (signerB|signerC), ready=%v\n", sigBit, ready)
	if !ready {
		panic("vote weight below threshold: execute transaction would be rejected")
	}

	// 7. Create a relayer account that sponsors the gas of the execute tx.
	relayer := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pkRelayer := relayer.Ed25519Public()
	accountRelayer, _ := crypto.NewAddressFromPublicKey(pkRelayer)
	if err = client.ClaimFaucet(relayer, accountRelayer, lib.PubKeySignatureMode{PublicKey: *pkRelayer}); err != nil {
		panic("failed to ClaimFaucet relayer MIL:" + err.Error())
	}
	accountRelayerBalanceClaimFaucet, err := client.BalanceOf(accountRelayer)
	if err != nil {
		panic("failed to get accountRelayer MIL:" + err.Error())
	}

	// 8. Execute the transfer with a vote-gated auth-bit-only signature:
	//    - accountRelayer: gas (bit63) only, unified-payer (sponsor)
	//    - accountMultiSig: ix0 + vote gate flag (bit62), auth-bit-only, i.e.
	//      no cryptographic signature once the on-chain vote has passed.
	voteGateAuth, err := lib.AuthVoteIxes([]uint8{0})
	if err != nil {
		panic("failed to build vote gate auth bits:" + err.Error())
	}
	tx, err = lib.NewTransactionBuilder([]api.PackedInstruction{transferWire}).
		WithPayer(accountRelayer).
		AddPayerSig(*accountRelayer, relayer, lib.PubKeySignatureMode{PublicKey: *pkRelayer}). // relayer: gas (bit63), unified-payer
		AddSignature(*accountMultiSig, lib.Unsigned(voteGateAuth)).                            // accountMultiSig: ix0 + bit62 vote gate, auth-bit-only
		Build()
	if err != nil {
		panic("failed to build vote execute transaction:" + err.Error())
	}
	err = client.SubmitTx(tx)
	if err != nil {
		panic("failed to submit vote execute transaction:" + err.Error())
	}
	getTxByHashResult, err = client.WaitForTransaction(tx.TxHash())
	if err != nil {
		panic("failed to wait for transaction:" + err.Error())
	}
	helper.CheckTxSuccess(getTxByHashResult.BodyTxHistory)
	fmt.Printf("vote execute committed: tx %s, gas charged: %d\n\n", tx.TxHash(), getTxByHashResult.BodyTxHistory.Receipt.GasCharged)

	fmt.Printf("\n================ Final MIL ================\n")

	accountBBalanceFinal, err := client.BalanceOf(accountB)
	if err != nil {
		panic("failed to get accountB MIL:" + err.Error())
	}
	fmt.Printf("accountB MIL: %d | diff: %d (voteInit gas) \n", accountBBalanceFinal, accountBBalanceClaim-accountBBalanceFinal)

	accountCBalanceFinal, err := client.BalanceOf(accountC)
	if err != nil {
		panic("failed to get accountC MIL:" + err.Error())
	}
	fmt.Printf("accountC MIL: %d | diff: %d (vote gas) \n", accountCBalanceFinal, accountCBalanceClaim-accountCBalanceFinal)

	accountRelayerBalanceSponsor, err := client.BalanceOf(accountRelayer)
	if err != nil {
		panic("failed to get accountRelayer MIL:" + err.Error())
	}
	fmt.Printf("accountRelayer MIL: %d | diff: %d (execute gas) \n", accountRelayerBalanceSponsor, accountRelayerBalanceClaimFaucet-accountRelayerBalanceSponsor)

	accountMultiSigBalanceFinal, err := client.BalanceOf(accountMultiSig)
	if err != nil {
		panic("failed to get accountMultiSig MIL:" + err.Error())
	}
	fmt.Printf("accountMultiSig MIL: %d | diff: %d (create multisig gas + 1000 MIL transfer out) \n", accountMultiSigBalanceFinal, accountMultiSigBalanceClaimFaucet-accountMultiSigBalanceFinal)

	accountABalance, err := client.BalanceOf(accountA)
	if err != nil {
		panic("failed to get accountA MIL:" + err.Error())
	}
	fmt.Printf("accountA MIL: %d | diff: %d (1000 MIL transfer in) \n", accountABalance, accountABalance)
}
