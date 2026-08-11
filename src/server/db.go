package server

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/derkajecht/Beatrice/src/client"
	"github.com/derkajecht/Beatrice/src/shared"
	_ "modernc.org/sqlite"
)

// NewDatabase initializes a new database connection
// and checks if the connection is successful
// if not, it will log the error and exit the program
func NewDatabase(dbName, dbLocation string) (string, error) {

	// create NewDatabaseInfo struct
	cfg := NewDatabaseInfo(dbName, dbLocation)

	// loop through the inputs and assign the default values to the cfg struct if empty
	if shared.HasEmptyArgs(dbName, dbLocation) {
		slog.Warn("No database name or location provided: Defaulting to beatrice.db and ./beatrice")
		return "", fmt.Errorf("no database name or location provided")
	}

	// validate the database location
	// if the location is not valid, log a warning and use the default location
	if valid, err := cfg.IsValidLocation(dbLocation); !valid {
		slog.Error("Invalid database location:", "err", err)
	}

	// create the target directory if it doesn't exist
	// if it does exist, it will be ignored
	err := os.MkdirAll(cfg.Location, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	// open the database connection
	db, err := cfg.Pingdb()
	if err != nil {
		return "", fmt.Errorf("failed to attach database: %w", err)
	}
	cfg.DB = db // set db to the struct so it can be called throughout the server
	defer cfg.DB.Close()

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
		return "", fmt.Errorf("failed to create database schema: %w", err)
	}

	// log that the database was initialized successfully
	log.Printf("Database initialized successfully at %s", cfg.Location)

	return cfg.Location, nil
}

// StoreUser adds a new user to the database - nickname and public key
func (cfg *DatabaseInfo) StoreUser(u *client.User) error {
	db, err := cfg.Pingdb()
	if err != nil {
		return fmt.Errorf("failed to attach database: %w", err)
	}

	// get user nickname and public key from the user struct
	nickname := u.Nickname
	publicKey := u.Crypto.PubKey

	// create a prepared statement
	schema := `
		INSERT INTO users (nickname, public_key)
		VALUES (?, ?);`

	// add user nickname and public key to the db
	_, err = db.Exec(schema, nickname, publicKey)
	if err != nil {
		db.Close() // clean up on failure
		return fmt.Errorf("failed to create database schema: %w", err)
	}
	return nil
}
