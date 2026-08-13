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
	InstructionByName          map[string]*Instruction // key=Instruction.Name
	InstructionByDiscriminator map[uint16]*Instruction // key=Instruction.Discriminator
	IDLTypeByName              map[string]IDLType      // key=IDLType.Name
	IDLTypeByTypeTag           map[uint64]*IDLType     // key=IDLType.typeTag,
	EventByTypeTag             map[uint64]Event        // key=Event.typeTag
}

func NewProvider(idl IDL) *Provider {
	instructionByName := make(map[string]*Instruction, len(idl.Instructions))
	instructionByDiscriminator := make(map[uint16]*Instruction, len(idl.Instructions))

	for i := range idl.Instructions {
		instruction := &idl.Instructions[i]
		instructionByName[instruction.Name] = instruction
		instructionByDiscriminator[instruction.Discriminator] = instruction
	}

	idlTypeByName := make(map[string]IDLType, len(idl.Types))
	idlTypeByTypeTag := make(map[uint64]*IDLType, len(idl.Types))
	for i := range idl.Types {
		value := &idl.Types[i]
		idlTypeByName[value.Name] = *value
		idlTypeByTypeTag[value.TypeTag] = value
	}

	eventByTypeTag := make(map[uint64]Event, len(idl.Events))
	for _, event := range idl.Events {
		// Convert the event into an IDLType so it can be handled uniformly
		idlType := &IDLType{
			Name:    event.Name,
			TypeTag: event.TypeTag,
			Kind:    "struct", // Event is essentially a struct
			Fields:  make([]StructField, len(event.Fields)),
		}
		for i, field := range event.Fields {
			idlType.Fields[i] = StructField{
				Name: field.Name,
				Type: field.Type,
			}
		}

		// Register as IDLType only when it does not collide with an existing
		// type definition (same name or typeTag); the existing type wins so a
		// malformed IDL cannot silently shadow a real type.
		if _, exists := idlTypeByName[idlType.Name]; !exists {
			idlTypeByName[idlType.Name] = *idlType
		}
		if _, exists := idlTypeByTypeTag[idlType.TypeTag]; !exists {
			idlTypeByTypeTag[idlType.TypeTag] = idlType
		}

		// Also register it under EventByTypeTag
		eventByTypeTag[event.TypeTag] = event
	}

	return &Provider{
		IDL:                        idl,
		InstructionByName:          instructionByName,
		InstructionByDiscriminator: instructionByDiscriminator,
		IDLTypeByName:              idlTypeByName,
		IDLTypeByTypeTag:           idlTypeByTypeTag,
		EventByTypeTag:             eventByTypeTag,
	}
}

func LoadProviderFromFile(path string) (*Provider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read IDL file %s: %w", path, err)
	}
	var idl IDL
	if err = json.Unmarshal(data, &idl); err != nil {
		return nil, fmt.Errorf("failed to unmarshal IDL %s: %w", path, err)
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
	return idlType, ok
}

func (p *Provider) GetEventByTypeTag(typeTag uint64) (*Event, bool) {
	event, ok := p.EventByTypeTag[typeTag]
	if !ok {
		return nil, false
	}
	return &event, true
}

// Encode encodes instruction args into wire bytes for on-chain submission.
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
	// 1. app_id (1 byte)
	if err := serializer.SerializeU8(p.appID()); err != nil {
		return nil, err
	}

	// 2. discriminator (u16 LE, 2 bytes)
	serializer.SerializeFixedBytes([]byte{byte(instruction.Discriminator), byte(instruction.Discriminator >> 8)})

	// 3. args in IDL order
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

