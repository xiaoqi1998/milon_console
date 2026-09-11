package handler

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"

	"github.com/milon-labs/milon-go-sdk/provider"
)

// CoerceArgsForEncode preprocesses HTTP-JSON-bound provider.Args so that values
// match what the SDK's provider.Encode expects after the removal of
// coerceBytesFromJSON. Specifically:
//   - "bytes" typed args: hex string (with/without 0x) or JSON number array → []byte
//   - "Signature" typed args: JSON number array → []byte (string is still accepted)
//
// The coercion walks the IDL type tree recursively (vec, option, map, tuple,
// struct, enum) so nested bytes/Signature fields are also handled.
func CoerceArgsForEncode(pd *provider.Provider, methodName string, args provider.Args) (provider.Args, error) {
	instruction, err := pd.GetInstructionByName(methodName)
	if err != nil {
		// Let Encode produce the canonical error later.
		return args, nil
	}

	out := make(provider.Args, len(args))
	for k, v := range args {
		out[k] = v
	}

	for _, arg := range instruction.Args {
		val, exists := out[arg.Name]
		if !exists {
			continue
		}
		coerced, err := coerceValue(pd, arg.Type, val)
		if err != nil {
			return nil, fmt.Errorf("arg %q (%s): %w", arg.Name, arg.Type, err)
		}
		out[arg.Name] = coerced
	}

	return out, nil
}

// coerceValue recursively coerces a value according to its IDL type name.
func coerceValue(pd *provider.Provider, typeName string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}

	// vec<T>
	if inner, ok := parseWrapped(typeName, "vec"); ok {
		items, err := toSlice(value)
		if err != nil {
			return nil, fmt.Errorf("expects an array: %w", err)
		}
		out := make([]any, len(items))
		for i, item := range items {
			coerced, err := coerceValue(pd, inner, item)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			out[i] = coerced
		}
		return out, nil
	}

	// option<T>
	if inner, ok := parseWrapped(typeName, "option"); ok {
		if isNilValue(value) {
			return nil, nil
		}
		return coerceValue(pd, inner, value)
	}

	// map<K,V>
	if keyType, valType, ok := parseMap(typeName); ok {
		entries, err := toMapEntries(value)
		if err != nil {
			return nil, fmt.Errorf("expects a map: %w", err)
		}
		out := make(map[string]any, len(entries))
		for k, v := range entries {
			ck, err := coerceValue(pd, keyType, k)
			if err != nil {
				return nil, fmt.Errorf("map key: %w", err)
			}
			cv, err := coerceValue(pd, valType, v)
			if err != nil {
				return nil, fmt.Errorf("map value: %w", err)
			}
			ks, ok := ck.(string)
			if !ok {
				ks = fmt.Sprint(ck)
			}
			out[ks] = cv
		}
		return out, nil
	}

	// tuple<T1,T2,...>
	if tupleTypes, ok := parseTuple(typeName); ok {
		items, err := toSlice(value)
		if err != nil {
			// tuple may also be passed as map {"0": v0, "1": v1, ...}
			if record, isMap := value.(map[string]any); isMap {
				out := make([]any, len(tupleTypes))
				for i, t := range tupleTypes {
					fv, exists := record[fmt.Sprintf("%d", i)]
					if !exists {
						return nil, fmt.Errorf("missing tuple element %d", i)
					}
					coerced, err := coerceValue(pd, t, fv)
					if err != nil {
						return nil, fmt.Errorf("tuple[%d]: %w", i, err)
					}
					out[i] = coerced
				}
				return out, nil
			}
			return nil, fmt.Errorf("expects an array: %w", err)
		}
		if len(items) != len(tupleTypes) {
			return nil, fmt.Errorf("tuple expects %d values, got %d", len(tupleTypes), len(items))
		}
		out := make([]any, len(items))
		for i, item := range items {
			coerced, err := coerceValue(pd, tupleTypes[i], item)
			if err != nil {
				return nil, fmt.Errorf("tuple[%d]: %w", i, err)
			}
			out[i] = coerced
		}
		return out, nil
	}

	// Custom IDL type (struct / enum / builtin)
	if idlType, ok := pd.IDLTypeByName[typeName]; ok {
		switch idlType.Kind {
		case "struct":
			record, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s expects an object", typeName)
			}
			out := make(map[string]any, len(record))
			for k, v := range record {
				out[k] = v
			}
			for _, field := range idlType.Fields {
				fv, exists := out[field.Name]
				if !exists {
					continue
				}
				coerced, err := coerceValue(pd, field.Type, fv)
				if err != nil {
					return nil, fmt.Errorf("field %q: %w", field.Name, err)
				}
				out[field.Name] = coerced
			}
			return out, nil

		case "enum":
			return coerceEnumValue(pd, idlType, value)

		case "builtin":
			// Fall through to primitive handling below.
		}
	}

	// Primitive types that need coercion.
	switch typeName {
	case "bytes":
		return coerceBytes(value)
	case "B96", "B144", "B160", "B256":
		// Fixed-size byte types: coerce hex string / number array to []byte;
		// the SDK's serializer enforces the exact length afterwards.
		return coerceBytes(value)
	case "Signature":
		return coerceSignature(value)
	}

	// Everything else passes through unchanged.
	return value, nil
}

