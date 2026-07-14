package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/api"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/config"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Server represents the HTTP server
type Server struct {
	config      *config.Config
	db          *gorm.DB
	router      *gin.Engine
	staticFiles embed.FS
}

// New creates a new server instance
func New(cfg *config.Config, db *gorm.DB, staticFiles embed.FS) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &Server{
		config:      cfg,
		db:          db,
		router:      router,
		staticFiles: staticFiles,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// API routes
	apiRouter := api.NewRouter(s.db, s.config)

	// Reconcile any stale "running" server state left over from a previous run.
	if err := apiRouter.ProcessManager().ReconcileOnStartup(); err != nil {
		fmt.Printf("warning: startup reconciliation failed: %v\n", err)
	}
	// Refresh the persisted `installed` flag from on-disk reality.
	if err := apiRouter.ProcessManager().ReconcileInstalled(); err != nil {
		fmt.Printf("warning: installed reconciliation failed: %v\n", err)
	}

	apiGroup := s.router.Group("/api")
	apiRouter.RegisterRoutes(apiGroup)

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
