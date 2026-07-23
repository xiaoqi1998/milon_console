package milon

import (
	"bytes"
	"github.com/milon-labs/milon-go-sdk/api"
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/types"
	"github.com/stretchr/testify/assert"
)

func TestNewAccountSignatureWithPubKey(t *testing.T) {
	signature := crypto.Signature{
		Variant: crypto.SignatureTypeEd25519,
		Bytes:   make([]byte, 64),
	}

	pubKey := crypto.PublicKey{
		Variant: crypto.PublicKeyTypeEd25519,
		Bytes:   make([]byte, 32),
	}

	accountSig := NewAccountSignatureWithPubKey(signature, pubKey)

	assert.Equal(t, uint64(0), accountSig.AuthBit.Raw())
	assert.Equal(t, uint64(0), accountSig.SigBit.Raw())
	assert.Len(t, accountSig.Signatures, 1)
	assert.True(t, bytes.Equal(accountSig.PubKey.Bytes, pubKey.Bytes))
}

func TestNewAccountSignature(t *testing.T) {
	t.Run("valid index", func(t *testing.T) {
		signature := crypto.Signature{
			Variant: crypto.SignatureTypeEd25519,
			Bytes:   make([]byte, 64),
		}

		accountSig, err := NewAccountSignature(5, signature)
		assert.NoError(t, err)

		assert.Equal(t, uint64(0), accountSig.AuthBit.Raw())
		assert.Equal(t, uint64(1)<<5, accountSig.SigBit.Raw())
		assert.Len(t, accountSig.Signatures, 1)
		assert.Nil(t, accountSig.PubKey)
	})

	t.Run("max valid index", func(t *testing.T) {
		signature := crypto.Signature{
			Variant: crypto.SignatureTypeEd25519,
			Bytes:   make([]byte, 64),
		}

		accountSig, err := NewAccountSignature(AuthPayerBit, signature)
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), accountSig.AuthBit.Raw())
		assert.Equal(t, uint64(1)<<AuthPayerBit, accountSig.SigBit.Raw())
		assert.Len(t, accountSig.Signatures, 1)
		assert.Nil(t, accountSig.PubKey)
	})

	t.Run("invalid index out of range", func(t *testing.T) {
		signature := crypto.Signature{
			Variant: crypto.SignatureTypeEd25519,
			Bytes:   make([]byte, 64),
		}

		_, err := NewAccountSignature(AuthPayerBit+1, signature)
		assert.Error(t, err)
	})
}

func TestAccountSignature_Add(t *testing.T) {
	t.Run("add signature successfully", func(t *testing.T) {
		signature1 := crypto.Signature{
			Variant: crypto.SignatureTypeEd25519,
			Bytes:   []byte("sig1"),
		}
		signature2 := crypto.Signature{
			Variant: crypto.SignatureTypeEd25519,
			Bytes:   []byte("sig2"),
		}

		var sig1Bit uint8 = 0
		accountSig, err := NewAccountSignature(sig1Bit, signature1)
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), accountSig.AuthBit.Raw())
		assert.Equal(t, uint64(1)<<sig1Bit, accountSig.SigBit.Raw())
		assert.Len(t, accountSig.Signatures, 1)
		assert.Nil(t, accountSig.PubKey)

		var sig2Bit uint8 = 1
		err = accountSig.Add(sig2Bit, signature2)
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), accountSig.AuthBit.Raw())
		assert.Equal(t, uint64(1)<<sig1Bit|uint64(1)<<sig2Bit, accountSig.SigBit.Raw())
		assert.Len(t, accountSig.Signatures, 2)
		assert.Nil(t, accountSig.PubKey)
	})

	t.Run("add with invalid index", func(t *testing.T) {
		signature := crypto.Signature{
			Variant: crypto.SignatureTypeEd25519,
			Bytes:   []byte("sig"),
		}

		accountSig, err := NewAccountSignature(0, signature)
		assert.NoError(t, err)

		err = accountSig.Add(64, signature)
		assert.Error(t, err)
	})

	t.Run("add when pubkey is set", func(t *testing.T) {
		signature := crypto.Signature{
			Variant: crypto.SignatureTypeEd25519,
			Bytes:   []byte("sig"),
		}

		pubKey := crypto.PublicKey{
			Variant: crypto.PublicKeyTypeEd25519,
			Bytes:   make([]byte, 32),
		}

		accountSig := NewAccountSignatureWithPubKey(signature, pubKey)

		err := accountSig.Add(1, signature)
		assert.Error(t, err)
	})
}

