// Package validation contains utility functions for the project such as validation, listener,
// responses and router
package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

func UsernameSanitizer(username string) string {
	for _, r := range username {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			username = strings.ReplaceAll(username, string(r), "")
			// TODO: Draw this to the TUI and log it
			slog.Warn("Invalid character in username. Replacing with empty character.", "username", username)
		}
	}
	return username
}

// IsValidUsername returns true if the given string is not already taken
// also checks username length and sanitizes the username
// func IsValidUsername(username string) bool {
// 	// Sanitize the username first to remove any invalid characters
// 	username = UsernameSanitizer(username)
// 	if len(username) < 3 {
// 		// Draw to TUI and re-prompt the user for a valid username
// 		slog.Warn("Username is too short. Please try again.", "username", username)
// 		return false // username is too short
// 	}
// 	// ping the db first
// 	db, err := Pingdb()
// 	if err != nil {
// 		slog.Error("Error pinging database:", "err", err)
// 		return false
// 	}
//
// 	// declare a variable to store the number of rows
// 	var exists int
// 	// check if the username exists in the database
// 	err = db.QueryRow("SELECT id FROM users WHERE nickname = ?", username).Scan(&exists)
// 	if err != nil {
// 		if errors.Is(err, sql.ErrNoRows) {
// 			return true // username is available
// 		}
// 		// this could be caused by a race condition, so just log the error
// 		slog.Error("Error checking if username is available:", "err", err, "username", username)
// 	}
// 	return false // username is already taken
// }

const (
	darwin  = iota // 0
	linux          // 1
	windows        // 2
	unknown        // 3
)

// simple check for OS
func checkOS() int {
	switch os := runtime.GOOS; os {
	case "darwin":
		return darwin
	case "linux":
		return linux
	case "windows":
		return windows
	default:
		return unknown
	}
}

// CreateLocation creates the target directory for the database depending on the OS
func (cfg *DatabaseInfo) CreateLocation() error {
	// store the OS type in j
	j := checkOS()
	// switch on the OS type and assign the appropriate location
	switch j {
	case darwin:
		cfg.Location = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", cfg.Name)
	case linux:
		cfg.Location = filepath.Join(os.Getenv("HOME"), ".beatrice")
	case windows:
		cfg.Location = filepath.Join(os.Getenv("APPDATA"), cfg.Name)
	default:
		return fmt.Errorf("unknown OS")
	}
	return nil
}

// IsValidLocation returns true if the given string is a valid location
// queries the file system to check if the location exists
func (cfg *DatabaseInfo) IsValidLocation(location string) (bool, error) {
	if _, err := os.Open(location); err != nil {
		err := cfg.CreateLocation()
		if err != nil {
			return false, fmt.Errorf("failed to create location: %w", err)
		}
	}
	return true, nil
}

// Pingdb checks if the database is accessible
// uses the location and name from the config struct. No need to pass it in.
func (cfg *DatabaseInfo) Pingdb() (*sql.DB, error) {
	// join the target directory and the database name to create the final path
	// ensure that ~ is expanded before opening the connection
	if strings.HasPrefix(cfg.Location, "~") {
		expandedPath, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		cfg.Location = strings.Replace(cfg.Location, "~", expandedPath, 1)
	}
	dbPath := filepath.Join(cfg.Location, cfg.Name)
	cfg.Dsn = fmt.Sprintf("file:%s/?_foreign_keys=on&_pragma=busy_timeout(50000)", dbPath)

	// open the database connection
	db, err := sql.Open("sqlite", cfg.Dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// check if the database is accessible
	if err := db.PingContext(ctx); err != nil {
		db.Close() // clean up on failure
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return db, nil
}
