package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// DisplayTxHistory prints a decoded transaction history: its instructions,
// access records (including decoded resource values) and events.
func DisplayTxHistory(client *milon.Client, txHistory *api.TxHistory) error {
	fmt.Printf("\n================ Display TxHistory ================\n")

	if txHistory == nil {
		fmt.Printf("txHistory: nil\n")
		fmt.Printf("\n================ Display TxHistory ================\n")
		return nil
	}

	fmt.Printf("txHistory: %+v \n\n", txHistory)

	if err := displayInstructions(client, txHistory.Instructions); err != nil {
		return err
	}
	if err := displayAccessRecords(client, txHistory.Receipt.Access); err != nil {
		return err
	}
	if err := displayEvents(client, txHistory.Receipt.Events); err != nil {
		return err
	}

	fmt.Printf("\n================ Display TxHistory ================\n")
	return nil
}

func displayInstructions(client *milon.Client, instructions []api.PackedInstruction) error {
	fmt.Printf("\nInstructions (len=%d):\n", len(instructions))
	for i, instruction := range instructions {
		fmt.Printf("\t [%d] instruction: \n", i)

		decodedInstruction, err := client.GetProviderManager().DecodeInstruction(instruction)
		if err != nil {
			return fmt.Errorf("failed to decode instruction: %w", err)
		}

		fmt.Printf("\t\t decodedInstruction = %#v \n", decodedInstruction)
		fmt.Printf("\t\t FormatDecodedInstruction = %+v \n\n", client.GetProviderManager().FormatDecodedInstruction(decodedInstruction))
	}
	return nil
}

// displayAccessRecords prints one access record per resource. The receipt keeps
// them sorted by resource ID hash ascending, so a consumer looking for a
// specific resource must match ResourceID rather than a fixed index.
func displayAccessRecords(client *milon.Client, records []api.AccessRecord) error {
	fmt.Printf("\nAccess Records (len=%d):\n", len(records))
	for i, record := range records {
		fmt.Printf("\t [%d] ResourceID: %v\n", i, record.ResourceID)

		displayResourcePath(client, record.ResourceID)

		if _, _, err := displayPersistedValue(client, "FirstSnapshot", record.FirstSnapshot); err != nil {
			return err
		}
		if err := displayLastWritten(client, record); err != nil {
			return err
		}
	}

	displayBatchResourcePaths(client, records)
	return nil
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
func displayLastWritten(client *milon.Client, record api.AccessRecord) error {
	pd, idlType, err := displayPersistedValue(client, "LastWritten", &record.LastWritten)
	if err != nil {
		return err
	}
	if pd == nil || idlType == nil {
		return nil
	}

	getResourceResult, err := client.GetResource(record.ResourceID)
	if err != nil {
		fmt.Printf("\t\t\t client.GetResource error: %+v \n", err)
		return nil
	}

	valueDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, getResourceResult.BodyGetResource.Data.Value)
	if err != nil {
		return fmt.Errorf("failed to decode LastWritten GetResource data: %w", err)
	}

	fmt.Printf("\t\t\t now client.GetResource.BodyGetResource: %+v\n", getResourceResult.BodyGetResource)
	fmt.Printf("\t\t\t now getResourceResult.BodyGetResource.Data.Value Decoded (%s): %+v\n\n", idlType.Name, valueDecoded)
	return nil
}

// displayPersistedValue prints one PersistedValue (Inline or External). For the
// Inline variant it also returns the resolved IDL type for further lookups.
func displayPersistedValue(client *milon.Client, label string, pv *api.PersistedValue) (*provider.Provider, *provider.IDLType, error) {
	if pv == nil {
		fmt.Printf("\t\t %s: None\n", label)
		return nil, nil, nil
	}

	fmt.Printf("\t\t %s: \n", label)
	switch pv.Variant {
	case 0: // Inline
		fmt.Printf("\t\t\t Inline(type_tag=%d, data_len=%d)\n", pv.TypeTag, len(pv.InlineData))
		fmt.Printf("\t\t\t Data: %x\n", pv.InlineData)
		return decodeInlineData(client, label, pv.TypeTag, pv.InlineData)
	case 1: // External
		fmt.Printf("\t\t\t External(BlobHash=%x)\n", pv.ExternalHash)
		if err := displayExternalData(client, label, pv.ExternalHash); err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown PersistedValue variant %d", pv.Variant)
	}
}

// displayExternalData fetches and prints the blob an External PersistedValue
// points at. A blob the node no longer has (Data == nil) is reported instead of
// being treated as an error.
func displayExternalData(client *milon.Client, label string, blobHash api.BlobHash) error {
	getAccessValueResult, err := client.GetAccessValue(blobHash)
	if err != nil {
		return fmt.Errorf("failed to get %s access value: %w", label, err)
	}

	accessValue := getAccessValueResult.BodyGetAccessValue
	if accessValue == nil || accessValue.Data == nil {
		fmt.Printf("\t\t\t no access value stored for BlobHash=%x\n", blobHash)
		return nil
	}

	fmt.Printf("\t\t\t AccessValue(BlobHash=%x, type_tag=%d, data_len=%d)\n",
		accessValue.BlobHash[:], accessValue.Data.TypeTag, len(accessValue.Data.Value))
	fmt.Printf("\t\t\t Data: %x\n", accessValue.Data.Value)

	pd, idlType := client.FindIDLTypeByTag(accessValue.Data.TypeTag)
	if pd == nil || idlType == nil {
		fmt.Printf("\t\t\t Warning: unknown type_tag %d, skipping decode\n", accessValue.Data.TypeTag)
		return nil
	}

	decodedValue, err := pd.DecodeDataByIDLTypeName(idlType.Name, accessValue.Data.Value)
	if err != nil {
		return fmt.Errorf("failed to decode %s access value data: %w", label, err)
	}

	fmt.Printf("\t\t\t Value Decoded (%s): %+v\n\n", idlType.Name, decodedValue)
	return nil
}

func displayEvents(client *milon.Client, events []api.TypeTagWithData) error {
	fmt.Printf("\nEvents (len=%d):\n", len(events))
	for i, event := range events {
		fmt.Printf("\t [%d] TypeTag: %d \n", i, event.TypeTag)
		fmt.Printf("\t\t Value (hex): %x\n", event.Value)

		decodedEvent, err := client.GetProviderManager().DecodeEventDataByTag(event.TypeTag, event.Value)
		if err != nil {
			return fmt.Errorf("failed to decode event: %w", err)
		}

		fmt.Printf("\t\t decodedEvent: %+v\n", decodedEvent)
		fmt.Printf("\t\t FormatDecodedEvent: %+v \n\n", client.GetProviderManager().FormatDecodedEvent(decodedEvent))
	}
	return nil
}

func decodeInlineData(client *milon.Client, label string, typeTag uint64, data []byte) (*provider.Provider, *provider.IDLType, error) {
	pd, idlType := client.FindIDLTypeByTag(typeTag)
	if pd == nil || idlType == nil {
		fmt.Printf("\t\t\t Warning: unknown type_tag %d, skipping decode\n", typeTag)
		return nil, nil, nil
	}

	valueDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode %s InlineData: %w", label, err)
	}

	fmt.Printf("\t\t\t Value Decoded (%s): %+v\n", idlType.Name, valueDecoded)
	return pd, idlType, nil
}
