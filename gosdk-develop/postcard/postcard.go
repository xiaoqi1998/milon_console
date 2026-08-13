package postcard

type Marshaler interface {
	MarshalPostcard(*Serializer) error
}

type Unmarshaler interface {
	UnmarshalPostcard(*Deserializer) error
}

type DeserializerFunc[T any] func(*Deserializer) (T, error)
type SerializerFunc[T any] func(*Serializer, T) error

func SerializePostcard(value Marshaler) ([]byte, error) {
	serializer := NewSerializer()
	if err := value.MarshalPostcard(serializer); err != nil {
		return nil, err
	}
	return serializer.Bytes(), nil
}

func SerializeSeq[T any](serializer *Serializer, values []T, fn SerializerFunc[T]) error {
	if err := serializer.SerializeU32(uint32(len(values))); err != nil {
		return err
	}
	for _, value := range values {
		if err := fn(serializer, value); err != nil {
			return err
		}
	}
	return nil
}

func SerializeOption[T any](serializer *Serializer, value *T, fn SerializerFunc[T]) error {
	if err := serializer.SerializeBool(value != nil); err != nil {
		return err
	}
	if value != nil {
		return fn(serializer, *value)
	}
	return nil
}

// DeserializePostcard deserializes Postcard-encoded data from a byte slice
//
// It uses the provided deserializer function to extract structured data from
// the binary data, and can optionally verify data integrity (checks for trailing bytes).
//
// Parameters:
//   - data: byte slice containing Postcard-encoded data
//   - fn: deserializer function that extracts a value of the concrete type from the Deserializer
//   - allowTrailing: whether unconsumed trailing bytes are allowed after deserialization
//   - false: strict mode, all data must be consumed, otherwise an error is returned
//   - true: lenient mode, trailing bytes are allowed without error
//
// Returns:
//   - T: the deserialized value of the target type, or the zero value on error
//   - error: error during deserialization, nil on success
func DeserializePostcard[T any](data []byte, fn DeserializerFunc[T], allowTrailing bool) (T, error) {
	deserializer := NewDeserializer(data)
	value, err := fn(deserializer)
	if err != nil {
		var zero T
		return zero, err
	}
	if !allowTrailing {
		if err := deserializer.AssertEnd(); err != nil {
			var zero T
			return zero, err
		}
	}
	return value, nil
}

func DeserializeValue[T any](deserializer *Deserializer, fn DeserializerFunc[T]) (T, error) {
	return fn(deserializer)
}

func DeserializeSeq[T any](deserializer *Deserializer, fn DeserializerFunc[T]) ([]T, error) {
	length, err := deserializer.DeserializeU32()
	if err != nil {
		return nil, err
	}
	values := make([]T, 0, length)
	for i := uint32(0); i < length; i++ {
		value, err := fn(deserializer)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func DeserializeOption[T any](deserializer *Deserializer, fn DeserializerFunc[T]) (*T, error) {
	hasValue, err := deserializer.DeserializeBool()
	if err != nil {
		return nil, err
	}
	if !hasValue {
		return nil, nil
	}
	value, err := fn(deserializer)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
