package models

import "time"

// Server represents a Palworld server instance.
//
// Explicit column tags pin every DB column name. The code addresses columns by
// raw string in Select/Where/Update/Order (e.g. "pid", "install_path"), so the
// column names must be deterministic and not depend on GORM's naming strategy —
// notably PID, which GORM would otherwise map to "p_id" (not a known initialism).
type Server struct {
	ID          int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"column:name;not null"`
	InstallPath string    `json:"install_path" gorm:"column:install_path;not null"`
	Status      string    `json:"status" gorm:"-"` // derived value (running/stopped/installing/error); NOT persisted
	PID         int       `json:"pid" gorm:"column:pid;default:0"`
	LaunchArgs  string    `json:"launch_args" gorm:"column:launch_args;default:''"`         // JSON-encoded palconfig.LaunchArgs
	Installed   bool      `json:"installed" gorm:"column:installed;default:false"`         // server files present at install_path
	LastError   string    `json:"last_error,omitempty" gorm:"column:last_error;default:''"` // last install/start failure; cleared on success
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`                     // auto-managed by GORM
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`                     // auto-managed by GORM
}

// Mod represents a workshop mod
type Mod struct {
	ID          int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ServerID    int64     `json:"server_id" gorm:"column:server_id;not null;index"`
	WorkshopID  string    `json:"workshop_id" gorm:"column:workshop_id;not null"`
	Name        string    `json:"name" gorm:"column:name;not null"`
	Enabled     bool      `json:"enabled" gorm:"column:enabled;default:true"`
	InstallPath string    `json:"install_path" gorm:"column:install_path"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// User represents an authenticated user
type User struct {
	ID           int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Username     string    `json:"username" gorm:"column:username;uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"column:password_hash;not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"column:created_at"`
}