// serializeValue writes one value in postcard format.
func (p *Provider) serializeValue(serializer *postcard.Serializer, argName string, value any) error {
	// vec<T>: [len(varint)] + items...
	if inner, ok := parseWrappedType(argName, "vec"); ok {
		items, err := sliceValues(value)
		if err != nil {
			return fmt.Errorf("%s expects an array", argName)
		}

		if err = serializer.SerializeU32(uint32(len(items))); err != nil {
			return err
		}

		for _, item := range items {
			if err = p.serializeValue(serializer, inner, item); err != nil {
				return err
			}
		}

		return nil
	}

	// option<T>: [has_value(u8)] + [value if present]
	if inner, ok := parseWrappedType(argName, "option"); ok {
		if value == nil || isNilValue(value) {
			return serializer.SerializeBool(false)
		}

		if err := serializer.SerializeBool(true); err != nil {
			return err
		}

		return p.serializeValue(serializer, inner, value)
	}

	// map<K,V>: [len(varint)] + key/value pairs...
	if keyType, valueType, ok, err := parseMapType(argName); err != nil {
		return err
	} else if ok {
		var entries [][2]any
		entries, err = mapEntries(value)
		if err != nil {
			return fmt.Errorf("map expects a map or entry array")
		}

		if err = serializer.SerializeU32(uint32(len(entries))); err != nil {
			return err
		}

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

	// tuple<T1,T2,...>: elements in order
	if tupleTypes, ok, err := parseTupleType(argName); err != nil {
		return err
	} else if ok {
		var tuple []any
		tuple, err = tupleValues(value, len(tupleTypes))
		if err != nil {
			return err
		}

		for i, itemType := range tupleTypes {
			if err = p.serializeValue(serializer, itemType, tuple[i]); err != nil {
				return err
			}
		}
		return nil
	}

	// custom IDL type (struct/enum/builtin)
	if idlType, ok := p.IDLTypeByName[argName]; ok {
		switch idlType.Kind {
		case "struct":
			record, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("%s expects an object", argName)
			}

			for _, field := range idlType.Fields {
				fieldValue, ok := record[field.Name]
				if !ok {
					return fmt.Errorf("missing struct field: %s", field.Name)
				}

				if err := p.serializeValue(serializer, field.Type, fieldValue); err != nil {
					return err
				}
			}
			return nil
		case "enum":
			return p.serializeEnum(serializer, idlType, value)
		case "builtin":
			// fall through to the primitive switch below
			break
		default:
			return fmt.Errorf("unsupported type kind: %s for type %s", idlType.Kind, argName)
		}
	}

	switch argName {
	case "Address", "Signer", "AnySigner":
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
	case "u64", "Bitmap64", "Amount", "Epoch":
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
		return serializeFixedBytesValue(serializer, value, 12, "B96")
	case "B144":
		return serializeFixedBytesValue(serializer, value, 18, "B144")
	case "B160":
		return serializeFixedBytesValue(serializer, value, 20, "B160")
	case "B256":
		return serializeFixedBytesValue(serializer, value, 32, "B256")
	default:
		return fmt.Errorf("unsupported IDL type: %s", argName)
	}
}

