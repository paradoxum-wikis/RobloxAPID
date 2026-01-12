package main

import (
	"context"
	"log"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"robloxapid/pkg/config"
	"robloxapid/pkg/wiki"
)

const roapiModuleVersion = "0.0.17"

var roapiModuleContent = wiki.RoapidLua

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

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

	roapiModuleContent = strings.ReplaceAll(roapiModuleContent, "{{NAMESPACE}}", cfg.Wiki.Namespace)
	roapiModuleContent = strings.ReplaceAll(roapiModuleContent, "{{CATEGORY_PREFIX}}", cfg.DynamicEndpoints.CategoryPrefix)

	queueNote := cfg.LuaMessages.QueueNote
	if queueNote == "" {
		queueNote = "Publish this page and wait at least a minute for data to be fetched."
	}
	roapiModuleContent = strings.ReplaceAll(roapiModuleContent, "{{MSG_QUEUE_NOTE}}", queueNote)

	fpnf := cfg.LuaMessages.FieldPathNotFound
	if fpnf == "" {
		fpnf = "Field path not found (%s), [[%s|see fields]]."
	}
	roapiModuleContent = strings.ReplaceAll(roapiModuleContent, "{{MSG_FIELD_PATH_NOT_FOUND}}", fpnf)

	err = wikiClient.SetupRoapiModule(cfg.Wiki.Namespace+":Roapid", roapiModuleVersion, roapiModuleContent)
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

	documentationInterval, err := cfg.GetRefreshInterval("badges")
	if err != nil {
		log.Printf("Invalid documentation refresh interval: %v; falling back to %v", err, dataInterval)
		documentationInterval = dataInterval
	}

	log.Printf("Starting with intervals: categories every %v, default refresh every %v", categoryInterval, dataInterval)

	if err := processAboutEndpoint(wikiClient, cfg); err != nil {
		log.Printf("Initial about sync failed: %v", err)
	}
	if err := syncStaticDocs(wikiClient, cfg); err != nil {
		log.Printf("Initial documentation sync failed: %v", err)
	}

	processedEndpoints := make(map[string]*endpointState)
	var mu sync.Mutex
	var workers sync.WaitGroup

	startTicker := func(interval time.Duration, name string, fn func()) {
		if interval <= 0 {
			return
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					log.Printf("%s ticker stopping", name)
					return
				case <-ticker.C:
					fn()
				}
			}
		}()
	}

	bootstrapFromData(processedEndpoints, &mu, cfg)

	{
		now := time.Now()
		type toRef struct{ category, endpointType, id string }
		var immediate []toRef

		mu.Lock()
		for category, st := range processedEndpoints {
			if !st.nextRun.IsZero() && !now.Before(st.nextRun) {
				et, id, err := parseCategory(category, cfg.DynamicEndpoints.CategoryPrefix)
				if err != nil {
					continue
				}
				immediate = append(immediate, toRef{category, et, id})
			}
		}
		mu.Unlock()

		for _, r := range immediate {
			workers.Add(1)
			go func(r toRef) {
				defer workers.Done()
				select {
				case <-ctx.Done():
					log.Printf("[DEBUG] bootstrap: skipping %s due to shutdown", r.category)
					return
				default:
				}
				log.Printf("[DEBUG] bootstrap: immediate refresh %s", r.category)
				if err := processEndpoint(wikiClient, cfg, r.endpointType, r.id, r.category); err != nil {
					log.Printf("Error refreshing bootstrapped endpoint %s: %v", r.category, err)
					return
				}
				updateSchedule(processedEndpoints, &mu, r.category, r.endpointType, cfg, time.Time{})
			}(r)
		}
	}

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

	checkCategories()

	startTicker(aboutInterval, "about sync", func() {
		if err := processAboutEndpoint(wikiClient, cfg); err != nil {
			log.Printf("Scheduled about sync failed: %v", err)
		}
	})

	startTicker(documentationInterval, "documentation sync", func() {
		if err := syncStaticDocs(wikiClient, cfg); err != nil {
			log.Printf("Scheduled documentation sync failed: %v", err)
		}
	})

	startTicker(categoryInterval, "category scan", checkCategories)

	startTicker(dataInterval, "data refresh", func() {
		log.Println("Refreshing existing data...")

		mu.Lock()
		endpointsToRefresh := make(map[string]*endpointState)
		for k, v := range processedEndpoints {
			endpointsToRefresh[k] = v
		}
		mu.Unlock()

		for category, state := range endpointsToRefresh {
			if time.Now().Before(state.nextRun) {
				log.Printf("[DEBUG] refresh: skipping %s (nextRun %v)", category, state.nextRun)
				continue
			}

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
	})

	<-ctx.Done()
	log.Println("Shutdown signal received, waiting for workers to finish...")
	workers.Wait()
	log.Println("Shutdown complete.")
}
