package api

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/postcard"
)

/*
	rust (api/rpc/wire/src/payload.rs)
	pub struct NodeCapabilityView {
		pub features: Vec<u32>,
		pub binary_digest: Option<[u8; 32]>,
		pub height: u64,
		pub missing_active: Vec<u32>,
		pub missing_voting: Vec<u32>,
		pub supports_all: bool,
		pub tx_unknown: Vec<TxUnknownCount>,
	}
	pub struct TxUnknownCount {
		pub kind: String,
		pub value: String,
		pub count: u64,
	}
*/

// NodeCapabilityView is the response body of node_capability (method 30).
// It reports MIP-33 features compiled into the node's binary plus counters
// for transactions the node refused to admit because of unknown format,
// discriminator or parameter shapes.
type NodeCapabilityView struct {
	Features      []uint32
	BinaryDigest  *[32]byte
	Height        uint64
	MissingActive []uint32
	MissingVoting []uint32
	SupportsAll   bool
	TxUnknown     []TxUnknownCount
}

// TxUnknownCount is a single rejection counter carried by NodeCapabilityView.
// kind is one of "format", "method", "param".
type TxUnknownCount struct {
	Kind  string
	Value string
	Count uint64
}

func (v *NodeCapabilityView) MarshalPostcard(s *postcard.Serializer) error {
	if err := postcard.SerializeSeq(s, v.Features, func(se *postcard.Serializer, f uint32) error {
		return se.SerializeU32(f)
	}); err != nil {
		return fmt.Errorf("failed to serialize Features: %w", err)
	}

	if err := postcard.SerializeOption(s, v.BinaryDigest, func(se *postcard.Serializer, d [32]byte) error {
		se.SerializeFixedBytes(d[:])
		return nil
	}); err != nil {
		return fmt.Errorf("failed to serialize BinaryDigest: %w", err)
	}

	if err := s.SerializeU64(v.Height); err != nil {
		return fmt.Errorf("failed to serialize Height: %w", err)
	}

	if err := postcard.SerializeSeq(s, v.MissingActive, func(se *postcard.Serializer, f uint32) error {
		return se.SerializeU32(f)
	}); err != nil {
		return fmt.Errorf("failed to serialize MissingActive: %w", err)
	}

	if err := postcard.SerializeSeq(s, v.MissingVoting, func(se *postcard.Serializer, f uint32) error {
		return se.SerializeU32(f)
	}); err != nil {
		return fmt.Errorf("failed to serialize MissingVoting: %w", err)
	}

	if err := s.SerializeBool(v.SupportsAll); err != nil {
		return fmt.Errorf("failed to serialize SupportsAll: %w", err)
	}

	if err := postcard.SerializeSeq(s, v.TxUnknown, func(se *postcard.Serializer, t TxUnknownCount) error {
		return t.MarshalPostcard(se)
	}); err != nil {
		return fmt.Errorf("failed to serialize TxUnknown: %w", err)
	}

	return nil
}

func (v *NodeCapabilityView) UnmarshalPostcard(d *postcard.Deserializer) error {
	features, err := postcard.DeserializeSeq(d, func(se *postcard.Deserializer) (uint32, error) {
		return se.DeserializeU32()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize Features: %w", err)
	}
	v.Features = features

	binaryDigest, err := postcard.DeserializeOption(d, func(se *postcard.Deserializer) ([32]byte, error) {
		var out [32]byte
		raw, err := se.DeserializeFixedBytes(32)
		if err != nil {
			return out, err
		}
		copy(out[:], raw)
		return out, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize BinaryDigest: %w", err)
	}
	v.BinaryDigest = binaryDigest

	height, err := d.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Height: %w", err)
	}
	v.Height = height

	missingActive, err := postcard.DeserializeSeq(d, func(se *postcard.Deserializer) (uint32, error) {
		return se.DeserializeU32()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize MissingActive: %w", err)
	}
	v.MissingActive = missingActive

	missingVoting, err := postcard.DeserializeSeq(d, func(se *postcard.Deserializer) (uint32, error) {
		return se.DeserializeU32()
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize MissingVoting: %w", err)
	}
	v.MissingVoting = missingVoting

	supportsAll, err := d.DeserializeBool()
	if err != nil {
		return fmt.Errorf("failed to deserialize SupportsAll: %w", err)
	}
	v.SupportsAll = supportsAll

	txUnknown, err := postcard.DeserializeSeq(d, func(se *postcard.Deserializer) (TxUnknownCount, error) {
		var t TxUnknownCount
		if err := t.UnmarshalPostcard(se); err != nil {
			return t, err
		}
		return t, nil
	})
	if err != nil {
		return fmt.Errorf("failed to deserialize TxUnknown: %w", err)
	}
	v.TxUnknown = txUnknown

	return nil
}

func (t *TxUnknownCount) MarshalPostcard(s *postcard.Serializer) error {
	if err := s.SerializeStr(t.Kind); err != nil {
		return fmt.Errorf("failed to serialize Kind: %w", err)
	}
	if err := s.SerializeStr(t.Value); err != nil {
		return fmt.Errorf("failed to serialize Value: %w", err)
	}
	if err := s.SerializeU64(t.Count); err != nil {
		return fmt.Errorf("failed to serialize Count: %w", err)
	}
	return nil
}

func (t *TxUnknownCount) UnmarshalPostcard(d *postcard.Deserializer) error {
	kind, err := d.DeserializeStr()
	if err != nil {
		return fmt.Errorf("failed to deserialize Kind: %w", err)
	}
	t.Kind = kind

	value, err := d.DeserializeStr()
	if err != nil {
		return fmt.Errorf("failed to deserialize Value: %w", err)
	}
	t.Value = value

	count, err := d.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Count: %w", err)
	}
	t.Count = count

	return nil
}