// coerceEnumValue handles enum variant value coercion.
func coerceEnumValue(pd *provider.Provider, idlType provider.IDLType, value any) (any, error) {
	// Enum passed as plain string (unit variant, no data).
	if _, ok := value.(string); ok {
		return value, nil
	}

	record, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}

	// Determine variant name and inner value.
	var variantName string
	var variantValue any
	var valueKey string // which key holds the variant data

	if v, ok := record["variant"].(string); ok {
		variantName = v
		if inner, ok := record["value"]; ok {
			variantValue = inner
			valueKey = "value"
		} else if inner, ok := record["fields"]; ok {
			variantValue = inner
			valueKey = "fields"
		} else {
			variantValue = map[string]any{}
			valueKey = ""
		}
	} else if len(record) == 1 {
		for k, v := range record {
			variantName = k
			variantValue = v
			valueKey = k
		}
	} else {
		return value, nil
	}

	// Find the variant definition.
	var variant *provider.EnumVariant
	for i := range idlType.Variants {
		if strings.EqualFold(idlType.Variants[i].Name, variantName) {
			variant = &idlType.Variants[i]
			break
		}
	}
	if variant == nil || variant.Kind == "unit" {
		return value, nil
	}

	// Coerce variant fields.
	if variant.Kind == "tuple" {
		items, err := toSlice(variantValue)
		if err != nil {
			return value, nil
		}
		out := make([]any, len(items))
		for i, item := range items {
			if i < len(variant.Fields) {
				coerced, err := coerceValue(pd, variant.Fields[i].Type, item)
				if err != nil {
					return nil, fmt.Errorf("enum %s.%s[%d]: %w", idlType.Name, variantName, i, err)
				}
				out[i] = coerced
			} else {
				out[i] = item
			}
		}
		result := make(map[string]any, len(record))
		for k, v := range record {
			result[k] = v
		}
		if valueKey != "" {
			result[valueKey] = out
		}
		return result, nil
	}

	// Struct-like variant: coerce named fields.
	fieldRecord, ok := variantValue.(map[string]any)
	if !ok {
		return value, nil
	}
	out := make(map[string]any, len(fieldRecord))
	for k, v := range fieldRecord {
		out[k] = v
	}
	for _, field := range variant.Fields {
		fv, exists := out[field.Name]
		if !exists {
			continue
		}
		coerced, err := coerceValue(pd, field.Type, fv)
		if err != nil {
			return nil, fmt.Errorf("enum %s.%s field %q: %w", idlType.Name, variantName, field.Name, err)
		}
		out[field.Name] = coerced
	}
	result := make(map[string]any, len(record))
	for k, v := range record {
		result[k] = v
	}
	if valueKey != "" {
		result[valueKey] = out
	}
	return result, nil
}

// coerceBytes converts JSON-friendly byte representations to []byte.
// Accepts: []byte (passthrough), hex string (with/without 0x prefix),
// JSON number array ([]any of float64/int).
func coerceBytes(value any) (any, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		s := strings.TrimSpace(v)
		s = strings.TrimPrefix(s, "0x")
		s = strings.TrimPrefix(s, "0X")
		buf, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid hex string for bytes: %w", err)
		}
		return buf, nil
	case []any:
		buf := make([]byte, 0, len(v))
		for i, item := range v {
			n, err := toUint64(item)
			if err != nil {
				return nil, fmt.Errorf("bytes[%d]: %w", i, err)
			}
			if n > 255 {
				return nil, fmt.Errorf("bytes[%d]: value %d exceeds 255", i, n)
			}
			buf = append(buf, byte(n))
		}
		return buf, nil
	default:
		// Try reflection for other slice types (e.g. []float64 from some JSON libs).
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			buf := make([]byte, 0, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				n, err := toUint64(rv.Index(i).Interface())
				if err != nil {
					return nil, fmt.Errorf("bytes[%d]: %w", i, err)
				}
				if n > 255 {
					return nil, fmt.Errorf("bytes[%d]: value %d exceeds 255", i, n)
				}
				buf = append(buf, byte(n))
			}
			return buf, nil
		}
		return nil, fmt.Errorf("bytes expects a []byte, hex string, or number array, got %T", value)
	}
}

