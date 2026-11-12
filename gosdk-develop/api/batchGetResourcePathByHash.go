package api

import "fmt"

type BatchGetResourcePathInfo struct {
	RsHash RsHash
	Path   string // valid when the result is Ok
	ErrMsg string // valid when the result is Err
}

// UnmarshalBatchResourcePathListFromRawList parses a list of (RsHash, Result<String, String>) from raw JSON data.
// rawList: [][]any format, each element is [rsHashBytes([]interface{}), result(map with "Ok"|"Err" key)]
func UnmarshalBatchResourcePathListFromRawList(rawList [][]any) ([]*BatchGetResourcePathInfo, error) {
	batchResourcePaths := make([]*BatchGetResourcePathInfo, 0, len(rawList))
	for _, item := range rawList {
		// Verify the array has at least 2 elements (RsHash byte array + Result)
		if len(item) < 2 {
			return nil, fmt.Errorf("invalid BatchGetResourcePathInfo response")
		}

		// Parse RsHash byte array: JSON number array → Go byte array
		rsHashBytesRaw, ok := item[0].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid BatchGetResourcePathInfo response")
		}

		rsHash, err := UnmarshalRsHashFromJSONArray(rsHashBytesRaw)
		if err != nil {
			return nil, fmt.Errorf("failed to parse RsHash: %w", err)
		}

		// Parse Result<String, String>: externally tagged enum {"Ok": path} or {"Err": message}.
		// A result carrying neither tag is malformed and must not be silently
		// reported as an Ok with an empty path.
		resultMap, ok := item[1].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid BatchGetResourcePathInfo response")
		}

		info := &BatchGetResourcePathInfo{RsHash: rsHash}
		if raw, isOk := resultMap["Ok"]; isOk {
			path, isString := raw.(string)
			if !isString {
				return nil, fmt.Errorf("invalid BatchGetResourcePathInfo Ok path: %T", raw)
			}
			info.Path = path
		} else if raw, isErr := resultMap["Err"]; isErr {
			errMsg, isString := raw.(string)
			if !isString {
				return nil, fmt.Errorf("invalid BatchGetResourcePathInfo Err message: %T", raw)
			}
			info.ErrMsg = errMsg
		} else {
			return nil, fmt.Errorf("invalid BatchGetResourcePathInfo result: neither Ok nor Err present")
		}
		batchResourcePaths = append(batchResourcePaths, info)
	}
	return batchResourcePaths, nil
}
