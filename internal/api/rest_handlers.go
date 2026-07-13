package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/palapi"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palconfig"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/process"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// REST-availability reason codes returned by /rest/status and used by the
// frontend to render the correct guidance message.
const (
	reasonOK            = ""
	reasonNotFound      = "not_found"
	reasonNotRunning    = "not_running"
	reasonDisabled      = "restapi_disabled"
	reasonPasswordEmpty = "admin_password_empty"
	reasonUnreachable   = "unreachable"
)

// defaultRESTAPIPort is used when RESTAPIPort is missing or unparsable. Mirrors the
// registry default in palconfig/schema.go.
const defaultRESTAPIPort = 8212

// restResolution is the outcome of resolveRest: everything a REST handler needs
// to either forward a call or explain why it cannot. AdminPassword lives only
// inside client and is never surfaced to the caller/HTTP response.
type restResolution struct {
	client  *palapi.Client
	running bool
	enabled bool
	port    int
	reason  string
}

// ready reports whether the server can accept REST calls (running + enabled +
// non-empty password). When false, reason explains why.
func (r restResolution) ready() bool {
	return r.running && r.enabled && r.reason == reasonOK
}

// resolveRest loads a server's install path, derives its status, reads the
// PalWorldSettings.ini REST fields (single source of truth — never cached in
// the DB), and constructs a palapi.Client. The INI is the authority for
// port/password/enabled, matching GetServerConfig's approach.
//
// It returns (resolution, httpStatus, err). err is non-nil only for lookup
// failures that should short-circuit with httpStatus; availability problems
// (not running / disabled / empty password) are reported via the resolution's
// reason and enabled/running flags with a nil err so callers (notably
// RestStatus) can still respond structurally.
func (r *Router) resolveRest(id int64) (restResolution, int, error) {
	var res restResolution

	installPath, lastError, _, _, err := r.loadServerPathState(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			res.reason = reasonNotFound
			return res, http.StatusNotFound, errors.New("Server not found")
		}
		return res, http.StatusInternalServerError, errors.New("Failed to query server")
	}

	res.running = r.process.DeriveStatus(id, lastError) == process.StatusRunning
	if !res.running {
		res.reason = reasonNotRunning
	}

	// The INI is the source of truth for REST settings; read it on demand.
	settings, err := palconfig.LoadSettings(installPath)
	if err != nil {
		return res, http.StatusInternalServerError, err
	}

	res.enabled = parseBool(settings["RESTAPIEnabled"])
	res.port = parsePort(settings["RESTAPIPort"])
	password := settings["AdminPassword"]

	// Determine the most relevant blocking reason. Running is checked first so
	// a stopped server reports not_running even if REST is disabled.
	switch {
	case !res.running:
		res.reason = reasonNotRunning
	case !res.enabled:
		res.reason = reasonDisabled
	case password == "":
		res.reason = reasonPasswordEmpty
	default:
		res.reason = reasonOK
	}

	res.client = palapi.New(res.port, password)
	return res, http.StatusOK, nil
}

// parseBool interprets Palworld INI booleans ("True"/"False", case-insensitive).
func parseBool(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "True")
}

// parsePort converts the RESTAPIPort string to an int, falling back to the
// default when empty or unparsable.
func parsePort(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return defaultRESTAPIPort
	}
	return n
}

// restIDAndResolve parses :id and resolves REST state, writing the appropriate
// error response on failure. It returns (resolution, ok); when ok is false the
// caller must return immediately (response already written).
func (r *Router) restIDAndResolve(c *gin.Context) (restResolution, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return restResolution{}, false
	}
	res, code, rerr := r.resolveRest(id)
	if rerr != nil {
		c.JSON(code, gin.H{"error": rerr.Error()})
		return restResolution{}, false
	}
	return res, true
}

// requireReady is used by forwarding handlers: it resolves and enforces
// readiness, writing a 4xx error when the server cannot accept REST calls.
func (r *Router) requireReady(c *gin.Context) (restResolution, bool) {
	res, ok := r.restIDAndResolve(c)
	if !ok {
		return restResolution{}, false
	}
	if !res.ready() {
		c.JSON(http.StatusBadRequest, gin.H{"error": readyErrorMessage(res.reason)})
		return restResolution{}, false
	}
	return res, true
}

// readyErrorMessage maps a blocking reason to a human-readable error.
func readyErrorMessage(reason string) string {
	switch reason {
	case reasonNotRunning:
		return "Server is not running"
	case reasonDisabled:
		return "REST API is disabled (set RESTAPIEnabled=True and restart)"
	case reasonPasswordEmpty:
		return "AdminPassword is empty (set it in settings and restart)"
	default:
		return "REST API is unavailable"
	}
}

