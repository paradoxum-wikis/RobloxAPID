package wiki

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/antonholmquist/jason"
)

type WikiClient struct {
	client   *mwclient.Client
	editMu   sync.Mutex
	lastEdit time.Time
}

func NewWikiClient(apiURL, username, password string) (*WikiClient, error) {
	client, err := mwclient.New(apiURL, "RobloxAPID/1.0 (https://github.com/paradoxum-wikis/RobloxAPID; User:DarkGabonnie)")
	if err != nil {
		return nil, err
	}

	client.SetDebug(os.Stdout)

	if err := client.Login(username, password); err != nil {
		return nil, err
	}

	p := params.Values{
		"action":        "query",
		"meta":          "userinfo",
		"uiprop":        "rights",
		"format":        "json",
		"formatversion": "2",
	}
	resp, err := client.Get(p)
	if err != nil {
		return nil, fmt.Errorf("failed to query user rights: %w", err)
	}

	userinfo, err := resp.GetObject("query", "userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to parse userinfo: %w", err)
	}

	rights, err := userinfo.GetStringArray("rights")
	if err != nil {
		return nil, fmt.Errorf("failed to get user rights: %w", err)
	}

	hasBot := false
	for _, right := range rights {
		if right == "bot" {
			hasBot = true
			break
		}
	}

	if !hasBot {
		return nil, errors.New("user does not have bot user rights, you may not proceed without it")
	}

	return &WikiClient{client: client}, nil
}

func (w *WikiClient) Push(title, content, summary string) error {
	w.throttleEdit()
	log.Printf("[DEBUG] wiki.Push: preparing to push page %s (summary: %s)", title, summary)
	token, err := w.client.GetToken("csrf")
	if err != nil {
		log.Printf("[ERROR] wiki.Push: failed to get token for %s: %v", title, err)
		return err
	}

	p := params.Values{
		"action":  "edit",
		"title":   title,
		"text":    content,
		"summary": summary,
		"bot":     "true",
		"token":   token,
	}

	_, err = w.client.Post(p)
	if err != nil {
		log.Printf("[ERROR] wiki.Push: failed to push %s: %v", title, err)
		return err
	}

	log.Printf("[INFO] wiki.Push: successfully pushed %s", title)
	return nil
}

func (w *WikiClient) throttleEdit() {
	w.editMu.Lock()
	defer w.editMu.Unlock()

	now := time.Now()
	if !w.lastEdit.IsZero() {
		wait := time.Second - now.Sub(w.lastEdit)
		if wait > 0 {
			time.Sleep(wait)
			now = time.Now()
		}
	}
	w.lastEdit = now
}

func logJSON(label string, obj *jason.Object) {
	m := obj.Map()
	rawJSON, _ := json.MarshalIndent(m, "", "  ")
	log.Printf("[DEBUG] %s:\n%s", label, string(rawJSON))
}

func (w *WikiClient) GetPageByName(pageName string) (string, error) {
	p := params.Values{
		"action":        "query",
		"prop":          "revisions",
		"titles":        pageName,
		"rvprop":        "content",
		"rvslots":       "main",
		"format":        "json",
		"formatversion": "2",
	}

	resp, err := w.client.Get(p)
	if err != nil {
		return "", err
	}

	logJSON("GetPageByName response", resp)

	pages, err := resp.GetObjectArray("query", "pages")
	if err != nil {
		if strings.Contains(err.Error(), "no value for key") {
			return "", errors.New("page not found")
		}
		return "", err
	}
	if len(pages) == 0 {
		return "", errors.New("page not found")
	}

	page := pages[0]
	pageID, _ := page.GetInt64("pageid")
	if pageID == -1 {
		return "", errors.New("page not found")
	}
	if missing, _ := page.GetBoolean("missing"); missing {
		return "", errors.New("page not found")
	}

	revisions, err := page.GetObjectArray("revisions")
	if err != nil || len(revisions) == 0 {
		return "", errors.New("no revisions found")
	}

	mainSlot, err := revisions[0].GetObject("slots", "main")
	if err != nil {
		return "", err
	}

	content, err := mainSlot.GetString("content")
	if err != nil {
		return "", err
	}

	return content, nil
}

