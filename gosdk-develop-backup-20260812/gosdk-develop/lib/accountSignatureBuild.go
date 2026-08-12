package lib

import (
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/types"
)

// **************************************** AccountSignatureBuilder ****************************************//

// AccountSignatureBuilder provides a fluent API for building AccountSignature.
//
// Usage:
//
//	// Path 1: auth → sign (real signing)
//	sig, err := NewAccountSignatureBuilder().
//	    AuthorizeIx(0).
//	    AuthorizePayer().
//	    Sign(owner, sk, txHash, nil, mode).
//	    Build()
//
//	// Path 2: auth → simulate sign
//	sig, err := NewAccountSignatureBuilder().
//	    AuthorizeIxAndPayer(0).
//	    SimulateSign(owner, mode).
//	    Build()
//
//	// Path 3: unsigned → add multisig keys
//	sig, err := NewAccountSignatureBuilder().
//	    FromUnsigned(authBit).
//	    AddMultisigKey(0, sig0).
//	    AddMultisigKey(1, sig1).
//	    Build()
type AccountSignatureBuilder struct {
	as   *AccountSignature
	errs []error
}

// NewAccountSignatureBuilder creates a new AccountSignatureBuilder.
func NewAccountSignatureBuilder() *AccountSignatureBuilder {
	return &AccountSignatureBuilder{
		as: &AccountSignature{
			AuthBit:    types.NewBitmap64(0),
			SigBit:     types.NewBitmap64(0),
			Signatures: []crypto.Signature{},
			PubKey:     nil,
		},
	}
}

// AuthorizeIx adds authorization for a single instruction at the given index.
func (b *AccountSignatureBuilder) AuthorizeIx(ix uint8) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	bit, err := AuthIx(ix)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.as.AuthBit = types.Bitmap64(uint64(b.as.AuthBit) | uint64(bit))
	return b
}

// AuthorizeIxes adds authorization for multiple instruction indices.
func (b *AccountSignatureBuilder) AuthorizeIxes(indices []uint8) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	bits, err := AuthIxes(indices)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.as.AuthBit = types.Bitmap64(uint64(b.as.AuthBit) | uint64(bits))
	return b
}

// AuthorizePayer adds payer authorization (bit63).
func (b *AccountSignatureBuilder) AuthorizePayer() *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.as.AuthBit = types.Bitmap64(uint64(b.as.AuthBit) | uint64(AuthPayer()))
	return b
}

// AuthorizeIxAndPayer adds authorization for both an instruction index and payer (bit63).
func (b *AccountSignatureBuilder) AuthorizeIxAndPayer(ix uint8) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	bit, err := AuthIxAndPayer(ix)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.as.AuthBit = types.Bitmap64(uint64(b.as.AuthBit) | uint64(bit))
	return b
}

// FromUnsigned starts from an unsigned AccountSignature with the given authBit.
// This is the entry point for building multisig signatures.
func (b *AccountSignatureBuilder) FromUnsigned(authBit types.Bitmap64) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	unsigned := Unsigned(authBit)
	b.as = &unsigned
	return b
}

// Sign computes the auth message and creates a real AccountSignature with the actual signature.
// ixHashes can be nil if no ix-specific hashes are needed (payer-only signature).
func (b *AccountSignatureBuilder) Sign(owner crypto.Address, sk crypto.SecretKeyer, txHash api.TxHash, ixHashes []IxHashItem, mode AccountSignatureMode) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := Sign(owner, sk, b.as.AuthBit, txHash, ixHashes, mode)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.as = sig
	return b
}

// SimulateSign creates a simulated AccountSignature without actual cryptographic signing.
func (b *AccountSignatureBuilder) SimulateSign(owner crypto.Address, mode AccountSignatureMode) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := SimulateSign(owner, b.as.AuthBit, mode)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.as = sig
	return b
}

// AddMultisigKey appends a multisig key signature to the current AccountSignature.
func (b *AccountSignatureBuilder) AddMultisigKey(keyIndex uint8, signature crypto.Signature) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	if err := b.as.AddMultisigKey(keyIndex, signature); err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	return b
}

// Build finalizes and returns the AccountSignature.
func (b *AccountSignatureBuilder) Build() (*AccountSignature, error) {
	if len(b.errs) > 0 {
		return nil, b.errs[0]
	}
	return b.as, nil
}

// MustBuild returns the AccountSignature or panics on error.
func (b *AccountSignatureBuilder) MustBuild() *AccountSignature {
	if len(b.errs) > 0 {
		panic(b.errs[0])
	}
	return b.as
}
