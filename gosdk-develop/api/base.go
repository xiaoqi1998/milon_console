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

// TypeTagWithData contains type_tag and value bytes
type TypeTagWithData struct {
	TypeTag uint64
	Value   []byte // raw value bytes (without type_tag)
}

// DeserializeEventEntry deserializes an event entry (type_tag + value) from postcard format. Used by SimulateReceipt.
func DeserializeEventEntry(d *postcard.Deserializer) (TypeTagWithData, error) {
	typeTag, err := d.DeserializeU64()
	if err != nil {
		return TypeTagWithData{}, fmt.Errorf("failed to deserialize event type_tag: %w", err)
	}

	// body: Vec<u8>: length prefix + payload bytes
	raw, err := d.DeserializeBytes()
	if err != nil {
		return TypeTagWithData{}, fmt.Errorf("failed to read event body (type_tag=%d): %w", typeTag, err)
	}

	if resolver := d.TypeResolver(); resolver != nil {
		eventBytes, _, err := resolver.DecodeEvent(typeTag, raw)
		if err != nil {
			return TypeTagWithData{}, fmt.Errorf("TypeResolver.DecodeEvent failed (type_tag=%d): %w", typeTag, err)
		}
		return TypeTagWithData{TypeTag: typeTag, Value: eventBytes}, nil
	}

	return TypeTagWithData{TypeTag: typeTag, Value: raw}, nil
}