func (w *WikiClient) PageExists(title string) (bool, error) {
	p := params.Values{
		"action":        "query",
		"prop":          "info",
		"titles":        title,
		"format":        "json",
		"formatversion": "2",
	}

	resp, err := w.client.Get(p)
	if err != nil {
		return false, err
	}

	pages, err := resp.GetObjectArray("query", "pages")
	if err != nil {
		if strings.Contains(err.Error(), "no value for key") {
			return false, nil
		}
		return false, err
	}
	if len(pages) == 0 {
		return false, nil
	}

	page := pages[0]
	if missing, _ := page.GetBoolean("missing"); missing {
		return false, nil
	}
	pageID, _ := page.GetInt64("pageid")
	if pageID == -1 {
		return false, nil
	}

	return true, nil
}

// checks if Roapid module page exists and is at the required version
func (w *WikiClient) SetupRoapiModule(pageTitle, requiredVersion, content string) error {
	log.Printf("Checking wiki page: %s", pageTitle)

	existingContent, err := w.GetPageByName(pageTitle)
	if err != nil {
		if err.Error() == "page not found" {
			log.Printf("Page %s not found. Creating with version %s.", pageTitle, requiredVersion)
			return w.Push(pageTitle, content, "Initializing Roapid module, version "+requiredVersion)
		}
		return err
	}

	firstLine := strings.SplitN(existingContent, "\n", 2)[0]

	if !strings.HasPrefix(firstLine, "-- ") {
		log.Printf("Page %s missing version comment. Overwriting with version %s.", pageTitle, requiredVersion)
		return w.Push(pageTitle, content, "Updating Roapid module to version "+requiredVersion)
	}

	existingVersion := strings.TrimSpace(strings.TrimPrefix(firstLine, "-- "))
	if existingVersion != requiredVersion {
		log.Printf("Updating %s: version %s → %s", pageTitle, existingVersion, requiredVersion)
		return w.Push(pageTitle, content, "Updating Roapid module from "+existingVersion+" to "+requiredVersion)
	}

	log.Printf("Page %s is up to date (version %s).", pageTitle, requiredVersion)
	return nil
}

func (w *WikiClient) GetCategoriesWithPrefix(prefix string) ([]string, error) {
	if prefix == "" {
		return nil, errors.New("prefix cannot be empty")
	}

	acPrefix := strings.ToUpper(prefix[:1]) + prefix[1:]

	p := params.Values{
		"action":        "query",
		"list":          "allcategories",
		"acprefix":      acPrefix,
		"aclimit":       "max",
		"format":        "json",
		"formatversion": "2",
	}

	resp, err := w.client.Get(p)
	if err != nil {
		return nil, err
	}

	cats, err := resp.GetObjectArray("query", "allcategories")
	if err != nil {
		if strings.Contains(err.Error(), "no value for key") {
			return []string{}, nil
		}
		return nil, err
	}

	var titles []string
	for _, cat := range cats {
		name, err := cat.GetString("category")
		if err != nil || name == "" {
			name, _ = cat.GetString("*")
		}
		if name == "" {
			continue
		}
		titles = append(titles, "Category:"+name)
	}

	return titles, nil
}

func (w *WikiClient) GetCategoryMembers(category string) ([]string, error) {
	if category == "" {
		return nil, errors.New("category cannot be empty")
	}
	if !strings.HasPrefix(strings.ToLower(category), "category:") {
		category = "Category:" + category
	}

	p := params.Values{
		"action":        "query",
		"list":          "categorymembers",
		"cmtitle":       category,
		"cmlimit":       "max",
		"format":        "json",
		"formatversion": "2",
	}

	resp, err := w.client.Get(p)
	if err != nil {
		return nil, err
	}

	members, err := resp.GetObjectArray("query", "categorymembers")
	if err != nil {
		if strings.Contains(err.Error(), "no value for key") {
			return []string{}, nil
		}
		return nil, err
	}

	var titles []string
	for _, member := range members {
		title, err := member.GetString("title")
		if err != nil || title == "" {
			continue
		}
		titles = append(titles, title)
	}

	return titles, nil
}

func (w *WikiClient) PurgePages(titles []string) error {
	if len(titles) == 0 {
		return nil
	}
	p := params.Values{
		"action": "purge",
		"titles": strings.Join(titles, "|"),
		"format": "json",
	}
	_, err := w.client.Post(p)
	return err
}

func (w *WikiClient) PurgeCategoryMembers(category string) error {
	titles, err := w.GetCategoryMembers(category)
	if err != nil {
		return err
	}
	return w.PurgePages(titles)
}

//go:embed roapid.lua
var RoapidLua string
