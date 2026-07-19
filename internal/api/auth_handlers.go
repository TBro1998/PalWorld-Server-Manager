package api

import (
	"net/http"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/auth"
	"github.com/gin-gonic/gin"
)

// AuthStatus godoc
// @Summary      Check authentication status
// @Description  Returns whether the admin password has been configured. Public endpoint.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "configured: true/false"
// @Router       /auth/status [get]
func (r *Router) AuthStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"configured": r.config.Configured(),
	})
}

// SetupRequest is the body for POST /api/auth/setup.
type SetupRequest struct {
	Password string `json:"password" binding:"required,min=6"`
}

// Setup godoc
// @Summary      First-time admin password setup
// @Description  Sets the initial admin password. Only available when not yet configured. Returns a JWT token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      SetupRequest  true  "Password (min 6 chars)"
// @Success      200   {object}  map[string]interface{}  "token: jwt-string"
// @Failure      400   {object}  map[string]interface{}
// @Failure      409   {object}  map[string]interface{}  "Already configured"
// @Failure      500   {object}  map[string]interface{}
// @Router       /auth/setup [post]
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

// Login godoc
// @Summary      Admin login
// @Description  Validates the admin password and returns a JWT token (valid for 7 days)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest  true  "Admin password"
// @Success      200   {object}  map[string]interface{}  "token: jwt-string"
// @Failure      400   {object}  map[string]interface{}
// @Failure      401   {object}  map[string]interface{}  "Incorrect password"
// @Failure      500   {object}  map[string]interface{}
// @Router       /auth/login [post]
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