func TestAccountSignature_MarshalUnmarshalPostcard(t *testing.T) {
	t.Run("with multiple signatures and no pubkey", func(t *testing.T) {
		sig1Bytes := make([]byte, 64)
		copy(sig1Bytes, "signature1_data_here")
		sig1 := crypto.Signature{
			Variant: crypto.SignatureTypeEd25519,
			Bytes:   sig1Bytes,
		}

		sig2Bytes := make([]byte, 65)
		copy(sig2Bytes, "signature2_data_here")
		sig2 := crypto.Signature{
			Variant: crypto.SignatureTypeSecp256k1,
			Bytes:   sig2Bytes,
		}

		original := AccountSignature{
			AuthBit:    types.NewBitmap64(0),
			SigBit:     types.NewBitmap64(0x0000000000000003),
			Signatures: []crypto.Signature{sig1, sig2},
			PubKey:     nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (AccountSignature, error) {
			var as AccountSignature
			unmarshalErr := as.UnmarshalPostcard(d)
			return as, unmarshalErr
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.AuthBit.Raw(), deserialized.AuthBit.Raw())
		assert.Equal(t, original.SigBit.Raw(), deserialized.SigBit.Raw())
		assert.Len(t, deserialized.Signatures, len(original.Signatures))

		for i := range original.Signatures {
			assert.Equal(t, original.Signatures[i].Variant, deserialized.Signatures[i].Variant, i)
			assert.True(t, bytes.Equal(original.Signatures[i].Bytes, deserialized.Signatures[i].Bytes), i)
		}

		assert.Nil(t, deserialized.PubKey)
	})

	t.Run("with pubkey set", func(t *testing.T) {
		signatureBytes := make([]byte, 64)
		copy(signatureBytes, "test_signature_data")
		signature := crypto.Signature{
			Variant: crypto.SignatureTypeEd25519,
			Bytes:   signatureBytes,
		}

		pubKeyBytes := make([]byte, 32)
		copy(pubKeyBytes, "test_pubkey_data__")
		pubKey := crypto.PublicKey{
			Variant: crypto.PublicKeyTypeEd25519,
			Bytes:   pubKeyBytes,
		}

		original := NewAccountSignatureWithPubKey(signature, pubKey)

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (AccountSignature, error) {
			var as AccountSignature
			unmarshalErr := as.UnmarshalPostcard(d)
			return as, unmarshalErr
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.AuthBit.Raw(), deserialized.AuthBit.Raw())
		assert.Equal(t, original.SigBit.Raw(), deserialized.SigBit.Raw())
		assert.Len(t, deserialized.Signatures, len(original.Signatures))

		assert.NotNil(t, deserialized.PubKey)
		assert.Equal(t, original.PubKey.Variant, deserialized.PubKey.Variant)
		assert.True(t, bytes.Equal(original.PubKey.Bytes, deserialized.PubKey.Bytes))
	})

	t.Run("empty signatures list", func(t *testing.T) {
		original := AccountSignature{
			AuthBit:    types.NewBitmap64(0),
			SigBit:     types.NewBitmap64(0),
			Signatures: []crypto.Signature{},
			PubKey:     nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (AccountSignature, error) {
			var as AccountSignature
			unmarshalErr := as.UnmarshalPostcard(d)
			return as, unmarshalErr
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.AuthBit.Raw(), deserialized.AuthBit.Raw())
		assert.Equal(t, original.SigBit.Raw(), deserialized.SigBit.Raw())
		assert.Empty(t, deserialized.Signatures)
		assert.Nil(t, deserialized.PubKey)
	})
}

func TestAccountSignature_AuthorizesIx(t *testing.T) {
	t.Run("single authorized ix", func(t *testing.T) {
		accountSig := Unsigned(types.NewBitmap64(uint64(1) << 5))

		assert.True(t, accountSig.AuthorizesIx(5))
		assert.False(t, accountSig.AuthorizesIx(6))
	})

	t.Run("multiple authorized ix", func(t *testing.T) {
		accountSig := Unsigned(types.NewBitmap64(uint64(1)<<0 | uint64(1)<<1 | uint64(1)<<2))

		assert.True(t, accountSig.AuthorizesIx(0))
		assert.True(t, accountSig.AuthorizesIx(1))
		assert.True(t, accountSig.AuthorizesIx(2))
		assert.False(t, accountSig.AuthorizesIx(3))
	})
}

func TestAccountSignature_AuthorizesPayer(t *testing.T) {
	t.Run("payer authorized returns true", func(t *testing.T) {
		accountSig := Unsigned(types.NewBitmap64(uint64(1) << AuthPayerBit))
		assert.True(t, accountSig.AuthorizesPayer())
	})

	t.Run("payer not authorized returns false", func(t *testing.T) {
		accountSig := Unsigned(types.NewBitmap64(uint64(1) << 0))
		assert.False(t, accountSig.AuthorizesPayer())
	})

	t.Run("both ix and payer authorized", func(t *testing.T) {
		accountSig := Unsigned(types.NewBitmap64(uint64(1)<<3 | uint64(1)<<AuthPayerBit))

		assert.True(t, accountSig.AuthorizesIx(3))
		assert.True(t, accountSig.AuthorizesIx(AuthPayerBit))
		assert.True(t, accountSig.AuthorizesPayer())
	})
}

func TestAccountSignature_AuthMessage(t *testing.T) {
	t.Run("auth message with ix only", func(t *testing.T) {
		classicalSk := crypto.AsClassicalSecretKey(crypto.NewClassicalSecretKey())
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		var ixHash1, ixHash2 api.TxHash
		copy(ixHash1[:], "test_ix_hash_1_12345678901234567")
		copy(ixHash2[:], "test_ix_hash_2_12345678901234567")

		authBit := types.NewBitmap64(uint64(1)<<2 | uint64(1)<<4)
		accountSig := Unsigned(authBit)

		ixPart := []IxHashItem{
			{Index: 2, Hash: ixHash1},
			{Index: 4, Hash: ixHash2},
		}

		msg, err := accountSig.AuthMessage(*owner, api.TxHash{}, ixPart)
		assert.NoError(t, err)
		assert.NotEqual(t, api.TxHash{}, msg)
	})

	t.Run("auth message with ix and payer", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		var txHash api.TxHash
		copy(txHash[:], "test_tx_hash_1234567890123456789")

		var ixHash api.TxHash
		copy(ixHash[:], "test_ix_hash_1_12345678901234567")

		authBit := types.NewBitmap64(uint64(1)<<2 | uint64(1)<<AuthPayerBit)
		accountSig := Unsigned(authBit)

		ixPart := []IxHashItem{
			{Index: 2, Hash: ixHash}, // Index 必须是 2，与 authBit 对应
		}

		msg, err := accountSig.AuthMessage(*owner, txHash, ixPart)
		assert.NoError(t, err)
		assert.NotEqual(t, api.TxHash{}, msg)
	})

	t.Run("auth message different with different tx hash", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		var txHash1, txHash2 api.TxHash
		copy(txHash1[:], "test_tx_hash_1_1234567890123456")
		copy(txHash2[:], "test_tx_hash_2_1234567890123456")

		var ixHash api.TxHash
		copy(ixHash[:], "test_ix_hash_1_12345678901234567")

		authBit := types.NewBitmap64(uint64(1)<<2 | uint64(1)<<AuthPayerBit)
		accountSig := Unsigned(authBit)

		ixPart := []IxHashItem{
			{Index: 2, Hash: ixHash}, // Index 必须是 2，与 authBit 对应
		}

		msg1, err := accountSig.AuthMessage(*owner, txHash1, ixPart)
		assert.NoError(t, err)

		msg2, err := accountSig.AuthMessage(*owner, txHash2, ixPart)
		assert.NoError(t, err)

		assert.NotEqual(t, msg1, msg2)
	})
}

