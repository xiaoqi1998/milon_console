package provider

import (
	"bytes"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"math"
	"math/big"
	"testing"
)

func TestProviderDemoEncodeAndDecode(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/demo.idl.json")
	assert.NoError(t, err)

	t.Run("enum unit variant", func(t *testing.T) {
		wire, err := pd.Encode("SpecialTypes", Args{
			"mode":       "one",
			"maybe_note": "note",
			"tags":       []string{},
			"labels":     map[string]uint32{},
			"pair":       []any{1, 2},
		})
		assert.NoError(t, err)

		decoded, err := pd.Decode("SpecialTypes", wire)
		assert.NoError(t, err)

		mode := decoded["mode"].(map[string]any)
		assert.Equal(t, "One", mode["variant"])   // unit: variant name only
		assert.Equal(t, uint64(0), mode["index"]) // first variant -> index 0
		assert.NotContains(t, mode, "fields")     // unit carries no data

		assert.Equal(t, "note", decoded["maybe_note"])              // option<String> -> Some("note")
		assert.Equal(t, []any{}, decoded["tags"])                   // empty vec<String>
		assert.Equal(t, map[any]any{}, decoded["labels"])           // empty map<String,u32>
		assert.Equal(t, []any{uint8(1), int16(2)}, decoded["pair"]) // tuple<u8,i16>
	})

	t.Run("enum named variant", func(t *testing.T) {
		wire, err := pd.Encode("SpecialTypes", Args{
			"mode":       map[string]any{"two": map[string]any{"val": 5}},
			"maybe_note": "note",
			"tags":       []string{"x", "y"},
			"labels":     map[string]uint32{"alpha": 10},
			"pair":       []any{3, -4},
		})
		assert.NoError(t, err)

		decoded, err := pd.Decode("SpecialTypes", wire)
		assert.NoError(t, err)

		mode := decoded["mode"].(map[string]any)
		assert.Equal(t, "Two", mode["variant"])   // decode returns the IDL variant name
		assert.Equal(t, uint64(1), mode["index"]) // second variant -> index 1
		assert.Equal(t, uint32(5), mode["val"])   // struct field: u32 -> uint32
		assert.NotContains(t, mode, "fields")     // struct variant fields are inline

		assert.Equal(t, "note", decoded["maybe_note"])    // option<String> -> Some("note")
		assert.Equal(t, []any{"x", "y"}, decoded["tags"]) // vec<String>
		labels, ok := decoded["labels"].(map[any]any)
		assert.True(t, ok)
		assert.Len(t, labels, 1)                                     // map<String,u32> with 1 entry
		assert.Equal(t, uint32(10), labels["alpha"])                 // value u32 -> uint32
		assert.Equal(t, []any{uint8(3), int16(-4)}, decoded["pair"]) // tuple<u8,i16>
	})

	t.Run("enum tuple variant", func(t *testing.T) {
		wire, err := pd.Encode("SpecialTypes", Args{
			"mode":       map[string]any{"variant": "three", "fields": []any{1, -2}},
			"maybe_note": "note",
			"tags":       []string{},
			"labels":     map[string]uint32{},
			"pair":       []any{1, 2},
		})
		assert.NoError(t, err)

		decoded, err := pd.Decode("SpecialTypes", wire)
		assert.NoError(t, err)

		mode := decoded["mode"].(map[string]any)
		assert.Equal(t, "Three", mode["variant"])                   // decode returns the IDL variant name
		assert.Equal(t, uint64(2), mode["index"])                   // third variant -> index 2
		assert.Equal(t, []any{uint8(1), int16(-2)}, mode["fields"]) // tuple fields: u8 + i16

		assert.Equal(t, "note", decoded["maybe_note"])              // option<String> -> Some("note")
		assert.Equal(t, []any{}, decoded["tags"])                   // empty vec<String>
		assert.Equal(t, map[any]any{}, decoded["labels"])           // empty map<String,u32>
		assert.Equal(t, []any{uint8(1), int16(2)}, decoded["pair"]) // tuple<u8,i16>
	})
}

