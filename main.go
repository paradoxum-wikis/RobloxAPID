package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"robloxapid/pkg/checker"
	"robloxapid/pkg/config"
	"robloxapid/pkg/fetcher"
	"robloxapid/pkg/storage"
	"robloxapid/pkg/wiki"
)

const roapiModuleVersion = "0.0.8"
const roapiModuleContent = `-- 0.0.8
-- https://github.com/paradoxum-wikis/RobloxAPID
local roapid = {}
local function get_by_path(tbl, parts)
    local cur = tbl
    for i = 1, #parts do
        if type(cur) ~= "table" then
            return nil
        end
        cur = cur[parts[i]]
    end
    return cur
end

local function build_title(resource, id)
    if id == "" then
        return string.format("Module:roapid/%s.json", resource)
    else
        return string.format("Module:roapid/%s-%s.json", resource, id)
    end
end

local function get_queue_category(resource, id)
    if resource == "badges" and id ~= "" then
        return string.format("[[Category:robloxapid-queue-badges-%s]]", id)
    end
    return ""
end

function roapid._get(frame, resource, needs_id)
    local args = frame.args
    local id = needs_id and (args[1] or "") or ""
    local path = {}
    local start_idx = needs_id and 2 or 1
    local i = start_idx
    while args[tostring(i)] do
        local v = args[tostring(i)]
        if v and v ~= "" then
            path[#path + 1] = v
        end
        i = i + 1
    end

    if needs_id and id == "" then
        return ""
    end

    local module_name = build_title(resource, id)
    local ok, data = pcall(mw.loadJsonData, module_name)

    if not ok or type(data) ~= "table" then
        return get_queue_category(resource, id)
    end

    if #path == 0 then
        local json_ok, json = pcall(mw.text.jsonEncode, data)
        return json_ok and json or mw.text.jsonEncode(data)
    end

    local value = get_by_path(data, path)
    if value == nil then
        return ""
    end

    if type(value) == "table" then
        local json_ok, json = pcall(mw.text.jsonEncode, value)
        return json_ok and json or mw.text.jsonEncode(value)
    else
        return tostring(value)
    end
end

function roapid.badges(frame)
    local id = frame.args[1]
    if not id or id == "" then
        return roapid._get(frame, "badges", false)
    end
    return roapid._get(frame, "badges", true)
end

function roapid.about(frame)
    return roapid._get(frame, "about", false)
end

return roapid
`

type endpointState struct {
	endpointType string
	interval     time.Duration
	nextRun      time.Time
}

