// Engine: Go 1.26 | Milon Go SDK (local gosdk-develop) | tool
// Build: go run ./tools/gen_sft_holders -count 1000 -seed <seed> -out sft_1000x1_holders.json
// Run:   go run ./tools/gen_sft_holders
// Deps:  github.com/milon-labs/milon-go-sdk (replace ./gosdk-develop)

// gen_sft_holders deterministically derives N FN-DSA-512 (抗量子) holder keypairs
// plus one whale account for the SFT holder-distribution fixture. Recipients do not
// need MIL or signatures: sftoken.Mint mints directly to the derived addresses.
// 2026-11-26:经典 Ed25519 不再产出——链端 owner/signer 已 PQ 化,派生一批经典
// 账户等于派生废数据。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/pornin/go-fn-dsa/fndsa"
)

type holderOut struct {
	Index      int    `json:"index"`
	Name       string `json:"name"`
	Role       string `json:"role"`
	Curve      string `json:"curve"`
	Address    string `json:"address"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

type datasetOut struct {
	Seed    string      `json:"seed"`
	Count   int         `json:"count"`
	Whale   holderOut   `json:"whale"`
	Holders []holderOut `json:"holders"`
}

func derive(seed, role, name string, index int) holderOut {
	sum := sha256.Sum256([]byte(seed + ":" + role + ":" + name))
	signKey, verifyKey, err := fndsa.KeyGen(9, bytes.NewReader(sum[:]))
	if err != nil {
		panic(fmt.Sprintf("derive %s: %v", name, err))
	}
	if len(signKey) != crypto.FnDsa512KeySize {
		panic(fmt.Sprintf("derive %s: fndsa512 私钥长度异常 %d", name, len(signKey)))
	}
	pk, err := crypto.NewPublicKeyFromBytes(verifyKey)
	if err != nil {
		panic(fmt.Sprintf("public key %s: %v", name, err))
	}
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		panic(fmt.Sprintf("address %s: %v", name, err))
	}
	return holderOut{
		Index:      index,
		Name:       name,
		Role:       role,
		Curve:      "fndsa512",
		Address:    addr.ToBase58(),
		PublicKey:  pk.ToBase58(),
		PrivateKey: hex.EncodeToString(signKey),
	}
}

func main() {
	seed := flag.String("seed", "milon-sft-1000x1-plus-1x100-20261019", "deterministic seed phrase")
	count := flag.Int("count", 1000, "number of 1-share holders")
	out := flag.String("out", "sft_1000x1_holders.json", "output JSON path")
	flag.Parse()

	if *count <= 0 {
		panic("count must be positive")
	}

	ds := datasetOut{
		Seed:    *seed,
		Count:   *count,
		Whale:   derive(*seed, "whale", "whale0001", *count+1),
		Holders: make([]holderOut, 0, *count),
	}
	for i := 1; i <= *count; i++ {
		name := fmt.Sprintf("holder%04d", i)
		ds.Holders = append(ds.Holders, derive(*seed, "holder", name, i))
	}

	raw, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(*out, append(raw, '\n'), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s: %d holders + 1 whale\n", *out, len(ds.Holders))
	fmt.Printf("sample holder[0] = %s\n", ds.Holders[0].Address)
	fmt.Printf("whale            = %s\n", ds.Whale.Address)
}