func TestProviderTokenEncodeAndDecode(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	tokenSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	tokenPk := tokenSk.Ed25519Public()
	token, err := crypto.NewAddressFromPublicKey(tokenPk)
	assert.NoError(t, err)

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(ownerPk)
	assert.NoError(t, err)

	originalArgs := Args{
		"token": token,
		"owner": owner,
		"metadata": map[string]any{
			"name":     "TestCoin",
			"symbol":   "TST",
			"decimals": 6,
			"icon":     "https://example.com/icon.png",
		},
	}

	createBuf, err := pd.Encode("Create", originalArgs)
	assert.NoError(t, err)

	decodedArgs, err := pd.Decode("Create", createBuf)
	assert.NoError(t, err)

	tokenAddr, ok := decodedArgs["token"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, token.Bytes, tokenAddr.Bytes)

	ownerAddr, ok := decodedArgs["owner"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, owner.Bytes, ownerAddr.Bytes)

	if decodedMetadata, ok := decodedArgs["metadata"].(map[string]any); ok {
		assert.Equal(t, originalArgs["metadata"].(map[string]any)["name"], decodedMetadata["name"])
		assert.Equal(t, originalArgs["metadata"].(map[string]any)["symbol"], decodedMetadata["symbol"])
		assert.Equal(t, originalArgs["metadata"].(map[string]any)["icon"], decodedMetadata["icon"])

		origMeta := originalArgs["metadata"].(map[string]any)
		origDecimals := origMeta["decimals"]
		switch v := decodedMetadata["decimals"].(type) {
		case uint8:
			assert.EqualValues(t, origDecimals, v)
		}
	} else {
		assert.Fail(t, "metadata is not a map")
	}
}

