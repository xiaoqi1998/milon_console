package test

import (
	"github.com/milon-labs/milon-go-sdk/api"
	"testing"

	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestTxHistoryView_MarshalPostcard(t *testing.T) {
	t.Run("round trip with all fields", func(t *testing.T) {
		before := []byte("before-value")
		errVal := uint16(42)

		original := api.TxHistoryView{
			Stamp: 123456789,
			Instructions: []api.PackedInstruction{
				{0x01, 0x02, 0x03},
				{0x04, 0x05},
			},
			Receipt: api.TxReceiptView{
				TxId:   api.TxId{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
				TxHash: api.TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
				State:  api.TxStateSuccess,
				AccessResourceChanges: []api.AccessResourceChange{
					{
						ResourceHash: api.RsHash{10, 20, 30},
						Change: api.AccessChange{
							Variant: 0,
							TypeTag: 42,
							Before:  &before,
							After:   []byte("after-value"),
						},
					},
				},
				Events: []api.TxReceiptViewEvent{
					{EventIndex: 1, TypeTag: 100, Data: []byte("event1")},
					{EventIndex: 2, TypeTag: 200, Data: []byte("event2")},
				},
				Error: &errVal,
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			var rsp api.TxHistoryView
			if err = rsp.UnmarshalPostcard(d); err != nil {
				return rsp, err
			}
			return rsp, nil
		}, false)

		assert.NoError(t, err)

		assert.Equal(t, original.Stamp, deserialized.Stamp)

		for i := range original.Instructions {
			assert.Equal(t, original.Instructions[i], deserialized.Instructions[i])
		}

		assert.Equal(t, original.Receipt.TxId, deserialized.Receipt.TxId)
		assert.Equal(t, original.Receipt.TxHash, deserialized.Receipt.TxHash)
		assert.Equal(t, original.Receipt.State, deserialized.Receipt.State)

		assert.Equal(t, original.Receipt.AccessResourceChanges, deserialized.Receipt.AccessResourceChanges)
		for i := range original.Receipt.AccessResourceChanges {
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].ResourceHash, deserialized.Receipt.AccessResourceChanges[i].ResourceHash)
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].Change.Variant, deserialized.Receipt.AccessResourceChanges[i].Change.Variant)
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].Change.TypeTag, deserialized.Receipt.AccessResourceChanges[i].Change.TypeTag)
		}

		assert.Equal(t, original.Receipt.Events, deserialized.Receipt.Events)
		for i := range original.Receipt.Events {
			assert.Equal(t, original.Receipt.Events[i].EventIndex, deserialized.Receipt.Events[i].EventIndex)
			assert.Equal(t, original.Receipt.Events[i].TypeTag, deserialized.Receipt.Events[i].TypeTag)
			assert.Equal(t, original.Receipt.Events[i].Data, deserialized.Receipt.Events[i].Data)
		}

		assert.Equal(t, original.Receipt.Error, deserialized.Receipt.Error)
	})

	t.Run("round trip with empty fields", func(t *testing.T) {
		original := api.TxHistoryView{
			Stamp:        0,
			Instructions: nil,
			Receipt: api.TxReceiptView{
				TxId:                  api.TxId{},
				TxHash:                api.TxHash{},
				State:                 api.TxStatePending,
				AccessResourceChanges: make([]api.AccessResourceChange, 0),
				Events:                make([]api.TxReceiptViewEvent, 0),
				Error:                 nil,
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var deserialized api.TxHistoryView
		_, err = postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			if err := deserialized.UnmarshalPostcard(d); err != nil {
				return deserialized, err
			}
			return deserialized, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.Stamp, deserialized.Stamp)

		for i := range original.Instructions {
			assert.Equal(t, original.Instructions[i], deserialized.Instructions[i])
		}

		assert.Equal(t, original.Receipt.TxId, deserialized.Receipt.TxId)
		assert.Equal(t, original.Receipt.TxHash, deserialized.Receipt.TxHash)
		assert.Equal(t, original.Receipt.State, deserialized.Receipt.State)

		assert.Equal(t, original.Receipt.AccessResourceChanges, deserialized.Receipt.AccessResourceChanges)
		for i := range original.Receipt.AccessResourceChanges {
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].ResourceHash, deserialized.Receipt.AccessResourceChanges[i].ResourceHash)
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].Change.Variant, deserialized.Receipt.AccessResourceChanges[i].Change.Variant)
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].Change.TypeTag, deserialized.Receipt.AccessResourceChanges[i].Change.TypeTag)
		}

		assert.Equal(t, original.Receipt.Events, deserialized.Receipt.Events)
		for i := range original.Receipt.Events {
			assert.Equal(t, original.Receipt.Events[i].EventIndex, deserialized.Receipt.Events[i].EventIndex)
			assert.Equal(t, original.Receipt.Events[i].TypeTag, deserialized.Receipt.Events[i].TypeTag)
			assert.Equal(t, original.Receipt.Events[i].Data, deserialized.Receipt.Events[i].Data)
		}

		assert.Equal(t, original.Receipt.Error, deserialized.Receipt.Error)
	})

	t.Run("round trip with BlobHash variant", func(t *testing.T) {
		beforeHash := make([]byte, 32)
		for i := 0; i < 32; i++ {
			beforeHash[i] = byte(i)
		}
		afterHash := make([]byte, 32)
		for i := 0; i < 32; i++ {
			afterHash[i] = byte(0xFF - i)
		}

		original := api.TxHistoryView{
			Receipt: api.TxReceiptView{
				TxId:   api.TxId{1},
				TxHash: api.TxHash{2},
				State:  api.TxStateSuccess,
				AccessResourceChanges: []api.AccessResourceChange{
					{
						ResourceHash: api.RsHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18},
						Change: api.AccessChange{
							Variant: 1,
							Before:  &beforeHash,
							After:   afterHash,
						},
					},
				},
				Events: make([]api.TxReceiptViewEvent, 0),
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var deserialized api.TxHistoryView
		_, err = postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			if err := deserialized.UnmarshalPostcard(d); err != nil {
				return deserialized, err
			}
			return deserialized, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.Stamp, deserialized.Stamp)

		for i := range original.Instructions {
			assert.Equal(t, original.Instructions[i], deserialized.Instructions[i])
		}

		assert.Equal(t, original.Receipt.TxId, deserialized.Receipt.TxId)
		assert.Equal(t, original.Receipt.TxHash, deserialized.Receipt.TxHash)
		assert.Equal(t, original.Receipt.State, deserialized.Receipt.State)

		assert.Equal(t, original.Receipt.AccessResourceChanges, deserialized.Receipt.AccessResourceChanges)
		for i := range original.Receipt.AccessResourceChanges {
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].ResourceHash, deserialized.Receipt.AccessResourceChanges[i].ResourceHash)
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].Change.Variant, deserialized.Receipt.AccessResourceChanges[i].Change.Variant)
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].Change.TypeTag, deserialized.Receipt.AccessResourceChanges[i].Change.TypeTag)
		}

		assert.Equal(t, original.Receipt.Events, deserialized.Receipt.Events)
		for i := range original.Receipt.Events {
			assert.Equal(t, original.Receipt.Events[i].EventIndex, deserialized.Receipt.Events[i].EventIndex)
			assert.Equal(t, original.Receipt.Events[i].TypeTag, deserialized.Receipt.Events[i].TypeTag)
			assert.Equal(t, original.Receipt.Events[i].Data, deserialized.Receipt.Events[i].Data)
		}

		assert.Equal(t, original.Receipt.Error, deserialized.Receipt.Error)
	})

	t.Run("round trip with Inline variant nil before", func(t *testing.T) {
		original := api.TxHistoryView{
			Receipt: api.TxReceiptView{
				TxId:   api.TxId{1},
				TxHash: api.TxHash{2},
				State:  api.TxStatePending,
				AccessResourceChanges: []api.AccessResourceChange{
					{
						ResourceHash: api.RsHash{0xAA, 0xBB, 0xCC},
						Change: api.AccessChange{
							Variant: 0,
							TypeTag: 999,
							Before:  nil,
							After:   []byte("only-after"),
						},
					},
				},
				Events: make([]api.TxReceiptViewEvent, 0),
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var deserialized api.TxHistoryView
		_, err = postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			if err := deserialized.UnmarshalPostcard(d); err != nil {
				return deserialized, err
			}
			return deserialized, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.Stamp, deserialized.Stamp)

		for i := range original.Instructions {
			assert.Equal(t, original.Instructions[i], deserialized.Instructions[i])
		}

		assert.Equal(t, original.Receipt.TxId, deserialized.Receipt.TxId)
		assert.Equal(t, original.Receipt.TxHash, deserialized.Receipt.TxHash)
		assert.Equal(t, original.Receipt.State, deserialized.Receipt.State)

		assert.Equal(t, original.Receipt.AccessResourceChanges, deserialized.Receipt.AccessResourceChanges)
		for i := range original.Receipt.AccessResourceChanges {
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].ResourceHash, deserialized.Receipt.AccessResourceChanges[i].ResourceHash)
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].Change.Variant, deserialized.Receipt.AccessResourceChanges[i].Change.Variant)
			assert.Equal(t, original.Receipt.AccessResourceChanges[i].Change.TypeTag, deserialized.Receipt.AccessResourceChanges[i].Change.TypeTag)
		}

		assert.Equal(t, original.Receipt.Events, deserialized.Receipt.Events)
		for i := range original.Receipt.Events {
			assert.Equal(t, original.Receipt.Events[i].EventIndex, deserialized.Receipt.Events[i].EventIndex)
			assert.Equal(t, original.Receipt.Events[i].TypeTag, deserialized.Receipt.Events[i].TypeTag)
			assert.Equal(t, original.Receipt.Events[i].Data, deserialized.Receipt.Events[i].Data)
		}

		assert.Equal(t, original.Receipt.Error, deserialized.Receipt.Error)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		v1 := api.TxHistoryView{
			Stamp: 42,
			Instructions: []api.PackedInstruction{
				{1, 2, 3},
			},
			Receipt: api.TxReceiptView{
				TxId:   api.TxId{1},
				TxHash: api.TxHash{2},
				State:  api.TxStateSuccess,
				AccessResourceChanges: []api.AccessResourceChange{
					{
						ResourceHash: api.RsHash{1, 2, 3},
						Change: api.AccessChange{
							Variant: 0,
							TypeTag: 10,
							Before:  nil,
							After:   []byte("data"),
						},
					},
				},
				Events: []api.TxReceiptViewEvent{
					{EventIndex: 0, TypeTag: 1, Data: []byte("ev")},
				},
				Error: nil,
			},
		}
		v2 := api.TxHistoryView{
			Stamp: 42,
			Instructions: []api.PackedInstruction{
				{1, 2, 3},
			},
			Receipt: api.TxReceiptView{
				TxId:   api.TxId{1},
				TxHash: api.TxHash{2},
				State:  api.TxStateSuccess,
				AccessResourceChanges: []api.AccessResourceChange{
					{
						ResourceHash: api.RsHash{1, 2, 3},
						Change: api.AccessChange{
							Variant: 0,
							TypeTag: 10,
							Before:  nil,
							After:   []byte("data"),
						},
					},
				},
				Events: []api.TxReceiptViewEvent{
					{EventIndex: 0, TypeTag: 1, Data: []byte("ev")},
				},
				Error: nil,
			},
		}

		b1, err := postcard.SerializePostcard(&v1)
		assert.NoError(t, err)

		b2, err := postcard.SerializePostcard(&v2)
		assert.NoError(t, err)

		assert.Equal(t, b1, b2)
	})
}

