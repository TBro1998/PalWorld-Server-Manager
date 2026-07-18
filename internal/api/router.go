package api

import (
	"github.com/TBro1998/PalWorld-Server-Manager/internal/config"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/process"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/update"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Router handles API routing
type Router struct {
	db      *gorm.DB
	config  *config.Config
	process *process.Manager
	streams *logger.StreamManager
	saves   *saveCache
	checker *update.Checker
}

// NewRouter creates a new API router
func NewRouter(db *gorm.DB, cfg *config.Config, buildInfo update.BuildInfo) *Router {
	streams := logger.NewStreamManager()
	pm := process.NewManager(db, streams, cfg.LogDir, cfg.SteamCMDPath, cfg.SteamUsername)
	checker := update.NewChecker(cfg.GitHubRepo, buildInfo)
	return &Router{
		db:      db,
		config:  cfg,
		process: pm,
		streams: streams,
		saves:   newSaveCache(),
		checker: checker,
	}
}

// ProcessManager exposes the process manager for startup reconciliation.
func (r *Router) ProcessManager() *process.Manager {
	return r.process
}

// Checker exposes the update checker so server.go can start the background check.
func (r *Router) Checker() *update.Checker {
	return r.checker
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

			// Palworld REST API proxy: forwards to the co-located game server's
			// official REST API after reading port/password from its INI.
			rest := servers.Group("/:id/rest")
			{
				rest.GET("/status", r.RestStatus)
				rest.GET("/info", r.RestInfo)
				rest.GET("/metrics", r.RestMetrics)
				rest.GET("/players", r.RestPlayers)
				rest.GET("/settings", r.RestSettings)
				rest.POST("/announce", r.RestAnnounce)
				rest.POST("/kick", r.RestKick)
				rest.POST("/ban", r.RestBan)
				rest.POST("/unban", r.RestUnban)
				rest.POST("/save", r.RestSave)
				rest.POST("/shutdown", r.RestShutdown)
				rest.POST("/stop", r.RestStop)
			}

			// Save-file inspection: parses the co-located Level.sav / Players
			// saves (read-only) and exposes players, guilds, pals and
			// inventories. Independent of the live REST API.
			save := servers.Group("/:id/save")
			{
				save.GET("/players", r.SavePlayers)
				save.GET("/guilds", r.SaveGuilds)
				save.GET("/players/:uid/pals", r.SavePlayerPals)
				save.GET("/players/:uid/inventory", r.SavePlayerInventory)
			}
		}

		// Config schema (drives the structured config form)
		protected.GET("/config/schema", r.GetConfigSchema)

		// Global mod library (workshop-independent of any server)
		globalMods := protected.Group("/mods")
		{
			globalMods.GET("", r.ListGlobalMods)
			globalMods.POST("", r.AddGlobalMod)
			globalMods.DELETE("/:modId", r.DeleteGlobalMod)
			globalMods.POST("/:modId/download", r.DownloadGlobalMod)
			globalMods.GET("/:modId/logs/stream", r.ModLogStream)
		}

		// Per-server mod references (link/unlink/toggle/deploy from global library)
		serverMods := protected.Group("/servers/:id/mods")
		{
			serverMods.GET("", r.ListServerMods)
			serverMods.POST("", r.LinkServerMod)
			serverMods.DELETE("/:serverModId", r.UnlinkServerMod)
			serverMods.PUT("/:serverModId/toggle", r.ToggleServerMod)
			serverMods.POST("/deploy", r.DeployServerMods)
		}

		// Steam account (global, not per-server): app-in login that caches a
		// SteamCMD session for authenticated workshop downloads.
		steam := protected.Group("/steam")
		{
			steam.GET("/status", r.SteamStatus)
			steam.POST("/login", r.SteamLogin)
			steam.GET("/logs/stream", r.SteamLogStream)
			// Workshop search (proxies Steam Web API; key stays server-side).
			steam.GET("/workshop/search", r.WorkshopSearch)
			steam.GET("/workshop/mods/:workshopId/dependencies", r.WorkshopDependencies)
			steam.POST("/webapi-key", r.SetWebAPIKey)
		}

		// System monitoring
		protected.GET("/system/stats", r.GetSystemStats)

		// Version info & self-update
		system := protected.Group("/system")
		{
			system.GET("/version", r.GetVersion)
			system.GET("/update/check", r.CheckUpdate)
			system.GET("/update/status", r.GetUpdateStatus)
			// SECURITY: /update/apply downloads and replaces the running binary,
			// then restarts the process.  Once JWT auth is enabled this endpoint
			// will be automatically covered by the protected middleware.  Until
			// then the default host binding of 127.0.0.1 limits exposure.
			system.POST("/update/apply", r.ApplyUpdate)
			system.GET("/update/stream", r.UpdateStream)
			system.GET("/settings", r.GetSystemSettings)
			system.PUT("/settings", r.UpdateSystemSettings)
		}
	}
}
