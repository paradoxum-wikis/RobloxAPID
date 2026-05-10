package checker

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

func HasChanged(path string, newData []byte) (bool, error) {
	fullPath := filepath.Join("data", path)
	log.Printf("[DEBUG] checker.HasChanged: checking %s", fullPath)
	oldData, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
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

	var oldDataMap map[string]json.RawMessage
	if err := json.Unmarshal(oldData, &oldDataMap); err != nil {
		eq := bytes.Equal(oldData, newData)
		if eq {
			log.Printf("[DEBUG] checker.HasChanged: raw compare -> unchanged: %s", fullPath)
			return false, nil
		}
		log.Printf("[DEBUG] checker.HasChanged: raw compare -> changed: %s", fullPath)
		return true, nil
	}

	var newDataMap map[string]json.RawMessage
	if err := json.Unmarshal(newData, &newDataMap); err != nil {
		eq := bytes.Equal(oldData, newData)
		if eq {
			log.Printf("[DEBUG] checker.HasChanged: new data unmarshal failed but raw compare -> unchanged: %s", fullPath)
			return false, nil
		}
		log.Printf("[DEBUG] checker.HasChanged: new data unmarshal failed but raw compare -> changed: %s", fullPath)
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
		if !ok || !bytes.Equal(oldValue, newValue) {
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
