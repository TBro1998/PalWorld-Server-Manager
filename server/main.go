package main

import (
	"embed"
	"log"

	"github.com/zhuzhenghan/palworld-server-manager/internal/config"
	"github.com/zhuzhenghan/palworld-server-manager/internal/database"
	"github.com/zhuzhenghan/palworld-server-manager/internal/server"
)

//go:embed all:../../ui/out
var staticFiles embed.FS

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.Initialize(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create and start server
	srv := server.New(cfg, db, staticFiles)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
