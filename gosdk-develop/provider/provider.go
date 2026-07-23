package provider

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcutil/base58"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"math"
	"math/big"
	"os"
	"reflect"
	"strings"
)

type Provider struct {
	IDL                        IDL
	InstructionByName          map[string]*Instruction //key=Instruction.Name
	InstructionByDiscriminator map[uint16]*Instruction //key=Instruction.Discriminator
	InstructionNames           []string
	IDLTypeByName              map[string]IDLType //key=IDLType.Name
	IDLTypeByTypeTag           map[uint64]IDLType //key=IDLType.typeTag
	EventByTypeTag             map[uint64]Event   //key=Event.typeTag
}

func NewProvider(idl IDL) *Provider {
	instructionByName := make(map[string]*Instruction, len(idl.Instructions))
	instructionByDiscriminator := make(map[uint16]*Instruction, len(idl.Instructions))
	methodNames := make([]string, 0, len(idl.Instructions))

	for i := range idl.Instructions {
		instruction := &idl.Instructions[i]
		instructionByName[instruction.Name] = instruction
		instructionByDiscriminator[instruction.Discriminator] = instruction
		methodNames = append(methodNames, instruction.Name)
	}

	idlTypeByName := make(map[string]IDLType, len(idl.Types))
	idlTypeByTypeTag := make(map[uint64]IDLType, len(idl.Types))
	for _, value := range idl.Types {
		idlTypeByName[value.Name] = value
		idlTypeByTypeTag[value.TypeTag] = value
	}

	eventByTypeTag := make(map[uint64]Event, len(idl.Events))
	for _, event := range idl.Events {
		// 将 Event 转换为 IDLType，方便统一处理
		idlType := IDLType{
			Name:    event.Name,
			TypeTag: event.TypeTag,
			Kind:    "struct", // Event 本质上是 struct
			Fields:  make([]StructField, len(event.Fields)),
		}
		for i, field := range event.Fields {
			idlType.Fields[i] = StructField{
				Name: field.Name,
				Type: field.Type,
			}
		}

		idlTypeByName[idlType.Name] = idlType
		idlTypeByTypeTag[idlType.TypeTag] = idlType

		// 同时注册到 EventByTypeTag
		eventByTypeTag[event.TypeTag] = event
	}

	return &Provider{
		IDL:                        idl,
		InstructionByName:          instructionByName,
		InstructionByDiscriminator: instructionByDiscriminator,
		InstructionNames:           methodNames,
		IDLTypeByName:              idlTypeByName,
		IDLTypeByTypeTag:           idlTypeByTypeTag,
		EventByTypeTag:             eventByTypeTag,
	}
}

func LoadProviderFromFile(path string) (*Provider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idl IDL
	if err = json.Unmarshal(data, &idl); err != nil {
		return nil, err
	}
	return NewProvider(idl), nil
}

func (p *Provider) appID() uint8 {
	return p.IDL.Metadata.AppID
}

func (p *Provider) GetInstructionByName(name string) (*Instruction, error) {
	instruction, ok := p.InstructionByName[name]
	if !ok {
		return nil, fmt.Errorf("IDL method not found: %s", name)
	}
	return instruction, nil
}

func (p *Provider) GetIDLTypeByTypeTag(typeTag uint64) (*IDLType, bool) {
	idlType, ok := p.IDLTypeByTypeTag[typeTag]
	if !ok {
		return nil, false
	}
	return &idlType, true
}

func (p *Provider) GetIDLTypeByName(name string) (*IDLType, bool) {
	idlType, ok := p.IDLTypeByName[name]
	if !ok {
		return nil, false
	}
	return &idlType, true
}

func (p *Provider) GetEventByTypeTag(typeTag uint64) (*Event, bool) {
	event, ok := p.EventByTypeTag[typeTag]
	if !ok {
		return nil, false
	}
	return &event, true
}

// Encode encodes instruction into bytes for on-chain submission
func (p *Provider) Encode(instructionName string, args Args) ([]byte, error) {
	instruction, err := p.GetInstructionByName(instructionName)
	if err != nil {
		return nil, err
	}

	if instruction.Kind != "entry" && instruction.Kind != "view" {
		return nil, fmt.Errorf("unsupported instruction kind: %s (expected 'entry' or 'view')", instruction.Kind)
	}

	return p.encodeInstruction(instruction, args)
}

func (p *Provider) encodeInstruction(instruction *Instruction, args Args) ([]byte, error) {
	serializer := postcard.NewSerializer()
	// 1. Write app_id (1 byte)
	if err := serializer.SerializeU8(p.appID()); err != nil {
		return nil, err
	}
	//fmt.Printf("serializer bytes appID = %v \n", serializer.Bytes())

	// 2. Write discriminator (u16 LE little-endian encoding, 2 bytes)
	serializer.SerializeFixedBytes([]byte{byte(instruction.Discriminator), byte(instruction.Discriminator >> 8)})
	//fmt.Printf("serializer bytes Discriminator = %v \n", serializer.Bytes())

	// 3. Serialize all arguments in the order defined by IDL
	for _, arg := range instruction.Args {
		value, ok := args[arg.Name]
		if !ok {
			return nil, fmt.Errorf("missing IDL argument: %s", arg.Name)
		}

		if err := p.serializeValue(serializer, strings.TrimSpace(arg.Type), value); err != nil {
			return nil, err
		}
	}

	return serializer.Bytes(), nil
}

