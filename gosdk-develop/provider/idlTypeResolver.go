package provider

import (
	"fmt"
	"sort"
	"sync"
)

// IDLTypeResolver resolves typeTag values into raw byte ranges using the
// provider's IDL-driven deserializer, so the api package can decode resources
// and events without knowing concrete Go types in advance.
type IDLTypeResolver struct {
	// Providers maps IDL name -> Provider. It must be fully populated before
	// the first decode call and must not be mutated afterwards; the typeTag
	// indexes are built lazily on first use (thread-safe).
	Providers map[string]*Provider

	once                   sync.Once
	providerByTypeTag      map[uint64]*Provider // resource typeTag -> Provider
	providerByEventTypeTag map[uint64]*Provider // event typeTag -> Provider
}

// buildIndexes precomputes typeTag -> Provider maps for O(1) lookups.
// On collisions the first registered provider wins (deterministic).
func (r *IDLTypeResolver) buildIndexes() {
	r.providerByTypeTag = make(map[uint64]*Provider)
	r.providerByEventTypeTag = make(map[uint64]*Provider)
	names := make([]string, 0, len(r.Providers))
	for name := range r.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pd := r.Providers[name]
		for typeTag := range pd.IDLTypeByTypeTag {
			if _, ok := r.providerByTypeTag[typeTag]; !ok {
				r.providerByTypeTag[typeTag] = pd
			}
		}
		for typeTag := range pd.EventByTypeTag {
			if _, ok := r.providerByEventTypeTag[typeTag]; !ok {
				r.providerByEventTypeTag[typeTag] = pd
			}
		}
	}
}

// DecodeResource returns the consumed bytes of the value registered under
// typeTag plus the remaining bytes.
func (r *IDLTypeResolver) DecodeResource(typeTag uint64, bytes []byte) (valueBytes []byte, remaining []byte, err error) {
	r.once.Do(r.buildIndexes)

	targetProvider := r.providerByTypeTag[typeTag]
	if targetProvider == nil {
		return nil, bytes, fmt.Errorf("unknown resource type_tag %d (not found in any loaded IDL)", typeTag)
	}
	targetIDLType, _ := targetProvider.GetIDLTypeByTypeTag(typeTag)

	offset := 0
	if _, err = targetProvider.deserializeValue(targetIDLType.Name, bytes, &offset); err != nil {
		return nil, bytes, fmt.Errorf("deserialize %s failed: %w", targetIDLType.Name, err)
	}

	return bytes[:offset], bytes[offset:], nil
}

// DecodeEvent returns the consumed bytes of the event registered under
// typeTag plus the remaining bytes.
func (r *IDLTypeResolver) DecodeEvent(typeTag uint64, bytes []byte) (eventBytes []byte, remaining []byte, err error) {
	r.once.Do(r.buildIndexes)

	targetProvider := r.providerByEventTypeTag[typeTag]
	if targetProvider == nil {
		return nil, nil, fmt.Errorf("unknown event type_tag %d (not found in any loaded IDL)", typeTag)
	}
	targetEvent, _ := targetProvider.GetEventByTypeTag(typeTag)

	offset := 0
	for _, field := range targetEvent.Fields {
		if _, err = targetProvider.deserializeValue(field.Type, bytes, &offset); err != nil {
			return nil, nil, fmt.Errorf("deserialize event field %s (%s) failed: %w", field.Name, field.Type, err)
		}
	}

	return bytes[:offset], bytes[offset:], nil
}
