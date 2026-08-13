package provider

import "fmt"

// IDLTypeResolver resolves type_tag values into raw byte ranges by reusing the
// provider's IDL-driven deserializer. It is used by the api package to decode
// resources and events without knowing the concrete Go types in advance.
type IDLTypeResolver struct {
	Providers map[string]*Provider
}

// DecodeResource consumes the bytes of the value registered under typeTag and
// returns the consumed slice plus the remaining bytes.
func (r *IDLTypeResolver) DecodeResource(typeTag uint64, bytes []byte) (valueBytes []byte, remaining []byte, err error) {
	// Find the provider that registered this type_tag
	var targetProvider *Provider
	var targetIDLType *IDLType

	for _, pd := range r.Providers {
		if idlType, ok := pd.GetIDLTypeByTypeTag(typeTag); ok {
			targetProvider = pd
			targetIDLType = idlType
			break
		}
	}

	if targetProvider == nil || targetIDLType == nil {
		// Type tag definition not found; report error to help user check the IDL
		return nil, bytes, fmt.Errorf("unknown resource type_tag %d (not found in any loaded IDL)", typeTag)
	}

	// Reuse the provider deserializer to compute the consumed byte range
	offset := 0
	if _, err = targetProvider.deserializeValue(targetIDLType.Name, bytes, &offset); err != nil {
		return nil, bytes, fmt.Errorf("deserialize %s failed: %w", targetIDLType.Name, err)
	}

	return bytes[:offset], bytes[offset:], nil
}

// DecodeEvent consumes the bytes of the event registered under typeTag and
// returns the consumed slice plus the remaining bytes.
func (r *IDLTypeResolver) DecodeEvent(typeTag uint64, bytes []byte) (eventBytes []byte, remaining []byte, err error) {
	// Find the provider that registered this event type_tag
	var targetProvider *Provider
	var targetEvent *Event

	for _, pd := range r.Providers {
		if event, ok := pd.GetEventByTypeTag(typeTag); ok {
			targetProvider = pd
			targetEvent = event
			break
		}
	}

	if targetProvider == nil || targetEvent == nil {
		// Event definition not found; report error to help user check the IDL
		return nil, nil, fmt.Errorf("unknown event type_tag %d (not found in any loaded IDL)", typeTag)
	}

	// Reuse the provider deserializer to consume each field in order
	offset := 0
	for _, field := range targetEvent.Fields {
		if _, err = targetProvider.deserializeValue(field.Type, bytes, &offset); err != nil {
			return nil, nil, fmt.Errorf("deserialize event field %s (%s) failed: %w", field.Name, field.Type, err)
		}
	}

	return bytes[:offset], bytes[offset:], nil
}
