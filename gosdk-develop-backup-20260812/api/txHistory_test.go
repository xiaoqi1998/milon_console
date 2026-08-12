package api_test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/provider"
	"github.com/stretchr/testify/assert"
)

func TestTxHistory_WithRealProvider_EventCreditApplied(t *testing.T) {
	// 1. Load real provider from IDL (like client.go lines 82-86)
	pd, err := provider.LoadProviderFromFile("../provider/IDL/demo.idl.json")
	assert.NoError(t, err)

	api.SetGlobalTypeResolver(&provider.IDLTypeResolver{
		Providers: map[string]*provider.Provider{"demo": pd},
	})

	// 2. Build event data for EventCreditApplied (typeTag: 7407037194950745602)
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
			},
			Events: []api.TypeTagWithData{
				{TypeTag: 7407037194950745602, Value: eventValue},
			},
			Error:      nil,
			GasCharged: 888,
		},
	}

	// 4. Round-trip test
	data, err := postcard.SerializePostcard(&original)
	assert.NoError(t, err)

	deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxHistory, error) {
		var h api.TxHistory
		if err = h.UnmarshalPostcard(d); err != nil {
			return h, err
		}
		return h, nil
	}, false)
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
