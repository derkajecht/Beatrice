package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// NewDatabase initializes a new database connection
// and checks if the connection is successful
// if not, it will log the error and exit the program
func NewDatabase(dbName string) (*sql.DB, error, string) {

	// TODO: make this configurable in the future
	// set the target directory for the database file
	target := "./data"

	// create the target directory if it doesn't exist
	// if it does exist, it will be ignored
	err := os.MkdirAll(target, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err), ""
	}

	// join the target directory and the database name to create the final path
	finalPath := filepath.Join(target, dbName)

	// enable foreign key constraints
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys=(1)", finalPath)

	// open the database connection
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err), ""
	}

	// check if the database is accessible
	if err := db.Ping(); err != nil {
		db.Close() // clean up on failure
		return nil, fmt.Errorf("failed to ping database: %w", err), ""
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
		return nil, fmt.Errorf("failed to create database schema: %w", err), ""
	}

	// log that the database was initialized successfully
	log.Println("Database initialized successfully")

	return db, nil, finalPath
}
