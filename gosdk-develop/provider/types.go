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
	Kind          string        `json:"kind"`                      // entry + view
	Name          string        `json:"name"`
	Returns       ReturnValue   `json:"returns,omitempty"`         // required for view
	SignerLookups SignerLookups `json:"signer_lookups,omitempty"`  // optional for entry
	Sponsor       bool          `json:"sponsor,omitempty"`         // optional for entry
}

// SignerLookups maps signer role names to lookup configs.
// key: signer role name (e.g. "owner", "freezer")
// value: signer lookup config
type SignerLookups map[string]SignerLookup

// SignerLookup configures the lookup of a single signer.
type SignerLookup struct {
	Path LookupPath `json:"path"` // argument that holds the address
	Res  uint8      `json:"res"`  // resource ID (for access control)
}
type LookupPath struct {
	Arg  string `json:"arg"`  // argument name (e.g. "token")
	Type string `json:"type"` // argument type (e.g. "Address")
}

type Arg struct {
	Name string `json:"name"`
	Role string `json:"role"` // input + signer + any_signer
	Type string `json:"type"` // IDL type name (u8, u16, ...)
}
type ReturnValue struct {
	Type string `json:"type"` // IDL type name (u8, u16, ...)
}

type IDLType struct {
	Fields   []StructField `json:"fields,omitempty"`   // only for Kind=struct
	Variants []EnumVariant `json:"variants,omitempty"` // only for Kind=enum
	Kind     string        `json:"kind"`               // struct | enum | tuple | builtin | unit
	Name     string        `json:"name"`
	TypeTag  uint64        `json:"typeTag"`
}
type StructField struct {
	Name string `json:"name"`
	Type string `json:"type"` // IDL type name (u8, u16, ...)
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

// DecodedTaggedValue is the decoded return value of a ViewMulti method.
type DecodedTaggedValue struct {
	Value any
}

type B96 [12]byte
type B144 [18]byte
type B160 [20]byte
type B256 [32]byte