// serializeValue serializes values into postcard format
func (p *Provider) serializeValue(serializer *postcard.Serializer, argName string, value any) error {
	// Handle vec<T>
	// Format: [length(varint)] + [item1] + [item2] + ... + [itemN]
	if inner, ok := parseWrappedType(argName, "vec"); ok {
		items, err := sliceValues(value) // Convert Go slice to []any
		if err != nil {
			return fmt.Errorf("%s expects an array", argName)
		}

		// Write array length (varint encoding)
		if err = serializer.SerializeU32(uint32(len(items))); err != nil {
			return err
		}

		// Recursively serialize each element
		for _, item := range items {
			if err = p.serializeValue(serializer, inner, item); err != nil {
				return err
			}
		}

		return nil
	}

	// Handle option<T>
	// Format: [has_value(u8: 0 or 1)] + [value(if has_value != 0)]
	if inner, ok := parseWrappedType(argName, "option"); ok {
		// Check if nil
		if value == nil || isNilValue(value) {
			return serializer.SerializeBool(false) // has_value = false
		}

		// Has value case
		if err := serializer.SerializeBool(true); err != nil { // has_value = true
			return err
		}

		// Recursively serialize actual value
		return p.serializeValue(serializer, inner, value)
	}

	// Handle map<K,V>
	// Format: [length(varint)] + [key1 + value1] + [key2 + value2] + ... + [keyN + valueN]
	if keyType, valueType, ok, err := parseMapType(argName); err != nil {
		return err
	} else if ok {
		var entries [][2]any
		entries, err = mapEntries(value) // Convert Go map to [][]any
		if err != nil {
			return fmt.Errorf("map expects a map or entry array")
		}

		// Write key-value pair count (varint encoding)
		if err = serializer.SerializeU32(uint32(len(entries))); err != nil {
			return err
		}

		// Serialize each key-value pair in sequence
		for _, entry := range entries {
			if err = p.serializeValue(serializer, keyType, entry[0]); err != nil {
				return err
			}
			if err = p.serializeValue(serializer, valueType, entry[1]); err != nil {
				return err
			}
		}
		return nil
	}

	// Handle tuple<T1,T2,...>
	// Format: [item1] + [item2] + ... + [itemN]
	if tupleTypes, ok, err := parseTupleType(argName); err != nil {
		return err
	} else if ok {
		var tuple []any
		tuple, err = tupleValues(value, len(tupleTypes)) // Convert Go slice to fixed-length array
		if err != nil {
			return err
		}

		// Serialize each element in order
		for i, itemType := range tupleTypes {
			if err = p.serializeValue(serializer, itemType, tuple[i]); err != nil {
				return err
			}
		}
		return nil
	}

	// Handle custom types (struct/enum/builtin)
	if idlType, ok := p.IDLTypeByName[argName]; ok {
		switch idlType.Kind {
		case "struct":
			// Struct: serialize fields in order
			record, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("%s expects an object", argName)
			}

			for _, field := range idlType.Fields {
				fieldValue, ok := record[field.Name]
				if !ok {
					return fmt.Errorf("missing struct field: %s", field.Name)
				}

				// Recursively serialize each field
				if err := p.serializeValue(serializer, field.Type, fieldValue); err != nil {
					return err
				}
			}
			return nil
		case "enum":
			// Enum: write variant index first, then variant data
			return p.serializeEnum(serializer, idlType, value)
		case "builtin":
			// builtin types (like PublicKey, Address, etc.) are not handled here
			break
		default:
			return fmt.Errorf("unsupported type kind: %s for type %s", idlType.Kind, argName)
		}
	}

	switch argName {
	case "Address", "Signer":
		return serializeAddress(serializer, value)
	case "PublicKey":
		return serializePublicKey(serializer, value)
	case "String", "string":
		return serializer.SerializeStr(fmt.Sprint(value))
	case "bool", "boolean":
		boolean, ok := value.(bool)
		if !ok {
			return fmt.Errorf("%s expects a boolean", argName)
		}
		return serializer.SerializeBool(boolean)
	case "u8":
		number, err := asUint64(value, math.MaxUint8, "u8")
		if err != nil {
			return err
		}
		return serializer.SerializeU8(uint8(number))
	case "u16":
		number, err := asUint64(value, math.MaxUint16, "u16")
		if err != nil {
			return err
		}
		return serializer.SerializeU16(uint16(number))
	case "u32":
		number, err := asUint64(value, math.MaxUint32, "u32")
		if err != nil {
			return err
		}
		return serializer.SerializeU32(uint32(number))
	case "u64":
		number, err := asUint64(value, math.MaxUint64, "u64")
		if err != nil {
			return err
		}
		return serializer.SerializeU64(number)
	case "Bitmap64":
		number, err := asUint64(value, math.MaxUint64, "u64")
		if err != nil {
			return err
		}
		return serializer.SerializeU64(number)
	case "u128":
		number, err := asBigInt(value, false)
		if err != nil {
			return err
		}
		return serializer.SerializeU128(number)
	case "i8":
		number, err := asInt64(value, math.MinInt8, math.MaxInt8, "i8")
		if err != nil {
			return err
		}
		return serializer.SerializeI8(int8(number))
	case "i16":
		number, err := asInt64(value, math.MinInt16, math.MaxInt16, "i16")
		if err != nil {
			return err
		}
		return serializer.SerializeI16(int16(number))
	case "i32":
		number, err := asInt64(value, math.MinInt32, math.MaxInt32, "i32")
		if err != nil {
			return err
		}
		return serializer.SerializeI32(int32(number))
	case "i64":
		number, err := asInt64(value, math.MinInt64, math.MaxInt64, "i64")
		if err != nil {
			return err
		}
		return serializer.SerializeI64(number)
	case "bytes":
		buf, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("bytes expects a []byte slice")
		}
		return serializer.SerializeBytes(buf)
	case "B96":
		switch v := value.(type) {
		case [12]byte:
			serializer.SerializeFixedBytes(v[:])
		case []byte:
			if len(v) != 12 {
				return fmt.Errorf("B96 expects exactly 12 bytes, got %d", len(v))
			}
			serializer.SerializeFixedBytes(v)
		default:
			return fmt.Errorf("B96 expects [12]byte or []byte")
		}
		return nil
	case "B144":
		switch v := value.(type) {
		case [18]byte:
			serializer.SerializeFixedBytes(v[:])
		case []byte:
			if len(v) != 18 {
				return fmt.Errorf("B144 expects exactly 18 bytes, got %d", len(v))
			}
			serializer.SerializeFixedBytes(v)
		default:
			return fmt.Errorf("B144 expects [18]byte or []byte")
		}
		return nil
	case "B160":
		switch v := value.(type) {
		case [20]byte:
			serializer.SerializeFixedBytes(v[:])
		case []byte:
			if len(v) != 20 {
				return fmt.Errorf("B160 expects exactly 20 bytes, got %d", len(v))
			}
			serializer.SerializeFixedBytes(v)
		default:
			return fmt.Errorf("B160 expects [20]byte or []byte")
		}
		return nil
	case "B256":
		switch v := value.(type) {
		case [32]byte:
			serializer.SerializeFixedBytes(v[:])
		case []byte:
			if len(v) != 32 {
				return fmt.Errorf("B256 expects exactly 32 bytes, got %d", len(v))
			}
			serializer.SerializeFixedBytes(v)
		default:
			return fmt.Errorf("B256 expects [32]byte or []byte")
		}
		return nil
	default:
		return fmt.Errorf("unsupported IDL type: %s", argName)
	}
}

