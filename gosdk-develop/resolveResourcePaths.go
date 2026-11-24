package milon

import (
	"fmt"
	"sync"

	"github.com/milon-labs/milon-go-sdk/api"
)

// resolvePathBatchSize is the max number of hashes per BatchGetResourcePathByHash call.
// The SDK uses JSON RPC and encodes the request body as a decimal byte array (~4x larger than binary);
// a big block can have thousands of resources, so without sharding the body risks being too large/timing out.
// Any failed batch returns an error for the upper layer to break the stream and retry (never silently skipped, ensuring complete paths for the whole block).
const resolvePathBatchSize = 128

// resolvePathConcurrency is how many BatchGetResourcePathByHash calls are in
// flight at once. The shards are independent RPCs, so a block with thousands of
// resources resolves in parallel instead of one round trip at a time; the cap
// keeps a single block from flooding the node.
const resolvePathConcurrency = 8

// ResolveResourcePaths queries resource paths per block in batches, fetching the path of every resource in the block via batched RPC;
// on RPC failure it returns an error so the caller aborts processing the block, avoiding a whole block's missing paths being silently skipped.
func ResolveResourcePaths(client *Client, rsHashSet map[api.RsHash]struct{}) (map[api.RsHash]string, error) {
	pathMap := make(map[api.RsHash]string, len(rsHashSet))
	if len(rsHashSet) == 0 {
		return pathMap, nil
	}

	rsHashList := make([]api.RsHash, 0, len(rsHashSet))
	for rs := range rsHashSet {
		rsHashList = append(rsHashList, rs)
	}

	// Each shard writes only to its own slot, so no lock is needed; results are
	// merged in shard order once every goroutine has finished.
	type batchOutcome struct {
		infos []*api.BatchGetResourcePathInfo
		err   error
	}

	batchCount := (len(rsHashList) + resolvePathBatchSize - 1) / resolvePathBatchSize
	outcomes := make([]batchOutcome, batchCount)

	sem := make(chan struct{}, resolvePathConcurrency)
	var wg sync.WaitGroup
	for i := range batchCount {
		start := i * resolvePathBatchSize
		end := min(start+resolvePathBatchSize, len(rsHashList))

		wg.Add(1)
		index := i
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			batchResult, err := client.BatchGetResourcePathByHash(rsHashList[start:end])
			if err != nil {
				outcomes[index].err = fmt.Errorf("BatchGetResourcePathByHash failed (items %d-%d of %d): %w", start, end, len(rsHashList), err)
				return
			}
			outcomes[index].infos = batchResult.BodyBatchResourcePathList
		}()
	}
	wg.Wait()

	// Report the first failure by shard position rather than by completion order,
	// so the same failing block always yields the same error.
	for i := range outcomes {
		if outcomes[i].err != nil {
			return nil, outcomes[i].err
		}
		for _, info := range outcomes[i].infos {
			if info.ErrMsg != "" {
				pathMap[info.RsHash] = "<err: " + info.ErrMsg + ">"
				continue
			}
			pathMap[info.RsHash] = info.Path
		}
	}

	return pathMap, nil
}
