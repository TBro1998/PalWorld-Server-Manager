package api

import (
	"net/http"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/auth"
	"github.com/gin-gonic/gin"
)

// AuthStatus returns whether the admin password has been configured yet.
// This is a public endpoint used by the frontend to decide whether to show
// the first-time setup screen or the login screen.
//
// GET /api/auth/status
func (r *Router) AuthStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"configured": r.config.Configured(),
	})
}

// SetupRequest is the body for POST /api/auth/setup.
type SetupRequest struct {
	Password string `json:"password" binding:"required,min=6"`
}

// Setup sets the initial admin password. Only available when no password has
// been configured yet (i.e. config.yaml does not exist or has an empty
// password_hash). On success it creates / overwrites config.yaml with the
// bcrypt hash and a freshly generated jwt_secret, then returns a JWT so the
// user is immediately logged in without a second round-trip.
//
// POST /api/auth/setup
func (r *Router) Setup(c *gin.Context) {
	if r.config.Configured() {
		c.JSON(http.StatusConflict, gin.H{"error": "password already configured; use /api/auth/login"})
		return
	}

	var req SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate a random JWT secret.
	secret, err := auth.GenerateSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate secret"})
		return
	}

	// Hash the password.
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// Persist to config.yaml; host and port keep their current (default) values.
	r.config.JWTSecret = secret
	r.config.PasswordHash = hash
	if err := r.config.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	// Issue a JWT so the user is logged in immediately.
	token, err := auth.GenerateToken(secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// LoginRequest is the body for POST /api/auth/login.
type LoginRequest struct {
	Password string `json:"password" binding:"required"`
}

// Login validates the admin password and returns a JWT.
//
// POST /api/auth/login
func (r *Router) Login(c *gin.Context) {
	if !r.config.Configured() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not configured; use /api/auth/setup"})
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !auth.CheckPassword(r.config.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect password"})
		return
	}

	token, err := auth.GenerateToken(r.config.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// Register is kept for API compatibility but unused — this app uses a single
// admin password set via Setup rather than per-user accounts.
func (r *Router) Register(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error": "use /api/auth/setup to configure the admin password"})
}
