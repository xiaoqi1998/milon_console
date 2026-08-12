package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func DisplayTxHistory(client *milon.Client, txHistory *api.TxHistory) {
	fmt.Printf("\n================ Display TxHistory ================\n")

	fmt.Printf("txHistory: %+v \n\n", txHistory)

	// Instructions
	fmt.Printf("\nInstructions (len=%d):\n", len(txHistory.Instructions))
	for i, instruction := range txHistory.Instructions {
		fmt.Printf("\t [%d] instruction: \n", i)

		decodedInstruction, err := client.GetProviderManager().DecodeInstruction(instruction)
		if err != nil {
			panic("failed to decode instruction: " + err.Error())
		}

		fmt.Printf("\t\t decodedInstruction = %#v \n", decodedInstruction)
		fmt.Printf("\t\t FormatDecodedInstruction = %+v \n\n", client.GetProviderManager().FormatDecodedInstruction(decodedInstruction))
	}

	// Access Records
	fmt.Printf("\nAccess Records (len=%d):\n", len(txHistory.Receipt.Access))
	for i, record := range txHistory.Receipt.Access {
		fmt.Printf("\t [%d] ResourceID: %v\n", i, record.ResourceID)

		// FirstSnapshot
		if record.FirstSnapshot != nil {
			fmt.Printf("\t\t FirstSnapshot: \n")

			switch record.FirstSnapshot.Variant {
			case 0: // Inline
				fmt.Printf("\t\t\t Inline(type_tag=%d)\n", record.FirstSnapshot.TypeTag)
				fmt.Printf("\t\t\t Data: %x\n", record.FirstSnapshot.InlineData)
				decodeInlineData(client, "FirstSnapshot", record.FirstSnapshot.TypeTag, record.FirstSnapshot.InlineData)
			case 1: // External
				fmt.Printf("\t\t\t External(BlobHash=%x)\n", record.FirstSnapshot.ExternalHash)
			default:
				panic(fmt.Sprintf("Unknown(variant=%d)", record.FirstSnapshot.Variant))
			}
		} else {
			fmt.Printf("\t\t FirstSnapshot: None\n")
		}

		// LastWritten
		fmt.Printf("\t\t LastWritten: \n")
		switch record.LastWritten.Variant {
		case 0: // Inline
			fmt.Printf("\t\t\t Inline(type_tag=%d, data_len=%d)\n", record.LastWritten.TypeTag, len(record.LastWritten.InlineData))
			fmt.Printf("\t\t\t Data: %x\n", record.LastWritten.InlineData)

			valueDecoded, pd, idlType := decodeInlineData(client, "LastWritten", record.LastWritten.TypeTag, record.LastWritten.InlineData)
			if valueDecoded == nil {
				break
			}

			getResourceResult, err := client.GetResource(record.ResourceID)
			if err != nil {
				fmt.Printf("\t\t\t client.GetResource error: %+v \n", err)
			} else {
				valueDecoded, err = pd.DecodeDataByIDLTypeName(idlType.Name, getResourceResult.BodyGetResource.Data.Value)
				if err != nil {
					panic("failed to decode LastWritten GetResource data: " + err.Error())
				}

				fmt.Printf("\t\t\t client.GetResource.BodyGetResource: %+v\n", getResourceResult.BodyGetResource)
				fmt.Printf("\t\t\t getResourceResult.BodyGetResource.Data.Value Decoded (%s): %+v\n\n", idlType.Name, valueDecoded)
			}
		case 1: // External
			fmt.Printf("\t\t\t External(BlobHash=%x)\n", record.LastWritten.ExternalHash)

			getAccessValueResult, err := client.RpcClient.GetAccessValue([]api.BlobHash{record.LastWritten.ExternalHash})
			if err != nil {
				panic("failed to get LastWritten access value: " + err.Error())
			}

			fmt.Printf("\t\t\t client.RpcClient.GetAccessValue: %+v \n\n", getAccessValueResult)
			for j, value := range getAccessValueResult.BodyGetAccessValues {
				fmt.Printf("\t\t\t\t [%d] BlobHash: %+v\n", j, value.BlobHash)
				fmt.Printf("\t\t\t\t [%d] Data: %+v\n", j, value.Data)

				if value.Data != nil {
					pd, idlType := findIDLTypeByTag(client, value.Data.TypeTag)
					if pd != nil && idlType != nil {
						decodedValue, err := pd.DecodeDataByIDLTypeName(idlType.Name, value.Data.Value)
						if err != nil {
							panic("failed to decode LastWritten access value data: " + err.Error())
						}

						fmt.Printf("\t\t\t\t [%d] Value Decoded (%s): %+v\n", j, idlType.Name, decodedValue)
					}
				}
			}
		default:
			panic(fmt.Sprintf("Unknown(variant=%d)", record.LastWritten.Variant))
		}
	}

	// Events
	fmt.Printf("\nEvents (len=%d):\n", len(txHistory.Receipt.Events))
	for i, event := range txHistory.Receipt.Events {
		fmt.Printf("\t [%d] TypeTag: %d \n", i, event.TypeTag)
		fmt.Printf("\t\t Value (hex): %x\n", event.Value)

		decodedEvent, err := client.GetProviderManager().DecodeEventDataByTag(event.TypeTag, event.Value)
		if err != nil {
			panic("failed to decode event: " + err.Error())
		}

		fmt.Printf("\t\t decodedEvent: %+v\n", decodedEvent)
		fmt.Printf("\t\t FormatDecodedEvent: %+v \n\n", client.GetProviderManager().FormatDecodedEvent(decodedEvent))
	}

	fmt.Printf("\n================ Display TxHistory ================\n")
}

func findIDLTypeByTag(client *milon.Client, typeTag uint64) (*provider.Provider, *provider.IDLType) {
	for _, pd := range client.GetAllPd() {
		if idlType, ok := pd.GetIDLTypeByTypeTag(typeTag); ok {
			return pd, idlType
		}
	}
	return nil, nil
}

func decodeInlineData(client *milon.Client, label string, typeTag uint64, data []byte) (any, *provider.Provider, *provider.IDLType) {
	pd, idlType := findIDLTypeByTag(client, typeTag)
	if pd == nil || idlType == nil {
		fmt.Printf("\t\t\t Warning: unknown type_tag %d, skipping decode\n", typeTag)
		return nil, nil, nil
	}

	valueDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, data)
	if err != nil {
		panic("failed to decode " + label + " InlineData: " + err.Error())
	}
	fmt.Printf("\t\t\t Value Decoded (%s): %+v\n", idlType.Name, valueDecoded)
	return valueDecoded, pd, idlType
}
