package main

import (
	"testing"
)

//import "github.com/milon-labs/milon-go-sdk"

import "github.com/milon-labs/milon-go-sdk"

func Test_Main(t *testing.T) {
	t.Parallel()
	example(milon.DevNetConfig)
}