// DeserializeEventEntryNoLen deserializes an event entry (type_tag + value,
// value WITHOUT length prefix) from postcard format. Used by TxHistory.
func DeserializeEventEntryNoLen(d *postcard.Deserializer) (TypeTagWithData, error) {
	typeTag, err := d.DeserializeU64()
	if err != nil {
		return TypeTagWithData{}, fmt.Errorf("failed to deserialize event type_tag: %w", err)
	}

	if resolver := d.TypeResolver(); resolver != nil {
		remaining := d.Buffer()[d.Offset():]
		eventBytes, rest, err := resolver.DecodeEvent(typeTag, remaining)
		if err != nil {
			return TypeTagWithData{}, fmt.Errorf("TypeResolver.DecodeEvent failed (type_tag=%d): %w", typeTag, err)
		}

		consumed := len(remaining) - len(rest)
		if err = d.Advance(consumed); err != nil {
			return TypeTagWithData{}, fmt.Errorf("advance failed after DecodeEvent: %w", err)
		}
		return TypeTagWithData{TypeTag: typeTag, Value: eventBytes}, nil
	}

	val, err := d.DeserializeBytes()
	if err != nil {
		return TypeTagWithData{}, fmt.Errorf("unknown event type_tag %d (no TypeResolver), fallback DeserializeBytes failed: %w", typeTag, err)
	}
	return TypeTagWithData{TypeTag: typeTag, Value: val}, nil
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

// SerializePersistedValue serializes a PersistedValue; Inline values carry a
// length prefix (FramedSimulateReceipt/FramedPersistedValue wire format). Used by SimulateReceipt.
func SerializePersistedValue(serializer *postcard.Serializer, pv PersistedValue) error {
	if err := serializer.SerializeU32(pv.Variant); err != nil {
		return fmt.Errorf("failed to serialize variant: %w", err)
	}

	switch pv.Variant {
	case 0:
		// Inline(FramedDynamicValue): type_tag + body(Vec<u8>, length-prefixed)
		if err := serializer.SerializeU64(pv.TypeTag); err != nil {
			return fmt.Errorf("failed to serialize type_tag: %w", err)
		}
		if err := serializer.SerializeBytes(pv.InlineData); err != nil {
			return fmt.Errorf("failed to serialize Inline body: %w", err)
		}
	case 1:
		// External(BlobHash)
		serializer.SerializeFixedBytes(pv.ExternalHash[:])
	default:
		return fmt.Errorf("unknown PersistedValue variant: %d", pv.Variant)
	}
	return nil
}

// SerializePersistedValueNoLen serializes a PersistedValue; Inline values carry
// no length prefix (TxHistory/PersistedValue::Inline(AnySerializeOwned) wire format). Used by TxHistory.
func SerializePersistedValueNoLen(serializer *postcard.Serializer, pv PersistedValue) error {
	if err := serializer.SerializeU32(pv.Variant); err != nil {
		return fmt.Errorf("failed to serialize variant: %w", err)
	}

	switch pv.Variant {
	case 0:
		// Inline(AnySerializeOwned): type_tag + value (no length prefix)
		if err := serializer.SerializeU64(pv.TypeTag); err != nil {
			return fmt.Errorf("failed to serialize type_tag: %w", err)
		}
		serializer.SerializeFixedBytes(pv.InlineData)
	case 1:
		// External(BlobHash)
		serializer.SerializeFixedBytes(pv.ExternalHash[:])
	default:
		return fmt.Errorf("unknown PersistedValue variant: %d", pv.Variant)
	}
	return nil
}

// DeserializeAccessRecord deserializes an AccessRecord from postcard format. Used by SimulateReceipt.
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
		// Inline(FramedDynamicValue): type_tag + body(Vec<u8>, length-prefixed)
		typeTag, err := d.DeserializeU64()
		if err != nil {
			return PersistedValue{}, fmt.Errorf("failed to read type_tag: %w", err)
		}

		// body: Vec<u8>: length prefix + payload bytes
		raw, err := d.DeserializeBytes()
		if err != nil {
			return PersistedValue{}, fmt.Errorf("failed to read Inline body (type_tag=%d): %w", typeTag, err)
		}

		var inlineData []byte
		if resolver := d.TypeResolver(); resolver != nil {
			valueBytes, _, err := resolver.DecodeResource(typeTag, raw)
			if err != nil {
				return PersistedValue{}, fmt.Errorf("TypeTagWithDataResolver.DecodeResource failed (type_tag=%d): %w", typeTag, err)
			}
			inlineData = valueBytes
		} else {
			inlineData = raw
		}

		return PersistedValue{
			Variant:    variant,
			TypeTag:    typeTag,
			InlineData: inlineData,
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

// DeserializeAccessRecordNoLen deserializes an AccessRecord whose Inline/Event
// values carry NO length prefix (TxHistory/TxReceipt wire format). Used by TxHistory.
func DeserializeAccessRecordNoLen(d *postcard.Deserializer) (AccessRecord, error) {
	var rec AccessRecord

	// ResourceID (18 bytes)
	rid, err := d.DeserializeFixedBytes(RsHashLen)
	if err != nil {
		return rec, fmt.Errorf("failed to deserialize ResourceID: %w", err)
	}
	copy(rec.ResourceID[:], rid)

	// FirstSnapshot: Option<PersistedValue>
	firstSnapshot, err := postcard.DeserializeOption(d, deserializePersistedValueNoLen)
	if err != nil {
		return rec, fmt.Errorf("failed to deserialize FirstSnapshot: %w", err)
	}
	rec.FirstSnapshot = firstSnapshot

	// LastWritten: PersistedValue (non-Option)
	lastWritten, err := deserializePersistedValueNoLen(d)
	if err != nil {
		return rec, fmt.Errorf("failed to deserialize LastWritten: %w", err)
	}
	rec.LastWritten = lastWritten

	return rec, nil
}
func deserializePersistedValueNoLen(d *postcard.Deserializer) (PersistedValue, error) {
	variant, err := d.DeserializeU32()
	if err != nil {
		return PersistedValue{}, fmt.Errorf("failed to read variant: %w", err)
	}

	switch variant {
	case 0:
		// Inline(AnySerializeOwned): type_tag + value (no length prefix)
		typeTag, err := d.DeserializeU64()
		if err != nil {
			return PersistedValue{}, fmt.Errorf("failed to read type_tag: %w", err)
		}

		var inlineData []byte
		if resolver := d.TypeResolver(); resolver != nil {
			remaining := d.Buffer()[d.Offset():]
			valueBytes, rest, err := resolver.DecodeResource(typeTag, remaining)
			if err != nil {
				return PersistedValue{}, fmt.Errorf("TypeTagWithDataResolver.DecodeResource failed (type_tag=%d): %w", typeTag, err)
			}

			consumed := len(remaining) - len(rest)
			if err = d.Advance(consumed); err != nil {
				return PersistedValue{}, fmt.Errorf("Advance failed after DecodeResource: %w", err)
			}
			inlineData = valueBytes
		} else {
			val, err := d.DeserializeBytes()
			if err != nil {
				return PersistedValue{}, fmt.Errorf("unknown type_tag %d (no TypeResolver), fallback DeserializeBytes failed: %w", typeTag, err)
			}
			inlineData = val
		}

		return PersistedValue{
			Variant:    variant,
			TypeTag:    typeTag,
			InlineData: inlineData,
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
