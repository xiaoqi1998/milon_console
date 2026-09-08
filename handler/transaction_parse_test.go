package handler

import (
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func newTestAddress(t *testing.T) *crypto.Address {
	t.Helper()
	addrBytes := make([]byte, 20)
	for i := range addrBytes {
		addrBytes[i] = byte(i)
	}
	addr, err := crypto.NewAddressFromBytes(addrBytes)
	if err != nil {
		t.Fatalf("failed to build test address: %v", err)
	}
	return addr
}

func TestJSONDecodedValue(t *testing.T) {
	addr := newTestAddress(t)

	var b256 provider.B256
	copy(b256[:], []byte("01234567890123456789012345678901"))

	u128, ok := new(big.Int).SetString("340282366920938463463374607431768211455", 10)
	if !ok {
		t.Fatal("failed to build u128 test value")
	}

	cases := []struct {
		name  string
		input any
		want  any
	}{
		{"nil", nil, nil},
		{"address", *addr, addr.ToBase58()},
		{"address pointer", addr, addr.ToBase58()},
		{"bigint", u128, "340282366920938463463374607431768211455"},
		{"bytes", []byte{0xde, 0xad}, "dead"},
		{"b256", b256, hex.EncodeToString(b256[:])},
		{"uint64", uint64(42), uint64(42)},
		{"string", "hello", "hello"},
		{"slice", []any{uint8(1), uint8(2)}, []any{uint8(1), uint8(2)}},
		{
			"nested struct",
			map[string]any{"owner": *addr, "amount": big.NewInt(7)},
			map[string]any{"owner": addr.ToBase58(), "amount": "7"},
		},
		{
			"map with non-string keys",
			map[any]any{uint64(1): "a"},
			map[string]any{"1": "a"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonDecodedValue(tc.input); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v (%T), want %#v (%T)", got, got, tc.want, tc.want)
			}
		})
	}
}

func TestJSONDecodedValueNestedRecursion(t *testing.T) {
	addr := newTestAddress(t)

	decoded := map[string]any{
		"app_id":           uint8(2),
		"app_name":         "demo",
		"instruction_name": "transfer",
		"args": map[string]any{
			"to":     *addr,
			"amount": big.NewInt(100),
			"blob":   []byte{0x01, 0x02},
		},
	}

	sanitized, ok := jsonDecodedValue(decoded).(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", sanitized)
	}
	args, ok := sanitized["args"].(map[string]any)
	if !ok {
		t.Fatalf("expected args to remain a map, got %T", sanitized["args"])
	}
	if args["to"] != addr.ToBase58() {
		t.Fatalf("expected address converted to base58, got %v", args["to"])
	}
	if args["amount"] != "100" {
		t.Fatalf("expected big.Int converted to decimal string, got %v", args["amount"])
	}
	if args["blob"] != "0102" {
		t.Fatalf("expected bytes converted to hex, got %v", args["blob"])
	}
}
