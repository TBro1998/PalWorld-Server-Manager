package api

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/config"
)

// Router handles API routing
type Router struct {
	db     *sql.DB
	config *config.Config
}

// NewRouter creates a new API router
func NewRouter(db *sql.DB, cfg *config.Config) *Router {
	return &Router{
		db:     db,
		config: cfg,
	}
}

// RegisterRoutes registers all API routes
func (r *Router) RegisterRoutes(rg *gin.RouterGroup) {
	// Auth routes (public)
	auth := rg.Group("/auth")
	{
		auth.POST("/login", r.Login)
		auth.POST("/register", r.Register)
	}

	// Protected routes (require JWT)
	protected := rg.Group("")
	// protected.Use(authMiddleware(r.config.JWTSecret))
	{
		// Server management
		servers := protected.Group("/servers")
		{
			servers.GET("", r.ListServers)
			servers.POST("", r.CreateServer)
			servers.GET("/:id", r.GetServer)
			servers.PUT("/:id", r.UpdateServer)
			servers.DELETE("/:id", r.DeleteServer)
			servers.POST("/:id/install", r.InstallServer)
			servers.POST("/:id/start", r.StartServer)
			servers.POST("/:id/stop", r.StopServer)
			servers.POST("/:id/restart", r.RestartServer)
			servers.GET("/:id/logs", r.GetLogs)
			servers.GET("/:id/logs/stream", r.StreamLogs)
		}

		// Mod management
		mods := protected.Group("/servers/:id/mods")
		{
			mods.GET("", r.ListMods)
			mods.POST("", r.InstallMod)
			mods.DELETE("/:modId", r.UninstallMod)
			mods.PUT("/:modId/toggle", r.ToggleMod)
		}

		// System monitoring
		protected.GET("/system/stats", r.GetSystemStats)
	}
}
