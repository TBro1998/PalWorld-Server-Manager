package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/api"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/backup"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/config"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/update"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	_ "github.com/TBro1998/PalWorld-Server-Manager/docs" // swagger docs
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
)

// Server represents the HTTP server
type Server struct {
	config      *config.Config
	db          *gorm.DB
	router      *gin.Engine
	staticFiles embed.FS
	buildInfo   update.BuildInfo
}

// New creates a new server instance
func New(cfg *config.Config, db *gorm.DB, staticFiles embed.FS, buildInfo update.BuildInfo) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &Server{
		config:      cfg,
		db:          db,
		router:      router,
		staticFiles: staticFiles,
		buildInfo:   buildInfo,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// API routes
	apiRouter := api.NewRouter(s.db, s.config, s.buildInfo)

	// Reconcile any stale "running" server state left over from a previous run.
	if err := apiRouter.ProcessManager().ReconcileOnStartup(); err != nil {
		fmt.Printf("warning: startup reconciliation failed: %v\n", err)
	}
	// Refresh the persisted `installed` flag from on-disk reality.
	if err := apiRouter.ProcessManager().ReconcileInstalled(); err != nil {
		fmt.Printf("warning: installed reconciliation failed: %v\n", err)
	}

	// Start background update check (non-blocking; result cached for UI).
	apiRouter.Checker().StartBackgroundCheck()

	// Reconcile backup records with on-disk zips (drop orphans whose file is
	// gone), then start per-server automatic-backup schedules.
	if n, err := backup.Reconcile(s.db); err != nil {
		fmt.Printf("warning: backup reconciliation failed: %v\n", err)
	} else if n > 0 {
		fmt.Printf("backup reconciliation removed %d orphan record(s)\n", n)
	}
	apiRouter.BackupScheduler().Start()

	apiGroup := s.router.Group("/api")
	apiRouter.RegisterRoutes(apiGroup)

	// Swagger UI and OpenAPI spec (public, no auth required)
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Serve static files (Next.js build output)
	staticFS, err := fs.Sub(s.staticFiles, "ui/out")
	if err == nil {
		s.router.NoRoute(func(c *gin.Context) {
			c.FileFromFS(c.Request.URL.Path, http.FS(staticFS))
		})
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	fmt.Printf("Server starting on http://%s\n", addr)

	return s.router.Run(addr)
}
