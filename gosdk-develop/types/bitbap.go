package types

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"math/bits"
)

// Bitmap64 is a 64-slot bitmap: bit i (from LSB) indicates whether index i is occupied
type Bitmap64 uint64

// Bits is the number of valid bit slots
const Bits = 64

// NewBitmap64 constructs a Bitmap64 from a raw uint64
func NewBitmap64(raw uint64) Bitmap64 {
	return Bitmap64(raw)
}

// Raw returns the underlying uint64
func (b Bitmap64) Raw() uint64 {
	return uint64(b)
}

func (b Bitmap64) IsEmpty() bool {
	return b == 0
}

// Test checks whether the bit at the given position is 1
func (b Bitmap64) Test(bit uint8) bool {
	if bit >= Bits {
		return false
	}
	return (uint64(b)>>bit)&1 != 0
}

func (b Bitmap64) Set(bit uint8) Bitmap64 {
	if bit >= Bits {
		return b
	}
	return Bitmap64(uint64(b) | (1 << bit))
}

func (b Bitmap64) Clear(bit uint8) Bitmap64 {
	if bit >= Bits {
		return b
	}
	return Bitmap64(uint64(b) & ^(1 << bit))
}

// CountOnes returns the number of set bits
func (b Bitmap64) CountOnes() int {
	return bits.OnesCount64(uint64(b))
}

// LowestVacantIndex returns the lowest unset (vacant) index.
// When all 64 bits are occupied, it returns (0, false).
func (b Bitmap64) LowestVacantIndex() (uint32, bool) {
	inverted := ^uint64(b)
	if inverted == 0 {
		return 0, false // 无空位
	}
	return uint32(bits.TrailingZeros64(inverted)), true
}

// IsSubsetOf checks whether self is a subset of other
func (b Bitmap64) IsSubsetOf(other Bitmap64) bool {
	return (uint64(b) & uint64(other)) == uint64(b)
}

// IterSetBits returns a slice of indices where the bit is set
func (b Bitmap64) IterSetBits() []uint8 {
	value := uint64(b)
	result := make([]uint8, 0, bits.OnesCount64(value))
	for value != 0 {
		idx := bits.TrailingZeros64(value)
		result = append(result, uint8(idx))
		value &= value - 1 // clear the lowest set bit
	}
	return result
}

// String implements the fmt.Stringer interface
func (b Bitmap64) String() string {
	return fmt.Sprintf("%d", uint64(b))
}

// GoString implements the fmt.GoStringer interface
func (b Bitmap64) GoString() string {
	return fmt.Sprintf("Bitmap64(0b%064b)", uint64(b))
}

func (b Bitmap64) Format() string {
	return fmt.Sprintf("%064b", uint64(b))
}

func (b *Bitmap64) MarshalPostcard(serializer *postcard.Serializer) error {
	return serializer.SerializeU64(uint64(*b))
}

func (b *Bitmap64) UnmarshalPostcard(deserializer *postcard.Deserializer) error {
	data, err := deserializer.DeserializeU64()
	if err != nil {
		return fmt.Errorf("failed to deserialize Bitmap64: %w", err)
	}

	*b = Bitmap64(data)

	return nil
}

// LowBitsMask returns a mask with the lowest n bits set to 1: (1 << n) - 1
func LowBitsMask(n uint8) Bitmap64 {
	if n >= Bits {
		return Bitmap64(^uint64(0))
	}
	return Bitmap64((1 << n) - 1)
}