// serializeEnum writes an enum: [variant_index(varint)] + [variant data].
func (p *Provider) serializeEnum(serializer *postcard.Serializer, idlType IDLType, value any) error {
	variantName, variantValue, err := enumVariantInput(value)
	if err != nil {
		return err
	}

	variantIndex := -1
	var variant EnumVariant
	for i, candidate := range idlType.Variants {
		// case-insensitive match
		if strings.EqualFold(candidate.Name, variantName) {
			variantIndex = i
			variant = candidate
			break
		}
	}
	if variantIndex < 0 {
		return fmt.Errorf("unknown enum variant %s.%s", idlType.Name, variantName)
	}

	if err = serializer.SerializeEnumVariant(uint32(variantIndex)); err != nil {
		return err
	}

	// unit: no associated data
	if variant.Kind == "unit" {
		return nil
	}

	if variant.Kind == "tuple" {
		tuple, err := tupleValues(variantValue, len(variant.Fields))
		if err != nil {
			return err
		}
		for i, field := range variant.Fields {
			if err := p.serializeValue(serializer, field.Type, tuple[i]); err != nil {
				return err
			}
		}
		return nil
	}

	record, ok := variantValue.(map[string]any)
	if !ok {
		return fmt.Errorf("%s.%s expects an object", idlType.Name, variant.Name)
	}

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

// Decode decodes an encoded instruction body into its arguments.
func (p *Provider) Decode(instructionName string, body []byte) (Args, error) {
	instruction, err := p.GetInstructionByName(instructionName)
	if err != nil {
		return nil, err
	}

	offset := 0
	// at least 3 bytes: app_id + u16 discriminator
	if len(body) < 3 {
		return nil, fmt.Errorf("empty body: need at least 3 bytes")
	}

	appID := body[offset]
	offset++
	if appID != p.appID() {
		return nil, fmt.Errorf("app_id mismatch: expected %d, got %d", p.appID(), appID)
	}

	// discriminator (u16 LE)
	discriminatorLow := uint64(body[offset])
	discriminatorHigh := uint64(body[offset+1])
	discriminator := discriminatorLow | (discriminatorHigh << 8)
	offset += 2

	if discriminator != uint64(instruction.Discriminator) {
		return nil, fmt.Errorf("discriminator mismatch: expected %d, got %d", instruction.Discriminator, discriminator)
	}

	args := make(Args)
	for _, arg := range instruction.Args {
		value, err := p.deserializeValue(arg.Type, body, &offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode argument %s: %w", arg.Name, err)
		}
		args[arg.Name] = value
	}

	if offset != len(body) {
		return nil, fmt.Errorf("%d trailing bytes after decoding", len(body)-offset)
	}

	return args, nil
}

// deserializeValue decodes one value by its IDL type name.
func (p *Provider) deserializeValue(idlTypeName string, body []byte, offset *int) (any, error) {
	// vec<T>: [len(varint)] + elements...
	if inner, ok := parseWrappedType(idlTypeName, "vec"); ok {
		length, err := decodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}

		items := make([]any, length)
		for i := uint64(0); i < length; i++ {
			item, err := p.deserializeValue(inner, body, offset)
			if err != nil {
				return nil, err
			}
			items[i] = item
		}

		return items, nil
	}

	// option<T>: [has_value(u8)] + [value if present]
	if inner, ok := parseWrappedType(idlTypeName, "option"); ok {
		hasValue, err := decodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}

		if hasValue == 0 {
			return nil, nil
		}

		return p.deserializeValue(inner, body, offset)
	}

	// map<K,V>: [len(varint)] + key/value pairs...
	if keyType, valueType, ok, err := parseMapType(idlTypeName); err != nil {
		return nil, err
	} else if ok {
		length, err := decodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}

		result := make(map[any]any)
		for i := uint64(0); i < length; i++ {
			key, err := p.deserializeValue(keyType, body, offset)
			if err != nil {
				return nil, err
			}
			value, err := p.deserializeValue(valueType, body, offset)
			if err != nil {
				return nil, err
			}
			result[key] = value
		}

		return result, nil
	}

	// tuple<T1,T2,...>: elements in order
	if tupleTypes, ok, err := parseTupleType(idlTypeName); err != nil {
		return nil, err
	} else if ok {
		items := make([]any, len(tupleTypes))
		for i, itemType := range tupleTypes {
			item, err := p.deserializeValue(itemType, body, offset)
			if err != nil {
				return nil, err
			}
			items[i] = item
		}

		return items, nil
	}

	// custom IDL type (struct/enum/builtin)
	if idlType, ok := p.IDLTypeByName[idlTypeName]; ok {
		switch idlType.Kind {
		case "struct":
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
			// enum: [variant_index(varint)] + [fields if any]
			variantIndex, err := decodeViewVarUint(body, offset)
			if err != nil {
				return nil, fmt.Errorf("failed to read enum variant index for %s: %w", idlTypeName, err)
			}

			if int(variantIndex) >= len(idlType.Variants) {
				return nil, fmt.Errorf("invalid variant index %d for enum %s (has %d variants)", variantIndex, idlTypeName, len(idlType.Variants))
			}

			variant := idlType.Variants[variantIndex]
			switch variant.Kind {
			case "unit":
				// unit: no fields
				return map[string]any{
					"variant": variant.Name,
					"index":   variantIndex,
				}, nil
			case "struct":
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
			// fall through to the primitive switch below
			break
		default:
			return nil, fmt.Errorf("unsupported type kind: %s for type %s", idlType.Kind, idlTypeName)
		}
	}

	// primitive types
	switch idlTypeName {
	case "Address", "Signer", "AnySigner":
		// Address: fixed 20 bytes
		if *offset+20 > len(body) {
			return nil, fmt.Errorf("insufficient data for Address")
		}
		addrBytes := make([]byte, 20)
		copy(addrBytes, body[*offset:*offset+20])
		*offset += 20

		addr, err := crypto.NewAddressFromBytes(addrBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to create Address from bytes: %w", err)
		}

		return addr, nil
	case "PublicKey":
		// PublicKey: [variant(varint)] + [fixed-length data]
		variantRaw, err := decodeViewVarUint(body, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to read PublicKey variant: %w", err)
		}

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
		// String: [len(varint)] + UTF-8 bytes
		length, err := decodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
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
		val, err := decodeViewVarUint(body, offset)
		return uint8(val), err
	case "u16":
		val, err := decodeViewVarUint(body, offset)
		return uint16(val), err
	case "u32":
		val, err := decodeViewVarUint(body, offset)
		return uint32(val), err
	case "u64", "Bitmap64", "Amount", "Epoch":
		return decodeViewVarUint(body, offset)
	case "u128":
		return decodeViewVarUint128(body, offset)
	case "i8":
		// i8 is a single byte (SerializeI8 -> SerializeU8); varint decoding would
		// misread negative values (e.g. -1 -> 0xFF, continuation bit set).
		if *offset >= len(body) {
			return nil, fmt.Errorf("insufficient data for i8")
		}
		val := int8(body[*offset])
		*offset++
		return val, nil
	case "i16":
		val, err := decodeViewVarUint(body, offset)
		return int16(val), err
	case "i32":
		val, err := decodeViewVarUint(body, offset)
		return int32(val), err
	case "i64":
		val, err := decodeViewVarUint(body, offset)
		return int64(val), err
	case "bytes":
		// bytes: [len(varint)] + data
		length, err := decodeViewVarUint(body, offset)
		if err != nil {
			return nil, err
		}

		if *offset+int(length) > len(body) {
			return nil, fmt.Errorf("insufficient data for bytes")
		}

		byteList := make([]byte, length)
		copy(byteList, body[*offset:*offset+int(length)])
		*offset += int(length)
		return byteList, nil
	case "B96":
		return deserializeFixedBytesValue(body, offset, 12, "B96")
	case "B144":
		return deserializeFixedBytesValue(body, offset, 18, "B144")
	case "B160":
		return deserializeFixedBytesValue(body, offset, 20, "B160")
	case "B256":
		return deserializeFixedBytesValue(body, offset, 32, "B256")
	default:
		return nil, fmt.Errorf("unsupported IDL type: %s", idlTypeName)
	}
}

