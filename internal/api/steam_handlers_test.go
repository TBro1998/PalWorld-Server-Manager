package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/settings"
)

func TestSteamStatus(t *testing.T) {
	eng, r := newTestRouter(t)

	// Nothing configured yet.
	w := doJSON(t, eng, http.MethodGet, "/api/steam/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Username     string `json:"username"`
		SessionReady bool   `json:"sessionReady"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Username != "" || resp.SessionReady {
		t.Errorf("expected unconfigured status, got %+v", resp)
	}

	// Simulate a successful login persisting the username + session flag.
	if err := settings.Set(r.db, settings.KeySteamUsername, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := settings.Set(r.db, settings.KeySteamSessionReady, "true"); err != nil {
		t.Fatal(err)
	}

	w = doJSON(t, eng, http.MethodGet, "/api/steam/status", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Username != "bob" || !resp.SessionReady {
		t.Errorf("expected {bob,true}, got %+v", resp)
	}

	// Set is an upsert: overwriting a key does not error or duplicate.
	if err := settings.Set(r.db, settings.KeySteamUsername, "alice"); err != nil {
		t.Fatal(err)
	}
	got, err := settings.Get(r.db, settings.KeySteamUsername)
	if err != nil {
		t.Fatal(err)
	}
	if got != "alice" {
		t.Errorf("expected upsert to alice, got %q", got)
	}
}

func TestSteamLoginValidation(t *testing.T) {
	eng, _ := newTestRouter(t)

	// Missing password → 400 (binding required), no steamcmd invocation.
	w := doJSON(t, eng, http.MethodPost, "/api/steam/login", map[string]string{"username": "bob"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing password, got %d body=%s", w.Code, w.Body.String())
	}
}
