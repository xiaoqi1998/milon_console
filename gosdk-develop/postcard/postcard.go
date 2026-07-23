package postcard

import "fmt"

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

// DeserializePostcard 从字节数组中反序列化 Postcard 格式数据
//
// 该函数使用提供的反序列化函数从二进制数据中提取结构化数据，
// 并可选择性地验证数据完整性（检查是否存在多余字节）。
//
// 参数说明：
//   - data: 包含 Postcard 编码数据的字节数组
//   - fn: 反序列化函数，用于从 Deserializer 中提取具体类型的值
//   - allowTrailing: 是否允许反序列化完成后存在未消费的尾部字节
//   - false: 严格模式，必须完全消费所有数据，否则返回错误
//   - true: 宽松模式，允许存在多余字节而不报错
//
// 返回值：
//   - T: 反序列化后的目标类型值，出错时返回零值
//   - error: 反序列化过程中的错误信息，成功时为 nil
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

func DeserializeValue[T any](deserializer *Deserializer, fn DeserializerFunc[T]) (T, error) {
	return fn(deserializer)
}

func DeserializeSeq[T any](deserializer *Deserializer, fn DeserializerFunc[T]) ([]T, error) {
	deserializer.depth++
	defer func() { deserializer.depth-- }()
	if deserializer.depth > MaxDepth {
		return nil, fmt.Errorf("exceeded max deserialization depth %d", MaxDepth)
	}

	length, err := deserializer.DeserializeU32()
	if err != nil {
		return nil, err
	}
	if length > MaxSeqLen {
		return nil, fmt.Errorf("sequence length %d exceeds max %d", length, MaxSeqLen)
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
	deserializer.depth++
	defer func() { deserializer.depth-- }()
	if deserializer.depth > MaxDepth {
		return nil, fmt.Errorf("exceeded max deserialization depth %d", MaxDepth)
	}

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
