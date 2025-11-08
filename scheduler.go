package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"robloxapid/pkg/config"
)

type endpointState struct {
	endpointType string
	interval     time.Duration
	nextRun      time.Time
}

func parseCategory(category, prefix string) (endpointType, id string, err error) {
	normalized := normalizeCategory(category)
	expectedPrefix := "Category:" + prefix + "-"
	if len(normalized) < len(expectedPrefix) || !strings.EqualFold(normalized[:len(expectedPrefix)], expectedPrefix) {
		return "", "", fmt.Errorf("invalid category format: %s", category)
	}
	remainder := normalized[len(expectedPrefix):]
	parts := strings.SplitN(remainder, "-", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid category format: %s", category)
	}
	return parts[0], parts[1], nil
}

func normalizeCategory(category string) string {
	replacer := strings.NewReplacer(
		"\u2212", "−", // unicode minus
		"\u2010", "‐", // hyphen
		"\u2011", "‑", // nb hyphen
		"\u2012", "‒", // figure dash
		"\u2013", "–", // en dash
		"\u2014", "—", // em dash
		"\u2015", "―", // horizontal bar
		",", "", // commas
		"\u00a0", "", // nbsp
		"\u202f", "", // nnbsp
	)
	trimmed := strings.TrimSpace(category)
	return replacer.Replace(trimmed)
}

func updateSchedule(processed map[string]*endpointState, mu *sync.Mutex, category, endpointType string, cfg *config.Config, next time.Time) {
	interval := time.Duration(0)

	mu.Lock()
	if state, ok := processed[category]; ok && state != nil && state.interval > 0 {
		interval = state.interval
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
		state = &endpointState{}
		processed[category] = state
	}
	state.endpointType = endpointType
	state.interval = interval
	state.nextRun = next
	mu.Unlock()
}

func bootstrapFromData(processed map[string]*endpointState, mu *sync.Mutex, cfg *config.Config) {
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
		parts := strings.SplitN(base, "-", 2)
		if len(parts) != 2 {
			continue
		}

		endpointType, id := parts[0], parts[1]
		if id == "" {
			continue
		}

		category := fmt.Sprintf("Category:%s-%s-%s", cfg.DynamicEndpoints.CategoryPrefix, endpointType, id)

		mu.Lock()
		_, exists := processed[category]
		mu.Unlock()
		if exists {
			continue
		}

		log.Printf("[DEBUG] bootstrap: scheduling %s from %s", category, name)
		updateSchedule(processed, mu, category, endpointType, cfg, time.Now())
		count++
	}

	log.Printf("[DEBUG] bootstrap: scheduled %d endpoints from existing data files", count)
}
