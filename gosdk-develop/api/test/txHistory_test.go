package test_test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/provider"
	"github.com/stretchr/testify/assert"
)

// eventTagByName reads an event type_tag from the loaded IDL, so the test never
// hardcodes a value that the IDL later corrected.
func eventTagByName(t *testing.T, pd *provider.Provider, name string) uint64 {
	for _, event := range pd.IDL.Events {
		if event.Name == name {
			return event.TypeTag
		}
	}
	t.Fatalf("event %s not found in IDL", name)
	return 0
}

// resourceTagByName reads a resource type_tag from the loaded IDL.
func resourceTagByName(t *testing.T, pd *provider.Provider, name string) uint64 {
	resource, ok := pd.GetResourceByName(name)
	if !ok {
		t.Fatalf("resource %s not found in IDL", name)
	}
	return resource.TypeTag
}

// typeTagByName reads a raw type type_tag from the IDL types section.
func typeTagByName(t *testing.T, pd *provider.Provider, name string) uint64 {
	for _, idlType := range pd.IDL.Types {
		if idlType.Name == name {
			return idlType.TypeTag
		}
	}
	t.Fatalf("type %s not found in IDL", name)
	return 0
}

func TestTxHistory_WithRealProvider_EventCreditApplied(t *testing.T) {
	// 1. Load real provider from IDL
	pd, err := provider.LoadProviderFromFile("../../provider/IDL/demo.idl.json")
	assert.NoError(t, err)

	accountPD, err := provider.LoadProviderFromFile("../../provider/IDL/account.idl.json")
	assert.NoError(t, err)

	creditAppliedTag := eventTagByName(t, pd, "EventCreditApplied")
	scoreTag := resourceTagByName(t, pd, "_Score")
	accountTag := resourceTagByName(t, accountPD, "_Account")

	// 2. Build event data for EventCreditApplied
	// Fields: pool(Address, 20B), recipient(Address, 20B), amount(u64)
	pool := make([]byte, 20)
	for i := 0; i < 20; i++ {
		pool[i] = byte(i + 1)
	}

	recipient := make([]byte, 20)
	for i := 0; i < 20; i++ {
		recipient[i] = byte(i + 101)
	}

	amountSer := postcard.NewSerializer()
	err = amountSer.SerializeU64(42)
	assert.NoError(t, err)
	amountBytes := amountSer.Bytes()

	eventValue := make([]byte, 0, 40+len(amountBytes))
	eventValue = append(eventValue, pool...)
	eventValue = append(eventValue, recipient...)
	eventValue = append(eventValue, amountBytes...)

	// 3. Build TxHistory
	signer, err := crypto.NewAddressFromBytes([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20})
	assert.NoError(t, err)

	original := api.TxHistory{
		Stamp: 100,
		Payer: func() *uint8 { v := uint8(0); return &v }(),
		Signatures: []api.TxHistorySignature{
			{Signer: *signer, AuthBit: 1, SigBit: 2},
		},
		Instructions: []api.PackedInstruction{{1, 2, 3}},
		Receipt: api.TxReceipt{
			TxID:   api.TxId{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			TxHash: api.TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			State:  1,
			Access: []api.AccessRecord{
				{
					ResourceID:    api.RsHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18},
					FirstSnapshot: nil, // None
					LastWritten: api.PersistedValue{
						Variant:      1, // External(BlobHash)
						ExternalHash: [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
					},
				},
				{
					ResourceID: api.RsHash{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19},
					LastWritten: api.PersistedValue{
						Variant:    0,           // Inline(AnySerializeOwned), no length prefix
						TypeTag:    scoreTag,    // demo _Score (u64)
						InlineData: amountBytes, // varint-encoded u64 (42)
					},
				},
				{
					ResourceID: api.RsHash{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
					LastWritten: api.PersistedValue{
						Variant: 0,          // Inline(AnySerializeOwned), no length prefix
						TypeTag: accountTag, // account _Account resource
						// bitmap=7 (u64 varint), weight=6 (u8), threshold=4 (u8), last_modified_block=183 (u64 varint)
						InlineData: []byte{7, 6, 4, 0xB7, 0x01},
					},
				},
			},
			Events: []api.TypeTagWithData{
				{TypeTag: creditAppliedTag, Value: eventValue},
			},
			Error:      nil,
			GasCharged: 888,
		},
	}

	// 4. Round-trip test
	data, err := postcard.SerializePostcard(&original)
	assert.NoError(t, err)

	deserialized, err := postcard.DeserializePostcardWithResolver(data, func(d *postcard.Deserializer) (api.TxHistory, error) {
		var h api.TxHistory
		if err = h.UnmarshalPostcard(d); err != nil {
			return h, err
		}
		return h, nil
	}, false, &provider.IDLTypeResolver{
		Providers: map[string]*provider.Provider{"demo": pd, "account": accountPD},
	})
	assert.NoError(t, err)

	assert.Equal(t, original.Stamp, deserialized.Stamp)
	assert.Equal(t, original.Payer, deserialized.Payer)

	// Signatures
	for i := range original.Signatures {
		assert.Equal(t, original.Signatures[i].Signer.Bytes, deserialized.Signatures[i].Signer.Bytes)
		assert.Equal(t, original.Signatures[i].AuthBit, deserialized.Signatures[i].AuthBit)
		assert.Equal(t, original.Signatures[i].SigBit, deserialized.Signatures[i].SigBit)
	}

	// Instructions
	for i := range original.Instructions {
		assert.Equal(t, []byte(original.Instructions[i]), []byte(deserialized.Instructions[i]))
	}

	// Receipt
	assert.Equal(t, original.Receipt.TxID, deserialized.Receipt.TxID)
	assert.Equal(t, original.Receipt.TxHash, deserialized.Receipt.TxHash)
	assert.Equal(t, original.Receipt.State, deserialized.Receipt.State)
	assert.Equal(t, original.Receipt.Access, deserialized.Receipt.Access)
	assert.Equal(t, original.Receipt.Events, deserialized.Receipt.Events)
	assert.Equal(t, original.Receipt.Error, deserialized.Receipt.Error)
	assert.Equal(t, original.Receipt.GasCharged, deserialized.Receipt.GasCharged)

	// The inline Account value must decode as the full 4-field struct. Regression
	// for a missing last_modified_block field in account.idl.json: the resolver
	// consumed 3 of 5 bytes, desyncing the stream into "invalid postcard boolean".
	accountValue, err := accountPD.DecodeDataByIDLTypeName("Account", deserialized.Receipt.Access[2].LastWritten.InlineData)
	assert.NoError(t, err)
	accountRecord, ok := accountValue.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, uint64(7), accountRecord["bitmap"])
	assert.Equal(t, uint8(6), accountRecord["weight"])
	assert.Equal(t, uint8(4), accountRecord["threshold"])
	assert.Equal(t, uint64(183), accountRecord["last_modified_block"])
}

func TestTxHistory_WithError(t *testing.T) {
	code := uint16(123)
	original := api.TxHistory{
		Stamp:        1,
		Payer:        nil,
		Signatures:   []api.TxHistorySignature{},
		Instructions: []api.PackedInstruction{},
		Receipt: api.TxReceipt{
			TxID:       api.TxId{1},
			TxHash:     api.TxHash{2},
			State:      1,
			Access:     []api.AccessRecord{},
			Events:     []api.TypeTagWithData{},
			Error:      &code,
			GasCharged: 0,
		},
	}

	data, err := postcard.SerializePostcard(&original)
	assert.NoError(t, err)

	deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxHistory, error) {
		var h api.TxHistory
		if err := h.UnmarshalPostcard(d); err != nil {
			return h, err
		}
		return h, nil
	}, false)
	assert.NoError(t, err)
	assert.Equal(t, &code, deserialized.Receipt.Error)
	assert.Equal(t, original.Receipt, deserialized.Receipt)
}

