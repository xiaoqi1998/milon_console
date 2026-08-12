package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
)

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	chainHeadResult, err := client.GetChainHead(1)
	if err != nil {
		panic("Failed to GetChainHead :" + err.Error())
	}
	fmt.Printf("chainHeadResult.BodyChainHead : %+v \n\n", chainHeadResult.BodyChainHead)

	getBlockResult, err := client.GetBlockByHeight(chainHeadResult.BodyChainHead.BlockHeight, 1)
	if err != nil {
		panic("Failed to GetBlockByHeight :" + err.Error())
	}
	fmt.Printf("getBlockResult.BodyBlock : %+v \n", getBlockResult.BodyBlock)
}
