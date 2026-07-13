// Package palapi is a thin HTTP client for the Palworld dedicated server's
// official REST API (https://docs.palworldgame.com/category/rest-api).
//
// It has no dependency on gin or gorm: callers construct a Client from a port
// and the server's AdminPassword, then invoke the endpoint methods. Every
// request uses HTTP Basic Auth with the fixed username "admin". Errors are
// normalized so the caller can distinguish an unreachable server (connection
// refused / timeout) from an authenticated-but-failing request.
package palapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultTimeout bounds every REST call. The manager and game server run on the
// same host, so 5s is generous while still failing fast when the server is down.
const defaultTimeout = 5 * time.Second

// ErrUnreachable is the sentinel wrapped by all connection-level failures
// (connection refused, DNS, timeout). Callers use IsUnreachable to decide
// reachable=false without inspecting the underlying transport error.
var ErrUnreachable = errors.New("palapi: server unreachable")

// Client talks to a single Palworld server's REST API.
type Client struct {
	BaseURL  string
	Password string
	HTTP     *http.Client
}

// New builds a Client for 127.0.0.1:<port>. The manager and game server are
// co-located, so the host is always loopback.
func New(port int, password string) *Client {
	return &Client{
		BaseURL:  fmt.Sprintf("http://127.0.0.1:%d/v1/api", port),
		Password: password,
		HTTP:     &http.Client{Timeout: defaultTimeout},
	}
}

// IsUnreachable reports whether err originated from a connection-level failure
// (as opposed to a non-2xx HTTP response). Used to derive reachable=false.
func IsUnreachable(err error) bool {
	return errors.Is(err, ErrUnreachable)
}

// --- response structures ---

// Info mirrors GET /v1/api/info.
type Info struct {
	Version     string `json:"version"`
	ServerName  string `json:"servername"`
	Description string `json:"description"`
	WorldGUID   string `json:"worldguid"`
}

// Metrics mirrors GET /v1/api/metrics.
type Metrics struct {
	ServerFPS        int     `json:"serverfps"`
	CurrentPlayerNum int     `json:"currentplayernum"`
	ServerFrameTime  float64 `json:"serverframetime"`
	MaxPlayerNum     int     `json:"maxplayernum"`
	Uptime           int     `json:"uptime"`
	Days             int     `json:"days"`
}

// Player mirrors one entry of GET /v1/api/players.
type Player struct {
	Name          string  `json:"name"`
	AccountName   string  `json:"accountName"`
	PlayerID      string  `json:"playerId"`
	UserID        string  `json:"userId"`
	IP            string  `json:"ip"`
	Ping          float64 `json:"ping"`
	LocationX     float64 `json:"location_x"`
	LocationY     float64 `json:"location_y"`
	Level         int     `json:"level"`
	BuildingCount int     `json:"building_count"`
}

// Players mirrors GET /v1/api/players.
type Players struct {
	Players []Player `json:"players"`
}

// --- GET endpoints ---

// Info returns basic server identity.
func (c *Client) Info(ctx context.Context) (Info, error) {
	var out Info
	err := c.doJSON(ctx, http.MethodGet, "/info", nil, &out)
	return out, err
}

// Metrics returns runtime metrics (fps, player count, uptime, ...).
func (c *Client) Metrics(ctx context.Context) (Metrics, error) {
	var out Metrics
	err := c.doJSON(ctx, http.MethodGet, "/metrics", nil, &out)
	return out, err
}

// Players returns the list of currently connected players.
func (c *Client) Players(ctx context.Context) (Players, error) {
	var out Players
	err := c.doJSON(ctx, http.MethodGet, "/players", nil, &out)
	return out, err
}

// Settings returns the server's currently effective settings object.
func (c *Client) Settings(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.doJSON(ctx, http.MethodGet, "/settings", nil, &out)
	return out, err
}

// --- POST endpoints ---

// Announce broadcasts message to all players.
func (c *Client) Announce(ctx context.Context, message string) error {
	return c.doJSON(ctx, http.MethodPost, "/announce", map[string]any{"message": message}, nil)
}

// Kick disconnects a player identified by userid, showing message.
func (c *Client) Kick(ctx context.Context, userid, message string) error {
	return c.doJSON(ctx, http.MethodPost, "/kick", map[string]any{"userid": userid, "message": message}, nil)
}

// Ban bans a player identified by userid, showing message.
func (c *Client) Ban(ctx context.Context, userid, message string) error {
	return c.doJSON(ctx, http.MethodPost, "/ban", map[string]any{"userid": userid, "message": message}, nil)
}

// Unban lifts a ban for userid.
func (c *Client) Unban(ctx context.Context, userid string) error {
	return c.doJSON(ctx, http.MethodPost, "/unban", map[string]any{"userid": userid}, nil)
}

// Save forces a world save.
func (c *Client) Save(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodPost, "/save", nil, nil)
}

// Shutdown schedules a graceful shutdown after waittime seconds, broadcasting
// message beforehand.
func (c *Client) Shutdown(ctx context.Context, waittime int, message string) error {
	return c.doJSON(ctx, http.MethodPost, "/shutdown", map[string]any{"waittime": waittime, "message": message}, nil)
}

// Stop stops the server immediately.
func (c *Client) Stop(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodPost, "/stop", nil, nil)
}

// --- internal helpers ---

// doJSON performs an HTTP request with Basic Auth, encoding body as JSON (when
// non-nil) and decoding the response into out (when non-nil). It normalizes
// connection failures to ErrUnreachable and non-2xx responses to a message
// carrying the server's body (with a friendlier message for 401).
func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("palapi: encode request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return fmt.Errorf("palapi: build request: %w", err)
	}
	req.Header.Set("Authorization", c.basicAuth())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// Connection refused, DNS failure, or timeout: the server is not
		// answering. Normalize so the caller can report reachable=false.
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.statusError(resp)
	}

	if out == nil {
		// Drain the body so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("palapi: decode response: %w", err)
	}
	return nil
}

// statusError builds an error for a non-2xx response, giving 401 a clear
// credential message and otherwise surfacing the trimmed response body.
func (c *Client) statusError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(data))
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("palapi: invalid credentials (check AdminPassword)")
	}
	if msg == "" {
		return fmt.Errorf("palapi: unexpected status %d", resp.StatusCode)
	}
	return fmt.Errorf("palapi: status %d: %s", resp.StatusCode, msg)
}

// basicAuth returns the Authorization header value for user "admin".
func (c *Client) basicAuth() string {
	raw := "admin:" + c.Password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}