// newEmptyTxHistory builds a minimal valid history used by the format byte tests.
func newEmptyTxHistory() api.TxHistory {
	return api.TxHistory{
		Stamp:        100,
		Payer:        nil,
		Signatures:   []api.TxHistorySignature{},
		Instructions: []api.PackedInstruction{},
		Receipt: api.TxReceipt{
			TxID:       api.TxId{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			TxHash:     api.TxHash{},
			State:      api.TxStatePreFail,
			Access:     []api.AccessRecord{},
			Events:     []api.TypeTagWithData{},
			Error:      nil,
			GasCharged: 0,
		},
	}
}

// The chain encodes a persisted history as [format byte][postcard body] (MIP-33).
func TestTxHistory_FormatByte(t *testing.T) {
	assert.Equal(t, uint8(0), api.TxStatePending)
	assert.Equal(t, uint8(1), api.TxStateSuccess)
	assert.Equal(t, uint8(2), api.TxStateFailed)
	assert.Equal(t, uint8(3), api.TxStatePreFail)

	original := newEmptyTxHistory()
	data, err := postcard.SerializePostcard(&original)
	assert.NoError(t, err)
	assert.Equal(t, api.TxHistoryTypeBase, data[0])

	decode := func(b []byte) (api.TxHistory, error) {
		return postcard.DeserializePostcard(b, func(d *postcard.Deserializer) (api.TxHistory, error) {
			var h api.TxHistory
			if err := h.UnmarshalPostcard(d); err != nil {
				return h, err
			}
			return h, nil
		}, false)
	}

	t.Run("round trip", func(t *testing.T) {
		decoded, err := decode(data)
		assert.NoError(t, err)
		assert.Equal(t, original.Receipt, decoded.Receipt)
	})

	t.Run("unknown format byte should fail", func(t *testing.T) {
		_, err := decode(append([]byte{0x01}, data[1:]...))
		assert.ErrorContains(t, err, "unknown transaction history format byte")
	})

	t.Run("body without format byte should fail", func(t *testing.T) {
		_, err := decode(data[1:])
		assert.Error(t, err)
	})
}

func TestTxHistory_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (api.TxHistory, error) {
			var h api.TxHistory
			if err := h.UnmarshalPostcard(d); err != nil {
				return h, err
			}
			return h, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only stamp", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{0x64}, func(d *postcard.Deserializer) (api.TxHistory, error) {
			var h api.TxHistory
			if err := h.UnmarshalPostcard(d); err != nil {
				return h, err
			}
			return h, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := api.TxHistory{
			Stamp:        100,
			Payer:        nil,
			Signatures:   []api.TxHistorySignature{},
			Instructions: []api.PackedInstruction{},
			Receipt: api.TxReceipt{
				TxID:       api.TxId{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
				TxHash:     api.TxHash{},
				State:      0,
				Access:     []api.AccessRecord{},
				Events:     []api.TypeTagWithData{},
				Error:      nil,
				GasCharged: 12,
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.TxHistory, error) {
			var h api.TxHistory
			if err = h.UnmarshalPostcard(d); err != nil {
				return h, err
			}
			return h, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed when allowTrailing is true", func(t *testing.T) {
		original := api.TxHistory{
			Stamp:        100,
			Payer:        nil,
			Signatures:   []api.TxHistorySignature{},
			Instructions: []api.PackedInstruction{},
			Receipt: api.TxReceipt{
				TxID:       api.TxId{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
				TxHash:     api.TxHash{},
				State:      0,
				Access:     []api.AccessRecord{},
				Events:     []api.TypeTagWithData{},
				Error:      nil,
				GasCharged: 12,
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.TxHistory, error) {
			var h api.TxHistory
			if err = h.UnmarshalPostcard(d); err != nil {
				return h, err
			}
			return h, nil
		}, true)
		assert.NoError(t, err)
	})
}
