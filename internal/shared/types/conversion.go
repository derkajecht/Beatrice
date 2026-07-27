package types

import "encoding/json"

// ConvertPacketToMap converts a packet struct to a map[string]string
// and returns the map and an error if any
func ConvertPacketToMap(packet any) (map[string]any, error) {
	// marshal packet struct to JSON
	jsonBytes, err := json.Marshal(packet)
	if err != nil {
		return nil, err
	}

	// convert JSON bytes to map[string]any
	var resultMap map[string]any
	err = json.Unmarshal(jsonBytes, &resultMap)
	if err != nil {
		return nil, err
	}
	return resultMap, nil
}
