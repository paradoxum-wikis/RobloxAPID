package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func Save(path string, data []byte) ([]byte, error) {
	var dataMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &dataMap); err != nil {
		return nil, err
	}

	timestampStr := fmt.Sprintf(`"%s"`, time.Now().UTC().Format(time.RFC3339))
	dataMap["roLastUpdated"] = json.RawMessage(timestampStr)

	dataToSave, err := json.MarshalIndent(dataMap, "", "  ")
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join("data", path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(fullPath), filepath.Base(path)+".tmp-*")
	if err != nil {
		return nil, err
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(dataToSave); err != nil {
		return nil, err
	}
	if err := tempFile.Sync(); err != nil {
		return nil, err
	}
	if err := tempFile.Close(); err != nil {
		return nil, err
	}

	if err := os.Rename(tempPath, fullPath); err != nil {
		if removeErr := os.Remove(fullPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, fmt.Errorf("failed to replace %s: %w", fullPath, err)
		}
		if err := os.Rename(tempPath, fullPath); err != nil {
			return nil, err
		}
	}

	return dataToSave, nil
}
