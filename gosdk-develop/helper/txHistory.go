package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// DisplayTxHistory prints a decoded transaction history: its instructions,
// access records (including decoded resource values) and events.
func DisplayTxHistory(client *milon.Client, txHistory *api.TxHistory) {
	fmt.Printf("\n================ Display TxHistory ================\n")

	if txHistory == nil {
		fmt.Printf("txHistory: nil\n")
		fmt.Printf("\n================ Display TxHistory ================\n")
		return
	}

	fmt.Printf("txHistory: %+v \n\n", txHistory)

	displayInstructions(client, txHistory.Instructions)
	displayAccessRecords(client, txHistory.Receipt.Access)
	displayEvents(client, txHistory.Receipt.Events)

	fmt.Printf("\n================ Display TxHistory ================\n")
}

func displayInstructions(client *milon.Client, instructions []api.PackedInstruction) {
	fmt.Printf("\nInstructions (len=%d):\n", len(instructions))
	for i, instruction := range instructions {
		fmt.Printf("\t [%d] instruction: \n", i)

		decodedInstruction, err := client.GetProviderManager().DecodeInstruction(instruction)
		if err != nil {
			panic("failed to decode instruction: " + err.Error())
		}

		fmt.Printf("\t\t decodedInstruction = %#v \n", decodedInstruction)
		fmt.Printf("\t\t FormatDecodedInstruction = %+v \n\n", client.GetProviderManager().FormatDecodedInstruction(decodedInstruction))
	}
}

func displayAccessRecords(client *milon.Client, records []api.AccessRecord) {
	fmt.Printf("\nAccess Records (len=%d):\n", len(records))
	for i, record := range records {
		fmt.Printf("\t [%d] ResourceID: %v\n", i, record.ResourceID)

		displayResourcePath(client, record.ResourceID)

		displayPersistedValue(client, "FirstSnapshot", record.FirstSnapshot)
		displayLastWritten(client, record)
	}

	displayBatchResourcePaths(client, records)
}

// displayBatchResourcePaths prints all resource paths with a single
// BatchGetResourcePathByHash RPC call.
func displayBatchResourcePaths(client *milon.Client, records []api.AccessRecord) {
	if len(records) == 0 {
		return
	}

	rsHashList := make([]api.RsHash, 0, len(records))
	for _, record := range records {
		rsHashList = append(rsHashList, record.ResourceID)
	}

	batchResult, err := client.BatchGetResourcePathByHash(rsHashList)
	if err != nil {
		fmt.Printf("\nBatchGetResourcePathByHash error: %+v \n", err)
		return
	}

	fmt.Printf("\nBatch Resource Paths (len=%d):\n", len(batchResult.BodyBatchResourcePathList))
	for i, info := range batchResult.BodyBatchResourcePathList {
		if info.ErrMsg != "" {
			fmt.Printf("\t [%d] RsHash: %v, Err: %s\n", i, info.RsHash, info.ErrMsg)
			continue
		}
		fmt.Printf("\t [%d] RsHash: %v, Path: %s\n", i, info.RsHash, info.Path)
	}
}

// displayResourcePath prints the on-chain path resolved from a resource hash.
func displayResourcePath(client *milon.Client, resourceID api.RsHash) {
	getResourcePathResult, err := client.GetResourcePathByHash(resourceID)
	if err != nil {
		fmt.Printf("\t\t client.GetResourcePathByHash error: %+v \n", err)
		return
	}

	fmt.Printf("\t\t client.ResourcePath: %s\n", getResourcePathResult.Path)
}

// displayLastWritten prints the last-written value and, when it is an inline
// value, also fetches the current on-chain resource value so the snapshot at
// tx time can be compared with the latest state.
func displayLastWritten(client *milon.Client, record api.AccessRecord) {
	pd, idlType := displayPersistedValue(client, "LastWritten", &record.LastWritten)
	if pd == nil || idlType == nil {
		return
	}

	getResourceResult, err := client.GetResource(record.ResourceID)
	if err != nil {
		fmt.Printf("\t\t\t client.GetResource error: %+v \n", err)
		return
	}

	valueDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, getResourceResult.BodyGetResource.Data.Value)
	if err != nil {
		panic("failed to decode LastWritten GetResource data: " + err.Error())
	}

	fmt.Printf("\t\t\t now client.GetResource.BodyGetResource: %+v\n", getResourceResult.BodyGetResource)
	fmt.Printf("\t\t\t now getResourceResult.BodyGetResource.Data.Value Decoded (%s): %+v\n\n", idlType.Name, valueDecoded)
}

