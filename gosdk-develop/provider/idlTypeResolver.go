package provider

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"strings"
)

// IDLTypeResolver 基于 IDL 的 type_tag 解析器
type IDLTypeResolver struct {
	Providers map[string]*Provider
}

func (r *IDLTypeResolver) DecodeResource(typeTag uint64, bytes []byte) (valueBytes []byte, remaining []byte, err error) {
	// 遍历所有 provider 查找 type_tag 对应的 IDLType
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
		// 未找到 type_tag 定义，报错提示用户检查 IDL
		return nil, bytes, fmt.Errorf("unknown resource type_tag %d (not found in any loaded IDL)", typeTag)
	}

	// 找到了 IDLType，创建临时 Deserializer 并捕获消耗的字节
	d := postcard.NewDeserializer(bytes)

	// 根据 Kind 处理
	switch targetIDLType.Kind {
	case "builtin":
		captured, err := d.CaptureBytes(func() error {
			return r.deserializeBuiltin(d, targetIDLType.Name)
		})
		if err != nil {
			return nil, bytes, fmt.Errorf("deserialize builtin %s failed: %w", targetIDLType.Name, err)
		}

		return captured, d.Buffer()[d.Offset():], nil
	case "struct", "enum":
		// 对于 struct/enum，需要根据 IDL 定义的字段逐个解析
		captured, err := r.deserializeStructEnum(d, targetIDLType)
		if err != nil {
			return nil, bytes, fmt.Errorf("deserialize struct/enum %s failed: %w", targetIDLType.Name, err)
		}

		return captured, d.Buffer()[d.Offset():], nil
	default:
		return nil, bytes, fmt.Errorf("unsupported IDL type kind: %s", targetIDLType.Kind)
	}
}

func (r *IDLTypeResolver) DecodeEvent(typeTag uint64, bytes []byte) (eventBytes []byte, remaining []byte, err error) {
	// 事件的解码逻辑
	d := postcard.NewDeserializer(bytes)

	// 尝试从 EventByTypeTag 查找事件定义
	var targetEvent *Event

	for _, pd := range r.Providers {
		if event, ok := pd.GetEventByTypeTag(typeTag); ok {
			targetEvent = event
			break
		}
	}

	if targetEvent != nil {
		// 找到了事件定义，按字段顺序精确解析
		captured, err := d.CaptureBytes(func() error {
			for _, field := range targetEvent.Fields {
				if err = r.deserializeField(d, field.Type); err != nil {
					return fmt.Errorf("deserialize event field %s (%s) failed: %w", field.Name, field.Type, err)
				}
			}
			return nil
		})

		if err != nil {
			return nil, bytes, fmt.Errorf("deserialize event %s failed: %w", targetEvent.Name, err)
		}

		return captured, d.Buffer()[d.Offset():], nil
	}

	// 未找到事件定义，返回错误提示用户检查 IDL
	return nil, nil, fmt.Errorf("unknown event type_tag %d (not found in any loaded IDL)", typeTag)
}

// deserializeStructEnum 根据 IDL 定义动态解析 struct/enum
func (r *IDLTypeResolver) deserializeStructEnum(d *postcard.Deserializer, idlType *IDLType) ([]byte, error) {
	return d.CaptureBytes(func() error {
		switch idlType.Kind {
		case "struct":
			// 按顺序解析每个字段
			for _, field := range idlType.Fields {
				if err := r.deserializeField(d, field.Type); err != nil {
					return fmt.Errorf("deserialize struct field %s (%s) failed: %w", field.Name, field.Type, err)
				}
			}
			return nil

		case "enum":
			// enum 需要先读 variant index
			variantIndex, err := d.DeserializeU32()
			if err != nil {
				return fmt.Errorf("deserialize enum variant failed: %w", err)
			}

			// 校验 variant index 范围，越界时返回错误（避免 32 位平台 int 溢出导致静默返回 nil）
			if uint64(variantIndex) >= uint64(len(idlType.Variants)) {
				return fmt.Errorf("enum variant index %d out of range (max %d)", variantIndex, len(idlType.Variants)-1)
			}
			variant := idlType.Variants[variantIndex]
			for _, field := range variant.Fields {
				if err := r.deserializeField(d, field.Type); err != nil {
					return fmt.Errorf("deserialize enum variant %s field %s failed: %w", variant.Name, field.Name, err)
				}
			}
			return nil

		default:
			return fmt.Errorf("unexpected kind: %s", idlType.Kind)
		}
	})
}

