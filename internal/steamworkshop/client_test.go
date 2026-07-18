package steamworkshop_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/steamworkshop"
)

// queryFilesResponse builds a minimal QueryFiles/v1 response body.
func queryFilesResponse(items []map[string]any, nextCursor string, total int) []byte {
	body := map[string]any{
		"response": map[string]any{
			"total":                total,
			"next_cursor":          nextCursor,
			"publishedfiledetails": items,
		},
	}
	b, _ := json.Marshal(body)
	return b
}

// getDetailsResponse builds a minimal GetDetails/v1 response body.
func getDetailsResponse(items []map[string]any) []byte {
	body := map[string]any{
		"response": map[string]any{
			"publishedfiledetails": items,
		},
	}
	b, _ := json.Marshal(body)
	return b
}

// redirectTransport rewrites requests to the test server base URL,
// keeping the path and query string.
type redirectTransport struct {
	base string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := t.base + req.URL.Path + "?" + req.URL.RawQuery
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	return http.DefaultTransport.RoundTrip(newReq)
}

func newTestClient(srv *httptest.Server) *http.Client {
	return &http.Client{Transport: &redirectTransport{base: srv.URL}}
}

// ids parses publishedfileids[0], [1], … from query string.
func parseIDs(r *http.Request) []string {
	q := r.URL.Query()
	var ids []string
	for i := 0; ; i++ {
		id := q.Get("publishedfileids[" + strconv.Itoa(i) + "]")
		if id == "" {
			break
		}
		ids = append(ids, id)
	}
	return ids
}

func TestSearch_basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("appid") != "1623730" {
			t.Errorf("expected appid=1623730, got %s", r.URL.Query().Get("appid"))
		}
		body := queryFilesResponse([]map[string]any{
			{
				"publishedfileid":   "111",
				"title":             "Test Mod",
				"short_description": "A test mod",
				"preview_url":       "https://example.com/thumb.jpg",
				"creator":           "creator_steam_id",
				"subscriptions":     1234,
				"views":             5678,
				"time_updated":      int64(1700000000),
				"tags":              []map[string]any{{"tag": "Gameplay"}},
			},
		}, "cursor_next", 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	result, err := steamworkshop.Search(context.Background(), newTestClient(srv), "test_key", "palworld", "*", 10)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	item := result.Items[0]
	if item.WorkshopID != "111" {
		t.Errorf("WorkshopID: got %q want %q", item.WorkshopID, "111")
	}
	if item.Title != "Test Mod" {
		t.Errorf("Title: got %q", item.Title)
	}
	if len(item.Tags) != 1 || item.Tags[0] != "Gameplay" {
		t.Errorf("Tags: got %v", item.Tags)
	}
	if result.NextCursor != "cursor_next" {
		t.Errorf("NextCursor: got %q", result.NextCursor)
	}
	if result.Total != 1 {
		t.Errorf("Total: got %d", result.Total)
	}
}

