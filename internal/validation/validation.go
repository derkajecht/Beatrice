// Package validation contains utility functions for the project such as validation, listener,
// responses and router
package validation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"slices"
	"strings"
	"unicode"

	"github.com/derkajecht/Beatrice/internal/storage"
	"github.com/derkajecht/Beatrice/internal/utils"
)

// IsEmpty returns true if the given string is empty
func IsEmpty(s string) bool {
	return s == ""
}

// HasArgsEmpty returns false if any of the inputs are empty
func HasArgsEmpty(args ...string) bool {
	return slices.Contains(args, "")
}

func usernameSanitizer(username string) string {
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
func IsValidUsername(username string) bool {
	// Sanitize the username first to remove any invalid characters
	username = usernameSanitizer(username)
	if len(username) < 3 {
		// Draw to TUI and re-prompt the user for a valid username
		slog.Warn("Username is too short. Please try again.", "username", username)
		return false // username is too short
	}
	// ping the db first
	db, err := storage.Pingdb()
	if err != nil {
		slog.Error("Error pinging database:", "err", err)
		return false
	}

	// declare a variable to store the number of rows
	var exists int
	// check if the username exists in the database
	err = db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true // username is available
		}
		// this could be caused by a race condition, so just log the error
		slog.Error("Error checking if username is available:", "err", err, "username", username)
	}
	return false // username is already taken
}

// IsValidLocation returns true if the given string is a valid location
// queries the file system to check if the location exists
func IsValidLocation(location string) (bool, error) {
	if _, err := os.Open(location); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("invalid location: %w", err)
		}
		return false, fmt.Errorf("failed to open location: %w", err)
	}
	return true, nil
}

// UnmarshalPacket unmarshals the given buffer and returns the envelope struct and error
// this as passed to the handle function to handle the packet based on the packet type
func UnmarshalPacket(conn net.Conn, buf []byte, n int, env any) error {
	if err := json.Unmarshal(buf[:n], env); err != nil {
		// log error
		slog.Error("Error unmarshalling envelope:", "err", err, "client", conn.RemoteAddr())

		// Send error packet to client
		utils.SendError(conn, "invalid_protocol_format")

		return err
	}
	return nil
}