// DecodeViewDatas decodes a view response body of Vec<Result<T>>, one Result
// per invocation of the same view method.
func (p *Provider) DecodeViewDatas(instructionName string, body []byte) ([]DecodedTaggedValue, error) {
	instruction, err := p.GetInstructionByName(instructionName)
	if err != nil {
		return nil, err
	}

	if instruction.Kind != "view" {
		return nil, fmt.Errorf("%s is not a view instruction (kind=%s)", instructionName, instruction.Kind)
	}

	returnType := strings.TrimSpace(instruction.Returns.Type)
	if returnType == "" {
		return nil, fmt.Errorf("IDL view method %s is missing returns.type", instructionName)
	}

	offset := 0
	// 1. vec length (result count)
	resultCount, err := decodeViewVarUint(body, &offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode result count: %w", err)
	}

	results := make([]DecodedTaggedValue, resultCount)

	// 2. each Result item
	for i := uint64(0); i < resultCount; i++ {
		value, err := decodeViewResultItem(p, returnType, body, &offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode result[%d]: %w", i, err)
		}
		results[i] = value
	}

	if offset != len(body) {
		return nil, fmt.Errorf("%d trailing bytes after decoding", len(body)-offset)
	}

	return results, nil
}

// decodeViewResultItem decodes one Result<T> entry: variant 0 = Ok(value bytes),
// 1 = Err(TxFailurePayload).
func decodeViewResultItem(pd *Provider, returnType string, body []byte, offset *int) (DecodedTaggedValue, error) {
	variantIndex, err := decodeViewVarUint(body, offset)
	if err != nil {
		return DecodedTaggedValue{}, fmt.Errorf("failed to decode result variant: %w", err)
	}

	if variantIndex == 0 {
		okDataLen, err := decodeViewVarUint(body, offset)
		if err != nil {
			return DecodedTaggedValue{}, fmt.Errorf("failed to decode Ok data length: %w", err)
		}

		if *offset+int(okDataLen) > len(body) {
			return DecodedTaggedValue{}, fmt.Errorf("insufficient data for Ok payload")
		}
		okData := make([]byte, okDataLen)
		copy(okData, body[*offset:*offset+int(okDataLen)])
		*offset += int(okDataLen)

		valueOffset := 0
		value, err := pd.deserializeValue(returnType, okData, &valueOffset)
		if err != nil {
			return DecodedTaggedValue{}, fmt.Errorf("failed to deserialize Ok value: %w", err)
		}
		// Ok payload must be fully consumed
		if valueOffset != len(okData) {
			return DecodedTaggedValue{}, fmt.Errorf("%d trailing bytes after decoding Ok value", len(okData)-valueOffset)
		}

		return DecodedTaggedValue{Value: value}, nil
	}

	if variantIndex == 1 {
		failure, err := pd.decodeTxFailurePayload(body, offset)
		if err != nil {
			return DecodedTaggedValue{}, fmt.Errorf("failed to decode Err payload: %w", err)
		}

		return DecodedTaggedValue{Value: failure}, nil
	}

	return DecodedTaggedValue{}, fmt.Errorf("invalid result variant index: %d", variantIndex)
}

