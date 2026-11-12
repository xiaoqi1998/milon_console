package handler

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/milon-labs/milon-go-sdk/crypto"
)

// 链端 2026-11-12 起对 CreateMultisig 的 signers 强制 PQ 公钥：经典 ed25519/
// secp256k1 公钥直接被拒（273 Invalid public key，回执 Signers[].RequiresPqSignature=true；
// FN-DSA-512 公钥可通过并成功注册，实测 tx 1a4c66cd）。播种器的多签签者改为
// FN-DSA-512 密钥组。SDK 无法从 seed phrase 确定性派生 PQ 密钥（无 KDF 路径），
// 因此运行时随机生成、即生成即落盘 state：重跑幂等复用，换 state 文件则换密钥组。

// seedPqSigner 是一个多签 PQ 签者密钥组（持久化在 SeedState 中）。
type seedPqSigner struct {
	Name       string `json:"name"`       // 形如 ms01-pq1
	PrivateKey string `json:"privateKey"` // FN-DSA-512 expanded 私钥，1281B hex
	PublicKey  string `json:"publicKey"`  // FN-DSA-512 公钥，897B hex
	Address    string `json:"address"`    // 由公钥派生的链上地址，base58
}

// seedPqSignersStage 是某多签账户的 PQ 签者密钥组在 state 里的 stage 键。
func seedPqSignersStage(msName string) string { return "pqsigners:" + msName }

// seedPqOwnerStage 是某多签账户 owner PQ 密钥的 stage 键。
func seedPqOwnerStage(msName string) string { return "pqowner:" + msName }

// seedEnsurePqAccount 保证 ms 账户本体的 PQ 密钥存在并复用:返回 SecretKeyer、
// 公钥与派生地址。链端新策略下多签 owner 经典(ed25519/secp256k1)签名会被
// 授权校验拒绝(69 PqSignatureRequired),owner 也必须 PQ。
func seedEnsurePqAccount(st *SeedState, msName string) (crypto.SecretKeyer, crypto.PublicKey, crypto.Address, error) {
	module := "multisig"
	stage := seedPqOwnerStage(msName)
	type persisted struct {
		PrivateKey string `json:"privateKey"`
		PublicKey  string `json:"publicKey"`
		Address    string `json:"address"`
	}
	var rec persisted
	if v := st.Get(module, stage); v != nil {
		if raw, err := json.Marshal(v); err == nil && json.Unmarshal(raw, &rec) == nil && rec.PrivateKey != "" {
			sk, err := crypto.SecretKeyerFromStringRelaxed(rec.PrivateKey)
			if err != nil {
				return nil, crypto.PublicKey{}, crypto.Address{}, fmt.Errorf("pq owner %s privateKey: %w", msName, err)
			}
			pubRaw, err := hex.DecodeString(rec.PublicKey)
			if err != nil {
				return nil, crypto.PublicKey{}, crypto.Address{}, fmt.Errorf("pq owner %s publicKey hex: %w", msName, err)
			}
			pub := crypto.PublicKey{Variant: crypto.PublicKeyTypeFnDsa512, Bytes: pubRaw}
			adr, err := crypto.NewAddressFromRelaxed(rec.Address)
			if err != nil {
				return nil, crypto.PublicKey{}, crypto.Address{}, fmt.Errorf("pq owner %s address: %w", msName, err)
			}
			return sk, pub, *adr, nil
		}
	}
	sk, pub, err := crypto.NewFnDsa512SecretKey()
	if err != nil {
		return nil, crypto.PublicKey{}, crypto.Address{}, fmt.Errorf("NewFnDsa512SecretKey: %w", err)
	}
	adr, err := crypto.NewAddressFromPublicKey(pub)
	if err != nil {
		return nil, crypto.PublicKey{}, crypto.Address{}, fmt.Errorf("derive pq owner address: %w", err)
	}
	rec = persisted{PrivateKey: sk.ToHex(), PublicKey: hex.EncodeToString(pub.Bytes), Address: adr.ToBase58()}
	st.Mark(module, stage, rec)
	if err := st.Save(); err != nil {
		return nil, crypto.PublicKey{}, crypto.Address{}, fmt.Errorf("persist pq owner for %s: %w", msName, err)
	}
	return sk, *pub, *adr, nil
}

// seedEnsurePqSigners 保证 state 里有 n 个 msName 用的 PQ 签者：已有则复用，
// 不足则补齐并落盘（崩溃安全：每次补齐立即 Save）。
func seedEnsurePqSigners(st *SeedState, msName string, n int) ([]seedPqSigner, error) {
	module := "multisig"
	stage := seedPqSignersStage(msName)
	keys := decodeSeedPqSigners(st.Get(module, stage))
	if len(keys) > n {
		keys = keys[:n]
	}
	for len(keys) < n {
		k, err := seedGeneratePqSigner(fmt.Sprintf("%s-pq%d", msName, len(keys)+1))
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
		st.Mark(module, stage, keys)
		if err := st.Save(); err != nil {
			return nil, fmt.Errorf("persist pq signers for %s: %w", msName, err)
		}
	}
	return keys, nil
}

// seedGeneratePqSigner 随机生成一组 FN-DSA-512 密钥并派生链上地址。
func seedGeneratePqSigner(name string) (seedPqSigner, error) {
	sk, vk, err := crypto.KeyGen512()
	if err != nil {
		return seedPqSigner{}, fmt.Errorf("KeyGen512: %w", err)
	}
	pub := crypto.PublicKey{Variant: crypto.PublicKeyTypeFnDsa512, Bytes: append([]byte(nil), vk[:]...)}
	addr, err := crypto.NewAddressFromPublicKey(&pub)
	if err != nil {
		return seedPqSigner{}, fmt.Errorf("derive pq signer address: %w", err)
	}
	return seedPqSigner{
		Name:       name,
		PrivateKey: hex.EncodeToString(sk[:]),
		PublicKey:  hex.EncodeToString(vk[:]),
		Address:    addr.ToBase58(),
	}, nil
}

// decodeSeedPqSigners 把 state.Get 返回的 any（JSON 往返后的 []any of map）还原为密钥组。
func decodeSeedPqSigners(v any) []seedPqSigner {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var keys []seedPqSigner
	if json.Unmarshal(raw, &keys) != nil {
		return nil
	}
	return keys
}

// seedPqSignerPublicKeys 把密钥组转为 CreateMultisig 的 signers 公钥列表。
func seedPqSignerPublicKeys(keys []seedPqSigner) ([]crypto.PublicKey, error) {
	pubs := make([]crypto.PublicKey, 0, len(keys))
	for _, k := range keys {
		raw, err := hex.DecodeString(k.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("pq signer %s publicKey hex: %w", k.Name, err)
		}
		if len(raw) != crypto.FnDsa512VrfyKeyLen {
			return nil, fmt.Errorf("pq signer %s publicKey len = %d, want %d", k.Name, len(raw), crypto.FnDsa512VrfyKeyLen)
		}
		pubs = append(pubs, crypto.PublicKey{Variant: crypto.PublicKeyTypeFnDsa512, Bytes: append([]byte(nil), raw...)})
	}
	return pubs, nil
}
