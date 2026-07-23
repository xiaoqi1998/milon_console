package milon

import (
	"encoding/binary"
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/types"
)

const AuthPayerBit = 63

// AccountSignatureMode 签名模式接口
type AccountSignatureMode interface {
	isAccountSignatureMode()
}

// PubKeySignatureMode 单公钥签名模式
type PubKeySignatureMode struct {
	PublicKey crypto.PublicKey
}

func (PubKeySignatureMode) isAccountSignatureMode() {}

// MultisigKeySignatureMode 多签密钥模式
type MultisigKeySignatureMode struct {
	Index     uint8
	PublicKey crypto.PublicKey
}

func (MultisigKeySignatureMode) isAccountSignatureMode() {}

func NewAccountSignatureWithPubKey(signature crypto.Signature, pubKey crypto.PublicKey) AccountSignature {
	return AccountSignature{
		AuthBit:    types.NewBitmap64(0),
		SigBit:     types.NewBitmap64(0),
		Signatures: []crypto.Signature{signature},
		PubKey:     &pubKey,
	}
}

func NewAccountSignature(index uint8, signature crypto.Signature) (AccountSignature, error) {
	if index >= 64 {
		return AccountSignature{}, fmt.Errorf("index %d out of range (max 63)", index)
	}

	return AccountSignature{
		AuthBit:    types.NewBitmap64(0),
		SigBit:     types.NewBitmap64(1 << index),
		Signatures: []crypto.Signature{signature},
		PubKey:     nil,
	}, nil
}

type AccountSignature struct {
	AuthBit    types.Bitmap64     //1 2 4 8 ...
	SigBit     types.Bitmap64     //0=单签  非0=多签，最多64个1，根据AccountSignatureMode来获取
	Signatures []crypto.Signature //最多64个
	PubKey     *crypto.PublicKey  //PubKey为nil则SigBit为0。PubKey为有值则SigBit不为0
}

// Add 新增多签 key 的签名
func (as *AccountSignature) Add(index uint8, signature crypto.Signature) error {
	return as.AddMultisigKey(index, signature)
}

// AddMultisigKey 追加同一 auth_bit、同一授权消息下的多签 key 签名
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

// AuthorizesIx 检查是否授权了指定的 ix
func (as *AccountSignature) AuthorizesIx(ix uint8) bool {
	return as.AuthBit.Test(ix)
}

// AuthorizesPayer 检查是否授权了 payer
func (as *AccountSignature) AuthorizesPayer() bool {
	return as.AuthBit.Test(AuthPayerBit)
}

// IxHashItem ix 哈希项
type IxHashItem struct {
	Index uint8
	Hash  api.TxHash
}

