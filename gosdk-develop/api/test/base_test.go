package test

import (
	"encoding/hex"
	"github.com/milon-labs/milon-go-sdk/api"
	"testing"

	"github.com/btcsuite/btcutil/base58"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/provider"
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

func TestSerializePersistedValueRoundTrip(t *testing.T) {
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

		// Inline values are length-prefixed (FramedPersistedValue wire format)
		assert.Equal(t, []byte{3, 9, 8, 7}, d.Buffer()[d.Offset():])

		buf, err := d.DeserializeBytes()
		assert.NoError(t, err)
		assert.Equal(t, []byte{9, 8, 7}, buf)
	})

	t.Run("external variant round trip", func(t *testing.T) {
		pv := api.PersistedValue{Variant: 1, ExternalHash: [api.BlobHashLen]byte{1, 2, 3}}
		ser := postcard.NewSerializer()
		ser.SerializeFixedBytes(make([]byte, api.RsHashLen)) // ResourceID
		ser.SerializeBool(false)                             // FirstSnapshot: None
		assert.NoError(t, api.SerializePersistedValue(ser, pv))

		rec, err := api.DeserializeAccessRecord(postcard.NewDeserializer(ser.Bytes()))
		assert.NoError(t, err)
		assert.Equal(t, pv, rec.LastWritten)
	})

	t.Run("inline variant no length prefix", func(t *testing.T) {
		pv := api.PersistedValue{Variant: 0, TypeTag: 7, InlineData: []byte{9, 8, 7}}
		ser := postcard.NewSerializer()
		assert.NoError(t, api.SerializePersistedValueNoLen(ser, pv))

		d := postcard.NewDeserializer(ser.Bytes())
		variant, err := d.DeserializeU32()
		assert.NoError(t, err)
		assert.Equal(t, uint32(0), variant)

		typeTag, err := d.DeserializeU64()
		assert.NoError(t, err)
		assert.Equal(t, uint64(7), typeTag)

		// Inline values carry no length prefix (AnySerializeOwned wire format)
		assert.Equal(t, []byte{9, 8, 7}, d.Buffer()[d.Offset():])

		buf, err := d.DeserializeFixedBytes(3)
		assert.NoError(t, err)
		assert.Equal(t, []byte{9, 8, 7}, buf)
	})
}

func TestPersistedValueUnknownVariant(t *testing.T) {
	t.Run("serialize unknown variant", func(t *testing.T) {
		ser := postcard.NewSerializer()
		err := api.SerializePersistedValue(ser, api.PersistedValue{Variant: 2})
		assert.Error(t, err)
	})

	t.Run("deserialize unknown variant", func(t *testing.T) {
		ser := postcard.NewSerializer()
		ser.SerializeFixedBytes(make([]byte, api.RsHashLen))
		ser.SerializeBool(false) // FirstSnapshot: None
		ser.SerializeU32(2)      // unknown variant
		_, err := api.DeserializeAccessRecord(postcard.NewDeserializer(ser.Bytes()))
		assert.Error(t, err)
	})
}

func TestDeserializeAccessRecordNoLen_InlinePersistedValue(t *testing.T) {
	pd, err := provider.LoadProviderFromFile("../../provider/IDL/system.idl.json")
	assert.NoError(t, err)

	ser := postcard.NewSerializer()
	ser.SerializeFixedBytes(make([]byte, api.RsHashLen)) // ResourceID
	ser.SerializeBool(false)                             // FirstSnapshot: None
	ser.SerializeU32(0)                                  // LastWritten variant: Inline
	ser.SerializeU64(5563585020063213298)                // type_tag: u64 (system builtin)
	ser.SerializeFixedBytes([]byte{42})                  // u64 value, NO length prefix

	rec, err := postcard.DeserializePostcardWithResolver(ser.Bytes(), func(d *postcard.Deserializer) (api.AccessRecord, error) {
		return api.DeserializeAccessRecordNoLen(d)
	}, false, &provider.IDLTypeResolver{Providers: map[string]*provider.Provider{"system": pd}})
	assert.NoError(t, err)
	assert.Equal(t, uint32(0), rec.LastWritten.Variant)
	assert.Equal(t, uint64(5563585020063213298), rec.LastWritten.TypeTag)
	assert.Equal(t, []byte{42}, rec.LastWritten.InlineData)
}

func TestDeserializeEventEntry(t *testing.T) {
	t.Run("with resolver, has length prefix", func(t *testing.T) {
		pd, err := provider.LoadProviderFromFile("../../provider/IDL/demo.idl.json")
		assert.NoError(t, err)

		pool := make([]byte, 20)
		for i := range pool {
			pool[i] = byte(i + 1)
		}
		recipient := make([]byte, 20)
		for i := range recipient {
			recipient[i] = byte(i + 101)
		}

		body := append(append(pool, recipient...), 42) // EventCreditApplied payload: pool + recipient + amount

		ser := postcard.NewSerializer()
		ser.SerializeU64(7407037194950745602) // type_tag: EventCreditApplied (demo event)
		ser.SerializeBytes(body)              // body: Vec<u8>, WITH length prefix

		entry, err := postcard.DeserializePostcardWithResolver(ser.Bytes(), func(d *postcard.Deserializer) (api.TypeTagWithData, error) {
			return api.DeserializeEventEntry(d)
		}, false, &provider.IDLTypeResolver{Providers: map[string]*provider.Provider{"demo": pd}})
		assert.NoError(t, err)
		assert.Equal(t, uint64(7407037194950745602), entry.TypeTag)
		assert.Equal(t, body, entry.Value)
	})
}

func TestDeserializeEventEntryNoLen(t *testing.T) {
	t.Run("fallback without resolver", func(t *testing.T) {
		ser := postcard.NewSerializer()
		ser.SerializeU64(99)             // type_tag
		ser.SerializeBytes([]byte{1, 2}) // value as Vec<u8> (length-prefixed)

		entry, err := api.DeserializeEventEntryNoLen(postcard.NewDeserializer(ser.Bytes()))
		assert.NoError(t, err)
		assert.Equal(t, uint64(99), entry.TypeTag)
		assert.Equal(t, []byte{1, 2}, entry.Value)
	})

	t.Run("with resolver, no length prefix", func(t *testing.T) {
		pd, err := provider.LoadProviderFromFile("../../provider/IDL/demo.idl.json")
		assert.NoError(t, err)

		pool := make([]byte, 20)
		for i := range pool {
			pool[i] = byte(i + 1)
		}
		recipient := make([]byte, 20)
		for i := range recipient {
			recipient[i] = byte(i + 101)
		}

		ser := postcard.NewSerializer()
		ser.SerializeU64(7407037194950745602) // type_tag: EventCreditApplied (demo event)
		ser.SerializeFixedBytes(pool)         // pool (Address, 20B)
		ser.SerializeFixedBytes(recipient)    // recipient (Address, 20B)
		ser.SerializeFixedBytes([]byte{42})   // amount (u64 = 42, varint), NO length prefix

		entry, err := postcard.DeserializePostcardWithResolver(ser.Bytes(), func(d *postcard.Deserializer) (api.TypeTagWithData, error) {
			return api.DeserializeEventEntryNoLen(d)
		}, false, &provider.IDLTypeResolver{Providers: map[string]*provider.Provider{"demo": pd}})
		assert.NoError(t, err)
		assert.Equal(t, uint64(7407037194950745602), entry.TypeTag)

		want := append(append(pool, recipient...), 42)
		assert.Equal(t, want, entry.Value)
	})
}
