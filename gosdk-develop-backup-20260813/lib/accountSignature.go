package lib

import (
	"encoding/binary"
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/types"
)

const AuthPayerBit = 63
const AuthReservedBit = 62 // reserved, intentionally unused

// AccountSignatureMode is the interface for account signature modes.
type AccountSignatureMode interface {
	isAccountSignatureMode()
}

// PubKeySignatureMode signs with a single public key. When SkipPubKey is true,
// the public key is omitted from the wire format and SigBit must be resolved
// from the on-chain signers list (see Client.AccountSignerBit).
type PubKeySignatureMode struct {
	PublicKey  crypto.PublicKey
	SkipPubKey bool
	SigBit     types.Bitmap64
}

func (PubKeySignatureMode) isAccountSignatureMode() {}

// MultisigKeySignatureMode signs as one participant of a multisig account. The
// public key is located on-chain by SigBit = 1 << Index in the signers list.
type MultisigKeySignatureMode struct {
	Index     uint8
	PublicKey crypto.PublicKey
}

func (MultisigKeySignatureMode) isAccountSignatureMode() {}

type AccountSignature struct {
	AuthBit    types.Bitmap64     // ix/payer authorization bits (bit0-61 = ix, bit63 = payer)
	SigBit     types.Bitmap64     // signer slot bitmap: 0 in pubkey mode, 1<<Index in multisig mode
	Signatures []crypto.Signature // one signature per set SigBit (max 64)
	PubKey     *crypto.PublicKey  // set in pubkey mode (SigBit = 0); nil in multisig mode
}

// AddMultisigKey appends a multisig key signature under the same auth_bit and
// the same authorized message.
func (as *AccountSignature) AddMultisigKey(keyIndex uint8, signature crypto.Signature) error {
	if keyIndex >= 64 {
		return fmt.Errorf("key index %d out of range (max 63)", keyIndex)
	}
	if as.PubKey != nil {
		return fmt.Errorf("pubkey mode cannot add multisig keys")
	}

	as.SigBit = as.SigBit.Set(keyIndex)
	as.Signatures = append(as.Signatures, signature)
	return nil
}

// AuthorizesIx reports whether the given ix is authorized.
func (as *AccountSignature) AuthorizesIx(ix uint8) bool {
	return as.AuthBit.Test(ix)
}

// AuthorizesPayer reports whether the payer (bit63) is authorized.
func (as *AccountSignature) AuthorizesPayer() bool {
	return as.AuthBit.Test(AuthPayerBit)
}

// IxHashItem is an instruction hash paired with its ix index.
type IxHashItem struct {
	Index uint8
	Hash  api.TxHash
}

// AuthMessage assembles the account auth message:
// Blake3(MILON_ROOT || TX_AUTH_DOMAIN || chain_id || owner || auth_bit || tx_hash || ixHashes)
func (as *AccountSignature) AuthMessage(account crypto.Address, txHash api.TxHash, ixHashes []IxHashItem) (api.TxHash, error) {
	hasher := crypto.Hasher([]byte(crypto.MilonTxAuthDomainContext))

	chainIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chainIDBytes, GetChainId())
	hasher.Write(chainIDBytes)

	ownerBytes := account.AsBytes()
	hasher.Write(ownerBytes[:])

	authBitBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(authBitBytes, uint64(as.AuthBit))
	hasher.Write(authBitBytes)

	hasher.Write(txHash[:])

	for _, item := range ixHashes {
		if !as.AuthBit.Test(item.Index) {
			return api.TxHash{}, fmt.Errorf("ix index %d is not authorized in auth_bit", item.Index)
		}
		hasher.Write(item.Hash[:])
	}

	var result api.TxHash
	hasher.Sum(result[:0])
	return result, nil
}

// AuthMessageForTx assembles the auth message from a transaction context (verification).
func (as *AccountSignature) AuthMessageForTx(account crypto.Address, txHash api.TxHash, ixHashes []api.TxHash) (api.TxHash, error) {
	ixPart := CollectIxHashes(as.AuthBit, ixHashes)
	return as.AuthMessage(account, txHash, ixPart)
}