func TestTxReceiptView_MarshalPostcard(t *testing.T) {
	t.Run("round trip with all fields", func(t *testing.T) {
		before := []byte("before-value")
		after := []byte("after-value")
		errVal := uint16(42)

		original := api.TxReceiptView{
			TxId:   api.TxId{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			TxHash: api.TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			State:  api.TxStateSuccess,
			AccessResourceChanges: []api.AccessResourceChange{
				{
					ResourceHash: api.RsHash{10, 20, 30},
					Change: api.AccessChange{
						Variant: 0,
						TypeTag: 42,
						Before:  &before,
						After:   after,
					},
				},
			},
			Events: []api.TxReceiptViewEvent{
				{EventIndex: 1, TypeTag: 100, Data: []byte("event1")},
				{EventIndex: 2, TypeTag: 200, Data: []byte("event2")},
			},
			Error: &errVal,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var deserialized api.TxReceiptView
		_, err = postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxReceiptView, error) {
			if err := deserialized.UnmarshalPostcard(d); err != nil {
				return deserialized, err
			}
			return deserialized, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.TxId, deserialized.TxId)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Equal(t, original.State, deserialized.State)

		assert.Equal(t, original.AccessResourceChanges, deserialized.AccessResourceChanges)
		for i := range original.AccessResourceChanges {
			assert.Equal(t, original.AccessResourceChanges[i].ResourceHash, deserialized.AccessResourceChanges[i].ResourceHash)
			assert.Equal(t, original.AccessResourceChanges[i].Change.Variant, deserialized.AccessResourceChanges[i].Change.Variant)
			assert.Equal(t, original.AccessResourceChanges[i].Change.TypeTag, deserialized.AccessResourceChanges[i].Change.TypeTag)
		}

		assert.Equal(t, original.Events, deserialized.Events)
		for i := range original.Events {
			assert.Equal(t, original.Events[i].EventIndex, deserialized.Events[i].EventIndex)
			assert.Equal(t, original.Events[i].TypeTag, deserialized.Events[i].TypeTag)
			assert.Equal(t, original.Events[i].Data, deserialized.Events[i].Data)
		}

		assert.Equal(t, original.Error, deserialized.Error)
	})

	t.Run("round trip with empty fields", func(t *testing.T) {
		original := api.TxReceiptView{
			TxId:                  api.TxId{},
			TxHash:                api.TxHash{},
			State:                 api.TxStatePending,
			AccessResourceChanges: make([]api.AccessResourceChange, 0),
			Events:                make([]api.TxReceiptViewEvent, 0),
			Error:                 nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var deserialized api.TxReceiptView
		_, err = postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxReceiptView, error) {
			if err := deserialized.UnmarshalPostcard(d); err != nil {
				return deserialized, err
			}
			return deserialized, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.TxId, deserialized.TxId)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Equal(t, original.State, deserialized.State)

		assert.Equal(t, original.AccessResourceChanges, deserialized.AccessResourceChanges)
		for i := range original.AccessResourceChanges {
			assert.Equal(t, original.AccessResourceChanges[i].ResourceHash, deserialized.AccessResourceChanges[i].ResourceHash)
			assert.Equal(t, original.AccessResourceChanges[i].Change.Variant, deserialized.AccessResourceChanges[i].Change.Variant)
			assert.Equal(t, original.AccessResourceChanges[i].Change.TypeTag, deserialized.AccessResourceChanges[i].Change.TypeTag)
		}

		assert.Equal(t, original.Events, deserialized.Events)
		for i := range original.Events {
			assert.Equal(t, original.Events[i].EventIndex, deserialized.Events[i].EventIndex)
			assert.Equal(t, original.Events[i].TypeTag, deserialized.Events[i].TypeTag)
			assert.Equal(t, original.Events[i].Data, deserialized.Events[i].Data)
		}

		assert.Equal(t, original.Error, deserialized.Error)
	})

	t.Run("round trip with BlobHash variant", func(t *testing.T) {
		beforeHash := make([]byte, 32)
		for i := 0; i < 32; i++ {
			beforeHash[i] = byte(i)
		}
		afterHash := make([]byte, 32)
		for i := 0; i < 32; i++ {
			afterHash[i] = byte(0xFF - i)
		}

		original := api.TxReceiptView{
			TxId:   api.TxId{1},
			TxHash: api.TxHash{2},
			State:  api.TxStateSuccess,
			AccessResourceChanges: []api.AccessResourceChange{
				{
					ResourceHash: api.RsHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18},
					Change: api.AccessChange{
						Variant: 1,
						Before:  &beforeHash,
						After:   afterHash,
					},
				},
			},
			Events: make([]api.TxReceiptViewEvent, 0),
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var deserialized api.TxReceiptView
		_, err = postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxReceiptView, error) {
			if err := deserialized.UnmarshalPostcard(d); err != nil {
				return deserialized, err
			}
			return deserialized, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.TxId, deserialized.TxId)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Equal(t, original.State, deserialized.State)

		assert.Equal(t, original.AccessResourceChanges, deserialized.AccessResourceChanges)
		for i := range original.AccessResourceChanges {
			assert.Equal(t, original.AccessResourceChanges[i].ResourceHash, deserialized.AccessResourceChanges[i].ResourceHash)
			assert.Equal(t, original.AccessResourceChanges[i].Change.Variant, deserialized.AccessResourceChanges[i].Change.Variant)
			assert.Equal(t, original.AccessResourceChanges[i].Change.TypeTag, deserialized.AccessResourceChanges[i].Change.TypeTag)
		}

		assert.Equal(t, original.Events, deserialized.Events)
		for i := range original.Events {
			assert.Equal(t, original.Events[i].EventIndex, deserialized.Events[i].EventIndex)
			assert.Equal(t, original.Events[i].TypeTag, deserialized.Events[i].TypeTag)
			assert.Equal(t, original.Events[i].Data, deserialized.Events[i].Data)
		}

		assert.Equal(t, original.Error, deserialized.Error)
	})

	t.Run("round trip with failed state and error", func(t *testing.T) {
		errVal := uint16(9999)
		original := api.TxReceiptView{
			TxId:                  api.TxId{0xAA},
			TxHash:                api.TxHash{0xBB},
			State:                 api.TxStateFailed,
			AccessResourceChanges: make([]api.AccessResourceChange, 0),
			Events:                make([]api.TxReceiptViewEvent, 0),
			Error:                 &errVal,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var deserialized api.TxReceiptView
		_, err = postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxReceiptView, error) {
			if err := deserialized.UnmarshalPostcard(d); err != nil {
				return deserialized, err
			}
			return deserialized, nil
		}, false)
		assert.NoError(t, err)

		assert.Equal(t, original.TxId, deserialized.TxId)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Equal(t, original.State, deserialized.State)

		assert.Equal(t, original.AccessResourceChanges, deserialized.AccessResourceChanges)
		for i := range original.AccessResourceChanges {
			assert.Equal(t, original.AccessResourceChanges[i].ResourceHash, deserialized.AccessResourceChanges[i].ResourceHash)
			assert.Equal(t, original.AccessResourceChanges[i].Change.Variant, deserialized.AccessResourceChanges[i].Change.Variant)
			assert.Equal(t, original.AccessResourceChanges[i].Change.TypeTag, deserialized.AccessResourceChanges[i].Change.TypeTag)
		}

		assert.Equal(t, original.Events, deserialized.Events)
		for i := range original.Events {
			assert.Equal(t, original.Events[i].EventIndex, deserialized.Events[i].EventIndex)
			assert.Equal(t, original.Events[i].TypeTag, deserialized.Events[i].TypeTag)
			assert.Equal(t, original.Events[i].Data, deserialized.Events[i].Data)
		}

		assert.Equal(t, original.Error, deserialized.Error)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		r1 := api.TxReceiptView{
			TxId:   api.TxId{1},
			TxHash: api.TxHash{2},
			State:  api.TxStateSuccess,
		}
		r2 := api.TxReceiptView{
			TxId:   api.TxId{1},
			TxHash: api.TxHash{2},
			State:  api.TxStateSuccess,
		}

		b1, err := postcard.SerializePostcard(&r1)
		assert.NoError(t, err)

		b2, err := postcard.SerializePostcard(&r2)
		assert.NoError(t, err)

		assert.Equal(t, b1, b2)
	})
}

