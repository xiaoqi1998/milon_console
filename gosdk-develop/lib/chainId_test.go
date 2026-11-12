package lib

import "testing"

// snapshotChainID, restoreChainID and clearChainID let a test drive the
// process-wide chain id without leaking state into other tests.
func snapshotChainID() (uint64, bool) {
	chainIDMutex.RLock()
	defer chainIDMutex.RUnlock()
	return chainID, chainIDSet
}

func restoreChainID(id uint64, set bool) {
	chainIDMutex.Lock()
	defer chainIDMutex.Unlock()
	chainID, chainIDSet = id, set
}

func clearChainID() {
	restoreChainID(0, false)
}

func TestSetChainIdRejectsZero(t *testing.T) {
	if err := SetChainId(0); err == nil {
		t.Fatal("SetChainId must reject zero: it is indistinguishable from unset")
	}
}

func TestGetChainIdPanicsWhenUnset(t *testing.T) {
	savedID, savedSet := snapshotChainID()
	t.Cleanup(func() { restoreChainID(savedID, savedSet) })
	clearChainID()

	if ChainIdSet() {
		t.Fatal("chain id must start unset")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("GetChainId must panic while the chain id is unset")
		}
	}()
	GetChainId()
}

func TestChainIdRoundTrip(t *testing.T) {
	savedID, savedSet := snapshotChainID()
	t.Cleanup(func() { restoreChainID(savedID, savedSet) })
	clearChainID()

	if err := SetChainId(900_000_001); err != nil {
		t.Fatalf("SetChainId failed: %v", err)
	}
	if !ChainIdSet() {
		t.Fatal("ChainIdSet must report true after SetChainId")
	}
	if got := GetChainId(); got != 900_000_001 {
		t.Fatalf("GetChainId = %d, want %d", got, uint64(900_000_001))
	}
}
