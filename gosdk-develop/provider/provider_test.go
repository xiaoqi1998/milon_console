package provider

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProviderDemoEncode(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/demo.idl.json")
	assert.NoError(t, err)

	_, err = pd.Encode("SpecialTypes", Args{
		"mode":       map[string]any{"two": map[string]any{"val": 5}},
		"maybe_note": "note",
		"tags":       []string{"x", "y"},
		"labels":     map[string]uint32{"alpha": 10, "beta": 20},
		"pair":       []any{3, -4},
	})
	assert.NoError(t, err)
}

func TestProviderTokenCreateEncodeAndDecode(t *testing.T) {
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

	fmt.Printf("Deserialization results:\n")
	fmt.Printf("  token: %v (type: %T)\n", decodedArgs["token"], decodedArgs["token"])
	fmt.Printf("  owner: %v (type: %T)\n", decodedArgs["owner"], decodedArgs["owner"])

	tokenAddr, ok := decodedArgs["token"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, token.Bytes, tokenAddr.Bytes)

	ownerAddr, ok := decodedArgs["owner"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, owner.Bytes, ownerAddr.Bytes)

	if decodedMetadata, ok := decodedArgs["metadata"].(map[string]any); ok {
		fmt.Printf("  decodedMetadata.name: %#v\n", decodedMetadata["name"])
		fmt.Printf("  decodedMetadata.symbol: %#v\n", decodedMetadata["symbol"])
		fmt.Printf("  decodedMetadata.decimals: %#v (type: %T)\n", decodedMetadata["decimals"], decodedMetadata["decimals"])
		fmt.Printf("  decodedMetadata.icon: %#v\n", decodedMetadata["icon"])

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

func TestProviderTokenMintEncodeAndDecode(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	ownerSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	ownerPk := ownerSk.Ed25519Public()
	owner, err := crypto.NewAddressFromPublicKey(ownerPk)
	assert.NoError(t, err)

	toSk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	toPk := toSk.Ed25519Public()
	to, err := crypto.NewAddressFromPublicKey(toPk)
	assert.NoError(t, err)

	originalArgs := Args{
		"token":  owner,
		"to":     to,
		"amount": uint64(1000),
	}

	mint, err := pd.Encode("Mint", originalArgs)
	assert.NoError(t, err)

	decodedArgs, err := pd.Decode("Mint", mint)
	assert.NoError(t, err)

	fmt.Printf("Deserialization results:\n")
	fmt.Printf("  token: %v (type: %T)\n", decodedArgs["token"], decodedArgs["token"])
	fmt.Printf("  to: %v (type: %T)\n", decodedArgs["to"], decodedArgs["to"])
	fmt.Printf("  amount: %v (type: %T)\n", decodedArgs["amount"], decodedArgs["amount"])

	tokenAddr, ok := decodedArgs["token"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, owner.Bytes, tokenAddr.Bytes)

	toAddr, ok := decodedArgs["to"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, to.Bytes, toAddr.Bytes)

	assert.Equal(t, originalArgs["amount"], decodedArgs["amount"])
}

func TestProviderDecodeViewValues(t *testing.T) {
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
	for i, result := range results {
		if failure, ok := result.Value.(*api.TxFailurePayload); ok {
			// Handle failure case
			fmt.Printf("Instruction %d failed:\n", i)
			fmt.Printf("  Error code: %d\n", failure.Code)
			fmt.Printf("  Error message: %s\n", failure.Message)
			fmt.Printf("  Additional data: %v\n", failure.Data)
		} else {
			// Handle success case
			switch v := result.Value.(type) {
			case uint64:
				fmt.Printf("Instruction %d succeeded: return value = %d\n", i, v)
			case crypto.Address:
				fmt.Printf("Instruction %d succeeded: address = %s\n", i, v.ToBase58())
			case string:
				fmt.Printf("Instruction %d succeeded: string = %s\n", i, v)
			case map[string]any:
				fmt.Printf("Instruction %d succeeded: struct = %+v\n", i, v)
			default:
				fmt.Printf("Instruction %d succeeded: value = %v (type: %T)\n", i, v, v)
			}
		}
	}

	result, err := pd.DecodeViewData("BalanceOf", responseBody)
	assert.NoError(t, err)
	// Check if returns error
	if failure, ok := result.(*api.TxFailurePayload); ok {
		// Handle failure case
		fmt.Printf("Query failed:\n")
		fmt.Printf("  Error code: %d\n", failure.Code)
		fmt.Printf("  Error message: %s\n", failure.Message)
		fmt.Printf("  Additional data: %v\n", failure.Data)
		t.Fatalf("Expected success but got error: %s", failure.Message)
	} else {
		// Handle success case
		balance, ok := result.(uint64)
		if !ok {
			t.Fatalf("Expected uint64 but got %T", result)
		}

		fmt.Printf("Query succeeded: balance = %d\n", balance)

		// Verify return value is 700
		assert.Equal(t, uint64(700), balance)
	}
}

func TestProviderReportsErrors(t *testing.T) {
	pd, err := LoadProviderFromFile("./IDL/token.idl.json")
	assert.NoError(t, err)

	token, err := crypto.NewAddressFromStringRelaxed("0202020202020202020202020202020202020202")
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
		value := []any{}
		// [0]
		expected := []byte{0}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "vec<u8>", value)
		assert.NoError(t, err)

		result := serializer.Bytes()
		assert.Equal(t, expected, result)
	})

	t.Run("empty container - map<String,u64>", func(t *testing.T) {
		value := map[string]any{}
		// [0]
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
		// [0]
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

		result := serializer.Bytes()
		t.Logf("result: %v", result)
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
		// [1, 2, 3]
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
		// [3, 1, 10, 0, 1, 30]
		expected := []byte{
			3,     //length = 3 elements
			1, 10, //has_value = true (1st element: Some) 		u64 value = 10 (varint)
			0,     //has_value = false (2nd element: None, no value follows)
			1, 30, //has_value = true (3rd element: Some) 		u32 value = 30 (varint)
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

	t.Run("B256 固定长度哈希", func(t *testing.T) {
		hash := [32]byte{}
		for i := 0; i < 32; i++ {
			hash[i] = byte(i)
		}

		serializer := postcard.NewSerializer()
		err := provider.serializeValue(serializer, "B256", hash)
		if err != nil {
			t.Fatal(err)
		}

		result := serializer.Bytes()
		// 32字节，无长度前缀
		if len(result) != 32 {
			t.Errorf("B256 length: got %d, want 32", len(result))
		}
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
	// Bytes:           [7, M, y, T, o, k, e, n,     3, M, T, K,     18,      192, 132, 61]
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
	// Bytes:         [3,       1, 1, 2, 1,'a', 1,'b',     0,       1, 2, 1, 1,'c']
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
