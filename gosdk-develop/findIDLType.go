package milon

import "github.com/milon-labs/milon-go-sdk/provider"

// FindIDLTypeByTag locates the provider and IDL type that describe a persisted
// value's type_tag.
//
// Resource tags (declared in the IDL resources section) are checked first: a
// resource may name a type the IDL does not declare in its types section (e.g.
// system _B256 uses the B256 builtin), so that case falls back to decoding by
// name. The types index is the fallback for values that are not declared as
// resources, such as the bare builtin tags written by genesis transactions.
func (client *Client) FindIDLTypeByTag(typeTag uint64) (*provider.Provider, *provider.IDLType) {
	for _, pd := range client.GetAllPd() {
		if resource, ok := pd.GetResourceByTypeTag(typeTag); ok {
			if idlType, ok := pd.IDLTypeByName[resource.Type]; ok {
				return pd, &idlType
			}
			return pd, &provider.IDLType{Kind: "builtin", Name: resource.Type}
		}
	}
	for _, pd := range client.GetAllPd() {
		if idlType, ok := pd.GetIDLTypeByTypeTag(typeTag); ok {
			return pd, idlType
		}
	}
	return nil, nil
}
