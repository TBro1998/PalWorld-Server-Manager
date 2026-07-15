// Package settings provides tiny helpers over the models.Setting key/value
// table for runtime-adjustable app-wide settings (currently the Steam username
// and login-session marker used for workshop downloads).
//
// SECURITY: this store never holds secrets. Steam passwords are only used
// transiently during a login call and are never written here.
package settings

import (
	"errors"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// KeySteamUsername is the Steam account used to download workshop mods.
	KeySteamUsername = "steam_username"
	// KeySteamSessionReady is "true" once an app-in login has cached a SteamCMD
	// session; empty/unset otherwise.
	KeySteamSessionReady = "steam_session_ready"
)

// Get returns the stored value for key, or "" when it has never been set.
func Get(db *gorm.DB, key string) (string, error) {
	var s models.Setting
	err := db.First(&s, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s.Value, nil
}

// Set upserts key=value (insert or update on the primary key).
func Set(db *gorm.DB, key, value string) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&models.Setting{Key: key, Value: value}).Error
}
