package test

import (
	"os"
	"testing"

	"github.com/milon-labs/milon-go-sdk/lib"
)

// TestMain configures the process-wide chain id required by transaction
// hashing and signing; the SDK no longer ships a default chain id.
func TestMain(m *testing.M) {
	if err := lib.SetChainId(900_000_001); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
