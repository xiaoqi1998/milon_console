package main

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk"
)

func Test_Main(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a running Milon node")
	}
	t.Parallel()
	example(milon.DevNet)
}
