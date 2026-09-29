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

// example1 演示 gRPC 区块流封装:
// 从 block 0 开始订阅,只处理第一个块——逐笔打印 PrintFramedHistory,
// 回调返回 false 即停止获取,不做其余逻辑。
func example1(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// StreamBlocks(startBlock, onBlockDone):
	//   startBlock  = 0,从创世块开始
	//   onBlockDone = 当前块交易获取完毕后的回调,返回 false 停止获取
	err := client.StreamBlocks(0, func(height uint64, histories []*milon.FramedHistory) bool {
		fmt.Printf("\n================ block %d: %d tx(s) ================\n", height, len(histories))

		// 收集本块全部资源 hash,批量解析资源路径(PrintFramedHistory 展示 access path 需要)
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

		return false // 只获取 block 0:处理完即停
	})
	if err != nil {
		panic("failed to stream blocks:" + err.Error())
	}
	fmt.Println("\nstream stopped: block 0 handled, callback returned false")
}

// example2 演示 gRPC 区块流持续消费:
// 从 block 0 开始订阅,每块处理完(解析资源路径 + 打印 PrintFramedHistory)后
// 回调返回 true 继续获取——一直读取数据,到链尖后继续等待新块。
func example2(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	// StreamBlocks(startBlock, onBlockDone):
	//   startBlock  = 0,从创世块开始
	//   onBlockDone = 当前块交易获取完毕后的回调,返回 true 继续获取
	err := client.StreamBlocks(0, func(height uint64, histories []*milon.FramedHistory) bool {
		fmt.Printf("\n================ block %d: %d tx(s) ================\n", height, len(histories))

		// 收集本块全部资源 hash,批量解析资源路径(PrintFramedHistory 展示 access path 需要)
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

		return true // 持续读取:继续获取下一块
	})
	if err != nil {
		panic("failed to stream blocks:" + err.Error())
	}
}
