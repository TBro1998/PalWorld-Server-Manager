// Package steamworkshop provides a thin read-only client for the Steam Web API
// (IPublishedFileService) scoped to Palworld workshop content (App 1623730).
//
// All functions accept an *http.Client so callers can inject test doubles; pass
// nil to use http.DefaultClient. The API key is passed per-call so handlers can
// read it from settings without threading it through a constructor.
//
// No DB or model dependencies — matches the palmod/palsave/palconfig pattern of
// pure-logic packages that can be unit-tested without a running server.
package steamworkshop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// AppID is the Palworld game-client Steam App ID that owns workshop content.
// Workshop items are published against this ID (not the dedicated-server App ID
// 2394010). Cross-referenced with steamcmd.palworldClientAppID — both must stay
// "1623730"; if that constant changes, update this one too.
const AppID = "1623730"

// steamAPIBase is the base URL for Steam Web API calls.
const steamAPIBase = "https://api.steampowered.com"

// maxRecurseDepth caps recursive dependency resolution to prevent runaway
// traversal on malformed or circular dependency graphs.
const maxRecurseDepth = 5

// Item is a single workshop entry returned by Search.
type Item struct {
	WorkshopID   string   `json:"workshop_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`   // short description (may be empty)
	PreviewURL   string   `json:"preview_url"`   // thumbnail image URL
	Author       string   `json:"author"`        // Steam display name of creator
	Subscriptions int     `json:"subscriptions"` // lifetime subscriber count
	Views        int      `json:"views"`
	TimeUpdated  int64    `json:"time_updated"` // Unix timestamp
	Tags         []string `json:"tags"`
}

// SearchResult is the response envelope for Search.
type SearchResult struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"next_cursor"` // pass as cursor in the next call; empty = last page
	Total      int    `json:"total"`
}

// DepItem is a single dependency entry returned by ResolveDependencies.
type DepItem struct {
	WorkshopID  string `json:"workshop_id"`
	Title       string `json:"title"`
	PreviewURL  string `json:"preview_url"`
}

// Search queries the Palworld Steam Workshop via IPublishedFileService/QueryFiles/v1.
//
// query is the free-text search string (empty returns trending/popular items).
// cursor should be "*" for the first page; pass the NextCursor from the
// previous result to paginate. numPerPage is clamped to [1, 100].
func Search(ctx context.Context, client *http.Client, key, query, cursor string, numPerPage int) (SearchResult, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if numPerPage < 1 {
		numPerPage = 20
	}
	if numPerPage > 100 {
		numPerPage = 100
	}
	if cursor == "" {
		cursor = "*"
	}

	params := url.Values{}
	params.Set("key", key)
	params.Set("appid", AppID)
	params.Set("query_type", "12") // EPublishedFileQueryType::k_PublishedFileQueryType_RankedByTextSearch
	params.Set("search_text", query)
	params.Set("cursor", cursor)
	params.Set("numperpage", strconv.Itoa(numPerPage))
	params.Set("return_short_description", "true")
	params.Set("return_previews", "true")
	params.Set("return_tags", "true")
	params.Set("return_metadata", "true")

	endpoint := steamAPIBase + "/IPublishedFileService/QueryFiles/v1/?" + params.Encode()
	body, err := doGet(ctx, client, endpoint)
	if err != nil {
		return SearchResult{}, err
	}

	var raw struct {
		Response struct {
			Total      int    `json:"total"`
			NextCursor string `json:"next_cursor"`
			PublishedFileDetails []struct {
				PublishedFileID string `json:"publishedfileid"`
				Title           string `json:"title"`
				ShortDescription string `json:"short_description"`
				PreviewURL      string `json:"preview_url"`
				Creator         string `json:"creator"`
				Subscriptions   int    `json:"subscriptions"`
				Views           int    `json:"views"`
				TimeUpdated     int64  `json:"time_updated"`
				Tags            []struct {
					Tag string `json:"tag"`
				} `json:"tags"`
			} `json:"publishedfiledetails"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return SearchResult{}, fmt.Errorf("steamworkshop: parse QueryFiles response: %w", err)
	}

	items := make([]Item, 0, len(raw.Response.PublishedFileDetails))
	for _, d := range raw.Response.PublishedFileDetails {
		tags := make([]string, 0, len(d.Tags))
		for _, t := range d.Tags {
			if t.Tag != "" {
				tags = append(tags, t.Tag)
			}
		}
		items = append(items, Item{
			WorkshopID:    d.PublishedFileID,
			Title:         d.Title,
			Description:   d.ShortDescription,
			PreviewURL:    d.PreviewURL,
			Author:        d.Creator,
			Subscriptions: d.Subscriptions,
			Views:         d.Views,
			TimeUpdated:   d.TimeUpdated,
			Tags:          tags,
		})
	}

	nextCursor := raw.Response.NextCursor
	// Steam returns "*" as the cursor on the last page in some contexts; treat it
	// the same as the initial cursor — let the frontend detect "no more pages" by
	// comparing total vs items-seen or checking for an empty next page instead.
	// We normalise: if next_cursor equals the cursor we just sent, it's the last page.
	if nextCursor == cursor && len(items) < numPerPage {
		nextCursor = ""
	}

	return SearchResult{
		Items:      items,
		NextCursor: nextCursor,
		Total:      raw.Response.Total,
	}, nil
}

