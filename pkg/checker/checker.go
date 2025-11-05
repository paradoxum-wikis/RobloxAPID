package checker

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// checks if new data differs from stored data, ignoring 'lastUpdated'
func HasChanged(path string, newData []byte) (bool, error) {
	fullPath := filepath.Join("data", path)
	oldData, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}

	var oldDataMap map[string]interface{}
	if err := json.Unmarshal(oldData, &oldDataMap); err != nil {
		return !bytes.Equal(oldData, newData), nil
	}
	delete(oldDataMap, "lastUpdated")

	oldAPIData, err := json.Marshal(oldDataMap)
	if err != nil {
		return false, err
	}

	var newAPIDataMap map[string]interface{}
	if err := json.Unmarshal(newData, &newAPIDataMap); err != nil {
		return !bytes.Equal(oldData, newData), nil
	}
	newAPIData, err := json.Marshal(newAPIDataMap)
	if err != nil {
		return false, err
	}

	return !bytes.Equal(oldAPIData, newAPIData), nil
}
