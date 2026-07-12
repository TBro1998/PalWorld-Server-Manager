package main

import (
	"embed"
	"log"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/config"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/database"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/server"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/steamcmd"
)

//go:embed all:ui/out
var staticFiles embed.FS

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Check and install SteamCMD if not present
	log.Println("Checking SteamCMD installation...")
	if err := steamcmd.CheckAndInstall(cfg.SteamCMDPath); err != nil {
		log.Fatalf("Failed to setup SteamCMD: %v", err)
	}

	// Initialize database
	db, err := database.Initialize(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	// Create and start server
	srv := server.New(cfg, db, staticFiles)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
