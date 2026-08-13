package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
)

func DisplayEventsByTxHash(client *milon.Client, txHash any, typeTagFilter *uint64) {
	fmt.Printf("\n================ Display EventsByTxHash ================\n")

	eventsByTxHashResult, err := client.EventsByTxHash(txHash, typeTagFilter)
	if err != nil {
		panic("failed to get events by tx hash: " + err.Error())
	}
	fmt.Printf("BodyEventsByTxHash : %+v \n\n", eventsByTxHashResult.BodyEventsByTxHash)

	for i, eventEntry := range eventsByTxHashResult.BodyEventsByTxHash.Events {
		fmt.Printf("Events index=[%d]:\n", i)
		fmt.Printf("\t BlockHeight: %d\n", eventEntry.BlockHeight)
		fmt.Printf("\t TxHash: %v\n", eventEntry.TxHash)
		fmt.Printf("\t TxIndex : %d\n", eventEntry.TxIndex)
		fmt.Printf("\t EventIndex: %d\n", eventEntry.EventIndex)
		fmt.Printf("\t Data: %+v\n", eventEntry.Data)

		decodedEvent, err := client.GetProviderManager().DecodeEventDataByTag(eventEntry.Data.TypeTag, eventEntry.Data.Value)
		if err != nil {
			panic("failed to decode event data: " + err.Error())
		}
		fmt.Printf("\t decodedEvent = %+v \n\n", decodedEvent)
		fmt.Printf("\t FormatDecodedEvent: %+v \n\n", client.GetProviderManager().FormatDecodedEvent(decodedEvent))
	}

	fmt.Printf("\n================ Display EventsByTxHash ================\n")
}
