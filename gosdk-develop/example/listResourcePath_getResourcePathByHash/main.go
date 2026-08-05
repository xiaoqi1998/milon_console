package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
)

func example(networkConfig milon.Network) {
	client := milon.NewClient(networkConfig)

	listResourcePathResult, err := client.ListResourcePath(1)
	if err != nil {
		panic("Failed to ListResourcePath :" + err.Error())
	}

	bodyListResourcePathLen := len(listResourcePathResult.BodyListResourcePaths)
	for i, value := range listResourcePathResult.BodyListResourcePaths {
		fmt.Printf("Resouce index=[%d]:\n", i)
		fmt.Printf("\t RsHash: %d\n", value.RsHash)
		fmt.Printf("\t Path: %v\n", value.Path)

		if i > bodyListResourcePathLen-3 {
			getResourcePathByHashResult, err := client.GetResourcePathByHash(value.RsHash, 1)
			if err != nil {
				panic("Failed to GetResourcePathByHash :" + err.Error())
			}
			fmt.Printf("\t client.GetResourcePathByHash : %v \n", string(getResourcePathByHashResult.HttpRspBody))
		}
	}
}
