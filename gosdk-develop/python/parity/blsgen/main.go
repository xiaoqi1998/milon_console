// 生成 Go blst BLS12-381 黄金向量（供 Python py_ecc 字节兼容验证）。
// 运行：go run ./python/parity/blsgen
package main

import (
	"encoding/hex"
	"fmt"

	blst "github.com/supranational/blst/bindings/go"
)

func main() {
	seed := []byte("0123456789abcdef0123456789abcdef")
	msg := []byte("hello milon bls")

	sk := blst.KeyGen(seed)
	pk := new(blst.P1Affine).From(sk).Compress()
	sig := new(blst.P2Affine).Sign(sk, msg, nil).Compress()

	p := new(blst.P1Affine).Uncompress(pk)
	s := new(blst.P2Affine).Uncompress(sig)
	ok := s.Verify(true, p, true, msg, nil)

	// H(msg)：blst 默认 DST 的 hash-to-curve 压缩点（RFC 9380）
	hm := blst.HashToG2(msg, nil).Compress()
	// 候选 DST
	hm_empty := blst.HashToG2(msg, []byte{}).Compress()
	hm_nul := blst.HashToG2(msg, []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_NUL_")).Compress()
	hm_pop := blst.HashToG2(msg, []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_")).Compress()

	fmt.Printf("seed=%s\n", hex.EncodeToString(seed))
	fmt.Printf("msg=%s\n", hex.EncodeToString(msg))
	fmt.Printf("pk=%s\n", hex.EncodeToString(pk))
	fmt.Printf("sig=%s\n", hex.EncodeToString(sig))
	fmt.Printf("hm=%s\n", hex.EncodeToString(hm))
	fmt.Printf("hm_nil_eq_empty=%v\n", hex.EncodeToString(hm) == hex.EncodeToString(hm_empty))
	fmt.Printf("hm_nil_eq_nul=%v\n", hex.EncodeToString(hm) == hex.EncodeToString(hm_nul))
	fmt.Printf("hm_nil_eq_pop=%v\n", hex.EncodeToString(hm) == hex.EncodeToString(hm_pop))
	fmt.Printf("verify=%v\n", ok)
}
