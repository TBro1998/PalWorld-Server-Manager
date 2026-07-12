package models

import "time"

// Server represents a Palworld server instance
type Server struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	InstallPath string    `json:"install_path" db:"install_path"`
	Port        int       `json:"port" db:"port"`
	QueryPort   int       `json:"query_port" db:"query_port"`
	RCONPort    int       `json:"rcon_port" db:"rcon_port"`
	RCONEnabled bool      `json:"rcon_enabled" db:"rcon_enabled"`
	Status      string    `json:"status" db:"status"` // derived value (running/stopped/installing/error); NOT persisted
	PID         int       `json:"pid" db:"pid"`
	LaunchArgs  string    `json:"launch_args" db:"launch_args"`         // JSON-encoded palconfig.LaunchArgs
	Installed   bool      `json:"installed" db:"installed"`             // server files present at install_path
	LastError   string    `json:"last_error,omitempty" db:"last_error"` // last install/start failure; cleared on success
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Mod represents a workshop mod
type Mod struct {
	ID          int64     `json:"id" db:"id"`
	ServerID    int64     `json:"server_id" db:"server_id"`
	WorkshopID  string    `json:"workshop_id" db:"workshop_id"`
	Name        string    `json:"name" db:"name"`
	Enabled     bool      `json:"enabled" db:"enabled"`
	InstallPath string    `json:"install_path" db:"install_path"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// User represents an authenticated user
type User struct {
	ID           int64     `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}
