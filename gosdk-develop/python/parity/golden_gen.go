// 跨语言 parity 黄金向量生成器（Go 侧基准，供 Python 对拍）
// 用法：在 gosdk-develop 目录下执行 go run ../python/parity/golden_gen.go
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

// fixedSeed 固定 32 字节 ed25519 种子（确定性）
var fixedSeed = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

func main() {
	// 绑定 gen 应用（仅本地加载 IDL，不发起网络请求）
	_ = milon.NewClient(milon.DevNet)

	out := map[string]any{}

	// ---- 1. postcard 变长整数黄金向量 ----
	{
		varints := []uint64{0, 127, 128, 2581, 16384, 4294967295, 900000001}
		encoded := map[string]string{}
		for _, v := range varints {
			s := postcard.NewSerializer()
			_ = s.SerializeU64(v)
			encoded[fmt.Sprintf("%d", v)] = hex.EncodeToString(s.Bytes())
		}
		out["postcard_varints"] = encoded
	}

	// ---- 2. 地址派生 ----
	{
		sk := &crypto.ClassicalSecretKey{}
		_ = sk.FromBytes(fixedSeed)
		pk := sk.Ed25519Public()
		addr, _ := crypto.NewAddressFromPublicKey(pk)
		out["ed25519_pk"] = pk.ToBase58()
		out["ed25519_pk_hex"] = hex.EncodeToString(pk.Bytes)
		out["address_base58"] = addr.ToBase58()
		out["address_hex"] = addr.ToHex()
	}

	// ---- 3. 指令 wire（gen.Token.BalanceOf.Args）----
	{
		sk := &crypto.ClassicalSecretKey{}
		_ = sk.FromBytes(fixedSeed)
		pk := sk.Ed25519Public()
		addr, _ := crypto.NewAddressFromPublicKey(pk)
		wire, err := gen.Token.BalanceOf.Args(api.MILToken, addr).Encode()
		if err != nil {
			panic(err)
		}
		out["balance_of_wire"] = hex.EncodeToString(wire)
	}

	// ---- 4. IxHash / TxHash（确定性）----
	{
		sk := &crypto.ClassicalSecretKey{}
		_ = sk.FromBytes(fixedSeed)
		pk := sk.Ed25519Public()
		addr, _ := crypto.NewAddressFromPublicKey(pk)
		wire, _ := gen.Token.BalanceOf.Args(api.MILToken, addr).Encode()

		tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
			WithStamp(1700000000000).
			AddIxesSig(*addr, sk, []uint8{0}, false, lib.PubKeySignatureMode{PublicKey: *pk}).
			Build()
		if err != nil {
			panic(err)
		}
		out["tx_stamp"] = uint64(tx.Stamp)
		out["ix_hash"] = tx.IxHashFromWire(wire).ToBase58()
		out["ix_hash_hex"] = tx.IxHashFromWire(wire).ToHex()
		out["tx_hash"] = tx.TxHash().ToBase58()
		out["tx_hash_hex"] = tx.TxHash().ToHex()
		txBytes, _ := tx.ToBytes()
		out["tx_bytes_hex"] = hex.EncodeToString(txBytes)
		sig := tx.TxSigs[0].AccountSignature
		out["sig_base58"] = sig.Signatures[0].ToBase58()
		out["sig_hex"] = hex.EncodeToString(sig.Signatures[0].Bytes)
		out["auth_bit"] = fmt.Sprintf("%d", sig.AuthBit.Raw())
	}

	// ---- 5. AccountSignature / PublicKey postcard ----
	{
		sk := &crypto.ClassicalSecretKey{}
		_ = sk.FromBytes(fixedSeed)
		pk := sk.Ed25519Public()
		s := postcard.NewSerializer()
		_ = pk.MarshalPostcard(s)
		out["pubkey_postcard_hex"] = hex.EncodeToString(s.Bytes())
	}

	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}