// Decode decodes instruction bytes into readable argument
func (p *Provider) Decode(instructionName string, body []byte) (Args, error) {
	// Get instruction definition
	instruction, err := p.GetInstructionByName(instructionName)
	if err != nil {
		return nil, err
	}

	offset := 0
	// Check data length, need at least 3 bytes (app_id + discriminator u16 LE)
	if len(body) < 3 {
		return nil, fmt.Errorf("empty body: need at least 3 bytes")
	}

	// Read and verify appID
	appID := body[offset]
	offset++
	if appID != p.appID() {
		return nil, fmt.Errorf("app_id mismatch: expected %d, got %d", p.appID(), appID)
	}

	// Decode and verify discriminator (u16 LE little-endian encoding, 2 bytes)
	discriminatorLow := uint64(body[offset])
	discriminatorHigh := uint64(body[offset+1])
	discriminator := discriminatorLow | (discriminatorHigh << 8)
	offset += 2

	if discriminator != uint64(instruction.Discriminator) {
		return nil, fmt.Errorf("discriminator mismatch: expected %d, got %d", instruction.Discriminator, discriminator)
	}

	// Deserialize instruction arguments one by one
	args := make(Args)
	for _, arg := range instruction.Args {
		value, err := p.deserializeValue(arg.Type, body, &offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode argument %s: %w", arg.Name, err)
		}
		args[arg.Name] = value
	}

	// Check if there are unprocessed bytes
	if offset != len(body) {
		return nil, fmt.Errorf("%d trailing bytes after decoding", len(body)-offset)
	}

	return args, nil
}

