package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/settings"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/steamworkshop"
	"github.com/gin-gonic/gin"
)

// workshopHTTPTimeout bounds Steam Web API calls so a slow or down Steam API
// cannot hang requests indefinitely.
const workshopHTTPTimeout = 15 * time.Second

// GetWebAPIKey returns the stored Steam Web API key so the browser can call
// the Steam Workshop API directly, bypassing the server's network constraints.
//
// SECURITY: The key is a low-sensitivity read-only personal key used only for
// public workshop queries. This tool listens on 127.0.0.1 by default, so
// exposing the key over the local connection is acceptable. If the listen
// address is changed to a public interface, users should treat the key as
// semi-public workshop-search-only data.
func (r *Router) GetWebAPIKey(c *gin.Context) {
	key, err := settings.Get(r.db, settings.KeySteamWebAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read settings"})
		return
	}
	key = strings.TrimSpace(key)
	// Return an empty string when not configured; the frontend renders the
	// "no key" state and disables the browser-direct calls.
	c.JSON(http.StatusOK, gin.H{"key": key, "configured": key != ""})
}

// WorkshopSearch proxies IPublishedFileService/QueryFiles to the frontend.
// NOTE: This endpoint makes an outbound HTTPS request to api.steampowered.com
// from the server. If the server cannot reach Steam (e.g., behind a firewall),
// use the browser-direct approach via GetWebAPIKey instead.
//
// Query params:
//   - q      — free-text search string (empty = trending/popular)
//   - cursor — pagination cursor; omit or "*" for the first page
//   - num    — items per page [1, 100], default 20
func (r *Router) WorkshopSearch(c *gin.Context) {
	key, err := settings.Get(r.db, settings.KeySteamWebAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read settings"})
		return
	}
	key = strings.TrimSpace(key)
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "web_api_key_missing"})
		return
	}

	query := c.Query("q")
	cursor := c.Query("cursor")
	num, _ := strconv.Atoi(c.DefaultQuery("num", "20"))

	ctx, cancel := context.WithTimeout(c.Request.Context(), workshopHTTPTimeout)
	defer cancel()

	result, err := steamworkshop.Search(ctx, nil, key, query, cursor, num)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "steam_api_error", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// WorkshopDependencies resolves all transitive Steam Workshop dependencies of a
// single mod recursively. The result is a flat, deduplicated list of deps (not
// including the mod itself). Returns an empty slice when the mod has no
// declared Steam dependencies.
//
// NOTE: Same network caveat as WorkshopSearch — calls api.steampowered.com
// from the server. Prefer the browser-direct approach when the server is
// behind a firewall.
func (r *Router) WorkshopDependencies(c *gin.Context) {
	key, err := settings.Get(r.db, settings.KeySteamWebAPIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read settings"})
		return
	}
	key = strings.TrimSpace(key)
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "web_api_key_missing"})
		return
	}

	workshopID := strings.TrimSpace(c.Param("workshopId"))
	if workshopID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workshopId is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), workshopHTTPTimeout)
	defer cancel()

	deps, err := steamworkshop.ResolveDependencies(ctx, nil, key, workshopID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "steam_api_error", "detail": err.Error()})
		return
	}
	if deps == nil {
		deps = []steamworkshop.DepItem{}
	}
	c.JSON(http.StatusOK, gin.H{"dependencies": deps})
}

// SetWebAPIKeyRequest is the body for POST /api/steam/webapi-key.
type SetWebAPIKeyRequest struct {
	Key string `json:"key"`
}

// SetWebAPIKey persists (or clears) the Steam Web API key used for workshop
// search. Passing an empty key removes the configured value.
func (r *Router) SetWebAPIKey(c *gin.Context) {
	var req SetWebAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	trimmed := strings.TrimSpace(req.Key)
	if err := settings.Set(r.db, settings.KeySteamWebAPIKey, trimmed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"configured": trimmed != ""})
}
