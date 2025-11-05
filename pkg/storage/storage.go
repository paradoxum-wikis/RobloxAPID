package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Save adds a timestamp to the data and saves it to a file, returning the timestamped data
func Save(path string, data []byte) ([]byte, error) {
	var dataMap map[string]interface{}
	if err := json.Unmarshal(data, &dataMap); err != nil {
		return nil, err
	}

	dataMap["lastUpdated"] = time.Now().UTC().Format(time.RFC3339)

	dataToSave, err := json.MarshalIndent(dataMap, "", "  ")
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join("data", path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}

	err = os.WriteFile(fullPath, dataToSave, 0644)
	if err != nil {
		return nil, err
	}

	return dataToSave, nil
}