// IsVoteGateOnly reports whether the signature authorizes ix bits (bit0-61) but
// carries no actual signatures: no pubkey, no sig bits, no signature entries.
// The payer bit (bit63) is ignored. Used for vote gate scenarios.
func (as *AccountSignature) IsVoteGateOnly() bool {
	return as.PubKey == nil &&
		len(as.Signatures) == 0 &&
		as.SigBit.Raw() == 0 &&
		(as.AuthBit.Raw()&((uint64(1)<<AuthReservedBit)-1)) != 0
}

// MarshalPostcard implements the postcard.Marshaler interface.
func (as *AccountSignature) MarshalPostcard(serializer *postcard.Serializer) error {
	err := serializer.SerializeU64(as.AuthBit.Raw())
	if err != nil {
		return fmt.Errorf("failed to serialize AuthBit: %w", err)
	}

	err = serializer.SerializeU64(as.SigBit.Raw())
	if err != nil {
		return fmt.Errorf("failed to serialize SigBit: %w", err)
	}

	err = postcard.SerializeSeq(serializer, as.Signatures, func(s *postcard.Serializer, sig crypto.Signature) error {
		return sig.MarshalPostcard(s)
	})
	if err != nil {
		return fmt.Errorf("failed to serialize TxSigs: %w", err)
	}

	err = postcard.SerializeOption(serializer, as.PubKey, func(s *postcard.Serializer, pk crypto.PublicKey) error {
		return pk.MarshalPostcard(s)
	})
	if err != nil {
		return fmt.Errorf("failed to serialize PubKey: %w", err)
	}

	return nil
}

// UnmarshalPostcard implements the postcard.Unmarshaler interface.
func (as *AccountSignature) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	authBit, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize AuthBit: %w", err)
	}
	as.AuthBit = types.NewBitmap64(authBit)

	sigBit, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize SigBit: %w", err)
	}
	as.SigBit = types.NewBitmap64(sigBit)

	signatures, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (crypto.Signature, error) {
		var sig crypto.Signature
		if err = sig.UnmarshalPostcard(d); err != nil {
			return sig, err
		}
		return sig, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize TxSigs: %w", err)
	}
	as.Signatures = signatures

	pubKey, err := postcard.DeserializeOption(deserializer, func(d *postcard.Deserializer) (crypto.PublicKey, error) {
		var pk crypto.PublicKey
		if err = pk.UnmarshalPostcard(d); err != nil {
			return pk, err
		}
		return pk, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize PubKey: %w", err)
	}
	as.PubKey = pubKey

	return nil
}

// AuthIx creates the auth bitmap for a single ix (max ix = 61).
func AuthIx(ix uint8) (types.Bitmap64, error) {
	if ix >= AuthReservedBit {
		return types.NewBitmap64(0), fmt.Errorf("ix index %d out of range (max %d)", ix, AuthReservedBit-1)
	}
	return types.NewBitmap64(uint64(1) << ix), nil
}

// AuthIxes creates the auth bitmap for multiple ixs (max ix = 61).
func AuthIxes(indices []uint8) (types.Bitmap64, error) {
	var raw uint64
	for _, ix := range indices {
		if ix >= AuthReservedBit {
			return types.NewBitmap64(0), fmt.Errorf("ix index %d out of range (max %d)", ix, AuthReservedBit-1)
		}
		raw |= uint64(1) << ix
	}
	return types.NewBitmap64(raw), nil
}

// AuthPayer creates the payer auth bitmap (bit63).
func AuthPayer() types.Bitmap64 {
	return types.NewBitmap64(uint64(1) << AuthPayerBit)
}

// AuthIxAndPayer creates the combined ix + payer auth bitmap.
func AuthIxAndPayer(ix uint8) (types.Bitmap64, error) {
	if ix >= AuthReservedBit {
		return types.NewBitmap64(0), fmt.Errorf("ix index %d out of range (max %d)", ix, AuthReservedBit-1)
	}
	return types.NewBitmap64(uint64(1<<ix) | (1 << AuthPayerBit)), nil
}

// Unsigned creates an unsigned AccountSignature with only the auth_bit set.
func Unsigned(authBit types.Bitmap64) AccountSignature {
	return AccountSignature{
		AuthBit:    authBit,
		SigBit:     types.NewBitmap64(0),
		Signatures: []crypto.Signature{},
		PubKey:     nil,
	}
}

