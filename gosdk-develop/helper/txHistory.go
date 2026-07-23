package helper

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/provider"
)

func DisplayTxHistoryAndGetResourceAndGetAccessValue(client *milon.MolinClient, txHistory *api.TxHistory) {
	fmt.Printf("\n================ Display TxHistory ================\n")

	fmt.Printf("txHistory: %+v \n\n", txHistory)

	// Use IDLManager to auto-detect and decode instructions
	idlManager, err := provider.NewIDLManager(client.RpcClient.GetAllPd())
	if err != nil {
		panic(err)
	}

	// Instructions
	fmt.Printf("\nInstructions (len=%d):\n", len(txHistory.Instructions))
	for i, instruction := range txHistory.Instructions {
		fmt.Printf("\t [%d] instruction: \n", i)

		decodedInstruction, err := idlManager.DecodeInstruction(instruction)
		if err != nil {
			panic(err)
		}

		fmt.Printf("\t\t decodedInstruction = %#v \n", decodedInstruction)
		fmt.Printf("\t\t FormatDecodedInstruction = %+v \n\n", idlManager.FormatDecodedInstruction(decodedInstruction))
	}

	// Access Records
	fmt.Printf("\nAccess Records (len=%d):\n", len(txHistory.Receipt.Access))
	for i, record := range txHistory.Receipt.Access {
		fmt.Printf("\t [%d] ResourceID: %x\n", i, record.ResourceID)

		// FirstSnapshot
		if record.FirstSnapshot != nil {
			fmt.Printf("\t\t FirstSnapshot: \n")

			switch record.FirstSnapshot.Variant {
			case 0: // Inline
				fmt.Printf("\t\t\t Inline(type_tag=%d, data_len=%d)\n", record.FirstSnapshot.TypeTag, len(record.FirstSnapshot.InlineData))
				fmt.Printf("\t\t\t Data: %v\n", record.FirstSnapshot.InlineData)

				var idlType *provider.IDLType
				var pd *provider.Provider

				for _, tmpProvider := range client.GetAllPd() {
					tmpIDLType, ok := tmpProvider.GetIDLTypeByTypeTag(record.FirstSnapshot.TypeTag)
					if ok {
						idlType = tmpIDLType
						pd = tmpProvider
						break
					}
				}

				if pd == nil || idlType == nil {
					fmt.Printf("\t\t\t Warning: unknown type_tag %d, skipping decode\n", record.FirstSnapshot.TypeTag)
				} else {
					valueDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, record.FirstSnapshot.InlineData)
					if err != nil {
						panic(err)
					}
					fmt.Printf("\t\t\t Value Decoded (%s): %+v\n", idlType.Name, valueDecoded)

					getResourceResult, err := client.GetResource(record.ResourceID, 1)
					if err != nil {
						fmt.Printf("\t\t\t client.GetResource error: %+v \n", err)
					} else {
						valueDecoded, err = pd.DecodeDataByIDLTypeName(idlType.Name, getResourceResult.BodyGetResource.Data.Value)
						if err != nil {
							panic(err)
						}
						fmt.Printf("\t\t\t client.GetResource.BodyGetResource: %+v\n", getResourceResult.BodyGetResource)
						fmt.Printf("\t\t\t client.GetResource.BodyGetResource.Data.Value Decoded (%v): %v\n\n", idlType.Name, valueDecoded)
					}
				}

			case 1: // External
				fmt.Printf("\t\t\t External(BlobHash=%x)\n", record.FirstSnapshot.ExternalHash)

				getAccessValueResult, err := client.RpcClient.GetAccessValue([]api.BlobHash{record.FirstSnapshot.ExternalHash}, 1)
				if err != nil {
					panic(err)
				}

				fmt.Printf("\t\t\t client.rpcClientV1.GetAccessValue: %+v \n\n", getAccessValueResult)
				for i2, value := range getAccessValueResult.BodyGetAccessValues {
					fmt.Printf("\t\t\t\t [%d] BlobHash: %+v\n", i2, value.BlobHash)
					fmt.Printf("\t\t\t\t [%d] Data: %+v\n", i2, value.Data)

					if value.Data != nil {
						for _, pd := range client.GetAllPd() {
							idlType, ok := pd.GetIDLTypeByTypeTag(value.Data.TypeTag)
							if ok {
								decodedValue, err := pd.DecodeDataByIDLTypeName(idlType.Name, value.Data.Value)
								if err != nil {
									panic(err)
								}

								fmt.Printf("\t\t\t\t [%d] Value Decoded (%s): %+v\n", i2, idlType.Name, decodedValue)
								break
							}
						}
					}
				}
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
			fmt.Printf("\t\t\t Data: %v\n", record.LastWritten.InlineData)

			var idlType *provider.IDLType
			var pd *provider.Provider

			for _, tmpProvider := range client.GetAllPd() {
				tmpIDLType, ok := tmpProvider.GetIDLTypeByTypeTag(record.LastWritten.TypeTag)
				if ok {
					idlType = tmpIDLType
					pd = tmpProvider
					break
				}
			}

			if pd == nil || idlType == nil {
				fmt.Printf("\t\t\t Warning: unknown type_tag %d, skipping decode\n", record.LastWritten.TypeTag)
			} else {
				valueDecoded, err := pd.DecodeDataByIDLTypeName(idlType.Name, record.LastWritten.InlineData)
				if err != nil {
					panic(err)
				}
				fmt.Printf("\t\t\t Value Decoded (%v): %v\n\n", idlType.Name, valueDecoded)

				getResourceResult, err := client.GetResource(record.ResourceID, 1)
				if err != nil {
					fmt.Printf("\t\t\t client.GetResource error: %+v \n", err)
				} else {
					valueDecoded, err = pd.DecodeDataByIDLTypeName(idlType.Name, getResourceResult.BodyGetResource.Data.Value)
					if err != nil {
						panic(err)
					}
					fmt.Printf("\t\t\t client.GetResource.BodyGetResource: %+v\n", getResourceResult.BodyGetResource)
					fmt.Printf("\t\t\t getResourceResult.BodyGetResource.Data.Value Decoded (%v): %v\n\n", idlType.Name, valueDecoded)
				}
			}
		case 1: // External
			fmt.Printf("\t\t\t External(BlobHash=%v)\n", record.LastWritten.ExternalHash)

			getAccessValueResult, err := client.RpcClient.GetAccessValue([]api.BlobHash{record.LastWritten.ExternalHash}, 1)
			if err != nil {
				panic(err)
			}

			fmt.Printf("\t\t\t client.rpcClientV1.GetAccessValue: %+v \n\n", getAccessValueResult)
			for i2, value := range getAccessValueResult.BodyGetAccessValues {
				fmt.Printf("\t\t\t\t [%d] BlobHash: %+v\n", i2, value.BlobHash)
				fmt.Printf("\t\t\t\t [%d] Data: %+v\n", i2, value.Data)

				if value.Data != nil {
					for _, pd := range client.GetAllPd() {
						idlType, ok := pd.GetIDLTypeByTypeTag(value.Data.TypeTag)
						if ok {
							decodedValue, err := pd.DecodeDataByIDLTypeName(idlType.Name, value.Data.Value)
							if err != nil {
								panic(err)
							}

							fmt.Printf("\t\t\t\t [%d] Value Decoded (%s): %+v\n", i2, idlType.Name, decodedValue)
							break
						}
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
		fmt.Printf("\t\t ValueLen: %d bytes\n", len(event.Value))
		fmt.Printf("\t\t Value: %x\n", event.Value)

		decodedEvent, err := idlManager.DecodeEventDataByTag(event.TypeTag, event.Value)
		if err != nil {
			panic(err)
		}

		fmt.Printf("\t\t decodedEvent: %+v\n", decodedEvent)
		fmt.Printf("\t\t FormatDecodedEvent: %+v \n\n", idlManager.FormatDecodedEvent(decodedEvent))
	}

	fmt.Printf("\n================ Display TxHistory ================\n")
}