// displayPersistedValue prints one PersistedValue (Inline or External). For the
// Inline variant it also returns the resolved IDL type for further lookups.
func displayPersistedValue(client *milon.Client, label string, pv *api.PersistedValue) (*provider.Provider, *provider.IDLType) {
	if pv == nil {
		fmt.Printf("\t\t %s: None\n", label)
		return nil, nil
	}

	fmt.Printf("\t\t %s: \n", label)
	switch pv.Variant {
	case 0: // Inline
		fmt.Printf("\t\t\t Inline(type_tag=%d, data_len=%d)\n", pv.TypeTag, len(pv.InlineData))
		fmt.Printf("\t\t\t Data: %x\n", pv.InlineData)
		return decodeInlineData(client, label, pv.TypeTag, pv.InlineData)
	case 1: // External
		fmt.Printf("\t\t\t External(BlobHash=%x)\n", pv.ExternalHash)
		displayExternalData(client, label, pv.ExternalHash)
		return nil, nil
	default:
		panic(fmt.Sprintf("Unknown(variant=%d)", pv.Variant))
	}
}

func displayExternalData(client *milon.Client, label string, blobHash api.BlobHash) {
	getAccessValueResult, err := client.GetAccessValue([]api.BlobHash{blobHash})
	if err != nil {
		panic("failed to get " + label + " access value: " + err.Error())
	}

	fmt.Printf("\t\t\t now client.GetAccessValue: %+v \n\n", getAccessValueResult)
	for j, value := range getAccessValueResult.BodyGetAccessValues {
		fmt.Printf("\t\t\t\t [%d] BlobHash: %+v\n", j, value.BlobHash)
		fmt.Printf("\t\t\t\t [%d] Data: %+v\n", j, value.Data)

		if value.Data == nil {
			continue
		}

		pd, idlType := findIDLTypeByTag(client, value.Data.TypeTag)
		if pd == nil || idlType == nil {
			continue
		}

		decodedValue, err := pd.DecodeDataByIDLTypeName(idlType.Name, value.Data.Value)
		if err != nil {
			panic("failed to decode " + label + " access value data: " + err.Error())
		}

		fmt.Printf("\t\t\t\t [%d] Value Decoded (%s): %+v\n", j, idlType.Name, decodedValue)
	}
}

func displayEvents(client *milon.Client, events []api.TypeTagWithData) {
	fmt.Printf("\nEvents (len=%d):\n", len(events))
	for i, event := range events {
		fmt.Printf("\t [%d] TypeTag: %d \n", i, event.TypeTag)
		fmt.Printf("\t\t Value (hex): %x\n", event.Value)

		decodedEvent, err := client.GetProviderManager().DecodeEventDataByTag(event.TypeTag, event.Value)
		if err != nil {
			panic("failed to decode event: " + err.Error())
		}

		fmt.Printf("\t\t decodedEvent: %+v\n", decodedEvent)
		fmt.Printf("\t\t FormatDecodedEvent: %+v \n\n", client.GetProviderManager().FormatDecodedEvent(decodedEvent))
	}
}

func findIDLTypeByTag(client *milon.Client, typeTag uint64) (*provider.Provider, *provider.IDLType) {
	for _, pd := range client.GetAllPd() {
		if idlType, ok := pd.GetIDLTypeByTypeTag(typeTag); ok {
			return pd, idlType
		}
	}
	return nil, nil
}

func decodeInlineData(client *milon.Client, label string, typeTag uint64, data []byte) (*provider.Provider, *provider.IDLType) {
	pd, idlType := findIDLTypeByTag(client, typeTag)
	if pd == nil || idlType == nil {
		fmt.Printf("\t\t\t Warning: unknown type_tag %d, skipping decode\n", typeTag)
		return nil, nil
	}

	valueDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, data)
	if err != nil {
		panic("failed to decode " + label + " InlineData: " + err.Error())
	}

	fmt.Printf("\t\t\t Value Decoded (%s): %+v\n", idlType.Name, valueDecoded)
	return pd, idlType
}