func main() {
	log.Println("--- RobloxAPID ---")
	log.Println("Description: A daemon that bridges the Roblox API to Fandom wikis.")
	log.Println("Source: https://github.com/paradoxum-wikis/RobloxAPID")
	log.Println("--------------------")

	cfg, err := config.LoadConfig("config/config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	wikiClient, err := wiki.NewWikiClient(cfg.Wiki.APIURL, cfg.Wiki.Username, cfg.Wiki.Password)
	if err != nil {
		log.Fatalf("Failed to create wiki client: %v", err)
	}

	err = wikiClient.SetupRoapiModule("Module:Roapid", roapiModuleVersion, roapiModuleContent)
	if err != nil {
		log.Fatalf("Failed to setup Roapid module on wiki: %v", err)
	}

	// intervals
	categoryInterval, err := cfg.GetCategoryCheckInterval()
	if err != nil {
		log.Fatalf("Invalid category check interval: %v", err)
	}

	dataInterval, err := cfg.GetDataRefreshInterval()
	if err != nil {
		log.Fatalf("Invalid data refresh interval: %v", err)
	}

	aboutInterval, err := cfg.GetRefreshInterval("about")
	if err != nil {
		log.Printf("Invalid about refresh interval: %v; falling back to %v", err, dataInterval)
		aboutInterval = dataInterval
	}

	badgesInterval, err := cfg.GetRefreshInterval("badges")
	if err != nil {
		log.Printf("Invalid badges refresh interval: %v; falling back to %v", err, dataInterval)
		badgesInterval = dataInterval
	}

	log.Printf("Starting with intervals: categories every %v, default refresh every %v", categoryInterval, dataInterval)

	if err := processAboutEndpoint(wikiClient, cfg); err != nil {
		log.Printf("Initial about sync failed: %v", err)
	}
	if err := processBadgesRoot(wikiClient, cfg); err != nil {
		log.Printf("Initial badges root sync failed: %v", err)
	}

	go func() {
		if aboutInterval <= 0 {
			return
		}
		ticker := time.NewTicker(aboutInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := processAboutEndpoint(wikiClient, cfg); err != nil {
				log.Printf("Scheduled about sync failed: %v", err)
			}
		}
	}()

	go func() {
		if badgesInterval <= 0 {
			return
		}
		ticker := time.NewTicker(badgesInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := processBadgesRoot(wikiClient, cfg); err != nil {
				log.Printf("Scheduled badges root sync failed: %v", err)
			}
		}
	}()

	processedEndpoints := make(map[string]*endpointState)
	var mu sync.Mutex

	aboutCategory := fmt.Sprintf("Category:%s-about", cfg.DynamicEndpoints.CategoryPrefix)
	updateSchedule(processedEndpoints, &mu, aboutCategory, "about", cfg, time.Now())

	checkCategories := func() {
		log.Println("Checking for new wanted categories...")

		categories, err := wikiClient.GetCategoriesWithPrefix(cfg.DynamicEndpoints.CategoryPrefix)
		if err != nil {
			log.Printf("Error fetching queue categories: %v", err)
			return
		}

		for _, category := range categories {
			endpointType, id, err := parseCategory(category, cfg.DynamicEndpoints.CategoryPrefix)
			if err != nil {
				log.Printf("Error parsing category %s: %v", category, err)
				continue
			}

			mu.Lock()
			state, exists := processedEndpoints[category]
			mu.Unlock()

			if !exists {
				if err := processEndpoint(wikiClient, cfg, endpointType, id, category); err != nil {
					log.Printf("Error processing new endpoint %s: %v", category, err)
				} else {
					updateSchedule(processedEndpoints, &mu, category, endpointType, cfg, time.Time{})
				}
			} else if time.Now().After(state.nextRun) {
				log.Printf("Refreshing endpoint %s...", category)
				if err := processEndpoint(wikiClient, cfg, state.endpointType, id, category); err != nil {
					log.Printf("Error refreshing endpoint %s: %v", category, err)
				} else {
					updateSchedule(processedEndpoints, &mu, category, state.endpointType, cfg, time.Time{})
				}
			}
		}
	}

	go func() {
		checkCategories()

		ticker := time.NewTicker(categoryInterval)
		defer ticker.Stop()

		for range ticker.C {
			checkCategories()
		}
	}()

	go func() {
		ticker := time.NewTicker(dataInterval)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("Refreshing existing data...")

			mu.Lock()
			endpointsToRefresh := make(map[string]*endpointState)
			for k, v := range processedEndpoints {
				endpointsToRefresh[k] = v
			}
			mu.Unlock()

			for category, state := range endpointsToRefresh {
				endpointType, id, err := parseCategory(category, cfg.DynamicEndpoints.CategoryPrefix)
				if err != nil {
					log.Printf("Error parsing category %s: %v", category, err)
					continue
				}

				log.Printf("Refreshing endpoint %s...", category)
				if err := processEndpoint(wikiClient, cfg, endpointType, id, category); err != nil {
					log.Printf("Error refreshing endpoint %s: %v", category, err)
					continue
				}

				mu.Lock()
				if st, ok := processedEndpoints[category]; ok && st != nil {
					st.nextRun = time.Now().Add(st.interval)
				} else {
					processedEndpoints[category] = &endpointState{
						endpointType: endpointType,
						interval:     state.interval,
						nextRun:      time.Now().Add(state.interval),
					}
				}
				mu.Unlock()
			}
		}
	}()

	select {}
}

func parseCategory(category, prefix string) (endpointType, id string, err error) {
	expectedPrefix := "Category:" + prefix + "-"
	if len(category) < len(expectedPrefix) || !strings.EqualFold(category[:len(expectedPrefix)], expectedPrefix) {
		return "", "", fmt.Errorf("invalid category format: %s", category)
	}
	remainder := category[len(expectedPrefix):]
	parts := strings.SplitN(remainder, "-", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid category format: %s", category)
	}
	return parts[0], parts[1], nil
}

