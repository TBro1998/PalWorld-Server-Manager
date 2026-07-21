package config

import (
	"os"
	"strconv"

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
	JWTSecret string `yaml:"jwt_secret" env:"JWT_SECRET" envDefault:""`

	// Auth — bcrypt hash of the admin password; empty means not yet configured.
	PasswordHash string `yaml:"password_hash" env:"PASSWORD_HASH" envDefault:""`

	// Palworld server paths
	SteamCMDPath string `yaml:"steamcmd_path" env:"STEAMCMD_PATH" envDefault:"./steamcmd"`

	// SteamUsername is the Steam account used to download Workshop mods. Palworld
	// is a paid title, so anonymous login cannot download its workshop content —
	// this must be an account that OWNS Palworld. Only the username is used; the
	// user must run `steamcmd +login <user>` once interactively (handling any
	// Steam Guard prompt) so SteamCMD caches the session this reuses. Empty
	// disables authenticated download (falls back to anonymous, which fails for
	// Palworld mods).
	SteamUsername string `yaml:"steam_username" env:"STEAM_USERNAME" envDefault:""`

	// Logging
	LogDir string `yaml:"log_dir" env:"LOG_DIR" envDefault:"./logs"`

	// Backup root directory. Backups live at <BackupDir>/<serverID>/<backupID>.zip,
	// independent of a server's install path so reinstalling or deleting a server's
	// files never touches its backups. Mirrors the LogDir "global root" pattern.
	BackupDir string `yaml:"backup_dir" env:"BACKUP_DIR" envDefault:"./backups"`

	// Update settings (always enabled, update via UI)
	GitHubRepo string `yaml:"github_repo" env:"GITHUB_REPO" envDefault:"TBro1998/PalWorld-Server-Manager"`
}

// Configured reports whether an admin password has been set.
func (c *Config) Configured() bool {
	return c.PasswordHash != ""
}

// Load loads configuration with priority: env vars > config.yaml > defaults.
// This allows Docker deployments to override any config.yaml setting via
// environment variables without modifying the file.
func Load() (*Config, error) {
	// Step 1: apply defaults via env package (envDefault tags).
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Step 2: overlay config.yaml on top of defaults (if the file exists).
	if data, err := os.ReadFile("config.yaml"); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Step 3: re-apply any explicitly set environment variables so they always
	// win over whatever config.yaml says.
	if v, ok := os.LookupEnv("HOST"); ok {
		cfg.Host = v
	}
	if v, ok := os.LookupEnv("PORT"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v, ok := os.LookupEnv("DATABASE_PATH"); ok {
		cfg.DatabasePath = v
	}
	if v, ok := os.LookupEnv("JWT_SECRET"); ok {
		cfg.JWTSecret = v
	}
	if v, ok := os.LookupEnv("PASSWORD_HASH"); ok {
		cfg.PasswordHash = v
	}
	if v, ok := os.LookupEnv("STEAMCMD_PATH"); ok {
		cfg.SteamCMDPath = v
	}
	if v, ok := os.LookupEnv("STEAM_USERNAME"); ok {
		cfg.SteamUsername = v
	}
	if v, ok := os.LookupEnv("LOG_DIR"); ok {
		cfg.LogDir = v
	}
	if v, ok := os.LookupEnv("BACKUP_DIR"); ok {
		cfg.BackupDir = v
	}
	if v, ok := os.LookupEnv("GITHUB_REPO"); ok {
		cfg.GitHubRepo = v
	}

	return cfg, nil
}

// Save writes the current configuration to config.yaml, creating the file if
// it does not yet exist. Called after the first-time password setup so that the
// generated jwt_secret and password_hash are persisted.
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile("config.yaml", data, 0600)
}
