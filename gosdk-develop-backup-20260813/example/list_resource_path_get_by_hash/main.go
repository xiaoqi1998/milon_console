package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
)

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	listResourcePathResult, err := client.ListResourcePath()
	if err != nil {
		panic("failed to ListResourcePath :" + err.Error())
	}

	rsHashList := make([]api.RsHash, 0)

	bodyListResourcePathLen := len(listResourcePathResult.BodyListResourcePaths)
	for i, value := range listResourcePathResult.BodyListResourcePaths {
		fmt.Printf("Resouce index=[%d]:\n", i)
		fmt.Printf("\t RsHash: %d\n", value.RsHash)
		fmt.Printf("\t Path: %#v\n", value.Path)

		if i > bodyListResourcePathLen-20 {
			rsHashList = append(rsHashList, value.RsHash)

			getResourcePathByHashResult, err := client.GetResourcePathByHash(value.RsHash)
			if err != nil {
				fmt.Printf("\t failed to GetResourcePathByHash: %v\n", err.Error())
			} else {
				fmt.Printf("\t client.GetResourcePathByHash: %#v\n", getResourcePathByHashResult.Path)
			}
		}
	}

	fmt.Printf("\n================ BatchGetResourcePathByHash ================\n\n")

	batchGetResourcePathByHashResult, err := client.BatchGetResourcePathByHash(rsHashList)
	if err != nil {
		panic("failed to BatchGetResourcePathByHash :" + err.Error())
	}
	for _, r := range batchGetResourcePathByHashResult.BodyBatchResourcePathList {
		if r.ErrMsg != "" {
			fmt.Printf("RsHash %d -> Err(%s)\n", r.RsHash, r.ErrMsg)
		} else {
			fmt.Printf("RsHash %d -> Ok(path=%s)\n", r.RsHash, r.Path)
		}
	}
}
