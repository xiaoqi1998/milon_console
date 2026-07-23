package api

import "fmt"

type ListResourcePathInfo struct {
	RsHash RsHash
	Path   string
}

// UnmarshalListResourcePathListFromRawList parses a list of ListResourcePathInfo from raw JSON data
// rawList: [][]any format, each element is [rsHashBytes([]interface{}), path(string)]

func UnmarshalListResourcePathListFromRawList(rawList [][]any) ([]*ListResourcePathInfo, error) {
	listResourcePaths := make([]*ListResourcePathInfo, 0, len(rawList))
	for _, item := range rawList {
		// Verify the array has at least 2 elements (RsHash byte array + Path string)
		if len(item) < 2 {
			return nil, fmt.Errorf("invalid ListResourcePathInfo response")
		}

		// Parse RsHash byte array: JSON number array → Go byte array
		rsHashBytesRaw, ok := item[0].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid ListResourcePathInfo response")
		}

		var rsHash RsHash
		for i, b := range rsHashBytesRaw {
			// Limit to RsHash length (18 bytes)
			if i >= RsHashLen {
				return nil, fmt.Errorf("invalid ListResourcePathInfo response")
			}
			// JSON number is decoded as float64, convert to byte
			val, ok := b.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid ListResourcePathInfo response: RsHash byte at index %d is not a number", i)
			}
			rsHash[i] = byte(val)
		}

		pathStr, ok := item[1].(string)
		if !ok {
			return nil, fmt.Errorf("invalid ListResourcePathInfo response")
		}

		listResourcePaths = append(listResourcePaths, &ListResourcePathInfo{
			RsHash: rsHash,
			Path:   pathStr,
		})
	}

	return listResourcePaths, nil
}
