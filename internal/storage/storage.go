package storage

import (
	"crypto/rand"
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

	dataRoot, err := os.OpenRoot("data")
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll("data", 0755); err != nil {
				return nil, err
			}
			dataRoot, err = os.OpenRoot("data")
		}
		if err != nil {
			return nil, err
		}
	}
	defer dataRoot.Close()

	dir := filepath.Dir(path)
	if err := dataRoot.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	tempName := filepath.Join(dir, fmt.Sprintf("%s.tmp-%s", filepath.Base(path), rand.Text()))
	tempFile, err := dataRoot.OpenFile(tempName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return nil, err
	}
	defer func() {
		tempFile.Close()
		_ = dataRoot.Remove(tempName)
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

	if err := dataRoot.Rename(tempName, path); err != nil {
		if removeErr := dataRoot.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, fmt.Errorf("failed to replace %s: %w", filepath.Join("data", path), err)
		}
		if err := dataRoot.Rename(tempName, path); err != nil {
			return nil, err
		}
	}

	return dataToSave, nil
}
