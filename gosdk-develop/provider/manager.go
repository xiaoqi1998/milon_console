package provider

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"strings"
)

type IDLManager struct {
	providerByAppID map[uint8]*Provider // app_id -> Provider
}

func NewIDLManager(providerByIDLName map[string]*Provider) (*IDLManager, error) {
	providerByAppID := make(map[uint8]*Provider)

	for _, idl := range providerByIDLName {
		appID := idl.appID()
		if _, ok := providerByAppID[appID]; ok {
			return nil, fmt.Errorf("duplicate app_id: %d", appID)
		}
		providerByAppID[appID] = idl
	}

	return &IDLManager{
		providerByAppID: providerByAppID,
	}, nil
}

// DecodeInstructions decodes multiple instructions in batch
//
//	manager := NewIDLManager()
//	manager.LoadAllIDLsFromDir("./provider/IDL")
//
//	// Assume transaction contains 3 instructions
//	InstructionByName := [][]byte{
//	    mintWire,      // token::Mint
//	    transferWire,  // token::Transfer
//	    burnWire,      // token::Burn
//	}
//
//	// Batch decode
//	decodedList, err := manager.DecodeInstructions(InstructionByName)
//	if err != nil {
//	    log.Fatalf("Failed at instruction[%d]: %v", /* index */, err)
//	}
//
//	// ViewMulti all instructions
//	for i, decoded := range decodedList {
//	    fmt.Printf("[%d] %s::%s\n", i, decoded["app_name"], decoded["instruction_name"])
//	    fmt.Printf("    Args: %v\n", decoded["args"])
//	}
//	// Output:
//	// [0] token::Mint
//	//     Args: map[token:addr to:addr amount:1000]
//	// [1] token::Transfer
//	//     Args: map[from:addr to:addr amount:500]
//	// [2] token::Burn
//	//     Args: map[token:addr holder:addr amount:200]
func (m *IDLManager) DecodeInstructions(instructions [][]byte) ([]map[string]any, error) {
	results := make([]map[string]any, len(instructions))
	for i, instr := range instructions {
		decoded, err := m.DecodeInstruction(instr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode instruction[%d]: %w", i, err)
		}
		results[i] = decoded
	}
	return results, nil
}

func (m *IDLManager) DecodeInstruction(instruction []byte) (map[string]any, error) {
	if len(instruction) < 3 {
		return nil, fmt.Errorf("empty instruction: need at least 3 bytes (app_id + discriminator)")
	}

	// app_id (1 byte)
	appID := instruction[0]
	offset := 1

	// discriminator (u16 LE little-endian encoding, 2 bytes)
	discriminatorLow := uint64(instruction[offset])
	discriminatorHigh := uint64(instruction[offset+1])
	discriminator := discriminatorLow | (discriminatorHigh << 8)
	offset += 2

	// Find provider by app_id
	provider, ok := m.providerByAppID[appID]
	if !ok {
		return nil, fmt.Errorf("unknown app_id: %d", appID)
	}

	// Use index to quickly find matching instruction
	matchedInstruction, ok := provider.InstructionByDiscriminator[uint16(discriminator)]
	if !ok {
		return nil, fmt.Errorf("unknown discriminator: %d (app: %s)", discriminator, provider.IDL.Metadata.Name)
	}

	// Decode arguments
	args := make(map[string]any, len(matchedInstruction.Args))
	for _, arg := range matchedInstruction.Args {
		value, err := provider.deserializeValue(arg.Type, instruction, &offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode argument '%s' (%s): %w", arg.Name, arg.Type, err)
		}
		args[arg.Name] = value
	}

	// Verify no unparsed data remains
	if offset != len(instruction) {
		return nil, fmt.Errorf("%d trailing bytes after decoding all arguments", len(instruction)-offset)
	}

	return map[string]any{
		"app_id":           provider.IDL.Metadata.AppID,
		"app_name":         provider.IDL.Metadata.Name,
		"instruction_name": matchedInstruction.Name,
		"discriminator":    discriminator,
		"args":             args,
	}, nil
}

