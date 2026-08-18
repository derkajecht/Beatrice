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
	"strings"
	"time"
	"unicode"

	"github.com/derkajecht/Beatrice/src/shared"
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

// CreateLocation creates the target directory for the database depending on the OS
func (cfg *DatabaseInfo) CreateLocation() error {
	// store the OS type in j
	j := shared.CheckOS()
	// switch on the OS type and assign the appropriate location
	switch j {
	case shared.Darwin:
		cfg.Location = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Beatrice")
	case shared.Linux:
		cfg.Location = filepath.Join(os.Getenv("HOME"), ".beatrice")
	case shared.Windows:
		cfg.Location = filepath.Join(os.Getenv("APPDATA"), "Beatrice")
	default:
		return fmt.Errorf("unknown OS")
	}
	return nil
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
