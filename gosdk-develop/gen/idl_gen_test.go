package gen

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk/provider"
)

// tokenProvider builds a provider from the generated inline IDL, so the test
// does not depend on the JSON files under provider/IDL.
func tokenProvider(t *testing.T) *provider.Provider {
	t.Helper()
	for _, idl := range DefaultIDLs {
		if idl.Metadata.Name == "token" {
			return provider.NewProvider(idl)
		}
	}
	t.Fatal("generated IDLs carry no token app")
	return nil
}

// TestDecodeView_UsesSingleViewBody guards the MethodTypeView contract: the body
// handed to a generated DecodeView method is the bare return value, so a batch
// view body (vec length + Result envelope) must be rejected instead of being
// silently decoded.
func TestDecodeView_UsesSingleViewBody(t *testing.T) {
	pd := tokenProvider(t)

	t.Run("bare return value", func(t *testing.T) {
		supply, err := (&TokenTotalSupplyIx{pd: pd}).DecodeView([]byte{188, 5})
		if err != nil {
			t.Fatalf("TotalSupply DecodeView: %v", err)
		}
		if supply != 700 {
			t.Fatalf("TotalSupply = %d, want 700", supply)
		}

		balance, err := (&TokenBalanceOfIx{pd: pd}).DecodeView([]byte{188, 5})
		if err != nil {
			t.Fatalf("BalanceOf DecodeView: %v", err)
		}
		if balance != 700 {
			t.Fatalf("BalanceOf = %d, want 700", balance)
		}
	})

	t.Run("batch view body is rejected", func(t *testing.T) {
		// vec(length=1) + Result(Ok) + u64(700) belongs to method 16.
		_, err := (&TokenTotalSupplyIx{pd: pd}).DecodeView([]byte{1, 0, 2, 188, 5})
		if err == nil {
			t.Fatal("DecodeView accepted a batch view body")
		}
	})

	t.Run("unbound instruction", func(t *testing.T) {
		_, err := (&TokenTotalSupplyIx{}).DecodeView([]byte{0})
		if err == nil {
			t.Fatal("DecodeView on an unbound instruction must fail")
		}
	})
}
