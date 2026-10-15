package main

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
)

func main() {
	//example1(milon.DevNet)
	example2(milon.DevNet)
}

// example1 demonstrates the gRPC block stream wrapper:
// subscribes starting from block 0 and processes only the first block—printing each transaction with PrintFramedHistory,
// the callback returns false to stop fetching and does nothing else.
func example1(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// StreamBlocks(startBlock, onBlockDone):
	//   startBlock  = 0, starting from the genesis block
	//   onBlockDone = callback invoked after the current block's transactions are fetched; return false to stop fetching
	err := client.StreamBlocks(0, func(height uint64, histories []*milon.FramedHistory) bool {
		fmt.Printf("\n================ block %d: %d tx(s) ================\n", height, len(histories))

		// collect all resource hashes of this block and resolve resource paths in batch (needed for PrintFramedHistory to display the access path)
		rsHashSet := make(map[api.RsHash]struct{})
		for _, fh := range histories {
			for _, ac := range fh.Accesses {
				var rs api.RsHash
				copy(rs[:], ac.ResourceID)
				rsHashSet[rs] = struct{}{}
			}
		}
		pathMap, err := milon.ResolveResourcePaths(client, rsHashSet)
		if err != nil {
			panic("failed to resolve resource paths:" + err.Error())
		}

		for i, fh := range histories {
			milon.PrintFramedHistory(client, i, fh, pathMap)
		}

		return false // fetch only block 0: stop once processed
	})
	if err != nil {
		panic("failed to stream blocks:" + err.Error())
	}
	fmt.Println("\nstream stopped: block 0 handled, callback returned false")
}

// example2 demonstrates continuous consumption of the gRPC block stream:
// subscribes starting from block 0; after each block is processed (resolve resource paths + print PrintFramedHistory),
// the callback returns true to continue fetching—keeping reading data, and after reaching the chain tip it keeps waiting for new blocks.
func example2(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// StreamBlocks(startBlock, onBlockDone):
	//   startBlock  = 0, starting from the genesis block
	//   onBlockDone = callback invoked after the current block's transactions are fetched; return true to continue fetching
	err := client.StreamBlocks(0, func(height uint64, histories []*milon.FramedHistory) bool {
		fmt.Printf("\n================ block %d: %d tx(s) ================\n", height, len(histories))

		// collect all resource hashes of this block and resolve resource paths in batch (needed for PrintFramedHistory to display the access path)
		rsHashSet := make(map[api.RsHash]struct{})
		for _, fh := range histories {
			for _, ac := range fh.Accesses {
				var rs api.RsHash
				copy(rs[:], ac.ResourceID)
				rsHashSet[rs] = struct{}{}
			}
		}
		pathMap, err := milon.ResolveResourcePaths(client, rsHashSet)
		if err != nil {
			panic("failed to resolve resource paths:" + err.Error())
		}

		for i, fh := range histories {
			milon.PrintFramedHistory(client, i, fh, pathMap)
		}

		return true // keep reading: continue fetching the next block
	})
	if err != nil {
		panic("failed to stream blocks:" + err.Error())
	}
}