func TestAuthIx(t *testing.T) {
	t.Run("valid ix index", func(t *testing.T) {
		bitmap, err := AuthIx(0)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0, bitmap.Raw())

		bitmap, err = AuthIx(5)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<5, bitmap.Raw())

		bitmap, err = AuthIx(62)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<62, bitmap.Raw())
	})

	t.Run("invalid ix index", func(t *testing.T) {
		_, err := AuthIx(AuthPayerBit)
		assert.Error(t, err)
	})
}

func TestAuthIxes(t *testing.T) {
	t.Run("single ix", func(t *testing.T) {
		bitmap, err := AuthIxes([]uint8{5})
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<5, bitmap.Raw())
	})

	t.Run("multiple ixes", func(t *testing.T) {
		bitmap, err := AuthIxes([]uint8{0, 5, 10, 15})
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0|uint64(1)<<5|uint64(1)<<10|uint64(1)<<15, bitmap.Raw())
	})

	t.Run("invalid ix in list", func(t *testing.T) {
		_, err := AuthIxes([]uint8{0, 5, AuthPayerBit})
		assert.Error(t, err)
	})

	t.Run("empty list", func(t *testing.T) {
		bitmap, err := AuthIxes([]uint8{})
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), bitmap.Raw())
	})
}

func TestAuthPayer(t *testing.T) {
	bitmap := AuthPayer()
	assert.Equal(t, uint64(1)<<AuthPayerBit, bitmap.Raw())
}

