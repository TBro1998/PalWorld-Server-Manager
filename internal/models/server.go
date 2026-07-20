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
	Installed   bool      `json:"installed" gorm:"column:installed;default:false"`          // server files present at install_path
	LastError   string    `json:"last_error,omitempty" gorm:"column:last_error;default:''"` // last install/start failure; cleared on success
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`                      // auto-managed by GORM
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`                      // auto-managed by GORM
}

// Mod represents a workshop mod in the global library.
//
// WorkshopID is the unique Steam Workshop item id (global business key).
// PackageName, ModName, Version and Tags are backfilled from the mod's Info.json
// after a successful download. Downloaded tracks whether SteamCMD has fetched
// the files; DownloadPath is the steamcmd staging directory
// (<steamcmdPath>/steamapps/workshop/content/1623730/<workshopID>).
//
// Mods are NOT owned by any single server — they live in a shared library.
// Use ServerMod to link a mod to a server and track per-server deployment state.
type Mod struct {
	ID           int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	WorkshopID   string    `json:"workshop_id" gorm:"column:workshop_id;not null;uniqueIndex"`
	Name         string    `json:"name" gorm:"column:name;not null"`
	Downloaded   bool      `json:"downloaded" gorm:"column:downloaded;default:false"`
	DownloadPath string    `json:"download_path" gorm:"column:download_path;default:''"`   // steamcmd staging dir; set after download
	PackageName  string    `json:"package_name" gorm:"column:package_name;default:''"`     // from Info.json; ActiveModList uses this
	ModName      string    `json:"mod_name" gorm:"column:mod_name;default:''"`             // from Info.json; display-only
	Version      string    `json:"version" gorm:"column:version;default:''"`               // from Info.json; update detection / display
	Tags         []string  `json:"tags" gorm:"column:tags;serializer:json"`                 // from Info.json; display-only (JSON in DB, may be null)
	Dependencies []string  `json:"dependencies" gorm:"column:dependencies;serializer:json"` // from Info.json; PackageNames this mod depends on (JSON in DB, may be null)
	CreatedAt    time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// ServerMod represents a server's reference to a global Mod library entry.
//
// A server can reference any subset of the global mod library; each reference
// carries an Enabled flag (controls whether the mod appears in ActiveModList)
// and DeployedVersion (the version string last copied into the server's
// Mods/Workshop directory). When DeployedVersion differs from Mod.Version the
// UI shows a version-mismatch indicator and StartServer auto-syncs the files.
type ServerMod struct {
	ID              int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ServerID        int64     `json:"server_id" gorm:"column:server_id;not null;index"`
	ModID           int64     `json:"mod_id" gorm:"column:mod_id;not null;index"`
	Enabled         bool      `json:"enabled" gorm:"column:enabled;default:true"`
	DeployedVersion string    `json:"deployed_version" gorm:"column:deployed_version;default:''"` // version last deployed to this server
	CreatedAt       time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// Setting is a runtime-adjustable key/value store for app-wide settings. It
// currently holds the Steam username used for workshop downloads and a
// session-ready flag set after a successful app-in login. Passwords are NEVER
// stored here (or anywhere): only the username and the "session cached" marker
// are persisted.
type Setting struct {
	Key   string `json:"key" gorm:"column:key;primaryKey"`
	Value string `json:"value" gorm:"column:value"`
}

// User represents an authenticated user
type User struct {
	ID           int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Username     string    `json:"username" gorm:"column:username;uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"column:password_hash;not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"column:created_at"`
}
