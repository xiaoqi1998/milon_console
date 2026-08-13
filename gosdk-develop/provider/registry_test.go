package provider

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/stretchr/testify/assert"
)

func loadProviders(t *testing.T, names ...string) map[string]*Provider {
	t.Helper()
	providers := make(map[string]*Provider, len(names))
	for _, name := range names {
		pd, err := LoadProviderFromFile("./IDL/" + name + ".idl.json")
		assert.NoError(t, err)
		providers[name] = pd
	}
	return providers
}

// varintU64 encodes value as LEB128 varint bytes, matching the postcard
// serializer wire format; used to build event data with a type_tag prefix.
func varintU64(value uint64) []byte {
	var buf []byte
	for value >= 0x80 {
		buf = append(buf, byte(value&0x7f)|0x80)
		value >>= 7
	}
	return append(buf, byte(value))
}

func TestNewIDLRegistry_Duplicates(t *testing.T) {
	t.Run("duplicate app_id", func(t *testing.T) {
		providers := loadProviders(t, "token")
		providers["token_dup"] = providers["token"] // same app_id=2

		_, err := NewIDLRegistry(providers)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate app_id")
	})

	t.Run("duplicate event type_tag", func(t *testing.T) {
		pd1 := NewProvider(IDL{Metadata: Metadata{AppID: 1, Name: "a"}, Events: []Event{{Name: "E1", TypeTag: 123}}})
		pd2 := NewProvider(IDL{Metadata: Metadata{AppID: 2, Name: "b"}, Events: []Event{{Name: "E2", TypeTag: 123}}})

		_, err := NewIDLRegistry(map[string]*Provider{"a": pd1, "b": pd2})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate event type_tag")
	})

	t.Run("duplicate app name", func(t *testing.T) {
		pd1 := NewProvider(IDL{Metadata: Metadata{AppID: 1, Name: "dup"}})
		pd2 := NewProvider(IDL{Metadata: Metadata{AppID: 2, Name: "dup"}})

		_, err := NewIDLRegistry(map[string]*Provider{"a": pd1, "b": pd2})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate app name")
	})
}

func TestIDLRegistry_DecodeInstructions(t *testing.T) {
	reg, err := NewIDLRegistry(loadProviders(t, "token"))
	assert.NoError(t, err)

	pd := loadProviders(t, "token")["token"]
	token, err := crypto.NewAddressFromRelaxed("0202020202020202020202020202020202020202")
	assert.NoError(t, err)
	to, err := crypto.NewAddressFromRelaxed("0303030303030303030303030303030303030303")
	assert.NoError(t, err)

	wire1, err := pd.Encode("Mint", Args{"token": token, "to": to, "amount": uint64(1000)})
	assert.NoError(t, err)
	wire2, err := pd.Encode("Mint", Args{"token": token, "to": to, "amount": uint64(1)})
	assert.NoError(t, err)

	t.Run("batch decode", func(t *testing.T) {
		decodedList, err := reg.DecodeInstructions([][]byte{wire1, wire2})
		assert.NoError(t, err)
		assert.Len(t, decodedList, 2)

		for _, decoded := range decodedList {
			assert.Equal(t, uint8(2), decoded["app_id"])
			assert.Equal(t, "token", decoded["app_name"])
			assert.Equal(t, "Mint", decoded["instruction_name"])
			assert.Equal(t, uint64(20481), decoded["discriminator"])

			args := decoded["args"].(map[string]any)
			addr, ok := args["token"].(*crypto.Address)
			assert.True(t, ok)
			assert.Equal(t, token.Bytes, addr.Bytes)

			toAddr, ok := args["to"].(*crypto.Address)
			assert.True(t, ok)
			assert.Equal(t, to.Bytes, toAddr.Bytes)
		}
		// each batch element keeps its own args
		assert.Equal(t, uint64(1000), decodedList[0]["args"].(map[string]any)["amount"])
		assert.Equal(t, uint64(1), decodedList[1]["args"].(map[string]any)["amount"])
	})

	t.Run("error propagation", func(t *testing.T) {
		// 2nd element is only 2 bytes -> DecodeInstruction rejects it
		// and the batch fails with the failing index reported
		_, err := reg.DecodeInstructions([][]byte{wire1, {0x99, 0x99}})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode instruction[1]")
	})
}

func TestIDLRegistry_DecodeInstruction(t *testing.T) {
	reg, err := NewIDLRegistry(loadProviders(t, "token"))
	assert.NoError(t, err)

	pd := loadProviders(t, "token")["token"]
	token, err := crypto.NewAddressFromRelaxed("0202020202020202020202020202020202020202")
	assert.NoError(t, err)
	to, err := crypto.NewAddressFromRelaxed("0303030303030303030303030303030303030303")
	assert.NoError(t, err)

	wire, err := pd.Encode("Mint", Args{"token": token, "to": to, "amount": uint64(1000)})
	assert.NoError(t, err)

	decoded, err := reg.DecodeInstruction(wire)
	assert.NoError(t, err)
	assert.Equal(t, uint8(2), decoded["app_id"])
	assert.Equal(t, "token", decoded["app_name"])
	assert.Equal(t, "Mint", decoded["instruction_name"])
	assert.Equal(t, uint64(20481), decoded["discriminator"])

	args := decoded["args"].(map[string]any)
	assert.Equal(t, uint64(1000), args["amount"])

	addr, ok := args["token"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, token.Bytes, addr.Bytes)

	toAddr, ok := args["to"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, to.Bytes, toAddr.Bytes)
}

