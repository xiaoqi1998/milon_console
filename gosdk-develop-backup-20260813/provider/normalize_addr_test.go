package provider

import (
	"strings"
	"testing"

	"github.com/milon-labs/milon-go-sdk/crypto"
)

func TestNormalizeAddressValue(t *testing.T) {
	// Generate a real milon address (20-byte base58) to use as valid input.
	sk := crypto.NewClassicalSecretKey()
	cs := crypto.AsClassicalSecretKey(sk)
	pk, err := cs.Secp256k1Public()
	if err != nil {
		t.Fatalf("derive pubkey: %v", err)
	}
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		t.Fatalf("derive addr: %v", err)
	}
	valid := addr.ToBase58()
	validHex := addr.ToHex()

	cases := []struct {
		name      string
		typeName  string
		value     any
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid base58",
			typeName: "Address",
			value:    valid,
			wantErr:  false,
		},
		{
			name:     "valid hex",
			typeName: "Address",
			value:    "0x" + validHex,
			wantErr:  false,
		},
		{
			name:      "truncated address",
			typeName:  "Address",
			value:     "RqcFJ3s4kzLQicJGmhsMxbJalxM", // truncated 27 chars
			wantErr:   true,
			errSubstr: "invalid address",
		},
		{
			name:      "too-short address",
			typeName:  "Address",
			value:     "abc",
			wantErr:   true,
			errSubstr: "invalid address",
		},
		{
			name:      "garbage string",
			typeName:  "Address",
			value:     "!!!not-an-address!!!",
			wantErr:   true,
			errSubstr: "invalid address",
		},
		{
			name:     "nil option",
			typeName: "option<Address>",
			value:    nil,
			wantErr:  false,
		},
		{
			name:     "valid option address",
			typeName: "option<Address>",
			value:    valid,
			wantErr:  false,
		},
		{
			name:     "vec of valid addresses",
			typeName: "vec<Address>",
			value:    []any{valid, valid},
			wantErr:  false,
		},
		{
			name:      "vec with one bad address",
			typeName:  "vec<Address>",
			value:     []any{valid, "short"},
			wantErr:   true,
			errSubstr: "invalid address",
		},
		{
			name:     "non-address type passes through",
			typeName: "u8",
			value:    42,
			wantErr:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeAddressValue(tc.typeName, tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.errSubstr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeArgs(t *testing.T) {
	sk := crypto.NewClassicalSecretKey()
	cs := crypto.AsClassicalSecretKey(sk)
	pk, err := cs.Secp256k1Public()
	if err != nil {
		t.Fatalf("derive pubkey: %v", err)
	}
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		t.Fatalf("derive addr: %v", err)
	}
	valid := addr.ToBase58()

	// Build a synthetic provider with one instruction that has an Address arg.
	p := NewProvider(IDL{
		Metadata: Metadata{AppID: 1, Name: "test"},
		Instructions: []Instruction{
			{
				Name:          "GetHolder",
				Discriminator: 1,
				Kind:          "view",
				Args: []Arg{
					{Name: "holder", Role: "input", Type: "Address"},
				},
			},
		},
	})

	// Valid args -> normalized to crypto.Address
	norm, err := p.NormalizeArgs("GetHolder", Args{"holder": valid})
	if err != nil {
		t.Fatalf("NormalizeArgs valid: %v", err)
	}
	if _, ok := norm["holder"].(crypto.Address); !ok {
		if _, ok2 := norm["holder"].(*crypto.Address); !ok2 {
			t.Fatalf("expected holder normalized to crypto.Address, got %T", norm["holder"])
		}
	}

	// Invalid args -> clear error
	if _, err := p.NormalizeArgs("GetHolder", Args{"holder": "RqcFJ3s4kzLQicJGmhsMxbJalxM"}); err == nil {
		t.Fatalf("expected error for truncated address, got nil")
	}
}
