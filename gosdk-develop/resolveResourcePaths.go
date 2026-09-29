package milon

import (
	"fmt"

	"github.com/milon-labs/milon-go-sdk/api"
)

// resolvePathBatchSize 单次 BatchGetResourcePathByHash 的最大 hash 数。
// SDK 走 JSON RPC 且把请求体编码为十进制字节数组(相对二进制膨胀约 4 倍),
// 大块资源数可上千,不分片会有请求体过大/超时的风险;任何一批失败都返回 error,
// 由上层断流重试(绝不静默跳过,保证整块路径完整)。
const resolvePathBatchSize = 500

// ResolveResourcePaths 按块批量查询资源路径,分批 RPC 拿回本块所有资源的 path;
// RPC 失败返回 error,调用方中断本块处理,避免整块路径缺失被静默跳过。
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
