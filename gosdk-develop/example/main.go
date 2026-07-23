package main

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func main() {

	client := milon.NewMilonClient(milon.DevNetConfig)

	Client_MIL(client)

	//Client_GetChainHead_GetBlock(client)

	//Client_GetTxByHash(client, "8M12Gp7RqMQQ3uycyh19HTCX3QoDy9ZU1uin8TvEP5WC")
	//Client_GetBlock(client, 10)
	//Client_EventsByTxHash(client, "apiHashBase58", nil)
	//Client_ListResourcePath_GetResourcePathByHash(client)
}

func Client_MIL(client *milon.MolinClient) {
	ViewSingleTransactionResult1, err := client.BuildAndViewSingleIx(
		"token",
		"Metadata", provider.Args{
			"token": "M11on1111111111111111111111",
		},
		1,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("ViewSingleTransactionResult1 : %+v \n", ViewSingleTransactionResult1)
	if failure, ok := ViewSingleTransactionResult1.BodyValues.(*api.TxFailurePayload); ok {
		panic(failure)
	}

	fmt.Printf("ViewSingleTransactionResult1.BodyValues : %+v \n\n", ViewSingleTransactionResult1.BodyValues)

}

func Client_GetChainHead_GetBlock(client *milon.MolinClient) {

	fmt.Printf("\n\n ----------1.GetChainHead---------------------- \n\n")

	chainHeadResult, err := client.GetChainHead(1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("chainHeadResult : %+v \n\n", chainHeadResult)
	fmt.Printf("chainHeadResult.BodyChainHead : %+v \n\n", chainHeadResult.BodyChainHead)

	fmt.Printf("\n\n ----------2.GetBlock---------------------- \n\n")

	getBlockResult, err := client.GetBlock(chainHeadResult.BodyChainHead.BlockHeight, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("getBlockResult : %+v \n\n", getBlockResult)
	fmt.Printf("getBlockResult.BodyBlock : %+v \n", getBlockResult.BodyBlock)
}

func Client_ListResourcePath_GetResourcePathByHash(client *milon.MolinClient) {

	listResourcePathResult, err := client.ListResourcePath(1)
	if err != nil {
		panic(err)
	}

	fmt.Printf("listResourcePathResult : %+v \n", listResourcePathResult)

	bodyListResourcePathLen := len(listResourcePathResult.BodyListResourcePaths)

	for i, value := range listResourcePathResult.BodyListResourcePaths {
		fmt.Printf("Resouce(资源) index=[%d]:\n", i)
		fmt.Printf("\t RsHash: %d\n", value.RsHash)
		fmt.Printf("\t Path: %v\n", value.Path)

		if i > bodyListResourcePathLen-3 {
			getResourcePathByHashResult, err := client.GetResourcePathByHash(value.RsHash, 1)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\t path : %v \n", string(getResourcePathByHashResult.HttpRspBody))
		}
	}
}
