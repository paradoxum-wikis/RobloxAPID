package app

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"robloxapid/internal/config"
	"robloxapid/internal/storage"
)

type EndpointState struct {
	EndpointType string
	Interval     time.Duration
	NextRun      time.Time
}

var categoryNormalizer = strings.NewReplacer(
	"\u2212", "-", // unicode minus
	"\u2010", "-", // hyphen
	"\u2011", "-", // nb hyphen
	"\u2012", "-", // figure dash
	"\u2013", "-", // en dash
	"\u2014", "-", // em dash
	"\u2015", "-", // horizontal bar
	",", "", // commas
	"\u00a0", "", // nbsp
	"\u202f", "", // nnbsp
)

func ParseCategory(category, prefix string, apiMap map[string]string) (endpointType, id string, err error) {
	normalized := normalizeCategory(category)
	expectedPrefix := "Category:" + prefix + "-"
	if len(normalized) < len(expectedPrefix) || !strings.EqualFold(normalized[:len(expectedPrefix)], expectedPrefix) {
		return "", "", fmt.Errorf("invalid category format: %s", category)
	}
	endpointType, id, ok := splitTypeID(normalized[len(expectedPrefix):], apiMap)
	if !ok {
		return "", "", fmt.Errorf("invalid category format: %s", category)
	}
	return endpointType, id, nil
}

func splitTypeID(remainder string, apiMap map[string]string) (typ, id string, ok bool) {
	for t := range apiMap {
		if !strings.HasPrefix(remainder, t+"-") {
			continue
		}
		rest := remainder[len(t)+1:]
		if rest == "" || len(t) < len(typ) {
			continue
		}
		typ, id, ok = t, rest, true
	}
	return typ, id, ok
}

func normalizeCategory(category string) string {
	trimmed := strings.TrimSpace(category)
	return categoryNormalizer.Replace(trimmed)
}

func UpdateSchedule(processed map[string]*EndpointState, mu *sync.Mutex, category, endpointType string, cfg *config.Config, next time.Time) {
	var interval time.Duration

	mu.Lock()
	if state, ok := processed[category]; ok && state != nil && state.Interval > 0 {
		interval = state.Interval
	}
	mu.Unlock()

	if interval == 0 {
		var err error
		interval, err = cfg.GetRefreshInterval(endpointType)
		if err != nil {
			log.Printf("Invalid refresh interval for %s: %v", endpointType, err)
			if interval, err = cfg.GetDataRefreshInterval(); err != nil {
				interval = time.Minute
			}
		}
	}

	if next.IsZero() {
		next = time.Now().Add(interval)
	}

	mu.Lock()
	state, ok := processed[category]
	if !ok {
		state = new(EndpointState)
		processed[category] = state
	}
	state.EndpointType = endpointType
	state.Interval = interval
	state.NextRun = next
	mu.Unlock()
}

func BootstrapFromData(processed map[string]*EndpointState, mu *sync.Mutex, cfg *config.Config) {
	entries, err := os.ReadDir("data")
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[DEBUG] bootstrap: data directory not found; nothing to schedule yet")
			return
		}
		log.Printf("[ERROR] bootstrap: cannot read data directory: %v", err)
		return
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		base := strings.TrimSuffix(name, ".json")
		endpointType, id, ok := splitTypeID(base, cfg.DynamicEndpoints.APIMap)
		if !ok {
			continue
		}

		category := fmt.Sprintf("Category:%s-%s-%s", cfg.DynamicEndpoints.CategoryPrefix, endpointType, id)

		mu.Lock()
		_, exists := processed[category]
		mu.Unlock()
		if exists {
			continue
		}

		nextRun := time.Now()
		if lastUpdated, ok := storage.ReadRoLastUpdated(name); ok {
			interval, err := cfg.GetRefreshInterval(endpointType)
			if err != nil {
				interval, err = cfg.GetDataRefreshInterval()
				if err != nil {
					interval = time.Minute
				}
			}
			nextRun = lastUpdated.Add(interval)
		}
		log.Printf("[DEBUG] bootstrap: scheduling %s from %s (nextRun %v)", category, name, nextRun.UTC())
		UpdateSchedule(processed, mu, category, endpointType, cfg, nextRun)
		count++
	}

	log.Printf("[DEBUG] bootstrap: scheduled %d endpoints from existing data files", count)
}