func TestSearch_numPerPageClamping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := r.URL.Query().Get("numperpage")
		if n != "100" {
			t.Errorf("expected numperpage=100 for input 999, got %s", n)
		}
		_, _ = w.Write(queryFilesResponse(nil, "", 0))
	}))
	defer srv.Close()
	_, err := steamworkshop.Search(context.Background(), newTestClient(srv), "k", "q", "*", 999)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetDetails_children(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := getDetailsResponse([]map[string]any{
			{
				"publishedfileid": "200",
				"title":           "Dep Mod",
				"preview_url":     "https://example.com/dep.jpg",
				"children": []map[string]any{
					{"publishedfileid": "300"},
					{"publishedfileid": "400"},
				},
			},
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	details, err := steamworkshop.GetDetails(context.Background(), newTestClient(srv), "k", []string{"200"})
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}
	if len(details[0].Children) != 2 {
		t.Fatalf("expected 2 children, got %d: %v", len(details[0].Children), details[0].Children)
	}
}

func TestResolveDependencies_recursive(t *testing.T) {
	// Dependency graph: root → [A, B], A → [C], B → [], C → []
	detailsMap := map[string]map[string]any{
		"root": {
			"publishedfileid": "root", "title": "Root Mod", "preview_url": "",
			"children": []map[string]any{{"publishedfileid": "A"}, {"publishedfileid": "B"}},
		},
		"A": {
			"publishedfileid": "A", "title": "Mod A", "preview_url": "",
			"children": []map[string]any{{"publishedfileid": "C"}},
		},
		"B": {
			"publishedfileid": "B", "title": "Mod B", "preview_url": "",
			"children": []map[string]any{},
		},
		"C": {
			"publishedfileid": "C", "title": "Mod C", "preview_url": "",
			"children": []map[string]any{},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var items []map[string]any
		for _, id := range parseIDs(r) {
			if d, ok := detailsMap[id]; ok {
				items = append(items, d)
			}
		}
		_, _ = w.Write(getDetailsResponse(items))
	}))
	defer srv.Close()

	deps, err := steamworkshop.ResolveDependencies(context.Background(), newTestClient(srv), "k", "root")
	if err != nil {
		t.Fatalf("ResolveDependencies error: %v", err)
	}

	if len(deps) != 3 {
		t.Fatalf("expected 3 deps (A,B,C), got %d: %+v", len(deps), deps)
	}
	ids := map[string]bool{}
	for _, d := range deps {
		ids[d.WorkshopID] = true
	}
	for _, want := range []string{"A", "B", "C"} {
		if !ids[want] {
			t.Errorf("missing dep %q in result", want)
		}
	}
	if ids["root"] {
		t.Error("root should not appear in dependencies")
	}
}

func TestResolveDependencies_cycle(t *testing.T) {
	// Cycle: root → A → root — should terminate, return only A.
	detailsMap := map[string]map[string]any{
		"root": {
			"publishedfileid": "root", "title": "Root", "preview_url": "",
			"children": []map[string]any{{"publishedfileid": "A"}},
		},
		"A": {
			"publishedfileid": "A", "title": "A", "preview_url": "",
			"children": []map[string]any{{"publishedfileid": "root"}}, // cycle back
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var items []map[string]any
		for _, id := range parseIDs(r) {
			if d, ok := detailsMap[id]; ok {
				items = append(items, d)
			}
		}
		_, _ = w.Write(getDetailsResponse(items))
	}))
	defer srv.Close()

	deps, err := steamworkshop.ResolveDependencies(context.Background(), newTestClient(srv), "k", "root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 || deps[0].WorkshopID != "A" {
		t.Errorf("expected [A], got %+v", deps)
	}
}

func TestResolveDependencies_dedup(t *testing.T) {
	// Diamond: root → [A, B], A → [C], B → [C] — C should appear exactly once.
	detailsMap := map[string]map[string]any{
		"root": {
			"publishedfileid": "root", "title": "Root", "preview_url": "",
			"children": []map[string]any{{"publishedfileid": "A"}, {"publishedfileid": "B"}},
		},
		"A": {
			"publishedfileid": "A", "title": "A", "preview_url": "",
			"children": []map[string]any{{"publishedfileid": "C"}},
		},
		"B": {
			"publishedfileid": "B", "title": "B", "preview_url": "",
			"children": []map[string]any{{"publishedfileid": "C"}},
		},
		"C": {
			"publishedfileid": "C", "title": "C", "preview_url": "",
			"children": []map[string]any{},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var items []map[string]any
		for _, id := range parseIDs(r) {
			if d, ok := detailsMap[id]; ok {
				items = append(items, d)
			}
		}
		_, _ = w.Write(getDetailsResponse(items))
	}))
	defer srv.Close()

	deps, err := steamworkshop.ResolveDependencies(context.Background(), newTestClient(srv), "k", "root")
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]int{}
	for _, d := range deps {
		seen[d.WorkshopID]++
	}
	if seen["C"] != 1 {
		t.Errorf("C should appear exactly once, got %d: %+v", seen["C"], deps)
	}
	// A and B should also appear.
	for _, id := range []string{"A", "B"} {
		if seen[id] != 1 {
			t.Errorf("%s should appear exactly once, got %d", id, seen[id])
		}
	}
}