func TestDecode_Errors(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	token, err := crypto.NewAddressFromRelaxed("0202020202020202020202020202020202020202")
	assert.NoError(t, err)
	to, err := crypto.NewAddressFromRelaxed("0303030303030303030303030303030303030303")
	assert.NoError(t, err)
	valid, err := pd.Encode("Mint", Args{"token": token, "to": to, "amount": uint64(1000)})
	assert.NoError(t, err)

	// Wire format: [app_id(u8)] + [discriminator(u16 LE)] + [args...]
	// Each case triggers one validation gate in Decode, in order:
	// 1. minimum length, 2. app_id match, 3. discriminator match, 4. full consumption.
	cases := []struct {
		name string
		body []byte
		want string
	}{
		// 2 bytes < 3 (app_id + discriminator) -> rejected before any parsing
		{"too short", []byte{1, 2}, "need at least 3 bytes"},
		// app_id=9 vs token app_id=2 -> cross-app guard
		{"app_id mismatch", []byte{9, 0, 0}, "app_id mismatch"},
		// app_id=2 passes, discriminator 0xFFFF=65535 vs Mint 20481 -> method guard
		{"discriminator mismatch", []byte{2, 0xFF, 0xFF}, "discriminator mismatch"},
		// valid Mint wire + 1 extra byte (0xAA) -> leftover must not be ignored
		{"trailing bytes", append(valid, 0xAA), "trailing bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pd.Decode("Mint", tc.body)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestDecodeViewDatas(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	responseBody := []byte{1, 0, 2, 188, 5} //700
	/*
		[1, 0, 2, 188, 5]
		 ↑  ↑  ↑  -------↑
		 │  │  │     └──── Ok internal Vec<u8> data (varint encoding of u64(700))
		 │  │  └─────────── Vec<u8> length = 2 bytes
		 │  └────────────── Result variant index = 0 (Ok/Success)
		 └───────────────── Vec length = 1 result
	*/

	results, err := pd.DecodeViewDatas("BalanceOf", responseBody)
	assert.NoError(t, err)
	if !assert.Len(t, results, 1) {
		return
	}
	assert.Equal(t, uint64(700), results[0].Value)

	result, err := pd.DecodeViewData("BalanceOf", responseBody)
	assert.NoError(t, err)
	assert.Equal(t, uint64(700), result)
}

func TestDecodeViewDatas_ResultBranches(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	t.Run("Ok trailing bytes", func(t *testing.T) {
		// Vec len=1, Result variant=0 (Ok), Ok payload len=2, but the u64 return
		// value only consumes 1 byte (varint 0x01) -> 1 trailing byte inside the
		// Ok payload must be rejected.
		responseBody := []byte{1, 0, 2, 1, 99}

		_, err := pd.DecodeViewDatas("BalanceOf", responseBody)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "trailing bytes")
	})

	t.Run("Err branch", func(t *testing.T) {
		body := []byte{
			1,           // vec len=1
			1,           // variant=1 (Err)
			5,           // code=5(u16)
			2, 'o', 'k', // msg="ok"(len=2)
			2, 0xAA, 0xBB, // data=[0xAA,0xBB](len=2)
		}

		results, err := pd.DecodeViewDatas("BalanceOf", body)
		assert.NoError(t, err)
		failure, ok := results[0].Value.(*api.TxFailurePayload)
		assert.True(t, ok)
		assert.Equal(t, uint16(5), failure.Code)
		assert.Equal(t, "ok", failure.Message)
		assert.Equal(t, []byte{0xAA, 0xBB}, failure.Data)
	})

	t.Run("invalid variant", func(t *testing.T) {
		// vec len=1, Result variant=2 (invalid)
		_, err := pd.DecodeViewDatas("BalanceOf", []byte{1, 2})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid result variant index")
	})
}

func TestProviderReportsErrors(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	token, err := crypto.NewAddressFromRelaxed("0202020202020202020202020202020202020202")
	assert.NoError(t, err)

	_, err = pd.Encode("Nope", Args{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IDL method not found")

	_, err = pd.Encode("Mint", Args{"token": token, "amount": 1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing IDL argument")
}

func TestSerializeValue_ComplexTypes(t *testing.T) {
	var idl IDL
	idl.Metadata.Name = "test"
	idl.Metadata.AppID = 255

	provider := NewProvider(idl)

	t.Run("empty container - vec<u8>", func(t *testing.T) {
		var value []any
		expected := []byte{0}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "vec<u8>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})

	t.Run("empty container - map<String,u64>", func(t *testing.T) {
		value := map[string]any{}
		expected := []byte{0}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "map<String,u64>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})

	t.Run("nested vec - vec<vec<u8>>", func(t *testing.T) {
		value := []any{
			[]any{uint8(1), uint8(2), uint8(3)},
			[]any{uint8(4), uint8(5)},
		}
		// [2, 3, 1, 2, 3, 2, 4, 5]
		// ↑  ↑-----------↑  ↑------↑
		// len  1st vec       2nd vec
		expected := []byte{2, 3, 1, 2, 3, 2, 4, 5}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "vec<vec<u8>>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})

	t.Run("nested option - option<option>", func(t *testing.T) {
		value := "hello"
		//	[1, 1, 5, 'h','e','l','l','o']
		//	↑  ↑  ↑  -----------↑
		//	outer inner len    string content
		expected := []byte{1, 1, 5, 'h', 'e', 'l', 'l', 'o'}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "option<option<String>>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})

	t.Run("outer None - option<option>", func(t *testing.T) {
		var value any = nil
		expected := []byte{0}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "option<option<String>>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})

	t.Run("Map type - map<String,u64>", func(t *testing.T) {
		value := map[string]any{
			"alice": uint64(100),
			"bob":   uint64(200),
		}
		// [2, 5,'a','l','i','c','e', 100, 3,'b','o','b', 200, 1]
		//	↑  -------------------------↑  ------------------------↑
		//	len         1st kv pair              2nd kv pair
		//	           key="alice"(5 bytes)      key="bob"(3 bytes)
		//	           value=100(varint)        value=200(varint, 2 bytes)
		// Note: map iteration order is non-deterministic; only validation is format
		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "map<String,u64>", value)
		assert.NoError(t, err)

		// map iteration order is non-deterministic; parse the output and
		// verify the key/value set instead of byte order.
		deserializer := postcard.NewDeserializer(serializer.Bytes())
		length, err := deserializer.DeserializeU32()
		assert.NoError(t, err)
		assert.Equal(t, uint32(2), length)

		got := make(map[string]uint64, length)
		for i := uint32(0); i < length; i++ {
			key, err := deserializer.DeserializeStr()
			assert.NoError(t, err)
			v, err := deserializer.DeserializeU64()
			assert.NoError(t, err)
			got[key] = v
		}
		assert.Equal(t, map[string]uint64{"alice": 100, "bob": 200}, got)
	})

	t.Run("Tuple type - tuple<u8,u16,u32>", func(t *testing.T) {
		value := []any{uint8(1), uint16(256), uint32(65536)}
		// [1, 128, 2, 128, 128, 4]
		// ↑  ------↑  -----------↑
		// u8    u16       u32
		expected := []byte{1, 128, 2, 128, 128, 4}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "tuple<u8,u16,u32>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})

	t.Run("nested Tuple - tuple<u8,tuple>", func(t *testing.T) {
		value := []any{
			uint8(1),
			[]any{uint16(2), uint32(3)},
		}
		expected := []byte{1, 2, 3}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "tuple<u8,tuple<u16,u32>>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})

	t.Run("Option + Vec - option<vec<String>>", func(t *testing.T) {
		value := []any{"apple", "banana", "cherry"}
		expected := []byte{
			1,                          // has_value = true (option is Some)
			3,                          // vec length = 3 elements
			5, 'a', 'p', 'p', 'l', 'e', // string len=5 + "apple"
			6, 'b', 'a', 'n', 'a', 'n', 'a', // string len=6 + "banana"
			6, 'c', 'h', 'e', 'r', 'r', 'y', // string len=6 + "cherry"
		}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "option<vec<String>>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})

	t.Run("Vec + Option - vec<option<u64>>", func(t *testing.T) {
		value := []any{uint64(10), nil, uint32(30)}
		expected := []byte{
			3,     // vec length = 3 elements
			1, 10, // 1st: Some, u64 value = 10 (varint)
			0,     // 2nd: None, no value follows
			1, 30, // 3rd: Some, u32 value = 30 (varint)
		}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "vec<option<u64>>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})
}

func TestSerializeValue_AddressAndPublicKey(t *testing.T) {
	var idl IDL
	idl.Metadata.Name = "test"
	idl.Metadata.AppID = 255
	provider := NewProvider(idl)

	t.Run("Address", func(t *testing.T) {
		addrBytes := make([]byte, 20)
		for i := 0; i < 20; i++ {
			addrBytes[i] = byte(i)
		}
		addr, err := crypto.NewAddressFromBytes(addrBytes)
		assert.NoError(t, err)

		serializer := postcard.NewSerializer()
		err = provider.serializeValue(serializer, "Address", addr)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, len(result), 20)
		for i := 0; i < 20; i++ {
			assert.Equal(t, result[i], byte(i))
		}
	})

	t.Run("PublicKey Ed25519", func(t *testing.T) {
		pkBytes := make([]byte, 32)
		for i := 0; i < 32; i++ {
			pkBytes[i] = byte(i)
		}
		pk, err := crypto.NewPublicKeyFromBytes(pkBytes)
		assert.NoError(t, err)
		assert.True(t, pk.IsEd25519())

		serializer := postcard.NewSerializer()
		err = provider.serializeValue(serializer, "PublicKey", pk)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, result[0], byte(crypto.PublicKeyTypeEd25519))
		assert.Equal(t, len(result), 1+crypto.PublicKeyEd25519Size)

		for i := 0; i < crypto.PublicKeyEd25519Size; i++ {
			assert.Equal(t, result[i+1], byte(i))
		}
	})

	t.Run("B256", func(t *testing.T) {
		hash := [32]byte{}
		for i := 0; i < 32; i++ {
			hash[i] = byte(i)
		}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "B256", hash)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, len(serializer.Bytes()), 32)
	})
}

func TestSerializeValue_Struct(t *testing.T) {
	var idl IDL
	idl.Metadata.Name = "token"
	idl.Metadata.AppID = 2

	// Define Metadata type
	idl.Types = []IDLType{
		{
			Name: "Metadata",
			Kind: "struct",
			Fields: []StructField{
				{Name: "name", Type: "String"},
				{Name: "symbol", Type: "String"},
				{Name: "decimals", Type: "u8"},
				{Name: "totalSupply", Type: "u64"},
			},
		},
	}

	provider := NewProvider(idl)

	metadata := map[string]any{
		"name":        "MyToken",
		"symbol":      "MTK",
		"decimals":    uint8(18),
		"totalSupply": uint64(1000000),
	}

	serializer := postcard.NewSerializer()
	err := provider.serializeValue(serializer, "Metadata", metadata)
	if err != nil {
		t.Fatal(err)
	}

	result := serializer.Bytes()
	expected := []byte{
		// Metadata struct serialization
		7, 'M', 'y', 'T', 'o', 'k', 'e', 'n', // name: String len=7 + "MyToken"
		3, 'M', 'T', 'K', // symbol: String len=3 + "MTK"
		18,           // decimals: u8(18)
		192, 132, 61, // totalSupply: u64(1000000) varint encoding
	}

	for i := range expected {
		assert.Equal(t, result[i], expected[i])
	}
}

func TestSerializeValue_DeepNesting(t *testing.T) {
	var idl IDL
	idl.Metadata.Name = "test"
	idl.Metadata.AppID = 255
	provider := NewProvider(idl)

	// vec<option<tuple<u8,vec<String>>>>
	deepNested := []any{
		[]any{uint8(1), []any{"a", "b"}}, // Some((1, ["a","b"]))
		nil,                              // None
		[]any{uint8(2), []any{"c"}},      // Some((2, ["c"]))
	}

	serializer := postcard.NewSerializer()
	err := provider.serializeValue(serializer, "vec<option<tuple<u8,vec<String>>>>", deepNested)
	if err != nil {
		t.Fatal(err)
	}

	result := serializer.Bytes()
	expected := []byte{
		3, // vec length = 3

		// 1st element: Some((1, ["a","b"]))
		1,      // Some (option has_value = true)
		1,      // tuple[0] = u8(1)
		2,      // tuple[1] = vec length = 2
		1, 'a', // "a" (string len=1 + content)
		1, 'b', // "b" (string len=1 + content)

		// 2nd element: None
		0, // None (option has_value = false)

		// 3rd element: Some((2, ["c"]))
		1,      // Some (option has_value = true)
		2,      // tuple[0] = u8(2)
		1,      // tuple[1] = vec length = 1
		1, 'c', // "c" (string len=1 + content)
	}

	for i := range expected {
		assert.Equal(t, result[i], expected[i])
	}
}

func TestDeserializeValue_Integer(t *testing.T) {
	var idl IDL
	idl.Metadata.Name = "test"
	idl.Metadata.AppID = 255
	provider := NewProvider(idl)

	roundTrip := func(t *testing.T, typeName string, value any) {
		serializer := postcard.NewSerializer()
		assert.NoError(t, provider.serializeValue(serializer, typeName, value))
		body := serializer.Bytes()

		offset := 0
		got, err := provider.deserializeValue(typeName, body, &offset)
		assert.NoError(t, err)
		assert.Equal(t, len(body), offset)
		assert.Equal(t, value, got)
	}

	t.Run("i8", func(t *testing.T) {
		roundTrip(t, "i8", int8(-128))
		roundTrip(t, "i8", int8(-1))
		roundTrip(t, "i8", int8(0))
		roundTrip(t, "i8", int8(127))
	})
	t.Run("i16", func(t *testing.T) {
		roundTrip(t, "i16", int16(-32768))
		roundTrip(t, "i16", int16(-1))
		roundTrip(t, "i16", int16(32767))
	})
	t.Run("i32", func(t *testing.T) {
		roundTrip(t, "i32", int32(-1))
		roundTrip(t, "i32", int32(math.MinInt32))
		roundTrip(t, "i32", int32(math.MaxInt32))
	})
	t.Run("i64", func(t *testing.T) {
		roundTrip(t, "i64", int64(-1))
		roundTrip(t, "i64", int64(math.MinInt64))
		roundTrip(t, "i64", int64(math.MaxInt64))
	})
}

func TestDeserializeValue_FixedBytes(t *testing.T) {
	var idl IDL
	idl.Metadata.Name = "test"
	idl.Metadata.AppID = 255
	provider := NewProvider(idl)

	t.Run("roundtrip", func(t *testing.T) {
		cases := []struct {
			typeName string
			value    any
		}{
			{"B96", [12]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
			{"B144", [18]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18}},
			{"B160", [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}},
			{"B256", [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}},
		}
		for _, tc := range cases {
			t.Run(tc.typeName, func(t *testing.T) {
				serializer := postcard.NewSerializer()
				assert.NoError(t, provider.serializeValue(serializer, tc.typeName, tc.value))
				body := serializer.Bytes()

				offset := 0
				got, err := provider.deserializeValue(tc.typeName, body, &offset)
				assert.NoError(t, err)
				assert.Equal(t, len(body), offset)
				assert.Equal(t, tc.value, got)
			})
		}
	})

	t.Run("insufficient data", func(t *testing.T) {
		cases := []struct {
			typeName string
			size     int
		}{
			{"B96", 12},
			{"B144", 18},
			{"B160", 20},
			{"B256", 32},
		}
		for _, tc := range cases {
			offset := 0
			// size-1 bytes: exactly one byte short
			_, err := provider.deserializeValue(tc.typeName, make([]byte, tc.size-1), &offset)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "insufficient data for "+tc.typeName)
		}
	})

	t.Run("B96 as []byte input", func(t *testing.T) {
		serializer := postcard.NewSerializer()
		assert.NoError(t, provider.serializeValue(serializer, "B96", make([]byte, 12)))

		offset := 0
		got, err := provider.deserializeValue("B96", serializer.Bytes(), &offset)
		assert.NoError(t, err)
		assert.Equal(t, [12]byte{}, got) // all-zero []byte roundtrips to zero array
	})

	t.Run("wrong []byte length", func(t *testing.T) {
		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "B96", make([]byte, 11))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expects exactly 12 bytes")
	})
}

func TestDeserializeValue_U128(t *testing.T) {
	var idl IDL
	idl.Metadata.Name = "test"
	idl.Metadata.AppID = 255
	provider := NewProvider(idl)

	maxU128 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	values := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(127),
		big.NewInt(128),
		new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1)),
		new(big.Int).Set(maxU128),
	}
	for _, v := range values {
		serializer := postcard.NewSerializer()
		assert.NoError(t, serializer.SerializeU128(v))
		body := serializer.Bytes()

		offset := 0
		got, err := provider.deserializeValue("u128", body, &offset)
		assert.NoError(t, err)
		assert.Equal(t, len(body), offset)

		gotBig, ok := got.(*big.Int)
		assert.True(t, ok)
		assert.Equal(t, v, gotBig)
	}
}

func TestSerializeEnumVariant(t *testing.T) {
	testCases := []struct {
		name     string
		value    uint32
		expected []byte
	}{
		{
			name:     "0 - minimum value",
			value:    0,
			expected: []byte{0x00},
		},
		{
			name:     "1 - small value",
			value:    1,
			expected: []byte{0x01},
		},
		{
			name:     "127 - varint single byte maximum",
			value:    127,
			expected: []byte{0x7F},
		},
		{
			name:     "128 - varint 2 bytes minimum",
			value:    128,
			expected: []byte{0x80, 0x01},
		},
		{
			name:     "16383 - varint 2 bytes maximum",
			value:    16383,
			expected: []byte{0xFF, 0x7F},
		},
		{
			name:     "16384 - varint 3 bytes minimum",
			value:    16384,
			expected: []byte{0x80, 0x80, 0x01},
		},
		{
			name:     "2097151 - varint 3 bytes maximum",
			value:    2097151,
			expected: []byte{0xFF, 0xFF, 0x7F},
		},
		{
			name:     "2097152 - varint 4 bytes minimum",
			value:    2097152,
			expected: []byte{0x80, 0x80, 0x80, 0x01},
		},
		{
			name:     "uint32 max value (4294967295)",
			value:    4294967295,
			expected: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x0F},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serializer := postcard.NewSerializer()
			err := serializer.SerializeEnumVariant(tc.value)
			assert.NoError(t, err)

			result := serializer.Bytes()
			assert.Equal(t, tc.expected, result)

			variant, err := postcard.NewDeserializer(result).DeserializeU32()
			assert.NoError(t, err)

			assert.Equal(t, tc.value, variant)
		})
	}
}

func TestDecodeDataByIDLTypeName(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	// token::Metadata struct: name(String) + symbol(String) + decimals(u8) + icon(String)
	serializer := postcard.NewSerializer()
	assert.NoError(t, serializer.SerializeStr("TestCoin"))
	assert.NoError(t, serializer.SerializeStr("TST"))
	assert.NoError(t, serializer.SerializeU8(6))
	assert.NoError(t, serializer.SerializeStr("https://example.com/icon.png"))
	data := serializer.Bytes()

	value, err := pd.DecodeDataByIDLTypeName("Metadata", data)
	assert.NoError(t, err)
	record := value.(map[string]any)
	assert.Equal(t, "TestCoin", record["name"])
	assert.Equal(t, "TST", record["symbol"])
	assert.Equal(t, uint8(6), record["decimals"])
	assert.Equal(t, "https://example.com/icon.png", record["icon"])

	// trailing bytes must error
	_, err = pd.DecodeDataByIDLTypeName("Metadata", append(append([]byte{}, data...), 0xAA))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trailing bytes")

	// unknown type name must error
	_, err = pd.DecodeDataByIDLTypeName("NoSuchType", data)
	assert.Error(t, err)
}

func TestDecodeViewVarint_Errors(t *testing.T) {
	// Varint (LEB128): each byte carries 7 data bits + 1 continuation flag.
	// 0x80 = all data bits zero with continuation set (expects a next byte),
	// so it is the canonical byte for probing overflow / truncation paths.

	// uint64 varint: at most 10 bytes (10*7 = 70 >= 64 bits).
	// 10 continuation bytes -> the 10-byte limit is hit with no terminator -> too long.
	offset := 0
	_, err := decodeViewVarUint(bytes.Repeat([]byte{0x80}, 10), &offset)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "varint is too long")

	// Only 8 continuation bytes -> the 9th read runs past the input -> truncated.
	offset = 0
	_, err = decodeViewVarUint(bytes.Repeat([]byte{0x80}, 8), &offset)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected end of input")

	// u128 varint: at most 19 bytes (19*7 = 133 >= 128 bits; 18*7 = 126 < 128).
	// 19 continuation bytes -> limit hit with no terminator -> too long.
	offset = 0
	_, err = decodeViewVarUint128(bytes.Repeat([]byte{0x80}, 19), &offset)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "varint is too long")

	// u128: only 8 continuation bytes -> the 9th read runs past the input -> truncated.
	offset = 0
	_, err = decodeViewVarUint128(bytes.Repeat([]byte{0x80}, 8), &offset)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected end of input")
}
