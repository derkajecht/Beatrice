package types

import "encoding/json"

// ConvertPacketToMap converts a packet struct to a map[string]string
// and returns the map and an error if any
func ConvertPacketToMap(packet any) (map[string]string, error) {
	jsonBytes, err := json.Marshal(packet)
	if err != nil {
		return nil, err
	}

	var resultMap map[string]string
	err = json.Unmarshal(jsonBytes, &resultMap)
	if err != nil {
		return nil, err
	}
	return resultMap, nil
}
