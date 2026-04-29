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
)

type WikiClient struct {
	client   *mwclient.Client
	editMu   sync.Mutex
	lastEdit time.Time
}

type mwUserInfoResponse struct {
	Query struct {
		UserInfo struct {
			Rights []string `json:"rights"`
		} `json:"userinfo"`
	} `json:"query"`
}

type mwRevisionsResponse struct {
	Query struct {
		Pages []struct {
			PageID    int64 `json:"pageid"`
			Missing   bool  `json:"missing"`
			Revisions []struct {
				Slots struct {
					Main struct {
						Content string `json:"content"`
					} `json:"main"`
				} `json:"slots"`
			} `json:"revisions"`
		} `json:"pages"`
	} `json:"query"`
}

type mwInfoResponse struct {
	Query struct {
		Pages []struct {
			PageID  int64 `json:"pageid"`
			Missing bool  `json:"missing"`
		} `json:"pages"`
	} `json:"query"`
}

type mwAllCategoriesResponse struct {
	Query struct {
		AllCategories []struct {
			Category string `json:"category"`
			Star     string `json:"*"`
		} `json:"allcategories"`
	} `json:"query"`
}

type mwCategoryMembersResponse struct {
	Query struct {
		CategoryMembers []struct {
			Title string `json:"title"`
		} `json:"categorymembers"`
	} `json:"query"`
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

	respBody, err := client.GetRaw(p)
	if err != nil {
		return nil, fmt.Errorf("failed to query user rights: %w", err)
	}

	var res mwUserInfoResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, fmt.Errorf("failed to parse userinfo: %w", err)
	}

	hasBot := false
	for _, right := range res.Query.UserInfo.Rights {
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

func logRawJSON(label string, rawBody []byte) {
	var m interface{}
	if err := json.Unmarshal(rawBody, &m); err == nil {
		prettyJSON, _ := json.MarshalIndent(m, "", "  ")
		log.Printf("[DEBUG] %s:\n%s", label, string(prettyJSON))
	} else {
		log.Printf("[DEBUG] %s:\n%s", label, string(rawBody))
	}
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

	respBody, err := w.client.GetRaw(p)
	if err != nil {
		return "", err
	}
	logRawJSON("GetPageByName response:", respBody)

	var res mwRevisionsResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return "", fmt.Errorf("failed to parse page revisions: %w", err)
	}

	if len(res.Query.Pages) == 0 {
		return "", errors.New("page not found")
	}

	page := res.Query.Pages[0]
	if page.PageID == -1 || page.Missing {
		return "", errors.New("page not found")
	}

	if len(page.Revisions) == 0 {
		return "", errors.New("no revisions found")
	}

	return page.Revisions[0].Slots.Main.Content, nil
}

func (w *WikiClient) PageExists(title string) (bool, error) {
	p := params.Values{
		"action":        "query",
		"prop":          "info",
		"titles":        title,
		"format":        "json",
		"formatversion": "2",
	}

	respBody, err := w.client.GetRaw(p)
	if err != nil {
		return false, err
	}
	logRawJSON("PageExists response", respBody)

	var res mwInfoResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return false, nil
	}

	if len(res.Query.Pages) == 0 {
		return false, nil
	}

	page := res.Query.Pages[0]
	if page.Missing || page.PageID == -1 {
		return false, nil
	}

	return true, nil
}

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

	respBody, err := w.client.GetRaw(p)
	if err != nil {
		return nil, err
	}
	logRawJSON("GetCategoriesWithPrefix response", respBody)

	var res mwAllCategoriesResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return []string{}, nil
	}

	var titles []string
	for _, cat := range res.Query.AllCategories {
		name := cat.Category
		if name == "" {
			name = cat.Star
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

	respBody, err := w.client.GetRaw(p)
	if err != nil {
		return nil, err
	}
	logRawJSON("GetCategoryMembers response", respBody)

	var res mwCategoryMembersResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return []string{}, nil
	}

	var titles []string
	for _, member := range res.Query.CategoryMembers {
		if member.Title != "" {
			titles = append(titles, member.Title)
		}
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
