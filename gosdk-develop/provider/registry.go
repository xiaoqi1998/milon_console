package provider

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"strings"
)

type IDLRegistry struct {
	providerByAppID map[uint8]*Provider // app_id -> Provider
}

func NewIDLRegistry(providerByIDLName map[string]*Provider) (*IDLRegistry, error) {
	providerByAppID := make(map[uint8]*Provider)

	for _, pd := range providerByIDLName {
		appID := pd.appID()
		if _, ok := providerByAppID[appID]; ok {
			return nil, fmt.Errorf("duplicate app_id: %d", appID)
		}
		providerByAppID[appID] = pd
	}

	return &IDLRegistry{
		providerByAppID: providerByAppID,
	}, nil
}

// DecodeInstructions decodes multiple packed instructions in batch.
// Each element must be a wire-encoded instruction (app_id + discriminator + args).
//
//	decodedList, err := manager.DecodeInstructions(instructions)
//	if err != nil {
//	    log.Fatalf("failed to decode instructions: %v", err)
//	}
//	for i, decoded := range decodedList {
//	    fmt.Printf("[%d] %s::%s\n", i, decoded["app_name"], decoded["instruction_name"])
//	    fmt.Printf("    Args: %v\n", decoded["args"])
//	}
func (m *IDLRegistry) DecodeInstructions(instructions [][]byte) ([]map[string]any, error) {
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

func (m *IDLRegistry) DecodeInstruction(instruction []byte) (map[string]any, error) {
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
func (m *IDLRegistry) DecodeEventDataByTag(typeTag uint64, data []byte) (map[string]any, error) {
	// Find the provider that registered this event type_tag
	var matchedProvider *Provider
	var matchedEvent *Event

	for _, pd := range m.providerByAppID {
		if event, ok := pd.GetEventByTypeTag(typeTag); ok {
			matchedProvider = pd
			matchedEvent = event
			break
		}
	}

	if matchedProvider == nil || matchedEvent == nil {
		return nil, fmt.Errorf("unknown type tag: %d (loaded %d IDLs)", typeTag, len(m.providerByAppID))
	}

	// Decode all fields in order; skip a leading type_tag prefix when present.
	offset := 0
	savedOffset := offset
	if storedTypeTag, err := decodeViewVarUint(data, &offset); err == nil && storedTypeTag == typeTag {
		// Leading typeTag matched, skip it
	} else {
		// No matching typeTag prefix, decode fields from start
		offset = savedOffset
	}

	// Decode all fields in order
	record := make(map[string]any)
	for _, field := range matchedEvent.Fields {
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
		"app_id":     matchedProvider.IDL.Metadata.AppID,
		"app_name":   matchedProvider.IDL.Metadata.Name,
		"event_name": matchedEvent.Name,
		"data":       record,
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
//	    viewMultiTransactionResult.HTTPResponseBody,
//	)
func (m *IDLRegistry) DecodeViewDatas(appNameAndInstructionNames []string, body []byte) ([]DecodedTaggedValue, error) {
	offset := 0

	// 1. Read Vec length (result count)
	resultCount, err := decodeViewVarUint(body, &offset)
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

		// Find provider by app name
		var matchedProvider *Provider
		for _, pd := range m.providerByAppID {
			if pd.IDL.Metadata.Name == parts[0] {
				matchedProvider = pd
				break
			}
		}
		if matchedProvider == nil {
			return nil, fmt.Errorf("result[%d]: unknown app %q", i, parts[0])
		}

		// Get instruction by name
		instruction, err := matchedProvider.GetInstructionByName(parts[1])
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

		value, err := decodeViewResultItem(matchedProvider, returnType, body, &offset)
		if err != nil {
			return nil, fmt.Errorf("result[%d]: %w", i, err)
		}
		results[i] = value
	}

	// Verify no unparsed data remains
	if offset != len(body) {
		return nil, fmt.Errorf("%d trailing bytes after decoding %d view results", len(body)-offset, resultCount)
	}

	return results, nil
}

// FormatDecodedInstruction formats decoded instruction into readable string
func (m *IDLRegistry) FormatDecodedInstruction(decoded map[string]any) string {
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

// FormatDecodedEvent formats decoded event data into a readable string.
func (m *IDLRegistry) FormatDecodedEvent(decoded map[string]any) string {
	appName, _ := decoded["app_name"].(string)
	eventName, _ := decoded["event_name"].(string)
	data := decoded["data"]

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s\n", appName, eventName))
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

func (m *IDLRegistry) formatValue(value any) string {
	switch v := value.(type) {
	case crypto.Address:
		return fmt.Sprintf("Address(%s)", v.ToBase58())
	case *crypto.Address:
		if v == nil {
			return "Address(nil)"
		}
		return fmt.Sprintf("Address(%s)", v.ToBase58())
	case crypto.PublicKey:
		return fmt.Sprintf("PublicKey(%s)", v.ToBase58())
	case *crypto.PublicKey:
		if v == nil {
			return "PublicKey(nil)"
		}
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
	case []byte:
		return fmt.Sprintf("Bytes(%x)", v)
	case [12]byte:
		return fmt.Sprintf("B96(%x)", v[:])
	case [18]byte:
		return fmt.Sprintf("B144(%x)", v[:])
	case [20]byte:
		return fmt.Sprintf("B160(%x)", v[:])
	case [32]byte:
		return fmt.Sprintf("B256(%x)", v[:])
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