// AuthMessage 组装账户授权消息 	Blake3(MILON_ROOT || TX_AUTH_DOMAIN || chain_id || owner || auth_bit || tx_hash || ixHashes)
func (as *AccountSignature) AuthMessage(owner crypto.Address, txHash api.TxHash, ixHashes []IxHashItem) (api.TxHash, error) {
	hasher := crypto.Hash32Hasher([]byte(crypto.MilonTxAuthDomainContext))

	chainIDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(chainIDBytes, GetChainId())
	hasher.Write(chainIDBytes)

	ownerBytes := owner.AsBytes()
	hasher.Write(ownerBytes[:])

	authBitBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(authBitBytes, uint64(as.AuthBit))
	hasher.Write(authBitBytes)

	// v3 协议：总是包含 tx_hash
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

// AuthMessageForTx 按交易上下文组装 AccountAuthMessage（验签用）
func (as *AccountSignature) AuthMessageForTx(owner crypto.Address, txHash api.TxHash, ixHashes []api.TxHash) (api.TxHash, error) {
	ixPart := CollectIxHashes(as.AuthBit, ixHashes)
	return as.AuthMessage(owner, txHash, ixPart)
}

// IsVoteGateOnly 检查是否只设置了 ix 授权位（bit0-62），但不包含 payer 位（bit63），且没有实际的签名。这用于 vote gate 场景
func (as *AccountSignature) IsVoteGateOnly() bool {
	return as.PubKey == nil &&
		len(as.Signatures) == 0 &&
		as.SigBit.Raw() == 0 &&
		(as.AuthBit.Raw()&((uint64(1)<<AuthPayerBit)-1)) != 0
	/*
		(uint64(1)<<AuthPayerBit)-1) 	==>		bit0 到 bit62 全是 1，bit63 是 0

		as.AuthBit.Raw() & ...
			AuthBit:     1xxx...xxxx  (任意值)
			掩码:        0111...1111  (只保留低 63 位)
			===========
			结果:        0xxx...xxxx  (bit63 被清零)
	*/
}

// MarshalPostcard 实现 postcard.Marshaler 接口
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

// UnmarshalPostcard 实现 postcard.Unmarshaler 接口
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

// AuthIx 创建 ix 授权位图（最大ix为62）
func AuthIx(ix uint8) (types.Bitmap64, error) {
	if ix >= AuthPayerBit {
		return types.NewBitmap64(0), fmt.Errorf("ix index %d out of range (max %d)", ix, AuthPayerBit-1)
	}
	return types.NewBitmap64(uint64(1) << ix), nil
}

// AuthIxes 创建多个 ix 的授权位图（最大ix为62）
func AuthIxes(indices []uint8) (types.Bitmap64, error) {
	var raw uint64
	for _, ix := range indices {
		if ix >= AuthPayerBit {
			return types.NewBitmap64(0), fmt.Errorf("ix index %d out of range (max %d)", ix, AuthPayerBit-1)
		}
		raw |= uint64(1) << ix
	}
	return types.NewBitmap64(raw), nil
}

// AuthPayer 创建 payer 授权位图
func AuthPayer() types.Bitmap64 {
	return types.NewBitmap64(uint64(1) << AuthPayerBit)
}

// AuthIxAndPayer 创建 ix + payer 联合授权位图
func AuthIxAndPayer(ix uint8) (types.Bitmap64, error) {
	if ix >= AuthPayerBit {
		return types.NewBitmap64(0), fmt.Errorf("ix index %d out of range (max %d)", ix, AuthPayerBit-1)
	}
	return types.NewBitmap64(uint64(1<<ix) | (1 << AuthPayerBit)), nil
}

// Unsigned 创建未签名的 AccountSignature（仅设置 auth_bit）
func Unsigned(authBit types.Bitmap64) AccountSignature {
	return AccountSignature{
		AuthBit:    authBit,
		SigBit:     types.NewBitmap64(0),
		Signatures: []crypto.Signature{},
		PubKey:     nil,
	}
}

// Sign 给定 auth 上下文计算授权消息并签名
func Sign(owner crypto.Address, sk crypto.SecretKeyer, authBit types.Bitmap64, txHash api.TxHash, ixHashes []IxHashItem, mode AccountSignatureMode) (*AccountSignature, error) {
	var publicKey crypto.PublicKey
	var sigBit types.Bitmap64
	var pubKeyField *crypto.PublicKey

	switch m := mode.(type) {
	case PubKeySignatureMode:
		pkAddr, err := crypto.NewAddressFromPublicKey(&m.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get address from public key: %w", err)
		}
		if *pkAddr != owner {
			return nil, fmt.Errorf("public key does not match owner address")
		}
		publicKey = m.PublicKey
		sigBit = types.NewBitmap64(0)
		pubKeyField = &m.PublicKey
	case MultisigKeySignatureMode:
		if m.Index >= 64 {
			return nil, fmt.Errorf("multisig key index %d out of range (max 63)", m.Index)
		}
		publicKey = m.PublicKey
		sigBit = types.NewBitmap64(uint64(1) << m.Index)
		pubKeyField = nil
	default:
		return nil, fmt.Errorf("invalid signature mode")
	}

	accountSignature := AccountSignature{
		AuthBit:    authBit,
		SigBit:     sigBit,
		Signatures: []crypto.Signature{},
		PubKey:     pubKeyField,
	}

	authHash, err := accountSignature.AuthMessage(owner, txHash, ixHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to compute auth message: %w", err)
	}

	signature, err := sk.SignFor(publicKey, authHash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return &AccountSignature{
		AuthBit:    authBit,
		SigBit:     sigBit,
		Signatures: []crypto.Signature{*signature},
		PubKey:     pubKeyField,
	}, nil
}

// SimulateSign 给定 auth 上下文计算授权消息但不签名
func SimulateSign(owner crypto.Address, authBit types.Bitmap64, mode AccountSignatureMode) (*AccountSignature, error) {
	var sigBit types.Bitmap64
	var pubKeyField *crypto.PublicKey

	switch m := mode.(type) {
	case PubKeySignatureMode:
		pkAddr, err := crypto.NewAddressFromPublicKey(&m.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get address from public key: %w", err)
		}
		if *pkAddr != owner {
			return nil, fmt.Errorf("public key does not match owner address")
		}
		sigBit = types.NewBitmap64(0)
		pubKeyField = &m.PublicKey
	case MultisigKeySignatureMode:
		if m.Index >= 64 {
			return nil, fmt.Errorf("multisig key index %d out of range (max 63)", m.Index)
		}
		sigBit = types.NewBitmap64(uint64(1) << m.Index)
		pubKeyField = nil
	default:
		return nil, fmt.Errorf("invalid signature mode")
	}

	return &AccountSignature{
		AuthBit:    authBit,
		SigBit:     sigBit,
		Signatures: []crypto.Signature{},
		PubKey:     pubKeyField,
	}, nil
}

// CollectIxHashes 收集 auth_bit 中 ix 位对应的 IxHash
func CollectIxHashes(authBit types.Bitmap64, ixHashes []api.TxHash) []IxHashItem {
	var out []IxHashItem
	for i := uint8(0); i < 64; i++ {
		if !authBit.Test(i) {
			continue
		}
		if i == AuthPayerBit {
			continue
		}
		if int(i) < len(ixHashes) {
			out = append(out, IxHashItem{Index: i, Hash: ixHashes[i]})
		}
	}
	return out
}