func TestAuthIxAndPayer(t *testing.T) {
	t.Run("valid ix index", func(t *testing.T) {
		bitmap, err := AuthIxAndPayer(0)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<0|uint64(1)<<AuthPayerBit, bitmap.Raw())

		bitmap, err = AuthIxAndPayer(5)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1)<<5|uint64(1)<<AuthPayerBit, bitmap.Raw())
	})

	t.Run("invalid ix index", func(t *testing.T) {
		_, err := AuthIxAndPayer(AuthPayerBit)
		assert.Error(t, err)

		_, err = AuthIxAndPayer(100)
		assert.Error(t, err)
	})
}

func TestUnsigned(t *testing.T) {
	authBit := types.NewBitmap64(uint64(1)<<5 | uint64(1)<<AuthPayerBit)
	accountSig := Unsigned(authBit)

	assert.Equal(t, authBit.Raw(), accountSig.AuthBit.Raw())
	assert.Equal(t, uint64(0), accountSig.SigBit.Raw())
	assert.Empty(t, accountSig.Signatures)
	assert.Nil(t, accountSig.PubKey)
}

func TestSign(t *testing.T) {
	t.Run("sign ix only with pubkey mode", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		authBit := types.NewBitmap64(uint64(1) << 3)

		var ixHash api.TxHash
		copy(ixHash[:], "test_ix_hash_1_12345678901234567")

		ixPart := []IxHashItem{
			{Index: 3, Hash: ixHash},
		}

		accountSig, err := Sign(*owner, classicalSk, authBit, api.TxHash{}, ixPart, PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		assert.NotNil(t, accountSig)

		assert.Equal(t, authBit.Raw(), accountSig.AuthBit.Raw())
		assert.Equal(t, uint64(0), accountSig.SigBit.Raw())
		assert.Len(t, accountSig.Signatures, 1)
		assert.NotNil(t, accountSig.PubKey)
	})

	t.Run("sign ix and payer with pubkey mode", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		authBit := types.NewBitmap64(uint64(1)<<3 | uint64(1)<<AuthPayerBit)

		var ixHash api.TxHash
		copy(ixHash[:], "test_ix_hash_1_12345678901234567")
		ixPart := []IxHashItem{
			{Index: 3, Hash: ixHash},
		}

		accountSig, err := Sign(*owner, classicalSk, authBit, api.TxHash{}, ixPart, PubKeySignatureMode{PublicKey: *pubKey})
		assert.NoError(t, err)
		assert.NotNil(t, accountSig)

		assert.Equal(t, authBit.Raw(), accountSig.AuthBit.Raw())
		assert.Equal(t, uint64(0), accountSig.SigBit.Raw())
		assert.Len(t, accountSig.Signatures, 1)
		assert.NotNil(t, accountSig.PubKey)
	})

	t.Run("sign ix only with multisig mode", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)
		pubKey := classicalSk.Ed25519Public()

		anotherSk := crypto.NewClassicalSecretKey()
		anotherClassicalSk := crypto.AsClassicalSecretKey(anotherSk)
		assert.NotNil(t, anotherClassicalSk)
		anotherPubKey := anotherClassicalSk.Ed25519Public()

		owner, err := crypto.NewAddressFromPublicKey(anotherPubKey)
		assert.NoError(t, err)

		authBit := types.NewBitmap64(uint64(1) << 3)

		var ixHash api.TxHash
		copy(ixHash[:], "test_ix_hash_1_12345678901234567")
		ixPart := []IxHashItem{
			{Index: 3, Hash: ixHash},
		}

		accountSig, err := Sign(*owner, sk, authBit, api.TxHash{}, ixPart, MultisigKeySignatureMode{Index: 9, PublicKey: *pubKey})
		assert.NoError(t, err)
		assert.NotNil(t, accountSig)

		assert.Equal(t, authBit.Raw(), accountSig.AuthBit.Raw())
		assert.Equal(t, uint64(1)<<9, accountSig.SigBit.Raw())
		assert.Len(t, accountSig.Signatures, 1)
		assert.Nil(t, accountSig.PubKey)
	})

	t.Run("sign ix and payer with multisig mode", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)
		pubKey := classicalSk.Ed25519Public()

		anotherSk := crypto.NewClassicalSecretKey()
		anotherClassicalSk := crypto.AsClassicalSecretKey(anotherSk)
		assert.NotNil(t, anotherClassicalSk)
		anotherPubKey := anotherClassicalSk.Ed25519Public()

		owner, err := crypto.NewAddressFromPublicKey(anotherPubKey)
		assert.NoError(t, err)

		authBit := types.NewBitmap64(uint64(1)<<3 | uint64(1)<<AuthPayerBit)

		var ixHash api.TxHash
		copy(ixHash[:], "test_ix_hash_1_12345678901234567")
		ixPart := []IxHashItem{
			{Index: 3, Hash: ixHash},
		}

		accountSig, err := Sign(*owner, sk, authBit, api.TxHash{}, ixPart, MultisigKeySignatureMode{Index: 9, PublicKey: *pubKey})
		assert.NoError(t, err)
		assert.NotNil(t, accountSig)

		assert.Equal(t, authBit.Raw(), accountSig.AuthBit.Raw())
		assert.Equal(t, uint64(1)<<9, accountSig.SigBit.Raw())
		assert.Len(t, accountSig.Signatures, 1)
		assert.Nil(t, accountSig.PubKey)
	})

	t.Run("sign with mismatched address with pubkey mode", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)
		pubKey := classicalSk.Ed25519Public()

		wrongSk := crypto.NewClassicalSecretKey()
		wrongClassicalSk := crypto.AsClassicalSecretKey(wrongSk)
		assert.NotNil(t, wrongClassicalSk)
		wrongOwner, err := crypto.NewAddressFromPublicKey(wrongClassicalSk.Ed25519Public())
		assert.NoError(t, err)

		authBit := types.NewBitmap64(uint64(1) << 3)

		var ixHash api.TxHash
		copy(ixHash[:], "test_ix_hash_1_12345678901234567")
		ixPart := []IxHashItem{
			{Index: 3, Hash: ixHash},
		}

		_, err = Sign(*wrongOwner, sk, authBit, api.TxHash{}, ixPart, PubKeySignatureMode{PublicKey: *pubKey})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "public key does not match owner address")
	})

	t.Run("sign with invalid multisig index with multisig mode", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()

		anotherSk := crypto.NewClassicalSecretKey()
		anotherClassicalSk := crypto.AsClassicalSecretKey(anotherSk)
		assert.NotNil(t, anotherClassicalSk)
		anotherPubKey := anotherClassicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(anotherPubKey)
		assert.NoError(t, err)

		authBit := types.NewBitmap64(uint64(1) << 3)
		var ixPart []IxHashItem

		_, err = Sign(*owner, sk, authBit, api.TxHash{}, ixPart, MultisigKeySignatureMode{Index: 64, PublicKey: *pubKey})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})
}

