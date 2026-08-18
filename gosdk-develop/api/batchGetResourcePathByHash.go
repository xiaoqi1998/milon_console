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

		// Parse Result<String, String>: externally tagged enum {"Ok": path} or {"Err": message}
		resultMap, ok := item[1].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid BatchGetResourcePathInfo response")
		}

		info := &BatchGetResourcePathInfo{RsHash: rsHash}
		if path, ok := resultMap["Ok"].(string); ok {
			info.Path = path
		} else if errMsg, ok := resultMap["Err"].(string); ok {
			info.ErrMsg = errMsg
		}
		batchResourcePaths = append(batchResourcePaths, info)
	}
	return batchResourcePaths, nil
}