// DecodeEventDataByTag decodes event data based on typeTag
//
//	// Get events from transaction history
//	txHistory, _ := client.GetTxByHash(txHash)
//	for _, event := range txHistory.Receipt.Events {
//	    decoded, err := manager.DecodeEventDataByTag(event.TypeTag, event.Data)
//	    if err != nil {
//	        log.Printf("Decode failed: %v", err)
//	        continue
//	    }
//	    fmt.Printf("Event: %s::%s\n", decoded["app_name"], decoded["type_name"])
//	    fmt.Printf("  Data: %+v\n", decoded["data"])
//	}
//	// Output:
//	// Event: demo::EventCreditApplied
//	//   Data: map[amount:42 pool:Address(2Zcp...) recipient:Address(9vYs...)]
func (m *IDLManager) DecodeEventDataByTag(typeTag uint64, data []byte) (map[string]any, error) {
	// Iterate through all providers to find matching typeTag
	var matchedProvider *Provider
	var matchedTypeName string
	var matchedEventFields []EventField

	for _, provider := range m.providerByAppID {
		for _, event := range provider.IDL.Events {
			if event.TypeTag == typeTag {
				matchedProvider = provider
				matchedTypeName = event.Name
				matchedEventFields = event.Fields
				break
			}
		}

		if matchedProvider != nil {
			break
		}
	}

	if matchedProvider == nil {
		return nil, fmt.Errorf("unknown type tag: %d (loaded %d IDLs)", typeTag, len(m.providerByAppID))
	}

	//Decode all fields directly from data (no typeTag prefix — it's already in the event struct)
	offset := 0
	savedOffset := offset
	if storedTypeTag, err := DecodeViewVarUint(data, &offset); err == nil && storedTypeTag == typeTag {
		// Leading typeTag matched, skip it
	} else {
		// No matching typeTag prefix, decode fields from start
		offset = savedOffset
	}

	// Decode all fields in order
	record := make(map[string]any)
	for _, field := range matchedEventFields {
		if offset >= len(data) {
			return nil, fmt.Errorf("insufficient data for field '%s' (%s)", field.Name, field.Type)
		}

		value, decodeErr := matchedProvider.deserializeValue(field.Type, data, &offset)
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode field '%s' (%s): %w", field.Name, field.Type, decodeErr)
		}
		record[field.Name] = value
	}

	// Verify no unparsed data remains
	if offset != len(data) {
		return nil, fmt.Errorf("%d trailing bytes after decoding event data", len(data)-offset)
	}

	return map[string]any{
		"type_name": matchedTypeName,
		"app_name":  matchedProvider.IDL.Metadata.Name,
		"type_tag":  typeTag,
		"data":      record,
	}, nil
}

// DecodeViewDatas decodes view response body where each result corresponds
// to a different instruction method. instructionNames format: "appName::methodName"
//
//	results, err := manager.DecodeViewDatas(
//	    []string{
//	        "token::BalanceOf",
//	        "token::Metadata",
//	        "token::BalanceOf",
//	        "token::TotalSupply",
//	    },
//	    viewMultiTransactionResult.HttpRspBody,
//	)
func (m *IDLManager) DecodeViewDatas(appNameAndInstructionNames []string, body []byte) ([]DecodedTaggedValue, error) {
	offset := 0

	// 1. Read Vec length (result count)
	resultCount, err := DecodeViewVarUint(body, &offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode result count: %w", err)
	}

	if int(resultCount) != len(appNameAndInstructionNames) {
		return nil, fmt.Errorf("result count %d does not match instruction count %d", resultCount, len(appNameAndInstructionNames))
	}

	results := make([]DecodedTaggedValue, resultCount)

	// 2. Decode each Result item using the corresponding instruction's return type
	for i := uint64(0); i < resultCount; i++ {
		// Parse "appName::instructionName"
		parts := strings.Split(appNameAndInstructionNames[i], "::")
		if len(parts) != 2 {
			return nil, fmt.Errorf("result[%d]: invalid format %q (expected appName::methodName)", i, appNameAndInstructionNames[i])
		}
		appName := parts[0]
		instrName := parts[1]

		// Find provider by app name
		var matchedProvider *Provider
		for _, provider := range m.providerByAppID {
			if provider.IDL.Metadata.Name == appName {
				matchedProvider = provider
				break
			}
		}
		if matchedProvider == nil {
			return nil, fmt.Errorf("result[%d]: unknown app %q", i, appName)
		}

		// Get instruction by name
		instruction, err := matchedProvider.GetInstructionByName(instrName)
		if err != nil {
			return nil, fmt.Errorf("result[%d]: %w", i, err)
		}

		// Verify instruction type, must be view type
		if instruction.Kind != "view" {
			return nil, fmt.Errorf("result[%d]: %s kind=%s, expected view", i, appNameAndInstructionNames[i], instruction.Kind)
		}

		// Get return value type definition
		returnType := strings.TrimSpace(instruction.Returns.Type)
		if returnType == "" {
			return nil, fmt.Errorf("result[%d]: %s has no returns.type in IDL", i, appNameAndInstructionNames[i])
		}

		// Read Result variant index: 0 = Ok, 1 = Err
		variantIndex, err := DecodeViewVarUint(body, &offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode result[%d] variant: %w", i, err)
		}

		if variantIndex == 0 {
			// Ok branch: contains actual return value bytes
			okDataLen, err := DecodeViewVarUint(body, &offset)
			if err != nil {
				return nil, fmt.Errorf("failed to decode result[%d] Ok data length: %w", i, err)
			}

			if offset+int(okDataLen) > len(body) {
				return nil, fmt.Errorf("insufficient data for result[%d] Ok payload", i)
			}
			okData := make([]byte, okDataLen)
			copy(okData, body[offset:offset+int(okDataLen)])
			offset += int(okDataLen)

			// Deserialize actual return value using the matched provider
			valueOffset := 0
			value, err := matchedProvider.deserializeValue(returnType, okData, &valueOffset)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize result[%d] %s Ok value: %w", i, appNameAndInstructionNames[i], err)
			}

			results[i] = DecodedTaggedValue{Value: value}
		} else if variantIndex == 1 {
			// Err branch: contains TxFailurePayload
			failure, err := matchedProvider.decodeTxFailurePayload(body, &offset)
			if err != nil {
				return nil, fmt.Errorf("failed to decode result[%d] %s Err payload: %w", i, appNameAndInstructionNames[i], err)
			}
			results[i] = DecodedTaggedValue{Value: failure}
		} else {
			return nil, fmt.Errorf("result[%d]: invalid variant index %d", i, variantIndex)
		}
	}

	// Verify no unparsed data remains
	if offset != len(body) {
		return nil, fmt.Errorf("%d trailing bytes after decoding %d view results", len(body)-offset, resultCount)
	}

	return results, nil
}

