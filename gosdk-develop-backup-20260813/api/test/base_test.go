package test

import (
	"encoding/hex"
	"github.com/milon-labs/milon-go-sdk/api"
	"testing"

	"github.com/btcsuite/btcutil/base58"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestNewTxHashFromRelaxed(t *testing.T) {
	hash := api.TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	tests := []struct {
		name    string
		input   any
		want    api.TxHash
		wantErr bool
	}{
		{name: "TxHash value", input: hash, want: hash},
		{name: "*TxHash pointer", input: &hash, want: hash},
		{name: "nil *TxHash", input: (*api.TxHash)(nil), wantErr: true},
		{name: "hex string", input: hex.EncodeToString(hash[:]), want: hash},
		{name: "base58 string", input: base58.Encode(hash[:]), want: hash},
		{name: "hex with wrong length", input: "0102", wantErr: true},
		{name: "base58 with wrong length", input: "a", wantErr: true},
		{name: "invalid string", input: "!", wantErr: true},
		{name: "unsupported type", input: 42, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := api.NewTxHashFromRelaxed(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTxHashString(t *testing.T) {
	hash := api.TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	assert.Equal(t, base58.Encode(hash[:]), hash.String())
}

func TestDeserializeAccessRecord_InlinePersistedValue(t *testing.T) {
	prev := api.GlobalTypeResolver
	api.SetGlobalTypeResolver(nil)
	defer api.SetGlobalTypeResolver(prev)

	ser := postcard.NewSerializer()
	ser.SerializeFixedBytes(make([]byte, api.RsHashLen)) // ResourceID
	ser.SerializeBool(false)                             // FirstSnapshot: None
	ser.SerializeU32(0)                                  // LastWritten variant: Inline
	ser.SerializeU64(42)                                 // type_tag
	ser.SerializeBytes([]byte{1, 2, 3})                  // InlineData (Vec<u8>)

	rec, err := api.DeserializeAccessRecord(postcard.NewDeserializer(ser.Bytes()))
	assert.NoError(t, err)
	assert.Nil(t, rec.FirstSnapshot)
	assert.Equal(t, uint32(0), rec.LastWritten.Variant)
	assert.Equal(t, uint64(42), rec.LastWritten.TypeTag)
	assert.Equal(t, []byte{1, 2, 3}, rec.LastWritten.InlineData)
}

func TestDeserializeAccessRecord_ExternalWithFirstSnapshot(t *testing.T) {
	prev := api.GlobalTypeResolver
	api.SetGlobalTypeResolver(nil)
	defer api.SetGlobalTypeResolver(prev)

	ser := postcard.NewSerializer()
	ser.SerializeFixedBytes(make([]byte, api.RsHashLen))   // ResourceID
	ser.SerializeBool(true)                                // FirstSnapshot: Some
	ser.SerializeU32(1)                                    // FirstSnapshot variant: External
	ser.SerializeFixedBytes(make([]byte, api.BlobHashLen)) // FirstSnapshot BlobHash
	ser.SerializeU32(1)                                    // LastWritten variant: External
	ser.SerializeFixedBytes(make([]byte, api.BlobHashLen)) // LastWritten BlobHash

	rec, err := api.DeserializeAccessRecord(postcard.NewDeserializer(ser.Bytes()))
	assert.NoError(t, err)
	assert.NotNil(t, rec.FirstSnapshot)
	assert.Equal(t, uint32(1), rec.FirstSnapshot.Variant)
	assert.Equal(t, uint32(1), rec.LastWritten.Variant)
	assert.Equal(t, [api.BlobHashLen]byte{}, rec.LastWritten.ExternalHash)
}

func TestPersistedValueSerializeRoundTrip(t *testing.T) {
	t.Run("inline variant serialized format", func(t *testing.T) {
		pv := api.PersistedValue{Variant: 0, TypeTag: 7, InlineData: []byte{9, 8, 7}}
		ser := postcard.NewSerializer()
		assert.NoError(t, api.SerializePersistedValue(ser, pv))

		d := postcard.NewDeserializer(ser.Bytes())
		variant, err := d.DeserializeU32()
		assert.NoError(t, err)
		assert.Equal(t, uint32(0), variant)

		typeTag, err := d.DeserializeU64()
		assert.NoError(t, err)
		assert.Equal(t, uint64(7), typeTag)

		assert.Equal(t, []byte{9, 8, 7}, d.Buffer()[d.Offset():])
	})

	t.Run("external variant round trip", func(t *testing.T) {
		prev := api.GlobalTypeResolver
		api.SetGlobalTypeResolver(nil)
		defer api.SetGlobalTypeResolver(prev)

		pv := api.PersistedValue{Variant: 1, ExternalHash: [api.BlobHashLen]byte{1, 2, 3}}
		ser := postcard.NewSerializer()
		ser.SerializeFixedBytes(make([]byte, api.RsHashLen)) // ResourceID
		ser.SerializeBool(false)                             // FirstSnapshot: None
		assert.NoError(t, api.SerializePersistedValue(ser, pv))

		rec, err := api.DeserializeAccessRecord(postcard.NewDeserializer(ser.Bytes()))
		assert.NoError(t, err)
		assert.Equal(t, pv, rec.LastWritten)
	})
}

func TestPersistedValueUnknownVariant(t *testing.T) {
	prev := api.GlobalTypeResolver
	api.SetGlobalTypeResolver(nil)
	defer api.SetGlobalTypeResolver(prev)

	t.Run("serialize unknown variant", func(t *testing.T) {
		ser := postcard.NewSerializer()
		err := api.SerializePersistedValue(ser, api.PersistedValue{Variant: 2})
		assert.Error(t, err)
	})

	t.Run("deserialize unknown variant", func(t *testing.T) {
		ser := postcard.NewSerializer()
		ser.SerializeFixedBytes(make([]byte, api.RsHashLen))
		ser.SerializeBool(false)
		ser.SerializeU32(2) // unknown variant
		_, err := api.DeserializeAccessRecord(postcard.NewDeserializer(ser.Bytes()))
		assert.Error(t, err)
	})
}

func TestDeserializeEventEntryFallback(t *testing.T) {
	prev := api.GlobalTypeResolver
	api.SetGlobalTypeResolver(nil)
	defer api.SetGlobalTypeResolver(prev)

	ser := postcard.NewSerializer()
	ser.SerializeU64(99)             // type_tag
	ser.SerializeBytes([]byte{1, 2}) // event value (Vec<u8>)

	entry, err := api.DeserializeEventEntry(postcard.NewDeserializer(ser.Bytes()))
	assert.NoError(t, err)
	assert.Equal(t, uint64(99), entry.TypeTag)
	assert.Equal(t, []byte{1, 2}, entry.Value)
}
