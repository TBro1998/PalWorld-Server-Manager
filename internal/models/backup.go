package models

import "time"

// Backup scope values: what a backup archive contains.
const (
	BackupScopeSave   = "save"   // world save tree (Pal/Saved/SaveGames/0)
	BackupScopeConfig = "config" // server config dir (Pal/Saved/Config/<OS>Server)
	BackupScopeAll    = "all"    // both save and config
)

// Backup source values: how the backup was created.
const (
	BackupSourceManual     = "manual"      // user-triggered
	BackupSourceAuto       = "auto"        // scheduler-triggered
	BackupSourcePreRestore = "pre-restore" // safety snapshot taken before a restore
)

// Backup records a single archived backup for a server. The DB row is the
// authoritative source for the backup list; FilePath points at the on-disk zip
// (<BackupDir>/<serverID>/<id>.zip). The zip also embeds a manifest.json so a
// backup remains self-describing outside this tool.
//
// Explicit column tags pin every column name; the code addresses columns by raw
// string in Where/Select/Order, matching the Server/Mod convention.
type Backup struct {
	ID        int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	ServerID  int64     `json:"server_id" gorm:"column:server_id;not null;index"`
	Scope     string    `json:"scope" gorm:"column:scope;not null"`     // save | config | all
	Source    string    `json:"source" gorm:"column:source;not null"`   // manual | auto | pre-restore
	Hot       bool      `json:"hot" gorm:"column:hot;default:false"`    // taken while the server was running
	SizeBytes int64     `json:"size_bytes" gorm:"column:size_bytes;default:0"`
	FilePath  string    `json:"-" gorm:"column:file_path;not null"`     // on-disk zip path; not exposed to clients
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`    // auto-managed by GORM
}

// BackupSchedule holds the per-server automatic-backup configuration. One row
// per server (ServerID is the primary key). KeepCount / KeepDays of 0 mean "no
// limit" on that dimension.
type BackupSchedule struct {
	ServerID        int64     `json:"server_id" gorm:"column:server_id;primaryKey"`
	Enabled         bool      `json:"enabled" gorm:"column:enabled;default:false"`
	IntervalMinutes int       `json:"interval_minutes" gorm:"column:interval_minutes;default:60"`
	Scope           string    `json:"scope" gorm:"column:scope;default:'all'"`
	KeepCount       int       `json:"keep_count" gorm:"column:keep_count;default:10"` // 0 = unlimited
	KeepDays        int       `json:"keep_days" gorm:"column:keep_days;default:0"`    // 0 = unlimited
	UpdatedAt       time.Time `json:"updated_at" gorm:"column:updated_at"`
}
