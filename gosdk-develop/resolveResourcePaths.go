package milon

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/api"
)

// resolvePathBatchSize is the max number of hashes per BatchGetResourcePathByHash call.
// The SDK uses JSON RPC and encodes the request body as a decimal byte array (~4x larger than binary);
// a big block can have thousands of resources, so without sharding the body risks being too large/timing out.
// Any failed batch returns an error for the upper layer to break the stream and retry (never silently skipped, ensuring complete paths for the whole block).
const resolvePathBatchSize = 500

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

	for start := 0; start < len(rsHashList); start += resolvePathBatchSize {
		end := start + resolvePathBatchSize
		if end > len(rsHashList) {
			end = len(rsHashList)
		}

		batchResult, err := client.BatchGetResourcePathByHash(rsHashList[start:end])
		if err != nil {
			return nil, fmt.Errorf("BatchGetResourcePathByHash failed (items %d-%d of %d): %w", start, end, len(rsHashList), err)
		}

		for _, info := range batchResult.BodyBatchResourcePathList {
			if info.ErrMsg != "" {
				pathMap[info.RsHash] = "<err: " + info.ErrMsg + ">"
				continue
			}
			pathMap[info.RsHash] = info.Path
		}
	}

	return pathMap, nil
}