// FormatDecodedInstruction formats decoded instruction into readable string
func (m *IDLManager) FormatDecodedInstruction(decoded map[string]any) string {
	appId, _ := decoded["app_id"].(uint8)
	appName, _ := decoded["app_name"].(string)
	instructionName, _ := decoded["instruction_name"].(string)
	discriminator, _ := decoded["discriminator"].(uint64)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s\n", appName, instructionName))
	sb.WriteString("Struct {\n")
	sb.WriteString(fmt.Sprintf("    appId: %d,\n", appId))
	sb.WriteString(fmt.Sprintf("    appName: \"%s\",\n", appName))
	sb.WriteString(fmt.Sprintf("    instructionName: \"%s\",\n", instructionName))
	sb.WriteString(fmt.Sprintf("    discriminator: %d,\n", discriminator))
	sb.WriteString("    fields: [\n")

	args, _ := decoded["args"].(map[string]any)
	first := true
	for name, value := range args {
		if !first {
			sb.WriteString(",\n")
		}
		first = false

		sb.WriteString(fmt.Sprintf("        NamedToken {\n"))
		sb.WriteString(fmt.Sprintf("            name: \"%s\",\n", name))
		sb.WriteString(fmt.Sprintf("            value: %s,\n", m.formatValue(value)))
		sb.WriteString("        }")
	}

	sb.WriteString("\n    ],\n")
	sb.WriteString("}")

	return sb.String()
}

// FormatDecodedEvent formats event data into readable string  todo----后续可能不要了，结构体变了
func (m *IDLManager) FormatDecodedEvent(decoded map[string]any) string {
	appName, _ := decoded["app_name"].(string)
	typeName, _ := decoded["type_name"].(string)
	data := decoded["data"]

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s\n", appName, typeName))
	sb.WriteString("Struct {\n")

	switch v := data.(type) {
	case map[string]any:
		first := true
		for k, val := range v {
			if !first {
				sb.WriteString(",\n")
			}
			first = false
			sb.WriteString(fmt.Sprintf("    %s: %s", k, m.formatValue(val)))
		}
	default:
		sb.WriteString(fmt.Sprintf("    value: %s", m.formatValue(v)))
	}

	sb.WriteString("\n}")
	return sb.String()
}

func (m *IDLManager) formatValue(value any) string {
	switch v := value.(type) {
	case crypto.Address:
		return fmt.Sprintf("Address(%s)", v.ToBase58())
	case crypto.PublicKey:
		return fmt.Sprintf("PublicKey(%s)", v.ToBase58())
	case string:
		return fmt.Sprintf("String(\"%s\")", v)
	case uint8:
		return fmt.Sprintf("U8(%d)", v)
	case uint16:
		return fmt.Sprintf("U16(%d)", v)
	case uint32:
		return fmt.Sprintf("U32(%d)", v)
	case uint64:
		return fmt.Sprintf("U64(%d)", v)
	case int8:
		return fmt.Sprintf("I8(%d)", v)
	case int16:
		return fmt.Sprintf("I16(%d)", v)
	case int32:
		return fmt.Sprintf("I32(%d)", v)
	case int64:
		return fmt.Sprintf("I64(%d)", v)
	case bool:
		return fmt.Sprintf("Bool(%v)", v)
	case []any:
		items := make([]string, len(v))
		for i, item := range v {
			items[i] = m.formatValue(item)
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	case map[string]any:
		var sb strings.Builder
		sb.WriteString("Struct {\n")
		first := true
		for k, val := range v {
			if !first {
				sb.WriteString(",\n")
			}
			first = false
			sb.WriteString(fmt.Sprintf("                %s: %s", k, m.formatValue(val)))
		}
		sb.WriteString("\n            }")
		return sb.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