func TestTxReceiptViewEvent_MarshalPostcard(t *testing.T) {
	t.Run("round trip with data", func(t *testing.T) {
		original := api.TxReceiptViewEvent{
			EventIndex: 42,
			TypeTag:    1000,
			Data:       []byte("event payload"),
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var deserialized api.TxReceiptViewEvent
		_, err = postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxReceiptViewEvent, error) {
			if err := deserialized.UnmarshalPostcard(d); err != nil {
				return deserialized, err
			}
			return deserialized, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.EventIndex, deserialized.EventIndex)
		assert.Equal(t, original.TypeTag, deserialized.TypeTag)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("round trip with empty data", func(t *testing.T) {
		original := api.TxReceiptViewEvent{
			EventIndex: 0,
			TypeTag:    0,
			Data:       []byte{},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var deserialized api.TxReceiptViewEvent
		_, err = postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (api.TxReceiptViewEvent, error) {
			if err := deserialized.UnmarshalPostcard(d); err != nil {
				return deserialized, err
			}
			return deserialized, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, uint32(0), deserialized.EventIndex)
		assert.Equal(t, uint64(0), deserialized.TypeTag)
		assert.Empty(t, deserialized.Data)
	})

	t.Run("error on truncated data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{0x01}, func(d *postcard.Deserializer) (api.TxReceiptViewEvent, error) {
			var ev api.TxReceiptViewEvent
			if err := ev.UnmarshalPostcard(d); err != nil {
				return ev, err
			}
			return ev, nil
		}, false)
		assert.Error(t, err)
	})
}

