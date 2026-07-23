package main

import (
	"github.com/milon-labs/milon-go-sdk"
	"testing"
)

func Test_Main(t *testing.T) {
	t.Parallel()
	example(milon.DevNetConfig)
}
