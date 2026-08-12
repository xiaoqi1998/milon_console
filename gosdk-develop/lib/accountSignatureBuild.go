package lib

import (
	"fmt"
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
//	    AuthorizeIxAndPayer(0).
//	    Sign(owner, sk, txHash, nil, mode).
//	    Build()
//
//	// Path 2: auth → simulate sign
//	sig, err := NewAccountSignatureBuilder().
//	    AuthorizeIxAndPayer(0).
//	    SimulateSign(owner, mode).
//	    Build()
type AccountSignatureBuilder struct {
	acSig *AccountSignature
	errs  []error
}

// NewAccountSignatureBuilder creates a new AccountSignatureBuilder.
func NewAccountSignatureBuilder() *AccountSignatureBuilder {
	acSig := Unsigned(types.NewBitmap64(0))
	return &AccountSignatureBuilder{
		acSig: &acSig,
	}
}

// orAuthBit merges the given authorization bits into the current AuthBit.
func (b *AccountSignatureBuilder) orAuthBit(bits types.Bitmap64) {
	b.acSig.AuthBit |= bits
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
	b.orAuthBit(bits)
	return b
}

// AuthorizePayer adds payer authorization (bit63).
func (b *AccountSignatureBuilder) AuthorizePayer() *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	b.orAuthBit(AuthPayer())
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
	b.orAuthBit(bit)
	return b
}

// Sign computes the auth message and creates a real AccountSignature with the actual signature.
// ixHashes can be nil if no ix-specific hashes are needed (payer-only signature).
func (b *AccountSignatureBuilder) Sign(account crypto.Address, sk crypto.SecretKeyer, txHash api.TxHash, ixHashes []IxHashItem, mode AccountSignatureMode) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := Sign(account, sk, b.acSig.AuthBit, txHash, ixHashes, mode)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.acSig = sig
	return b
}

// SimulateSign creates a simulated AccountSignature without actual cryptographic signing.
func (b *AccountSignatureBuilder) SimulateSign(account crypto.Address, mode AccountSignatureMode) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	sig, err := SimulateSign(account, b.acSig.AuthBit, mode)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.acSig = sig
	return b
}

// SignMultisigKey signs the same auth message as the previous Sign with the given
// multisig participant key and appends it to the current signature.
func (b *AccountSignatureBuilder) SignMultisigKey(account crypto.Address, sk crypto.SecretKeyer, txHash api.TxHash, ixHashes []IxHashItem, mode AccountSignatureMode) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	index, publicKey, err := resolveMultisigMode(mode, "SignMultisigKey")
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}

	msg, err := b.acSig.AuthMessage(account, txHash, ixHashes)
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	signature, err := sk.SignFor(publicKey, msg[:])
	if err != nil {
		b.errs = append(b.errs, fmt.Errorf("failed to sign message: %w", err))
		return b
	}
	if err = b.acSig.AddMultisigKey(index, *signature); err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	return b
}

// SimulateSignMultisigKey appends a zero-filled placeholder signature for a
// multisig participant without real cryptographic signing, keeping the wire
// size identical to a real signature for accurate gas simulation.
// Only the mode is needed: the placeholder length is determined by the
// participant public key type.
func (b *AccountSignatureBuilder) SimulateSignMultisigKey(mode AccountSignatureMode) *AccountSignatureBuilder {
	if len(b.errs) > 0 {
		return b
	}
	index, publicKey, err := resolveMultisigMode(mode, "SimulateSignMultisigKey")
	if err != nil {
		b.errs = append(b.errs, err)
		return b
	}

	if err = b.acSig.AddMultisigKey(index, placeholderSignature(&publicKey)); err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	return b
}

// resolveMultisigMode validates that the mode is a MultisigKeySignatureMode and
// returns the signer index and public key.
func resolveMultisigMode(mode AccountSignatureMode, caller string) (uint8, crypto.PublicKey, error) {
	m, ok := mode.(MultisigKeySignatureMode)
	if !ok {
		return 0, crypto.PublicKey{}, fmt.Errorf("%s requires MultisigKeySignatureMode", caller)
	}
	if m.Index >= 64 {
		return 0, crypto.PublicKey{}, fmt.Errorf("multisig key index %d out of range (max 63)", m.Index)
	}
	return m.Index, m.PublicKey, nil
}

// Build finalizes and returns a copy of the AccountSignature, so later chain
// calls on the builder cannot mutate the built signature.
func (b *AccountSignatureBuilder) Build() (*AccountSignature, error) {
	if len(b.errs) > 0 {
		return nil, b.errs[0]
	}
	sig := *b.acSig
	sig.Signatures = append([]crypto.Signature(nil), b.acSig.Signatures...)
	return &sig, nil
}