// deserializeValue deserializes a single value into Go object
func (p *Provider) deserializeValue(idlTypeName string, body []byte, offset *int) (any, error) {
	// Handle vec<T>
	// Format: [length(varint)] + [element1] + [element2] + ... + [elementN]
	if inner, ok := parseWrappedType(idlTypeName, "vec"); ok {
		// Read array length
		length, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}

		// 防止恶意输入导致 OOM（也覆盖 32 位平台 int 溢出）
		const maxVecLen = 1 << 20 // ~100万
		if length > maxVecLen {
			return nil, fmt.Errorf("vec length %d exceeds max %d", length, maxVecLen)
		}

		items := make([]any, length)
		for i := uint64(0); i < length; i++ {
			// Recursively deserialize each element
			item, err := p.deserializeValue(inner, body, offset)
			if err != nil {
				return nil, err
			}
			items[i] = item
		}

		return items, nil
	}

	// Handle option<T>
	// Format: [has_value(u8: 0 or 1)] + [value(if has_value != 0)]
	if inner, ok := parseWrappedType(idlTypeName, "option"); ok {
		// Read has_value flag
		hasValue, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}

		// If no value, return nil
		if hasValue == 0 {
			return nil, nil
		}

		// Continue to deserialize actual value if has value
		return p.deserializeValue(inner, body, offset)
	}

	// Handle map<K,V> type
	// Format: [length(varint)] + [key1 + value1] + [key2 + value2] + ... + [keyN + valueN]
	if keyType, valueType, ok, err := parseMapType(idlTypeName); err != nil {
		return nil, err
	} else if ok {
		// Read key-value pair count
		length, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}

		result := make(map[any]any)
		for i := uint64(0); i < length; i++ {
			// Recursively deserialize key
			key, err := p.deserializeValue(keyType, body, offset)
			if err != nil {
				return nil, err
			}

			// Recursively deserialize value
			value, err := p.deserializeValue(valueType, body, offset)
			if err != nil {
				return nil, err
			}

			result[key] = value
		}

		return result, nil
	}

	// Handle tuple<T1,T2,...>
	// Format: [element1] + [element2] + ... + [elementN]
	if tupleTypes, ok, err := parseTupleType(idlTypeName); err != nil {
		return nil, err
	} else if ok {
		items := make([]any, len(tupleTypes))

		for i, itemType := range tupleTypes {
			// Recursively deserialize each element in order
			item, err := p.deserializeValue(itemType, body, offset)
			if err != nil {
				return nil, err
			}
			items[i] = item
		}

		return items, nil
	}

	// Handle custom IDL types (struct/enum/builtin)
	if idlType, ok := p.IDLTypeByName[idlTypeName]; ok {
		switch idlType.Kind {
		case "struct":
			// Struct: deserialize fields in order
			record := make(map[string]any)

			for _, field := range idlType.Fields {
				value, err := p.deserializeValue(field.Type, body, offset)
				if err != nil {
					return nil, err
				}
				record[field.Name] = value
			}

			return record, nil
		case "enum":
			// Enum: [variant_index(varint)] + [fields(if any)]
			// Read variant index
			variantIndex, err := DecodeViewVarUint(body, offset)
			if err != nil {
				return nil, fmt.Errorf("failed to read enum variant index for %s: %w", idlTypeName, err)
			}

			// Validate variant index
			if int(variantIndex) >= len(idlType.Variants) {
				return nil, fmt.Errorf("invalid variant index %d for enum %s (has %d variants)", variantIndex, idlTypeName, len(idlType.Variants))
			}

			variant := idlType.Variants[variantIndex]

			// Handle different variant kinds
			switch variant.Kind {
			case "unit":
				// Unit variant: no fields, just return the variant name
				return map[string]any{
					"variant": variant.Name,
					"index":   variantIndex,
				}, nil
			case "struct":
				// Struct variant: has named fields
				record := make(map[string]any)
				record["variant"] = variant.Name
				record["index"] = variantIndex

				for _, field := range variant.Fields {
					value, err := p.deserializeValue(field.Type, body, offset)
					if err != nil {
						return nil, fmt.Errorf("failed to deserialize field %s of variant %s: %w", field.Name, variant.Name, err)
					}
					record[field.Name] = value
				}

				return record, nil
			case "tuple":
				// Tuple variant: has unnamed fields
				fields := make([]any, len(variant.Fields))
				for i, field := range variant.Fields {
					value, err := p.deserializeValue(field.Type, body, offset)
					if err != nil {
						return nil, fmt.Errorf("failed to deserialize field %d of variant %s: %w", i, variant.Name, err)
					}
					fields[i] = value
				}

				return map[string]any{
					"variant": variant.Name,
					"index":   variantIndex,
					"fields":  fields,
				}, nil
			default:
				return nil, fmt.Errorf("unsupported variant kind %s for %s::%s", variant.Kind, idlTypeName, variant.Name)
			}
		case "builtin":
			// builtin types are not handled here, continue to execute switch idlTypeName below
			break
		default:
			return nil, fmt.Errorf("unsupported type kind: %s for type %s", idlType.Kind, idlTypeName)
		}
	}

	// Handle built-in basic types
	switch idlTypeName {
	case "Address", "Signer":
		// Address: fixed 20 bytes
		if *offset+20 > len(body) {
			return nil, fmt.Errorf("insufficient data for Address")
		}
		addrBytes := make([]byte, 20)
		copy(addrBytes, body[*offset:*offset+20])
		*offset += 20

		addr, err := crypto.NewAddressFromBytes(addrBytes)
		if err != nil {
			return addrBytes, nil
		}

		return addr, nil
	case "PublicKey":
		// PublicKey: [variant(varint)] + [byte data(fixed length, depends on key type)]
		// First read Variant (using varint encoding)
		variantRaw, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to read PublicKey variant: %w", err)
		}

		// Determine byte length based on Variant
		var expectedLen int
		switch crypto.PublicKeyType(uint32(variantRaw)) {
		case crypto.PublicKeyTypeSecp256k1:
			expectedLen = crypto.PublicKeySecp256k1Size // 33
		case crypto.PublicKeyTypeEd25519:
			expectedLen = crypto.PublicKeyEd25519Size // 32
		case crypto.PublicKeyTypeBLS12381:
			expectedLen = crypto.PublicKeyBLS12381Size // 48
		case crypto.PublicKeyTypeFnDsa512:
			expectedLen = crypto.PublicKeyFnDsa512Size // 897
		default:
			return nil, fmt.Errorf("unknown public key variant: %d", variantRaw)
		}

		// Read fixed length byte data
		if *offset+expectedLen > len(body) {
			return nil, fmt.Errorf("insufficient data for PublicKey bytes: expected %d, got %d", expectedLen, len(body)-*offset)
		}
		pkBytes := make([]byte, expectedLen)
		copy(pkBytes, body[*offset:*offset+expectedLen])
		*offset += expectedLen

		pk, err := crypto.NewPublicKeyFromBytes(pkBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to create PublicKey from bytes: %w", err)
		}

		return pk, nil
	case "String", "string":
		// String: [length(varint)] + [UTF-8 byte data]
		length, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}
		if length > math.MaxInt32 {
			return nil, fmt.Errorf("string length %d exceeds MaxInt32", length)
		}

		if *offset+int(length) > len(body) {
			return nil, fmt.Errorf("insufficient data for String")
		}

		str := string(body[*offset : *offset+int(length)])
		*offset += int(length)
		return str, nil
	case "bool", "boolean":
		if *offset >= len(body) {
			return nil, fmt.Errorf("insufficient data for bool")
		}
		val := body[*offset] != 0
		*offset++
		return val, nil
	case "u8":
		val, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}
		if val > math.MaxUint8 {
			return nil, fmt.Errorf("u8 value overflow: %d", val)
		}
		return uint8(val), nil
	case "u16":
		val, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}
		if val > math.MaxUint16 {
			return nil, fmt.Errorf("u16 value overflow: %d", val)
		}
		return uint16(val), nil
	case "u32":
		val, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}
		if val > math.MaxUint32 {
			return nil, fmt.Errorf("u32 value overflow: %d", val)
		}
		return uint32(val), nil
	case "u64", "Bitmap64":
		return DecodeViewVarUint(body, offset)
	case "i8":
		val, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}
		if val > math.MaxInt8 {
			return nil, fmt.Errorf("i8 value overflow: %d", val)
		}
		return int8(val), nil
	case "i16":
		val, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}
		if val > math.MaxInt16 {
			return nil, fmt.Errorf("i16 value overflow: %d", val)
		}
		return int16(val), nil
	case "i32":
		val, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}
		if val > math.MaxInt32 {
			return nil, fmt.Errorf("i32 value overflow: %d", val)
		}
		return int32(val), nil
	case "i64":
		return DecodeViewVarUint(body, offset)
	case "bytes":
		// bytes: [length(varint)] + [byte data]
		length, err := DecodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}
		if length > math.MaxInt32 {
			return nil, fmt.Errorf("bytes length %d exceeds MaxInt32", length)
		}

		if *offset+int(length) > len(body) {
			return nil, fmt.Errorf("insufficient data for bytes")
		}

		byteList := make([]byte, length)
		copy(byteList, body[*offset:*offset+int(length)])
		*offset += int(length)
		return byteList, nil
	case "B96":
		// B96: fixed 12 bytes, no length prefix
		if *offset+12 > len(body) {
			return nil, fmt.Errorf("insufficient data for B96")
		}
		var b96 [12]byte
		copy(b96[:], body[*offset:*offset+12])
		*offset += 12
		return b96, nil
	case "B144":
		// B144: fixed 18 bytes, no length prefix
		if *offset+18 > len(body) {
			return nil, fmt.Errorf("insufficient data for B144")
		}
		var b144 [18]byte
		copy(b144[:], body[*offset:*offset+18])
		*offset += 18
		return b144, nil
	case "B160":
		// B160: fixed 20 bytes, no length prefix
		if *offset+20 > len(body) {
			return nil, fmt.Errorf("insufficient data for B160")
		}
		var b160 [20]byte
		copy(b160[:], body[*offset:*offset+20])
		*offset += 20
		return b160, nil
	case "B256":
		// B256: fixed 32 bytes, no length prefix
		if *offset+32 > len(body) {
			return nil, fmt.Errorf("insufficient data for B256")
		}
		var b256 [32]byte
		copy(b256[:], body[*offset:*offset+32])
		*offset += 32
		return b256, nil
	default:
		return nil, fmt.Errorf("unsupported IDL type: %s", idlTypeName)
	}
}

