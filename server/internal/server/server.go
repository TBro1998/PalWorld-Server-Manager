package server

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zhuzhenghan/palworld-server-manager/internal/api"
	"github.com/zhuzhenghan/palworld-server-manager/internal/config"
)

// Server represents the HTTP server
type Server struct {
	config      *config.Config
	db          *sql.DB
	router      *gin.Engine
	staticFiles embed.FS
}

// New creates a new server instance
func New(cfg *config.Config, db *sql.DB, staticFiles embed.FS) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

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
