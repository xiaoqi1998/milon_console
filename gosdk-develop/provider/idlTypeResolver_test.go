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

	// token::_Metadata resource: type Metadata (name(String) + symbol(String)
	// + decimals(u8) + icon(String) + uri(String)); the resource typeTag is
	// independent from the Metadata struct typeTag.
	resource, ok := pd.GetResourceByTypeTag(13005725662941815531)
	assert.True(t, ok)
	assert.Equal(t, "_Metadata", resource.Name)
	assert.Equal(t, "Metadata", resource.Type)

	serializer := postcard.NewSerializer()
	assert.NoError(t, serializer.SerializeStr("TestCoin"))
	assert.NoError(t, serializer.SerializeStr("TST"))
	assert.NoError(t, serializer.SerializeU8(6))
	assert.NoError(t, serializer.SerializeStr("https://example.com/icon.png"))
	assert.NoError(t, serializer.SerializeStr("https://example.com/token.json"))
	payload := serializer.Bytes()

	// extra bytes after the resource value
	full := append(append([]byte{}, payload...), 0xAA, 0xBB)

	valueBytes, remaining, err := resolver.DecodeResource(resource.TypeTag, full)
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

func TestIDLTypeResolver_DecodeResource_BuiltinTypeFallback(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/system.idl.json")
	assert.NoError(t, err)

	resolver := &IDLTypeResolver{Providers: map[string]*Provider{"system": pd}}

	// The genesis transaction persists a bare Address value under the Address
	// builtin tag, which is declared in the types section but not as a
	// resource; decoding must fall back to the types index.
	addressBytes := make([]byte, 20)
	for i := range addressBytes {
		addressBytes[i] = byte(i + 1)
	}

	// extra bytes after the value must stay unconsumed
	full := append(append([]byte{}, addressBytes...), 0xAA)

	valueBytes, remaining, err := resolver.DecodeResource(17438174819379414968, full)
	assert.NoError(t, err)
	assert.Equal(t, addressBytes, valueBytes)
	assert.Equal(t, []byte{0xAA}, remaining)
}

func TestIDLTypeResolver_ResourceWinsOverType(t *testing.T) {
	// the same typeTag is declared both as a resource and as a raw type; the
	// resource declaration must win. u8 consumes 1 byte while Address needs
	// 20, so the test can tell which branch ran.
	pd := NewProvider(IDL{
		Metadata:  Metadata{AppID: 1, Name: "a"},
		Types:     []IDLType{{Kind: "builtin", Name: "Address", TypeTag: 999}},
		Resources: []Resource{{Name: "_Shared", Type: "u8", TypeTag: 999}},
	})

	resolver := &IDLTypeResolver{Providers: map[string]*Provider{"a": pd}}

	valueBytes, remaining, err := resolver.DecodeResource(999, []byte{7, 8})
	assert.NoError(t, err)
	assert.Equal(t, []byte{7}, valueBytes)
	assert.Equal(t, []byte{8}, remaining)
}

func TestIDLTypeResolver_MultipleProviders(t *testing.T) {
	tokenPd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)
	demoPd, err := LoadProviderFromFile("./IDL/demo.idl.json")
	assert.NoError(t, err)

	resolver := &IDLTypeResolver{Providers: map[string]*Provider{"token": tokenPd, "demo": demoPd}}

	// resource from token (_Metadata tag)
	serializer := postcard.NewSerializer()
	assert.NoError(t, serializer.SerializeStr("A"))
	assert.NoError(t, serializer.SerializeStr("B"))
	assert.NoError(t, serializer.SerializeU8(2))
	assert.NoError(t, serializer.SerializeStr("C"))
	assert.NoError(t, serializer.SerializeStr("D"))
	resourceBytes := serializer.Bytes()

	valueBytes, remaining, err := resolver.DecodeResource(13005725662941815531, resourceBytes)
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
	// both providers declare resource typeTag 999; "a" sorts first and wins.
	// The types differ on purpose so the test can tell which provider was
	// selected: u8 consumes 1 byte while B256 would need 32.
	pdA := NewProvider(IDL{
		Metadata:  Metadata{AppID: 1, Name: "a"},
		Resources: []Resource{{Name: "_Shared", Type: "u8", TypeTag: 999}},
	})
	pdB := NewProvider(IDL{
		Metadata:  Metadata{AppID: 2, Name: "b"},
		Resources: []Resource{{Name: "_Shared", Type: "B256", TypeTag: 999}},
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