func processEndpoint(wikiClient *wiki.WikiClient, cfg *config.Config, endpointType, id, category string) error {
	urlTemplate, ok := cfg.DynamicEndpoints.APIMap[endpointType]
	if !ok {
		return fmt.Errorf("unknown endpoint type: %s", endpointType)
	}

	url := fmt.Sprintf(urlTemplate, id)
	path := fmt.Sprintf("%s-%s.json", endpointType, id)

	newData, err := fetcher.Fetch(url)
	if err != nil {
		return fmt.Errorf("error fetching data from %s: %v", url, err)
	}

	hasChanged, err := checker.HasChanged(path, newData)
	if err != nil {
		return fmt.Errorf("error checking changes for %s: %v", path, err)
	}

	if !hasChanged {
		log.Printf("No changes for %s, but making sure page exists.", url)
	}

	log.Printf("Updating data for %s...", url)
	dataToPush, err := storage.Save(path, newData)
	if err != nil {
		return fmt.Errorf("error saving data to %s: %v", path, err)
	}

	wikiTitle := fmt.Sprintf("%s:roapid/%s-%s.json", cfg.Wiki.Namespace, endpointType, id)
	summary := fmt.Sprintf("Automated update from %s", url)
	err = wikiClient.Push(wikiTitle, string(dataToPush), summary)
	if err != nil {
		return fmt.Errorf("error pushing to wiki for %s: %v", wikiTitle, err)
	}

	if err := wikiClient.PurgeCategoryMembers(category); err != nil {
		log.Printf("Error purging pages for %s: %v", category, err)
	}

	log.Printf("Successfully updated %s", wikiTitle)
	return nil
}

func processAboutEndpoint(wikiClient *wiki.WikiClient, cfg *config.Config) error {
	const aboutFilename = "about.json"
	localPath := filepath.Join("config", aboutFilename)

	aboutJSON, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", localPath, err)
	}

	hasChanged, err := checker.HasChanged(aboutFilename, aboutJSON)
	if err != nil {
		return fmt.Errorf("error checking changes for %s: %w", aboutFilename, err)
	}
	if !hasChanged {
		log.Printf("%s unchanged; skipping wiki update.", aboutFilename)
		return nil
	}

	dataToPush, err := storage.Save(aboutFilename, aboutJSON)
	if err != nil {
		return fmt.Errorf("error saving about data: %w", err)
	}

	wikiTitle := fmt.Sprintf("%s:roapid/about.json", cfg.Wiki.Namespace)
	summary := "Automated sync of about information"
	if err := wikiClient.Push(wikiTitle, string(dataToPush), summary); err != nil {
		return fmt.Errorf("error pushing about page to wiki: %w", err)
	}

	if err := wikiClient.PurgePages([]string{wikiTitle}); err != nil {
		log.Printf("Error purging %s: %v", wikiTitle, err)
	}

	log.Printf("Successfully synced %s", wikiTitle)
	return nil
}

func processBadgesRoot(wikiClient *wiki.WikiClient, cfg *config.Config) error {
	const badgesFilename = "badges.json"
	localPath := filepath.Join("config", badgesFilename)

	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", localPath, err)
	}

	hasChanged, err := checker.HasChanged(badgesFilename, content)
	if err != nil {
		return fmt.Errorf("error checking changes for %s: %w", badgesFilename, err)
	}
	if !hasChanged {
		log.Printf("%s unchanged; skipping wiki update.", badgesFilename)
		return nil
	}

	dataToPush, err := storage.Save(badgesFilename, content)
	if err != nil {
		return fmt.Errorf("error saving badges data: %w", err)
	}

	wikiTitle := fmt.Sprintf("%s:roapid/badges.json", cfg.Wiki.Namespace)
	summary := "Automated sync of badges index"
	if err := wikiClient.Push(wikiTitle, string(dataToPush), summary); err != nil {
		return fmt.Errorf("error pushing badges index to wiki: %w", err)
	}

	if err := wikiClient.PurgePages([]string{wikiTitle}); err != nil {
		log.Printf("Error purging %s: %v", wikiTitle, err)
	}

	log.Printf("Successfully synced %s", wikiTitle)
	return nil
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