// forwardErr writes the correct HTTP status for a palapi call failure:
// 502 for an unreachable server, 400 otherwise. Never leaks credentials.
func forwardErr(c *gin.Context, err error) {
	if palapi.IsUnreachable(err) {
		c.JSON(http.StatusBadGateway, gin.H{"error": "REST API unreachable"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

// --- status ---

// RestStatusResponse is the structured availability payload. It deliberately
// omits AdminPassword.
type RestStatusResponse struct {
	Enabled   bool   `json:"enabled"`
	Running   bool   `json:"running"`
	Reachable bool   `json:"reachable"`
	Port      int    `json:"port"`
	Reason    string `json:"reason"`
	// Info carries the payload from the reachability probe (a /v1/api/info call).
	// Exposing it lets the frontend render server info without a second, redundant
	// fetch. Present only when reachable; omitted otherwise.
	Info *palapi.Info `json:"info,omitempty"`
}

// RestStatus reports whether the REST API is usable. Unlike the forwarding
// handlers it always returns 200 with a structured body (except for lookup
// failures), so the frontend can render guidance.
func (r *Router) RestStatus(c *gin.Context) {
	res, ok := r.restIDAndResolve(c)
	if !ok {
		return
	}

	resp := RestStatusResponse{
		Enabled: res.enabled,
		Running: res.running,
		Port:    res.port,
		Reason:  res.reason,
	}

	// Only probe reachability when the server would otherwise be usable; a
	// lightweight Info call doubles as the connectivity check. The result is
	// returned in the response so the frontend can reuse it instead of issuing a
	// second /v1/api/info request for the same data.
	if res.ready() {
		if info, err := res.client.Info(c.Request.Context()); err != nil {
			// Either connection-level failure or an auth/other error: from the
			// frontend's perspective the API is not usable, so surface it as
			// unreachable regardless of the underlying cause.
			resp.Reachable = false
			resp.Reason = reasonUnreachable
		} else {
			resp.Reachable = true
			resp.Reason = reasonOK
			resp.Info = &info
		}
	}

	c.JSON(http.StatusOK, resp)
}

// --- GET forwarders ---

// RestInfo forwards GET /info.
func (r *Router) RestInfo(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	out, err := res.client.Info(c.Request.Context())
	if err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// RestMetrics forwards GET /metrics.
func (r *Router) RestMetrics(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	out, err := res.client.Metrics(c.Request.Context())
	if err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// RestPlayers forwards GET /players.
func (r *Router) RestPlayers(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	out, err := res.client.Players(c.Request.Context())
	if err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// RestSettings forwards GET /settings.
func (r *Router) RestSettings(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	out, err := res.client.Settings(c.Request.Context())
	if err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// --- POST forwarders ---

type announceRequest struct {
	Message string `json:"message" binding:"required"`
}

// RestAnnounce forwards POST /announce.
func (r *Router) RestAnnounce(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	var req announceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := res.client.Announce(c.Request.Context(), req.Message); err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "announced"})
}

type kickBanRequest struct {
	UserID  string `json:"userid" binding:"required"`
	Message string `json:"message"`
}

// RestKick forwards POST /kick.
func (r *Router) RestKick(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	var req kickBanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := res.client.Kick(c.Request.Context(), req.UserID, req.Message); err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "kicked"})
}

// RestBan forwards POST /ban.
func (r *Router) RestBan(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	var req kickBanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := res.client.Ban(c.Request.Context(), req.UserID, req.Message); err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "banned"})
}

type unbanRequest struct {
	UserID string `json:"userid" binding:"required"`
}

// RestUnban forwards POST /unban.
func (r *Router) RestUnban(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	var req unbanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := res.client.Unban(c.Request.Context(), req.UserID); err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unbanned"})
}

// RestSave forwards POST /save.
func (r *Router) RestSave(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	if err := res.client.Save(c.Request.Context()); err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "saved"})
}

type shutdownRequest struct {
	// WaitTime is the countdown in seconds. Pointer so 0 is distinguishable
	// from "omitted"; required so callers must supply it explicitly.
	WaitTime *int   `json:"waittime" binding:"required"`
	Message  string `json:"message"`
}

// RestShutdown forwards POST /shutdown.
func (r *Router) RestShutdown(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	var req shutdownRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := res.client.Shutdown(c.Request.Context(), *req.WaitTime, req.Message); err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "shutdown scheduled"})
}

// RestStop forwards POST /stop.
func (r *Router) RestStop(c *gin.Context) {
	res, ok := r.requireReady(c)
	if !ok {
		return
	}
	if err := res.client.Stop(c.Request.Context()); err != nil {
		forwardErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "stopping"})
}
