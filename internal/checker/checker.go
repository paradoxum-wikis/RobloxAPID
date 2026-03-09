package checker

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// checks if new data differs from stored data, ignoring 'roLastUpdated'
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

	var oldDataMap map[string]interface{}
	if err := json.Unmarshal(oldData, &oldDataMap); err != nil {
		// if we can't unmarshal, fall back to raw byte compare
		eq := bytes.Equal(oldData, newData)
		if eq {
			log.Printf("[DEBUG] checker.HasChanged: raw compare -> unchanged: %s", fullPath)
			return false, nil
		}
		log.Printf("[DEBUG] checker.HasChanged: raw compare -> changed: %s", fullPath)
		return true, nil
	}
	delete(oldDataMap, "roLastUpdated")
	oldAPIData, err := json.Marshal(oldDataMap)
	if err != nil {
		log.Printf("[ERROR] checker.HasChanged: failed to marshal old data for %s: %v", fullPath, err)
		return false, err
	}

	var newDataMap map[string]interface{}
	if err := json.Unmarshal(newData, &newDataMap); err != nil {
		eq := bytes.Equal(oldData, newData)
		if eq {
			log.Printf("[DEBUG] checker.HasChanged: new data unmarshal failed but raw compare -> unchanged: %s", fullPath)
			return false, nil
		}
		log.Printf("[DEBUG] checker.HasChanged: new data unmarshal failed but raw compare -> changed: %s", fullPath)
		return true, nil
	}
	delete(newDataMap, "roLastUpdated")
	newAPIData, err := json.Marshal(newDataMap)
	if err != nil {
		log.Printf("[ERROR] checker.HasChanged: failed to marshal new data for %s: %v", fullPath, err)
		return false, err
	}

	changed := !bytes.Equal(oldAPIData, newAPIData)
	if changed {
		log.Printf("[DEBUG] checker.HasChanged: content changed: %s", fullPath)
	} else {
		log.Printf("[DEBUG] checker.HasChanged: content unchanged: %s", fullPath)
	}
	return changed, nil
}
