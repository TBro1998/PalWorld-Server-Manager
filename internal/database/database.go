package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Initialize creates and initializes the database
func Initialize(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS servers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		install_path TEXT NOT NULL,
		status TEXT DEFAULT 'stopped',
		pid INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS mods (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server_id INTEGER NOT NULL,
		workshop_id TEXT NOT NULL,
		name TEXT NOT NULL,
		enabled BOOLEAN DEFAULT 1,
		install_path TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// Additive column migrations for existing databases.
	if err := addColumnIfMissing(db, "servers", "launch_args", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(db, "servers", "installed", "BOOLEAN DEFAULT 0"); err != nil {
		return err
	}
	if err := addColumnIfMissing(db, "servers", "last_error", "TEXT DEFAULT ''"); err != nil {
		return err
	}

	// Remove deprecated lazy metadata columns from existing databases. These
	// columns (game/query/RCON ports and the RCON toggle) were never wired into
	// the runtime — launch_args + PalWorldSettings.ini are the sole source of
	// truth — so dropping them is non-destructive. Failures are logged and
	// tolerated: a residual column is harmless (nothing reads it) and must not
	// block startup.
	for _, col := range []string{"port", "query_port", "rcon_port", "rcon_enabled"} {
		if err := dropColumnIfExists(db, "servers", col); err != nil {
			fmt.Printf("warning: failed to drop column servers.%s: %v\n", col, err)
		}
	}
	return nil
}

// dropColumnIfExists drops a column from a table only when it currently exists,
// making the migration idempotent across restarts and versions. Unlike
// addColumnIfMissing, a failed DROP is returned to the caller (which logs and
// continues) rather than treated as fatal: the affected columns are unused
// lazy metadata, so a residual column is harmless and must not abort startup.
func dropColumnIfExists(db *sql.DB, table, column string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			dfltValue  sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &primaryKey); err != nil {
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !found {
		return nil // already absent
	}

	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column)); err != nil {
		return fmt.Errorf("drop column %s.%s: %w", table, column, err)
	}
	return nil
}

// addColumnIfMissing adds a column to a table only when it does not already
// exist, making migrations idempotent across restarts and versions.
func addColumnIfMissing(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			dfltValue  sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &primaryKey); err != nil {
			return err
		}
		if name == column {
			return nil // already present
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}
