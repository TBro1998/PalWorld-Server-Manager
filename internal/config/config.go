package config

import (
	"os"

	"github.com/caarlos0/env/v11"
	"gopkg.in/yaml.v3"
)

// Config holds application configuration
type Config struct {
	// Server settings
	Host string `yaml:"host" env:"HOST" envDefault:"127.0.0.1"`
	Port int    `yaml:"port" env:"PORT" envDefault:"8080"`

	// Database
	DatabasePath string `yaml:"database_path" env:"DATABASE_PATH" envDefault:"./palworld.db"`

	// JWT
	JWTSecret string `yaml:"jwt_secret" env:"JWT_SECRET" envDefault:"change-me-in-production"`

	// Palworld server paths
	SteamCMDPath     string `yaml:"steamcmd_path" env:"STEAMCMD_PATH" envDefault:"./steamcmd"`
	PalworldBasePath string `yaml:"palworld_base_path" env:"PALWORLD_BASE_PATH" envDefault:"./palworld"`

	// Logging
	LogDir string `yaml:"log_dir" env:"LOG_DIR" envDefault:"./logs"`

	// Update settings (always enabled, update via UI)
	GitHubRepo string `yaml:"github_repo" env:"GITHUB_REPO" envDefault:"TBro1998/PalWorld-Server-Manager"`
}

// Load loads configuration with priority: config.yaml > environment variables > defaults
func Load() (*Config, error) {
	cfg := &Config{}

	// Try to load from config.yaml first
	if data, err := os.ReadFile("config.yaml"); err == nil {
		// config.yaml exists, use it
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	// config.yaml doesn't exist, load from environment variables with defaults
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
