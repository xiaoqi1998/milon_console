package api

import (
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcutil/base58"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
)

type PackedInstruction []byte

const TxHashLen = 32
const TxProofIdentifierLen = 12
const TxIdLen = 12
const RsHashLen = 18
const BlobHashLen = 32

type TxHash [TxHashLen]byte
type TxProofIdentifier [TxProofIdentifierLen]byte
type TxId [TxIdLen]byte
type RsHash [RsHashLen]byte
type BlobHash [BlobHashLen]byte

const MIL = "M11on1111111111111111111111"

var MILToken *crypto.Address

func init() {
	var err error
	MILToken, err = crypto.NewAddressFromBytes(base58.Decode(MIL))
	if err != nil {
		panic(fmt.Sprintf("failed to decode MIL token address: %v", err))
	}
}

// String implements the Stringer interface, returning Base58 format
func (txHash TxHash) String() string {
	return base58.Encode(txHash[:])
}

func (txHash TxHash) ToHex() string {
	return hex.EncodeToString(txHash[:])
}

func (txHash TxHash) ToBase58() string {
	return base58.Encode(txHash[:])
}

func (txId TxId) ToHex() string {
	return hex.EncodeToString(txId[:])
}

func (txId TxId) ToBase58() string {
	return base58.Encode(txId[:])
}

//func (rsHash RsHash) String() string {
//	return base58.Encode(rsHash[:])
//}
//func (blobHash BlobHash) String() string {
//	return base58.Encode(blobHash[:])
//}

func NewTxHashFromRelaxed(input any) (TxHash, error) {
	var hash TxHash

	switch v := input.(type) {
	case *TxHash:
		if v == nil {
			return hash, fmt.Errorf("nil hash")
		}
		return *v, nil
	case TxHash:
		return v, nil
	case string:
		// try hex decode first
		buf, err := hex.DecodeString(v)
		if err == nil {
			if len(buf) != TxHashLen {
				return hash, fmt.Errorf("invalid hex decoded length: expected %d, got %d", TxHashLen, len(buf))
			}
			copy(hash[:], buf)
			return hash, nil
		}

		// try base58 decode if hex fails
		buf = base58.Decode(v)
		if len(buf) != TxHashLen {
			return hash, fmt.Errorf("invalid base58 decoded length: expected %d, got %d", TxHashLen, len(buf))
		}
		copy(hash[:], buf)
		return hash, nil
	default:
		return hash, fmt.Errorf("unsupported type for TxHash: %T (expected string or api.TxHash)", input)
	}
}

func NewTxHashOrTxIdFromRelaxed(input any) ([]byte, error) {
	switch v := input.(type) {
	case *TxHash:
		if v == nil {
			return nil, fmt.Errorf("nil hash")
		}
		return v[:], nil
	case TxHash:
		return v[:], nil
	case *TxId:
		if v == nil {
			return nil, fmt.Errorf("nil id")
		}
		return v[:], nil
	case TxId:
		return v[:], nil
	case string:
		// try hex decode first
		buf, err := hex.DecodeString(v)
		if err == nil {
			if len(buf) == TxHashLen || len(buf) == TxIdLen {
				return buf, nil
			}
			return nil, fmt.Errorf("invalid hex decoded length: expected %d or %d, got %d", TxHashLen, TxIdLen, len(buf))
		}

		// try base58 decode if hex fails
		buf = base58.Decode(v)
		if len(buf) == TxHashLen || len(buf) == TxIdLen {
			return buf, nil
		}
		return nil, fmt.Errorf("invalid base58 decoded length: expected %d or %d, got %d", TxHashLen, TxIdLen, len(buf))
	case []byte:
		if len(v) == TxHashLen || len(v) == TxIdLen {
			return v, nil
		}
		return nil, fmt.Errorf("invalid byte array length: expected %d or %d, got %d", TxHashLen, TxIdLen, len(v))
	default:
		return nil, fmt.Errorf("unsupported type for TxHash: %T (expected string or api.TxHash)", input)
	}
}

// TypeTagWithData contains type_tag and value bytes
type TypeTagWithData struct {
	TypeTag uint64
	Value   []byte // raw value bytes (without type_tag)
}

// DeserializeEventEntry deserializes an event entry (type_tag + value) from postcard format
func DeserializeEventEntry(d *postcard.Deserializer) (TypeTagWithData, error) {
	typeTag, err := d.DeserializeU64()
	if err != nil {
		return TypeTagWithData{}, fmt.Errorf("failed to deserialize event type_tag: %w", err)
	}

	data, err := readEventValue(d, typeTag)
	if err != nil {
		return TypeTagWithData{}, fmt.Errorf("failed to read event value (type_tag=%d): %w", typeTag, err)
	}

	return TypeTagWithData{
		TypeTag: typeTag,
		Value:   data,
	}, nil
}
func readEventValue(d *postcard.Deserializer, typeTag uint64) ([]byte, error) {
	if resolver := d.TypeResolver(); resolver != nil {
		remaining := d.Buffer()[d.Offset():]
		eventBytes, rest, err := resolver.DecodeEvent(typeTag, remaining)
		if err != nil {
			return nil, fmt.Errorf("TypeResolver.DecodeEvent failed (type_tag=%d): %w", typeTag, err)
		}

		consumed := len(remaining) - len(rest)
		if err = d.Advance(consumed); err != nil {
			return nil, fmt.Errorf("Advance failed after DecodeEvent: %w", err)
		}
		return eventBytes, nil
	}

	// fallback
	val, err := d.DeserializeBytes()
	if err != nil {
		return nil, fmt.Errorf("unknown event type_tag %d (no TypeResolver), fallback DeserializeBytes failed: %w", typeTag, err)
	}
	return val, nil
}

