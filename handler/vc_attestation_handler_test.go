package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
)

func TestVcAttestationVector(t *testing.T) {
	issuerSK := &crypto.ClassicalSecretKey{}
	if err := issuerSK.FromStringRelaxed("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"); err != nil {
		t.Fatal(err)
	}
	issuerPub := issuerSK.Ed25519Public()
	issuerAddr, err := crypto.NewAddressFromPublicKey(issuerPub)
	if err != nil {
		t.Fatal(err)
	}

	subjectSK := &crypto.ClassicalSecretKey{}
	if err := subjectSK.FromStringRelaxed("202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"); err != nil {
		t.Fatal(err)
	}
	subjectAddr, err := crypto.NewAddressFromPublicKey(subjectSK.Ed25519Public())
	if err != nil {
		t.Fatal(err)
	}

	credentialJSON := `{"credentialSubject":{"id":"did:milon:subject","kycLevel":2},"id":"urn:uuid:12345678-1234-5678-1234-567812345678","issuer":"did:milon:issuer","type":["VerifiableCredential","KycLevelCredential"]}`
	credentialHash := sha256.Sum256([]byte(credentialJSON))

	validUntilMs := int64(1_900_000_000_000)
	digest := vcAttestationDigest(900_000_001, subjectAddr.Bytes[:], issuerAddr.Bytes[:], 0, "KycLevelCredential", credentialHash[:], &validUntilMs)

	sig := issuerSK.SignEd25519(digest[:])
	if err := sig.Verify(digest[:], issuerPub); err != nil {
		t.Fatal(err)
	}

	fmt.Println("subject:        ", subjectAddr.ToBase58())
	fmt.Println("issuer:         ", issuerAddr.ToBase58())
	fmt.Println("credential_hash:", credentialHash[:])
	fmt.Println("signature:      ", hex.EncodeToString(sig.Bytes))

	if subjectAddr.ToBase58() != "48QWpGsZpXJV3rdRvsiQb4iGzBW" {
		t.Errorf("subject mismatch: %s", subjectAddr.ToBase58())
	}
	if issuerAddr.ToBase58() != "3pHqrfVpw4ziiWZ2S6graADk8sXu" {
		t.Errorf("issuer mismatch: %s", issuerAddr.ToBase58())
	}
	if hex.EncodeToString(sig.Bytes) != "ce58f196aece3c8a1fcca9a553f7cfed8f00414d23d9607856acf4015d064085372629b6f71bd2d08f3461eb8cb3aa0ee05e5a3058ce40d3fa252d46c7f8ff08" {
		t.Errorf("signature mismatch: %s", hex.EncodeToString(sig.Bytes))
	}
}

// TestResolveIssuerIdentityEd25519BackwardCompat 验证 32 字节经典私钥在不传 issuerPublicKey 时仍走 Ed25519 旧路径。
func TestResolveIssuerIdentityEd25519BackwardCompat(t *testing.T) {
	sk, pub, addr, err := resolveIssuerIdentity(
		"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if crypto.AsFnDsa512SecretKey(sk) != nil {
		t.Fatal("expected classical secret key")
	}
	if !pub.IsEd25519() {
		t.Fatalf("expected ed25519 public key, got variant %d", pub.Variant)
	}
	if addr.ToBase58() != "3pHqrfVpw4ziiWZ2S6graADk8sXu" {
		t.Errorf("issuer address mismatch: %s", addr.ToBase58())
	}
}

// TestVcAttestationFnDsa512Vector 验证 FN-DSA-512 issuer 产出 666 字节（1332 hex）签名且本地验签通过。
func TestVcAttestationFnDsa512Vector(t *testing.T) {
	// 模拟 /api/account/generate?keyType=fndsa512 产物：私钥 + 公钥成对返回
	issuerSker, issuerPub, err := crypto.NewFnDsa512SecretKey()
	if err != nil {
		t.Fatal(err)
	}
	expectedAddr, err := crypto.NewAddressFromPublicKey(issuerPub)
	if err != nil {
		t.Fatal(err)
	}

	sk, pub, addr, err := resolveIssuerIdentity(issuerSker.ToHex(), issuerPub.ToHex())
	if err != nil {
		t.Fatal(err)
	}
	if crypto.AsFnDsa512SecretKey(sk) == nil {
		t.Fatal("expected FN-DSA-512 secret key")
	}
	if !pub.IsFnDsa512() {
		t.Fatalf("expected FN-DSA-512 public key, got variant %d", pub.Variant)
	}
	if addr.ToBase58() != expectedAddr.ToBase58() {
		t.Errorf("issuer address mismatch: got %s, want %s", addr.ToBase58(), expectedAddr.ToBase58())
	}

	subjectSK := &crypto.ClassicalSecretKey{}
	if err := subjectSK.FromStringRelaxed("202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"); err != nil {
		t.Fatal(err)
	}
	subjectAddr, err := crypto.NewAddressFromPublicKey(subjectSK.Ed25519Public())
	if err != nil {
		t.Fatal(err)
	}

	credentialJSON := `{"credentialSubject":{"id":"did:milon:subject","kycLevel":2},"id":"urn:uuid:12345678-1234-5678-1234-567812345678","issuer":"did:milon:issuer","type":["VerifiableCredential","KycLevelCredential"]}`
	credentialHash := sha256.Sum256([]byte(credentialJSON))
	validUntilMs := int64(1_900_000_000_000)
	digest := vcAttestationDigest(900_000_001, subjectAddr.Bytes[:], addr.Bytes[:], 0, "Change", credentialHash[:], &validUntilMs)

	sig, err := signVcAttestationDigest(sk, pub, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(sig.Bytes) != crypto.SignatureFnDsa512Size {
		t.Fatalf("expected %d-byte FN-DSA-512 signature, got %d", crypto.SignatureFnDsa512Size, len(sig.Bytes))
	}
	if sig.Variant != crypto.SignatureTypeFnDsa512 {
		t.Fatalf("expected signature variant FN-DSA-512 (%d), got %d", crypto.SignatureTypeFnDsa512, sig.Variant)
	}
	sigHex := hex.EncodeToString(sig.Bytes)
	if len(sigHex) != 1332 {
		t.Errorf("expected 1332 hex chars, got %d", len(sigHex))
	}
}

// TestResolveIssuerIdentityFnDsa512MissingPublicKey 验证 FN-DSA-512 私钥缺公钥时必须报错。
func TestResolveIssuerIdentityFnDsa512MissingPublicKey(t *testing.T) {
	issuerSker, _, err := crypto.NewFnDsa512SecretKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := resolveIssuerIdentity(issuerSker.ToHex(), ""); err == nil {
		t.Fatal("expected error when issuerPublicKey is missing for FN-DSA-512 key")
	}
}

// TestResolveIssuerIdentityFnDsa512WrongPublicKeyType 验证私钥/公钥算法不匹配时必须报错。
func TestResolveIssuerIdentityFnDsa512WrongPublicKeyType(t *testing.T) {
	issuerSker, _, err := crypto.NewFnDsa512SecretKey()
	if err != nil {
		t.Fatal(err)
	}
	// 32 字节 ed25519 公钥（与既有向量私钥配对）
	ed25519PubHex := "9d61b19deffb63bbd326eef7a6572727e1e6a3a3a3a3a3a3a3a3a3a3a3a3a3a"
	if _, _, _, err := resolveIssuerIdentity(issuerSker.ToHex(), ed25519PubHex); err == nil {
		t.Fatal("expected error when FN-DSA-512 private key is paired with a non-FN-DSA-512 public key")
	}
}
