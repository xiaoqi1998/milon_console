package types

import (
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBitmap64SetAndClear(t *testing.T) {
	var b Bitmap64
	assert.True(t, b.IsEmpty())
	assert.Equal(t, b.CountOnes(), 0)
	assert.False(t, b.Test(3))

	b = b.Set(3)
	assert.False(t, b.IsEmpty())
	assert.Equal(t, b.CountOnes(), 1)
	assert.True(t, b.Test(3))

	b = b.Clear(3)
	assert.True(t, b.IsEmpty())
	assert.Equal(t, b.CountOnes(), 0)
	assert.False(t, b.Test(3))
}

func TestBitmap64CountOnes(t *testing.T) {
	tests := []struct {
		name     string
		bits     uint64
		expected int
	}{
		{"zero", 0, 0},
		{"one", 1, 1},
		{"binary_1010", 0b1010, 2},
		{"binary_11111111", 0b11111111, 8},
		{"all_ones", ^uint64(0), 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBitmap64(tt.bits)
			assert.Equal(t, tt.expected, b.CountOnes())
		})
	}
}

func TestBitmap64LowestVacantIndex(t *testing.T) {
	tests := []struct {
		name        string
		bits        uint64
		expected    uint32
		expectedOk  bool
	}{
		{"empty", 0, 0, true},
		{"only_bit0_set", 1, 1, true},
		{"bits_0-2_set", 0b111, 3, true},
		{"bits_0-3_set", 0b1111, 4, true},
		{"alternating", 0b10101, 1, true},
		{"all_set", ^uint64(0), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBitmap64(tt.bits)
			idx, ok := b.LowestVacantIndex()
			assert.Equal(t, tt.expectedOk, ok)
			assert.Equal(t, tt.expected, idx)
		})
	}
}

func TestBitmap64IsSubsetOf(t *testing.T) {
	occupied := NewBitmap64(0b1111)

	assert.True(t, NewBitmap64(0b1).IsSubsetOf(occupied))

	assert.True(t, NewBitmap64(0b01).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b10).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b11).IsSubsetOf(occupied))

	assert.True(t, NewBitmap64(0b001).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b010).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b100).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b011).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b101).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b110).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b111).IsSubsetOf(occupied))

	assert.True(t, NewBitmap64(0b1000).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b0100).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b0010).IsSubsetOf(occupied))
	assert.True(t, NewBitmap64(0b0001).IsSubsetOf(occupied))

	assert.False(t, NewBitmap64(0b10000).IsSubsetOf(occupied))
}

func TestBitmap64IterSetBits(t *testing.T) {
	b := NewBitmap64(0b1010)
	assert.Equal(t, b.IterSetBits(), []uint8{1, 3})
}
func TestBitmap64StringAndGoStringAndFormat(t *testing.T) {
	tests := []struct {
		name             string
		bits             uint64
		expectedStr      string
		expectedGoString string
		expectedFormat   string
	}{
		{
			name:             "zero",
			bits:             0b0,
			expectedStr:      "0",
			expectedGoString: "Bitmap64(0b0000000000000000000000000000000000000000000000000000000000000000)",
			expectedFormat:   "0000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name:             "bit_0_set",
			bits:             0b01,
			expectedStr:      "1",
			expectedGoString: "Bitmap64(0b0000000000000000000000000000000000000000000000000000000000000001)",
			expectedFormat:   "0000000000000000000000000000000000000000000000000000000000000001",
		},
		{
			name:             "multiple_bits",
			bits:             0b1010,
			expectedStr:      "10",
			expectedGoString: "Bitmap64(0b0000000000000000000000000000000000000000000000000000000000001010)",
			expectedFormat:   "0000000000000000000000000000000000000000000000000000000000001010",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBitmap64(tt.bits)
			assert.Equal(t, tt.expectedStr, b.String())
			assert.Equal(t, tt.expectedGoString, b.GoString())
			assert.Equal(t, tt.expectedFormat, b.Format())
		})
	}
}

func TestBitmap64MarshalPostcard(t *testing.T) {
	tests := []struct {
		name string
		bits uint64
	}{
		{"zero", 0},
		{"one", 1},
		{"small_value", 42},
		{"medium_value", 0xFF},
		{"large_value", 0xFFFF},
		{"very_large_value", 0xFFFFFFFF},
		{"max_value", ^uint64(0)},
		{"alternating_bits", 0xAAAAAAAAAAAAAAAA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := NewBitmap64(tt.bits)

			serializer := postcard.NewSerializer()
			err := original.MarshalPostcard(serializer)
			assert.NoError(t, err)

			deserializer := postcard.NewDeserializer(serializer.Bytes())
			var back Bitmap64
			err = back.UnmarshalPostcard(deserializer)
			assert.NoError(t, err)
			assert.Equal(t, original, back)
			assert.Equal(t, tt.bits, back.Raw())

			err = deserializer.AssertEnd()
			assert.NoError(t, err)
		})
	}
}

func TestLowBitsMask(t *testing.T) {
	tests := []struct {
		name     string
		n        uint8
		expected uint64
	}{
		{"n=0", 0, 0},
		{"n=1", 1, 1},
		{"n=2", 2, 3},
		{"n=3", 3, 7},
		{"n=8", 8, 255},
		{"n=16", 16, 65535},
		{"n=32", 32, 4294967295},
		{"n=64", 64, ^uint64(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LowBitsMask(tt.n)
			assert.Equal(t, tt.expected, result.Raw())
		})
	}
}