func (p *Provider) DecodeViewDatas(instructionName string, body []byte) ([]DecodedTaggedValue, error) {
	instruction, err := p.GetInstructionByName(instructionName)
	if err != nil {
		return nil, err
	}

	// Verify instruction type, must be view type
	if instruction.Kind != "view" {
		return nil, fmt.Errorf("%s is not a view instruction (kind=%s)", instructionName, instruction.Kind)
	}

	// Get return value type definition
	returnType := strings.TrimSpace(instruction.Returns.Type)
	if returnType == "" {
		return nil, fmt.Errorf("IDL view method %s is missing returns.type", instructionName)
	}

	// Deserialize outer Vec<Result<...>> structure
	offset := 0

	// 1. Read Vec length (instruction count)
	resultCount, err := DecodeViewVarUint(body, &offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode result count: %w", err)
	}
	if resultCount > math.MaxInt32 {
		return nil, fmt.Errorf("result count %d exceeds MaxInt32", resultCount)
	}

	results := make([]DecodedTaggedValue, resultCount)

	// 2. Decode each Result item one by one
	for i := uint64(0); i < resultCount; i++ {
		// Read Result variant index: 0 = Ok, 1 = Err
		variantIndex, err := DecodeViewVarUint(body, &offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode result[%d] variant: %w", i, err)
		}

		if variantIndex == 0 {
			// Ok branch: contains actual return value bytes

			// First read Ok internal Vec<u8> length
			okDataLen, err := DecodeViewVarUint(body, &offset)
			if err != nil {
				return nil, fmt.Errorf("failed to decode result[%d] Ok data length: %w", i, err)
			}
			if okDataLen > math.MaxInt32 {
				return nil, fmt.Errorf("ok data length %d exceeds MaxInt32", okDataLen)
			}

			// Read Ok internal byte data
			if offset+int(okDataLen) > len(body) {
				return nil, fmt.Errorf("insufficient data for result[%d] Ok payload", i)
			}
			okData := make([]byte, okDataLen)
			copy(okData, body[offset:offset+int(okDataLen)])
			offset += int(okDataLen)

			// Deserialize actual return value
			valueOffset := 0
			value, err := p.deserializeValue(returnType, okData, &valueOffset)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize result[%d] Ok value: %w", i, err)
			}

			results[i] = DecodedTaggedValue{Value: value}
		} else if variantIndex == 1 {
			// Err branch: contains TxFailurePayload
			failure, err := p.decodeTxFailurePayload(body, &offset)
			if err != nil {
				return nil, fmt.Errorf("failed to decode result[%d] Err payload: %w", i, err)
			}

			results[i] = DecodedTaggedValue{Value: failure}
		} else {
			return nil, fmt.Errorf("invalid result variant index: %d", variantIndex)
		}
	}

	// Check if there are unprocessed bytes
	if offset != len(body) {
		return nil, fmt.Errorf("%d trailing bytes after decoding", len(body)-offset)
	}

	return results, nil
}
func (p *Provider) decodeTxFailurePayload(body []byte, offset *int) (*api.TxFailurePayload, error) {
	// Decode code (u16)
	codeRaw, err := DecodeViewVarUint(body, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode failure code: %w", err)
	}
	if codeRaw > math.MaxUint16 {
		return nil, fmt.Errorf("failure code overflow: %d", codeRaw)
	}
	code := uint16(codeRaw)

	// Decode message (String)
	messageLen, err := DecodeViewVarUint(body, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode message length: %w", err)
	}
	if messageLen > math.MaxInt32 {
		return nil, fmt.Errorf("message length %d exceeds MaxInt32", messageLen)
	}
	if *offset+int(messageLen) > len(body) {
		return nil, fmt.Errorf("insufficient data for failure message")
	}
	message := string(body[*offset : *offset+int(messageLen)])
	*offset += int(messageLen)

	// Decode data (Vec<u8>)
	dataLen, err := DecodeViewVarUint(body, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode data length: %w", err)
	}
	if dataLen > math.MaxInt32 {
		return nil, fmt.Errorf("data length %d exceeds MaxInt32", dataLen)
	}
	if *offset+int(dataLen) > len(body) {
		return nil, fmt.Errorf("insufficient data for failure data")
	}
	data := make([]byte, dataLen)
	copy(data, body[*offset:*offset+int(dataLen)])
	*offset += int(dataLen)

	return &api.TxFailurePayload{
		Code:    code,
		Message: message,
		Data:    data,
	}, nil
}

