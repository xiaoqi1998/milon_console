package provider

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"sort"
	"strings"
)

type IDLRegistry struct {
	providerByAppID           map[uint8]*Provider  // app_id -> Provider
	providerByName            map[string]*Provider // app name -> Provider (O(1) lookup for DecodeViewDatas)
	providerByEventTypeTag    map[uint64]*Provider // event typeTag -> Provider (global index)
	providerByResourceTypeTag map[uint64]*Provider // resource typeTag -> Provider (global index, first wins)
	providerByTypeTag         map[uint64]*Provider // IDL type typeTag -> Provider (global index, first wins)
}

// typeTagIndexes holds the three global typeTag -> Provider maps. Both
// IDLRegistry and IDLTypeResolver need them, so they are built from a single
// shared routine to guarantee the two resolve any tag to the same Provider.
type typeTagIndexes struct {
	byResource map[uint64]*Provider
	byType     map[uint64]*Provider
	byEvent    map[uint64]*Provider
}

// buildTypeTagIndexes iterates providers in sorted name order and fills the
// three typeTag indexes. Sorting keeps the first-wins result deterministic
// regardless of map iteration order.
//
// resource and type typeTags legitimately repeat across IDLs (builtins such as
// Address or u64 mean the same thing everywhere), so the first provider wins.
// Event typeTags must be globally unique for decoding to stay unambiguous:
// when strictEvent is true a duplicate returns an error so the registry fails
// fast at load time, while the type resolver passes false to stay tolerant.
func buildTypeTagIndexes(providers map[string]*Provider, strictEvent bool) (typeTagIndexes, error) {
	indexes := typeTagIndexes{
		byResource: make(map[uint64]*Provider),
		byType:     make(map[uint64]*Provider),
		byEvent:    make(map[uint64]*Provider),
	}

	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		pd := providers[name]

		for typeTag := range pd.ResourceByTypeTag {
			if _, ok := indexes.byResource[typeTag]; !ok {
				indexes.byResource[typeTag] = pd
			}
		}

		for typeTag := range pd.IDLTypeByTypeTag {
			if _, ok := indexes.byType[typeTag]; !ok {
				indexes.byType[typeTag] = pd
			}
		}

		for typeTag := range pd.EventByTypeTag {
			if _, ok := indexes.byEvent[typeTag]; ok {
				if strictEvent {
					return typeTagIndexes{}, fmt.Errorf("duplicate event type_tag %d across loaded IDLs", typeTag)
				}
				continue
			}
			indexes.byEvent[typeTag] = pd
		}
	}

	return indexes, nil
}