// coerceSignature converts JSON number arrays to []byte for Signature args.
// Strings are left as-is (the new SDK's serializeSignature accepts string via
// NewSignatureFromStringRelaxed). []byte passes through.
func coerceSignature(value any) (any, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		// New SDK accepts string (hex or base58) directly.
		return v, nil
	case []any:
		buf := make([]byte, 0, len(v))
		for i, item := range v {
			n, err := toUint64(item)
			if err != nil {
				return nil, fmt.Errorf("Signature[%d]: %w", i, err)
			}
			if n > 255 {
				return nil, fmt.Errorf("Signature[%d]: value %d exceeds 255", i, n)
			}
			buf = append(buf, byte(n))
		}
		return buf, nil
	default:
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			buf := make([]byte, 0, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				n, err := toUint64(rv.Index(i).Interface())
				if err != nil {
					return nil, fmt.Errorf("Signature[%d]: %w", i, err)
				}
				if n > 255 {
					return nil, fmt.Errorf("Signature[%d]: value %d exceeds 255", i, n)
				}
				buf = append(buf, byte(n))
			}
			return buf, nil
		}
		// crypto.Signature or other types pass through.
		return value, nil
	}
}

// --- type string parsing helpers (mirroring unexported provider functions) ---

func parseWrapped(typeName, wrapper string) (string, bool) {
	prefix := wrapper + "<"
	if len(typeName) < len(prefix)+1 || !strings.HasSuffix(typeName, ">") {
		return "", false
	}
	if !strings.EqualFold(typeName[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(typeName[len(prefix) : len(typeName)-1]), true
}

func parseMap(typeName string) (string, string, bool) {
	inner, ok := parseWrapped(typeName, "map")
	if !ok {
		return "", "", false
	}
	parts := splitTopLevel(inner, ',')
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func parseTuple(typeName string) ([]string, bool) {
	inner, ok := parseWrapped(typeName, "tuple")
	if !ok {
		return nil, false
	}
	parts := splitTopLevel(inner, ',')
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, true
}

func splitTopLevel(s string, sep rune) []string {
	var parts []string
	depth := 0
	start := 0
	for i, ch := range s {
		switch ch {
		case '<', '(', '[':
			depth++
		case '>', ')', ']':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// --- value shape helpers ---

func toSlice(value any) ([]any, error) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil, fmt.Errorf("nil value")
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, fmt.Errorf("not a slice: %T", value)
	}
	out := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = rv.Index(i).Interface()
	}
	return out, nil
}

func toMapEntries(value any) (map[string]any, error) {
	if m, ok := value.(map[string]any); ok {
		return m, nil
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Map {
		out := make(map[string]any, rv.Len())
		for _, k := range rv.MapKeys() {
			out[fmt.Sprint(k.Interface())] = rv.MapIndex(k).Interface()
		}
		return out, nil
	}
	return nil, fmt.Errorf("not a map: %T", value)
}

func toUint64(v any) (uint64, error) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, fmt.Errorf("negative value %v", n)
		}
		return uint64(n), nil
	case float32:
		if n < 0 {
			return 0, fmt.Errorf("negative value %v", n)
		}
		return uint64(n), nil
	case int:
		if n < 0 {
			return 0, fmt.Errorf("negative value %v", n)
		}
		return uint64(n), nil
	case int8:
		if n < 0 {
			return 0, fmt.Errorf("negative value %v", n)
		}
		return uint64(n), nil
	case int16:
		if n < 0 {
			return 0, fmt.Errorf("negative value %v", n)
		}
		return uint64(n), nil
	case int32:
		if n < 0 {
			return 0, fmt.Errorf("negative value %v", n)
		}
		return uint64(n), nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("negative value %v", n)
		}
		return uint64(n), nil
	case uint:
		return uint64(n), nil
	case uint8:
		return uint64(n), nil
	case uint16:
		return uint64(n), nil
	case uint32:
		return uint64(n), nil
	case uint64:
		return n, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to number", v)
	}
}

func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

// encodeWithCoercion coerces HTTP-JSON args then encodes the instruction.
// Use this instead of pd.Encode when args originate from an HTTP request body.
func encodeWithCoercion(pd *provider.Provider, methodName string, args provider.Args) ([]byte, error) {
	coerced, err := CoerceArgsForEncode(pd, methodName, args)
	if err != nil {
		return nil, err
	}
	return pd.Encode(methodName, coerced)
}