type AccessRecord struct {
	ResourceID    RsHash
	FirstSnapshot *PersistedValue
	LastWritten   PersistedValue
}
type PersistedValue struct {
	Variant      uint32
	TypeTag      uint64   // Inline type_tag (only valid when Variant==0)
	InlineData   []byte   // Inline raw value bytes (only valid when Variant==0)
	ExternalHash [32]byte // External BlobHash (only valid when Variant==1)
}

// DeserializeAccessRecord deserializes an AccessRecord from postcard format
func DeserializeAccessRecord(d *postcard.Deserializer) (AccessRecord, error) {
	var rec AccessRecord

	// ResourceID (18 bytes)
	rid, err := d.DeserializeFixedBytes(RsHashLen)
	if err != nil {
		return rec, fmt.Errorf("failed to deserialize ResourceID: %w", err)
	}
	copy(rec.ResourceID[:], rid)

	// FirstSnapshot: Option<PersistedValue>
	firstSnapshot, err := postcard.DeserializeOption(d, deserializePersistedValue)
	if err != nil {
		return rec, fmt.Errorf("failed to deserialize FirstSnapshot: %w", err)
	}
	rec.FirstSnapshot = firstSnapshot

	// LastWritten: PersistedValue (non-Option)
	lastWritten, err := deserializePersistedValue(d)
	if err != nil {
		return rec, fmt.Errorf("failed to deserialize LastWritten: %w", err)
	}
	rec.LastWritten = lastWritten

	return rec, nil
}
func deserializePersistedValue(d *postcard.Deserializer) (PersistedValue, error) {
	variant, err := d.DeserializeU32()
	if err != nil {
		return PersistedValue{}, fmt.Errorf("failed to read variant: %w", err)
	}

	switch variant {
	case 0:
		// Inline(AnySerializeOwned)
		typeTag, err := d.DeserializeU64()
		if err != nil {
			return PersistedValue{}, fmt.Errorf("failed to read type_tag: %w", err)
		}

		valueBytes, err := ReadAnySerializeValueWithTypeTag(d, typeTag)
		if err != nil {
			return PersistedValue{}, fmt.Errorf("failed to read Inline value (type_tag=%d): %w", typeTag, err)
		}

		return PersistedValue{
			Variant:    variant,
			TypeTag:    typeTag,
			InlineData: valueBytes,
		}, nil
	case 1:
		// External(BlobHash)
		hash, err := d.DeserializeFixedBytes(BlobHashLen)
		if err != nil {
			return PersistedValue{}, fmt.Errorf("failed to read External BlobHash: %w", err)
		}

		var extHash [BlobHashLen]byte
		copy(extHash[:], hash)

		return PersistedValue{
			Variant:      variant,
			ExternalHash: extHash,
		}, nil
	default:
		return PersistedValue{}, fmt.Errorf("unknown PersistedValue variant: %d", variant)
	}
}

// TypeTagWithDataResolver dynamically resolves type_tag from bytes based on IDL
// Deprecated: use postcard.TypeResolver, injected via postcard.Deserializer.SetTypeResolver.
type TypeTagWithDataResolver interface {
	DecodeResource(typeTag uint64, bytes []byte) (valueBytes []byte, remaining []byte, err error)
	DecodeEvent(typeTag uint64, bytes []byte) (eventBytes []byte, remaining []byte, err error)
}

// ReadAnySerializeValueWithTypeTag reads value bytes by type_tag
func ReadAnySerializeValueWithTypeTag(d *postcard.Deserializer, typeTag uint64) ([]byte, error) {
	if resolver := d.TypeResolver(); resolver != nil {
		remaining := d.Buffer()[d.Offset():]
		valueBytes, rest, err := resolver.DecodeResource(typeTag, remaining)
		if err != nil {
			return nil, fmt.Errorf("TypeTagWithDataResolver.DecodeResource failed (type_tag=%d): %w", typeTag, err)
		}

		// manually advance deserializer offset
		consumed := len(remaining) - len(rest)
		if err = d.Advance(consumed); err != nil {
			return nil, fmt.Errorf("Advance failed after DecodeResource: %w", err)
		}
		return valueBytes, nil
	}

	// todo----fallback: only works for Vec<u8>/String
	val, err := d.DeserializeBytes()
	if err != nil {
		return nil, fmt.Errorf("unknown type_tag %d (no TypeResolver), fallback DeserializeBytes failed: %w", typeTag, err)
	}

	return val, nil
}

// UnmarshalRsHashFromJSONArray parses an RsHash from a JSON number array ([]interface{}).
// Each element is a float64 (JSON number → Go json.Decoder default) converted to byte.
func UnmarshalRsHashFromJSONArray(raw []interface{}) (RsHash, error) {
	var rsHash RsHash
	for i, b := range raw {
		if i >= RsHashLen {
			return rsHash, fmt.Errorf("rsHash byte array length exceeds %d", RsHashLen)
		}
		if val, ok := b.(float64); ok {
			rsHash[i] = byte(val)
		}
	}
	return rsHash, nil

}