func (p *Provider) DecodeViewData(instructionName string, body []byte) (any, error) {
	values, err := p.DecodeViewDatas(instructionName, body)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("view %s returned no values", instructionName)
	}
	return values[0].Value, nil
}

func (p *Provider) DecodeDataByIDLTypeName(idlTypeName string, data []byte) (any, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty resource data")
	}

	offset := 0
	value, err := p.deserializeValue(idlTypeName, data, &offset)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize %s: %w", idlTypeName, err)
	}

	if offset != len(data) {
		return nil, fmt.Errorf("%d trailing bytes after decoding %s", len(data)-offset, idlTypeName)
	}

	return value, nil
}

// serializeEnum serializes enum type to postcard format
func (p *Provider) serializeEnum(serializer *postcard.Serializer, idlType IDLType, value any) error {
	// Parse enum input, get variant name and variant value
	variantName, variantValue, err := enumVariantInput(value)
	if err != nil {
		return err
	}

	// Find variant index in IDL definition
	variantIndex := -1
	var variant EnumVariant
	for i, candidate := range idlType.Variants {
		// Use case-insensitive matching
		if strings.EqualFold(candidate.Name, variantName) {
			variantIndex = i
			variant = candidate
			break
		}
	}
	if variantIndex < 0 {
		return fmt.Errorf("unknown enum variant %s.%s", idlType.Name, variantName)
	}

	// Write variant index (varint encoding)
	if err = serializer.SerializeEnumVariant(uint32(variantIndex)); err != nil {
		return err
	}

	// Handle Unit variant (no associated data)
	if variant.Kind == "unit" {
		return nil
	}

	// Handle Tuple variant (associated data is tuple)
	if variant.Kind == "tuple" {
		// Convert value to fixed-length tuple
		tuple, err := tupleValues(variantValue, len(variant.Fields))
		if err != nil {
			return err
		}
		// Serialize each element of tuple in order
		for i, field := range variant.Fields {
			if err := p.serializeValue(serializer, field.Type, tuple[i]); err != nil {
				return err
			}
		}
		return nil
	}

	// Handle Struct variant (associated data is named fields)
	record, ok := variantValue.(map[string]any)
	if !ok {
		return fmt.Errorf("%s.%s expects an object", idlType.Name, variant.Name)
	}

	// Serialize each field in the order defined by IDL
	for _, field := range variant.Fields {
		fieldValue, ok := record[field.Name]
		if !ok {
			return fmt.Errorf("missing enum field: %s", field.Name)
		}
		if err := p.serializeValue(serializer, field.Type, fieldValue); err != nil {
			return err
		}
	}

	return nil
}

//********************** Helper Functions ********************

// DecodeViewVarUint varint (variable length integer) decoding function
func DecodeViewVarUint(input []byte, offset *int) (uint64, error) {
	var value uint64 // Store final value
	var shift uint   // Shift amount, increases by 7 each time

	for i := 0; i < 10; i++ { // Loop at most 10 times (uint64 needs at most 10 bytes)
		if *offset >= len(input) {
			return 0, fmt.Errorf("unexpected end of input") // Insufficient data, report error
		}

		b := input[*offset]   // Read current byte
		*offset = *offset + 1 // Move read position

		value |= uint64(b&0x7f) << shift // Take lower 7 bits, left shift and merge into value
		if b&0x80 == 0 {                 // If highest bit is 0, it's the last byte
			return value, nil // Decoding complete, return result
		}

		shift += 7 // Otherwise continue reading next byte, shift amount increases by 7
	}

	return 0, fmt.Errorf("varint is too long") // Exceeds 10 bytes, report error
}

