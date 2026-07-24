package storage

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/derkajecht/Beatrice/internal/config"
	"github.com/derkajecht/Beatrice/internal/types"
	"github.com/derkajecht/Beatrice/internal/validation"
	_ "modernc.org/sqlite"
)

func Pingdb() (*sql.DB, error) {
	cfg := config.NewDatabaseInfo()
	// enable foreign key constraints
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys=(1)", cfg.Location)
	// open the database connection
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// check if the database is accessible
	if err := db.Ping(); err != nil {
		db.Close() // clean up on failure
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return db, nil
}

// NewDatabase initializes a new database connection
// and checks if the connection is successful
// if not, it will log the error and exit the program
func NewDatabase(dbName, dbLocation string) (*sql.DB, string, error) {

	// create NewDatabaseInfo struct
	cfg := config.NewDatabaseInfo()

	// loop through the inputs and assign the default values to the cfg struct if empty
	if validation.HasArgsEmpty(dbName, dbLocation) {
		slog.Warn("No database name or location provided: Defaulting to beatrice.db and ./beatrice")
		return nil, "", fmt.Errorf("no database name or location provided")
	}

	// validate the database location
	// if the location is not valid, log a warning and use the default location
	if valid, err := validation.IsValidLocation(dbLocation); !valid {
		slog.Error("Invalid database location:", "err", err)
		cfg.Location = "./beatrice"
	}

	// if the above validation is successful, assign the input values to the cfg struct
	// join the target directory and the database name to create the final path
	cfg.Name = dbName
	cfg.Location = filepath.Join(dbLocation, cfg.Name)

	// create the target directory if it doesn't exist
	// if it does exist, it will be ignored
	err := os.MkdirAll(cfg.Location, 0755)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create target directory: %w", err)
	}

	// open the database connection
	db, err := Pingdb()
	if err != nil {
		return nil, "", fmt.Errorf("failed to attach database: %w", err)
	}

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
		return nil, "", fmt.Errorf("failed to create database schema: %w", err)
	}

	// log that the database was initialized successfully
	log.Println("Database initialized successfully")

	return db, cfg.Location, nil
}

// StoreUser adds a new user to the database - nickname and public key
func StoreUser(u *types.User) error {
	db, err := Pingdb()
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