// resolveMode validates the signature mode against the owner and derives the
// signing public key, sigBit and PubKey field for the AccountSignature.
func resolveMode(account crypto.Address, mode AccountSignatureMode) (crypto.PublicKey, types.Bitmap64, *crypto.PublicKey, error) {
	switch m := mode.(type) {
	case PubKeySignatureMode:
		pkAddr, err := crypto.NewAddressFromPublicKey(&m.PublicKey)
		if err != nil {
			return crypto.PublicKey{}, types.NewBitmap64(0), nil, fmt.Errorf("failed to get address from public key: %w", err)
		}
		if *pkAddr != account {
			return crypto.PublicKey{}, types.NewBitmap64(0), nil, fmt.Errorf("public key does not match owner address")
		}
		if m.SkipPubKey {
			if m.SigBit.Raw() == 0 {
				return crypto.PublicKey{}, types.NewBitmap64(0), nil, fmt.Errorf("SkipPubKey requires SigBit resolved from the on-chain signers list")
			}
			return m.PublicKey, m.SigBit, nil, nil
		}
		return m.PublicKey, types.NewBitmap64(0), &m.PublicKey, nil
	case MultisigKeySignatureMode:
		if m.Index >= 64 {
			return crypto.PublicKey{}, types.NewBitmap64(0), nil, fmt.Errorf("multisig key index %d out of range (max 63)", m.Index)
		}
		return m.PublicKey, types.NewBitmap64(uint64(1) << m.Index), nil, nil
	default:
		return crypto.PublicKey{}, types.NewBitmap64(0), nil, fmt.Errorf("invalid signature mode")
	}
}

// Sign computes the auth message for the given auth context and signs it.
func Sign(account crypto.Address, sk crypto.SecretKeyer, authBit types.Bitmap64, txHash api.TxHash, ixHashes []IxHashItem, mode AccountSignatureMode) (*AccountSignature, error) {
	publicKey, sigBit, pubKeyField, err := resolveMode(account, mode)
	if err != nil {
		return nil, err
	}

	accountSignature := AccountSignature{
		AuthBit:    authBit,
		SigBit:     sigBit,
		Signatures: []crypto.Signature{},
		PubKey:     pubKeyField,
	}

	authHash, err := accountSignature.AuthMessage(account, txHash, ixHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to compute auth message: %w", err)
	}

	signature, err := sk.SignFor(publicKey, authHash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	accountSignature.Signatures = []crypto.Signature{*signature}
	return &accountSignature, nil
}

// SimulateSign computes the auth context without signing: a zero-filled
// placeholder signature with the same length as a real one is attached, keeping
// the wire size identical for accurate gas simulation.
func SimulateSign(account crypto.Address, authBit types.Bitmap64, mode AccountSignatureMode) (*AccountSignature, error) {
	publicKey, sigBit, pubKeyField, err := resolveMode(account, mode)
	if err != nil {
		return nil, err
	}

	return &AccountSignature{
		AuthBit:    authBit,
		SigBit:     sigBit,
		Signatures: []crypto.Signature{placeholderSignature(&publicKey)},
		PubKey:     pubKeyField,
	}, nil
}

// placeholderSignature returns a zero-filled signature whose length matches the
// public key type, used for gas simulation without real cryptographic signing.
func placeholderSignature(pk *crypto.PublicKey) crypto.Signature {
	return crypto.Signature{
		Variant: crypto.SignatureType(pk.Variant),
		Bytes:   make([]byte, signatureSizeForPublicKey(pk)),
	}
}
func signatureSizeForPublicKey(pk *crypto.PublicKey) int {
	switch pk.Variant {
	case crypto.PublicKeyTypeSecp256k1:
		return crypto.SignatureSecp256k1Size
	case crypto.PublicKeyTypeEd25519:
		return crypto.SignatureEd25519Size
	case crypto.PublicKeyTypeBLS12381:
		return crypto.SignatureBLS12381Size
	case crypto.PublicKeyTypeFnDsa512:
		return crypto.SignatureFnDsa512Size
	default:
		return 0
	}
}

// CollectIxHashes collects the IxHashItems for the ix bits set in auth_bit.
func CollectIxHashes(authBit types.Bitmap64, ixHashes []api.TxHash) []IxHashItem {
	var out []IxHashItem
	for i := uint8(0); i < AuthReservedBit; i++ {
		if !authBit.Test(i) {
			continue
		}
		if int(i) < len(ixHashes) {
			out = append(out, IxHashItem{Index: i, Hash: ixHashes[i]})
		}
	}
	return out
}
