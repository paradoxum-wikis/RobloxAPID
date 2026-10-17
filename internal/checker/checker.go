package checker

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

func HasChanged(path string, newData []byte) (bool, error) {
	dataRoot, err := os.OpenRoot("data")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("[DEBUG] checker.HasChanged: data directory does not exist -> treat as changed")
			return true, nil
		}
		log.Printf("[ERROR] checker.HasChanged: failed to open data root: %v", err)
		return false, err
	}
	defer dataRoot.Close()

	fullPath := filepath.Join("data", path)
	log.Printf("[DEBUG] checker.HasChanged: checking %s", fullPath)
	oldData, err := dataRoot.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("[DEBUG] checker.HasChanged: file does not exist -> treat as changed: %s", fullPath)
			return true, nil
		}
		log.Printf("[ERROR] checker.HasChanged: failed to read %s: %v", fullPath, err)
		return false, err
	}

	if bytes.Equal(oldData, newData) {
		log.Printf("[DEBUG] checker.HasChanged: raw compare -> unchanged: %s", fullPath)
		return false, nil
	}

	var oldDataMap, newDataMap map[string]json.RawMessage
	if json.Unmarshal(oldData, &oldDataMap) != nil || json.Unmarshal(newData, &newDataMap) != nil {
		log.Printf("[DEBUG] checker.HasChanged: unparseable JSON, raw compare -> changed: %s", fullPath)
		return true, nil
	}
	changed := !equalIgnoringRo(oldDataMap, newDataMap)
	if changed {
		log.Printf("[DEBUG] checker.HasChanged: content changed: %s", fullPath)
	} else {
		log.Printf("[DEBUG] checker.HasChanged: content unchanged: %s", fullPath)
	}
	return changed, nil
}

func equalIgnoringRo(oldDataMap, newDataMap map[string]json.RawMessage) bool {
	for key, oldValue := range oldDataMap {
		if key == "roLastUpdated" {
			continue
		}
		newValue, ok := newDataMap[key]
		if !ok {
			return false
		}
		// stored files are indented by storage.Save
		// while API responses are compact lmao
		var oldCompact, newCompact bytes.Buffer
		if json.Compact(&oldCompact, oldValue) != nil || json.Compact(&newCompact, newValue) != nil {
			if !bytes.Equal(oldValue, newValue) {
				return false
			}
		} else if !bytes.Equal(oldCompact.Bytes(), newCompact.Bytes()) {
			return false
		}
	}

	for key := range newDataMap {
		if key == "roLastUpdated" {
			continue
		}
		if _, ok := oldDataMap[key]; !ok {
			return false
		}
	}

	return true
}