// parseWrappedType parses wrapped types (such as vec<T>, option<T>, map<K,V>, tuple<T1,T2>)
//
// Examples:
//
//	parseWrappedType("vec<u8>", "vec")           → ("u8", true)
//	parseWrappedType("option<String>", "option") → ("String", true)
//	parseWrappedType("map<K,V>", "map")          → ("K,V", true)
//	parseWrappedType("Vec<u8>", "vec")           → ("u8", true)  // ignore case
//	parseWrappedType("u64", "vec")               → ("", false)
func parseWrappedType(argName, wrapper string) (string, bool) {
	prefix := wrapper + "<"

	if !strings.HasPrefix(strings.ToLower(argName), prefix) || !strings.HasSuffix(argName, ">") {
		return "", false
	}

	// Extract content inside angle brackets
	return strings.TrimSpace(argName[len(prefix) : len(argName)-1]), true
}

// parseMapType parses map type, extracts key and value types
//
// Examples:
//
//	parseMapType("map<String,u64>")     → ("String", "u64", true, nil)
//	parseMapType("map<Address,vec<u8>>") → ("Address", "vec<u8>", true, nil)
//	parseMapType("u64")                  → ("", "", false, nil)  // not map type
//	parseMapType("map<String>")          → ("", "", false, err)  // missing value type
func parseMapType(argName string) (string, string, bool, error) {
	inner, ok := parseWrappedType(argName, "map")
	if !ok {
		return "", "", false, nil
	}

	// Split by comma, supports nested types (such as map<String,vec<u8>>)
	parts := splitTopLevel(inner, ',')
	if len(parts) != 2 {
		return "", "", false, fmt.Errorf("invalid map type: %s", argName)
	}

	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true, nil
}

// parseTupleType parses tuple type, extracts all element types
//
// Examples:
//
//	parseTupleType("tuple<u8,u16,u32>")       → (["u8", "u16", "u32"], true, nil)
//	parseTupleType("tuple<Address,vec<u8>>")   → (["Address", "vec<u8>"], true, nil)
//	parseTupleType("u64")                      → (nil, false, nil)  // not tuple type
//	parseTupleType("tuple<>")                  → ([], true, nil)  // empty tuple
func parseTupleType(argName string) ([]string, bool, error) {
	inner, ok := parseWrappedType(argName, "tuple")
	if !ok {
		return nil, false, nil
	}

	// Split by comma, supports nested types
	parts := splitTopLevel(inner, ',')

	// Remove leading/trailing spaces from each element
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, true, nil
}

func splitTopLevel(value string, separator rune) []string {
	parts := []string{}
	depth := 0
	start := 0
	for i, char := range value {
		if char == '<' {
			depth++
		}
		if char == '>' {
			depth--
		}
		if char == separator && depth == 0 {
			parts = append(parts, value[start:i])
			start = i + 1
		}
	}
	parts = append(parts, value[start:])
	return parts
}

func sliceValues(value any) ([]any, error) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil, fmt.Errorf("invalid slice")
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, fmt.Errorf("invalid slice")
	}
	out := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = rv.Index(i).Interface()
	}
	return out, nil
}

func mapEntries(value any) ([][2]any, error) {
	if value == nil {
		return nil, fmt.Errorf("invalid map")
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Map:
		entries := make([][2]any, 0, rv.Len())
		for _, key := range rv.MapKeys() {
			entries = append(entries, [2]any{key.Interface(), rv.MapIndex(key).Interface()})
		}
		return entries, nil
	case reflect.Slice, reflect.Array:
		entries := make([][2]any, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			item := rv.Index(i).Interface()
			switch typed := item.(type) {
			case [2]any:
				entries = append(entries, typed)
			case []any:
				if len(typed) != 2 {
					return nil, fmt.Errorf("invalid map entry")
				}
				entries = append(entries, [2]any{typed[0], typed[1]})
			default:
				return nil, fmt.Errorf("invalid map entry")
			}
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("invalid map")
	}
}

func tupleValues(value any, length int) ([]any, error) {
	if items, err := sliceValues(value); err == nil {
		if len(items) != length {
			return nil, fmt.Errorf("tuple expects %d values", length)
		}
		return items, nil
	}
	record, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tuple expects an array or object")
	}
	out := make([]any, length)
	for i := 0; i < length; i++ {
		key := fmt.Sprintf("%d", i)
		fieldValue, ok := record[key]
		if !ok {
			return nil, fmt.Errorf("missing tuple field: %s", key)
		}
		out[i] = fieldValue
	}
	return out, nil
}

func enumVariantInput(value any) (string, any, error) {
	if name, ok := value.(string); ok {
		return name, nil, nil
	}
	record, ok := value.(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("enum expects a variant string or object")
	}
	if variant, ok := record["variant"].(string); ok {
		if inner, ok := record["value"]; ok {
			return variant, inner, nil
		}
		if inner, ok := record["fields"]; ok {
			return variant, inner, nil
		}
		return variant, map[string]any{}, nil
	}
	if len(record) != 1 {
		return "", nil, fmt.Errorf("enum object must contain exactly one variant")
	}
	for key, inner := range record {
		return key, inner, nil
	}
	return "", nil, fmt.Errorf("enum object must contain exactly one variant")
}

func serializeAddress(serializer *postcard.Serializer, value any) error {
	var buf []byte

	switch typed := value.(type) {
	case crypto.Address:
		buf = append([]byte(nil), typed.Bytes[:]...)
	case *crypto.Address:
		if typed == nil {
			return fmt.Errorf("nil Address")
		}
		buf = append([]byte(nil), typed.Bytes[:]...)
	case []byte:
		if len(typed) != 20 {
			return fmt.Errorf("address must be 20 bytes")
		}
		buf = append([]byte(nil), typed...)
	case string:
		// Try hex parsing
		hexBuf, err := decodeHex(typed)
		if err == nil && len(hexBuf) == 20 {
			buf = hexBuf
		} else {
			// Try base58 parsing
			b58Buf := base58.Decode(typed)
			if len(b58Buf) != 20 {
				return fmt.Errorf("address must be 20 bytes")
			}
			buf = b58Buf
		}
	default:
		return fmt.Errorf("invalid type for Address: %T", value)
	}

	serializer.SerializeFixedBytes(buf)
	return nil
}

