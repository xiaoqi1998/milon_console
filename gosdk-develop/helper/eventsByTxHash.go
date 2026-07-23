package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func DisplayEventsByTxHash(client *milon.MolinClient, txHash string, typeTagFilter *uint64) {
	fmt.Printf("\n================ Display EventsByTxHash ================\n")

	eventsByTxHashResult, err := client.EventsByTxHash(txHash, typeTagFilter, 1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("eventsByTxHashResult : %+v \n\n", eventsByTxHashResult)
	fmt.Printf("eventsByTxHashResult.BodyEventsByTxHash : %+v \n\n", eventsByTxHashResult.BodyEventsByTxHash)

	idlManager, err := provider.NewIDLManager(client.RpcClient.GetAllPd())
	if err != nil {
		panic(err)
	}

	for i, eventEntry := range eventsByTxHashResult.BodyEventsByTxHash.Events {
		fmt.Printf("Events index=[%d]:\n", i)
		fmt.Printf("\t	BlockHeight: %d\n", eventEntry.BlockHeight)
		fmt.Printf("\t 	TxHash: %v\n", eventEntry.TxHash)
		fmt.Printf("\t 	TxIndex : %d\n", eventEntry.TxIndex)
		fmt.Printf("\t 	EventIndex: %d\n", eventEntry.EventIndex)
		fmt.Printf("\t	Data (hex): %x\n", eventEntry.Data)

		decodedEvent, err := idlManager.DecodeEventDataByTag(eventEntry.Data.TypeTag, eventEntry.Data.Value)
		if err != nil {
			panic(err)
		}
		fmt.Printf("\t decodedEvent = %+v \n\n", decodedEvent)
		fmt.Printf("\t FormatDecodedEvent: %+v \n\n", idlManager.FormatDecodedEvent(decodedEvent))
	}

	fmt.Printf("\n================ Display EventsByTxHash ================\n")
}
