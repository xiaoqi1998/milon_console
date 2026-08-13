package provider

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
)

func TestIDLTypeResolver_DecodeResource(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	resolver := &IDLTypeResolver{Providers: map[string]*Provider{"token": pd}}

	// token::Metadata struct: name(String) + symbol(String) + decimals(u8) + icon(String)
	metadata, ok := pd.IDLTypeByName["Metadata"]
	assert.True(t, ok)

	serializer := postcard.NewSerializer()
	assert.NoError(t, serializer.SerializeStr("TestCoin"))
	assert.NoError(t, serializer.SerializeStr("TST"))
	assert.NoError(t, serializer.SerializeU8(6))
	assert.NoError(t, serializer.SerializeStr("https://example.com/icon.png"))
	payload := serializer.Bytes()

	// extra bytes after the resource value
	full := append(append([]byte{}, payload...), 0xAA, 0xBB)

	valueBytes, remaining, err := resolver.DecodeResource(metadata.TypeTag, full)
	assert.NoError(t, err)
	assert.Equal(t, payload, valueBytes)
	assert.Equal(t, []byte{0xAA, 0xBB}, remaining)
}

func TestIDLTypeResolver_DecodeResource_Errors(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	resolver := &IDLTypeResolver{Providers: map[string]*Provider{"token": pd}}

	_, _, err = resolver.DecodeResource(424242424242, []byte{1, 2, 3})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resource type_tag")
}

func TestIDLTypeResolver_MultipleProviders(t *testing.T) {
	tokenPd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)
	demoPd, err := LoadProviderFromFile("./IDL/demo.idl.json")
	assert.NoError(t, err)

	resolver := &IDLTypeResolver{Providers: map[string]*Provider{"token": tokenPd, "demo": demoPd}}

	// resource from token (Metadata typeTag)
	metadata, _ := tokenPd.IDLTypeByName["Metadata"]
	serializer := postcard.NewSerializer()
	assert.NoError(t, serializer.SerializeStr("A"))
	assert.NoError(t, serializer.SerializeStr("B"))
	assert.NoError(t, serializer.SerializeU8(2))
	assert.NoError(t, serializer.SerializeStr("C"))
	resourceBytes := serializer.Bytes()

	valueBytes, remaining, err := resolver.DecodeResource(metadata.TypeTag, resourceBytes)
	assert.NoError(t, err)
	assert.Equal(t, resourceBytes, valueBytes)
	assert.Len(t, remaining, 0)

	// event from demo: pool(20) + recipient(20) + amount varint(42 -> 1 byte)
	eventData := append(append([]byte{}, make([]byte, 40)...), 42)
	eventBytes, remaining, err := resolver.DecodeEvent(7407037194950745602, eventData)
	assert.NoError(t, err)
	assert.Len(t, eventBytes, 41)
	assert.Len(t, remaining, 0)
}

func TestIDLTypeResolver_CollisionFirstWins(t *testing.T) {
	// both providers register typeTag 999; "a" sorts first and wins
	pdA := NewProvider(IDL{
		Metadata: Metadata{AppID: 1, Name: "a"},
		Types: []IDLType{
			{Name: "Shared", TypeTag: 999, Kind: "struct", Fields: []StructField{{Name: "x", Type: "u8"}}},
		},
	})
	pdB := NewProvider(IDL{
		Metadata: Metadata{AppID: 2, Name: "b"},
		Types: []IDLType{
			{Name: "Shared", TypeTag: 999, Kind: "struct", Fields: []StructField{{Name: "y", Type: "u8"}, {Name: "z", Type: "u8"}}},
		},
	})

	resolver := &IDLTypeResolver{Providers: map[string]*Provider{"a": pdA, "b": pdB}}

	valueBytes, remaining, err := resolver.DecodeResource(999, []byte{7})
	assert.NoError(t, err)
	assert.Equal(t, []byte{7}, valueBytes)
	assert.Len(t, remaining, 0)
}

func TestIDLTypeResolver_DecodeEvent(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/demo.idl.json")
	assert.NoError(t, err)

	resolver := &IDLTypeResolver{Providers: map[string]*Provider{"demo": pd}}

	// demo EventCreditApplied: pool(Address) + recipient(Address) + amount(u64)
	serializer := postcard.NewSerializer()
	serializer.SerializeFixedBytes(make([]byte, 20)) // no error return
	serializer.SerializeFixedBytes(make([]byte, 20))
	assert.NoError(t, serializer.SerializeU64(7))
	payload := serializer.Bytes()

	eventBytes, remaining, err := resolver.DecodeEvent(7407037194950745602, append(append([]byte{}, payload...), 0xEE))
	assert.NoError(t, err)
	assert.Equal(t, payload, eventBytes)
	assert.Equal(t, []byte{0xEE}, remaining)
}

func TestIDLTypeResolver_DecodeEvent_Errors(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/demo.idl.json")
	assert.NoError(t, err)

	resolver := &IDLTypeResolver{Providers: map[string]*Provider{"demo": pd}}

	_, _, err = resolver.DecodeEvent(424242424242, []byte{1, 2, 3})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown event type_tag")
}