// DetailItem is a workshop item with its direct child/dependency IDs.
type DetailItem struct {
	WorkshopID string
	Title      string
	PreviewURL string
	Children   []string // direct Steam dependency/child publishedfileids
}

// GetDetails fetches workshop item details (including children) for a batch of IDs.
// ids must be non-empty and ≤ 100 items (Steam API limit per call).
func GetDetails(ctx context.Context, client *http.Client, key string, ids []string) ([]DetailItem, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if len(ids) == 0 {
		return nil, nil
	}

	params := url.Values{}
	params.Set("key", key)
	params.Set("return_children", "true")
	params.Set("return_short_description", "true")
	params.Set("return_previews", "true")
	for i, id := range ids {
		params.Set(fmt.Sprintf("publishedfileids[%d]", i), id)
	}

	endpoint := steamAPIBase + "/IPublishedFileService/GetDetails/v1/?" + params.Encode()
	body, err := doGet(ctx, client, endpoint)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Response struct {
			PublishedFileDetails []struct {
				PublishedFileID string `json:"publishedfileid"`
				Title           string `json:"title"`
				PreviewURL      string `json:"preview_url"`
				Children        []struct {
					PublishedFileID string `json:"publishedfileid"`
				} `json:"children"`
			} `json:"publishedfiledetails"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("steamworkshop: parse GetDetails response: %w", err)
	}

	result := make([]DetailItem, 0, len(raw.Response.PublishedFileDetails))
	for _, d := range raw.Response.PublishedFileDetails {
		children := make([]string, 0, len(d.Children))
		for _, c := range d.Children {
			if c.PublishedFileID != "" {
				children = append(children, c.PublishedFileID)
			}
		}
		result = append(result, DetailItem{
			WorkshopID: d.PublishedFileID,
			Title:      d.Title,
			PreviewURL: d.PreviewURL,
			Children:   children,
		})
	}
	return result, nil
}

// ResolveDependencies recursively resolves all Steam Workshop dependencies of
// rootID (i.e., its transitive children), up to maxRecurseDepth levels deep.
//
// The returned slice is flat, deduplicated, and does NOT include rootID itself.
// Items appear in BFS order (direct deps first, then their deps, etc.).
// If a dependency graph contains cycles (unusual but possible), the visited set
// prevents infinite recursion.
func ResolveDependencies(ctx context.Context, client *http.Client, key, rootID string) ([]DepItem, error) {
	visited := map[string]bool{rootID: true}
	var result []DepItem
	queue := []string{rootID}

	for depth := 0; depth < maxRecurseDepth && len(queue) > 0; depth++ {
		// Collect all children at the current frontier.
		details, err := GetDetails(ctx, client, key, queue)
		if err != nil {
			return result, fmt.Errorf("steamworkshop: dependency resolution (depth %d): %w", depth, err)
		}

		nextQueue := []string{}
		for _, d := range details {
			for _, childID := range d.Children {
				childID = strings.TrimSpace(childID)
				if childID == "" || visited[childID] {
					continue
				}
				visited[childID] = true
				nextQueue = append(nextQueue, childID)
				// We don't have the title/preview for newly discovered children yet;
				// they'll be populated when GetDetails is called on the next iteration.
				// For root's direct children we got details already, look up in details.
			}
		}

		// Populate DepItems for items we got details for (excluding root on first pass).
		for _, d := range details {
			if d.WorkshopID == rootID {
				continue // skip root
			}
			result = append(result, DepItem{
				WorkshopID: d.WorkshopID,
				Title:      d.Title,
				PreviewURL: d.PreviewURL,
			})
		}

		queue = nextQueue
	}

	return result, nil
}

// doGet performs a GET request and returns the response body.
func doGet(ctx context.Context, client *http.Client, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("steamworkshop: build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("steamworkshop: HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("steamworkshop: read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// Truncate body to avoid leaking API key from error responses.
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200] + "…"
		}
		return nil, fmt.Errorf("steamworkshop: Steam API returned %d: %s", resp.StatusCode, preview)
	}
	return body, nil
}