func (p *Provider) decodeTxFailurePayload(body []byte, offset *int) (*api.TxFailurePayload, error) {
	codeRaw, err := decodeViewVarUint(body, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode failure code: %w", err)
	}
	code := uint16(codeRaw)

	messageLen, err := decodeViewVarUint(body, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode message length: %w", err)
	}
	if *offset+int(messageLen) > len(body) {
		return nil, fmt.Errorf("insufficient data for failure message")
	}
	message := string(body[*offset : *offset+int(messageLen)])
	*offset += int(messageLen)

	dataLen, err := decodeViewVarUint(body, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode data length: %w", err)
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

// decodeViewVarUint decodes a varint (uint64, at most 10 bytes).
func decodeViewVarUint(input []byte, offset *int) (uint64, error) {
	var value uint64
	var shift uint

	for i := 0; i < 10; i++ {
		if *offset >= len(input) {
			return 0, fmt.Errorf("unexpected end of input")
		}

		b := input[*offset]
		*offset++

		value |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, nil
		}

		shift += 7
	}

	return 0, fmt.Errorf("varint is too long")
}

// decodeViewVarUint128 decodes a varint into a big.Int (u128: at most 19 bytes).
func decodeViewVarUint128(input []byte, offset *int) (*big.Int, error) {
	var buf [19]byte
	n := 0
	for ; n < len(buf); n++ {
		if *offset >= len(input) {
			return nil, fmt.Errorf("unexpected end of input")
		}
		b := input[*offset]
		*offset++
		buf[n] = b & 0x7f
		if b&0x80 == 0 {
			break
		}
	}
	if n == len(buf) {
		return nil, fmt.Errorf("varint is too long")
	}

	// assemble little-endian 7-bit groups with reused scratch values
	value := new(big.Int)
	group := new(big.Int)
	shifted := new(big.Int)
	for i := 0; i <= n; i++ {
		value.Or(value, shifted.Lsh(group.SetInt64(int64(buf[i])), uint(i*7)))
	}
	return value, nil
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

	// Fast path: length guard + suffix check, then case-insensitive prefix match
	if len(argName) < len(prefix)+1 || !strings.HasSuffix(argName, ">") {
		return "", false
	}
	if !strings.EqualFold(argName[:len(prefix)], prefix) {
		return "", false
	}

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

	parts := splitTopLevel(inner, ',')

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
	case uint8:
		return big.NewInt(int64(typed)), nil
	case uint16:
		return big.NewInt(int64(typed)), nil
	case uint32:
		return big.NewInt(int64(typed)), nil
	case uint64:
		return new(big.Int).SetUint64(typed), nil
	case uint:
		return new(big.Int).SetUint64(uint64(typed)), nil
	case int:
		if !signed && typed < 0 {
			return nil, fmt.Errorf("u128 out of range")
		}
		return big.NewInt(int64(typed)), nil
	case int8:
		if !signed && typed < 0 {
			return nil, fmt.Errorf("u128 out of range")
		}
		return big.NewInt(int64(typed)), nil
	case int16:
		if !signed && typed < 0 {
			return nil, fmt.Errorf("u128 out of range")
		}
		return big.NewInt(int64(typed)), nil
	case int32:
		if !signed && typed < 0 {
			return nil, fmt.Errorf("u128 out of range")
		}
		return big.NewInt(int64(typed)), nil
	case int64:
		if !signed && typed < 0 {
			return nil, fmt.Errorf("u128 out of range")
		}
		return big.NewInt(typed), nil
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

// serializeFixedBytesValue writes a fixed-size byte value (B96/B144/B160/B256);
// accepts []byte or the matching fixed-size array type.
func serializeFixedBytesValue(serializer *postcard.Serializer, value any, size int, typeName string) error {
	switch typed := value.(type) {
	case []byte:
		if len(typed) != size {
			return fmt.Errorf("%s expects exactly %d bytes, got %d", typeName, size, len(typed))
		}
		serializer.SerializeFixedBytes(typed)
		return nil
	case [12]byte:
		if size != 12 {
			return fmt.Errorf("%s expects %d bytes", typeName, size)
		}
		serializer.SerializeFixedBytes(typed[:])
		return nil
	case [18]byte:
		if size != 18 {
			return fmt.Errorf("%s expects %d bytes", typeName, size)
		}
		serializer.SerializeFixedBytes(typed[:])
		return nil
	case [20]byte:
		if size != 20 {
			return fmt.Errorf("%s expects %d bytes", typeName, size)
		}
		serializer.SerializeFixedBytes(typed[:])
		return nil
	case [32]byte:
		if size != 32 {
			return fmt.Errorf("%s expects %d bytes", typeName, size)
		}
		serializer.SerializeFixedBytes(typed[:])
		return nil
	default:
		return fmt.Errorf("%s expects [%d]byte or []byte", typeName, size)
	}
}

// deserializeFixedBytesValue reads exactly size bytes (B96/B144/B160/B256) and
// returns the matching fixed-size array: [12]byte/[18]byte/[20]byte/[32]byte.
func deserializeFixedBytesValue(body []byte, offset *int, size int, typeName string) (any, error) {
	if *offset+size > len(body) {
		return nil, fmt.Errorf("insufficient data for %s", typeName)
	}
	switch size {
	case 12:
		var buf [12]byte
		copy(buf[:], body[*offset:*offset+12])
		*offset += 12
		return buf, nil
	case 18:
		var buf [18]byte
		copy(buf[:], body[*offset:*offset+18])
		*offset += 18
		return buf, nil
	case 20:
		var buf [20]byte
		copy(buf[:], body[*offset:*offset+20])
		*offset += 20
		return buf, nil
	case 32:
		var buf [32]byte
		copy(buf[:], body[*offset:*offset+32])
		*offset += 32
		return buf, nil
	default:
		return nil, fmt.Errorf("unsupported fixed byte size %d for %s", size, typeName)
	}
}
