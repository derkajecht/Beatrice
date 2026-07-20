// Package utils contains utility functions for the project such as validation, listener,
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

	"github.com/derkajecht/Beatrice/internal/utils"
)

// IsEmpty returns true if the given string is empty
func IsEmpty(s string) bool {
	return s == ""
}

// IsArgsEmpty return false if any of the inputs are empty
func IsArgsEmpty(args ...string) bool {
	// len of args
	numArgs := len(args)
	// check if inputs are empty
	inputs := make(map[string]string, numArgs)
	for _, addr := range inputs {
		if IsEmpty(addr) {
			return false
		}
	}
	return true
}

// IsValidUsername returns true if the given string is not already taken
// queries the database
func IsValidUsername(db *sql.DB, username string) bool {
	// declare a variable to store the number of rows
	var exists int
	// check if the username exists in the database
	err := db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true // username is available
		}
		return false // an error occurred
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