func NewIDLRegistry(providerByIDLName map[string]*Provider) (*IDLRegistry, error) {
	providerByAppID := make(map[uint8]*Provider, len(providerByIDLName))
	providerByName := make(map[string]*Provider, len(providerByIDLName))

	// Sort provider names so duplicate app_id / app name errors are reported
	// deterministically regardless of map iteration order.
	names := make([]string, 0, len(providerByIDLName))
	for name := range providerByIDLName {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, idlName := range names {
		pd := providerByIDLName[idlName]
		appID := pd.appID()
		if _, ok := providerByAppID[appID]; ok {
			return nil, fmt.Errorf("duplicate app_id: %d", appID)
		}
		providerByAppID[appID] = pd

		name := pd.IDL.Metadata.Name
		if name != "" {
			if _, ok := providerByName[name]; ok {
				return nil, fmt.Errorf("duplicate app name: %s", name)
			}
			providerByName[name] = pd
		}
	}

	// The three typeTag indexes are shared with IDLTypeResolver; event tags must
	// be unique, so strictEvent fails construction instead of leaving decoding
	// ambiguous later.
	indexes, err := buildTypeTagIndexes(providerByIDLName, true)
	if err != nil {
		return nil, err
	}

	return &IDLRegistry{
		providerByAppID:           providerByAppID,
		providerByName:            providerByName,
		providerByEventTypeTag:    indexes.byEvent,
		providerByResourceTypeTag: indexes.byResource,
		providerByTypeTag:         indexes.byType,
	}, nil
}

// DecodeInstructions decodes multiple packed instructions in batch; each
// element must be a wire-encoded instruction (app_id + discriminator + args).
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

	// discriminator (u16 LE, 2 bytes)
	discriminatorLow := uint64(instruction[offset])
	discriminatorHigh := uint64(instruction[offset+1])
	discriminator := discriminatorLow | (discriminatorHigh << 8)
	offset += 2

	provider, ok := m.providerByAppID[appID]
	if !ok {
		return nil, fmt.Errorf("unknown app_id: %d", appID)
	}

	matchedInstruction, ok := provider.InstructionByDiscriminator[uint16(discriminator)]
	if !ok {
		return nil, fmt.Errorf("unknown discriminator: %d (app: %s)", discriminator, provider.IDL.Metadata.Name)
	}

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

// DecodeViewDatas decodes a view response body where each Result corresponds
// to a different method; instructionNames use "appName::methodName" format.
func (m *IDLRegistry) DecodeViewDatas(appNameAndInstructionNames []string, body []byte) ([]DecodedTaggedValue, error) {
	offset := 0

	// 1. vec length (result count)
	resultCount, err := decodeViewVarUint(body, &offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode result count: %w", err)
	}

	if int(resultCount) != len(appNameAndInstructionNames) {
		return nil, fmt.Errorf("result count %d does not match instruction count %d", resultCount, len(appNameAndInstructionNames))
	}

	results := make([]DecodedTaggedValue, resultCount)

	// 2. each Result item, decoded by the corresponding instruction's return type
	for i := uint64(0); i < resultCount; i++ {
		parts := strings.Split(appNameAndInstructionNames[i], "::")
		if len(parts) != 2 {
			return nil, fmt.Errorf("result[%d]: invalid format %q (expected appName::methodName)", i, appNameAndInstructionNames[i])
		}

		matchedProvider := m.providerByName[parts[0]]
		if matchedProvider == nil {
			return nil, fmt.Errorf("result[%d]: unknown app %q", i, parts[0])
		}

		instruction, err := matchedProvider.GetInstructionByName(parts[1])
		if err != nil {
			return nil, fmt.Errorf("result[%d]: %w", i, err)
		}

		if instruction.Kind != "view" {
			return nil, fmt.Errorf("result[%d]: %s kind=%s, expected view", i, appNameAndInstructionNames[i], instruction.Kind)
		}

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

	if offset != len(body) {
		return nil, fmt.Errorf("%d trailing bytes after decoding %d view results", len(body)-offset, resultCount)
	}

	return results, nil
}

// DecodeViewValue decodes the response body of the single-instruction view
// RPC (chain method 15); appNameAndInstruction uses the "appName::methodName"
// format.
func (m *IDLRegistry) DecodeViewValue(appNameAndInstruction string, body []byte) (any, error) {
	parts := strings.Split(appNameAndInstruction, "::")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid format %q (expected appName::methodName)", appNameAndInstruction)
	}

	matchedProvider := m.providerByName[parts[0]]
	if matchedProvider == nil {
		return nil, fmt.Errorf("unknown app %q", parts[0])
	}

	return matchedProvider.DecodeViewValue(parts[1], body)
}

// DecodeEventDataByTag decodes event data by its typeTag.
func (m *IDLRegistry) DecodeEventDataByTag(typeTag uint64, data []byte) (map[string]any, error) {
	matchedProvider := m.providerByEventTypeTag[typeTag]
	if matchedProvider == nil {
		return nil, fmt.Errorf("unknown type tag: %d (loaded %d IDLs)", typeTag, len(m.providerByAppID))
	}
	// Indexed from this provider's own EventByTypeTag, so the lookup cannot
	// miss; ok is intentionally ignored.
	matchedEvent, _ := matchedProvider.GetEventByTypeTag(typeTag)

	// Skip a leading type_tag varint prefix when present.
	offset := 0
	if storedTypeTag, err := decodeViewVarUint(data, &offset); err == nil && storedTypeTag == typeTag {
		// matched, keep offset
	} else {
		offset = 0
	}

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

// DecodeResourceDataByTag decodes a persisted value by its typeTag. Resource
// declarations win; when the tag is not declared as a resource the IDL types
// section is used as fallback (values persisted under their raw value-type
// tag, e.g. an Address written by the genesis transaction).
func (m *IDLRegistry) DecodeResourceDataByTag(typeTag uint64, data []byte) (map[string]any, error) {
	if matchedProvider := m.providerByResourceTypeTag[typeTag]; matchedProvider != nil {
		// Indexed from this provider's own ResourceByTypeTag, so the lookup
		// cannot miss; ok is intentionally ignored.
		matchedResource, _ := matchedProvider.GetResourceByTypeTag(typeTag)

		offset := 0
		value, err := matchedProvider.deserializeValue(matchedResource.Type, data, &offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode resource '%s' (%s): %w", matchedResource.Name, matchedResource.Type, err)
		}

		// Verify no unparsed data remains
		if offset != len(data) {
			return nil, fmt.Errorf("%d trailing bytes after decoding resource data", len(data)-offset)
		}

		return map[string]any{
			"app_id":        matchedProvider.IDL.Metadata.AppID,
			"app_name":      matchedProvider.IDL.Metadata.Name,
			"resource_name": matchedResource.Name,
			"resource_type": matchedResource.Type,
			"data":          value,
		}, nil
	}

	if matchedProvider := m.providerByTypeTag[typeTag]; matchedProvider != nil {
		// Indexed from this provider's own IDLTypeByTypeTag, so the lookup
		// cannot miss; ok is intentionally ignored.
		matchedType, _ := matchedProvider.GetIDLTypeByTypeTag(typeTag)

		offset := 0
		value, err := matchedProvider.deserializeValue(matchedType.Name, data, &offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode type '%s': %w", matchedType.Name, err)
		}

		// Verify no unparsed data remains
		if offset != len(data) {
			return nil, fmt.Errorf("%d trailing bytes after decoding type data", len(data)-offset)
		}

		return map[string]any{
			"app_id":        matchedProvider.IDL.Metadata.AppID,
			"app_name":      matchedProvider.IDL.Metadata.Name,
			"resource_name": matchedType.Name,
			"resource_type": matchedType.Name,
			"data":          value,
		}, nil
	}

	return nil, fmt.Errorf("unknown resource type tag: %d (loaded %d IDLs)", typeTag, len(m.providerByAppID))
}

// FormatDecodedInstruction formats decoded instruction into readable string.
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
	// Sort keys so the formatted output is deterministic
	argNames := make([]string, 0, len(args))
	for name := range args {
		argNames = append(argNames, name)
	}
	sort.Strings(argNames)

	first := true
	for _, name := range argNames {
		if !first {
			sb.WriteString(",\n")
		}
		first = false

		sb.WriteString(fmt.Sprintf("        NamedToken {\n"))
		sb.WriteString(fmt.Sprintf("            name: \"%s\",\n", name))
		sb.WriteString(fmt.Sprintf("            value: %s,\n", m.formatValue(args[name])))
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
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		first := true
		for _, k := range keys {
			if !first {
				sb.WriteString(",\n")
			}
			first = false
			sb.WriteString(fmt.Sprintf("    %s: %s", k, m.formatValue(v[k])))
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
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		first := true
		for _, k := range keys {
			if !first {
				sb.WriteString(",\n")
			}
			first = false
			sb.WriteString(fmt.Sprintf("                %s: %s", k, m.formatValue(v[k])))
		}
		sb.WriteString("\n            }")
		return sb.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