// deserializeField 解析单个字段
func (r *IDLTypeResolver) deserializeField(d *postcard.Deserializer, typeName string) error {
	typeName = strings.TrimSpace(typeName)

	// Handle vec<T>
	if inner, ok := parseWrappedType(typeName, "vec"); ok {
		length, err := d.DeserializeU32()
		if err != nil {
			return fmt.Errorf("read vec length failed: %w", err)
		}
		for i := uint32(0); i < length; i++ {
			if err := r.deserializeField(d, inner); err != nil {
				return fmt.Errorf("deserialize vec element[%d] failed: %w", i, err)
			}
		}
		return nil
	}

	// Handle option<T>
	if inner, ok := parseWrappedType(typeName, "option"); ok {
		hasValue, err := d.DeserializeBool()
		if err != nil {
			return fmt.Errorf("read option tag failed: %w", err)
		}
		if hasValue {
			return r.deserializeField(d, inner)
		}
		return nil
	}

	// 先尝试作为 builtin 处理
	if r.isBuiltinType(typeName) {
		return r.deserializeBuiltin(d, typeName)
	}

	// 如果不是 builtin，查找 IDL 定义（可能是 struct/enum）
	for _, pd := range r.Providers {
		if idlType, ok := pd.GetIDLTypeByName(typeName); ok {
			_, err := r.deserializeStructEnum(d, idlType)
			return err
		}
	}

	// 都找不到，报错
	return fmt.Errorf("unknown type: %s", typeName)
}

// isBuiltinType 判断是否为内置类型
func (r *IDLTypeResolver) isBuiltinType(typeName string) bool {
	switch typeName {
	case "Address", "Signer", "String", "string", "PublicKey",
		"bool", "boolean",
		"u8", "u16", "u32", "u64", "Bitmap64",
		"i8", "i16", "i32", "i64",
		"bytes", "B96", "B144", "B160", "B256":
		return true
	default:
		return false
	}
}

func (r *IDLTypeResolver) deserializeBuiltin(d *postcard.Deserializer, typeName string) error {
	switch typeName {
	case "Address", "Signer":
		_, err := d.DeserializeFixedBytes(20)
		return err

	case "PublicKey":
		// PublicKey: [variant(varint)] + [byte data(fixed length)]
		variant, err := d.DeserializeU64()
		if err != nil {
			return fmt.Errorf("read PublicKey variant: %w", err)
		}

		// 根据 variant 确定字节长度
		var expectedLen int
		switch crypto.PublicKeyType(uint32(variant)) {
		case crypto.PublicKeyTypeSecp256k1:
			expectedLen = crypto.PublicKeySecp256k1Size // 33
		case crypto.PublicKeyTypeEd25519:
			expectedLen = crypto.PublicKeyEd25519Size // 32
		case crypto.PublicKeyTypeBLS12381:
			expectedLen = crypto.PublicKeyBLS12381Size // 48
		case crypto.PublicKeyTypeFnDsa512:
			expectedLen = crypto.PublicKeyFnDsa512Size // 897
		default:
			return fmt.Errorf("unknown PublicKey variant: %d", variant)
		}

		_, err = d.DeserializeFixedBytes(expectedLen)
		return err

	case "String", "string":
		_, err := d.DeserializeStr()
		return err

	case "bool", "boolean":
		_, err := d.DeserializeBool()
		return err

	case "u8":
		_, err := d.DeserializeU8()
		return err

	case "u16":
		_, err := d.DeserializeU16()
		return err

	case "u32":
		_, err := d.DeserializeU32()
		return err

	case "u64", "Bitmap64":
		_, err := d.DeserializeU64()
		return err

	case "i8":
		_, err := d.DeserializeI8()
		return err

	case "i16":
		_, err := d.DeserializeI16()
		return err

	case "i32":
		_, err := d.DeserializeI32()
		return err

	case "i64":
		_, err := d.DeserializeI64()
		return err

	case "bytes":
		_, err := d.DeserializeBytes()
		return err

	case "B96":
		_, err := d.DeserializeFixedBytes(12)
		return err

	case "B144":
		_, err := d.DeserializeFixedBytes(18)
		return err

	case "B160":
		_, err := d.DeserializeFixedBytes(20)
		return err

	case "B256":
		_, err := d.DeserializeFixedBytes(32)
		return err

	default:
		return fmt.Errorf("unknown builtin type: %s", typeName)
	}
}