func TestCollectIxHashes(t *testing.T) {
	t.Run("collect single ix hash", func(t *testing.T) {
		authBit := types.NewBitmap64(uint64(1) << 5)

		ixHashes := make([]api.TxHash, AuthPayerBit)

		copy(ixHashes[5][:], "test_ix_hash_5_123456789012345")

		result := CollectIxHashes(authBit, ixHashes)

		assert.Len(t, result, 1)
		assert.EqualValues(t, uint8(5), result[0].Index)
		assert.Equal(t, ixHashes[5], result[0].Hash)
	})

	t.Run("collect multiple ix hashes", func(t *testing.T) {
		authBit := types.NewBitmap64(uint64(1)<<0 | uint64(1)<<2 | uint64(1)<<4)

		ixHashes := make([]api.TxHash, AuthPayerBit)
		copy(ixHashes[0][:], "test_ix_hash_0_123456789012345")
		copy(ixHashes[2][:], "test_ix_hash_2_123456789012345")
		copy(ixHashes[4][:], "test_ix_hash_4_123456789012345")

		result := CollectIxHashes(authBit, ixHashes)

		assert.Len(t, result, 3)
		assert.Equal(t, uint8(0), result[0].Index)
		assert.Equal(t, ixHashes[0], result[0].Hash)
		assert.Equal(t, uint8(2), result[1].Index)
		assert.Equal(t, ixHashes[2], result[1].Hash)
		assert.Equal(t, uint8(4), result[2].Index)
		assert.Equal(t, ixHashes[4], result[2].Hash)
	})

	t.Run("skip payer bit", func(t *testing.T) {
		authBit := types.NewBitmap64(uint64(1)<<5 | uint64(1)<<AuthPayerBit)

		ixHashes := make([]api.TxHash, AuthPayerBit)
		copy(ixHashes[5][:], "test_ix_hash_5_123456789012345")

		result := CollectIxHashes(authBit, ixHashes)

		assert.Len(t, result, 1)
		assert.Equal(t, uint8(5), result[0].Index)
		assert.Equal(t, ixHashes[5], result[0].Hash)
	})

	t.Run("empty auth bit", func(t *testing.T) {
		authBit := types.NewBitmap64(0)

		ixHashes := make([]api.TxHash, AuthPayerBit)

		result := CollectIxHashes(authBit, ixHashes)
		assert.Empty(t, result)
	})

	t.Run("index out of range", func(t *testing.T) {
		authBit := types.NewBitmap64(uint64(1) << 15)

		ixHashes := make([]api.TxHash, 15)

		result := CollectIxHashes(authBit, ixHashes)
		assert.Empty(t, result)
	})
}

