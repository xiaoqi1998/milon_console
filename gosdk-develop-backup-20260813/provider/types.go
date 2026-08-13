package provider

type IDL struct {
	Metadata     Metadata      `json:"metadata"`
	Instructions []Instruction `json:"instructions"`
	Types        []IDLType     `json:"types"`
	Events       []Event       `json:"events,omitempty"`
	Errors       []ErrorDef    `json:"errors,omitempty"`
	Constants    []Constant    `json:"constants,omitempty"`
}

type Metadata struct {
	AppID       uint8  `json:"app_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Instruction struct {
	Args          []Arg         `json:"args"`
	Discriminator uint16        `json:"discriminator"`
	Handler       string        `json:"handler"`
	Kind          string        `json:"kind"` //entry + view
	Name          string        `json:"name"`
	Returns       *ReturnValue  `json:"returns,omitempty"`        //view必须有----保持指针类型（推荐）
	SignerLookups SignerLookups `json:"signer_lookups,omitempty"` //entry可能有：签名者查找规则
	Sponsor       bool          `json:"sponsor,omitempty"`        //entry可能有
}

// SignerLookups 签名者查找规则映射
// key: 签名者角色名（如 "owner", "freezer"）
// value: 签名者查找配置
type SignerLookups map[string]SignerLookup

// SignerLookup 单个签名者的查找配置
type SignerLookup struct {
	Path LookupPath `json:"path"` // 从哪个参数中查找地址
	Res  uint8      `json:"res"`  // 资源ID（用于访问控制）
}

// LookupPath 地址查找路径
type LookupPath struct {
	Arg  string `json:"arg"`  // 参数名（如 "token"）
	Type string `json:"type"` // 参数类型（如 "Address"）
}

type Arg struct {
	Name string `json:"name"`
	Role string `json:"role"` //input + signer + any_signer
	Type string `json:"type"` //类型 u8 u16 。。。
}
type ReturnValue struct {
	Type string `json:"type"` //类型 u8 u16 。。。
}

type IDLType struct {
	Fields   []StructField `json:"fields,omitempty"`   //Kind=struct才有
	Variants []EnumVariant `json:"variants,omitempty"` //Kind=enum才有
	Kind     string        `json:"kind"`               //struct + enum + tuple +  builtin（内置类型） + unit（少）
	Name     string        `json:"name"`
	TypeTag  uint64        `json:"typeTag"`
}
type StructField struct {
	Name string `json:"name"`
	Type string `json:"type"` //类型 u8 u16 。。。
}
type EnumVariant struct {
	Name   string        `json:"name"`
	Kind   string        `json:"kind"`
	Fields []StructField `json:"fields"`
}

type Event struct {
	Name    string       `json:"name"`
	Fields  []EventField `json:"fields"`
	TypeTag uint64       `json:"typeTag"`
}
type EventField struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Indexed bool   `json:"indexed"`
}

type ErrorDef struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Name    string `json:"name"`
}

type Constant struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type Args map[string]any

// DecodedTaggedValue 解码 ViewMulti 方法的返回值
type DecodedTaggedValue struct {
	Value any
}

/*
vec<u8> → bytes
vec<vec<u8>> → vec<bytes>

pub type B96 = [FixedBytes<12>];

pub type B144 = FixedBytes<18>;

pub type B160 = FixedBytes<20>;

pub type B256 = FixedBytes<32>;
*/
type B96 [12]byte
type B144 [18]byte
type B160 [20]byte
type B256 [32]byte
