package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"robloxapid/pkg/checker"
	"robloxapid/pkg/config"
	"robloxapid/pkg/fetcher"
	"robloxapid/pkg/storage"
	"robloxapid/pkg/wiki"
)

type staticDoc struct {
	filename string
	wikiSlug string
	summary  string
}

var staticDocs = []staticDoc{
	{
		filename: "badges.json",
		wikiSlug: "badges.json",
		summary:  "Automated sync of legacy badges usage guide",
	},
	{
		filename: "users.json",
		wikiSlug: "users.json",
		summary:  "Automated sync of users usage guide",
	},
	{
		filename: "groups.json",
		wikiSlug: "groups.json",
		summary:  "Automated sync of groups usage guide",
	},
	{
		filename: "universes.json",
		wikiSlug: "universes.json",
		summary:  "Automated sync of universes usage guide",
	},
	{
		filename: "places.json",
		wikiSlug: "places.json",
		summary:  "Automated sync of places usage guide",
	},
	{
		filename: "games.json",
		wikiSlug: "games.json",
		summary:  "Automated sync of legacy games API guide",
	},
}

func processEndpoint(wikiClient *wiki.WikiClient, cfg *config.Config, endpointType, id, category string) error {
	urlTemplate, ok := cfg.DynamicEndpoints.APIMap[endpointType]
	if !ok {
		return fmt.Errorf("unknown endpoint type: %s", endpointType)
	}

	url, err := formatEndpointURL(endpointType, id, urlTemplate)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s-%s.json", endpointType, id)

	var newData []byte

	switch endpointType {
	case "users", "groups", "universes", "places":
		if cfg.OpenCloud.APIKey == "" {
			return fmt.Errorf("open cloud api key required for %s", endpointType)
		}
		headers := map[string]string{
			"x-api-key": cfg.OpenCloud.APIKey,
			"Accept":    "application/json",
		}
		newData, err = fetcher.FetchWithHeaders(url, headers)
	default:
		newData, err = fetcher.Fetch(url)
	}

	if err != nil {
		return fmt.Errorf("error fetching data from %s: %v", url, err)
	}

	hasChanged, err := checker.HasChanged(path, newData)
	if err != nil {
		return fmt.Errorf("error checking changes for %s: %v", path, err)
	}

	log.Printf("Updating data for %s...", url)
	dataToPush, err := storage.Save(path, newData)
	if err != nil {
		return fmt.Errorf("error saving data to %s: %v", path, err)
	}

	wikiTitle := fmt.Sprintf("%s:roapid/%s-%s.json", cfg.Wiki.Namespace, endpointType, id)

	shouldPush := hasChanged
	if !hasChanged {
		exists, err := wikiClient.PageExists(wikiTitle)
		if err != nil {
			log.Printf("Error checking if %s exists: %v", wikiTitle, err)
		} else if !exists {
			log.Printf("%s missing on wiki; forcing upload.", wikiTitle)
			shouldPush = true
		}
	}

	if !shouldPush {
		log.Printf("No meaningful changes for %s (only roLastUpdated or none), skipping wiki push.", url)
		if err := wikiClient.PurgeCategoryMembers(category); err != nil {
			log.Printf("Error purging pages for %s: %v", category, err)
		}
		return nil
	}

	log.Printf("Meaningful changes detected for %s, pushing to wiki.", url)
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

func processStaticDoc(wikiClient *wiki.WikiClient, cfg *config.Config, doc staticDoc) error {
	localPath := filepath.Join("config", doc.filename)

	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", localPath, err)
	}

	hasChanged, err := checker.HasChanged(doc.filename, content)
	if err != nil {
		return fmt.Errorf("error checking changes for %s: %w", doc.filename, err)
	}
	if !hasChanged {
		log.Printf("%s unchanged; skipping wiki update.", doc.filename)
		return nil
	}

	dataToPush, err := storage.Save(doc.filename, content)
	if err != nil {
		return fmt.Errorf("error saving %s data: %w", doc.filename, err)
	}

	wikiTitle := fmt.Sprintf("%s:roapid/%s", cfg.Wiki.Namespace, doc.wikiSlug)
	if err := wikiClient.Push(wikiTitle, string(dataToPush), doc.summary); err != nil {
		return fmt.Errorf("error pushing %s to wiki: %w", doc.filename, err)
	}

	if err := wikiClient.PurgePages([]string{wikiTitle}); err != nil {
		log.Printf("Error purging %s: %v", wikiTitle, err)
	}

	log.Printf("Successfully synced %s", wikiTitle)
	return nil
}

func syncStaticDocs(wikiClient *wiki.WikiClient, cfg *config.Config) error {
	var firstErr error
	for _, doc := range staticDocs {
		if err := processStaticDoc(wikiClient, cfg, doc); err != nil {
			log.Printf("Error syncing %s: %v", doc.filename, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func formatEndpointURL(endpointType, id, template string) (string, error) {
	var formatArg string

	switch endpointType {
	case "places":
		parts := strings.SplitN(id, "-", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", fmt.Errorf("invalid place identifier %q, expected universeId-placeId", id)
		}
		formatArg = fmt.Sprintf("universes/%s/places/%s", parts[0], parts[1])
	default:
		formatArg = id
	}

	return fmt.Sprintf(template, formatArg), nil
}
