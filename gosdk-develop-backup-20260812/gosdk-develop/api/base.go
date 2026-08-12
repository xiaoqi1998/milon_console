package api

import (
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcutil/base58"
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

// String implements the Stringer interface, returning Base58 format
func (txHash TxHash) String() string {
	return base58.Encode(txHash[:])
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

// TypeTagWithData contains type_tag and value bytes
type TypeTagWithData struct {
	TypeTag uint64
	Value   []byte // raw value bytes (without type_tag)
}

// TypeTagWithDataResolver dynamically resolves type_tag from bytes based on IDL
type TypeTagWithDataResolver interface {
	// DecodeResource decodes a resource of the given type_tag from bytes, returns the value and remaining
	DecodeResource(typeTag uint64, bytes []byte) (valueBytes []byte, remaining []byte, err error)

	// DecodeEvent decodes an event of the given type_tag from bytes, returns the event and remaining
	DecodeEvent(typeTag uint64, bytes []byte) (eventBytes []byte, remaining []byte, err error)
}

var GlobalTypeResolver TypeTagWithDataResolver

func SetGlobalTypeResolver(resolver TypeTagWithDataResolver) {
	GlobalTypeResolver = resolver
}

// ReadAnySerializeValueWithTypeTag reads value bytes by type_tag
func ReadAnySerializeValueWithTypeTag(d *postcard.Deserializer, typeTag uint64) ([]byte, error) {
	if GlobalTypeResolver != nil {
		remaining := d.Buffer()[d.Offset():]
		valueBytes, rest, err := GlobalTypeResolver.DecodeResource(typeTag, remaining)
		if err != nil {
			return nil, fmt.Errorf("TypeTagWithDataResolver.DecodeResource failed (type_tag=%d): %w", typeTag, err)
		}

		// manually advance deserializer offset
		consumed := len(remaining) - len(rest)
		d.Advance(consumed)
		return valueBytes, nil
	}

	// fallback: only works for Vec<u8>/String
	val, err := d.DeserializeBytes()
	if err != nil {
		return nil, fmt.Errorf("unknown type_tag %d (no TypeTagWithDataResolver), fallback DeserializeBytes failed: %w", typeTag, err)
	}

	return val, nil
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
	if GlobalTypeResolver != nil {
		remaining := d.Buffer()[d.Offset():]
		eventBytes, rest, err := GlobalTypeResolver.DecodeEvent(typeTag, remaining)
		if err != nil {
			return nil, fmt.Errorf("GlobalTypeResolver.DecodeEvent failed (type_tag=%d): %w", typeTag, err)
		}
		// manually advance deserializer offset
		consumed := len(remaining) - len(rest)
		d.Advance(consumed)
		return eventBytes, nil
	}

	// fallback
	val, err := d.DeserializeBytes()
	if err != nil {
		return nil, fmt.Errorf("unknown event type_tag %d (no GlobalTypeResolver), fallback DeserializeBytes failed: %w", typeTag, err)
	}
	return val, nil
}