func serializePublicKey(serializer *postcard.Serializer, value any) error {
	var pk *crypto.PublicKey
	var err error

	switch v := value.(type) {
	case crypto.PublicKey:
		pk = &v
	case *crypto.PublicKey:
		if v == nil {
			return fmt.Errorf("nil PublicKey")
		}
		pk = v
	case string:
		pk, err = crypto.NewPublicKeyFromStringRelaxed(v)
		if err != nil {
			return fmt.Errorf("failed to parse public key from string: %w", err)
		}
	case []byte:
		pk, err = crypto.NewPublicKeyFromBytes(v)
		if err != nil {
			return fmt.Errorf("failed to create PublicKey from bytes: %w", err)
		}
	default:
		return fmt.Errorf("invalid type for PublicKey: %T", value)
	}

	if pk == nil {
		return fmt.Errorf("public key is nil after parsing")
	}

	// First serialize Variant (4 bytes)
	if err = serializer.SerializeU32(uint32(pk.Variant)); err != nil {
		return fmt.Errorf("failed to serialize public key variant: %w", err)
	}

	// Then serialize byte data (fixed length, no length prefix)
	serializer.SerializeFixedBytes(pk.Bytes)
	return nil
}

func asUint64(value any, max uint64, name string) (uint64, error) {
	switch typed := value.(type) {
	case uint8:
		return uint64(typed), nil
	case uint16:
		return uint64(typed), nil
	case uint32:
		return uint64(typed), nil
	case uint64:
		if typed > max {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return typed, nil
	case uint:
		if uint64(typed) > max {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return uint64(typed), nil
	case int:
		if typed < 0 || uint64(typed) > max {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return uint64(typed), nil
	case int8:
		if typed < 0 {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return uint64(typed), nil
	case int16:
		if typed < 0 {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return uint64(typed), nil
	case int32:
		if typed < 0 {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return uint64(typed), nil
	case int64:
		if typed < 0 || uint64(typed) > max {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return uint64(typed), nil
	case float64:
		if typed < 0 || typed != math.Trunc(typed) || typed > float64(max) {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return uint64(typed), nil
	case json.Number:
		return asUint64(string(typed), max, name)
	case string:
		number, ok := new(big.Int).SetString(typed, 10)
		if !ok || number.Sign() < 0 || !number.IsUint64() || number.Uint64() > max {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return number.Uint64(), nil
	default:
		return 0, fmt.Errorf("%s out of range", name)
	}
}

func asBigInt(value any, signed bool) (*big.Int, error) {
	switch typed := value.(type) {
	case *big.Int:
		if typed == nil {
			return nil, fmt.Errorf("u128 out of range")
		}
		if !signed && typed.Sign() < 0 {
			return nil, fmt.Errorf("u128 out of range")
		}
		return new(big.Int).Set(typed), nil
	case uint8, uint16, uint32, uint64, uint, int, int8, int16, int32, int64:
		rv := reflect.ValueOf(typed)
		number := big.NewInt(0)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if !signed && rv.Int() < 0 {
				return nil, fmt.Errorf("u128 out of range")
			}
			number.SetInt64(rv.Int())
		default:
			number.SetUint64(rv.Convert(reflect.TypeOf(uint64(0))).Uint())
		}
		return number, nil
	case float64:
		if typed != math.Trunc(typed) {
			return nil, fmt.Errorf("u128 out of range")
		}
		if !signed && typed < 0 {
			return nil, fmt.Errorf("u128 out of range")
		}
		return big.NewInt(int64(typed)), nil
	case json.Number:
		return asBigInt(string(typed), signed)
	case string:
		number, ok := new(big.Int).SetString(typed, 10)
		if !ok || (!signed && number.Sign() < 0) {
			return nil, fmt.Errorf("u128 out of range")
		}
		return number, nil
	default:
		return nil, fmt.Errorf("u128 out of range")
	}
}

func asInt64(value any, min, max int64, name string) (int64, error) {
	switch typed := value.(type) {
	case int:
		if int64(typed) < min || int64(typed) > max {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return int64(typed), nil
	case int8:
		return int64(typed), nil
	case int16:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		if typed < min || typed > max {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return typed, nil
	case uint8, uint16, uint32, uint64, uint:
		rv := reflect.ValueOf(typed)
		number := rv.Convert(reflect.TypeOf(uint64(0))).Uint()
		if number > uint64(max) {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return int64(number), nil
	case float64:
		if typed != math.Trunc(typed) || typed < float64(min) || typed > float64(max) {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return int64(typed), nil
	case json.Number:
		return asInt64(string(typed), min, max, name)
	case string:
		number, ok := new(big.Int).SetString(typed, 10)
		if !ok || number.Cmp(big.NewInt(min)) < 0 || number.Cmp(big.NewInt(max)) > 0 {
			return 0, fmt.Errorf("%s out of range", name)
		}
		return number.Int64(), nil
	default:
		return 0, fmt.Errorf("%s out of range", name)
	}
}

func isNilValue(value any) bool {
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

func decodeHex(value string) ([]byte, error) {
	normalized := value
	if len(value) >= 2 && (value[:2] == "0x" || value[:2] == "0X") {
		normalized = value[2:]
	}
	buf, err := hex.DecodeString(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string")
	}
	return buf, nil
}

//************************ todo---临时调试代码
