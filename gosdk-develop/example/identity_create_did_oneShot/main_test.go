package main

import (
	"testing"

	"github.com/milon-labs/milon-go-sdk"
)

func Test_Main(t *testing.T) {
	t.Parallel()
	example(milon.DevNetConfig)
}
