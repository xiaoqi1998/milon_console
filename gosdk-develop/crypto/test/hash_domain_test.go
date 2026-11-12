package test

import (
	"bytes"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"sync"
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

// TestAcquireHasherMatchesUnpooledHasher pins the pooled path to the unpooled
// one: a reused hasher must produce exactly the same digest.
func TestAcquireHasherMatchesUnpooledHasher(t *testing.T) {
	domain := crypto.IxHashDomainBytes
	msg := []byte("milon pool check")

	unpooled := crypto.Hasher(domain)
	unpooled.Write(msg)
	var want [32]byte
	unpooled.Sum(want[:0])

	// Two rounds, so the second one reuses a released hasher.
	for i := 0; i < 2; i++ {
		hasher := crypto.AcquireHasher(domain)
		hasher.Write(msg)
		var got [32]byte
		hasher.Sum(got[:0])
		crypto.ReleaseHasher(hasher)
		assert.Equal(t, want, got)
	}
}

// TestAcquireHasherResetsStateAcrossReuse guards against a released hasher
// carrying digest state into the next acquisition.
func TestAcquireHasherResetsStateAcrossReuse(t *testing.T) {
	domain := crypto.TxHashDomainBytes

	hasher := crypto.AcquireHasher(domain)
	hasher.Write([]byte("dirty payload"))
	var sink [32]byte
	hasher.Sum(sink[:0])
	crypto.ReleaseHasher(hasher)

	want := crypto.Hash32(domain, []byte("clean payload"))
	reused := crypto.AcquireHasher(domain)
	reused.Write([]byte("clean payload"))
	var got [32]byte
	reused.Sum(got[:0])
	crypto.ReleaseHasher(reused)

	assert.Equal(t, want, got)
}

// TestAcquireHasherConcurrent must be run with -race: a pooled hasher must never
// be in use by two goroutines at the same time.
func TestAcquireHasherConcurrent(t *testing.T) {
	domain := crypto.TxAuthDomainBytes
	payloads := [][]byte{[]byte("alpha"), []byte("beta"), []byte("gamma"), []byte("delta")}

	want := make([][32]byte, len(payloads))
	for i, p := range payloads {
		want[i] = crypto.Hash32(domain, p)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for round := 0; round < 50; round++ {
				idx := (worker + round) % len(payloads)
				hasher := crypto.AcquireHasher(domain)
				hasher.Write(payloads[idx])
				var got [32]byte
				hasher.Sum(got[:0])
				crypto.ReleaseHasher(hasher)
				if got != want[idx] {
					t.Errorf("worker %d round %d: digest mismatch", worker, round)
					return
				}
			}
		}(worker)
	}
	wg.Wait()
}