func TestIDLRegistry_DecodeViewDatas(t *testing.T) {
	reg, err := NewIDLRegistry(loadProviders(t, "token", "demo"))
	assert.NoError(t, err)

	body := []byte{
		2,            // vec length = 2
		0, 2, 188, 5, // token::BalanceOf returns u64(700): [variant=0][okLen=2][188,5]
		0, 1, 1, // demo::OrderBalance returns u64(1): [variant=0][okLen=1][1]
	}

	results, err := reg.DecodeViewDatas([]string{"token::BalanceOf", "demo::OrderBalance"}, body)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	assert.Equal(t, uint64(700), results[0].Value)
	assert.Equal(t, uint64(1), results[1].Value)
}

func TestIDLRegistry_DecodeViewDatas_Errors(t *testing.T) {
	reg, err := NewIDLRegistry(loadProviders(t, "token"))
	assert.NoError(t, err)

	t.Run("unknown app", func(t *testing.T) {
		body := []byte{1}
		_, err := reg.DecodeViewDatas([]string{"nope::BalanceOf"}, body)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown app")
	})

	t.Run("count mismatch", func(t *testing.T) {
		// body declares 1 result but 2 instruction names are given
		_, err := reg.DecodeViewDatas([]string{"token::BalanceOf", "token::BalanceOf"}, []byte{1})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not match instruction count")
	})
}

func TestIDLRegistry_DecodeEventDataByTag(t *testing.T) {
	reg, err := NewIDLRegistry(loadProviders(t, "demo"))
	assert.NoError(t, err)

	// demo EventCreditApplied: pool(Address) + recipient(Address) + amount(u64)
	const typeTag = 7407037194950745602

	var pool [20]byte
	var recipient [20]byte
	recipient[19] = 1

	data := make([]byte, 0, 41)
	data = append(data, pool[:]...)
	data = append(data, recipient[:]...)
	data = append(data, 42) // amount = 42 (varint single byte)

	decoded, err := reg.DecodeEventDataByTag(typeTag, data)
	assert.NoError(t, err)

	assert.Equal(t, uint8(255), decoded["app_id"])
	assert.Equal(t, "demo", decoded["app_name"])
	assert.Equal(t, "EventCreditApplied", decoded["event_name"])

	record := decoded["data"].(map[string]any)

	poolAddr, ok := record["pool"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, pool, poolAddr.Bytes) // all-zero pool address

	recipientAddr, ok := record["recipient"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, recipient, recipientAddr.Bytes) // last byte = 1

	assert.Equal(t, uint64(42), record["amount"])

	// With a leading type_tag varint prefix, which DecodeEventDataByTag skips
	prefixed := make([]byte, 0, len(varintU64(typeTag))+len(data))
	prefixed = append(prefixed, varintU64(typeTag)...)
	prefixed = append(prefixed, data...)

	decodedPrefixed, err := reg.DecodeEventDataByTag(typeTag, prefixed)
	assert.NoError(t, err)
	assert.Equal(t, uint8(255), decodedPrefixed["app_id"])
	assert.Equal(t, "demo", decodedPrefixed["app_name"])
	assert.Equal(t, "EventCreditApplied", decodedPrefixed["event_name"])

	prefixedRecord := decodedPrefixed["data"].(map[string]any)

	prefixedPool, ok := prefixedRecord["pool"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, pool, prefixedPool.Bytes) // prefix skipped; pool intact

	prefixedRecipient, ok := prefixedRecord["recipient"].(*crypto.Address)
	assert.True(t, ok)
	assert.Equal(t, recipient, prefixedRecipient.Bytes) // prefix skipped; recipient intact

	assert.Equal(t, uint64(42), prefixedRecord["amount"])
}

func TestIDLRegistry_DecodeEventDataByTag_Errors(t *testing.T) {
	reg, err := NewIDLRegistry(loadProviders(t, "token"))
	assert.NoError(t, err)

	_, err = reg.DecodeEventDataByTag(999999999, []byte{1, 2, 3})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown type tag")
}

func TestIDLRegistry_FormatDecodedInstruction(t *testing.T) {
	reg, err := NewIDLRegistry(loadProviders(t, "token"))
	assert.NoError(t, err)

	decoded := map[string]any{
		"app_id":           uint8(2),
		"app_name":         "token",
		"instruction_name": "Mint",
		"discriminator":    uint64(20481),
		"args": map[string]any{
			"zebra": uint64(1),
			"alpha": uint64(2),
		},
	}
	out := reg.FormatDecodedInstruction(decoded)

	// header and metadata fields
	assert.Contains(t, out, "[token] Mint")
	assert.Contains(t, out, "appId: 2")
	assert.Contains(t, out, "appName: \"token\"")
	assert.Contains(t, out, "instructionName: \"Mint\"")
	assert.Contains(t, out, "discriminator: 20481")

	// arg blocks: name + typed value
	assert.Contains(t, out, "name: \"alpha\"")
	assert.Contains(t, out, "value: U64(2)")
	assert.Contains(t, out, "name: \"zebra\"")
	assert.Contains(t, out, "value: U64(1)")
}

func TestIDLRegistry_FormatDecodedEvent(t *testing.T) {
	reg, err := NewIDLRegistry(loadProviders(t, "demo"))
	assert.NoError(t, err)

	decoded := map[string]any{
		"app_name":   "demo",
		"event_name": "EventCreditApplied",
		"data":       map[string]any{"amount": uint64(42), "pool": "0x00"},
	}
	out := reg.FormatDecodedEvent(decoded)
	assert.Contains(t, out, "[demo] EventCreditApplied")
	assert.Contains(t, out, "amount: U64(42)")
}
