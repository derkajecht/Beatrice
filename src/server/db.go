package server

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/derkajecht/Beatrice/src/shared"
	_ "modernc.org/sqlite"
)

// NewDatabase initializes a new database connection
// and checks if the connection is successful
func NewDatabase(dbName, dbLocation string) (*DatabaseInfo, error) {

	// create NewDatabaseInfo struct
	cfg := NewDatabaseInfo(dbName, dbLocation)

	if shared.HasEmptyArgs(dbName, dbLocation) {
		return nil, fmt.Errorf("no database name or location provided")
	}

	// Expand ~
	if cfg.Location != "" {
		if cfg.Location[0] == '~' {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to expand home directory: %w", err)
			}
			cfg.Location = home + cfg.Location[1:]
		}
	} else {
		if err := cfg.CreateLocation(); err != nil {
			return nil, fmt.Errorf("failed to create default location: %w", err)
		}
	}

	// create the target directory if it doesn't exist
	if err := os.MkdirAll(cfg.Location, 0755); err != nil {
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