func TestAccessChange_MarshalPostcard(t *testing.T) {
	t.Run("Inline variant with before and after", func(t *testing.T) {
		before := []byte("before-data")
		original := api.AccessChange{
			Variant: 0,
			TypeTag: 42,
			Before:  &before,
			After:   []byte("after-data"),
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var ac api.AccessChange
		err = ac.UnmarshalPostcard(postcard.NewDeserializer(data))
		assert.NoError(t, err)

		assert.Equal(t, original.Variant, ac.Variant)
		assert.Equal(t, original.TypeTag, ac.TypeTag)
		assert.Equal(t, original.Before, ac.Before)
		assert.Equal(t, original.After, ac.After)
	})

	t.Run("Inline variant with nil before", func(t *testing.T) {
		original := api.AccessChange{
			Variant: 0,
			TypeTag: 99,
			Before:  nil,
			After:   []byte("only-after"),
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var ac api.AccessChange
		err = ac.UnmarshalPostcard(postcard.NewDeserializer(data))
		assert.NoError(t, err)

		assert.Equal(t, original.Variant, ac.Variant)
		assert.Equal(t, original.TypeTag, ac.TypeTag)
		assert.Equal(t, original.Before, ac.Before)
		assert.Equal(t, original.After, ac.After)
	})

	t.Run("BlobHash variant with before", func(t *testing.T) {
		beforeHash := make([]byte, 32)
		for i := 0; i < 32; i++ {
			beforeHash[i] = byte(i)
		}
		afterHash := make([]byte, 32)
		for i := 0; i < 32; i++ {
			afterHash[i] = byte(0xFF - i)
		}

		original := api.AccessChange{
			Variant: 1,
			Before:  &beforeHash,
			After:   afterHash,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var ac api.AccessChange
		err = ac.UnmarshalPostcard(postcard.NewDeserializer(data))
		assert.NoError(t, err)

		assert.Equal(t, original.Variant, ac.Variant)
		assert.Equal(t, original.TypeTag, ac.TypeTag)
		assert.Equal(t, original.Before, ac.Before)
		assert.Equal(t, original.After, ac.After)
	})

	t.Run("BlobHash variant with nil before", func(t *testing.T) {
		afterHash := make([]byte, 32)
		for i := 0; i < 32; i++ {
			afterHash[i] = byte(i)
		}

		original := api.AccessChange{
			Variant: 1,
			Before:  nil,
			After:   afterHash,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		var ac api.AccessChange
		err = ac.UnmarshalPostcard(postcard.NewDeserializer(data))
		assert.NoError(t, err)

		assert.Equal(t, original.Variant, ac.Variant)
		assert.Equal(t, original.TypeTag, ac.TypeTag)
		assert.Equal(t, original.Before, ac.Before)
		assert.Equal(t, original.After, ac.After)
	})

	t.Run("error on unknown variant", func(t *testing.T) {
		original := api.AccessChange{
			Variant: 99,
		}

		_, err := postcard.SerializePostcard(&original)
		assert.Error(t, err)
	})

	t.Run("error on empty data", func(t *testing.T) {
		var ac api.AccessChange
		err := ac.UnmarshalPostcard(postcard.NewDeserializer([]byte{}))
		assert.Error(t, err)
	})
}

func TestTxHistoryView_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			var v api.TxHistoryView
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated stamp", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{0x80}, func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			var v api.TxHistoryView
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := api.TxHistoryView{
			Receipt: api.TxReceiptView{
				TxId:   api.TxId{},
				TxHash: api.TxHash{},
				State:  api.TxStatePending,
			},
		}
		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)
		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			var v api.TxHistoryView
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := api.TxHistoryView{
			Receipt: api.TxReceiptView{
				TxId:   api.TxId{},
				TxHash: api.TxHash{},
				State:  api.TxStatePending,
			},
		}
		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)
		dataWithTrailing := append(data, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			var v api.TxHistoryView
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, true)
		assert.NoError(t, err)
	})

	t.Run("truncated TxId in receipt", func(t *testing.T) {
		// Manually build truncated data: only Stamp + empty instructions + 6 bytes of TxId
		ser := postcard.NewSerializer()
		ser.SerializeU64(0)
		ser.SerializeU32(0) // no instructions
		ser.SerializeFixedBytes(make([]byte, 6))

		_, err := postcard.DeserializePostcard(ser.Bytes(), func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			var v api.TxHistoryView
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("invalid AccessChange variant", func(t *testing.T) {
		ser := postcard.NewSerializer()
		ser.SerializeU64(0)
		ser.SerializeU32(0)                       // no instructions
		ser.SerializeFixedBytes(make([]byte, 12)) // TxId
		ser.SerializeFixedBytes(make([]byte, 32)) // TxHash
		ser.SerializeU8(0)                        // State
		ser.SerializeU32(1)                       // 1 access change
		ser.SerializeFixedBytes(make([]byte, 18)) // RsHash
		ser.SerializeU32(99)                      // invalid variant
		ser.SerializeU32(0)                       // no events
		ser.SerializeBool(false)                  // Error None

		_, err := postcard.DeserializePostcard(ser.Bytes(), func(d *postcard.Deserializer) (api.TxHistoryView, error) {
			var v api.TxHistoryView
			if err := v.UnmarshalPostcard(d); err != nil {
				return v, err
			}
			return v, nil
		}, false)
		assert.Error(t, err)
	})
}
