package test

import (
	"bytes"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHash32DeterministicAndDomainSeparated(t *testing.T) {
	msg := []byte("hello milon")

	h1 := crypto.Hash32([]byte("domain-a"), msg)
	h2 := crypto.Hash32([]byte("domain-a"), msg)
	h3 := crypto.Hash32([]byte("domain-b"), msg)

	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Len(t, h1, 32)
}

func TestHash32PartsEquivalentToConcat(t *testing.T) {
	domain := []byte("milon.ix.v1")
	parts := [][]byte{[]byte("a"), []byte("b"), []byte("c")}

	concat := crypto.Hash32(domain, bytes.Join(parts, nil))
	split := crypto.Hash32(domain, parts...)
	assert.Equal(t, concat, split)
}
