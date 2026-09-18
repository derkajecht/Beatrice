package server

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/derkajecht/Beatrice/src/shared"
	_ "modernc.org/sqlite"
)

// TODO: This still prepends $HOME to every non-~ location, so absolute paths
// and relative paths are both resolved somewhere other than the requested
// location (see TestNewDatabaseCreatesFileNotDir).

// NewDatabase initializes a new database connection
// and checks if the connection is successful
func NewDatabase(dbName, dbLocation string) (*DatabaseInfo, error) {
	// Check for empty args before initializing the db struct
	if shared.HasEmptyArgs(dbName, dbLocation) {
		return nil, fmt.Errorf("no database name or location provided")
	}

	// create NewDatabaseInfo struct
	cfg := NewDatabaseInfo(dbName, dbLocation)

	// Expand ~
	if cfg.Location != "" {
		home, err := os.UserHomeDir()
		if cfg.Location[0] == '~' {
			if err != nil {
				return nil, fmt.Errorf("failed to expand home directory: %w", err)
			}
			cfg.Location = home + cfg.Location[1:]
		} else {
			if err != nil {
				return nil, fmt.Errorf("failed to expand home directory: %w", err)
			}
			cfg.Location = home + "/" + cfg.Location
		}
	} else {
		err := cfg.CreateLocation()
		if err != nil {
			return nil, fmt.Errorf("failed to create default location: %w", err)
		}
	}

	// create the target directory if it doesn't exist
	if err := os.MkdirAll(cfg.Location, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	// open the database connection
	db, err := cfg.Pingdb()
	if err != nil {
		return nil, fmt.Errorf("failed to attach database: %w", err)
	}
	cfg.DB = db

	schema := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nickname TEXT NOT NULL UNIQUE,
			public_key TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
            sender TEXT NOT NULL,
            recipient TEXT NOT NULL,
            iv TEXT NOT NULL,
            encrypted_key TEXT NOT NULL,
            encrypted_content TEXT NOT NULL,
            signature TEXT NOT NULL,
            timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (sender) REFERENCES users(nickname)
        );`

	// execute the schema, commit the tables
	_, err = db.Exec(schema)
	if err != nil {
		db.Close() // clean up on failure
		return nil, fmt.Errorf("failed to create database schema: %w", err)
	}

	// log that the database was initialized successfully
	slog.Info("Database initialized successfully", "location", cfg.Location)

	return &cfg, nil
}

// StoreUser adds a new user to the database - nickname and public key
func (cfg *DatabaseInfo) StoreUser(nickname string, pubKey []byte) error {
	_, err := cfg.DB.Exec("INSERT INTO users (nickname, public_key) VALUES (?, ?)", nickname, pubKey)
	if err != nil {
		return fmt.Errorf("failed to store user %q: %w", nickname, err)
	}
	return nil
}

// PingUser queries the db to check if a user exists or not. Returns an error
// when the user is absent.
// TODO: Query returns an empty *sql.Rows without sql.ErrNoRows, so this
// function reports success for every successful query and also discards rows
// without closing them. Use QueryRow(...).Scan or inspect and close Rows.
// func (cfg *DatabaseInfo) PingUser(nickname string) (bool, error) {
// 	var enough bool
// 	if err := cfg.DB.QueryRow("SELECT * FROM users WHERE nickname = ?", nickname).Scan(&enough); err != nil {
// 		if err == sql.ErrNoRows {
// 			return false, fmt.Errorf("failed to look up user %q: %w", nickname, err)
// 		}
// 		return false, fmt.Errorf("failed to look up user %q: %w", nickname, err)
// 	}
// 	return true, nil
// }

// GetUserPK returns the stored public key for a nickname and whether the user exists.
func (cfg *DatabaseInfo) GetUserPK(nickname string) ([]byte, bool, error) {
	var pubKey []byte
	err := cfg.DB.QueryRow("SELECT public_key FROM users WHERE nickname = ?", nickname).Scan(&pubKey)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to look up user %q: %w", nickname, err)
	}
	return pubKey, true, nil
}