func TestAccountSignature_AuthMessageForTx(t *testing.T) {
	t.Run("auth message for tx with ix only", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		var ixHash1, ixHash2 api.TxHash
		copy(ixHash1[:], "test_ix_hash_1_12345678901234567")
		copy(ixHash2[:], "test_ix_hash_2_12345678901234567")

		authBit := types.NewBitmap64(uint64(1)<<2 | uint64(1)<<4)
		accountSig := Unsigned(authBit)

		ixHashes := []api.TxHash{
			{},
			{},
			ixHash1,
			{},
			ixHash2,
		}

		msg, err := accountSig.AuthMessageForTx(*owner, api.TxHash{}, ixHashes)
		assert.NoError(t, err)
		assert.NotEqual(t, api.TxHash{}, msg)
	})

	t.Run("auth message for tx with ix and payer", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		var ixHash api.TxHash
		copy(ixHash[:], "test_ix_hash_1_12345678901234567")

		authBit := types.NewBitmap64(uint64(1)<<2 | uint64(1)<<AuthPayerBit)
		accountSig := Unsigned(authBit)

		ixHashes := []api.TxHash{
			{},
			{},
			ixHash,
		}

		msg, err := accountSig.AuthMessageForTx(*owner, api.TxHash{}, ixHashes)
		assert.NoError(t, err)
		assert.NotEqual(t, api.TxHash{}, msg)
	})

	t.Run("auth message different with different tx hash", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		var txHash1, txHash2 api.TxHash
		copy(txHash1[:], "test_tx_hash_1_1234567890123456")
		copy(txHash2[:], "test_tx_hash_2_1234567890123456")

		var ixHash api.TxHash
		copy(ixHash[:], "test_ix_hash_1_12345678901234567")

		authBit := types.NewBitmap64(uint64(1)<<2 | uint64(1)<<AuthPayerBit)
		accountSig := Unsigned(authBit)

		ixHashes := []api.TxHash{
			{},
			{},
			ixHash,
		}

		msg1, err := accountSig.AuthMessageForTx(*owner, txHash1, ixHashes)
		assert.NoError(t, err)
		msg2, err := accountSig.AuthMessageForTx(*owner, txHash2, ixHashes)
		assert.NoError(t, err)

		assert.NotEqual(t, msg1, msg2)
	})

	t.Run("auth message consistency with AuthMessage", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		classicalSk := crypto.AsClassicalSecretKey(sk)
		assert.NotNil(t, classicalSk)

		pubKey := classicalSk.Ed25519Public()
		owner, err := crypto.NewAddressFromPublicKey(pubKey)
		assert.NoError(t, err)

		var txHash api.TxHash
		copy(txHash[:], "test_tx_hash_1234567890123456789")

		var ixHash1, ixHash2 api.TxHash
		copy(ixHash1[:], "test_ix_hash_1_12345678901234567")
		copy(ixHash2[:], "test_ix_hash_2_12345678901234567")

		authBit := types.NewBitmap64(uint64(1)<<1 | uint64(1)<<3 | uint64(1)<<AuthPayerBit)
		accountSig := Unsigned(authBit)

		ixHashes := []api.TxHash{
			{},
			ixHash1,
			{},
			ixHash2,
		}

		msgFromAuthMessageForTx, err := accountSig.AuthMessageForTx(*owner, txHash, ixHashes)
		assert.NoError(t, err)

		ixPart := CollectIxHashes(authBit, ixHashes)
		msgFromAuthMessage, err := accountSig.AuthMessage(*owner, txHash, ixPart)
		assert.NoError(t, err)

		assert.Equal(t, msgFromAuthMessageForTx, msgFromAuthMessage)
	})
}

