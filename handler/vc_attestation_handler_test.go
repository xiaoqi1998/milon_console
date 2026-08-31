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
