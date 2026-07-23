package api_test

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/provider"
	"github.com/stretchr/testify/assert"
)

func TestSimulateReceipt_WithRealProvider_EventCreditApplied(t *testing.T) {
	// 1. Load real provider from IDL
	pd, err := provider.LoadProviderFromFile("../provider/IDL/demo.idl.json")
	assert.NoError(t, err)

	api.SetGlobalTypeResolver(&provider.IDLTypeResolver{
		Providers: map[string]*provider.Provider{"demo": pd},
	})

	// 2. Build event data for EventCreditApplied (typeTag: 7407037194950745602)
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

	// 3. Build SimulateReceipt
	original := api.SimulateReceipt{
		TxID:   api.TxId{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		TxHash: api.TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		State:  1,
		Access: []api.AccessRecord{
			{
				ResourceID:    api.RsHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18},
				FirstSnapshot: nil,
				LastWritten: api.PersistedValue{
					Variant:      1,
					ExternalHash: [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
				},
			},
		},
		Events: []api.TypeTagWithData{
			{TypeTag: 7407037194950745602, Value: eventValue},
		},
		Error:      nil,
		GasCharged: 22,
	}

	// 4. Round-trip test
	data, err := postcard.SerializePostcard(&original)
	assert.NoError(t, err)

	deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.SimulateReceipt, error) {
		var r api.SimulateReceipt
		if err := r.UnmarshalPostcard(d); err != nil {
			return r, err
		}
		return r, nil
	}, false)
	assert.NoError(t, err)

	assert.Equal(t, original.TxID, deserialized.TxID)
	assert.Equal(t, original.TxHash, deserialized.TxHash)
	assert.Equal(t, original.State, deserialized.State)
	assert.Equal(t, original.Access, deserialized.Access)
	assert.Equal(t, original.Events, deserialized.Events)
	assert.Equal(t, original.Error, deserialized.Error)
}

func TestSimulateReceipt_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (api.SimulateReceipt, error) {
			var r api.SimulateReceipt
			if err := r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only txid", func(t *testing.T) {
		_, err := postcard.DeserializePostcard(make([]byte, 12), func(d *postcard.Deserializer) (api.SimulateReceipt, error) {
			var r api.SimulateReceipt
			if err := r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := api.SimulateReceipt{
			TxID:       api.TxId{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			TxHash:     api.TxHash{},
			State:      0,
			Access:     []api.AccessRecord{},
			Events:     []api.TypeTagWithData{},
			Error:      nil,
			GasCharged: 22,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.SimulateReceipt, error) {
			var r api.SimulateReceipt
			if err := r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed when allowTrailing is true", func(t *testing.T) {
		original := api.SimulateReceipt{
			TxID:       api.TxId{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			TxHash:     api.TxHash{},
			State:      0,
			Access:     []api.AccessRecord{},
			Events:     []api.TypeTagWithData{},
			Error:      nil,
			GasCharged: 22,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.SimulateReceipt, error) {
			var r api.SimulateReceipt
			if err := r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, true)
		assert.NoError(t, err)
	})
}