func TestAccountSignature_IsVoteGateOnly(t *testing.T) {
	t.Run("vote gate only - has ix auth bits", func(t *testing.T) {
		authBit, err := AuthIx(0)
		assert.NoError(t, err)

		as := Unsigned(authBit)
		assert.True(t, as.IsVoteGateOnly())
	})

	t.Run("vote gate only - multiple ix auth bits", func(t *testing.T) {
		authBit, err := AuthIxes([]uint8{0, 1, 2})
		assert.NoError(t, err)

		as := Unsigned(authBit)
		assert.True(t, as.IsVoteGateOnly())
	})

	t.Run("not vote gate - has payer bit only", func(t *testing.T) {
		authBit := AuthPayer()
		as := Unsigned(authBit)
		assert.False(t, as.IsVoteGateOnly())
	})

	t.Run("not vote gate - has signatures", func(t *testing.T) {
		authBit, err := AuthIx(0)
		assert.NoError(t, err)

		as := AccountSignature{
			AuthBit:    authBit,
			SigBit:     types.NewBitmap64(0),
			Signatures: []crypto.Signature{{Variant: crypto.SignatureTypeEd25519, Bytes: make([]byte, 64)}},
			PubKey:     nil,
		}
		assert.False(t, as.IsVoteGateOnly())
	})

	t.Run("not vote gate - has pubkey", func(t *testing.T) {
		sk := crypto.NewClassicalSecretKey()
		pubKey := sk.(*crypto.ClassicalSecretKey).Ed25519Public()

		authBit, err := AuthIx(0)
		assert.NoError(t, err)

		as := AccountSignature{
			AuthBit:    authBit,
			SigBit:     types.NewBitmap64(0),
			Signatures: []crypto.Signature{},
			PubKey:     pubKey,
		}
		assert.False(t, as.IsVoteGateOnly())
	})

	t.Run("not vote gate - has sig_bit", func(t *testing.T) {
		authBit, err := AuthIx(0)
		assert.NoError(t, err)

		as := AccountSignature{
			AuthBit:    authBit,
			SigBit:     types.NewBitmap64(1 << 5), // 设置了多签位
			Signatures: []crypto.Signature{},
			PubKey:     nil,
		}

		assert.False(t, as.IsVoteGateOnly())
	})

	t.Run("not vote gate - empty auth_bit", func(t *testing.T) {
		as := Unsigned(types.NewBitmap64(0))
		assert.False(t, as.IsVoteGateOnly())
	})

	t.Run("vote gate with payer and ix bits - still true", func(t *testing.T) {
		authBit, err := AuthIxAndPayer(0)
		assert.NoError(t, err)

		as := Unsigned(authBit)
		assert.True(t, as.IsVoteGateOnly())
	})
}
