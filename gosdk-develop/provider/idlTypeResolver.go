package provider

import (
	"fmt"
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

	once                      sync.Once
	providerByResourceTypeTag map[uint64]*Provider // resource typeTag -> Provider
	providerByTypeTag         map[uint64]*Provider // IDL type typeTag -> Provider (types section)
	providerByEventTypeTag    map[uint64]*Provider // event typeTag -> Provider
}

// buildIndexes fills the typeTag -> Provider maps used for O(1) lookups. The
// construction is shared with IDLRegistry via buildTypeTagIndexes so both
// resolve a tag to the identical provider. Event collisions are tolerated
// (first-wins) here rather than rejected, matching the resolver's read-only
// role; on the strictEvent=false path buildTypeTagIndexes never errors.
func (r *IDLTypeResolver) buildIndexes() {
	indexes, _ := buildTypeTagIndexes(r.Providers, false)
	r.providerByResourceTypeTag = indexes.byResource
	r.providerByTypeTag = indexes.byType
	r.providerByEventTypeTag = indexes.byEvent
}

// DecodeResource returns the consumed bytes of the persisted value registered
// under typeTag plus the remaining bytes. Resource declarations win; when the
// tag is not declared as a resource the IDL types section is used as fallback,
// mirroring the Rust SDK which also registers decoders for builtin and legacy
// type tags.
func (r *IDLTypeResolver) DecodeResource(typeTag uint64, bytes []byte) (valueBytes []byte, remaining []byte, err error) {
	r.once.Do(r.buildIndexes)

	if targetProvider := r.providerByResourceTypeTag[typeTag]; targetProvider != nil {
		// Indexed from this provider's own ResourceByTypeTag, so the lookup
		// cannot miss; ok is intentionally ignored.
		targetResource, _ := targetProvider.GetResourceByTypeTag(typeTag)

		offset := 0
		if _, err = targetProvider.deserializeValue(targetResource.Type, bytes, &offset); err != nil {
			return nil, bytes, fmt.Errorf("deserialize resource %s (%s) failed: %w", targetResource.Name, targetResource.Type, err)
		}

		return bytes[:offset], bytes[offset:], nil
	}

	if targetProvider := r.providerByTypeTag[typeTag]; targetProvider != nil {
		// Indexed from this provider's own IDLTypeByTypeTag, so the lookup
		// cannot miss; ok is intentionally ignored.
		targetType, _ := targetProvider.GetIDLTypeByTypeTag(typeTag)

		offset := 0
		if _, err = targetProvider.deserializeValue(targetType.Name, bytes, &offset); err != nil {
			return nil, bytes, fmt.Errorf("deserialize type %s failed: %w", targetType.Name, err)
		}

		return bytes[:offset], bytes[offset:], nil
	}

	return nil, bytes, fmt.Errorf("unknown resource type_tag %d (not found in any loaded IDL)", typeTag)
}

// DecodeEvent returns the consumed bytes of the event registered under
// typeTag plus the remaining bytes.
func (r *IDLTypeResolver) DecodeEvent(typeTag uint64, bytes []byte) (eventBytes []byte, remaining []byte, err error) {
	r.once.Do(r.buildIndexes)

	targetProvider := r.providerByEventTypeTag[typeTag]
	if targetProvider == nil {
		return nil, nil, fmt.Errorf("unknown event type_tag %d (not found in any loaded IDL)", typeTag)
	}
	// Indexed from this provider's own EventByTypeTag, so the lookup cannot
	// miss; ok is intentionally ignored.
	targetEvent, _ := targetProvider.GetEventByTypeTag(typeTag)

	offset := 0
	for _, field := range targetEvent.Fields {
		if _, err = targetProvider.deserializeValue(field.Type, bytes, &offset); err != nil {
			return nil, nil, fmt.Errorf("deserialize event field %s (%s) failed: %w", field.Name, field.Type, err)
		}
	}

	return bytes[:offset], bytes[offset:], nil
}
