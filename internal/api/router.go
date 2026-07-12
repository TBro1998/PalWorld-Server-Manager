package api

import (
	"github.com/TBro1998/PalWorld-Server-Manager/internal/config"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/process"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Router handles API routing
type Router struct {
	db      *gorm.DB
	config  *config.Config
	process *process.Manager
	streams *logger.StreamManager
}

// NewRouter creates a new API router
func NewRouter(db *gorm.DB, cfg *config.Config) *Router {
	streams := logger.NewStreamManager()
	pm := process.NewManager(db, streams, cfg.LogDir, cfg.SteamCMDPath)
	return &Router{
		db:      db,
		config:  cfg,
		process: pm,
		streams: streams,
	}
}

// ProcessManager exposes the process manager for startup reconciliation.
func (r *Router) ProcessManager() *process.Manager {
	return r.process
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
			servers.GET("/:id/config", r.GetServerConfig)
			servers.PUT("/:id/config", r.UpdateServerConfig)
		}

		// Config schema (drives the structured config form)
		protected.GET("/config/schema", r.GetConfigSchema)

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
