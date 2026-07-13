package palapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient points a Client at the given test server, keeping the real
// transport but overriding BaseURL.
func newTestClient(baseURL, password string) *Client {
	c := New(8212, password)
	c.BaseURL = baseURL + "/v1/api"
	return c
}

func TestInfoSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/info" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Verify Basic Auth for user "admin".
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("bad basic auth: user=%q pass=%q ok=%v", user, pass, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Info{
			Version:     "v0.1.5.0",
			ServerName:  "My Server",
			Description: "hello",
			WorldGUID:   "abc",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "secret")
	got, err := c.Info(context.Background())
	if err != nil {
		t.Fatalf("Info returned error: %v", err)
	}
	if got.ServerName != "My Server" || got.Version != "v0.1.5.0" {
		t.Fatalf("unexpected info: %+v", got)
	}
}

func TestAnnounceSendsMessage(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/api/announce" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("unexpected content-type: %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "secret")
	if err := c.Announce(context.Background(), "restart soon"); err != nil {
		t.Fatalf("Announce returned error: %v", err)
	}
	if received["message"] != "restart soon" {
		t.Fatalf("unexpected announce body: %+v", received)
	}
}

func TestUnauthorizedNormalized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "Unauthorized")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "wrong")
	_, err := c.Info(context.Background())
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if IsUnreachable(err) {
		t.Fatal("401 must not be classified as unreachable")
	}
	if !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("expected credential message, got: %v", err)
	}
}

func TestNon2xxCarriesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "bad param")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "secret")
	err := c.Save(context.Background())
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if IsUnreachable(err) {
		t.Fatal("400 must not be classified as unreachable")
	}
	if !strings.Contains(err.Error(), "bad param") {
		t.Fatalf("expected body in error, got: %v", err)
	}
}

func TestUnreachableNormalized(t *testing.T) {
	// Start then immediately close a server so the port refuses connections.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := newTestClient(url, "secret")
	_, err := c.Info(context.Background())
	if err == nil {
		t.Fatal("expected error for closed server")
	}
	if !IsUnreachable(err) {
		t.Fatalf("expected unreachable error, got: %v", err)
	}
}

func TestBasicAuthHeader(t *testing.T) {
	c := New(8212, "pw")
	got := c.basicAuth()
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:pw"))
	if got != want {
		t.Fatalf("basicAuth = %q, want %q", got, want)
	}
}
