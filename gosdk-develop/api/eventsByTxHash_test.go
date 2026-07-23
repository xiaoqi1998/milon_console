package api

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestEventsByTxHashRequest_MarshalPostcard(t *testing.T) {
	t.Run("round trip with TypeTagFilter set", func(t *testing.T) {
		filter := uint64(42)
		original := EventsByTxHashReq{
			TxHash:        TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TypeTagFilter: &filter,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventsByTxHashReq, error) {
			var e EventsByTxHashReq
			if err = e.UnmarshalPostcard(d); err != nil {
				return e, err
			}
			return e, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.NotNil(t, deserialized.TypeTagFilter)
		assert.Equal(t, original.TypeTagFilter, deserialized.TypeTagFilter)
	})

	t.Run("round trip with TypeTagFilter nil", func(t *testing.T) {
		original := EventsByTxHashReq{
			TxHash:        TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TypeTagFilter: nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventsByTxHashReq, error) {
			var e EventsByTxHashReq
			if err = e.UnmarshalPostcard(d); err != nil {
				return e, err
			}
			return e, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Equal(t, original.TypeTagFilter, deserialized.TypeTagFilter)
	})

	t.Run("round trip with zero TxHash and nil filter", func(t *testing.T) {
		original := EventsByTxHashReq{
			TxHash:        TxHash{},
			TypeTagFilter: nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventsByTxHashReq, error) {
			var e EventsByTxHashReq
			if err = e.UnmarshalPostcard(d); err != nil {
				return e, err
			}
			return e, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Nil(t, original.TypeTagFilter, deserialized.TypeTagFilter)
	})

	t.Run("round trip with max uint64 filter", func(t *testing.T) {
		filter := uint64(18446744073709551615)
		original := EventsByTxHashReq{
			TxHash:        TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TypeTagFilter: &filter,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventsByTxHashReq, error) {
			var e EventsByTxHashReq
			if err = e.UnmarshalPostcard(d); err != nil {
				return e, err
			}
			return e, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Equal(t, original.TypeTagFilter, deserialized.TypeTagFilter)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		filter := uint64(100)
		e1 := EventsByTxHashReq{
			TxHash:        TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TypeTagFilter: &filter,
		}
		e2 := EventsByTxHashReq{
			TxHash:        TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TypeTagFilter: &filter,
		}

		data1, err := postcard.SerializePostcard(&e1)
		assert.NoError(t, err)

		data2, err := postcard.SerializePostcard(&e2)
		assert.NoError(t, err)

		assert.Equal(t, data1, data2)
	})
}

func TestEventsByTxHashRequest_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (EventsByTxHashReq, error) {
			var e EventsByTxHashReq
			if err := e.UnmarshalPostcard(d); err != nil {
				return e, err
			}
			return e, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("truncated data - only partial tx hash", func(t *testing.T) {
		_, err := postcard.DeserializePostcard(make([]byte, 32), func(d *postcard.Deserializer) (EventsByTxHashReq, error) {
			var e EventsByTxHashReq
			if err := e.UnmarshalPostcard(d); err != nil {
				return e, err
			}
			return e, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		filter := uint64(1)
		original := EventsByTxHashReq{
			TxHash:        TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TypeTagFilter: &filter,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (EventsByTxHashReq, error) {
			var e EventsByTxHashReq
			if err = e.UnmarshalPostcard(d); err != nil {
				return e, err
			}
			return e, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := EventsByTxHashReq{
			TxHash:        TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TypeTagFilter: nil,
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (EventsByTxHashReq, error) {
			var e EventsByTxHashReq
			if err = e.UnmarshalPostcard(d); err != nil {
				return e, err
			}
			return e, nil
		}, true)
		assert.NoError(t, err)
	})
}

func TestEventEntry_MarshalPostcard(t *testing.T) {
	t.Run("round trip with all fields populated", func(t *testing.T) {
		original := EventEntry{
			BlockHeight: 1000,
			TxHash:      TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TxIndex:     5,
			EventIndex:  99,
			Data: TypeTagWithData{
				TypeTag: 11,
				Value:   []byte{22},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventEntry, error) {
			var entry EventEntry
			if err = entry.UnmarshalPostcard(d); err != nil {
				return entry, err
			}
			return entry, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.BlockHeight, deserialized.BlockHeight)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Equal(t, original.TxIndex, deserialized.TxIndex)
		assert.Equal(t, original.EventIndex, deserialized.EventIndex)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("round trip with zero values", func(t *testing.T) {
		original := EventEntry{
			BlockHeight: 0,
			TxHash:      TxHash{},
			TxIndex:     0,
			EventIndex:  0,
			Data: TypeTagWithData{
				TypeTag: 11,
				Value:   []byte{22},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventEntry, error) {
			var entry EventEntry
			if err = entry.UnmarshalPostcard(d); err != nil {
				return entry, err
			}
			return entry, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.BlockHeight, deserialized.BlockHeight)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Equal(t, original.TxIndex, deserialized.TxIndex)
		assert.Equal(t, original.EventIndex, deserialized.EventIndex)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("round trip with max uint32 and uint64", func(t *testing.T) {
		original := EventEntry{
			BlockHeight: 18446744073709551615,
			TxHash:      TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TxIndex:     4294967295,
			EventIndex:  4294967295,
			Data: TypeTagWithData{
				TypeTag: 11,
				Value:   []byte{22},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventEntry, error) {
			var entry EventEntry
			if err = entry.UnmarshalPostcard(d); err != nil {
				return entry, err
			}
			return entry, nil
		}, false)
		assert.NoError(t, err)
		assert.Equal(t, original.BlockHeight, deserialized.BlockHeight)
		assert.Equal(t, original.TxHash, deserialized.TxHash)
		assert.Equal(t, original.TxIndex, deserialized.TxIndex)
		assert.Equal(t, original.EventIndex, deserialized.EventIndex)
		assert.Equal(t, original.Data, deserialized.Data)
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		e1 := EventEntry{
			BlockHeight: 100,
			TxHash:      TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TxIndex:     10,
			EventIndex:  20,
			Data: TypeTagWithData{
				TypeTag: 11,
				Value:   []byte{22},
			},
		}
		e2 := EventEntry{
			BlockHeight: 100,
			TxHash:      TxHash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			TxIndex:     10,
			EventIndex:  20,
			Data: TypeTagWithData{
				TypeTag: 11,
				Value:   []byte{22},
			},
		}

		data1, err := postcard.SerializePostcard(&e1)
		assert.NoError(t, err)

		data2, err := postcard.SerializePostcard(&e2)
		assert.NoError(t, err)

		assert.Equal(t, data1, data2)
	})
}

func TestEventEntry_DeserializeErrors(t *testing.T) {
	t.Run("truncated data - only block height", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{0x01}, func(d *postcard.Deserializer) (EventEntry, error) {
			var entry EventEntry
			if err := entry.UnmarshalPostcard(d); err != nil {
				return entry, err
			}
			return entry, nil
		}, false)
		assert.Error(t, err)
	})
}

func TestEventsByTxHash_MarshalPostcard(t *testing.T) {
	t.Run("round trip with multiple events", func(t *testing.T) {
		original := EventsByTxHash{
			Events: []EventEntry{
				{
					BlockHeight: 100,
					TxHash:      TxHash{1, 2, 3},
					TxIndex:     1,
					EventIndex:  10,
					Data: TypeTagWithData{
						TypeTag: 11,
						Value:   []byte{22},
					},
				},
				{
					BlockHeight: 200,
					TxHash:      TxHash{4, 5, 6},
					TxIndex:     2,
					EventIndex:  20,
					Data: TypeTagWithData{
						TypeTag: 33,
						Value:   []byte{44},
					},
				},
				{
					BlockHeight: 300,
					TxHash:      TxHash{7, 8, 9},
					TxIndex:     3,
					EventIndex:  30,
					Data: TypeTagWithData{
						TypeTag: 55,
						Value:   []byte{66},
					},
				},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventsByTxHash, error) {
			var r EventsByTxHash
			if err = r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, false)
		assert.NoError(t, err)
		for i := range original.Events {
			assert.Equal(t, original.Events[i].BlockHeight, deserialized.Events[i].BlockHeight)
			assert.Equal(t, original.Events[i].TxHash, deserialized.Events[i].TxHash)
			assert.Equal(t, original.Events[i].TxIndex, deserialized.Events[i].TxIndex)
			assert.Equal(t, original.Events[i].EventIndex, deserialized.Events[i].EventIndex)
			assert.Equal(t, original.Events[i].Data, deserialized.Events[i].Data)
		}
	})

	t.Run("round trip with empty events", func(t *testing.T) {
		original := EventsByTxHash{
			Events: []EventEntry{},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventsByTxHash, error) {
			var r EventsByTxHash
			if err = r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, false)
		assert.NoError(t, err)
		for i := range original.Events {
			assert.Equal(t, original.Events[i].BlockHeight, deserialized.Events[i].BlockHeight)
			assert.Equal(t, original.Events[i].TxHash, deserialized.Events[i].TxHash)
			assert.Equal(t, original.Events[i].TxIndex, deserialized.Events[i].TxIndex)
			assert.Equal(t, original.Events[i].EventIndex, deserialized.Events[i].EventIndex)
			assert.Equal(t, original.Events[i].Data, deserialized.Events[i].Data)
		}
	})

	t.Run("round trip with single event", func(t *testing.T) {
		original := EventsByTxHash{
			Events: []EventEntry{
				{
					BlockHeight: 999,
					TxHash:      TxHash{0xFF, 0xEE, 0xDD},
					TxIndex:     7,
					EventIndex:  8,
					Data: TypeTagWithData{
						TypeTag: 11,
						Value:   []byte{22},
					},
				},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		deserialized, err := postcard.DeserializePostcard(data, func(d *postcard.Deserializer) (EventsByTxHash, error) {
			var r EventsByTxHash
			if err = r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, false)
		assert.NoError(t, err)
		for i := range original.Events {
			assert.Equal(t, original.Events[i].BlockHeight, deserialized.Events[i].BlockHeight)
			assert.Equal(t, original.Events[i].TxHash, deserialized.Events[i].TxHash)
			assert.Equal(t, original.Events[i].TxIndex, deserialized.Events[i].TxIndex)
			assert.Equal(t, original.Events[i].EventIndex, deserialized.Events[i].EventIndex)
			assert.Equal(t, original.Events[i].Data, deserialized.Events[i].Data)
		}
	})

	t.Run("serialized data is deterministic", func(t *testing.T) {
		r1 := EventsByTxHash{
			Events: []EventEntry{
				{BlockHeight: 1, TxHash: TxHash{0xAA}, TxIndex: 1, EventIndex: 1, Data: TypeTagWithData{TypeTag: 11, Value: []byte{22}}},
				{BlockHeight: 2, TxHash: TxHash{0xBB}, TxIndex: 2, EventIndex: 2, Data: TypeTagWithData{TypeTag: 11, Value: []byte{22}}},
			},
		}
		r2 := EventsByTxHash{
			Events: []EventEntry{
				{BlockHeight: 1, TxHash: TxHash{0xAA}, TxIndex: 1, EventIndex: 1, Data: TypeTagWithData{TypeTag: 11, Value: []byte{22}}},
				{BlockHeight: 2, TxHash: TxHash{0xBB}, TxIndex: 2, EventIndex: 2, Data: TypeTagWithData{TypeTag: 11, Value: []byte{22}}},
			},
		}

		data1, err := postcard.SerializePostcard(&r1)
		assert.NoError(t, err)

		data2, err := postcard.SerializePostcard(&r2)
		assert.NoError(t, err)

		assert.Equal(t, data1, data2)
	})
}

func TestEventsByTxHashResponse_DeserializeErrors(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		_, err := postcard.DeserializePostcard([]byte{}, func(d *postcard.Deserializer) (EventsByTxHash, error) {
			var r EventsByTxHash
			if err := r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes not allowed", func(t *testing.T) {
		original := EventsByTxHash{
			Events: []EventEntry{
				{BlockHeight: 1, TxHash: TxHash{1}, TxIndex: 1, EventIndex: 1, Data: TypeTagWithData{TypeTag: 11, Value: []byte{22}}},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (EventsByTxHash, error) {
			var r EventsByTxHash
			if err = r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, false)
		assert.Error(t, err)
	})

	t.Run("trailing bytes allowed", func(t *testing.T) {
		original := EventsByTxHash{
			Events: []EventEntry{
				{BlockHeight: 1, TxHash: TxHash{1}, TxIndex: 1, EventIndex: 1, Data: TypeTagWithData{TypeTag: 11, Value: []byte{22}}},
			},
		}

		data, err := postcard.SerializePostcard(&original)
		assert.NoError(t, err)

		dataWithTrailing := append(data, 0xFF, 0xFF)

		_, err = postcard.DeserializePostcard(dataWithTrailing, func(d *postcard.Deserializer) (EventsByTxHash, error) {
			var r EventsByTxHash
			if err = r.UnmarshalPostcard(d); err != nil {
				return r, err
			}
			return r, nil
		}, true)
		assert.NoError(t, err)
	})
}
